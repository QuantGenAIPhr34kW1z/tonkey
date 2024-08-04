// Package graphql provides a GraphQL API endpoint for tonkey.
// It offers flexible querying of wallets, transactions, NFTs, and Jettons.
package graphql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Schema defines the GraphQL schema as a string.
const Schema = `
type Query {
  # Wallet operations
  wallet(address: String!): Wallet
  walletBalance(address: String!): Balance

  # Transaction operations
  transaction(id: String!): Transaction
  transactions(filter: TransactionFilter, limit: Int, offset: Int): TransactionConnection!

  # User operations
  me: User
  user(id: ID!): User
  users(limit: Int, offset: Int): UserConnection!

  # NFT operations
  nftCollection(id: ID!): NFTCollection
  nftCollections(ownerID: ID, limit: Int, offset: Int): NFTCollectionConnection!
  nft(id: ID!): NFT
  nfts(collectionID: ID, ownerID: ID, limit: Int, offset: Int): NFTConnection!
  nftListing(id: ID!): NFTListing
  nftListings(collectionID: ID, status: String, limit: Int, offset: Int): NFTListingConnection!

  # Jetton operations
  jetton(id: ID!): Jetton
  jettons(ownerID: ID, limit: Int, offset: Int): JettonConnection!
  jettonBalance(jettonID: ID!, userID: ID!): JettonBalance

  # Organization operations
  organization(id: ID!): Organization

  # Webhook operations
  webhooks(limit: Int, offset: Int): WebhookConnection!

  # API Key operations
  apiKeys(limit: Int, offset: Int): APIKeyConnection!

  # Audit logs
  auditLogs(filter: AuditFilter, limit: Int, offset: Int): AuditLogConnection!
}

type Mutation {
  # Transaction operations
  sendTransaction(input: SendTransactionInput!): Transaction!

  # NFT operations
  createNFTCollection(input: CreateNFTCollectionInput!): NFTCollection!
  mintNFT(input: MintNFTInput!): NFT!
  transferNFT(input: TransferNFTInput!): NFTTransfer!
  listNFT(input: ListNFTInput!): NFTListing!
  buyNFT(listingID: ID!): NFTListing!
  cancelListing(listingID: ID!): Boolean!

  # Jetton operations
  createJetton(input: CreateJettonInput!): Jetton!
  transferJetton(input: TransferJettonInput!): JettonTransfer!
  mintJetton(input: MintJettonInput!): JettonMint!
  burnJetton(input: BurnJettonInput!): JettonBurn!

  # Webhook operations
  createWebhook(input: CreateWebhookInput!): Webhook!
  deleteWebhook(id: ID!): Boolean!

  # API Key operations
  createAPIKey(input: CreateAPIKeyInput!): APIKey!
  revokeAPIKey(id: ID!): Boolean!
}

type Subscription {
  transactionUpdated(address: String): Transaction!
  balanceChanged(address: String!): Balance!
  nftTransferred(collectionID: ID): NFTTransfer!
  jettonTransferred(jettonID: ID): JettonTransfer!
}

# Types
type Wallet {
  address: String!
  balance: String!
  lastActivity: DateTime
}

type Balance {
  address: String!
  balance: String!
  timestamp: DateTime!
}

type Transaction {
  id: ID!
  from: String!
  to: String!
  amount: String!
  status: String!
  txHash: String
  createdAt: DateTime!
  updatedAt: DateTime
}

type TransactionConnection {
  nodes: [Transaction!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type User {
  id: ID!
  username: String!
  email: String
  role: String!
  organization: Organization
  createdAt: DateTime!
  lastLogin: DateTime
}

type UserConnection {
  nodes: [User!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type Organization {
  id: ID!
  name: String!
  plan: String!
  members: [User!]!
  createdAt: DateTime!
}

type NFTCollection {
  id: ID!
  address: String
  name: String!
  description: String
  imageURL: String
  type: String!
  maxSupply: Int
  currentSupply: Int!
  royaltyPercent: Float!
  owner: User!
  nfts(limit: Int, offset: Int): NFTConnection!
  deployedAt: DateTime
  createdAt: DateTime!
}

type NFTCollectionConnection {
  nodes: [NFTCollection!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type NFT {
  id: ID!
  collection: NFTCollection!
  address: String
  index: Int!
  name: String!
  description: String
  imageURL: String
  contentURL: String
  attributes: [NFTAttribute!]
  owner: User!
  listing: NFTListing
  transferHistory: [NFTTransfer!]!
  mintedAt: DateTime
  createdAt: DateTime!
}

type NFTConnection {
  nodes: [NFT!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type NFTAttribute {
  traitType: String!
  value: String!
  displayType: String
}

type NFTListing {
  id: ID!
  nft: NFT!
  seller: User!
  price: String!
  currency: String!
  status: String!
  expiresAt: DateTime
  buyer: User
  soldAt: DateTime
  createdAt: DateTime!
}

type NFTListingConnection {
  nodes: [NFTListing!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type NFTTransfer {
  id: ID!
  nft: NFT!
  fromAddress: String!
  toAddress: String!
  txHash: String
  price: String
  createdAt: DateTime!
}

type Jetton {
  id: ID!
  address: String
  name: String!
  symbol: String!
  description: String
  imageURL: String
  decimals: Int!
  totalSupply: String!
  maxSupply: String
  type: String!
  mintingEnabled: Boolean!
  burningEnabled: Boolean!
  transfersEnabled: Boolean!
  owner: User!
  holders(limit: Int, offset: Int): JettonHolderConnection!
  deployedAt: DateTime
  createdAt: DateTime!
}

type JettonConnection {
  nodes: [Jetton!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type JettonBalance {
  jetton: Jetton!
  user: User!
  balance: String!
  isFrozen: Boolean!
}

type JettonHolderConnection {
  nodes: [JettonBalance!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type JettonTransfer {
  id: ID!
  jetton: Jetton!
  fromUser: User!
  toUser: User!
  amount: String!
  txHash: String
  comment: String
  createdAt: DateTime!
}

type JettonMint {
  id: ID!
  jetton: Jetton!
  toUser: User!
  amount: String!
  txHash: String
  minter: User!
  createdAt: DateTime!
}

type JettonBurn {
  id: ID!
  jetton: Jetton!
  fromUser: User!
  amount: String!
  txHash: String
  createdAt: DateTime!
}

type Webhook {
  id: ID!
  url: String!
  eventType: String!
  filterAddress: String
  isActive: Boolean!
  createdAt: DateTime!
}

type WebhookConnection {
  nodes: [Webhook!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type APIKey {
  id: ID!
  name: String!
  keyPrefix: String!
  scopes: [String!]!
  lastUsed: DateTime
  expiresAt: DateTime
  createdAt: DateTime!
}

type APIKeyConnection {
  nodes: [APIKey!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type AuditLog {
  id: ID!
  action: String!
  userID: ID
  resourceType: String
  resourceID: String
  ipAddress: String
  userAgent: String
  details: String
  createdAt: DateTime!
}

type AuditLogConnection {
  nodes: [AuditLog!]!
  totalCount: Int!
  pageInfo: PageInfo!
}

type PageInfo {
  hasNextPage: Boolean!
  hasPreviousPage: Boolean!
  startCursor: String
  endCursor: String
}

# Input types
input TransactionFilter {
  status: String
  from: String
  to: String
  minAmount: String
  maxAmount: String
  startDate: DateTime
  endDate: DateTime
}

input AuditFilter {
  action: String
  userID: ID
  resourceType: String
  startDate: DateTime
  endDate: DateTime
}

input SendTransactionInput {
  from: String!
  to: String!
  amount: String!
  comment: String
}

input CreateNFTCollectionInput {
  name: String!
  description: String
  imageURL: String
  type: String
  maxSupply: Int
  royaltyPercent: Float
}

input MintNFTInput {
  collectionID: ID!
  name: String!
  description: String
  imageURL: String
  contentURL: String
  attributes: [NFTAttributeInput!]
  recipientID: ID
}

input NFTAttributeInput {
  traitType: String!
  value: String!
  displayType: String
}

input TransferNFTInput {
  nftID: ID!
  toAddress: String
  toUserID: ID
}

input ListNFTInput {
  nftID: ID!
  price: String!
  currency: String
  expiresAt: DateTime
}

input CreateJettonInput {
  name: String!
  symbol: String!
  description: String
  imageURL: String
  decimals: Int
  initialSupply: String!
  maxSupply: String
  mintingEnabled: Boolean
  burningEnabled: Boolean
}

input TransferJettonInput {
  jettonID: ID!
  toUserID: ID!
  amount: String!
  comment: String
}

input MintJettonInput {
  jettonID: ID!
  toUserID: ID
  amount: String!
}

input BurnJettonInput {
  jettonID: ID!
  amount: String!
}

input CreateWebhookInput {
  url: String!
  eventType: String!
  filterAddress: String
  secret: String!
}

input CreateAPIKeyInput {
  name: String!
  scopes: [String!]!
  expiresAt: DateTime
}

scalar DateTime
`

