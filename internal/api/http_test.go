package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"tonkey/internal/store"
	"tonkey/internal/ton"
)

func TestHealthz(t *testing.T) {
	srv := &Server{
		Secret:   []byte("test-secret"),
		Store:    nil, // not needed for healthz
		Provider: ton.NewMock(),
	}

	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "ok" {
		t.Errorf("GET /healthz body = %q, want 'ok'", w.Body.String())
	}
}

func TestLogin(t *testing.T) {
	srv := &Server{
		Secret:   []byte("test-secret"),
		Store:    nil,
		Provider: ton.NewMock(),
	}

	mux := http.NewServeMux()
	srv.Register(mux)

	tests := []struct {
		name       string
		payload    LoginReq
		wantStatus int
	}{
		{"valid credentials", LoginReq{User: "demo", Pass: "demo"}, http.StatusOK},
		{"invalid user", LoginReq{User: "wrong", Pass: "demo"}, http.StatusUnauthorized},
		{"invalid pass", LoginReq{User: "demo", Pass: "wrong"}, http.StatusUnauthorized},
		{"empty credentials", LoginReq{}, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("POST /auth/login status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp LoginResp
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.Token == "" {
					t.Error("Expected non-empty token")
				}
			}
		})
	}
}

func TestLoginMethodNotAllowed(t *testing.T) {
	srv := &Server{
		Secret:   []byte("test-secret"),
		Store:    nil,
		Provider: ton.NewMock(),
	}

	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /auth/login status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestWalletBalanceWithoutAuth(t *testing.T) {
	srv := &Server{
		Secret:   []byte("test-secret"),
		Store:    nil,
		Provider: ton.NewMock(),
	}

	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest("GET", "/v1/wallet/EQTestAddr/balance", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /v1/wallet/.../balance without auth status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
