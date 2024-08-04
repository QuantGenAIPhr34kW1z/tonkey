package main

import (
	"context"
	_ "embed"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"database/sql"

	"tonkey/internal/api"
	"tonkey/internal/apikeys"
	"tonkey/internal/audit"
	"tonkey/internal/batch"
	"tonkey/internal/config"
	"tonkey/internal/contracts"
	"tonkey/internal/dashboard"
	"tonkey/internal/georatelimit"
	"tonkey/internal/graphql"
	"tonkey/internal/ipfilter"
	"tonkey/internal/jetton"
	"tonkey/internal/jobs"
	"tonkey/internal/logger"
	"tonkey/internal/middleware"
	"tonkey/internal/migrations"
	"tonkey/internal/multisig"
	"tonkey/internal/nft"
	"tonkey/internal/openapi"
	"tonkey/internal/registration"
	"tonkey/internal/reset"
	"tonkey/internal/store"
	"tonkey/internal/tg"
	"tonkey/internal/ton"
	"tonkey/internal/totp"
	"tonkey/internal/tracing"
	"tonkey/internal/users"
	"tonkey/internal/webhooks"
	"tonkey/internal/websocket"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed schema.sql
var schemaSQL string

func main() {
	cfgPath := flag.String("config", "./configs/config.example.yaml", "config path")
	debug := flag.Bool("debug", false, "enable debug logging")
	migrate := flag.Bool("migrate", false, "run database migrations and exit")
	flag.Parse()

	logger.SetDebugMode(*debug)
	logger.Info.Printf("Starting tonkey 3.4-prod")

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error.Fatalf("Failed to load config: %v", err)
	}
	logger.Info.Printf("Loaded config from %s", *cfgPath)

	// Open database based on driver
	var st store.TxStore
	var sqlDB *sql.DB

	switch cfg.Database.Driver {
	case "postgres":
		pgStore, err := store.OpenPostgres(cfg.Database.ConnectionString)
		if err != nil {
			logger.Error.Fatalf("Failed to open PostgreSQL database: %v", err)
		}
		st = pgStore
		sqlDB = pgStore.DB
		logger.Info.Printf("PostgreSQL database opened")

	case "sqlite":
		sqliteStore, err := store.Open(cfg.Database.Path, schemaSQL)
		if err != nil {
			logger.Error.Fatalf("Failed to open SQLite database: %v", err)
		}
		st = sqliteStore
		sqlDB = sqliteStore.DB
		logger.Info.Printf("SQLite database opened: %s", cfg.Database.Path)

	default:
		logger.Error.Fatalf("Unknown database driver: %s", cfg.Database.Driver)
	}

	// Initialize migration manager
	migMgr := migrations.NewManager(sqlDB)
	migMgr.Init()
	migMgr.RegisterDefaultMigrations()

	// Handle migration-only mode
	if *migrate {
		status, _ := migMgr.GetStatus()
		logger.Info.Printf("Current schema version: %d, pending migrations: %d", status.CurrentVersion, status.PendingCount)

		applied, err := migMgr.Migrate()
		if err != nil {
			logger.Error.Fatalf("Migration failed: %v", err)
		}
		for _, m := range applied {
			logger.Info.Printf("Applied migration %d: %s", m.Version, m.Name)
		}
		if len(applied) == 0 {
			logger.Info.Println("No migrations to apply")
		}
		os.Exit(0)
	}

	// Auto-migrate if enabled
	if cfg.Migrations.AutoMigrate {
		applied, err := migMgr.Migrate()
		if err != nil {
			logger.Error.Printf("Auto-migration failed: %v", err)
		} else if len(applied) > 0 {
			logger.Info.Printf("Auto-applied %d migrations", len(applied))
		}
	}

	// Initialize TON provider
	var provider ton.Provider
	switch cfg.Provider.Kind {
	case "mock":
		provider = ton.NewMock()
		logger.Info.Println("Using mock TON provider")
	case "toncenter":
		if cfg.Provider.APIKey != "" {
			provider = ton.NewTonCenterWithKey(cfg.Provider.TonCenterURL, cfg.Provider.APIKey)
			logger.Info.Println("Using TON Center provider with API key")
		} else {
			provider = ton.NewTonCenter(cfg.Provider.TonCenterURL)
			logger.Info.Println("Using TON Center provider")
		}
	case "liteclient":
		liteServers := make([]ton.LiteServerConfig, len(cfg.Provider.LiteServers))
		for i, s := range cfg.Provider.LiteServers {
			liteServers[i] = ton.LiteServerConfig{
				Host:      s.Host,
				Port:      s.Port,
				PublicKey: s.PublicKey,
			}
		}
		liteClient, err := ton.NewLiteClient(ton.LiteClientConfig{
			Servers: liteServers,
			Timeout: cfg.Provider.Timeout,
		})
		if err != nil {
			logger.Error.Fatalf("Failed to create lite client: %v", err)
		}
		if err := liteClient.Connect(context.Background()); err != nil {
			logger.Warn.Printf("Lite client initial connection failed: %v", err)
		}
		provider = liteClient
		logger.Info.Printf("Using lite client provider with %d servers", len(liteServers))
	default:
		logger.Warn.Printf("Unknown provider kind %q, using mock", cfg.Provider.Kind)
		provider = ton.NewMock()
	}

	// Initialize optional components
	var wsHub *websocket.Hub
	if cfg.WebSocket.Enabled {
		wsHub = websocket.NewHub()
		go wsHub.Run()
		logger.Info.Println("WebSocket hub started")
	}

	var webhookMgr *webhooks.Manager
	if cfg.Webhooks.Enabled {
		webhookMgr = webhooks.NewManager(sqlDB)
		webhookMgr.Start(cfg.Webhooks.Workers)
		logger.Info.Printf("Webhook manager started with %d workers", cfg.Webhooks.Workers)
	}

	// Initialize user management (enabled by default for database-backed auth)
	userMgr := users.NewManager(sqlDB)
	logger.Info.Println("User management enabled")

	var totpMgr *totp.Manager
	if cfg.TOTP.Enabled {
		totpMgr = totp.NewManager(sqlDB, cfg.TOTP.Issuer)
		logger.Info.Println("TOTP 2FA enabled")
	}

	var auditLogger *audit.Logger
	if cfg.Audit.Enabled {
		auditLogger = audit.NewLogger(sqlDB, true)
		logger.Info.Printf("Audit logging enabled (retention: %d days)", cfg.Audit.RetentionDays)
	} else {
		auditLogger = audit.NewLogger(sqlDB, false)
	}

	resetMgr := reset.NewManager(sqlDB)
	logger.Debug.Println("Password reset manager initialized")

	var ipFilter *ipfilter.Filter
	if cfg.IPFilter.Enabled {
		ipFilter = ipfilter.NewFilter(sqlDB, true, cfg.IPFilter.DefaultMode)
		logger.Info.Printf("IP filtering enabled (default: %s)", cfg.IPFilter.DefaultMode)
	}

	var regMgr *registration.Manager
	if cfg.Auth.AllowRegistration {
		regMgr = registration.NewManager(sqlDB, registration.Config{
			Enabled:             true,
			RequireEmail:        true,
			RequireVerification: cfg.Auth.RequireVerification,
			DefaultOrgID:        cfg.Auth.DefaultOrgID,
			DefaultRole:         "user",
			PasswordMinLength:   cfg.Auth.PasswordMinLength,
			AllowedDomains:      cfg.Auth.AllowedDomains,
		})
		logger.Info.Println("User self-registration enabled")
	}

	var contractMgr *contracts.Manager
	if cfg.Contracts.Enabled {
		contractMgr = contracts.NewManager(sqlDB, &contractProvider{provider})
		logger.Info.Println("Smart contract management enabled")
	}

	var multisigMgr *multisig.Manager
	if cfg.Multisig.Enabled {
		multisigMgr = multisig.NewManager(sqlDB)
		logger.Info.Println("Multi-signature wallet support enabled")
	}

	var batchMgr *batch.Manager
	if cfg.Batch.Enabled {
		batchMgr = batch.NewManager(sqlDB, provider)
		logger.Info.Println("Transaction batching enabled")
	}

	var apiKeyMgr *apikeys.Manager
	if cfg.APIKeys.Enabled {
		apiKeyMgr = apikeys.NewManager(sqlDB)
		logger.Info.Println("API key authentication enabled")
	}

	var jobQueue *jobs.Queue
	if cfg.Jobs.Enabled {
		jobQueue = jobs.NewQueue(sqlDB, jobs.Config{
			WorkerCount:  cfg.Jobs.WorkerCount,
			PollInterval: cfg.Jobs.PollInterval,
		})
		registerJobHandlers(jobQueue, auditLogger, resetMgr, webhookMgr)
		jobQueue.Start()
		logger.Info.Printf("Background job queue started with %d workers", cfg.Jobs.WorkerCount)
	}

	var tracer *tracing.Tracer
	if cfg.Tracing.Enabled {
		tracer = tracing.NewTracer(tracing.Config{
			ServiceName: cfg.Tracing.ServiceName,
			Endpoint:    cfg.Tracing.Endpoint,
			Exporter:    cfg.Tracing.Exporter,
			SampleRate:  cfg.Tracing.SampleRate,
		})
		logger.Info.Printf("Distributed tracing enabled (exporter: %s, sample rate: %.2f)", cfg.Tracing.Exporter, cfg.Tracing.SampleRate)
	}

	var geoLimiter *georatelimit.GeoRateLimiter
	if cfg.GeoRateLimit.Enabled {
		geoLimiter = georatelimit.NewGeoRateLimiter(sqlDB, georatelimit.Config{
			Enabled:          true,
			DefaultRPS:       cfg.GeoRateLimit.DefaultRPS,
			DefaultBurst:     cfg.GeoRateLimit.DefaultBurst,
			BlockedCountries: cfg.GeoRateLimit.BlockedCountries,
		})
		logger.Info.Printf("Geographic rate limiting enabled (default: %d rps, blocked countries: %v)", cfg.GeoRateLimit.DefaultRPS, cfg.GeoRateLimit.BlockedCountries)
	}

	var adminDashboard *dashboard.Dashboard
	if cfg.Dashboard.Enabled {
		adminDashboard = dashboard.New(sqlDB)
		logger.Info.Printf("Admin dashboard enabled at %s", cfg.Dashboard.Path)
	}

	var nftMgr *nft.Manager
	if cfg.NFT.Enabled {
		nftMgr = nft.NewManager(sqlDB, nil)
		logger.Info.Printf("NFT support enabled (marketplace: %v)", cfg.NFT.MarketplaceEnabled)
	}

	var jettonMgr *jetton.Manager
	if cfg.Jetton.Enabled {
		jettonMgr = jetton.NewManager(sqlDB, nil)
		logger.Info.Printf("Jetton (token) support enabled (minting: %v, burning: %v)", cfg.Jetton.AllowMinting, cfg.Jetton.AllowBurning)
	}

	var graphqlServer *graphql.Server
	if cfg.GraphQL.Enabled {
		graphqlServer = graphql.NewServer(sqlDB, cfg.GraphQL.Playground)
		logger.Info.Printf("GraphQL API enabled at %s (playground: %v)", cfg.GraphQL.Path, cfg.GraphQL.Playground)
	}

	// Create HTTP server
	srv := &api.Server{
		Secret:      []byte(cfg.JWTSecret),
		Store:       st,
		Provider:    provider,
		WSHub:       wsHub,
		WebhookMgr:  webhookMgr,
		UserMgr:     userMgr,
		TOTPMgr:     totpMgr,
		AuditLogger: auditLogger,
		ResetMgr:    resetMgr,
		RegMgr:      regMgr,
		ContractMgr: contractMgr,
		MultisigMgr: multisigMgr,
		BatchMgr:    batchMgr,
		APIKeyMgr:   apiKeyMgr,
		JobQueue:    jobQueue,
		NFTMgr:      nftMgr,
		JettonMgr:   jettonMgr,
	}

	mux := http.NewServeMux()
	srv.Register(mux)

	// Add metrics endpoint if enabled
	if cfg.Metrics.Enabled {
		mux.Handle("/metrics", promhttp.Handler())
		logger.Info.Println("Metrics endpoint enabled at /metrics")
	}

	// Add OpenAPI documentation if enabled
	if cfg.OpenAPI.Enabled {
		mux.HandleFunc(cfg.OpenAPI.Path, openapi.UIHandler("/api/openapi.json"))
		mux.HandleFunc("/api/openapi.json", openapi.Handler())
		logger.Info.Printf("OpenAPI documentation enabled at %s", cfg.OpenAPI.Path)
	}

	// Add admin dashboard if enabled
	if adminDashboard != nil {
		adminDashboard.Register(mux)
	}

	// Add GraphQL endpoint if enabled
	if graphqlServer != nil {
		mux.HandleFunc(cfg.GraphQL.Path, graphqlServer.Handler())
	}

	// Build middleware chain
	var handler http.Handler = mux

	// Apply IP filtering if enabled
	if ipFilter != nil {
		handler = ipFilter.Middleware(nil)(handler)
		logger.Debug.Println("IP filter middleware applied")
	}

	// Apply rate limiting if enabled
	if cfg.RateLimit.Enabled {
		rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.RPS, cfg.RateLimit.Burst)
		handler = middleware.RateLimitMiddleware(rateLimiter, middleware.IPKeyFunc)(handler)
		logger.Info.Printf("Rate limiting enabled: %d rps, burst %d", cfg.RateLimit.RPS, cfg.RateLimit.Burst)
	}

	// Apply geographic rate limiting if enabled
	if geoLimiter != nil {
		handler = geoLimiter.Middleware()(handler)
		logger.Debug.Println("Geographic rate limiting middleware applied")
	}

	// Apply distributed tracing if enabled
	if tracer != nil {
		handler = tracer.Middleware()(handler)
		logger.Debug.Println("Distributed tracing middleware applied")
	}

	httpSrv := &http.Server{
		Addr:         cfg.Bind,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start Telegram bot if enabled
	if cfg.Telegram.Enabled {
		b, err := tgbot.NewBotAPI(cfg.Telegram.Token)
		if err != nil {
			logger.Error.Fatalf("Failed to create Telegram bot: %v", err)
		}
		allow := map[int64]bool{}
		for _, id := range cfg.Telegram.AllowChats {
			allow[id] = true
		}
		logger.Info.Printf("Telegram bot enabled, allowing %d chats", len(allow))
		bot := &tg.Bot{API: b, AllowChats: allow, Store: st, Provider: provider}
		go func() {
			if err := bot.RunPoll(ctx); err != nil {
				logger.Error.Printf("Telegram bot error: %v", err)
			}
		}()
	} else {
		logger.Info.Println("Telegram bot disabled")
	}

	// Start periodic cleanup tasks
	if cfg.Audit.Enabled {
		go runPeriodicCleanup(ctx, auditLogger, cfg.Audit.RetentionDays)
	}

	// Start HTTP server
	go func() {
		logger.Info.Printf("HTTP server listening on %s", cfg.Bind)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Info.Println("Shutdown signal received, gracefully stopping...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop background services
	if jobQueue != nil {
		jobQueue.Stop()
		logger.Debug.Println("Job queue stopped")
	}

	if wsHub != nil {
		// WebSocket hub cleanup handled by its own goroutine
	}

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error.Printf("HTTP server shutdown error: %v", err)
	}

	if err := st.Close(); err != nil {
		logger.Error.Printf("Database close error: %v", err)
	}

	logger.Info.Println("tonkey stopped gracefully")
}

// contractProvider adapts ton.Provider for contracts.Manager
type contractProvider struct {
	provider ton.Provider
}

func (p *contractProvider) GetAccountState(ctx context.Context, addr string) (int64, string, error) {
	balance, err := p.provider.Balance(ctx, addr)
	if err != nil {
		return 0, "", err
	}
	return balance, "active", nil
}

func (p *contractProvider) RunGetMethod(ctx context.Context, addr, method string, params []any) ([]any, int64, int, error) {
	// Placeholder - would call lite client's RunGetMethod in real implementation
	return nil, 0, 0, nil
}

func (p *contractProvider) SendBOC(ctx context.Context, boc []byte) (string, error) {
	// Placeholder - would send BOC via lite client in real implementation
	return "", nil
}

// registerJobHandlers registers handlers for background jobs
func registerJobHandlers(q *jobs.Queue, auditLogger *audit.Logger, resetMgr *reset.Manager, webhookMgr *webhooks.Manager) {
	// Audit log cleanup job
	q.RegisterHandler("audit_cleanup", func(ctx context.Context, job *jobs.Job) error {
		if auditLogger != nil {
			deleted, err := auditLogger.Cleanup(90)
			if err != nil {
				return err
			}
			logger.Debug.Printf("Cleaned up %d old audit logs", deleted)
		}
		return nil
	})

	// Password reset token cleanup job
	q.RegisterHandler("reset_token_cleanup", func(ctx context.Context, job *jobs.Job) error {
		if resetMgr != nil {
			deleted, err := resetMgr.Cleanup()
			if err != nil {
				return err
			}
			logger.Debug.Printf("Cleaned up %d expired reset tokens", deleted)
		}
		return nil
	})

	// Webhook retry job
	q.RegisterHandler("webhook_retry", func(ctx context.Context, job *jobs.Job) error {
		// Would implement webhook retry logic here
		return nil
	})
}

// runPeriodicCleanup runs cleanup tasks periodically
func runPeriodicCleanup(ctx context.Context, auditLogger *audit.Logger, retentionDays int) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if auditLogger != nil {
				deleted, err := auditLogger.Cleanup(retentionDays)
				if err != nil {
					logger.Error.Printf("Audit cleanup error: %v", err)
				} else if deleted > 0 {
					logger.Info.Printf("Cleaned up %d old audit logs", deleted)
				}
			}
		}
	}
}
