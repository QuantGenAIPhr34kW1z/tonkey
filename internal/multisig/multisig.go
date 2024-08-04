// Package multisig provides multi-signature wallet functionality.
package multisig

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrWalletNotFound      = errors.New("multisig wallet not found")
	ErrWalletExists        = errors.New("multisig wallet already exists")
	ErrProposalNotFound    = errors.New("proposal not found")
	ErrAlreadyApproved     = errors.New("already approved this proposal")
	ErrAlreadyRejected     = errors.New("already rejected this proposal")
	ErrNotASigner          = errors.New("not a signer of this wallet")
	ErrInvalidThreshold    = errors.New("invalid threshold")
	ErrProposalExpired     = errors.New("proposal has expired")
	ErrProposalNotPending  = errors.New("proposal is not pending")
	ErrInsufficientApprovals = errors.New("insufficient approvals")
)

// ActionType defines the type of multisig action.
type ActionType string

const (
	ActionTypeSend     ActionType = "send"
	ActionTypeAddSigner ActionType = "add_signer"
	ActionTypeRemoveSigner ActionType = "remove_signer"
	ActionTypeChangeThreshold ActionType = "change_threshold"
	ActionTypeCustom   ActionType = "custom"
)

// ProposalStatus defines proposal states.
type ProposalStatus string

const (
	StatusPending   ProposalStatus = "pending"
	StatusApproved  ProposalStatus = "approved"
	StatusRejected  ProposalStatus = "rejected"
	StatusExecuted  ProposalStatus = "executed"
	StatusExpired   ProposalStatus = "expired"
	StatusCancelled ProposalStatus = "cancelled"
)

// Wallet represents a multi-signature wallet.
type Wallet struct {
	ID        int64    `json:"id"`
	Address   string   `json:"address"`
	OrgID     int64    `json:"org_id"`
	Name      string   `json:"name,omitempty"`
	Threshold int      `json:"threshold"`
	Signers   []Signer `json:"signers"`
	IsActive  bool     `json:"is_active"`
	CreatedAt int64    `json:"created_at"`
}

// Signer represents a wallet signer.
type Signer struct {
	UserID   int64  `json:"user_id"`
	Address  string `json:"address,omitempty"`
	Name     string `json:"name,omitempty"`
	AddedAt  int64  `json:"added_at"`
}

// Proposal represents a multisig transaction proposal.
type Proposal struct {
	ID          int64          `json:"id"`
	WalletID    int64          `json:"wallet_id"`
	ProposerID  int64          `json:"proposer_id"`
	ActionType  ActionType     `json:"action_type"`
	ActionData  map[string]any `json:"action_data"`
	Approvals   []Approval     `json:"approvals"`
	Rejections  []Approval     `json:"rejections"`
	Status      ProposalStatus `json:"status"`
	TxHash      string         `json:"tx_hash,omitempty"`
	CreatedAt   int64          `json:"created_at"`
	ExpiresAt   *int64         `json:"expires_at,omitempty"`
	ExecutedAt  *int64         `json:"executed_at,omitempty"`
}

// Approval represents a signer's approval or rejection.
type Approval struct {
	UserID    int64  `json:"user_id"`
	Signature string `json:"signature,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// Manager handles multisig operations.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new multisig manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// CreateWallet creates a new multisig wallet.
func (m *Manager) CreateWallet(orgID int64, address, name string, threshold int, signers []Signer) (*Wallet, error) {
	if threshold < 1 || threshold > len(signers) {
		return nil, ErrInvalidThreshold
	}

	// Check if wallet already exists
	var exists int
	err := m.db.QueryRow("SELECT 1 FROM multisig_wallets WHERE address = ?", address).Scan(&exists)
	if err == nil {
		return nil, ErrWalletExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check wallet: %w", err)
	}

	signersJSON, err := json.Marshal(signers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signers: %w", err)
	}

	now := time.Now().Unix()
	result, err := m.db.Exec(`
		INSERT INTO multisig_wallets (address, org_id, name, threshold, signers, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)`,
		address, orgID, name, threshold, string(signersJSON), now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Wallet{
		ID:        id,
		Address:   address,
		OrgID:     orgID,
		Name:      name,
		Threshold: threshold,
		Signers:   signers,
		IsActive:  true,
		CreatedAt: now,
	}, nil
}

// GetWallet retrieves a wallet by address.
func (m *Manager) GetWallet(address string) (*Wallet, error) {
	var w Wallet
	var name sql.NullString
	var signersJSON string

	err := m.db.QueryRow(`
		SELECT id, address, org_id, name, threshold, signers, is_active, created_at
		FROM multisig_wallets WHERE address = ?`,
		address,
	).Scan(&w.ID, &w.Address, &w.OrgID, &name, &w.Threshold, &signersJSON, &w.IsActive, &w.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	if name.Valid {
		w.Name = name.String
	}
	json.Unmarshal([]byte(signersJSON), &w.Signers)

	return &w, nil
}

// ListWallets lists wallets for an organization.
func (m *Manager) ListWallets(orgID int64) ([]Wallet, error) {
	rows, err := m.db.Query(`
		SELECT id, address, org_id, name, threshold, signers, is_active, created_at
		FROM multisig_wallets WHERE org_id = ? AND is_active = 1
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}
	defer rows.Close()

	var wallets []Wallet
	for rows.Next() {
		var w Wallet
		var name sql.NullString
		var signersJSON string

		err := rows.Scan(&w.ID, &w.Address, &w.OrgID, &name, &w.Threshold, &signersJSON, &w.IsActive, &w.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan wallet: %w", err)
		}

		if name.Valid {
			w.Name = name.String
		}
		json.Unmarshal([]byte(signersJSON), &w.Signers)
		wallets = append(wallets, w)
	}

	return wallets, nil
}

