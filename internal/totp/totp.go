// Package totp provides Two-Factor Authentication using TOTP (Time-based One-Time Password).
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCode    = errors.New("invalid TOTP code")
	ErrTOTPNotEnabled = errors.New("TOTP not enabled for user")
	ErrTOTPNotSetup   = errors.New("TOTP not set up for user")
	ErrBackupCodeUsed = errors.New("backup code already used")
	ErrNoBackupCodes  = errors.New("no valid backup codes remaining")
)

const (
	// DefaultPeriod is the time step in seconds (30 seconds is standard)
	DefaultPeriod = 30
	// DefaultDigits is the number of digits in the TOTP code
	DefaultDigits = 6
	// DefaultWindow allows codes from adjacent time periods
	DefaultWindow = 1
	// BackupCodeCount is the number of backup codes to generate
	BackupCodeCount = 10
	// BackupCodeLength is the length of each backup code
	BackupCodeLength = 8
)

// Manager handles TOTP operations.
type Manager struct {
	db     *sql.DB
	issuer string
}

// NewManager creates a new TOTP manager.
func NewManager(db *sql.DB, issuer string) *Manager {
	return &Manager{
		db:     db,
		issuer: issuer,
	}
}

// SetupResult contains the TOTP setup information.
type SetupResult struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qr_code_url"`
	BackupCodes []string `json:"backup_codes"`
}

// GenerateSecret creates a new TOTP secret.
func GenerateSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateCode generates a TOTP code for the given secret and time.
func GenerateCode(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("invalid secret: %w", err)
	}

	counter := uint64(t.Unix() / DefaultPeriod)
	return generateHOTP(key, counter, DefaultDigits), nil
}

// ValidateCode validates a TOTP code against the secret.
func ValidateCode(secret, code string) bool {
	now := time.Now()
	for i := -DefaultWindow; i <= DefaultWindow; i++ {
		t := now.Add(time.Duration(i*DefaultPeriod) * time.Second)
		expected, err := GenerateCode(secret, t)
		if err != nil {
			continue
		}
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}

// generateHOTP generates an HOTP code.
func generateHOTP(key []byte, counter uint64, digits int) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binCode := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	code := binCode % uint32(pow10(digits))
	return fmt.Sprintf("%0*d", digits, code)
}

func pow10(n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// Setup initiates TOTP setup for a user.
func (m *Manager) Setup(userID int64, username string) (*SetupResult, error) {
	secret, err := GenerateSecret()
	if err != nil {
		return nil, err
	}

	// Store the secret (not yet enabled)
	_, err = m.db.Exec(
		"UPDATE users SET totp_secret = ? WHERE id = ?",
		secret, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store TOTP secret: %w", err)
	}

	// Generate backup codes
	backupCodes, err := m.generateBackupCodes(userID)
	if err != nil {
		return nil, err
	}

	// Generate QR code URL (otpauth:// format)
	qrURL := fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		m.issuer, username, secret, m.issuer, DefaultDigits, DefaultPeriod,
	)

	return &SetupResult{
		Secret:      secret,
		QRCodeURL:   qrURL,
		BackupCodes: backupCodes,
	}, nil
}

// Verify verifies a TOTP code and enables 2FA if not already enabled.
func (m *Manager) Verify(userID int64, code string) error {
	var secret sql.NullString
	var enabled bool
	err := m.db.QueryRow(
		"SELECT totp_secret, totp_enabled FROM users WHERE id = ?",
		userID,
	).Scan(&secret, &enabled)
	if err != nil {
		return fmt.Errorf("failed to get user TOTP info: %w", err)
	}

	if !secret.Valid || secret.String == "" {
		return ErrTOTPNotSetup
	}

	if !ValidateCode(secret.String, code) {
		return ErrInvalidCode
	}

	// Enable TOTP if not already enabled
	if !enabled {
		now := time.Now().Unix()
		_, err = m.db.Exec(
			"UPDATE users SET totp_enabled = 1, totp_verified_at = ? WHERE id = ?",
			now, userID,
		)
		if err != nil {
			return fmt.Errorf("failed to enable TOTP: %w", err)
		}
	}

	return nil
}

