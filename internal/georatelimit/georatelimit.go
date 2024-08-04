// Package georatelimit provides geographic-based rate limiting.
package georatelimit

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"tonkey/internal/logger"
)

// Config holds geographic rate limiting configuration.
type Config struct {
	Enabled          bool              `yaml:"enabled"`
	DefaultRPS       int               `yaml:"default_rps"`
	DefaultBurst     int               `yaml:"default_burst"`
	BlockedCountries []string          `yaml:"blocked_countries"`
	CountryLimits    map[string]Limits `yaml:"country_limits"`
	GeoIPDatabase    string            `yaml:"geoip_database"` // Path to MaxMind GeoLite2 database
}

// Limits defines rate limits.
type Limits struct {
	RPS   int `yaml:"rps"`
	Burst int `yaml:"burst"`
}

// GeoRateLimiter provides geographic-aware rate limiting.
type GeoRateLimiter struct {
	db               *sql.DB
	config           Config
	mu               sync.RWMutex
	limiters         map[string]*rateLimiter
	geoCache         map[string]*geoInfo
	geoCacheMu       sync.RWMutex
	blockedCountries map[string]bool
}

type geoInfo struct {
	CountryCode string
	Country     string
	City        string
	Latitude    float64
	Longitude   float64
	CachedAt    time.Time
}

type rateLimiter struct {
	tokens     float64
	lastUpdate time.Time
	rps        float64
	burst      float64
	mu         sync.Mutex
}

// GeoIPLookup defines the interface for GeoIP lookups.
type GeoIPLookup interface {
	Lookup(ip string) (*geoInfo, error)
}

// New creates a new GeoRateLimiter.
func New(db *sql.DB, config Config) *GeoRateLimiter {
	if config.DefaultRPS <= 0 {
		config.DefaultRPS = 100
	}
	if config.DefaultBurst <= 0 {
		config.DefaultBurst = 200
	}

	blocked := make(map[string]bool)
	for _, code := range config.BlockedCountries {
		blocked[strings.ToUpper(code)] = true
	}

	limiter := &GeoRateLimiter{
		db:               db,
		config:           config,
		limiters:         make(map[string]*rateLimiter),
		geoCache:         make(map[string]*geoInfo),
		blockedCountries: blocked,
	}

	// Initialize blocked countries table if db is provided
	if db != nil {
		limiter.initDB()
	}

	// Start cleanup goroutine
	go limiter.cleanupLoop()

	logger.Info.Printf("Geographic rate limiter initialized: default_rps=%d, blocked_countries=%d",
		config.DefaultRPS, len(config.BlockedCountries))

	return limiter
}

func (g *GeoRateLimiter) initDB() {
	_, err := g.db.Exec(`
		CREATE TABLE IF NOT EXISTS geo_rate_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			country_code TEXT NOT NULL,
			rps INTEGER NOT NULL,
			burst INTEGER NOT NULL,
			is_blocked BOOLEAN DEFAULT 0,
			description TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_geo_rate_rules_country ON geo_rate_rules(country_code);
	`)
	if err != nil {
		logger.Error.Printf("Failed to create geo_rate_rules table: %v", err)
	}
}

