// Package ipfilter provides IP-based access control (allowlist/blocklist).
package ipfilter

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrBlocked     = errors.New("IP address is blocked")
	ErrNotAllowed  = errors.New("IP address not in allowlist")
	ErrInvalidCIDR = errors.New("invalid CIDR notation")
)

// RuleType defines the type of IP rule.
type RuleType string

const (
	RuleTypeAllow RuleType = "allow"
	RuleTypeBlock RuleType = "block"
)

// Rule represents an IP filtering rule.
type Rule struct {
	ID          int64    `json:"id"`
	OrgID       *int64   `json:"org_id,omitempty"`
	CIDR        string   `json:"cidr"`
	RuleType    RuleType `json:"rule_type"`
	Description string   `json:"description,omitempty"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   int64    `json:"created_at"`
	ExpiresAt   *int64   `json:"expires_at,omitempty"`
}

// Filter manages IP-based access control.
type Filter struct {
	db          *sql.DB
	enabled     bool
	defaultMode string // "allow" or "deny" - default behavior when no rules match
	mu          sync.RWMutex
	cache       map[string][]Rule // org_id -> rules cache
	cacheTime   time.Time
	cacheTTL    time.Duration
}

// NewFilter creates a new IP filter.
func NewFilter(db *sql.DB, enabled bool, defaultMode string) *Filter {
	if defaultMode == "" {
		defaultMode = "allow" // Default to allow if no rules match
	}
	return &Filter{
		db:          db,
		enabled:     enabled,
		defaultMode: defaultMode,
		cache:       make(map[string][]Rule),
		cacheTTL:    5 * time.Minute,
	}
}

// Check checks if an IP address is allowed.
func (f *Filter) Check(ip string, orgID *int64) error {
	if !f.enabled {
		return nil
	}

	// Parse IP address
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		// Try to extract from IP:port format
		host, _, err := net.SplitHostPort(ip)
		if err != nil {
			return nil // Allow if we can't parse
		}
		parsedIP = net.ParseIP(host)
		if parsedIP == nil {
			return nil
		}
	}

	// Get rules (from cache or database)
	rules, err := f.getRules(orgID)
	if err != nil {
		return nil // Fail open on database errors
	}

	// Check block rules first
	for _, rule := range rules {
		if rule.RuleType != RuleTypeBlock {
			continue
		}
		if f.matchesRule(parsedIP, rule) {
			return ErrBlocked
		}
	}

	// Check allow rules
	hasAllowRules := false
	for _, rule := range rules {
		if rule.RuleType != RuleTypeAllow {
			continue
		}
		hasAllowRules = true
		if f.matchesRule(parsedIP, rule) {
			return nil // Allowed
		}
	}

	// If there are allow rules but none matched, deny
	if hasAllowRules {
		return ErrNotAllowed
	}

	// No rules matched, use default mode
	if f.defaultMode == "deny" {
		return ErrNotAllowed
	}

	return nil
}

// matchesRule checks if an IP matches a rule.
func (f *Filter) matchesRule(ip net.IP, rule Rule) bool {
	if !rule.IsActive {
		return false
	}

	// Check expiration
	if rule.ExpiresAt != nil && time.Now().Unix() > *rule.ExpiresAt {
		return false
	}

	// Parse CIDR
	_, network, err := net.ParseCIDR(rule.CIDR)
	if err != nil {
		// Try as single IP
		ruleIP := net.ParseIP(rule.CIDR)
		if ruleIP == nil {
			return false
		}
		return ip.Equal(ruleIP)
	}

	return network.Contains(ip)
}

// getRules retrieves rules for an organization (with caching).
func (f *Filter) getRules(orgID *int64) ([]Rule, error) {
	f.mu.RLock()
	cacheKey := "global"
	if orgID != nil {
		cacheKey = fmt.Sprintf("org_%d", *orgID)
	}

	if time.Since(f.cacheTime) < f.cacheTTL {
		if rules, ok := f.cache[cacheKey]; ok {
			f.mu.RUnlock()
			return rules, nil
		}
	}
	f.mu.RUnlock()

	// Load from database
	rules, err := f.loadRules(orgID)
	if err != nil {
		return nil, err
	}

	// Update cache
	f.mu.Lock()
	f.cache[cacheKey] = rules
	f.cacheTime = time.Now()
	f.mu.Unlock()

	return rules, nil
}

// loadRules loads rules from the database.
func (f *Filter) loadRules(orgID *int64) ([]Rule, error) {
	query := `
		SELECT id, org_id, cidr, rule_type, description, is_active, created_at, expires_at
		FROM ip_rules
		WHERE is_active = 1 AND (org_id IS NULL`

	args := []any{}
	if orgID != nil {
		query += " OR org_id = ?"
		args = append(args, *orgID)
	}
	query += ") ORDER BY rule_type DESC, created_at ASC" // Block rules first

	rows, err := f.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query IP rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		var orgIDVal sql.NullInt64
		var description sql.NullString
		var expiresAt sql.NullInt64

		err := rows.Scan(&r.ID, &orgIDVal, &r.CIDR, &r.RuleType, &description, &r.IsActive, &r.CreatedAt, &expiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan IP rule: %w", err)
		}

		if orgIDVal.Valid {
			r.OrgID = &orgIDVal.Int64
		}
		if description.Valid {
			r.Description = description.String
		}
		if expiresAt.Valid {
			r.ExpiresAt = &expiresAt.Int64
		}

		rules = append(rules, r)
	}

	return rules, nil
}

// AddRule adds a new IP rule.
func (f *Filter) AddRule(rule Rule) (int64, error) {
	// Validate CIDR
	if _, _, err := net.ParseCIDR(rule.CIDR); err != nil {
		// Try as single IP
		if ip := net.ParseIP(rule.CIDR); ip == nil {
			return 0, ErrInvalidCIDR
		}
	}

	result, err := f.db.Exec(`
		INSERT INTO ip_rules (org_id, cidr, rule_type, description, is_active, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rule.OrgID, rule.CIDR, rule.RuleType, rule.Description, true, time.Now().Unix(), rule.ExpiresAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add IP rule: %w", err)
	}

	// Invalidate cache
	f.mu.Lock()
	f.cache = make(map[string][]Rule)
	f.mu.Unlock()

	return result.LastInsertId()
}

// RemoveRule removes an IP rule.
func (f *Filter) RemoveRule(id int64) error {
	_, err := f.db.Exec("DELETE FROM ip_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove IP rule: %w", err)
	}

	// Invalidate cache
	f.mu.Lock()
	f.cache = make(map[string][]Rule)
	f.mu.Unlock()

	return nil
}

// ListRules lists all rules for an organization.
func (f *Filter) ListRules(orgID *int64) ([]Rule, error) {
	return f.loadRules(orgID)
}

// Middleware returns an HTTP middleware for IP filtering.
func (f *Filter) Middleware(getOrgID func(*http.Request) *int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)
			var orgID *int64
			if getOrgID != nil {
				orgID = getOrgID(r)
			}

			if err := f.Check(ip, orgID); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetClientIP extracts the client IP from an HTTP request.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// InvalidateCache clears the rule cache.
func (f *Filter) InvalidateCache() {
	f.mu.Lock()
	f.cache = make(map[string][]Rule)
	f.mu.Unlock()
}
