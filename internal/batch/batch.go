// Package batch provides transaction batching functionality.
package batch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrBatchNotFound    = errors.New("batch not found")
	ErrBatchNotPending  = errors.New("batch is not pending")
	ErrBatchEmpty       = errors.New("batch is empty")
	ErrBatchProcessing  = errors.New("batch is already processing")
	ErrTxLimitExceeded  = errors.New("transaction limit exceeded")
)

// BatchStatus defines batch states.
type BatchStatus string

const (
	StatusPending    BatchStatus = "pending"
	StatusProcessing BatchStatus = "processing"
	StatusCompleted  BatchStatus = "completed"
	StatusFailed     BatchStatus = "failed"
	StatusPartial    BatchStatus = "partial"
)

// MaxBatchSize is the maximum number of transactions in a batch.
const MaxBatchSize = 100

// Batch represents a transaction batch.
type Batch struct {
	ID             int64       `json:"id"`
	UserID         int64       `json:"user_id"`
	OrgID          *int64      `json:"org_id,omitempty"`
	Status         BatchStatus `json:"status"`
	TotalAmount    int64       `json:"total_amount"`
	TxCount        int         `json:"tx_count"`
	ProcessedCount int         `json:"processed_count"`
	Error          string      `json:"error,omitempty"`
	CreatedAt      int64       `json:"created_at"`
	StartedAt      *int64      `json:"started_at,omitempty"`
	CompletedAt    *int64      `json:"completed_at,omitempty"`
}

