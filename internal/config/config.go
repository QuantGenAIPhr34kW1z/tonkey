package config

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Bind      string `yaml:"bind"`
	JWTSecret string `yaml:"jwt_secret"`

	Database struct {
		Driver           string `yaml:"driver"`            // sqlite, postgres
		Path             string `yaml:"path"`              // for sqlite
		ConnectionString string `yaml:"connection_string"` // for postgres
		MaxConns         int    `yaml:"max_conns"`
		MaxIdleConns     int    `yaml:"max_idle_conns"`
	} `yaml:"database"`

	Provider struct {
		Kind         string `yaml:"kind"` // mock, toncenter, liteclient
		TonCenterURL string `yaml:"toncenter_url"`
		APIKey       string `yaml:"api_key"`
		// Lite client config
		LiteServers []LiteServerConfig `yaml:"lite_servers"`
		Timeout     time.Duration      `yaml:"timeout"`
	} `yaml:"provider"`

	RateLimit struct {
		Enabled bool `yaml:"enabled"`
		RPS     int  `yaml:"requests_per_second"`
		Burst   int  `yaml:"burst"`
	} `yaml:"rate_limit"`

	WebSocket struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"websocket"`

	Webhooks struct {
		Enabled    bool `yaml:"enabled"`
		Workers    int  `yaml:"workers"`
		MaxRetries int  `yaml:"max_retries"`
	} `yaml:"webhooks"`

	Metrics struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"metrics"`

	Telegram struct {
		Enabled    bool    `yaml:"enabled"`
		Token      string  `yaml:"token"`
		AllowChats []int64 `yaml:"allow_chats"`
		Poll       bool    `yaml:"poll"`
	} `yaml:"telegram"`

	Auth struct {
		AllowRegistration   bool     `yaml:"allow_registration"`
		RequireVerification bool     `yaml:"require_verification"`
		PasswordMinLength   int      `yaml:"password_min_length"`
		AllowedDomains      []string `yaml:"allowed_domains"`
		DefaultOrgID        *int64   `yaml:"default_org_id"`
	} `yaml:"auth"`

	TOTP struct {
		Enabled      bool   `yaml:"enabled"`
		Issuer       string `yaml:"issuer"`
		EnforceAdmin bool   `yaml:"enforce_admin"` // Require 2FA for admin users
	} `yaml:"totp"`

	Audit struct {
		Enabled       bool `yaml:"enabled"`
		RetentionDays int  `yaml:"retention_days"`
	} `yaml:"audit"`

	IPFilter struct {
		Enabled     bool   `yaml:"enabled"`
		DefaultMode string `yaml:"default_mode"` // allow, deny
	} `yaml:"ip_filter"`

	Contracts struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"contracts"`

	Multisig struct {
		Enabled            bool          `yaml:"enabled"`
		ProposalExpiration time.Duration `yaml:"proposal_expiration"`
	} `yaml:"multisig"`

	Batch struct {
		Enabled      bool `yaml:"enabled"`
		MaxBatchSize int  `yaml:"max_batch_size"`
	} `yaml:"batch"`

	APIKeys struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"api_keys"`

	Jobs struct {
		Enabled      bool          `yaml:"enabled"`
		WorkerCount  int           `yaml:"worker_count"`
		PollInterval time.Duration `yaml:"poll_interval"`
	} `yaml:"jobs"`

	Migrations struct {
		AutoMigrate bool `yaml:"auto_migrate"`
	} `yaml:"migrations"`

	OpenAPI struct {
		Enabled bool   `yaml:"enabled"`
		Path    string `yaml:"path"` // URL path for docs (default: /api/docs)
	} `yaml:"openapi"`

	Dashboard struct {
		Enabled bool   `yaml:"enabled"`
		Path    string `yaml:"path"` // URL path prefix (default: /admin/dashboard)
	} `yaml:"dashboard"`

	Tracing struct {
		Enabled     bool    `yaml:"enabled"`
		ServiceName string  `yaml:"service_name"`
		Endpoint    string  `yaml:"endpoint"`    // OTLP endpoint
		Exporter    string  `yaml:"exporter"`    // otlp, jaeger, zipkin, stdout
		SampleRate  float64 `yaml:"sample_rate"` // 0.0-1.0
	} `yaml:"tracing"`

	GeoRateLimit struct {
		Enabled          bool     `yaml:"enabled"`
		DefaultRPS       int      `yaml:"default_rps"`
		DefaultBurst     int      `yaml:"default_burst"`
		BlockedCountries []string `yaml:"blocked_countries"`
		GeoIPDatabase    string   `yaml:"geoip_database"` // Path to MaxMind GeoIP2 database
	} `yaml:"geo_rate_limit"`

	NFT struct {
		Enabled            bool    `yaml:"enabled"`
		MaxCollections     int     `yaml:"max_collections"`   // Per user/org
		MaxNFTsPerMint     int     `yaml:"max_nfts_per_mint"` // Batch minting limit
		DefaultRoyalty     float64 `yaml:"default_royalty"`   // Default royalty percentage
		MarketplaceEnabled bool    `yaml:"marketplace_enabled"`
	} `yaml:"nft"`

	Jetton struct {
		Enabled         bool `yaml:"enabled"`
		MaxTokens       int  `yaml:"max_tokens"`       // Per user/org
		DefaultDecimals int  `yaml:"default_decimals"` // Default 9 for TON
		AllowMinting    bool `yaml:"allow_minting"`    // Allow users to mint new tokens
		AllowBurning    bool `yaml:"allow_burning"`    // Allow users to burn tokens
	} `yaml:"jetton"`

	GraphQL struct {
		Enabled       bool   `yaml:"enabled"`
		Path          string `yaml:"path"`           // URL path (default: /graphql)
		Playground    bool   `yaml:"playground"`     // Enable GraphQL Playground
		MaxDepth      int    `yaml:"max_depth"`      // Query depth limit
		MaxComplexity int    `yaml:"max_complexity"` // Query complexity limit
	} `yaml:"graphql"`
}