// Server provides the GraphQL HTTP handler.
type Server struct {
	db          *sql.DB
	schema      string
	resolvers   *Resolvers
	playground  bool
}

// Config holds GraphQL server configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	Path       string `yaml:"path"`
	Playground bool   `yaml:"playground"`
}

// Resolvers holds all GraphQL resolvers.
type Resolvers struct {
	db *sql.DB
}

// Request represents a GraphQL request.
type Request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// Response represents a GraphQL response.
type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []Error     `json:"errors,omitempty"`
}

// Error represents a GraphQL error.
type Error struct {
	Message   string     `json:"message"`
	Locations []Location `json:"locations,omitempty"`
	Path      []string   `json:"path,omitempty"`
}

// Location represents an error location.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// NewServer creates a new GraphQL server.
func NewServer(db *sql.DB, playground bool) *Server {
	return &Server{
		db:         db,
		schema:     Schema,
		resolvers:  &Resolvers{db: db},
		playground: playground,
	}
}

// Handler returns the HTTP handler for GraphQL.
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && s.playground {
			s.servePlayground(w, r)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{
				Errors: []Error{{Message: "Method not allowed"}},
			})
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{
				Errors: []Error{{Message: "Invalid JSON: " + err.Error()}},
			})
			return
		}

		// Get user context from request
		ctx := r.Context()

		// Execute the query
		result := s.Execute(ctx, req)
		json.NewEncoder(w).Encode(result)
	}
}