// Transaction represents a single transaction in a batch.
type Transaction struct {
	ID        string `json:"id"`
	BatchID   int64  `json:"batch_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// CreateBatchRequest represents a batch creation request.
type CreateBatchRequest struct {
	Transactions []TransactionInput `json:"transactions"`
}

// TransactionInput represents input for a single transaction.
type TransactionInput struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount int64  `json:"amount"`
}

// Provider interface for sending transactions.
type Provider interface {
	Send(ctx context.Context, from, to string, amount int64) (string, error)
}

// Manager handles batch operations.
type Manager struct {
	db       *sql.DB
	provider Provider
	mu       sync.Mutex
	processing map[int64]bool
}

// NewManager creates a new batch manager.
func NewManager(db *sql.DB, provider Provider) *Manager {
	return &Manager{
		db:         db,
		provider:   provider,
		processing: make(map[int64]bool),
	}
}

// Create creates a new transaction batch.
func (m *Manager) Create(userID int64, orgID *int64, txs []TransactionInput) (*Batch, error) {
	if len(txs) == 0 {
		return nil, ErrBatchEmpty
	}
	if len(txs) > MaxBatchSize {
		return nil, ErrTxLimitExceeded
	}

	var totalAmount int64
	for _, tx := range txs {
		totalAmount += tx.Amount
	}

	now := time.Now().Unix()
	result, err := m.db.Exec(`
		INSERT INTO tx_batches (user_id, org_id, status, total_amount, tx_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, orgID, StatusPending, totalAmount, len(txs), now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch: %w", err)
	}

	batchID, _ := result.LastInsertId()

	// Insert transactions
	for _, tx := range txs {
		txID := fmt.Sprintf("batch_%d_%d", batchID, time.Now().UnixNano())
		_, err = m.db.Exec(`
			INSERT INTO tx_log (id, user_id, org_id, from_addr, to_addr, amount, status, batch_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			txID, userID, orgID, tx.From, tx.To, tx.Amount, "pending", batchID, now,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to add transaction: %w", err)
		}
	}

	return &Batch{
		ID:          batchID,
		UserID:      userID,
		OrgID:       orgID,
		Status:      StatusPending,
		TotalAmount: totalAmount,
		TxCount:     len(txs),
		CreatedAt:   now,
	}, nil
}

// Get retrieves a batch by ID.
func (m *Manager) Get(id int64) (*Batch, error) {
	var b Batch
	var orgID sql.NullInt64
	var errorStr sql.NullString
	var startedAt, completedAt sql.NullInt64

	err := m.db.QueryRow(`
		SELECT id, user_id, org_id, status, total_amount, tx_count, processed_count, error, created_at, started_at, completed_at
		FROM tx_batches WHERE id = ?`,
		id,
	).Scan(&b.ID, &b.UserID, &orgID, &b.Status, &b.TotalAmount, &b.TxCount, &b.ProcessedCount, &errorStr, &b.CreatedAt, &startedAt, &completedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBatchNotFound
		}
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}

	if orgID.Valid {
		b.OrgID = &orgID.Int64
	}
	if errorStr.Valid {
		b.Error = errorStr.String
	}
	if startedAt.Valid {
		b.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		b.CompletedAt = &completedAt.Int64
	}

	return &b, nil
}

// List lists batches for a user.
func (m *Manager) List(userID int64, status *BatchStatus, limit int) ([]Batch, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := "SELECT id, user_id, org_id, status, total_amount, tx_count, processed_count, error, created_at, started_at, completed_at FROM tx_batches WHERE user_id = ?"
	args := []any{userID}

	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list batches: %w", err)
	}
	defer rows.Close()

	var batches []Batch
	for rows.Next() {
		var b Batch
		var orgID sql.NullInt64
		var errorStr sql.NullString
		var startedAt, completedAt sql.NullInt64

		err := rows.Scan(&b.ID, &b.UserID, &orgID, &b.Status, &b.TotalAmount, &b.TxCount, &b.ProcessedCount, &errorStr, &b.CreatedAt, &startedAt, &completedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch: %w", err)
		}

		if orgID.Valid {
			b.OrgID = &orgID.Int64
		}
		if errorStr.Valid {
			b.Error = errorStr.String
		}
		if startedAt.Valid {
			b.StartedAt = &startedAt.Int64
		}
		if completedAt.Valid {
			b.CompletedAt = &completedAt.Int64
		}

		batches = append(batches, b)
	}

	return batches, nil
}

// GetTransactions retrieves transactions for a batch.
func (m *Manager) GetTransactions(batchID int64) ([]Transaction, error) {
	rows, err := m.db.Query(`
		SELECT id, from_addr, to_addr, amount, status, created_at
		FROM tx_log WHERE batch_id = ? ORDER BY created_at`,
		batchID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		var tx Transaction
		tx.BatchID = batchID
		err := rows.Scan(&tx.ID, &tx.From, &tx.To, &tx.Amount, &tx.Status, &tx.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

// Process processes a pending batch.
func (m *Manager) Process(ctx context.Context, batchID int64) error {
	m.mu.Lock()
	if m.processing[batchID] {
		m.mu.Unlock()
		return ErrBatchProcessing
	}
	m.processing[batchID] = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.processing, batchID)
		m.mu.Unlock()
	}()

	batch, err := m.Get(batchID)
	if err != nil {
		return err
	}

	if batch.Status != StatusPending {
		return ErrBatchNotPending
	}

	// Mark as processing
	now := time.Now().Unix()
	_, err = m.db.Exec(
		"UPDATE tx_batches SET status = ?, started_at = ? WHERE id = ?",
		StatusProcessing, now, batchID,
	)
	if err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}

	// Get pending transactions
	txs, err := m.GetTransactions(batchID)
	if err != nil {
		return err
	}

	var successCount, failCount int
	var lastError string

	for _, tx := range txs {
		if tx.Status != "pending" {
			continue
		}

		// Process transaction
		_, err := m.provider.Send(ctx, tx.From, tx.To, tx.Amount)
		if err != nil {
			failCount++
			lastError = err.Error()
			m.db.Exec("UPDATE tx_log SET status = ? WHERE id = ?", "failed", tx.ID)
		} else {
			successCount++
			m.db.Exec("UPDATE tx_log SET status = ? WHERE id = ?", "submitted", tx.ID)
		}

		// Update progress
		m.db.Exec(
			"UPDATE tx_batches SET processed_count = ? WHERE id = ?",
			successCount+failCount, batchID,
		)
	}

	// Determine final status
	completedAt := time.Now().Unix()
	var finalStatus BatchStatus
	if failCount == 0 {
		finalStatus = StatusCompleted
	} else if successCount == 0 {
		finalStatus = StatusFailed
	} else {
		finalStatus = StatusPartial
	}

	_, err = m.db.Exec(
		"UPDATE tx_batches SET status = ?, error = ?, completed_at = ? WHERE id = ?",
		finalStatus, lastError, completedAt, batchID,
	)
	if err != nil {
		return fmt.Errorf("failed to finalize batch: %w", err)
	}

	return nil
}

// Cancel cancels a pending batch.
func (m *Manager) Cancel(batchID, userID int64) error {
	batch, err := m.Get(batchID)
	if err != nil {
		return err
	}

	if batch.UserID != userID {
		return ErrBatchNotFound
	}

	if batch.Status != StatusPending {
		return ErrBatchNotPending
	}

	m.mu.Lock()
	if m.processing[batchID] {
		m.mu.Unlock()
		return ErrBatchProcessing
	}
	m.mu.Unlock()

	now := time.Now().Unix()
	_, err = m.db.Exec(
		"UPDATE tx_batches SET status = 'cancelled', completed_at = ? WHERE id = ?",
		now, batchID,
	)
	if err != nil {
		return fmt.Errorf("failed to cancel batch: %w", err)
	}

	// Cancel pending transactions
	_, err = m.db.Exec(
		"UPDATE tx_log SET status = 'cancelled' WHERE batch_id = ? AND status = 'pending'",
		batchID,
	)
	if err != nil {
		return fmt.Errorf("failed to cancel transactions: %w", err)
	}

	return nil
}

// Summary represents a batch summary.
type Summary struct {
	TotalBatches    int   `json:"total_batches"`
	PendingBatches  int   `json:"pending_batches"`
	CompletedBatches int  `json:"completed_batches"`
	FailedBatches   int   `json:"failed_batches"`
	TotalAmount     int64 `json:"total_amount"`
	TotalTxCount    int   `json:"total_tx_count"`
}

// GetSummary returns a summary for a user.
func (m *Manager) GetSummary(userID int64) (*Summary, error) {
	var s Summary

	err := m.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(total_amount), 0), COALESCE(SUM(tx_count), 0)
		FROM tx_batches WHERE user_id = ?`,
		userID,
	).Scan(&s.TotalBatches, &s.TotalAmount, &s.TotalTxCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	m.db.QueryRow("SELECT COUNT(*) FROM tx_batches WHERE user_id = ? AND status = ?", userID, StatusPending).Scan(&s.PendingBatches)
	m.db.QueryRow("SELECT COUNT(*) FROM tx_batches WHERE user_id = ? AND status = ?", userID, StatusCompleted).Scan(&s.CompletedBatches)
	m.db.QueryRow("SELECT COUNT(*) FROM tx_batches WHERE user_id = ? AND status = ?", userID, StatusFailed).Scan(&s.FailedBatches)

	return &s, nil
}

// BatchResult represents the result of processing a batch.
type BatchResult struct {
	BatchID    int64       `json:"batch_id"`
	Status     BatchStatus `json:"status"`
	Successful int         `json:"successful"`
	Failed     int         `json:"failed"`
	Results    []TxResult  `json:"results"`
}

// TxResult represents the result of a single transaction.
type TxResult struct {
	TxID   string `json:"tx_id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Amount int64  `json:"amount"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ToJSON converts a batch to JSON.
func (b *Batch) ToJSON() ([]byte, error) {
	return json.Marshal(b)
}
