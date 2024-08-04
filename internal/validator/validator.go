package validator

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidAddress = errors.New("invalid TON address")
	ErrInvalidAmount  = errors.New("invalid amount")
)

// TON address validation - basic format check
// Real TON addresses are base64url encoded and start with EQ, UQ, or kQ
var tonAddressRegex = regexp.MustCompile(`^[EUku]Q[A-Za-z0-9_-]{46}$`)

// ValidateTONAddress checks if the provided string is a valid TON address format
func ValidateTONAddress(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ErrInvalidAddress
	}
	// For mock provider, allow test addresses
	if strings.HasPrefix(addr, "EQDemo") || strings.HasPrefix(addr, "EQTest") {
		return nil
	}
	if !tonAddressRegex.MatchString(addr) {
		return ErrInvalidAddress
	}
	return nil
}

// ValidateAmount checks if the amount is positive
func ValidateAmount(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

// Email validation regex - basic RFC 5322 compliance
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail checks if the provided string is a valid email format
func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	return emailRegex.MatchString(email)
}

// URL validation regex - basic http/https URL check
var urlRegex = regexp.MustCompile(`^https?://[a-zA-Z0-9.\-]+(:[0-9]+)?(/.*)?$`)

// IsValidURL checks if the provided string is a valid HTTP/HTTPS URL
func IsValidURL(url string) bool {
	url = strings.TrimSpace(url)
	if url == "" {
		return false
	}
	return urlRegex.MatchString(url)
}

// IsValidTONAddress is a convenience wrapper around ValidateTONAddress that returns bool
func IsValidTONAddress(addr string) bool {
	return ValidateTONAddress(addr) == nil
}