// Execute executes a GraphQL request.
func (s *Server) Execute(ctx context.Context, req Request) Response {
	// Parse and execute the query
	query := strings.TrimSpace(req.Query)

	// Determine query type
	if strings.HasPrefix(query, "query") || strings.HasPrefix(query, "{") {
		return s.executeQuery(ctx, req)
	} else if strings.HasPrefix(query, "mutation") {
		return s.executeMutation(ctx, req)
	} else if strings.HasPrefix(query, "subscription") {
		return Response{
			Errors: []Error{{Message: "Subscriptions not supported over HTTP"}},
		}
	}

	return s.executeQuery(ctx, req)
}

// executeQuery handles GraphQL queries.
func (s *Server) executeQuery(ctx context.Context, req Request) Response {
	data := make(map[string]interface{})

	// Parse the query to extract fields
	fields := parseQueryFields(req.Query)

	for _, field := range fields {
		switch field.Name {
		case "wallet":
			address := getStringArg(field.Args, "address", req.Variables)
			data["wallet"] = s.resolvers.wallet(ctx, address)

		case "walletBalance":
			address := getStringArg(field.Args, "address", req.Variables)
			data["walletBalance"] = s.resolvers.walletBalance(ctx, address)

		case "transaction":
			id := getStringArg(field.Args, "id", req.Variables)
			data["transaction"] = s.resolvers.transaction(ctx, id)

		case "transactions":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["transactions"] = s.resolvers.transactions(ctx, nil, limit, offset)

		case "me":
			data["me"] = s.resolvers.me(ctx)

		case "user":
			id := getIntArg(field.Args, "id", req.Variables, 0)
			data["user"] = s.resolvers.user(ctx, int64(id))

		case "users":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["users"] = s.resolvers.users(ctx, limit, offset)

		case "nftCollection":
			id := getIntArg(field.Args, "id", req.Variables, 0)
			data["nftCollection"] = s.resolvers.nftCollection(ctx, int64(id))

		case "nftCollections":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["nftCollections"] = s.resolvers.nftCollections(ctx, nil, limit, offset)

		case "nft":
			id := getIntArg(field.Args, "id", req.Variables, 0)
			data["nft"] = s.resolvers.nft(ctx, int64(id))

		case "nfts":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["nfts"] = s.resolvers.nfts(ctx, nil, nil, limit, offset)

		case "nftListings":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["nftListings"] = s.resolvers.nftListings(ctx, nil, "active", limit, offset)

		case "jetton":
			id := getIntArg(field.Args, "id", req.Variables, 0)
			data["jetton"] = s.resolvers.jetton(ctx, int64(id))

		case "jettons":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["jettons"] = s.resolvers.jettons(ctx, nil, limit, offset)

		case "jettonBalance":
			jettonID := getIntArg(field.Args, "jettonID", req.Variables, 0)
			userID := getIntArg(field.Args, "userID", req.Variables, 0)
			data["jettonBalance"] = s.resolvers.jettonBalance(ctx, int64(jettonID), int64(userID))

		case "webhooks":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["webhooks"] = s.resolvers.webhooks(ctx, limit, offset)

		case "apiKeys":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["apiKeys"] = s.resolvers.apiKeys(ctx, limit, offset)

		case "auditLogs":
			limit := getIntArg(field.Args, "limit", req.Variables, 50)
			offset := getIntArg(field.Args, "offset", req.Variables, 0)
			data["auditLogs"] = s.resolvers.auditLogs(ctx, nil, limit, offset)

		case "__schema":
			data["__schema"] = s.introspectSchema()

		case "__type":
			typeName := getStringArg(field.Args, "name", req.Variables)
			data["__type"] = s.introspectType(typeName)
		}
	}

	return Response{Data: data}
}

