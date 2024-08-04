package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tonkey/internal/audit"
	"tonkey/internal/ipfilter"
	"tonkey/internal/logger"
	"tonkey/internal/registration"
)

// 2FA Handlers

func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := s.TOTPMgr.Setup(actx.UserID, actx.User)
	if err != nil {
		logger.Error.Printf("2FA setup failed: %v", err)
		http.Error(w, "failed to setup 2FA", http.StatusInternalServerError)
		return
	}

	// Log audit event
	if s.AuditLogger != nil {
		s.AuditLogger.LogAction(&actx.UserID, nil, audit.ActionTOTPSetup, audit.ResourceUser, strconv.FormatInt(actx.UserID, 10), ipfilter.GetClientIP(r), r.UserAgent(), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handle2FAVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	err := s.TOTPMgr.Verify(actx.UserID, req.Code)
	if err != nil {
		logger.Warn.Printf("2FA verification failed for user %d: %v", actx.UserID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log audit event
	if s.AuditLogger != nil {
		s.AuditLogger.LogAction(&actx.UserID, nil, audit.ActionTOTPVerify, audit.ResourceUser, strconv.FormatInt(actx.UserID, 10), ipfilter.GetClientIP(r), r.UserAgent(), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": true})
}

func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	err := s.TOTPMgr.Disable(actx.UserID, req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log audit event
	if s.AuditLogger != nil {
		s.AuditLogger.LogAction(&actx.UserID, nil, audit.ActionTOTPDisable, audit.ResourceUser, strconv.FormatInt(actx.UserID, 10), ipfilter.GetClientIP(r), r.UserAgent(), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"disabled": true})
}

// Password Reset Handlers

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	result, err := s.ResetMgr.CreateToken(req.Email)
	if err != nil {
		logger.Warn.Printf("Password reset request failed: %v", err)
		// Don't reveal if email exists
	}

	// In production, you would send the token via email
	// For now, just acknowledge the request
	w.Header().Set("Content-Type", "application/json")
	if result != nil {
		// In development/testing, return the token
		json.NewEncoder(w).Encode(map[string]any{
			"message": "If the email exists, a reset link has been sent",
			"token":   result.Token, // Remove this in production
		})
	} else {
		json.NewEncoder(w).Encode(map[string]string{
			"message": "If the email exists, a reset link has been sent",
		})
	}
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	err := s.ResetMgr.ResetPassword(req.Token, req.NewPassword)
	if err != nil {
		logger.Warn.Printf("Password reset failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log audit event
	if s.AuditLogger != nil {
		s.AuditLogger.LogAction(nil, nil, audit.ActionPasswordReset, "", "", ipfilter.GetClientIP(r), r.UserAgent(), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Registration Handlers

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registration.RegistrationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	result, err := s.RegMgr.Register(req)
	if err != nil {
		logger.Warn.Printf("Registration failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log audit event
	if s.AuditLogger != nil {
		s.AuditLogger.LogAction(&result.UserID, nil, audit.ActionRegistration, audit.ResourceUser, strconv.FormatInt(result.UserID, 10), ipfilter.GetClientIP(r), r.UserAgent(), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" && r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var token string
	if r.Method == "GET" {
		token = r.URL.Query().Get("token")
	} else {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}
		token = req.Token
	}

	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	err := s.RegMgr.VerifyEmail(token)
	if err != nil {
		logger.Warn.Printf("Email verification failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log audit event
	if s.AuditLogger != nil {
		s.AuditLogger.LogAction(nil, nil, audit.ActionEmailVerify, "", "", ipfilter.GetClientIP(r), r.UserAgent(), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"verified": true})
}

// Audit Log Handler

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Build query from URL params
	q := audit.Query{
		Limit:  100,
		Offset: 0,
	}

	if action := r.URL.Query().Get("action"); action != "" {
		q.Action = action
	}
	if resourceType := r.URL.Query().Get("resource_type"); resourceType != "" {
		q.ResourceType = resourceType
	}
	if resourceID := r.URL.Query().Get("resource_id"); resourceID != "" {
		q.ResourceID = resourceID
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			q.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			q.Offset = o
		}
	}
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if t, err := strconv.ParseInt(startTime, 10, 64); err == nil {
			q.StartTime = &t
		}
	}
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if t, err := strconv.ParseInt(endTime, 10, 64); err == nil {
			q.EndTime = &t
		}
	}

	entries, total, err := s.AuditLogger.Search(q)
	if err != nil {
		logger.Error.Printf("Audit search failed: %v", err)
		http.Error(w, "failed to search audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"total":   total,
		"limit":   q.Limit,
		"offset":  q.Offset,
	})
}

// Helper to get auth context
func getAuthCtx(r *http.Request) *AuthCtx {
	ctx := r.Context().Value(authCtxKey)
	if ctx == nil {
		return nil
	}
	actx, ok := ctx.(AuthCtx)
	if !ok {
		return nil
	}
	return &actx
}

//  handlers

func (s *Server) handleContracts(w http.ResponseWriter, r *http.Request) {
	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/contracts")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case "GET":
		if path == "" {
			// List contracts
			contracts, err := s.ContractMgr.List(actx.UserID, nil)
			if err != nil {
				http.Error(w, "failed to list contracts", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(contracts)
		} else {
			// Get contract by address
			contract, err := s.ContractMgr.Get(path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(contract)
		}

	case "POST":
		var req struct {
			Address      string `json:"address"`
			Name         string `json:"name"`
			ContractType string `json:"contract_type"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}

		contract, err := s.ContractMgr.Register(actx.UserID, nil, req.Address, req.Name, "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(contract)

	case "DELETE":
		if path == "" {
			http.Error(w, "contract address required", http.StatusBadRequest)
			return
		}
		if err := s.ContractMgr.Delete(path, actx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMultisig(w http.ResponseWriter, r *http.Request) {
	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/multisig")
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	switch r.Method {
	case "GET":
		if path == "" || parts[0] == "" {
			// List wallets (would need org_id from auth context in production)
			wallets, err := s.MultisigMgr.ListWallets(1) // Default org
			if err != nil {
				http.Error(w, "failed to list wallets", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(wallets)
		} else if len(parts) >= 2 && parts[1] == "proposals" {
			// List proposals for wallet
			wallet, err := s.MultisigMgr.GetWallet(parts[0])
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			proposals, err := s.MultisigMgr.ListProposals(wallet.ID, nil)
			if err != nil {
				http.Error(w, "failed to list proposals", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(proposals)
		} else {
			// Get wallet by address
			wallet, err := s.MultisigMgr.GetWallet(parts[0])
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(wallet)
		}

	case "POST":
		if len(parts) >= 2 && parts[1] == "proposals" {
			// Create proposal
			wallet, err := s.MultisigMgr.GetWallet(parts[0])
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}

			var req struct {
				ActionType string         `json:"action_type"`
				ActionData map[string]any `json:"action_data"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
				http.Error(w, "invalid JSON payload", http.StatusBadRequest)
				return
			}

			proposal, err := s.MultisigMgr.CreateProposal(wallet.ID, actx.UserID, "", req.ActionData, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(proposal)
		} else if len(parts) >= 3 && parts[2] == "approve" {
			// Approve proposal
			proposalID, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				http.Error(w, "invalid proposal ID", http.StatusBadRequest)
				return
			}

			var req struct {
				Signature string `json:"signature"`
			}
			json.NewDecoder(r.Body).Decode(&req)

			if err := s.MultisigMgr.Approve(proposalID, actx.UserID, req.Signature); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"approved": true})
		} else if len(parts) >= 3 && parts[2] == "reject" {
			// Reject proposal
			proposalID, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				http.Error(w, "invalid proposal ID", http.StatusBadRequest)
				return
			}

			if err := s.MultisigMgr.Reject(proposalID, actx.UserID); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"rejected": true})
		} else {
			http.Error(w, "invalid path", http.StatusBadRequest)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/tx/batch")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case "GET":
		if path == "" {
			// List batches
			batches, err := s.BatchMgr.List(actx.UserID, nil, 50)
			if err != nil {
				http.Error(w, "failed to list batches", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(batches)
		} else {
			// Get batch by ID
			batchID, err := strconv.ParseInt(path, 10, 64)
			if err != nil {
				http.Error(w, "invalid batch ID", http.StatusBadRequest)
				return
			}
			batch, err := s.BatchMgr.Get(batchID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			txs, _ := s.BatchMgr.GetTransactions(batchID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"batch":        batch,
				"transactions": txs,
			})
		}

	case "POST":
		var req struct {
			Transactions []struct {
				From   string `json:"from"`
				To     string `json:"to"`
				Amount int64  `json:"amount"`
			} `json:"transactions"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}

		txInputs := make([]struct {
			From   string
			To     string
			Amount int64
		}, len(req.Transactions))
		for i, tx := range req.Transactions {
			txInputs[i].From = tx.From
			txInputs[i].To = tx.To
			txInputs[i].Amount = tx.Amount
		}

		// Convert to batch.TransactionInput
		batchTxs := make([]struct {
			From   string
			To     string
			Amount int64
		}, len(req.Transactions))
		for i, tx := range req.Transactions {
			batchTxs[i] = struct {
				From   string
				To     string
				Amount int64
			}{tx.From, tx.To, tx.Amount}
		}

		// Use the batch manager with proper type
		batch, err := s.BatchMgr.Create(actx.UserID, nil, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Log audit event
		if s.AuditLogger != nil {
			s.AuditLogger.LogAction(&actx.UserID, nil, audit.ActionTxBatch, "batch", strconv.FormatInt(batch.ID, 10), ipfilter.GetClientIP(r), r.UserAgent(), nil)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(batch)

	case "DELETE":
		if path == "" {
			http.Error(w, "batch ID required", http.StatusBadRequest)
			return
		}
		batchID, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			http.Error(w, "invalid batch ID", http.StatusBadRequest)
			return
		}
		if err := s.BatchMgr.Cancel(batchID, actx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/api-keys")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case "GET":
		keys, err := s.APIKeyMgr.List(actx.UserID)
		if err != nil {
			http.Error(w, "failed to list API keys", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)

	case "POST":
		var req struct {
			Name      string   `json:"name"`
			Scopes    []string `json:"scopes"`
			ExpiresIn *int     `json:"expires_in"` // days
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		var expiresAt *int64
		if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
			exp := time.Now().AddDate(0, 0, *req.ExpiresIn).Unix()
			expiresAt = &exp
		}

		result, err := s.APIKeyMgr.Create(actx.UserID, nil, req.Name, req.Scopes, nil, expiresAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Log audit event
		if s.AuditLogger != nil {
			s.AuditLogger.LogAction(&actx.UserID, nil, audit.ActionAPIKeyCreate, audit.ResourceAPIKey, strconv.FormatInt(result.Key.ID, 10), ipfilter.GetClientIP(r), r.UserAgent(), nil)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)

	case "DELETE":
		if path == "" {
			http.Error(w, "key ID required", http.StatusBadRequest)
			return
		}
		keyID, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			http.Error(w, "invalid key ID", http.StatusBadRequest)
			return
		}
		if err := s.APIKeyMgr.Revoke(keyID, actx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Log audit event
		if s.AuditLogger != nil {
			s.AuditLogger.LogAction(&actx.UserID, nil, audit.ActionAPIKeyRevoke, audit.ResourceAPIKey, path, ipfilter.GetClientIP(r), r.UserAgent(), nil)
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	actx := getAuthCtx(r)
	if actx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/jobs")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case "GET":
		if path == "" || path == "stats" {
			// Get stats
			stats, err := s.JobQueue.GetStats()
			if err != nil {
				http.Error(w, "failed to get job stats", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
		} else {
			// Get job by ID
			jobID, err := strconv.ParseInt(path, 10, 64)
			if err != nil {
				http.Error(w, "invalid job ID", http.StatusBadRequest)
				return
			}
			job, err := s.JobQueue.GetJob(jobID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(job)
		}

	case "POST":
		if strings.HasSuffix(path, "/retry") {
			// Retry job
			jobID, err := strconv.ParseInt(strings.TrimSuffix(path, "/retry"), 10, 64)
			if err != nil {
				http.Error(w, "invalid job ID", http.StatusBadRequest)
				return
			}
			if err := s.JobQueue.RetryJob(jobID); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"retried": true})
		} else {
			// Enqueue new job
			var req struct {
				JobType      string `json:"job_type"`
				Payload      any    `json:"payload"`
				Queue        string `json:"queue"`
				Priority     int    `json:"priority"`
				DelaySeconds int    `json:"delay_seconds"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
				http.Error(w, "invalid JSON payload", http.StatusBadRequest)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			_ = ctx // would use for async operations

			jobID, err := s.JobQueue.Enqueue(req.JobType, req.Payload)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]int64{"job_id": jobID})
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
