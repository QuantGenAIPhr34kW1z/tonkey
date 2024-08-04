// Package registration provides user self-registration functionality.
package registration

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrRegistrationDisabled = errors.New("user registration is disabled")
	ErrUserExists           = errors.New("username already exists")
	ErrEmailExists          = errors.New("email already registered")
	ErrInvalidUsername      = errors.New("invalid username format")
	ErrInvalidEmail         = errors.New("invalid email format")
	ErrInvalidPassword      = errors.New("password does not meet requirements")
	ErrTokenNotFound        = errors.New("verification token not found")
	ErrTokenExpired         = errors.New("verification token has expired")
	ErrAlreadyVerified      = errors.New("email already verified")
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

const (
	// TokenLength for email verification
	TokenLength = 32
	// TokenExpiration is the verification token expiration time
	TokenExpiration = 24 * time.Hour
)

// Config holds registration configuration.
type Config struct {
	Enabled            bool
	RequireEmail       bool
	RequireVerification bool
	DefaultOrgID       *int64
	DefaultRole        string
	PasswordMinLength  int
	AllowedDomains     []string // Empty means all domains allowed
}

// Manager handles user registration.
type Manager struct {
	db     *sql.DB
	config Config
}

// NewManager creates a new registration manager.
func NewManager(db *sql.DB, config Config) *Manager {
	if config.DefaultRole == "" {
		config.DefaultRole = "user"
	}
	if config.PasswordMinLength < 8 {
		config.PasswordMinLength = 8
	}
	return &Manager{
		db:     db,
		config: config,
	}
}

// RegistrationRequest represents a registration request.
type RegistrationRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// RegistrationResult contains the registration outcome.
type RegistrationResult struct {
	UserID            int64  `json:"user_id"`
	Username          string `json:"username"`
	VerificationToken string `json:"verification_token,omitempty"`
	RequiresVerification bool `json:"requires_verification"`
}

// Register registers a new user.
func (m *Manager) Register(req RegistrationRequest) (*RegistrationResult, error) {
	if !m.config.Enabled {
		return nil, ErrRegistrationDisabled
	}

	// Validate input
	if err := m.validateUsername(req.Username); err != nil {
		return nil, err
	}
	if err := m.validatePassword(req.Password); err != nil {
		return nil, err
	}
	if m.config.RequireEmail || req.Email != "" {
		if err := m.validateEmail(req.Email); err != nil {
			return nil, err
		}
	}

	// Check if username exists
	var exists int
	err := m.db.QueryRow("SELECT 1 FROM users WHERE username = ?", req.Username).Scan(&exists)
	if err == nil {
		return nil, ErrUserExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}

	// Check if email exists
	if req.Email != "" {
		err = m.db.QueryRow("SELECT 1 FROM users WHERE email = ?", req.Email).Scan(&exists)
		if err == nil {
			return nil, ErrEmailExists
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to check email: %w", err)
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().Unix()
	isActive := !m.config.RequireVerification

	// Insert user
	result, err := m.db.Exec(`
		INSERT INTO users (username, password_hash, email, org_id, role, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.Username, string(hash), nullString(req.Email), m.config.DefaultOrgID,
		m.config.DefaultRole, isActive, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	res := &RegistrationResult{
		UserID:              userID,
		Username:            req.Username,
		RequiresVerification: m.config.RequireVerification,
	}

	// Generate verification token if needed
	if m.config.RequireVerification && req.Email != "" {
		token, err := m.createVerificationToken(userID, req.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to create verification token: %w", err)
		}
		res.VerificationToken = token
	}

	return res, nil
}

// VerifyEmail verifies a user's email using a token.
func (m *Manager) VerifyEmail(token string) error {
	tokenHash := hashToken(token)

	var id, userID int64
	var expiresAt int64
	var verifiedAt sql.NullInt64

	err := m.db.QueryRow(`
		SELECT id, user_id, expires_at, verified_at
		FROM email_verifications
		WHERE token_hash = ?`,
		tokenHash,
	).Scan(&id, &userID, &expiresAt, &verifiedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("failed to find token: %w", err)
	}

	if verifiedAt.Valid {
		return ErrAlreadyVerified
	}

	if time.Now().Unix() > expiresAt {
		return ErrTokenExpired
	}

	now := time.Now().Unix()

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Mark token as verified
	_, err = tx.Exec("UPDATE email_verifications SET verified_at = ? WHERE id = ?", now, id)
	if err != nil {
		return fmt.Errorf("failed to mark token verified: %w", err)
	}

	// Activate user
	_, err = tx.Exec("UPDATE users SET is_active = 1 WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to activate user: %w", err)
	}

	return tx.Commit()
}

// ResendVerification creates a new verification token.
func (m *Manager) ResendVerification(email string) (string, error) {
	var userID int64
	var isActive bool

	err := m.db.QueryRow(
		"SELECT id, is_active FROM users WHERE email = ?",
		email,
	).Scan(&userID, &isActive)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't reveal if email exists
			return "", nil
		}
		return "", fmt.Errorf("failed to find user: %w", err)
	}

	if isActive {
		return "", ErrAlreadyVerified
	}

	return m.createVerificationToken(userID, email)
}

// createVerificationToken generates and stores a verification token.
func (m *Manager) createVerificationToken(userID int64, email string) (string, error) {
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := hashToken(token)

	now := time.Now()
	expiresAt := now.Add(TokenExpiration).Unix()

	_, err := m.db.Exec(`
		INSERT INTO email_verifications (user_id, email, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		userID, email, tokenHash, expiresAt, now.Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to store verification token: %w", err)
	}

	return token, nil
}

// validateUsername validates a username.
func (m *Manager) validateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

// validatePassword validates a password.
func (m *Manager) validatePassword(password string) error {
	if len(password) < m.config.PasswordMinLength {
		return ErrInvalidPassword
	}
	return nil
}

// validateEmail validates an email address.
func (m *Manager) validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}

	// Check allowed domains
	if len(m.config.AllowedDomains) > 0 {
		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			return ErrInvalidEmail
		}
		domain := strings.ToLower(parts[1])
		allowed := false
		for _, d := range m.config.AllowedDomains {
			if strings.ToLower(d) == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrInvalidEmail
		}
	}

	return nil
}

// hashToken creates a SHA256 hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// nullString returns a sql.NullString.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// Cleanup removes expired verification tokens.
func (m *Manager) Cleanup() (int64, error) {
	cutoff := time.Now().Add(-48 * time.Hour).Unix()
	result, err := m.db.Exec(
		"DELETE FROM email_verifications WHERE expires_at < ?",
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup tokens: %w", err)
	}
	return result.RowsAffected()
}

// IsEnabled returns whether registration is enabled.
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}
