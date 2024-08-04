package query

import (
	"database/sql"
	"fmt"
	"strings"

	"tonkey/internal/store"
)

// DBQuerier interface for types that provide a DB connection
type DBQuerier interface {
	GetDB() *sql.DB
}

func QueryTx(s store.TxStore, filters map[string]string, limit int) ([]store.Tx, error) {
	// Extract DB from store
	var db *sql.DB
	switch v := s.(type) {
	case *store.Store:
		db = v.DB
	case *store.PostgresStore:
		db = v.DB
	default:
		return nil, fmt.Errorf("unsupported store type")
	}

	where := []string{}
	args := []any{}

	for k, v := range filters {
		switch k {
		case "status":
			where = append(where, "status=?")
			args = append(args, v)
		case "from":
			where = append(where, "from_addr=?")
			args = append(args, v)
		case "to":
			where = append(where, "to_addr=?")
			args = append(args, v)
		default:
			return nil, fmt.Errorf("unsupported filter: %s", k)
		}
	}

	q := "SELECT id, user_id, org_id, created_at, from_addr, to_addr, amount, status FROM tx_log"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.Tx
	for rows.Next() {
		var t store.Tx
		if err := rows.Scan(&t.ID, &t.UserID, &t.OrgID, &t.CreatedAt, &t.From, &t.To, &t.Amount, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