// Middleware returns HTTP middleware for geographic rate limiting.
func (g *GeoRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !g.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			ip := g.getClientIP(r)
			geo := g.lookupGeo(ip)

			// Check if country is blocked
			if geo != nil && g.isBlocked(geo.CountryCode) {
				logger.Warn.Printf("Blocked request from %s (country: %s)", ip, geo.CountryCode)
				http.Error(w, "Access denied from your region", http.StatusForbidden)
				return
			}

			// Get rate limits for this country
			limits := g.getLimits(geo)

			// Check rate limit
			key := g.getRateLimitKey(ip, geo)
			if !g.allow(key, limits.RPS, limits.Burst) {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Limit", string(rune(limits.RPS)))
				w.Header().Set("X-RateLimit-Remaining", "0")
				if geo != nil {
					w.Header().Set("X-Geo-Country", geo.CountryCode)
				}
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Add geo info to response headers (optional)
			if geo != nil {
				w.Header().Set("X-Geo-Country", geo.CountryCode)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (g *GeoRateLimiter) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (g *GeoRateLimiter) lookupGeo(ip string) *geoInfo {
	// Check cache first
	g.geoCacheMu.RLock()
	if cached, ok := g.geoCache[ip]; ok {
		if time.Since(cached.CachedAt) < 24*time.Hour {
			g.geoCacheMu.RUnlock()
			return cached
		}
	}
	g.geoCacheMu.RUnlock()

	// Perform lookup (simplified - in production, use MaxMind GeoLite2)
	geo := g.simpleLookup(ip)
	if geo == nil {
		return nil
	}

	// Cache result
	geo.CachedAt = time.Now()
	g.geoCacheMu.Lock()
	g.geoCache[ip] = geo
	g.geoCacheMu.Unlock()

	return geo
}

// simpleLookup provides a basic IP to country lookup.
// In production, replace with MaxMind GeoLite2 or similar.
func (g *GeoRateLimiter) simpleLookup(ip string) *geoInfo {
	// Parse IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}

	// Check if private IP
	if parsedIP.IsPrivate() || parsedIP.IsLoopback() {
		return &geoInfo{
			CountryCode: "LOCAL",
			Country:     "Local Network",
		}
	}

	// Simple heuristic based on first octet (not accurate - for demo only)
	// In production, use a proper GeoIP database
	if parsedIP.To4() != nil {
		firstOctet := parsedIP.To4()[0]
		switch {
		case firstOctet >= 1 && firstOctet <= 126:
			return &geoInfo{CountryCode: "US", Country: "United States"}
		case firstOctet >= 128 && firstOctet <= 191:
			return &geoInfo{CountryCode: "EU", Country: "Europe"}
		case firstOctet >= 192 && firstOctet <= 223:
			return &geoInfo{CountryCode: "APAC", Country: "Asia Pacific"}
		}
	}

	return &geoInfo{CountryCode: "UNKNOWN", Country: "Unknown"}
}

func (g *GeoRateLimiter) isBlocked(countryCode string) bool {
	if countryCode == "" {
		return false
	}

	// Check in-memory blocklist
	if g.blockedCountries[strings.ToUpper(countryCode)] {
		return true
	}

	// Check database if available
	if g.db != nil {
		var isBlocked bool
		err := g.db.QueryRow(
			"SELECT is_blocked FROM geo_rate_rules WHERE country_code = ?",
			strings.ToUpper(countryCode),
		).Scan(&isBlocked)
		if err == nil {
			return isBlocked
		}
	}

	return false
}

func (g *GeoRateLimiter) getLimits(geo *geoInfo) Limits {
	if geo == nil {
		return Limits{RPS: g.config.DefaultRPS, Burst: g.config.DefaultBurst}
	}

	code := strings.ToUpper(geo.CountryCode)

	// Check config for country-specific limits
	if limits, ok := g.config.CountryLimits[code]; ok {
		return limits
	}

	// Check database for country-specific limits
	if g.db != nil {
		var rps, burst int
		err := g.db.QueryRow(
			"SELECT rps, burst FROM geo_rate_rules WHERE country_code = ? AND is_blocked = 0",
			code,
		).Scan(&rps, &burst)
		if err == nil && rps > 0 {
			return Limits{RPS: rps, Burst: burst}
		}
	}

	return Limits{RPS: g.config.DefaultRPS, Burst: g.config.DefaultBurst}
}

func (g *GeoRateLimiter) getRateLimitKey(ip string, geo *geoInfo) string {
	// Rate limit by IP
	return ip
}

func (g *GeoRateLimiter) allow(key string, rps, burst int) bool {
	g.mu.Lock()
	limiter, ok := g.limiters[key]
	if !ok {
		limiter = &rateLimiter{
			tokens:     float64(burst),
			lastUpdate: time.Now(),
			rps:        float64(rps),
			burst:      float64(burst),
		}
		g.limiters[key] = limiter
	}
	g.mu.Unlock()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(limiter.lastUpdate).Seconds()
	limiter.tokens += elapsed * limiter.rps
	if limiter.tokens > limiter.burst {
		limiter.tokens = limiter.burst
	}
	limiter.lastUpdate = now

	if limiter.tokens < 1 {
		return false
	}

	limiter.tokens--
	return true
}

func (g *GeoRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		g.cleanup()
	}
}

func (g *GeoRateLimiter) cleanup() {
	g.mu.Lock()
	defer g.mu.Unlock()

	threshold := time.Now().Add(-10 * time.Minute)
	for key, limiter := range g.limiters {
		limiter.mu.Lock()
		if limiter.lastUpdate.Before(threshold) {
			delete(g.limiters, key)
		}
		limiter.mu.Unlock()
	}

	// Cleanup geo cache
	g.geoCacheMu.Lock()
	for ip, geo := range g.geoCache {
		if time.Since(geo.CachedAt) > 24*time.Hour {
			delete(g.geoCache, ip)
		}
	}
	g.geoCacheMu.Unlock()
}

// BlockCountry adds a country to the blocklist.
func (g *GeoRateLimiter) BlockCountry(countryCode, description string) error {
	code := strings.ToUpper(countryCode)
	g.blockedCountries[code] = true

	if g.db != nil {
		now := time.Now().Unix()
		_, err := g.db.Exec(`
			INSERT INTO geo_rate_rules (country_code, rps, burst, is_blocked, description, created_at, updated_at)
			VALUES (?, 0, 0, 1, ?, ?, ?)
			ON CONFLICT(country_code) DO UPDATE SET is_blocked = 1, description = ?, updated_at = ?
		`, code, description, now, now, description, now)
		return err
	}
	return nil
}

// UnblockCountry removes a country from the blocklist.
func (g *GeoRateLimiter) UnblockCountry(countryCode string) error {
	code := strings.ToUpper(countryCode)
	delete(g.blockedCountries, code)

	if g.db != nil {
		_, err := g.db.Exec(
			"UPDATE geo_rate_rules SET is_blocked = 0, updated_at = ? WHERE country_code = ?",
			time.Now().Unix(), code,
		)
		return err
	}
	return nil
}

// SetCountryLimits sets custom rate limits for a country.
func (g *GeoRateLimiter) SetCountryLimits(countryCode string, limits Limits) error {
	code := strings.ToUpper(countryCode)

	if g.db != nil {
		now := time.Now().Unix()
		_, err := g.db.Exec(`
			INSERT INTO geo_rate_rules (country_code, rps, burst, is_blocked, created_at, updated_at)
			VALUES (?, ?, ?, 0, ?, ?)
			ON CONFLICT(country_code) DO UPDATE SET rps = ?, burst = ?, updated_at = ?
		`, code, limits.RPS, limits.Burst, now, now, limits.RPS, limits.Burst, now)
		return err
	}
	return nil
}

// GetStats returns current rate limiter statistics.
func (g *GeoRateLimiter) GetStats() map[string]any {
	g.mu.RLock()
	limiterCount := len(g.limiters)
	g.mu.RUnlock()

	g.geoCacheMu.RLock()
	cacheCount := len(g.geoCache)
	g.geoCacheMu.RUnlock()

	return map[string]any{
		"active_limiters":   limiterCount,
		"geo_cache_entries": cacheCount,
		"blocked_countries": len(g.blockedCountries),
		"default_rps":       g.config.DefaultRPS,
		"default_burst":     g.config.DefaultBurst,
	}
}

// GetBlockedCountries returns the list of blocked countries.
func (g *GeoRateLimiter) GetBlockedCountries() []string {
	countries := make([]string, 0, len(g.blockedCountries))
	for code := range g.blockedCountries {
		countries = append(countries, code)
	}

	// Also get from database
	if g.db != nil {
		rows, err := g.db.Query("SELECT country_code FROM geo_rate_rules WHERE is_blocked = 1")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code string
				if rows.Scan(&code) == nil {
					// Avoid duplicates
					found := false
					for _, c := range countries {
						if c == code {
							found = true
							break
						}
					}
					if !found {
						countries = append(countries, code)
					}
				}
			}
		}
	}

	return countries
}

