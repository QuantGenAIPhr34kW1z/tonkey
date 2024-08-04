// Package apikeys provides API key authentication for service-to-service communication.
package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrKeyNotFound   = errors.New("API key not found")
	ErrKeyExpired    = errors.New("API key has expired")
	ErrKeyInactive   = errors.New("API key is inactive")
	ErrInvalidScope  = errors.New("API key does not have required scope")
	ErrRateLimited   = errors.New("API key rate limit exceeded")
)

const (
	// KeyLength is the length of API keys in bytes
	KeyLength = 32
	// PrefixLength is the length of the key prefix (for identification)
	PrefixLength = 8
)

// Key represents an API key.
type Key struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"user_id"`
	OrgID     *int64   `json:"org_id,omitempty"`
	KeyPrefix string   `json:"key_prefix"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	RateLimit *int     `json:"rate_limit,omitempty"`
	LastUsed  *int64   `json:"last_used,omitempty"`
	UseCount  int64    `json:"use_count"`
	ExpiresAt *int64   `json:"expires_at,omitempty"`
	IsActive  bool     `json:"is_active"`
	CreatedAt int64    `json:"created_at"`
}

// CreateKeyResult contains the result of creating an API key.
type CreateKeyResult struct {
	Key       Key    `json:"key"`
	PlainKey  string `json:"plain_key"` // Only returned once at creation
}