// executeMutation handles GraphQL mutations.
func (s *Server) executeMutation(ctx context.Context, req Request) Response {
	data := make(map[string]interface{})
	fields := parseQueryFields(req.Query)

	for _, field := range fields {
		switch field.Name {
		case "sendTransaction":
			input := getInputArg(field.Args, "input", req.Variables)
			data["sendTransaction"] = s.resolvers.sendTransaction(ctx, input)

		case "createNFTCollection":
			input := getInputArg(field.Args, "input", req.Variables)
			data["createNFTCollection"] = s.resolvers.createNFTCollection(ctx, input)

		case "mintNFT":
			input := getInputArg(field.Args, "input", req.Variables)
			data["mintNFT"] = s.resolvers.mintNFT(ctx, input)

		case "transferNFT":
			input := getInputArg(field.Args, "input", req.Variables)
			data["transferNFT"] = s.resolvers.transferNFT(ctx, input)

		case "listNFT":
			input := getInputArg(field.Args, "input", req.Variables)
			data["listNFT"] = s.resolvers.listNFT(ctx, input)

		case "buyNFT":
			listingID := getIntArg(field.Args, "listingID", req.Variables, 0)
			data["buyNFT"] = s.resolvers.buyNFT(ctx, int64(listingID))

		case "cancelListing":
			listingID := getIntArg(field.Args, "listingID", req.Variables, 0)
			data["cancelListing"] = s.resolvers.cancelListing(ctx, int64(listingID))

		case "createJetton":
			input := getInputArg(field.Args, "input", req.Variables)
			data["createJetton"] = s.resolvers.createJetton(ctx, input)

		case "transferJetton":
			input := getInputArg(field.Args, "input", req.Variables)
			data["transferJetton"] = s.resolvers.transferJetton(ctx, input)

		case "mintJetton":
			input := getInputArg(field.Args, "input", req.Variables)
			data["mintJetton"] = s.resolvers.mintJetton(ctx, input)

		case "burnJetton":
			input := getInputArg(field.Args, "input", req.Variables)
			data["burnJetton"] = s.resolvers.burnJetton(ctx, input)

		case "createWebhook":
			input := getInputArg(field.Args, "input", req.Variables)
			data["createWebhook"] = s.resolvers.createWebhook(ctx, input)

		case "deleteWebhook":
			id := getIntArg(field.Args, "id", req.Variables, 0)
			data["deleteWebhook"] = s.resolvers.deleteWebhook(ctx, int64(id))

		case "createAPIKey":
			input := getInputArg(field.Args, "input", req.Variables)
			data["createAPIKey"] = s.resolvers.createAPIKey(ctx, input)

		case "revokeAPIKey":
			id := getIntArg(field.Args, "id", req.Variables, 0)
			data["revokeAPIKey"] = s.resolvers.revokeAPIKey(ctx, int64(id))
		}
	}

	return Response{Data: data}
}

// Resolver implementations

