// Package jetton provides Jetton (fungible token) support for TON blockchain.
// It handles token creation, minting, transfers, and management.
package jetton

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Standard errors for Jetton operations.
var (
	ErrJettonNotFound      = errors.New("jetton not found")
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrNotOwner            = errors.New("not the owner")
	ErrNotMinter           = errors.New("not authorized to mint")
	ErrMintingDisabled     = errors.New("minting is disabled")
	ErrBurnDisabled        = errors.New("burning is disabled")
	ErrTransferDisabled    = errors.New("transfers are disabled")
	ErrInvalidAmount       = errors.New("invalid amount")
)

// JettonType represents the type of Jetton.
type JettonType string

const (
	JettonTypeStandard   JettonType = "standard"
	JettonTypeMintable   JettonType = "mintable"
	JettonTypeBurnable   JettonType = "burnable"
	JettonTypeFreezable  JettonType = "freezable"
	JettonTypeGovernance JettonType = "governance"
)

// Jetton represents a Jetton (fungible token) on TON.
type Jetton struct {
	ID                int64           `json:"id"`
	OwnerID           int64           `json:"owner_id"`
	OrgID             *int64          `json:"org_id,omitempty"`
	Address           string          `json:"address"`
	Name              string          `json:"name"`
	Symbol            string          `json:"symbol"`
	Description       string          `json:"description,omitempty"`
	ImageURL          string          `json:"image_url,omitempty"`
	Decimals          int             `json:"decimals"`
	TotalSupply       string          `json:"total_supply"`
	MaxSupply         *string         `json:"max_supply,omitempty"`
	Type              JettonType      `json:"type"`
	MintingEnabled    bool            `json:"minting_enabled"`
	BurningEnabled    bool            `json:"burning_enabled"`
	TransfersEnabled  bool            `json:"transfers_enabled"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	ContractCode      string          `json:"contract_code,omitempty"`
	AdminAddress      string          `json:"admin_address,omitempty"`
	DeployedAt        *time.Time      `json:"deployed_at,omitempty"`
	DeployTxHash      string          `json:"deploy_tx_hash,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// JettonWallet represents a user's wallet for a specific Jetton.
type JettonWallet struct {
	ID          int64      `json:"id"`
	JettonID    int64      `json:"jetton_id"`
	UserID      int64      `json:"user_id"`
	Address     string     `json:"address"`
	Balance     string     `json:"balance"`
	IsFrozen    bool       `json:"is_frozen"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Transfer represents a Jetton transfer.
type Transfer struct {
	ID          int64     `json:"id"`
	JettonID    int64     `json:"jetton_id"`
	FromWallet  int64     `json:"from_wallet"`
	ToWallet    int64     `json:"to_wallet"`
	Amount      string    `json:"amount"`
	TxHash      string    `json:"tx_hash"`
	Status      string    `json:"status"`
	Comment     string    `json:"comment,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// MintRecord represents a minting event.
type MintRecord struct {
	ID        int64     `json:"id"`
	JettonID  int64     `json:"jetton_id"`
	ToWallet  int64     `json:"to_wallet"`
	Amount    string    `json:"amount"`
	TxHash    string    `json:"tx_hash"`
	MinterID  int64     `json:"minter_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BurnRecord represents a burning event.
type BurnRecord struct {
	ID         int64     `json:"id"`
	JettonID   int64     `json:"jetton_id"`
	FromWallet int64     `json:"from_wallet"`
	Amount     string    `json:"amount"`
	TxHash     string    `json:"tx_hash"`
	BurnerID   int64     `json:"burner_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateJettonRequest represents a request to create a new Jetton.
type CreateJettonRequest struct {
	Name             string          `json:"name"`
	Symbol           string          `json:"symbol"`
	Description      string          `json:"description,omitempty"`
	ImageURL         string          `json:"image_url,omitempty"`
	Decimals         int             `json:"decimals"`
	InitialSupply    string          `json:"initial_supply"`
	MaxSupply        *string         `json:"max_supply,omitempty"`
	Type             JettonType      `json:"type"`
	MintingEnabled   bool            `json:"minting_enabled"`
	BurningEnabled   bool            `json:"burning_enabled"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
}

// TransferRequest represents a Jetton transfer request.
type TransferRequest struct {
	JettonID  int64  `json:"jetton_id"`
	ToAddress string `json:"to_address"`
	ToUserID  *int64 `json:"to_user_id,omitempty"`
	Amount    string `json:"amount"`
	Comment   string `json:"comment,omitempty"`
}

// MintRequest represents a minting request.
type MintRequest struct {
	JettonID  int64  `json:"jetton_id"`
	ToAddress string `json:"to_address"`
	ToUserID  *int64 `json:"to_user_id,omitempty"`
	Amount    string `json:"amount"`
}

// BurnRequest represents a burning request.
type BurnRequest struct {
	JettonID int64  `json:"jetton_id"`
	Amount   string `json:"amount"`
}

// Manager handles Jetton operations.
type Manager struct {
	db       *sql.DB
	provider JettonProvider
}

// JettonProvider defines the interface for blockchain Jetton operations.
type JettonProvider interface {
	DeployJetton(ctx context.Context, jetton *Jetton, initialSupply *big.Int) (string, string, error)
	MintJetton(ctx context.Context, jetton *Jetton, toAddress string, amount *big.Int) (string, error)
	BurnJetton(ctx context.Context, jetton *Jetton, fromAddress string, amount *big.Int) (string, error)
	TransferJetton(ctx context.Context, jetton *Jetton, fromAddress, toAddress string, amount *big.Int) (string, error)
	GetJettonData(ctx context.Context, address string) (*Jetton, error)
	GetWalletBalance(ctx context.Context, jettonAddress, walletAddress string) (*big.Int, error)
}

// NewManager creates a new Jetton manager.
func NewManager(db *sql.DB, provider JettonProvider) *Manager {
	m := &Manager{
		db:       db,
		provider: provider,
	}
	m.initSchema()
	return m
}

// initSchema creates the required database tables.
func (m *Manager) initSchema() {
	schema := `
	CREATE TABLE IF NOT EXISTS jettons (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id INTEGER NOT NULL,
		org_id INTEGER,
		address TEXT UNIQUE,
		name TEXT NOT NULL,
		symbol TEXT NOT NULL,
		description TEXT,
		image_url TEXT,
		decimals INTEGER DEFAULT 9,
		total_supply TEXT NOT NULL,
		max_supply TEXT,
		type TEXT DEFAULT 'standard',
		minting_enabled INTEGER DEFAULT 1,
		burning_enabled INTEGER DEFAULT 1,
		transfers_enabled INTEGER DEFAULT 1,
		metadata TEXT,
		contract_code TEXT,
		admin_address TEXT,
		deployed_at DATETIME,
		deploy_tx_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS jetton_wallets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		jetton_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		address TEXT,
		balance TEXT DEFAULT '0',
		is_frozen INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (jetton_id) REFERENCES jettons(id),
		UNIQUE(jetton_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS jetton_transfers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		jetton_id INTEGER NOT NULL,
		from_wallet INTEGER NOT NULL,
		to_wallet INTEGER NOT NULL,
		amount TEXT NOT NULL,
		tx_hash TEXT,
		status TEXT DEFAULT 'completed',
		comment TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (jetton_id) REFERENCES jettons(id)
	);

	CREATE TABLE IF NOT EXISTS jetton_mints (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		jetton_id INTEGER NOT NULL,
		to_wallet INTEGER NOT NULL,
		amount TEXT NOT NULL,
		tx_hash TEXT,
		minter_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (jetton_id) REFERENCES jettons(id)
	);

	CREATE TABLE IF NOT EXISTS jetton_burns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		jetton_id INTEGER NOT NULL,
		from_wallet INTEGER NOT NULL,
		amount TEXT NOT NULL,
		tx_hash TEXT,
		burner_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (jetton_id) REFERENCES jettons(id)
	);

	CREATE INDEX IF NOT EXISTS idx_jetton_wallets_jetton ON jetton_wallets(jetton_id);
	CREATE INDEX IF NOT EXISTS idx_jetton_wallets_user ON jetton_wallets(user_id);
	CREATE INDEX IF NOT EXISTS idx_jetton_transfers_jetton ON jetton_transfers(jetton_id);
	CREATE INDEX IF NOT EXISTS idx_jettons_symbol ON jettons(symbol);
	`
	m.db.Exec(schema)
}

// Create creates a new Jetton.
func (m *Manager) Create(ctx context.Context, userID int64, orgID *int64, req *CreateJettonRequest) (*Jetton, error) {
	if req.Decimals <= 0 {
		req.Decimals = 9 // TON default
	}
	if req.Type == "" {
		req.Type = JettonTypeStandard
	}

	initialSupply, ok := new(big.Int).SetString(req.InitialSupply, 10)
	if !ok || initialSupply.Sign() < 0 {
		return nil, ErrInvalidAmount
	}

	if req.MaxSupply != nil {
		maxSupply, ok := new(big.Int).SetString(*req.MaxSupply, 10)
		if !ok || maxSupply.Sign() < 0 {
			return nil, ErrInvalidAmount
		}
		if initialSupply.Cmp(maxSupply) > 0 {
			return nil, errors.New("initial supply cannot exceed max supply")
		}
	}

	metadataJSON, _ := json.Marshal(req.Metadata)

	jetton := &Jetton{
		OwnerID:          userID,
		OrgID:            orgID,
		Name:             req.Name,
		Symbol:           req.Symbol,
		Description:      req.Description,
		ImageURL:         req.ImageURL,
		Decimals:         req.Decimals,
		TotalSupply:      req.InitialSupply,
		MaxSupply:        req.MaxSupply,
		Type:             req.Type,
		MintingEnabled:   req.MintingEnabled,
		BurningEnabled:   req.BurningEnabled,
		TransfersEnabled: true,
		Metadata:         metadataJSON,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO jettons (owner_id, org_id, name, symbol, description, image_url, decimals,
			total_supply, max_supply, type, minting_enabled, burning_enabled, transfers_enabled,
			metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jetton.OwnerID, jetton.OrgID, jetton.Name, jetton.Symbol, jetton.Description,
		jetton.ImageURL, jetton.Decimals, jetton.TotalSupply, jetton.MaxSupply, jetton.Type,
		jetton.MintingEnabled, jetton.BurningEnabled, jetton.TransfersEnabled,
		string(metadataJSON), jetton.CreatedAt, jetton.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create jetton: %w", err)
	}

	jetton.ID, _ = res.LastInsertId()

	// Create owner's wallet with initial supply
	_, err = m.getOrCreateWallet(ctx, jetton.ID, userID)
	if err != nil {
		return nil, err
	}

	// Credit initial supply
	if initialSupply.Sign() > 0 {
		m.db.ExecContext(ctx, `UPDATE jetton_wallets SET balance = ? WHERE jetton_id = ? AND user_id = ?`,
			req.InitialSupply, jetton.ID, userID)
	}

	return jetton, nil
}

// Deploy deploys the Jetton contract to the blockchain.
func (m *Manager) Deploy(ctx context.Context, userID int64, jettonID int64) (*Jetton, error) {
	jetton, err := m.Get(ctx, jettonID)
	if err != nil {
		return nil, err
	}

	if jetton.OwnerID != userID {
		return nil, ErrNotOwner
	}

	totalSupply, _ := new(big.Int).SetString(jetton.TotalSupply, 10)

	if m.provider != nil {
		address, txHash, err := m.provider.DeployJetton(ctx, jetton, totalSupply)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy jetton: %w", err)
		}
		jetton.Address = address
		jetton.DeployTxHash = txHash
	} else {
		// Mock deployment
		jetton.Address = fmt.Sprintf("EQJetton_%d_%d", jettonID, time.Now().Unix())
		jetton.DeployTxHash = fmt.Sprintf("tx_deploy_%d", time.Now().UnixNano())
	}

	now := time.Now()
	jetton.DeployedAt = &now
	jetton.AdminAddress = jetton.Address

	_, err = m.db.ExecContext(ctx, `
		UPDATE jettons SET address = ?, admin_address = ?, deployed_at = ?, deploy_tx_hash = ?, updated_at = ?
		WHERE id = ?`,
		jetton.Address, jetton.AdminAddress, jetton.DeployedAt, jetton.DeployTxHash, now, jettonID)
	if err != nil {
		return nil, err
	}

	return jetton, nil
}

// Get retrieves a Jetton by ID.
func (m *Manager) Get(ctx context.Context, id int64) (*Jetton, error) {
	var jetton Jetton
	var metadataStr, maxSupply sql.NullString
	var deployedAt sql.NullTime

	err := m.db.QueryRowContext(ctx, `
		SELECT id, owner_id, org_id, address, name, symbol, description, image_url, decimals,
			total_supply, max_supply, type, minting_enabled, burning_enabled, transfers_enabled,
			metadata, admin_address, deployed_at, deploy_tx_hash, created_at, updated_at
		FROM jettons WHERE id = ?`, id).Scan(
		&jetton.ID, &jetton.OwnerID, &jetton.OrgID, &jetton.Address, &jetton.Name, &jetton.Symbol,
		&jetton.Description, &jetton.ImageURL, &jetton.Decimals, &jetton.TotalSupply, &maxSupply,
		&jetton.Type, &jetton.MintingEnabled, &jetton.BurningEnabled, &jetton.TransfersEnabled,
		&metadataStr, &jetton.AdminAddress, &deployedAt, &jetton.DeployTxHash,
		&jetton.CreatedAt, &jetton.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrJettonNotFound
	}
	if err != nil {
		return nil, err
	}

	if metadataStr.Valid {
		jetton.Metadata = json.RawMessage(metadataStr.String)
	}
	if maxSupply.Valid {
		jetton.MaxSupply = &maxSupply.String
	}
	if deployedAt.Valid {
		jetton.DeployedAt = &deployedAt.Time
	}

	return &jetton, nil
}

// GetBySymbol retrieves a Jetton by symbol.
func (m *Manager) GetBySymbol(ctx context.Context, symbol string) (*Jetton, error) {
	var id int64
	err := m.db.QueryRowContext(ctx, `SELECT id FROM jettons WHERE symbol = ?`, symbol).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, ErrJettonNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.Get(ctx, id)
}

// List lists Jettons with optional filters.
func (m *Manager) List(ctx context.Context, userID *int64, orgID *int64, limit, offset int) ([]*Jetton, int, error) {
	var args []interface{}
	query := `SELECT id, owner_id, address, name, symbol, decimals, total_supply, type, deployed_at, created_at
		FROM jettons WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM jettons WHERE 1=1`

	if orgID != nil {
		query += ` AND org_id = ?`
		countQuery += ` AND org_id = ?`
		args = append(args, *orgID)
	} else if userID != nil {
		query += ` AND owner_id = ?`
		countQuery += ` AND owner_id = ?`
		args = append(args, *userID)
	}

	var total int
	m.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jettons []*Jetton
	for rows.Next() {
		var j Jetton
		var deployedAt sql.NullTime
		err := rows.Scan(&j.ID, &j.OwnerID, &j.Address, &j.Name, &j.Symbol, &j.Decimals,
			&j.TotalSupply, &j.Type, &deployedAt, &j.CreatedAt)
		if err != nil {
			continue
		}
		if deployedAt.Valid {
			j.DeployedAt = &deployedAt.Time
		}
		jettons = append(jettons, &j)
	}

	return jettons, total, nil
}

// getOrCreateWallet gets or creates a wallet for a user.
func (m *Manager) getOrCreateWallet(ctx context.Context, jettonID, userID int64) (*JettonWallet, error) {
	var wallet JettonWallet

	err := m.db.QueryRowContext(ctx, `
		SELECT id, jetton_id, user_id, address, balance, is_frozen, created_at, updated_at
		FROM jetton_wallets WHERE jetton_id = ? AND user_id = ?`, jettonID, userID).Scan(
		&wallet.ID, &wallet.JettonID, &wallet.UserID, &wallet.Address, &wallet.Balance,
		&wallet.IsFrozen, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err == nil {
		return &wallet, nil
	}

	// Create new wallet
	wallet = JettonWallet{
		JettonID:  jettonID,
		UserID:    userID,
		Address:   fmt.Sprintf("EQJettonWallet_%d_%d", jettonID, userID),
		Balance:   "0",
		IsFrozen:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO jetton_wallets (jetton_id, user_id, address, balance, is_frozen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		wallet.JettonID, wallet.UserID, wallet.Address, wallet.Balance, wallet.IsFrozen,
		wallet.CreatedAt, wallet.UpdatedAt)
	if err != nil {
		return nil, err
	}

	wallet.ID, _ = res.LastInsertId()
	return &wallet, nil
}

// GetWallet retrieves a user's wallet for a Jetton.
func (m *Manager) GetWallet(ctx context.Context, jettonID, userID int64) (*JettonWallet, error) {
	return m.getOrCreateWallet(ctx, jettonID, userID)
}

// GetBalance retrieves a user's balance for a Jetton.
func (m *Manager) GetBalance(ctx context.Context, jettonID, userID int64) (string, error) {
	wallet, err := m.getOrCreateWallet(ctx, jettonID, userID)
	if err != nil {
		return "0", err
	}
	return wallet.Balance, nil
}

// Transfer transfers Jettons between users.
func (m *Manager) Transfer(ctx context.Context, fromUserID int64, req *TransferRequest) (*Transfer, error) {
	jetton, err := m.Get(ctx, req.JettonID)
	if err != nil {
		return nil, err
	}

	if !jetton.TransfersEnabled {
		return nil, ErrTransferDisabled
	}

	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	fromWallet, err := m.getOrCreateWallet(ctx, req.JettonID, fromUserID)
	if err != nil {
		return nil, err
	}

	if fromWallet.IsFrozen {
		return nil, errors.New("wallet is frozen")
	}

	fromBalance, _ := new(big.Int).SetString(fromWallet.Balance, 10)
	if fromBalance.Cmp(amount) < 0 {
		return nil, ErrInsufficientBalance
	}

	// Determine recipient
	var toUserID int64
	if req.ToUserID != nil {
		toUserID = *req.ToUserID
	} else {
		// In a real implementation, you'd look up the user by address
		return nil, errors.New("to_user_id is required")
	}

	toWallet, err := m.getOrCreateWallet(ctx, req.JettonID, toUserID)
	if err != nil {
		return nil, err
	}

	// Execute transfer
	txHash := fmt.Sprintf("tx_transfer_%d_%d", time.Now().UnixNano(), req.JettonID)

	newFromBalance := new(big.Int).Sub(fromBalance, amount)
	toBalance, _ := new(big.Int).SetString(toWallet.Balance, 10)
	newToBalance := new(big.Int).Add(toBalance, amount)

	now := time.Now()

	// Update balances
	_, err = m.db.ExecContext(ctx, `UPDATE jetton_wallets SET balance = ?, updated_at = ? WHERE id = ?`,
		newFromBalance.String(), now, fromWallet.ID)
	if err != nil {
		return nil, err
	}

	_, err = m.db.ExecContext(ctx, `UPDATE jetton_wallets SET balance = ?, updated_at = ? WHERE id = ?`,
		newToBalance.String(), now, toWallet.ID)
	if err != nil {
		return nil, err
	}

	// Record transfer
	transfer := &Transfer{
		JettonID:   req.JettonID,
		FromWallet: fromWallet.ID,
		ToWallet:   toWallet.ID,
		Amount:     req.Amount,
		TxHash:     txHash,
		Status:     "completed",
		Comment:    req.Comment,
		CreatedAt:  now,
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO jetton_transfers (jetton_id, from_wallet, to_wallet, amount, tx_hash, status, comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		transfer.JettonID, transfer.FromWallet, transfer.ToWallet, transfer.Amount,
		transfer.TxHash, transfer.Status, transfer.Comment, transfer.CreatedAt)
	if err != nil {
		return nil, err
	}

	transfer.ID, _ = res.LastInsertId()
	return transfer, nil
}

// Mint mints new Jettons.
func (m *Manager) Mint(ctx context.Context, minterID int64, req *MintRequest) (*MintRecord, error) {
	jetton, err := m.Get(ctx, req.JettonID)
	if err != nil {
		return nil, err
	}

	if jetton.OwnerID != minterID {
		return nil, ErrNotMinter
	}

	if !jetton.MintingEnabled {
		return nil, ErrMintingDisabled
	}

	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	// Check max supply
	if jetton.MaxSupply != nil {
		maxSupply, _ := new(big.Int).SetString(*jetton.MaxSupply, 10)
		totalSupply, _ := new(big.Int).SetString(jetton.TotalSupply, 10)
		newTotal := new(big.Int).Add(totalSupply, amount)
		if newTotal.Cmp(maxSupply) > 0 {
			return nil, errors.New("minting would exceed max supply")
		}
	}

	// Determine recipient
	var toUserID int64
	if req.ToUserID != nil {
		toUserID = *req.ToUserID
	} else {
		toUserID = minterID
	}

	toWallet, err := m.getOrCreateWallet(ctx, req.JettonID, toUserID)
	if err != nil {
		return nil, err
	}

	txHash := fmt.Sprintf("tx_mint_%d_%d", time.Now().UnixNano(), req.JettonID)
	now := time.Now()

	// Update balance
	toBalance, _ := new(big.Int).SetString(toWallet.Balance, 10)
	newBalance := new(big.Int).Add(toBalance, amount)

	_, err = m.db.ExecContext(ctx, `UPDATE jetton_wallets SET balance = ?, updated_at = ? WHERE id = ?`,
		newBalance.String(), now, toWallet.ID)
	if err != nil {
		return nil, err
	}

	// Update total supply
	totalSupply, _ := new(big.Int).SetString(jetton.TotalSupply, 10)
	newTotalSupply := new(big.Int).Add(totalSupply, amount)

	_, err = m.db.ExecContext(ctx, `UPDATE jettons SET total_supply = ?, updated_at = ? WHERE id = ?`,
		newTotalSupply.String(), now, req.JettonID)
	if err != nil {
		return nil, err
	}

	// Record mint
	mint := &MintRecord{
		JettonID:  req.JettonID,
		ToWallet:  toWallet.ID,
		Amount:    req.Amount,
		TxHash:    txHash,
		MinterID:  minterID,
		CreatedAt: now,
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO jetton_mints (jetton_id, to_wallet, amount, tx_hash, minter_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		mint.JettonID, mint.ToWallet, mint.Amount, mint.TxHash, mint.MinterID, mint.CreatedAt)
	if err != nil {
		return nil, err
	}

	mint.ID, _ = res.LastInsertId()
	return mint, nil
}

// Burn burns Jettons.
func (m *Manager) Burn(ctx context.Context, burnerID int64, req *BurnRequest) (*BurnRecord, error) {
	jetton, err := m.Get(ctx, req.JettonID)
	if err != nil {
		return nil, err
	}

	if !jetton.BurningEnabled {
		return nil, ErrBurnDisabled
	}

	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	fromWallet, err := m.getOrCreateWallet(ctx, req.JettonID, burnerID)
	if err != nil {
		return nil, err
	}

	fromBalance, _ := new(big.Int).SetString(fromWallet.Balance, 10)
	if fromBalance.Cmp(amount) < 0 {
		return nil, ErrInsufficientBalance
	}

	txHash := fmt.Sprintf("tx_burn_%d_%d", time.Now().UnixNano(), req.JettonID)
	now := time.Now()

	// Update balance
	newBalance := new(big.Int).Sub(fromBalance, amount)

	_, err = m.db.ExecContext(ctx, `UPDATE jetton_wallets SET balance = ?, updated_at = ? WHERE id = ?`,
		newBalance.String(), now, fromWallet.ID)
	if err != nil {
		return nil, err
	}

	// Update total supply
	totalSupply, _ := new(big.Int).SetString(jetton.TotalSupply, 10)
	newTotalSupply := new(big.Int).Sub(totalSupply, amount)

	_, err = m.db.ExecContext(ctx, `UPDATE jettons SET total_supply = ?, updated_at = ? WHERE id = ?`,
		newTotalSupply.String(), now, req.JettonID)
	if err != nil {
		return nil, err
	}

	// Record burn
	burn := &BurnRecord{
		JettonID:   req.JettonID,
		FromWallet: fromWallet.ID,
		Amount:     req.Amount,
		TxHash:     txHash,
		BurnerID:   burnerID,
		CreatedAt:  now,
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO jetton_burns (jetton_id, from_wallet, amount, tx_hash, burner_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		burn.JettonID, burn.FromWallet, burn.Amount, burn.TxHash, burn.BurnerID, burn.CreatedAt)
	if err != nil {
		return nil, err
	}

	burn.ID, _ = res.LastInsertId()
	return burn, nil
}

// GetTransferHistory retrieves transfer history for a Jetton or wallet.
func (m *Manager) GetTransferHistory(ctx context.Context, jettonID int64, walletID *int64, limit, offset int) ([]*Transfer, int, error) {
	var args []interface{}
	query := `SELECT id, jetton_id, from_wallet, to_wallet, amount, tx_hash, status, comment, created_at
		FROM jetton_transfers WHERE jetton_id = ?`
	countQuery := `SELECT COUNT(*) FROM jetton_transfers WHERE jetton_id = ?`
	args = append(args, jettonID)

	if walletID != nil {
		query += ` AND (from_wallet = ? OR to_wallet = ?)`
		countQuery += ` AND (from_wallet = ? OR to_wallet = ?)`
		args = append(args, *walletID, *walletID)
	}

	var total int
	m.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transfers []*Transfer
	for rows.Next() {
		var t Transfer
		err := rows.Scan(&t.ID, &t.JettonID, &t.FromWallet, &t.ToWallet, &t.Amount,
			&t.TxHash, &t.Status, &t.Comment, &t.CreatedAt)
		if err != nil {
			continue
		}
		transfers = append(transfers, &t)
	}

	return transfers, total, nil
}

// FreezeWallet freezes a user's wallet.
func (m *Manager) FreezeWallet(ctx context.Context, adminID int64, jettonID, userID int64) error {
	jetton, err := m.Get(ctx, jettonID)
	if err != nil {
		return err
	}

	if jetton.OwnerID != adminID {
		return ErrNotOwner
	}

	if jetton.Type != JettonTypeFreezable {
		return errors.New("jetton does not support freezing")
	}

	_, err = m.db.ExecContext(ctx, `UPDATE jetton_wallets SET is_frozen = 1, updated_at = ? WHERE jetton_id = ? AND user_id = ?`,
		time.Now(), jettonID, userID)
	return err
}

// UnfreezeWallet unfreezes a user's wallet.
func (m *Manager) UnfreezeWallet(ctx context.Context, adminID int64, jettonID, userID int64) error {
	jetton, err := m.Get(ctx, jettonID)
	if err != nil {
		return err
	}

	if jetton.OwnerID != adminID {
		return ErrNotOwner
	}

	_, err = m.db.ExecContext(ctx, `UPDATE jetton_wallets SET is_frozen = 0, updated_at = ? WHERE jetton_id = ? AND user_id = ?`,
		time.Now(), jettonID, userID)
	return err
}

// UpdateSettings updates Jetton settings.
func (m *Manager) UpdateSettings(ctx context.Context, ownerID int64, jettonID int64, mintingEnabled, burningEnabled, transfersEnabled *bool) error {
	jetton, err := m.Get(ctx, jettonID)
	if err != nil {
		return err
	}

	if jetton.OwnerID != ownerID {
		return ErrNotOwner
	}

	if mintingEnabled != nil {
		jetton.MintingEnabled = *mintingEnabled
	}
	if burningEnabled != nil {
		jetton.BurningEnabled = *burningEnabled
	}
	if transfersEnabled != nil {
		jetton.TransfersEnabled = *transfersEnabled
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE jettons SET minting_enabled = ?, burning_enabled = ?, transfers_enabled = ?, updated_at = ?
		WHERE id = ?`,
		jetton.MintingEnabled, jetton.BurningEnabled, jetton.TransfersEnabled, time.Now(), jettonID)
	return err
}

// GetHolders retrieves all holders of a Jetton.
func (m *Manager) GetHolders(ctx context.Context, jettonID int64, limit, offset int) ([]*JettonWallet, int, error) {
	var total int
	m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jetton_wallets WHERE jetton_id = ? AND balance != '0'`, jettonID).Scan(&total)

	rows, err := m.db.QueryContext(ctx, `
		SELECT id, jetton_id, user_id, address, balance, is_frozen, created_at, updated_at
		FROM jetton_wallets WHERE jetton_id = ? AND balance != '0'
		ORDER BY CAST(balance AS INTEGER) DESC LIMIT ? OFFSET ?`, jettonID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var wallets []*JettonWallet
	for rows.Next() {
		var w JettonWallet
		err := rows.Scan(&w.ID, &w.JettonID, &w.UserID, &w.Address, &w.Balance,
			&w.IsFrozen, &w.CreatedAt, &w.UpdatedAt)
		if err != nil {
			continue
		}
		wallets = append(wallets, &w)
	}

	return wallets, total, nil
}