// Manager handles API key operations.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new API key manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Create creates a new API key.
func (m *Manager) Create(userID int64, orgID *int64, name string, scopes []string, rateLimit *int, expiresAt *int64) (*CreateKeyResult, error) {
	// Generate key
	keyBytes := make([]byte, KeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	plainKey := hex.EncodeToString(keyBytes)
	keyPrefix := plainKey[:PrefixLength]
	keyHash := hashKey(plainKey)

	now := time.Now().Unix()

	// Convert scopes to JSON
	scopesJSON := "[]"
	if len(scopes) > 0 {
		scopesJSON = fmt.Sprintf(`["%s"]`, strings.Join(scopes, `","`))
	}

	result, err := m.db.Exec(`
		INSERT INTO api_keys (user_id, org_id, key_hash, key_prefix, name, scopes, rate_limit, expires_at, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		userID, orgID, keyHash, keyPrefix, name, scopesJSON, rateLimit, expiresAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get key ID: %w", err)
	}

	key := Key{
		ID:        id,
		UserID:    userID,
		OrgID:     orgID,
		KeyPrefix: keyPrefix,
		Name:      name,
		Scopes:    scopes,
		RateLimit: rateLimit,
		ExpiresAt: expiresAt,
		IsActive:  true,
		CreatedAt: now,
	}

	return &CreateKeyResult{
		Key:      key,
		PlainKey: "tk_" + plainKey, // Prefix for easy identification
	}, nil
}

// Validate validates an API key and returns the associated user/org info.
func (m *Manager) Validate(apiKey string) (*Key, error) {
	// Remove prefix if present
	plainKey := strings.TrimPrefix(apiKey, "tk_")
	if len(plainKey) < PrefixLength*2 {
		return nil, ErrKeyNotFound
	}

	keyHash := hashKey(plainKey)

	var key Key
	var orgID sql.NullInt64
	var rateLimit sql.NullInt64
	var lastUsed, expiresAt sql.NullInt64
	var scopesJSON string

	err := m.db.QueryRow(`
		SELECT id, user_id, org_id, key_prefix, name, scopes, rate_limit, last_used, use_count, expires_at, is_active, created_at
		FROM api_keys
		WHERE key_hash = ?`,
		keyHash,
	).Scan(&key.ID, &key.UserID, &orgID, &key.KeyPrefix, &key.Name, &scopesJSON,
		&rateLimit, &lastUsed, &key.UseCount, &expiresAt, &key.IsActive, &key.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to validate key: %w", err)
	}

	if !key.IsActive {
		return nil, ErrKeyInactive
	}

	if expiresAt.Valid && time.Now().Unix() > expiresAt.Int64 {
		return nil, ErrKeyExpired
	}

	if orgID.Valid {
		key.OrgID = &orgID.Int64
	}
	if rateLimit.Valid {
		rl := int(rateLimit.Int64)
		key.RateLimit = &rl
	}
	if lastUsed.Valid {
		key.LastUsed = &lastUsed.Int64
	}
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Int64
	}

	// Parse scopes
	key.Scopes = parseScopes(scopesJSON)

	// Update usage stats
	now := time.Now().Unix()
	_, _ = m.db.Exec(
		"UPDATE api_keys SET last_used = ?, use_count = use_count + 1 WHERE id = ?",
		now, key.ID,
	)

	return &key, nil
}

// HasScope checks if a key has the required scope.
func (k *Key) HasScope(required string) bool {
	if len(k.Scopes) == 0 {
		return true // No scopes means all access
	}

	for _, scope := range k.Scopes {
		if scope == "*" || scope == required {
			return true
		}
		// Check wildcard patterns (e.g., "read:*" matches "read:wallet")
		if strings.HasSuffix(scope, ":*") {
			prefix := strings.TrimSuffix(scope, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

// List lists API keys for a user.
func (m *Manager) List(userID int64) ([]Key, error) {
	rows, err := m.db.Query(`
		SELECT id, user_id, org_id, key_prefix, name, scopes, rate_limit, last_used, use_count, expires_at, is_active, created_at
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	defer rows.Close()

	var keys []Key
	for rows.Next() {
		var key Key
		var orgID sql.NullInt64
		var rateLimit sql.NullInt64
		var lastUsed, expiresAt sql.NullInt64
		var scopesJSON string

		err := rows.Scan(&key.ID, &key.UserID, &orgID, &key.KeyPrefix, &key.Name, &scopesJSON,
			&rateLimit, &lastUsed, &key.UseCount, &expiresAt, &key.IsActive, &key.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}

		if orgID.Valid {
			key.OrgID = &orgID.Int64
		}
		if rateLimit.Valid {
			rl := int(rateLimit.Int64)
			key.RateLimit = &rl
		}
		if lastUsed.Valid {
			key.LastUsed = &lastUsed.Int64
		}
		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Int64
		}
		key.Scopes = parseScopes(scopesJSON)

		keys = append(keys, key)
	}

	return keys, nil
}

// Revoke revokes an API key.
func (m *Manager) Revoke(keyID, userID int64) error {
	result, err := m.db.Exec(
		"UPDATE api_keys SET is_active = 0 WHERE id = ? AND user_id = ?",
		keyID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// Delete permanently deletes an API key.
func (m *Manager) Delete(keyID, userID int64) error {
	result, err := m.db.Exec(
		"DELETE FROM api_keys WHERE id = ? AND user_id = ?",
		keyID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// Middleware returns an HTTP middleware for API key authentication.
func (m *Manager) Middleware(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				// Fall through to other auth methods
				next.ServeHTTP(w, r)
				return
			}

			key, err := m.Validate(apiKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			if requiredScope != "" && !key.HasScope(requiredScope) {
				http.Error(w, ErrInvalidScope.Error(), http.StatusForbidden)
				return
			}

			// Store key info in request context
			// (This would typically use context.WithValue)
			next.ServeHTTP(w, r)
		})
	}
}

// hashKey creates a SHA256 hash of an API key.
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// parseScopes parses a JSON scopes array.
func parseScopes(scopesJSON string) []string {
	if scopesJSON == "" || scopesJSON == "[]" {
		return nil
	}

	// Simple JSON array parsing
	scopesJSON = strings.Trim(scopesJSON, "[]")
	if scopesJSON == "" {
		return nil
	}

	parts := strings.Split(scopesJSON, ",")
	scopes := make([]string, 0, len(parts))
	for _, p := range parts {
		scope := strings.Trim(strings.TrimSpace(p), `"`)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// Cleanup removes expired and inactive keys older than the specified days.
func (m *Manager) Cleanup(days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	result, err := m.db.Exec(
		"DELETE FROM api_keys WHERE (expires_at IS NOT NULL AND expires_at < ?) OR (is_active = 0 AND created_at < ?)",
		time.Now().Unix(), cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup keys: %w", err)
	}
	return result.RowsAffected()
}
