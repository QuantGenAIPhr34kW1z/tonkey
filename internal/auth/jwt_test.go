package auth

import (
	"testing"
	"time"
)

func TestIssueAndVerify(t *testing.T) {
	secret := []byte("test-secret-key")
	user := "testuser"

	// Issue token
	token, err := Issue(secret, user)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" {
		t.Fatal("Issue() returned empty token")
	}

	// Verify token
	claims, err := Verify(secret, token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.User != user {
		t.Errorf("Verify() user = %v, want %v", claims.User, user)
	}
	if claims.Issuer != "tonkey" {
		t.Errorf("Verify() issuer = %v, want 'tonkey'", claims.Issuer)
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	secret := []byte("test-secret-key")

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"malformed token", "not.a.jwt"},
		{"invalid signature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoidGVzdCJ9.invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Verify(secret, tt.token)
			if err == nil {
				t.Errorf("Verify() expected error for %v, got nil", tt.name)
			}
		})
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	secret1 := []byte("secret1")
	secret2 := []byte("secret2")

	token, err := Issue(secret1, "testuser")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	// Try to verify with different secret
	_, err = Verify(secret2, token)
	if err == nil {
		t.Error("Verify() should fail with different secret")
	}
}

func TestTokenExpiration(t *testing.T) {
	secret := []byte("test-secret")
	user := "testuser"

	token, err := Issue(secret, user)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	claims, err := Verify(secret, token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	// Check expiration is in the future
	if claims.ExpiresAt.Before(time.Now()) {
		t.Error("Token should not be expired immediately after issuance")
	}

	// Check expiration is approximately 24 hours from now
	expectedExpiry := time.Now().Add(24 * time.Hour)
	diff := claims.ExpiresAt.Sub(expectedExpiry)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("Token expiry = %v, want approximately %v (diff: %v)", claims.ExpiresAt, expectedExpiry, diff)
	}
}