// CreateProposal creates a new proposal.
func (m *Manager) CreateProposal(walletID, proposerID int64, actionType ActionType, actionData map[string]any, expiresIn *time.Duration) (*Proposal, error) {
	// Verify proposer is a signer
	wallet, err := m.getWalletByID(walletID)
	if err != nil {
		return nil, err
	}

	if !m.isSigner(wallet, proposerID) {
		return nil, ErrNotASigner
	}

	actionDataJSON, err := json.Marshal(actionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action data: %w", err)
	}

	now := time.Now().Unix()
	var expiresAt *int64
	if expiresIn != nil {
		exp := now + int64(expiresIn.Seconds())
		expiresAt = &exp
	}

	result, err := m.db.Exec(`
		INSERT INTO multisig_proposals (wallet_id, proposer_id, action_type, action_data, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		walletID, proposerID, actionType, string(actionDataJSON), StatusPending, now, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposal: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Proposal{
		ID:         id,
		WalletID:   walletID,
		ProposerID: proposerID,
		ActionType: actionType,
		ActionData: actionData,
		Approvals:  []Approval{},
		Rejections: []Approval{},
		Status:     StatusPending,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
	}, nil
}

// Approve approves a proposal.
func (m *Manager) Approve(proposalID, userID int64, signature string) error {
	proposal, err := m.GetProposal(proposalID)
	if err != nil {
		return err
	}

	if proposal.Status != StatusPending {
		return ErrProposalNotPending
	}

	if proposal.ExpiresAt != nil && time.Now().Unix() > *proposal.ExpiresAt {
		m.updateProposalStatus(proposalID, StatusExpired)
		return ErrProposalExpired
	}

	// Verify user is a signer
	wallet, err := m.getWalletByID(proposal.WalletID)
	if err != nil {
		return err
	}

	if !m.isSigner(wallet, userID) {
		return ErrNotASigner
	}

	// Check if already approved
	for _, a := range proposal.Approvals {
		if a.UserID == userID {
			return ErrAlreadyApproved
		}
	}

	// Add approval
	proposal.Approvals = append(proposal.Approvals, Approval{
		UserID:    userID,
		Signature: signature,
		Timestamp: time.Now().Unix(),
	})

	approvalsJSON, _ := json.Marshal(proposal.Approvals)
	_, err = m.db.Exec(
		"UPDATE multisig_proposals SET approvals = ? WHERE id = ?",
		string(approvalsJSON), proposalID,
	)
	if err != nil {
		return fmt.Errorf("failed to update approvals: %w", err)
	}

	// Check if threshold reached
	if len(proposal.Approvals) >= wallet.Threshold {
		m.updateProposalStatus(proposalID, StatusApproved)
	}

	return nil
}

// Reject rejects a proposal.
func (m *Manager) Reject(proposalID, userID int64) error {
	proposal, err := m.GetProposal(proposalID)
	if err != nil {
		return err
	}

	if proposal.Status != StatusPending {
		return ErrProposalNotPending
	}

	// Verify user is a signer
	wallet, err := m.getWalletByID(proposal.WalletID)
	if err != nil {
		return err
	}

	if !m.isSigner(wallet, userID) {
		return ErrNotASigner
	}

	// Check if already rejected
	for _, r := range proposal.Rejections {
		if r.UserID == userID {
			return ErrAlreadyRejected
		}
	}

	// Add rejection
	proposal.Rejections = append(proposal.Rejections, Approval{
		UserID:    userID,
		Timestamp: time.Now().Unix(),
	})

	rejectionsJSON, _ := json.Marshal(proposal.Rejections)
	_, err = m.db.Exec(
		"UPDATE multisig_proposals SET rejections = ? WHERE id = ?",
		string(rejectionsJSON), proposalID,
	)
	if err != nil {
		return fmt.Errorf("failed to update rejections: %w", err)
	}

	// Check if majority rejected
	if len(proposal.Rejections) > len(wallet.Signers)-wallet.Threshold {
		m.updateProposalStatus(proposalID, StatusRejected)
	}

	return nil
}

// Execute executes an approved proposal.
func (m *Manager) Execute(proposalID int64, txHash string) error {
	proposal, err := m.GetProposal(proposalID)
	if err != nil {
		return err
	}

	if proposal.Status != StatusApproved {
		if proposal.Status == StatusPending {
			wallet, _ := m.getWalletByID(proposal.WalletID)
			if wallet != nil && len(proposal.Approvals) < wallet.Threshold {
				return ErrInsufficientApprovals
			}
		}
		return ErrProposalNotPending
	}

	now := time.Now().Unix()
	_, err = m.db.Exec(
		"UPDATE multisig_proposals SET status = ?, tx_hash = ?, executed_at = ? WHERE id = ?",
		StatusExecuted, txHash, now, proposalID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute proposal: %w", err)
	}

	return nil
}

// GetProposal retrieves a proposal.
func (m *Manager) GetProposal(id int64) (*Proposal, error) {
	var p Proposal
	var actionDataJSON, approvalsJSON, rejectionsJSON string
	var txHash sql.NullString
	var expiresAt, executedAt sql.NullInt64

	err := m.db.QueryRow(`
		SELECT id, wallet_id, proposer_id, action_type, action_data, approvals, rejections, status, tx_hash, created_at, expires_at, executed_at
		FROM multisig_proposals WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.WalletID, &p.ProposerID, &p.ActionType, &actionDataJSON, &approvalsJSON, &rejectionsJSON, &p.Status, &txHash, &p.CreatedAt, &expiresAt, &executedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProposalNotFound
		}
		return nil, fmt.Errorf("failed to get proposal: %w", err)
	}

	json.Unmarshal([]byte(actionDataJSON), &p.ActionData)
	json.Unmarshal([]byte(approvalsJSON), &p.Approvals)
	json.Unmarshal([]byte(rejectionsJSON), &p.Rejections)

	if p.Approvals == nil {
		p.Approvals = []Approval{}
	}
	if p.Rejections == nil {
		p.Rejections = []Approval{}
	}

	if txHash.Valid {
		p.TxHash = txHash.String
	}
	if expiresAt.Valid {
		p.ExpiresAt = &expiresAt.Int64
	}
	if executedAt.Valid {
		p.ExecutedAt = &executedAt.Int64
	}

	return &p, nil
}

