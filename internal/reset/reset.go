// Package reset provides password reset functionality.
package reset

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrTokenNotFound = errors.New("reset token not found")
	ErrTokenExpired  = errors.New("reset token has expired")
	ErrTokenUsed     = errors.New("reset token has already been used")
	ErrUserNotFound  = errors.New("user not found")
	ErrRateLimited   = errors.New("too many reset requests, please try again later")
)

const (
	// TokenLength is the length of reset tokens in bytes (32 bytes = 64 hex chars)
	TokenLength = 32
	// DefaultExpiration is the default token expiration time
	DefaultExpiration = 1 * time.Hour
	// MinRequestInterval is the minimum time between reset requests for the same user
	MinRequestInterval = 5 * time.Minute
)

// Manager handles password reset operations.
type Manager struct {
	db         *sql.DB
	expiration time.Duration
}

// NewManager creates a new password reset manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:         db,
		expiration: DefaultExpiration,
	}
}

// SetExpiration sets the token expiration duration.
func (m *Manager) SetExpiration(d time.Duration) {
	m.expiration = d
}

// ResetRequest represents a password reset request.
type ResetRequest struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// CreateToken generates a password reset token for a user.
func (m *Manager) CreateToken(email string) (*ResetRequest, error) {
	// Find user by email
	var userID int64
	err := m.db.QueryRow("SELECT id FROM users WHERE email = ? AND is_active = 1", email).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't reveal if user exists
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Check rate limiting
	var lastRequest sql.NullInt64
	err = m.db.QueryRow(
		"SELECT MAX(created_at) FROM password_reset_tokens WHERE user_id = ?",
		userID,
	).Scan(&lastRequest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	if lastRequest.Valid {
		lastTime := time.Unix(lastRequest.Int64, 0)
		if time.Since(lastTime) < MinRequestInterval {
			return nil, ErrRateLimited
		}
	}

	// Generate token
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Hash token for storage
	tokenHash := hashToken(token)

	now := time.Now()
	expiresAt := now.Add(m.expiration).Unix()

	// Store token
	_, err = m.db.Exec(`
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?)`,
		userID, tokenHash, expiresAt, now.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store reset token: %w", err)
	}

	return &ResetRequest{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateToken validates a password reset token.
func (m *Manager) ValidateToken(token string) (int64, error) {
	tokenHash := hashToken(token)

	var userID int64
	var expiresAt int64
	var usedAt sql.NullInt64

	err := m.db.QueryRow(`
		SELECT user_id, expires_at, used_at
		FROM password_reset_tokens
		WHERE token_hash = ?`,
		tokenHash,
	).Scan(&userID, &expiresAt, &usedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTokenNotFound
		}
		return 0, fmt.Errorf("failed to validate token: %w", err)
	}

	if usedAt.Valid {
		return 0, ErrTokenUsed
	}

	if time.Now().Unix() > expiresAt {
		return 0, ErrTokenExpired
	}

	return userID, nil
}

// ResetPassword resets a user's password using a valid token.
func (m *Manager) ResetPassword(token, newPassword string) error {
	userID, err := m.ValidateToken(token)
	if err != nil {
		return err
	}

	// Hash the new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update password
	_, err = tx.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark token as used
	tokenHash := hashToken(token)
	_, err = tx.Exec(
		"UPDATE password_reset_tokens SET used_at = ? WHERE token_hash = ?",
		time.Now().Unix(), tokenHash,
	)
	if err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	// Invalidate all other tokens for this user
	_, err = tx.Exec(
		"UPDATE password_reset_tokens SET used_at = ? WHERE user_id = ? AND used_at IS NULL",
		time.Now().Unix(), userID,
	)
	if err != nil {
		return fmt.Errorf("failed to invalidate other tokens: %w", err)
	}

	return tx.Commit()
}

// Cleanup removes expired tokens.
func (m *Manager) Cleanup() (int64, error) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	result, err := m.db.Exec(
		"DELETE FROM password_reset_tokens WHERE expires_at < ? OR used_at IS NOT NULL",
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup tokens: %w", err)
	}
	return result.RowsAffected()
}

// hashToken creates a SHA256 hash of the token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// GetUserByResetToken returns user info for a valid reset token.
func (m *Manager) GetUserByResetToken(token string) (int64, string, error) {
	userID, err := m.ValidateToken(token)
	if err != nil {
		return 0, "", err
	}

	var username string
	err = m.db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get user: %w", err)
	}

	return userID, username, nil
}
