package store

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store provides database operations for transactions and watchlists
type Store struct{ DB *sql.DB }

// Tx represents a transaction record
type Tx struct {
	ID        string `json:"id"`
	UserID    int64  `json:"user_id,omitempty"`
	OrgID     int64  `json:"org_id,omitempty"`
	CreatedAt int64  `json:"created_at"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
}

// TxStore defines the interface for transaction storage operations
type TxStore interface {
	InsertTx(t Tx) error
	GetTx(id string) (*Tx, error)
	AddWatch(chatID int64, addr string) error
	RemoveWatch(chatID int64, addr string) error
	ListWatch(chatID int64) ([]string, error)
	Close() error
}

func Open(path string, schemaSQL string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) InsertTx(t Tx) error {
	_, err := s.DB.Exec(`
		INSERT INTO tx_log(id, user_id, org_id, created_at, from_addr, to_addr, amount, status)
		VALUES(?,?,?,?,?,?,?,?)
	`, t.ID, t.UserID, t.OrgID, t.CreatedAt, t.From, t.To, t.Amount, t.Status)
	return err
}

func (s *Store) GetTx(id string) (*Tx, error) {
	row := s.DB.QueryRow(`
		SELECT id, user_id, org_id, created_at, from_addr, to_addr, amount, status
		FROM tx_log WHERE id=?
	`, id)
	var t Tx
	if err := row.Scan(&t.ID, &t.UserID, &t.OrgID, &t.CreatedAt, &t.From, &t.To, &t.Amount, &t.Status); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) AddWatch(chatID int64, addr string) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO watchlist(chat_id, addr, created_at) VALUES(?,?,?)`, chatID, addr, time.Now().Unix())
	return err
}

func (s *Store) RemoveWatch(chatID int64, addr string) error {
	_, err := s.DB.Exec(`DELETE FROM watchlist WHERE chat_id=? AND addr=?`, chatID, addr)
	return err
}

func (s *Store) ListWatch(chatID int64) ([]string, error) {
	rows, err := s.DB.Query(`SELECT addr FROM watchlist WHERE chat_id=? ORDER BY created_at DESC`, chatID)
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

// Close closes the database connection
func (s *Store) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}