// LiteServerConfig represents a TON lite server configuration.
type LiteServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	PublicKey string `yaml:"public_key"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, err
	}

	// Required settings
	if c.JWTSecret == "" {
		return nil, errors.New("jwt_secret must be set")
	}

	// Server defaults
	if c.Bind == "" {
		c.Bind = "0.0.0.0:8080"
	}

	// Database defaults
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.Path == "" && c.Database.Driver == "sqlite" {
		c.Database.Path = "./tonkey.db"
	}
	if c.Database.MaxConns == 0 {
		c.Database.MaxConns = 25
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}

	// Provider defaults
	if c.Provider.Kind == "" {
		c.Provider.Kind = "mock"
	}
	if c.Provider.Timeout == 0 {
		c.Provider.Timeout = 10 * time.Second
	}

	// Rate limit defaults
	if c.RateLimit.Enabled && c.RateLimit.RPS == 0 {
		c.RateLimit.RPS = 10
	}
	if c.RateLimit.Enabled && c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 20
	}

	// Webhook defaults
	if c.Webhooks.Workers == 0 {
		c.Webhooks.Workers = 4
	}
	if c.Webhooks.MaxRetries == 0 {
		c.Webhooks.MaxRetries = 5
	}

	// Auth defaults
	if c.Auth.PasswordMinLength == 0 {
		c.Auth.PasswordMinLength = 8
	}

	// TOTP defaults
	if c.TOTP.Issuer == "" {
		c.TOTP.Issuer = "tonkey"
	}

	// Audit defaults
	if c.Audit.RetentionDays == 0 {
		c.Audit.RetentionDays = 90
	}

	// IP filter defaults
	if c.IPFilter.DefaultMode == "" {
		c.IPFilter.DefaultMode = "allow"
	}

	// Multisig defaults
	if c.Multisig.ProposalExpiration == 0 {
		c.Multisig.ProposalExpiration = 7 * 24 * time.Hour
	}

	// Batch defaults
	if c.Batch.MaxBatchSize == 0 {
		c.Batch.MaxBatchSize = 100
	}

	// Jobs defaults
	if c.Jobs.WorkerCount == 0 {
		c.Jobs.WorkerCount = 4
	}
	if c.Jobs.PollInterval == 0 {
		c.Jobs.PollInterval = 5 * time.Second
	}

	// OpenAPI defaults
	if c.OpenAPI.Path == "" {
		c.OpenAPI.Path = "/api/docs"
	}

	// Dashboard defaults
	if c.Dashboard.Path == "" {
		c.Dashboard.Path = "/admin/dashboard"
	}

	// Tracing defaults
	if c.Tracing.ServiceName == "" {
		c.Tracing.ServiceName = "tonkey"
	}
	if c.Tracing.Exporter == "" {
		c.Tracing.Exporter = "stdout"
	}
	if c.Tracing.SampleRate == 0 {
		c.Tracing.SampleRate = 1.0
	}

	// GeoRateLimit defaults
	if c.GeoRateLimit.DefaultRPS == 0 {
		c.GeoRateLimit.DefaultRPS = 100
	}
	if c.GeoRateLimit.DefaultBurst == 0 {
		c.GeoRateLimit.DefaultBurst = 200
	}

	// NFT defaults
	if c.NFT.MaxCollections == 0 {
		c.NFT.MaxCollections = 100
	}
	if c.NFT.MaxNFTsPerMint == 0 {
		c.NFT.MaxNFTsPerMint = 100
	}
	if c.NFT.DefaultRoyalty == 0 {
		c.NFT.DefaultRoyalty = 5.0 // 5%
	}

	// Jetton defaults
	if c.Jetton.MaxTokens == 0 {
		c.Jetton.MaxTokens = 50
	}
	if c.Jetton.DefaultDecimals == 0 {
		c.Jetton.DefaultDecimals = 9
	}

	// GraphQL defaults
	if c.GraphQL.Path == "" {
		c.GraphQL.Path = "/graphql"
	}
	if c.GraphQL.MaxDepth == 0 {
		c.GraphQL.MaxDepth = 10
	}
	if c.GraphQL.MaxComplexity == 0 {
		c.GraphQL.MaxComplexity = 100
	}

	return &c, nil
}

// Validate performs comprehensive validation of the configuration.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return errors.New("jwt_secret is required")
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("jwt_secret should be at least 32 characters")
	}

	if c.Database.Driver != "sqlite" && c.Database.Driver != "postgres" {
		return errors.New("database.driver must be 'sqlite' or 'postgres'")
	}

	if c.Database.Driver == "postgres" && c.Database.ConnectionString == "" {
		return errors.New("database.connection_string is required for postgres")
	}

	if c.Provider.Kind != "mock" && c.Provider.Kind != "toncenter" && c.Provider.Kind != "liteclient" {
		return errors.New("provider.kind must be 'mock', 'toncenter', or 'liteclient'")
	}

	if c.Provider.Kind == "toncenter" && c.Provider.TonCenterURL == "" {
		return errors.New("provider.toncenter_url is required for toncenter provider")
	}

	if c.Provider.Kind == "liteclient" && len(c.Provider.LiteServers) == 0 {
		return errors.New("provider.lite_servers is required for liteclient provider")
	}

	if c.Telegram.Enabled && c.Telegram.Token == "" {
		return errors.New("telegram.token is required when telegram is enabled")
	}

	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Provider.Kind == "mock"
}
