package api

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tonkey/internal/apikeys"
	"tonkey/internal/audit"
	"tonkey/internal/auth"
	"tonkey/internal/batch"
	"tonkey/internal/contracts"
	"tonkey/internal/jetton"
	"tonkey/internal/jobs"
	"tonkey/internal/logger"
	"tonkey/internal/multisig"
	"tonkey/internal/nft"
	"tonkey/internal/query"
	"tonkey/internal/registration"
	"tonkey/internal/reset"
	"tonkey/internal/store"
	"tonkey/internal/ton"
	"tonkey/internal/totp"
	"tonkey/internal/users"
	"tonkey/internal/validator"
	"tonkey/internal/webhooks"
	ws "tonkey/internal/websocket"

	"github.com/gorilla/websocket"
)

type Server struct {
	Secret      []byte
	Store       store.TxStore
	Provider    ton.Provider
	WSHub       *ws.Hub
	WebhookMgr  *webhooks.Manager
	UserMgr     *users.Manager
	TOTPMgr     *totp.Manager
	AuditLogger *audit.Logger
	ResetMgr    *reset.Manager
	RegMgr      *registration.Manager
	ContractMgr *contracts.Manager
	MultisigMgr *multisig.Manager
	BatchMgr    *batch.Manager
	APIKeyMgr   *apikeys.Manager
	JobQueue    *jobs.Queue
	NFTMgr      *nft.Manager
	JettonMgr   *jetton.Manager
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // TODO: Configure origins
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/auth/login", s.handleLogin)

	authed := func(h http.HandlerFunc) http.Handler {
		return WithAuth(s.Secret, h)
	}

	// Core endpoints
	mux.Handle("/v1/wallet/", authed(s.handleWalletBalance))
	mux.Handle("/v1/tx/send", authed(s.handleSendTx))
	mux.Handle("/v1/tx/", authed(s.handleGetTx))
	mux.Handle("/v1/query", authed(s.handleQuery))

	// WebSocket endpoint (if enabled)
	if s.WSHub != nil {
		mux.HandleFunc("/ws", s.handleWebSocket)
	}

	// Webhook endpoints (if enabled)
	if s.WebhookMgr != nil {
		mux.Handle("/v1/webhooks", authed(s.handleWebhooks))
		mux.Handle("/v1/webhooks/", authed(s.handleWebhooks)) // For DELETE /v1/webhooks/{id}
	}

	// User management endpoints (if enabled)
	if s.UserMgr != nil {
		mux.Handle("/admin/users", authed(s.handleUsers))
		mux.Handle("/admin/users/", authed(s.handleUsers)) // For PUT/DELETE /admin/users/{id}
	}

	if s.TOTPMgr != nil {
		mux.Handle("/auth/2fa/setup", authed(s.handle2FASetup))
		mux.Handle("/auth/2fa/verify", authed(s.handle2FAVerify))
		mux.Handle("/auth/2fa/disable", authed(s.handle2FADisable))
	}

	if s.ResetMgr != nil {
		mux.HandleFunc("/auth/forgot-password", s.handleForgotPassword)
		mux.HandleFunc("/auth/reset-password", s.handleResetPassword)
	}

	if s.RegMgr != nil && s.RegMgr.IsEnabled() {
		mux.HandleFunc("/auth/register", s.handleRegister)
		mux.HandleFunc("/auth/verify-email", s.handleVerifyEmail)
	}

	if s.AuditLogger != nil {
		mux.Handle("/admin/audit", authed(s.handleAuditLogs))
	}

	if s.ContractMgr != nil {
		mux.Handle("/v1/contracts", authed(s.handleContracts))
		mux.Handle("/v1/contracts/", authed(s.handleContracts))
	}

	if s.MultisigMgr != nil {
		mux.Handle("/v1/multisig", authed(s.handleMultisig))
		mux.Handle("/v1/multisig/", authed(s.handleMultisig))
	}

	if s.BatchMgr != nil {
		mux.Handle("/v1/tx/batch", authed(s.handleBatch))
		mux.Handle("/v1/tx/batch/", authed(s.handleBatch))
	}

	if s.APIKeyMgr != nil {
		mux.Handle("/v1/api-keys", authed(s.handleAPIKeys))
		mux.Handle("/v1/api-keys/", authed(s.handleAPIKeys))
	}

	if s.JobQueue != nil {
		mux.Handle("/admin/jobs", authed(s.handleJobs))
		mux.Handle("/admin/jobs/", authed(s.handleJobs))
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Use UserMgr if available, otherwise fall back to demo auth
	if s.UserMgr != nil {
		user, err := s.UserMgr.Authenticate(req.User, req.Pass)
		if err != nil {
			logger.Warn.Printf("Failed login attempt for user: %s", req.User)
			time.Sleep(100 * time.Millisecond) // Prevent timing attacks
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		tok, err := auth.Issue(s.Secret, req.User)
		if err != nil {
			logger.Error.Printf("Failed to issue JWT token: %v", err)
			http.Error(w, "token generation failed", http.StatusInternalServerError)
			return
		}

		logger.Info.Printf("Successful login: user=%s (id=%d)", req.User, user.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoginResp{Token: tok})
		return
	}

	// Fallback: intentionally simple demo auth
	if subtle.ConstantTimeCompare([]byte(req.User), []byte("demo")) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Pass), []byte("demo")) != 1 {
		logger.Warn.Printf("Failed login attempt for user: %s", req.User)
		time.Sleep(100 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	tok, err := auth.Issue(s.Secret, req.User)
	if err != nil {
		logger.Error.Printf("Failed to issue JWT token: %v", err)
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	logger.Info.Printf("Successful login: user=%s (demo mode)", req.User)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResp{Token: tok})
}

func (s *Server) handleWalletBalance(w http.ResponseWriter, r *http.Request) {
	// /v1/wallet/{addr}/balance
	path := strings.TrimPrefix(r.URL.Path, "/v1/wallet/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "balance" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	addr := parts[0]

	// Validate address
	if err := validator.ValidateTONAddress(addr); err != nil {
		logger.Warn.Printf("Invalid address requested: %s", addr)
		http.Error(w, "invalid TON address format", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	bal, err := s.Provider.Balance(ctx, addr)
	if err != nil {
		logger.Error.Printf("Provider error for balance query (addr=%s): %v", addr, err)
		http.Error(w, "provider error", http.StatusBadGateway)
		return
	}

	logger.Info.Printf("Balance query successful: addr=%s, balance=%d", addr, bal)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"address": addr,
		"balance": bal,
	})
}

func (s *Server) handleSendTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendTxReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if err := validator.ValidateTONAddress(req.From); err != nil {
		http.Error(w, "invalid 'from' address", http.StatusBadRequest)
		return
	}
	if err := validator.ValidateTONAddress(req.To); err != nil {
		http.Error(w, "invalid 'to' address", http.StatusBadRequest)
		return
	}
	if err := validator.ValidateAmount(req.Amount); err != nil {
		http.Error(w, "amount must be positive", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	id, err := s.Provider.Send(ctx, req.From, req.To, req.Amount)
	if err != nil {
		logger.Error.Printf("Failed to send tx (from=%s, to=%s, amount=%d): %v", req.From, req.To, req.Amount, err)
		http.Error(w, "provider error", http.StatusBadGateway)
		return
	}

	tx := store.Tx{
		ID:        id,
		CreatedAt: time.Now().Unix(),
		From:      req.From,
		To:        req.To,
		Amount:    req.Amount,
		Status:    "submitted",
	}

	if err := s.Store.InsertTx(tx); err != nil {
		logger.Error.Printf("Failed to store tx %s: %v", id, err)
		// Still return success since tx was submitted to provider
	}

	logger.Info.Printf("Transaction submitted: id=%s, from=%s, to=%s, amount=%d", id, req.From, req.To, req.Amount)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SendTxResp{ID: id})
}

func (s *Server) handleGetTx(w http.ResponseWriter, r *http.Request) {
	// /v1/tx/{id}
	id := strings.TrimPrefix(r.URL.Path, "/v1/tx/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	t, err := s.Store.GetTx(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(t)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req QueryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}

	if req.Entity != "tx" {
		http.Error(w, "unknown entity", http.StatusBadRequest)
		return
	}

	rows, err := query.QueryTx(s.Store, req.Filters, req.Limit)
	if err != nil {
		http.Error(w, "query error", http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"entity": req.Entity,
		"count":  len(rows),
		"rows":   rows,
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := auth.Verify(s.Secret, token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Create and register client
	client := ws.NewClient(s.WSHub, conn, claims.User)
	s.WSHub.Broadcast(ws.Message{Type: "info", Data: map[string]interface{}{"clients": s.WSHub.GetClientCount() + 1}})
	client.Start()
}

func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	authCtx := fromAuth(r.Context())
	if authCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// For DELETE /v1/webhooks/{id}, extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/webhooks/")
	if path != r.URL.Path && r.Method == "DELETE" {
		s.handleDeleteWebhook(w, r, authCtx, path)
		return
	}

	switch r.Method {
	case "GET":
		s.handleListWebhooks(w, r, authCtx)
	case "POST":
		s.handleCreateWebhook(w, r, authCtx)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx) {
	if s.WebhookMgr == nil {
		http.Error(w, "webhooks not enabled", http.StatusServiceUnavailable)
		return
	}

	// TODO: Get real user ID from user management system
	userID := authCtx.UserID
	if userID == 0 {
		userID = 1 // Temporary fallback for demo
	}

	webhooks, err := s.WebhookMgr.ListWebhooks(userID)
	if err != nil {
		logger.Error.Printf("Failed to list webhooks: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Convert to response format (without secrets)
	response := make([]WebhookResp, len(webhooks))
	for i, wh := range webhooks {
		response[i] = WebhookResp{
			ID:            wh.ID,
			URL:           wh.URL,
			EventType:     wh.EventType,
			FilterAddress: wh.FilterAddress,
			IsActive:      wh.IsActive,
			CreatedAt:     wh.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"webhooks": response,
		"count":    len(response),
	})
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx) {
	if s.WebhookMgr == nil {
		http.Error(w, "webhooks not enabled", http.StatusServiceUnavailable)
		return
	}

	var req CreateWebhookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if req.EventType == "" {
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}
	if req.EventType != "transaction" && req.EventType != "balance_change" {
		http.Error(w, "event_type must be 'transaction' or 'balance_change'", http.StatusBadRequest)
		return
	}
	if req.Secret == "" {
		http.Error(w, "secret is required for HMAC signing", http.StatusBadRequest)
		return
	}
	if !validator.IsValidURL(req.URL) {
		http.Error(w, "invalid url format", http.StatusBadRequest)
		return
	}
	if req.FilterAddress != "" && !validator.IsValidTONAddress(req.FilterAddress) {
		http.Error(w, "invalid filter_address format", http.StatusBadRequest)
		return
	}

	// TODO: Get real user ID from user management system
	userID := authCtx.UserID
	if userID == 0 {
		userID = 1 // Temporary fallback for demo
	}

	webhook, err := s.WebhookMgr.CreateWebhook(userID, 0, req.URL, req.EventType, req.FilterAddress, req.Secret)
	if err != nil {
		logger.Error.Printf("Failed to create webhook: %v", err)
		http.Error(w, "failed to create webhook", http.StatusInternalServerError)
		return
	}

	response := WebhookResp{
		ID:            webhook.ID,
		URL:           webhook.URL,
		EventType:     webhook.EventType,
		FilterAddress: webhook.FilterAddress,
		IsActive:      webhook.IsActive,
		CreatedAt:     webhook.CreatedAt,
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx, idStr string) {
	if s.WebhookMgr == nil {
		http.Error(w, "webhooks not enabled", http.StatusServiceUnavailable)
		return
	}

	// Parse webhook ID
	var id int64
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil || id <= 0 {
		http.Error(w, "invalid webhook id", http.StatusBadRequest)
		return
	}

	// TODO: Get real user ID from user management system
	userID := authCtx.UserID
	if userID == 0 {
		userID = 1 // Temporary fallback for demo
	}

	err = s.WebhookMgr.DeleteWebhook(id, userID)
	if err != nil {
		logger.Error.Printf("Failed to delete webhook: %v", err)
		http.Error(w, "failed to delete webhook", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "webhook deleted successfully",
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	authCtx := fromAuth(r.Context())
	if authCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// For PUT/DELETE /admin/users/{id}, extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	if path != r.URL.Path {
		// This is /admin/users/{id}
		switch r.Method {
		case "PUT":
			s.handleUpdateUser(w, r, authCtx, path)
		case "DELETE":
			s.handleDeleteUser(w, r, authCtx, path)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// This is /admin/users
	switch r.Method {
	case "GET":
		s.handleListUsers(w, r, authCtx)
	case "POST":
		s.handleCreateUser(w, r, authCtx)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx) {
	if s.UserMgr == nil {
		http.Error(w, "user management not enabled", http.StatusServiceUnavailable)
		return
	}

	// TODO: Add proper role-based access control
	// For now, any authenticated user can list users

	// List all users (orgID=0 means no filter)
	usersList, err := s.UserMgr.ListUsers(0)
	if err != nil {
		logger.Error.Printf("Failed to list users: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Convert to response format (without password hashes)
	response := make([]UserResp, len(usersList))
	for i, u := range usersList {
		response[i] = UserResp{
			ID:        u.ID,
			Username:  u.Username,
			Email:     u.Email,
			OrgID:     u.OrgID,
			Role:      u.Role,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt,
			LastLogin: u.LastLogin,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"users": response,
		"count": len(response),
	})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx) {
	if s.UserMgr == nil {
		http.Error(w, "user management not enabled", http.StatusServiceUnavailable)
		return
	}

	var req CreateUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if req.Email != "" && !validator.IsValidEmail(req.Email) {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}

	// Default to user role if not specified
	role := req.Role
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		http.Error(w, "role must be 'user' or 'admin'", http.StatusBadRequest)
		return
	}

	user, err := s.UserMgr.CreateUser(req.Username, req.Password, req.Email, req.OrgID, role)
	if err != nil {
		if err == users.ErrUserExists {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}
		logger.Error.Printf("Failed to create user: %v", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	response := UserResp{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		OrgID:     user.OrgID,
		Role:      user.Role,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		LastLogin: user.LastLogin,
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx, idStr string) {
	if s.UserMgr == nil {
		http.Error(w, "user management not enabled", http.StatusServiceUnavailable)
		return
	}

	// Parse user ID
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	// Get current user to preserve unmodified fields
	currentUser, err := s.UserMgr.GetUser(id)
	if err != nil {
		if err == users.ErrUserNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		logger.Error.Printf("Failed to get user: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var req UpdateUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Use provided values or fall back to current values
	email := currentUser.Email
	if req.Email != "" {
		if !validator.IsValidEmail(req.Email) {
			http.Error(w, "invalid email format", http.StatusBadRequest)
			return
		}
		email = req.Email
	}

	role := currentUser.Role
	if req.Role != "" {
		if req.Role != "user" && req.Role != "admin" {
			http.Error(w, "role must be 'user' or 'admin'", http.StatusBadRequest)
			return
		}
		role = req.Role
	}

	isActive := currentUser.IsActive
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	err = s.UserMgr.UpdateUser(id, email, role, isActive)
	if err != nil {
		logger.Error.Printf("Failed to update user: %v", err)
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "user updated successfully",
	})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, authCtx *AuthCtx, idStr string) {
	if s.UserMgr == nil {
		http.Error(w, "user management not enabled", http.StatusServiceUnavailable)
		return
	}

	// Parse user ID
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	// Prevent self-deletion
	if authCtx.UserID == id {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	err = s.UserMgr.DeleteUser(id)
	if err != nil {
		if err == users.ErrUserNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		logger.Error.Printf("Failed to delete user: %v", err)
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "user deleted successfully",
	})
}
