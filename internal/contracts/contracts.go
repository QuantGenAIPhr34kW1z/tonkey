// Package contracts provides smart contract interaction functionality.
package contracts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrContractNotFound  = errors.New("contract not found")
	ErrContractExists    = errors.New("contract already registered")
	ErrInvalidABI        = errors.New("invalid contract ABI")
	ErrMethodNotFound    = errors.New("method not found in ABI")
	ErrInvalidParams     = errors.New("invalid method parameters")
	ErrDeployFailed      = errors.New("contract deployment failed")
	ErrCallFailed        = errors.New("contract call failed")
)

// ContractType defines common contract types.
type ContractType string

const (
	ContractTypeWallet   ContractType = "wallet"
	ContractTypeJetton   ContractType = "jetton"
	ContractTypeNFT      ContractType = "nft"
	ContractTypeCustom   ContractType = "custom"
)

// Contract represents a registered smart contract.
type Contract struct {
	ID           int64        `json:"id"`
	Address      string       `json:"address"`
	UserID       int64        `json:"user_id"`
	OrgID        *int64       `json:"org_id,omitempty"`
	Name         string       `json:"name,omitempty"`
	ContractType ContractType `json:"contract_type,omitempty"`
	ABI          *ABI         `json:"abi,omitempty"`
	State        string       `json:"state,omitempty"`
	DeployedAt   *int64       `json:"deployed_at,omitempty"`
	CreatedAt    int64        `json:"created_at"`
}

// ABI represents a contract's Application Binary Interface.
type ABI struct {
	Name       string   `json:"name"`
	Version    string   `json:"version,omitempty"`
	GetMethods []Method `json:"get_methods,omitempty"`
	Messages   []Method `json:"messages,omitempty"`
}

// Method represents a contract method.
type Method struct {
	Name    string  `json:"name"`
	Inputs  []Param `json:"inputs,omitempty"`
	Outputs []Param `json:"outputs,omitempty"`
}

// Param represents a method parameter.
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DeployRequest represents a contract deployment request.
type DeployRequest struct {
	Code       []byte `json:"code"`
	Data       []byte `json:"data,omitempty"`
	Amount     int64  `json:"amount"` // Initial balance
	Name       string `json:"name,omitempty"`
	ContractType ContractType `json:"contract_type,omitempty"`
	ABI        *ABI   `json:"abi,omitempty"`
}

// DeployResult represents the result of a deployment.
type DeployResult struct {
	Address   string `json:"address"`
	TxHash    string `json:"tx_hash"`
	Contract  *Contract `json:"contract"`
}

// CallRequest represents a contract call request.
type CallRequest struct {
	Address string `json:"address"`
	Method  string `json:"method"`
	Params  []any  `json:"params,omitempty"`
	Amount  int64  `json:"amount,omitempty"` // For send methods
}

// CallResult represents the result of a contract call.
type CallResult struct {
	TxHash   string `json:"tx_hash,omitempty"`
	GasUsed  int64  `json:"gas_used"`
	ExitCode int    `json:"exit_code"`
	Result   []any  `json:"result,omitempty"`
}

// StateResult represents contract state.
type StateResult struct {
	Address  string         `json:"address"`
	Balance  int64          `json:"balance"`
	State    string         `json:"state"`
	Code     string         `json:"code,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	LastTx   string         `json:"last_tx,omitempty"`
}

// Provider interface for blockchain operations.
type Provider interface {
	GetAccountState(ctx context.Context, addr string) (balance int64, state string, err error)
	RunGetMethod(ctx context.Context, addr, method string, params []any) ([]any, int64, int, error)
	SendBOC(ctx context.Context, boc []byte) (string, error)
}

// Manager handles contract operations.
type Manager struct {
	db       *sql.DB
	provider Provider
}

// NewManager creates a new contract manager.
func NewManager(db *sql.DB, provider Provider) *Manager {
	return &Manager{
		db:       db,
		provider: provider,
	}
}

// Register registers an existing contract.
func (m *Manager) Register(userID int64, orgID *int64, address, name string, contractType ContractType, abi *ABI) (*Contract, error) {
	// Check if already registered
	var exists int
	err := m.db.QueryRow("SELECT 1 FROM contracts WHERE address = ?", address).Scan(&exists)
	if err == nil {
		return nil, ErrContractExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check contract: %w", err)
	}

	var abiJSON []byte
	if abi != nil {
		abiJSON, err = json.Marshal(abi)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ABI: %w", err)
		}
	}

	now := time.Now().Unix()
	result, err := m.db.Exec(`
		INSERT INTO contracts (address, user_id, org_id, name, contract_type, abi, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		address, userID, orgID, name, contractType, string(abiJSON), now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register contract: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Contract{
		ID:           id,
		Address:      address,
		UserID:       userID,
		OrgID:        orgID,
		Name:         name,
		ContractType: contractType,
		ABI:          abi,
		CreatedAt:    now,
	}, nil
}

// Get retrieves a contract by address.
func (m *Manager) Get(address string) (*Contract, error) {
	var c Contract
	var orgID sql.NullInt64
	var name, contractType, abiJSON, state sql.NullString
	var deployedAt sql.NullInt64

	err := m.db.QueryRow(`
		SELECT id, address, user_id, org_id, name, contract_type, abi, state, deployed_at, created_at
		FROM contracts WHERE address = ?`,
		address,
	).Scan(&c.ID, &c.Address, &c.UserID, &orgID, &name, &contractType, &abiJSON, &state, &deployedAt, &c.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrContractNotFound
		}
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}

	if orgID.Valid {
		c.OrgID = &orgID.Int64
	}
	if name.Valid {
		c.Name = name.String
	}
	if contractType.Valid {
		c.ContractType = ContractType(contractType.String)
	}
	if state.Valid {
		c.State = state.String
	}
	if deployedAt.Valid {
		c.DeployedAt = &deployedAt.Int64
	}
	if abiJSON.Valid && abiJSON.String != "" {
		c.ABI = &ABI{}
		json.Unmarshal([]byte(abiJSON.String), c.ABI)
	}

	return &c, nil
}