// ValidateLogin validates a TOTP code during login.
func (m *Manager) ValidateLogin(userID int64, code string) error {
	var secret sql.NullString
	var enabled bool
	err := m.db.QueryRow(
		"SELECT totp_secret, totp_enabled FROM users WHERE id = ?",
		userID,
	).Scan(&secret, &enabled)
	if err != nil {
		return fmt.Errorf("failed to get user TOTP info: %w", err)
	}

	if !enabled {
		return ErrTOTPNotEnabled
	}

	if !secret.Valid || secret.String == "" {
		return ErrTOTPNotSetup
	}

	// Try TOTP code first
	if ValidateCode(secret.String, code) {
		return nil
	}

	// Try backup code
	return m.useBackupCode(userID, code)
}

// IsEnabled checks if TOTP is enabled for a user.
func (m *Manager) IsEnabled(userID int64) (bool, error) {
	var enabled bool
	err := m.db.QueryRow(
		"SELECT totp_enabled FROM users WHERE id = ?",
		userID,
	).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return enabled, nil
}

// Disable disables TOTP for a user.
func (m *Manager) Disable(userID int64, code string) error {
	// Validate the code first
	if err := m.ValidateLogin(userID, code); err != nil {
		return err
	}

	// Disable TOTP
	_, err := m.db.Exec(
		"UPDATE users SET totp_enabled = 0, totp_secret = NULL, totp_verified_at = NULL WHERE id = ?",
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to disable TOTP: %w", err)
	}

	// Delete backup codes
	_, err = m.db.Exec("DELETE FROM backup_codes WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to delete backup codes: %w", err)
	}

	return nil
}

// generateBackupCodes generates and stores backup codes for a user.
func (m *Manager) generateBackupCodes(userID int64) ([]string, error) {
	// Delete existing backup codes
	_, err := m.db.Exec("DELETE FROM backup_codes WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete existing backup codes: %w", err)
	}

	codes := make([]string, BackupCodeCount)
	now := time.Now().Unix()

	for i := 0; i < BackupCodeCount; i++ {
		code := generateBackupCode()
		codes[i] = code

		// Hash the code before storing
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash backup code: %w", err)
		}

		_, err = m.db.Exec(
			"INSERT INTO backup_codes (user_id, code_hash, created_at) VALUES (?, ?, ?)",
			userID, string(hash), now,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to store backup code: %w", err)
		}
	}

	return codes, nil
}

// generateBackupCode generates a single backup code.
func generateBackupCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, BackupCodeLength)
	randomBytes := make([]byte, BackupCodeLength)
	rand.Read(randomBytes)
	for i := 0; i < BackupCodeLength; i++ {
		code[i] = charset[randomBytes[i]%byte(len(charset))]
	}
	return string(code)
}

// useBackupCode attempts to use a backup code.
func (m *Manager) useBackupCode(userID int64, code string) error {
	rows, err := m.db.Query(
		"SELECT id, code_hash FROM backup_codes WHERE user_id = ? AND used_at IS NULL",
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to get backup codes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			continue
		}

		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			// Mark code as used
			now := time.Now().Unix()
			_, err = m.db.Exec(
				"UPDATE backup_codes SET used_at = ? WHERE id = ?",
				now, id,
			)
			if err != nil {
				return fmt.Errorf("failed to mark backup code as used: %w", err)
			}
			return nil
		}
	}

	return ErrInvalidCode
}

// RegenerateBackupCodes regenerates backup codes (requires valid TOTP code).
func (m *Manager) RegenerateBackupCodes(userID int64, totpCode string) ([]string, error) {
	// Validate TOTP code first
	if err := m.ValidateLogin(userID, totpCode); err != nil {
		return nil, err
	}

	return m.generateBackupCodes(userID)
}

// GetRemainingBackupCodes returns the count of unused backup codes.
func (m *Manager) GetRemainingBackupCodes(userID int64) (int, error) {
	var count int
	err := m.db.QueryRow(
		"SELECT COUNT(*) FROM backup_codes WHERE user_id = ? AND used_at IS NULL",
		userID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// HashForChecksum creates a checksum for audit logging.
func HashForChecksum(data string) string {
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:8])
}
