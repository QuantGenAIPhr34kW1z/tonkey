package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStore implements Store interface for PostgreSQL
type PostgresStore struct {
	DB *sql.DB
}

// OpenPostgres opens a PostgreSQL connection
func OpenPostgres(connString string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize schema
	if err := initPostgresSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return &PostgresStore{DB: db}, nil
}

// initPostgresSchema creates tables if they don't exist
func initPostgresSchema(db *sql.DB) error {
	schema := `
-- Organizations for multi-tenancy
CREATE TABLE IF NOT EXISTS organizations (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  plan TEXT NOT NULL DEFAULT 'free',
  is_active BOOLEAN DEFAULT true,
  created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug);

-- Users with bcrypt password hashing
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  email TEXT,
  org_id BIGINT REFERENCES organizations(id),
  role TEXT NOT NULL DEFAULT 'user',
  is_active BOOLEAN DEFAULT true,
  created_at BIGINT NOT NULL,
  last_login BIGINT
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);

-- Transaction log
CREATE TABLE IF NOT EXISTS tx_log (
  id TEXT PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  org_id BIGINT REFERENCES organizations(id),
  created_at BIGINT NOT NULL,
  from_addr TEXT NOT NULL,
  to_addr TEXT NOT NULL,
  amount BIGINT NOT NULL,
  status TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tx_log_user_id ON tx_log(user_id);
CREATE INDEX IF NOT EXISTS idx_tx_log_org_id ON tx_log(org_id);
CREATE INDEX IF NOT EXISTS idx_tx_log_status ON tx_log(status);

-- Watchlist for Telegram bot
CREATE TABLE IF NOT EXISTS watchlist (
  chat_id BIGINT NOT NULL,
  addr TEXT NOT NULL,
  user_id BIGINT REFERENCES users(id),
  created_at BIGINT NOT NULL,
  PRIMARY KEY(chat_id, addr)
);

-- API Keys
CREATE TABLE IF NOT EXISTS api_keys (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  key_hash TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  last_used BIGINT,
  expires_at BIGINT,
  created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);

-- Webhooks
CREATE TABLE IF NOT EXISTS webhooks (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  org_id BIGINT REFERENCES organizations(id),
  url TEXT NOT NULL,
  event_type TEXT NOT NULL,
  filter_address TEXT,
  secret TEXT NOT NULL,
  is_active BOOLEAN DEFAULT true,
  created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhooks_user_id ON webhooks(user_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_event_type ON webhooks(event_type);

-- Webhook delivery logs
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id BIGSERIAL PRIMARY KEY,
  webhook_id BIGINT NOT NULL REFERENCES webhooks(id),
  event_data TEXT NOT NULL,
  status_code INTEGER,
  response_body TEXT,
  error TEXT,
  delivered_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
`

	_, err := db.Exec(schema)
	return err
}

// Close closes the PostgreSQL connection
func (s *PostgresStore) Close() error {
	return s.DB.Close()
}

// Transaction methods use the same interface as SQLite
func (s *PostgresStore) InsertTx(t Tx) error {
	_, err := s.DB.Exec(`
		INSERT INTO tx_log(id, user_id, org_id, created_at, from_addr, to_addr, amount, status)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.ID, t.UserID, t.OrgID, t.CreatedAt, t.From, t.To, t.Amount, t.Status)
	return err
}

func (s *PostgresStore) GetTx(id string) (*Tx, error) {
	row := s.DB.QueryRow(`
		SELECT id, user_id, org_id, created_at, from_addr, to_addr, amount, status
		FROM tx_log WHERE id=$1
	`, id)

	var t Tx
	err := row.Scan(&t.ID, &t.UserID, &t.OrgID, &t.CreatedAt, &t.From, &t.To, &t.Amount, &t.Status)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PostgresStore) AddWatch(chatID int64, addr string) error {
	_, err := s.DB.Exec(`
		INSERT INTO watchlist(chat_id, addr, created_at)
		VALUES($1, $2, $3)
		ON CONFLICT (chat_id, addr) DO NOTHING
	`, chatID, addr, time.Now().Unix())
	return err
}

func (s *PostgresStore) RemoveWatch(chatID int64, addr string) error {
	_, err := s.DB.Exec("DELETE FROM watchlist WHERE chat_id=$1 AND addr=$2", chatID, addr)
	return err
}

func (s *PostgresStore) ListWatch(chatID int64) ([]string, error) {
	rows, err := s.DB.Query("SELECT addr FROM watchlist WHERE chat_id=$1 ORDER BY created_at DESC", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