// List lists contracts for a user.
func (m *Manager) List(userID int64, orgID *int64) ([]Contract, error) {
	query := "SELECT id, address, user_id, org_id, name, contract_type, state, deployed_at, created_at FROM contracts WHERE user_id = ?"
	args := []any{userID}

	if orgID != nil {
		query += " OR org_id = ?"
		args = append(args, *orgID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}
	defer rows.Close()

	var contracts []Contract
	for rows.Next() {
		var c Contract
		var org sql.NullInt64
		var name, contractType, state sql.NullString
		var deployedAt sql.NullInt64

		err := rows.Scan(&c.ID, &c.Address, &c.UserID, &org, &name, &contractType, &state, &deployedAt, &c.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}

		if org.Valid {
			c.OrgID = &org.Int64
		}
		if name.Valid {
			c.Name = name.String
		}
		if contractType.Valid {
			c.ContractType = ContractType(contractType.String)
		}
		if state.Valid {
			c.State = state.String
		}
		if deployedAt.Valid {
			c.DeployedAt = &deployedAt.Int64
		}

		contracts = append(contracts, c)
	}

	return contracts, nil
}

// GetState retrieves current contract state from blockchain.
func (m *Manager) GetState(ctx context.Context, address string) (*StateResult, error) {
	balance, state, err := m.provider.GetAccountState(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	return &StateResult{
		Address: address,
		Balance: balance,
		State:   state,
	}, nil
}

// CallGetMethod calls a get method on a contract.
func (m *Manager) CallGetMethod(ctx context.Context, req CallRequest) (*CallResult, error) {
	result, gasUsed, exitCode, err := m.provider.RunGetMethod(ctx, req.Address, req.Method, req.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to call method: %w", err)
	}

	return &CallResult{
		GasUsed:  gasUsed,
		ExitCode: exitCode,
		Result:   result,
	}, nil
}

// Delete removes a contract registration.
func (m *Manager) Delete(address string, userID int64) error {
	result, err := m.db.Exec(
		"DELETE FROM contracts WHERE address = ? AND user_id = ?",
		address, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete contract: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrContractNotFound
	}

	return nil
}

// UpdateABI updates a contract's ABI.
func (m *Manager) UpdateABI(address string, userID int64, abi *ABI) error {
	abiJSON, err := json.Marshal(abi)
	if err != nil {
		return fmt.Errorf("failed to marshal ABI: %w", err)
	}

	result, err := m.db.Exec(
		"UPDATE contracts SET abi = ? WHERE address = ? AND user_id = ?",
		string(abiJSON), address, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update ABI: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrContractNotFound
	}

	return nil
}

// ValidateABI validates an ABI structure.
func ValidateABI(abi *ABI) error {
	if abi == nil {
		return nil
	}

	validTypes := map[string]bool{
		"int":     true,
		"uint":    true,
		"int8":    true,
		"int16":   true,
		"int32":   true,
		"int64":   true,
		"int128":  true,
		"int256":  true,
		"uint8":   true,
		"uint16":  true,
		"uint32":  true,
		"uint64":  true,
		"uint128": true,
		"uint256": true,
		"bool":    true,
		"address": true,
		"cell":    true,
		"slice":   true,
		"builder": true,
		"string":  true,
		"bytes":   true,
	}

	for _, method := range append(abi.GetMethods, abi.Messages...) {
		for _, param := range append(method.Inputs, method.Outputs...) {
			if !validTypes[param.Type] {
				return fmt.Errorf("%w: unknown type %s in method %s", ErrInvalidABI, param.Type, method.Name)
			}
		}
	}

	return nil
}