func (r *Resolvers) wallet(ctx context.Context, address string) map[string]interface{} {
	return map[string]interface{}{
		"address":      address,
		"balance":      "1000000000",
		"lastActivity": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) walletBalance(ctx context.Context, address string) map[string]interface{} {
	return map[string]interface{}{
		"address":   address,
		"balance":   "1000000000",
		"timestamp": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) transaction(ctx context.Context, id string) map[string]interface{} {
	var tx struct {
		ID        string
		From      string
		To        string
		Amount    int64
		Status    string
		CreatedAt time.Time
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id, sender_addr, receiver_addr, amount, status, created_at
		FROM tx_log WHERE id = ?`, id).Scan(&tx.ID, &tx.From, &tx.To, &tx.Amount, &tx.Status, &tx.CreatedAt)
	if err != nil {
		return nil
	}

	return map[string]interface{}{
		"id":        tx.ID,
		"from":      tx.From,
		"to":        tx.To,
		"amount":    fmt.Sprintf("%d", tx.Amount),
		"status":    tx.Status,
		"createdAt": tx.CreatedAt.Format(time.RFC3339),
	}
}

func (r *Resolvers) transactions(ctx context.Context, filter map[string]interface{}, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tx_log`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sender_addr, receiver_addr, amount, status, created_at
		FROM tx_log ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var tx struct {
			ID        string
			From      string
			To        string
			Amount    int64
			Status    string
			CreatedAt time.Time
		}
		if rows.Scan(&tx.ID, &tx.From, &tx.To, &tx.Amount, &tx.Status, &tx.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":        tx.ID,
				"from":      tx.From,
				"to":        tx.To,
				"amount":    fmt.Sprintf("%d", tx.Amount),
				"status":    tx.Status,
				"createdAt": tx.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) me(ctx context.Context) map[string]interface{} {
	// Would get from context in real implementation
	return map[string]interface{}{
		"id":        "1",
		"username":  "demo",
		"email":     "demo@example.com",
		"role":      "user",
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) user(ctx context.Context, id int64) map[string]interface{} {
	var user struct {
		ID        int64
		Username  string
		Email     sql.NullString
		Role      string
		CreatedAt time.Time
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, role, created_at FROM users WHERE id = ?`, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil
	}

	result := map[string]interface{}{
		"id":        fmt.Sprintf("%d", user.ID),
		"username":  user.Username,
		"role":      user.Role,
		"createdAt": user.CreatedAt.Format(time.RFC3339),
	}
	if user.Email.Valid {
		result["email"] = user.Email.String
	}
	return result
}

func (r *Resolvers) users(ctx context.Context, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, email, role, created_at FROM users LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var user struct {
			ID        int64
			Username  string
			Email     sql.NullString
			Role      string
			CreatedAt time.Time
		}
		if rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt) == nil {
			node := map[string]interface{}{
				"id":        fmt.Sprintf("%d", user.ID),
				"username":  user.Username,
				"role":      user.Role,
				"createdAt": user.CreatedAt.Format(time.RFC3339),
			}
			if user.Email.Valid {
				node["email"] = user.Email.String
			}
			nodes = append(nodes, node)
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) nftCollection(ctx context.Context, id int64) map[string]interface{} {
	var col struct {
		ID            int64
		Name          string
		Description   sql.NullString
		ImageURL      sql.NullString
		Type          string
		CurrentSupply int64
		RoyaltyPct    float64
		CreatedAt     time.Time
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, image_url, type, current_supply, royalty_percent, created_at
		FROM nft_collections WHERE id = ?`, id).Scan(
		&col.ID, &col.Name, &col.Description, &col.ImageURL, &col.Type,
		&col.CurrentSupply, &col.RoyaltyPct, &col.CreatedAt)
	if err != nil {
		return nil
	}

	result := map[string]interface{}{
		"id":             fmt.Sprintf("%d", col.ID),
		"name":           col.Name,
		"type":           col.Type,
		"currentSupply":  col.CurrentSupply,
		"royaltyPercent": col.RoyaltyPct,
		"createdAt":      col.CreatedAt.Format(time.RFC3339),
	}
	if col.Description.Valid {
		result["description"] = col.Description.String
	}
	if col.ImageURL.Valid {
		result["imageURL"] = col.ImageURL.String
	}
	return result
}

func (r *Resolvers) nftCollections(ctx context.Context, ownerID *int64, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nft_collections`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, current_supply, created_at FROM nft_collections LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var col struct {
			ID            int64
			Name          string
			Type          string
			CurrentSupply int64
			CreatedAt     time.Time
		}
		if rows.Scan(&col.ID, &col.Name, &col.Type, &col.CurrentSupply, &col.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":            fmt.Sprintf("%d", col.ID),
				"name":          col.Name,
				"type":          col.Type,
				"currentSupply": col.CurrentSupply,
				"createdAt":     col.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) nft(ctx context.Context, id int64) map[string]interface{} {
	var nft struct {
		ID          int64
		Name        string
		Description sql.NullString
		ImageURL    sql.NullString
		Index       int64
		CreatedAt   time.Time
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, image_url, idx, created_at FROM nfts WHERE id = ?`, id).Scan(
		&nft.ID, &nft.Name, &nft.Description, &nft.ImageURL, &nft.Index, &nft.CreatedAt)
	if err != nil {
		return nil
	}

	result := map[string]interface{}{
		"id":        fmt.Sprintf("%d", nft.ID),
		"name":      nft.Name,
		"index":     nft.Index,
		"createdAt": nft.CreatedAt.Format(time.RFC3339),
	}
	if nft.Description.Valid {
		result["description"] = nft.Description.String
	}
	if nft.ImageURL.Valid {
		result["imageURL"] = nft.ImageURL.String
	}
	return result
}

func (r *Resolvers) nfts(ctx context.Context, collectionID, ownerID *int64, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nfts`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, idx, created_at FROM nfts LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var nft struct {
			ID        int64
			Name      string
			Index     int64
			CreatedAt time.Time
		}
		if rows.Scan(&nft.ID, &nft.Name, &nft.Index, &nft.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":        fmt.Sprintf("%d", nft.ID),
				"name":      nft.Name,
				"index":     nft.Index,
				"createdAt": nft.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) nftListings(ctx context.Context, collectionID *int64, status string, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nft_listings WHERE status = ?`, status).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, nft_id, price, currency, status, created_at
		FROM nft_listings WHERE status = ? LIMIT ? OFFSET ?`, status, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var listing struct {
			ID        int64
			NFTID     int64
			Price     int64
			Currency  string
			Status    string
			CreatedAt time.Time
		}
		if rows.Scan(&listing.ID, &listing.NFTID, &listing.Price, &listing.Currency, &listing.Status, &listing.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":        fmt.Sprintf("%d", listing.ID),
				"price":     fmt.Sprintf("%d", listing.Price),
				"currency":  listing.Currency,
				"status":    listing.Status,
				"createdAt": listing.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) jetton(ctx context.Context, id int64) map[string]interface{} {
	var j struct {
		ID          int64
		Name        string
		Symbol      string
		Decimals    int
		TotalSupply string
		Type        string
		CreatedAt   time.Time
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, symbol, decimals, total_supply, type, created_at FROM jettons WHERE id = ?`, id).Scan(
		&j.ID, &j.Name, &j.Symbol, &j.Decimals, &j.TotalSupply, &j.Type, &j.CreatedAt)
	if err != nil {
		return nil
	}

	return map[string]interface{}{
		"id":          fmt.Sprintf("%d", j.ID),
		"name":        j.Name,
		"symbol":      j.Symbol,
		"decimals":    j.Decimals,
		"totalSupply": j.TotalSupply,
		"type":        j.Type,
		"createdAt":   j.CreatedAt.Format(time.RFC3339),
	}
}

func (r *Resolvers) jettons(ctx context.Context, ownerID *int64, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jettons`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, symbol, decimals, total_supply, type, created_at FROM jettons LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var j struct {
			ID          int64
			Name        string
			Symbol      string
			Decimals    int
			TotalSupply string
			Type        string
			CreatedAt   time.Time
		}
		if rows.Scan(&j.ID, &j.Name, &j.Symbol, &j.Decimals, &j.TotalSupply, &j.Type, &j.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":          fmt.Sprintf("%d", j.ID),
				"name":        j.Name,
				"symbol":      j.Symbol,
				"decimals":    j.Decimals,
				"totalSupply": j.TotalSupply,
				"type":        j.Type,
				"createdAt":   j.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) jettonBalance(ctx context.Context, jettonID, userID int64) map[string]interface{} {
	var balance struct {
		Balance  string
		IsFrozen bool
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT balance, is_frozen FROM jetton_wallets WHERE jetton_id = ? AND user_id = ?`, jettonID, userID).Scan(
		&balance.Balance, &balance.IsFrozen)
	if err != nil {
		return map[string]interface{}{
			"balance":  "0",
			"isFrozen": false,
		}
	}

	return map[string]interface{}{
		"balance":  balance.Balance,
		"isFrozen": balance.IsFrozen,
	}
}

func (r *Resolvers) webhooks(ctx context.Context, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhooks`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, url, event_type, is_active, created_at FROM webhooks LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var wh struct {
			ID        int64
			URL       string
			EventType string
			IsActive  bool
			CreatedAt time.Time
		}
		if rows.Scan(&wh.ID, &wh.URL, &wh.EventType, &wh.IsActive, &wh.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":        fmt.Sprintf("%d", wh.ID),
				"url":       wh.URL,
				"eventType": wh.EventType,
				"isActive":  wh.IsActive,
				"createdAt": wh.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) apiKeys(ctx context.Context, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, key_prefix, scopes, created_at FROM api_keys LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var ak struct {
			ID        int64
			Name      string
			KeyPrefix string
			Scopes    string
			CreatedAt time.Time
		}
		if rows.Scan(&ak.ID, &ak.Name, &ak.KeyPrefix, &ak.Scopes, &ak.CreatedAt) == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":        fmt.Sprintf("%d", ak.ID),
				"name":      ak.Name,
				"keyPrefix": ak.KeyPrefix,
				"scopes":    strings.Split(ak.Scopes, ","),
				"createdAt": ak.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

func (r *Resolvers) auditLogs(ctx context.Context, filter map[string]interface{}, limit, offset int) map[string]interface{} {
	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, action, user_id, resource_type, resource_id, ip_address, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]interface{}{"nodes": []interface{}{}, "totalCount": 0, "pageInfo": pageInfo(false, false)}
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var log struct {
			ID           int64
			Action       string
			UserID       sql.NullInt64
			ResourceType sql.NullString
			ResourceID   sql.NullString
			IPAddress    sql.NullString
			CreatedAt    time.Time
		}
		if rows.Scan(&log.ID, &log.Action, &log.UserID, &log.ResourceType, &log.ResourceID, &log.IPAddress, &log.CreatedAt) == nil {
			node := map[string]interface{}{
				"id":        fmt.Sprintf("%d", log.ID),
				"action":    log.Action,
				"createdAt": log.CreatedAt.Format(time.RFC3339),
			}
			if log.UserID.Valid {
				node["userID"] = fmt.Sprintf("%d", log.UserID.Int64)
			}
			if log.ResourceType.Valid {
				node["resourceType"] = log.ResourceType.String
			}
			if log.ResourceID.Valid {
				node["resourceID"] = log.ResourceID.String
			}
			if log.IPAddress.Valid {
				node["ipAddress"] = log.IPAddress.String
			}
			nodes = append(nodes, node)
		}
	}

	return map[string]interface{}{
		"nodes":      nodes,
		"totalCount": total,
		"pageInfo":   pageInfo(offset+limit < total, offset > 0),
	}
}

// Mutation resolvers (stubs - would integrate with actual managers)

func (r *Resolvers) sendTransaction(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("tx_%d", time.Now().UnixNano()),
		"from":      input["from"],
		"to":        input["to"],
		"amount":    input["amount"],
		"status":    "pending",
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) createNFTCollection(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":            fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":          input["name"],
		"type":          "standard",
		"currentSupply": 0,
		"createdAt":     time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) mintNFT(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":      input["name"],
		"index":     1,
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) transferNFT(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":          fmt.Sprintf("%d", time.Now().UnixNano()),
		"fromAddress": "EQFrom...",
		"toAddress":   input["toAddress"],
		"createdAt":   time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) listNFT(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"price":     input["price"],
		"currency":  "TON",
		"status":    "active",
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) buyNFT(ctx context.Context, listingID int64) map[string]interface{} {
	return map[string]interface{}{
		"id":     fmt.Sprintf("%d", listingID),
		"status": "sold",
		"soldAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) cancelListing(ctx context.Context, listingID int64) bool {
	return true
}

func (r *Resolvers) createJetton(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":          fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":        input["name"],
		"symbol":      input["symbol"],
		"decimals":    9,
		"totalSupply": input["initialSupply"],
		"type":        "standard",
		"createdAt":   time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) transferJetton(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"amount":    input["amount"],
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) mintJetton(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"amount":    input["amount"],
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) burnJetton(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"amount":    input["amount"],
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) createWebhook(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"url":       input["url"],
		"eventType": input["eventType"],
		"isActive":  true,
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) deleteWebhook(ctx context.Context, id int64) bool {
	return true
}

func (r *Resolvers) createAPIKey(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":        fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":      input["name"],
		"keyPrefix": "tk_",
		"scopes":    input["scopes"],
		"createdAt": time.Now().Format(time.RFC3339),
	}
}

func (r *Resolvers) revokeAPIKey(ctx context.Context, id int64) bool {
	return true
}

// Introspection

func (s *Server) introspectSchema() map[string]interface{} {
	return map[string]interface{}{
		"queryType":        map[string]interface{}{"name": "Query"},
		"mutationType":     map[string]interface{}{"name": "Mutation"},
		"subscriptionType": map[string]interface{}{"name": "Subscription"},
	}
}

func (s *Server) introspectType(name string) map[string]interface{} {
	return map[string]interface{}{
		"name": name,
		"kind": "OBJECT",
	}
}

// servePlayground serves the GraphQL Playground UI.
func (s *Server) servePlayground(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(playgroundHTML))
}

const playgroundHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>tonkey GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css">
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png">
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root">
    <style>
      body { background-color: rgb(23, 42, 58); height: 100vh; margin: 0; }
      #root { height: 100%; }
      .loading { font-family: sans-serif; color: #fff; text-align: center; padding: 50px; }
    </style>
    <div class="loading">Loading GraphQL Playground...</div>
  </div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: window.location.href,
        settings: {
          'editor.theme': 'dark',
          'request.credentials': 'include'
        }
      });
    });
  </script>
</body>
</html>`

// Helper types and functions

type queryField struct {
	Name string
	Args map[string]string
}

func parseQueryFields(query string) []queryField {
	var fields []queryField

	// Simple parser - in production use a proper GraphQL parser
	lines := strings.Split(query, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "{" || line == "}" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "query") || strings.HasPrefix(line, "mutation") {
			continue
		}

		// Extract field name
		name := line
		args := make(map[string]string)

		// Check for arguments
		if idx := strings.Index(line, "("); idx > 0 {
			name = strings.TrimSpace(line[:idx])
			argsStr := line[idx+1:]
			if endIdx := strings.Index(argsStr, ")"); endIdx > 0 {
				argsStr = argsStr[:endIdx]
				// Parse args
				for _, arg := range strings.Split(argsStr, ",") {
					arg = strings.TrimSpace(arg)
					if colonIdx := strings.Index(arg, ":"); colonIdx > 0 {
						argName := strings.TrimSpace(arg[:colonIdx])
						argVal := strings.TrimSpace(arg[colonIdx+1:])
						argVal = strings.Trim(argVal, "\"")
						args[argName] = argVal
					}
				}
			}
		} else if idx := strings.Index(line, " "); idx > 0 {
			name = line[:idx]
		} else if idx := strings.Index(line, "{"); idx > 0 {
			name = strings.TrimSpace(line[:idx])
		}

		name = strings.TrimSuffix(name, "{")
		name = strings.TrimSpace(name)

		if name != "" && !strings.HasPrefix(name, "}") {
			fields = append(fields, queryField{Name: name, Args: args})
		}
	}

	return fields
}

func getStringArg(args map[string]string, name string, vars map[string]interface{}) string {
	if val, ok := args[name]; ok {
		if strings.HasPrefix(val, "$") {
			varName := val[1:]
			if v, ok := vars[varName]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
		return val
	}
	return ""
}

func getIntArg(args map[string]string, name string, vars map[string]interface{}, defaultVal int) int {
	if val, ok := args[name]; ok {
		if strings.HasPrefix(val, "$") {
			varName := val[1:]
			if v, ok := vars[varName]; ok {
				switch n := v.(type) {
				case float64:
					return int(n)
				case int:
					return n
				}
			}
		}
		var n int
		fmt.Sscanf(val, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return defaultVal
}

func getInputArg(args map[string]string, name string, vars map[string]interface{}) map[string]interface{} {
	if val, ok := args[name]; ok {
		if strings.HasPrefix(val, "$") {
			varName := val[1:]
			if v, ok := vars[varName]; ok {
				if m, ok := v.(map[string]interface{}); ok {
					return m
				}
			}
		}
	}
	// Check variables directly for "input"
	if v, ok := vars["input"]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return make(map[string]interface{})
}

func pageInfo(hasNext, hasPrev bool) map[string]interface{} {
	return map[string]interface{}{
		"hasNextPage":     hasNext,
		"hasPreviousPage": hasPrev,
	}
}
