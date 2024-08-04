// Package nft provides NFT (Non-Fungible Token) support for TON blockchain.
// It handles NFT collections, minting, transfers, and marketplace integration.
package nft

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Standard errors for NFT operations.
var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrNFTNotFound        = errors.New("NFT not found")
	ErrNotOwner           = errors.New("not the owner of this NFT")
	ErrInvalidMetadata    = errors.New("invalid NFT metadata")
	ErrListingNotFound    = errors.New("listing not found")
	ErrListingExpired     = errors.New("listing has expired")
	ErrInsufficientFunds  = errors.New("insufficient funds for purchase")
)

// CollectionType represents the type of NFT collection.
type CollectionType string

const (
	CollectionTypeStandard  CollectionType = "standard"
	CollectionTypeEdition   CollectionType = "edition"
	CollectionTypeDynamic   CollectionType = "dynamic"
)

// Collection represents an NFT collection on TON.
type Collection struct {
	ID              int64          `json:"id"`
	OwnerID         int64          `json:"owner_id"`
	OrgID           *int64         `json:"org_id,omitempty"`
	Address         string         `json:"address"`
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	ImageURL        string         `json:"image_url,omitempty"`
	CoverURL        string         `json:"cover_url,omitempty"`
	Type            CollectionType `json:"type"`
	MaxSupply       *int64         `json:"max_supply,omitempty"`
	CurrentSupply   int64          `json:"current_supply"`
	RoyaltyPercent  float64        `json:"royalty_percent"`
	RoyaltyAddress  string         `json:"royalty_address,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	ContractCode    string         `json:"contract_code,omitempty"`
	DeployedAt      *time.Time     `json:"deployed_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// NFT represents an individual NFT item.
type NFT struct {
	ID            int64           `json:"id"`
	CollectionID  int64           `json:"collection_id"`
	OwnerID       int64           `json:"owner_id"`
	Address       string          `json:"address"`
	Index         int64           `json:"index"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	ImageURL      string          `json:"image_url,omitempty"`
	ContentURL    string          `json:"content_url,omitempty"`
	ContentType   string          `json:"content_type,omitempty"`
	Attributes    json.RawMessage `json:"attributes,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	MintedAt      *time.Time      `json:"minted_at,omitempty"`
	MintTxHash    string          `json:"mint_tx_hash,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// NFTAttribute represents a single attribute of an NFT.
type NFTAttribute struct {
	TraitType   string      `json:"trait_type"`
	Value       interface{} `json:"value"`
	DisplayType string      `json:"display_type,omitempty"`
}

// Transfer represents an NFT transfer record.
type Transfer struct {
	ID          int64     `json:"id"`
	NFTID       int64     `json:"nft_id"`
	FromAddress string    `json:"from_address"`
	ToAddress   string    `json:"to_address"`
	TxHash      string    `json:"tx_hash"`
	Price       *int64    `json:"price,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListingStatus represents the status of a marketplace listing.
type ListingStatus string

const (
	ListingStatusActive    ListingStatus = "active"
	ListingStatusSold      ListingStatus = "sold"
	ListingStatusCancelled ListingStatus = "cancelled"
	ListingStatusExpired   ListingStatus = "expired"
)

// Listing represents an NFT listing in the marketplace.
type Listing struct {
	ID          int64         `json:"id"`
	NFTID       int64         `json:"nft_id"`
	SellerID    int64         `json:"seller_id"`
	Price       int64         `json:"price"`
	Currency    string        `json:"currency"`
	Status      ListingStatus `json:"status"`
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"`
	BuyerID     *int64        `json:"buyer_id,omitempty"`
	SoldAt      *time.Time    `json:"sold_at,omitempty"`
	TxHash      string        `json:"tx_hash,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Offer represents a buy offer for an NFT.
type Offer struct {
	ID          int64      `json:"id"`
	NFTID       int64      `json:"nft_id"`
	BuyerID     int64      `json:"buyer_id"`
	Price       int64      `json:"price"`
	Currency    string     `json:"currency"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// MintRequest represents a request to mint a new NFT.
type MintRequest struct {
	CollectionID int64           `json:"collection_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	ImageURL     string          `json:"image_url,omitempty"`
	ContentURL   string          `json:"content_url,omitempty"`
	ContentType  string          `json:"content_type,omitempty"`
	Attributes   []NFTAttribute  `json:"attributes,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	RecipientID  *int64          `json:"recipient_id,omitempty"`
}

// TransferRequest represents a request to transfer an NFT.
type TransferRequest struct {
	NFTID       int64  `json:"nft_id"`
	ToAddress   string `json:"to_address"`
	ToUserID    *int64 `json:"to_user_id,omitempty"`
}

// ListingRequest represents a request to list an NFT for sale.
type ListingRequest struct {
	NFTID     int64      `json:"nft_id"`
	Price     int64      `json:"price"`
	Currency  string     `json:"currency"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Manager handles NFT operations.
type Manager struct {
	db       *sql.DB
	provider NFTProvider
}

// NFTProvider defines the interface for blockchain NFT operations.
type NFTProvider interface {
	DeployCollection(ctx context.Context, collection *Collection) (string, error)
	MintNFT(ctx context.Context, collection *Collection, nft *NFT) (string, string, error)
	TransferNFT(ctx context.Context, nft *NFT, toAddress string) (string, error)
	GetNFTData(ctx context.Context, address string) (*NFT, error)
	GetCollectionData(ctx context.Context, address string) (*Collection, error)
}

// NewManager creates a new NFT manager.
func NewManager(db *sql.DB, provider NFTProvider) *Manager {
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
	CREATE TABLE IF NOT EXISTS nft_collections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id INTEGER NOT NULL,
		org_id INTEGER,
		address TEXT UNIQUE,
		name TEXT NOT NULL,
		description TEXT,
		image_url TEXT,
		cover_url TEXT,
		type TEXT DEFAULT 'standard',
		max_supply INTEGER,
		current_supply INTEGER DEFAULT 0,
		royalty_percent REAL DEFAULT 0,
		royalty_address TEXT,
		metadata TEXT,
		contract_code TEXT,
		deployed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS nfts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		collection_id INTEGER NOT NULL,
		owner_id INTEGER NOT NULL,
		address TEXT UNIQUE,
		idx INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		image_url TEXT,
		content_url TEXT,
		content_type TEXT,
		attributes TEXT,
		metadata TEXT,
		minted_at DATETIME,
		mint_tx_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (collection_id) REFERENCES nft_collections(id)
	);

	CREATE TABLE IF NOT EXISTS nft_transfers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nft_id INTEGER NOT NULL,
		from_address TEXT NOT NULL,
		to_address TEXT NOT NULL,
		tx_hash TEXT,
		price INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (nft_id) REFERENCES nfts(id)
	);

	CREATE TABLE IF NOT EXISTS nft_listings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nft_id INTEGER NOT NULL,
		seller_id INTEGER NOT NULL,
		price INTEGER NOT NULL,
		currency TEXT DEFAULT 'TON',
		status TEXT DEFAULT 'active',
		expires_at DATETIME,
		buyer_id INTEGER,
		sold_at DATETIME,
		tx_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (nft_id) REFERENCES nfts(id)
	);

	CREATE TABLE IF NOT EXISTS nft_offers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nft_id INTEGER NOT NULL,
		buyer_id INTEGER NOT NULL,
		price INTEGER NOT NULL,
		currency TEXT DEFAULT 'TON',
		status TEXT DEFAULT 'pending',
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (nft_id) REFERENCES nfts(id)
	);

	CREATE INDEX IF NOT EXISTS idx_nfts_collection ON nfts(collection_id);
	CREATE INDEX IF NOT EXISTS idx_nfts_owner ON nfts(owner_id);
	CREATE INDEX IF NOT EXISTS idx_nft_listings_status ON nft_listings(status);
	CREATE INDEX IF NOT EXISTS idx_nft_transfers_nft ON nft_transfers(nft_id);
	`
	m.db.Exec(schema)
}

// CreateCollection creates a new NFT collection.
func (m *Manager) CreateCollection(ctx context.Context, userID int64, orgID *int64, col *Collection) (*Collection, error) {
	col.OwnerID = userID
	col.OrgID = orgID
	col.CurrentSupply = 0
	col.CreatedAt = time.Now()
	col.UpdatedAt = time.Now()

	if col.Type == "" {
		col.Type = CollectionTypeStandard
	}

	metadataJSON, _ := json.Marshal(col.Metadata)

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO nft_collections (owner_id, org_id, name, description, image_url, cover_url, type,
			max_supply, royalty_percent, royalty_address, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		col.OwnerID, col.OrgID, col.Name, col.Description, col.ImageURL, col.CoverURL, col.Type,
		col.MaxSupply, col.RoyaltyPercent, col.RoyaltyAddress, string(metadataJSON),
		col.CreatedAt, col.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	col.ID, _ = res.LastInsertId()
	return col, nil
}

// DeployCollection deploys a collection contract to the blockchain.
func (m *Manager) DeployCollection(ctx context.Context, userID int64, collectionID int64) (*Collection, error) {
	col, err := m.GetCollection(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	if col.OwnerID != userID {
		return nil, ErrNotOwner
	}

	if m.provider == nil {
		// Mock deployment
		col.Address = fmt.Sprintf("EQCollection_%d_%d", collectionID, time.Now().Unix())
		now := time.Now()
		col.DeployedAt = &now
	} else {
		address, err := m.provider.DeployCollection(ctx, col)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy collection: %w", err)
		}
		col.Address = address
		now := time.Now()
		col.DeployedAt = &now
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE nft_collections SET address = ?, deployed_at = ?, updated_at = ? WHERE id = ?`,
		col.Address, col.DeployedAt, time.Now(), col.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update collection: %w", err)
	}

	return col, nil
}

// GetCollection retrieves a collection by ID.
func (m *Manager) GetCollection(ctx context.Context, id int64) (*Collection, error) {
	var col Collection
	var metadataStr sql.NullString
	var deployedAt sql.NullTime

	err := m.db.QueryRowContext(ctx, `
		SELECT id, owner_id, org_id, address, name, description, image_url, cover_url, type,
			max_supply, current_supply, royalty_percent, royalty_address, metadata, deployed_at,
			created_at, updated_at
		FROM nft_collections WHERE id = ?`, id).Scan(
		&col.ID, &col.OwnerID, &col.OrgID, &col.Address, &col.Name, &col.Description,
		&col.ImageURL, &col.CoverURL, &col.Type, &col.MaxSupply, &col.CurrentSupply,
		&col.RoyaltyPercent, &col.RoyaltyAddress, &metadataStr, &deployedAt,
		&col.CreatedAt, &col.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrCollectionNotFound
	}
	if err != nil {
		return nil, err
	}

	if metadataStr.Valid {
		col.Metadata = json.RawMessage(metadataStr.String)
	}
	if deployedAt.Valid {
		col.DeployedAt = &deployedAt.Time
	}

	return &col, nil
}

// ListCollections lists collections for a user or organization.
func (m *Manager) ListCollections(ctx context.Context, userID int64, orgID *int64, limit, offset int) ([]*Collection, int, error) {
	var args []interface{}
	query := `SELECT id, owner_id, org_id, address, name, description, image_url, type,
		current_supply, royalty_percent, deployed_at, created_at FROM nft_collections WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM nft_collections WHERE 1=1`

	if orgID != nil {
		query += ` AND org_id = ?`
		countQuery += ` AND org_id = ?`
		args = append(args, *orgID)
	} else {
		query += ` AND owner_id = ?`
		countQuery += ` AND owner_id = ?`
		args = append(args, userID)
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

	var collections []*Collection
	for rows.Next() {
		var col Collection
		var deployedAt sql.NullTime
		err := rows.Scan(&col.ID, &col.OwnerID, &col.OrgID, &col.Address, &col.Name,
			&col.Description, &col.ImageURL, &col.Type, &col.CurrentSupply,
			&col.RoyaltyPercent, &deployedAt, &col.CreatedAt)
		if err != nil {
			continue
		}
		if deployedAt.Valid {
			col.DeployedAt = &deployedAt.Time
		}
		collections = append(collections, &col)
	}

	return collections, total, nil
}

// Mint creates a new NFT in a collection.
func (m *Manager) Mint(ctx context.Context, userID int64, req *MintRequest) (*NFT, error) {
	col, err := m.GetCollection(ctx, req.CollectionID)
	if err != nil {
		return nil, err
	}

	if col.OwnerID != userID {
		return nil, ErrNotOwner
	}

	if col.MaxSupply != nil && col.CurrentSupply >= *col.MaxSupply {
		return nil, errors.New("collection has reached max supply")
	}

	recipientID := userID
	if req.RecipientID != nil {
		recipientID = *req.RecipientID
	}

	attributesJSON, _ := json.Marshal(req.Attributes)
	metadataJSON, _ := json.Marshal(req.Metadata)

	nft := &NFT{
		CollectionID: req.CollectionID,
		OwnerID:      recipientID,
		Index:        col.CurrentSupply + 1,
		Name:         req.Name,
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		ContentURL:   req.ContentURL,
		ContentType:  req.ContentType,
		Attributes:   attributesJSON,
		Metadata:     metadataJSON,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if m.provider != nil && col.Address != "" {
		address, txHash, err := m.provider.MintNFT(ctx, col, nft)
		if err != nil {
			return nil, fmt.Errorf("failed to mint NFT: %w", err)
		}
		nft.Address = address
		nft.MintTxHash = txHash
		now := time.Now()
		nft.MintedAt = &now
	} else {
		// Mock minting
		nft.Address = fmt.Sprintf("EQNFT_%d_%d", req.CollectionID, nft.Index)
		nft.MintTxHash = fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), nft.Index)
		now := time.Now()
		nft.MintedAt = &now
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO nfts (collection_id, owner_id, address, idx, name, description, image_url,
			content_url, content_type, attributes, metadata, minted_at, mint_tx_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nft.CollectionID, nft.OwnerID, nft.Address, nft.Index, nft.Name, nft.Description,
		nft.ImageURL, nft.ContentURL, nft.ContentType, string(nft.Attributes), string(nft.Metadata),
		nft.MintedAt, nft.MintTxHash, nft.CreatedAt, nft.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save NFT: %w", err)
	}

	nft.ID, _ = res.LastInsertId()

	// Update collection supply
	m.db.ExecContext(ctx, `UPDATE nft_collections SET current_supply = current_supply + 1, updated_at = ? WHERE id = ?`,
		time.Now(), req.CollectionID)

	return nft, nil
}

// GetNFT retrieves an NFT by ID.
func (m *Manager) GetNFT(ctx context.Context, id int64) (*NFT, error) {
	var nft NFT
	var attributesStr, metadataStr sql.NullString
	var mintedAt sql.NullTime

	err := m.db.QueryRowContext(ctx, `
		SELECT id, collection_id, owner_id, address, idx, name, description, image_url,
			content_url, content_type, attributes, metadata, minted_at, mint_tx_hash, created_at, updated_at
		FROM nfts WHERE id = ?`, id).Scan(
		&nft.ID, &nft.CollectionID, &nft.OwnerID, &nft.Address, &nft.Index, &nft.Name,
		&nft.Description, &nft.ImageURL, &nft.ContentURL, &nft.ContentType,
		&attributesStr, &metadataStr, &mintedAt, &nft.MintTxHash, &nft.CreatedAt, &nft.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNFTNotFound
	}
	if err != nil {
		return nil, err
	}

	if attributesStr.Valid {
		nft.Attributes = json.RawMessage(attributesStr.String)
	}
	if metadataStr.Valid {
		nft.Metadata = json.RawMessage(metadataStr.String)
	}
	if mintedAt.Valid {
		nft.MintedAt = &mintedAt.Time
	}

	return &nft, nil
}

// ListNFTs lists NFTs for a collection or owner.
func (m *Manager) ListNFTs(ctx context.Context, collectionID *int64, ownerID *int64, limit, offset int) ([]*NFT, int, error) {
	var args []interface{}
	query := `SELECT id, collection_id, owner_id, address, idx, name, description, image_url,
		minted_at, created_at FROM nfts WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM nfts WHERE 1=1`

	if collectionID != nil {
		query += ` AND collection_id = ?`
		countQuery += ` AND collection_id = ?`
		args = append(args, *collectionID)
	}
	if ownerID != nil {
		query += ` AND owner_id = ?`
		countQuery += ` AND owner_id = ?`
		args = append(args, *ownerID)
	}

	var total int
	m.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query += ` ORDER BY idx ASC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var nfts []*NFT
	for rows.Next() {
		var nft NFT
		var mintedAt sql.NullTime
		err := rows.Scan(&nft.ID, &nft.CollectionID, &nft.OwnerID, &nft.Address, &nft.Index,
			&nft.Name, &nft.Description, &nft.ImageURL, &mintedAt, &nft.CreatedAt)
		if err != nil {
			continue
		}
		if mintedAt.Valid {
			nft.MintedAt = &mintedAt.Time
		}
		nfts = append(nfts, &nft)
	}

	return nfts, total, nil
}

// Transfer transfers an NFT to another address.
func (m *Manager) Transfer(ctx context.Context, userID int64, req *TransferRequest) (*Transfer, error) {
	nft, err := m.GetNFT(ctx, req.NFTID)
	if err != nil {
		return nil, err
	}

	if nft.OwnerID != userID {
		return nil, ErrNotOwner
	}

	fromAddress := nft.Address
	var txHash string

	if m.provider != nil {
		txHash, err = m.provider.TransferNFT(ctx, nft, req.ToAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer NFT: %w", err)
		}
	} else {
		txHash = fmt.Sprintf("tx_transfer_%d_%d", time.Now().UnixNano(), req.NFTID)
	}

	// Update owner
	newOwnerID := userID
	if req.ToUserID != nil {
		newOwnerID = *req.ToUserID
	}

	_, err = m.db.ExecContext(ctx, `UPDATE nfts SET owner_id = ?, updated_at = ? WHERE id = ?`,
		newOwnerID, time.Now(), req.NFTID)
	if err != nil {
		return nil, err
	}

	// Record transfer
	transfer := &Transfer{
		NFTID:       req.NFTID,
		FromAddress: fromAddress,
		ToAddress:   req.ToAddress,
		TxHash:      txHash,
		CreatedAt:   time.Now(),
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO nft_transfers (nft_id, from_address, to_address, tx_hash, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		transfer.NFTID, transfer.FromAddress, transfer.ToAddress, transfer.TxHash, transfer.CreatedAt)
	if err != nil {
		return nil, err
	}

	transfer.ID, _ = res.LastInsertId()
	return transfer, nil
}

// GetTransferHistory retrieves the transfer history for an NFT.
func (m *Manager) GetTransferHistory(ctx context.Context, nftID int64) ([]*Transfer, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, nft_id, from_address, to_address, tx_hash, price, created_at
		FROM nft_transfers WHERE nft_id = ? ORDER BY created_at DESC`, nftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []*Transfer
	for rows.Next() {
		var t Transfer
		var price sql.NullInt64
		err := rows.Scan(&t.ID, &t.NFTID, &t.FromAddress, &t.ToAddress, &t.TxHash, &price, &t.CreatedAt)
		if err != nil {
			continue
		}
		if price.Valid {
			t.Price = &price.Int64
		}
		transfers = append(transfers, &t)
	}

	return transfers, nil
}

// CreateListing lists an NFT for sale.
func (m *Manager) CreateListing(ctx context.Context, userID int64, req *ListingRequest) (*Listing, error) {
	nft, err := m.GetNFT(ctx, req.NFTID)
	if err != nil {
		return nil, err
	}

	if nft.OwnerID != userID {
		return nil, ErrNotOwner
	}

	// Check for existing active listing
	var existingID int64
	m.db.QueryRowContext(ctx, `SELECT id FROM nft_listings WHERE nft_id = ? AND status = 'active'`, req.NFTID).Scan(&existingID)
	if existingID > 0 {
		return nil, errors.New("NFT already has an active listing")
	}

	currency := req.Currency
	if currency == "" {
		currency = "TON"
	}

	listing := &Listing{
		NFTID:     req.NFTID,
		SellerID:  userID,
		Price:     req.Price,
		Currency:  currency,
		Status:    ListingStatusActive,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO nft_listings (nft_id, seller_id, price, currency, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		listing.NFTID, listing.SellerID, listing.Price, listing.Currency, listing.Status,
		listing.ExpiresAt, listing.CreatedAt, listing.UpdatedAt)
	if err != nil {
		return nil, err
	}

	listing.ID, _ = res.LastInsertId()
	return listing, nil
}

// GetListing retrieves a listing by ID.
func (m *Manager) GetListing(ctx context.Context, id int64) (*Listing, error) {
	var listing Listing
	var expiresAt, soldAt sql.NullTime
	var buyerID sql.NullInt64

	err := m.db.QueryRowContext(ctx, `
		SELECT id, nft_id, seller_id, price, currency, status, expires_at, buyer_id, sold_at, tx_hash, created_at, updated_at
		FROM nft_listings WHERE id = ?`, id).Scan(
		&listing.ID, &listing.NFTID, &listing.SellerID, &listing.Price, &listing.Currency,
		&listing.Status, &expiresAt, &buyerID, &soldAt, &listing.TxHash, &listing.CreatedAt, &listing.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrListingNotFound
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		listing.ExpiresAt = &expiresAt.Time
	}
	if buyerID.Valid {
		listing.BuyerID = &buyerID.Int64
	}
	if soldAt.Valid {
		listing.SoldAt = &soldAt.Time
	}

	return &listing, nil
}

// ListActiveListings lists active marketplace listings.
func (m *Manager) ListActiveListings(ctx context.Context, collectionID *int64, limit, offset int) ([]*Listing, int, error) {
	var args []interface{}
	baseQuery := `FROM nft_listings l`
	where := ` WHERE l.status = 'active' AND (l.expires_at IS NULL OR l.expires_at > ?)`
	args = append(args, time.Now())

	if collectionID != nil {
		baseQuery += ` JOIN nfts n ON l.nft_id = n.id`
		where += ` AND n.collection_id = ?`
		args = append(args, *collectionID)
	}

	var total int
	m.db.QueryRowContext(ctx, `SELECT COUNT(*) `+baseQuery+where, args...).Scan(&total)

	query := `SELECT l.id, l.nft_id, l.seller_id, l.price, l.currency, l.status, l.expires_at, l.created_at ` +
		baseQuery + where + ` ORDER BY l.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var listings []*Listing
	for rows.Next() {
		var l Listing
		var expiresAt sql.NullTime
		err := rows.Scan(&l.ID, &l.NFTID, &l.SellerID, &l.Price, &l.Currency, &l.Status, &expiresAt, &l.CreatedAt)
		if err != nil {
			continue
		}
		if expiresAt.Valid {
			l.ExpiresAt = &expiresAt.Time
		}
		listings = append(listings, &l)
	}

	return listings, total, nil
}

// BuyListing purchases an NFT from a listing.
func (m *Manager) BuyListing(ctx context.Context, buyerID int64, listingID int64) (*Listing, error) {
	listing, err := m.GetListing(ctx, listingID)
	if err != nil {
		return nil, err
	}

	if listing.Status != ListingStatusActive {
		return nil, errors.New("listing is not active")
	}

	if listing.ExpiresAt != nil && listing.ExpiresAt.Before(time.Now()) {
		return nil, ErrListingExpired
	}

	if listing.SellerID == buyerID {
		return nil, errors.New("cannot buy your own listing")
	}

	// In a real implementation, this would handle payment processing
	txHash := fmt.Sprintf("tx_buy_%d_%d", time.Now().UnixNano(), listingID)
	now := time.Now()

	listing.Status = ListingStatusSold
	listing.BuyerID = &buyerID
	listing.SoldAt = &now
	listing.TxHash = txHash
	listing.UpdatedAt = now

	_, err = m.db.ExecContext(ctx, `
		UPDATE nft_listings SET status = ?, buyer_id = ?, sold_at = ?, tx_hash = ?, updated_at = ?
		WHERE id = ?`,
		listing.Status, listing.BuyerID, listing.SoldAt, listing.TxHash, listing.UpdatedAt, listingID)
	if err != nil {
		return nil, err
	}

	// Transfer NFT ownership
	_, err = m.db.ExecContext(ctx, `UPDATE nfts SET owner_id = ?, updated_at = ? WHERE id = ?`,
		buyerID, now, listing.NFTID)
	if err != nil {
		return nil, err
	}

	// Record transfer with price
	nft, _ := m.GetNFT(ctx, listing.NFTID)
	m.db.ExecContext(ctx, `
		INSERT INTO nft_transfers (nft_id, from_address, to_address, tx_hash, price, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		listing.NFTID, nft.Address, fmt.Sprintf("user:%d", buyerID), txHash, listing.Price, now)

	return listing, nil
}

// CancelListing cancels an active listing.
func (m *Manager) CancelListing(ctx context.Context, userID int64, listingID int64) error {
	listing, err := m.GetListing(ctx, listingID)
	if err != nil {
		return err
	}

	if listing.SellerID != userID {
		return ErrNotOwner
	}

	if listing.Status != ListingStatusActive {
		return errors.New("listing is not active")
	}

	_, err = m.db.ExecContext(ctx, `UPDATE nft_listings SET status = ?, updated_at = ? WHERE id = ?`,
		ListingStatusCancelled, time.Now(), listingID)
	return err
}

// CreateOffer creates a buy offer for an NFT.
func (m *Manager) CreateOffer(ctx context.Context, buyerID int64, nftID int64, price int64, expiresAt *time.Time) (*Offer, error) {
	_, err := m.GetNFT(ctx, nftID)
	if err != nil {
		return nil, err
	}

	offer := &Offer{
		NFTID:     nftID,
		BuyerID:   buyerID,
		Price:     price,
		Currency:  "TON",
		Status:    "pending",
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	res, err := m.db.ExecContext(ctx, `
		INSERT INTO nft_offers (nft_id, buyer_id, price, currency, status, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		offer.NFTID, offer.BuyerID, offer.Price, offer.Currency, offer.Status, offer.ExpiresAt, offer.CreatedAt)
	if err != nil {
		return nil, err
	}

	offer.ID, _ = res.LastInsertId()
	return offer, nil
}

// GetOffersForNFT retrieves all offers for an NFT.
func (m *Manager) GetOffersForNFT(ctx context.Context, nftID int64) ([]*Offer, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, nft_id, buyer_id, price, currency, status, expires_at, created_at
		FROM nft_offers WHERE nft_id = ? AND status = 'pending'
		ORDER BY price DESC`, nftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []*Offer
	for rows.Next() {
		var o Offer
		var expiresAt sql.NullTime
		err := rows.Scan(&o.ID, &o.NFTID, &o.BuyerID, &o.Price, &o.Currency, &o.Status, &expiresAt, &o.CreatedAt)
		if err != nil {
			continue
		}
		if expiresAt.Valid {
			o.ExpiresAt = &expiresAt.Time
		}
		offers = append(offers, &o)
	}

	return offers, nil
}

// AcceptOffer accepts a buy offer for an NFT.
func (m *Manager) AcceptOffer(ctx context.Context, ownerID int64, offerID int64) error {
	var offer Offer
	err := m.db.QueryRowContext(ctx, `
		SELECT id, nft_id, buyer_id, price FROM nft_offers WHERE id = ? AND status = 'pending'`, offerID).Scan(
		&offer.ID, &offer.NFTID, &offer.BuyerID, &offer.Price)
	if err != nil {
		return errors.New("offer not found")
	}

	nft, err := m.GetNFT(ctx, offer.NFTID)
	if err != nil {
		return err
	}

	if nft.OwnerID != ownerID {
		return ErrNotOwner
	}

	// Transfer ownership
	txHash := fmt.Sprintf("tx_offer_%d_%d", time.Now().UnixNano(), offerID)
	now := time.Now()

	_, err = m.db.ExecContext(ctx, `UPDATE nfts SET owner_id = ?, updated_at = ? WHERE id = ?`,
		offer.BuyerID, now, offer.NFTID)
	if err != nil {
		return err
	}

	// Update offer status
	m.db.ExecContext(ctx, `UPDATE nft_offers SET status = 'accepted' WHERE id = ?`, offerID)

	// Cancel other offers
	m.db.ExecContext(ctx, `UPDATE nft_offers SET status = 'cancelled' WHERE nft_id = ? AND id != ? AND status = 'pending'`,
		offer.NFTID, offerID)

	// Cancel any active listings
	m.db.ExecContext(ctx, `UPDATE nft_listings SET status = 'cancelled', updated_at = ? WHERE nft_id = ? AND status = 'active'`,
		now, offer.NFTID)

	// Record transfer
	m.db.ExecContext(ctx, `
		INSERT INTO nft_transfers (nft_id, from_address, to_address, tx_hash, price, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		offer.NFTID, nft.Address, fmt.Sprintf("user:%d", offer.BuyerID), txHash, offer.Price, now)

	return nil
}