// GeoInfo represents geographic information about an IP.
type GeoInfo struct {
	IP          string  `json:"ip"`
	CountryCode string  `json:"country_code"`
	Country     string  `json:"country"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
}

// LookupIP returns geographic information for an IP address.
func (g *GeoRateLimiter) LookupIP(ip string) *GeoInfo {
	geo := g.lookupGeo(ip)
	if geo == nil {
		return nil
	}
	return &GeoInfo{
		IP:          ip,
		CountryCode: geo.CountryCode,
		Country:     geo.Country,
		City:        geo.City,
		Latitude:    geo.Latitude,
		Longitude:   geo.Longitude,
	}
}

// Handler returns an HTTP handler for geo rate limit management.
func (g *GeoRateLimiter) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Return stats and blocked countries
			stats := g.GetStats()
			stats["blocked_countries_list"] = g.GetBlockedCountries()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)

		case "POST":
			// Block/unblock country or set limits
			var req struct {
				Action      string `json:"action"` // "block", "unblock", "set_limits"
				CountryCode string `json:"country_code"`
				RPS         int    `json:"rps"`
				Burst       int    `json:"burst"`
				Description string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}

			var err error
			switch req.Action {
			case "block":
				err = g.BlockCountry(req.CountryCode, req.Description)
			case "unblock":
				err = g.UnblockCountry(req.CountryCode)
			case "set_limits":
				err = g.SetCountryLimits(req.CountryCode, Limits{RPS: req.RPS, Burst: req.Burst})
			default:
				http.Error(w, "unknown action", http.StatusBadRequest)
				return
			}

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"success": true})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