// ListProposals lists proposals for a wallet.
func (m *Manager) ListProposals(walletID int64, status *ProposalStatus) ([]Proposal, error) {
	query := "SELECT id, wallet_id, proposer_id, action_type, action_data, approvals, rejections, status, tx_hash, created_at, expires_at, executed_at FROM multisig_proposals WHERE wallet_id = ?"
	args := []any{walletID}

	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list proposals: %w", err)
	}
	defer rows.Close()

	var proposals []Proposal
	for rows.Next() {
		var p Proposal
		var actionDataJSON, approvalsJSON, rejectionsJSON string
		var txHash sql.NullString
		var expiresAt, executedAt sql.NullInt64

		err := rows.Scan(&p.ID, &p.WalletID, &p.ProposerID, &p.ActionType, &actionDataJSON, &approvalsJSON, &rejectionsJSON, &p.Status, &txHash, &p.CreatedAt, &expiresAt, &executedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan proposal: %w", err)
		}

		json.Unmarshal([]byte(actionDataJSON), &p.ActionData)
		json.Unmarshal([]byte(approvalsJSON), &p.Approvals)
		json.Unmarshal([]byte(rejectionsJSON), &p.Rejections)

		if p.Approvals == nil {
			p.Approvals = []Approval{}
		}
		if p.Rejections == nil {
			p.Rejections = []Approval{}
		}

		if txHash.Valid {
			p.TxHash = txHash.String
		}
		if expiresAt.Valid {
			p.ExpiresAt = &expiresAt.Int64
		}
		if executedAt.Valid {
			p.ExecutedAt = &executedAt.Int64
		}

		proposals = append(proposals, p)
	}

	return proposals, nil
}

// getWalletByID retrieves a wallet by ID.
func (m *Manager) getWalletByID(id int64) (*Wallet, error) {
	var w Wallet
	var name sql.NullString
	var signersJSON string

	err := m.db.QueryRow(`
		SELECT id, address, org_id, name, threshold, signers, is_active, created_at
		FROM multisig_wallets WHERE id = ?`,
		id,
	).Scan(&w.ID, &w.Address, &w.OrgID, &name, &w.Threshold, &signersJSON, &w.IsActive, &w.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	if name.Valid {
		w.Name = name.String
	}
	json.Unmarshal([]byte(signersJSON), &w.Signers)

	return &w, nil
}

// isSigner checks if a user is a signer.
func (m *Manager) isSigner(wallet *Wallet, userID int64) bool {
	for _, s := range wallet.Signers {
		if s.UserID == userID {
			return true
		}
	}
	return false
}

// updateProposalStatus updates a proposal's status.
func (m *Manager) updateProposalStatus(id int64, status ProposalStatus) error {
	_, err := m.db.Exec("UPDATE multisig_proposals SET status = ? WHERE id = ?", status, id)
	return err
}

// ExpireProposals marks expired proposals.
func (m *Manager) ExpireProposals() (int64, error) {
	now := time.Now().Unix()
	result, err := m.db.Exec(
		"UPDATE multisig_proposals SET status = ? WHERE status = ? AND expires_at IS NOT NULL AND expires_at < ?",
		StatusExpired, StatusPending, now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to expire proposals: %w", err)
	}
	return result.RowsAffected()
}
