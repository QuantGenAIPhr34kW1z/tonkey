// Package migrations provides database schema migration management.
package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrMigrationFailed   = errors.New("migration failed")
	ErrNoMigrations      = errors.New("no migrations to apply")
	ErrAlreadyApplied    = errors.New("migration already applied")
	ErrMigrationNotFound = errors.New("migration not found")
)

// Migration represents a database migration.
type Migration struct {
	Version   int
	Name      string
	Up        string
	Down      string
	AppliedAt *int64
}

// Manager handles database migrations.
type Manager struct {
	db         *sql.DB
	migrations []Migration
}

// NewManager creates a new migration manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:         db,
		migrations: make([]Migration, 0),
	}
}

// Register registers a migration.
func (m *Manager) Register(version int, name, up, down string) {
	m.migrations = append(m.migrations, Migration{
		Version: version,
		Name:    name,
		Up:      up,
		Down:    down,
	})
}

// Init initializes the migrations table.
func (m *Manager) Init() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at INTEGER NOT NULL
		)
	`)
	return err
}

// CurrentVersion returns the current schema version.
func (m *Manager) CurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// Pending returns pending migrations.
func (m *Manager) Pending() ([]Migration, error) {
	current, err := m.CurrentVersion()
	if err != nil {
		return nil, err
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	var pending []Migration
	for _, mig := range m.migrations {
		if mig.Version > current {
			pending = append(pending, mig)
		}
	}

	return pending, nil
}

// Applied returns applied migrations.
func (m *Manager) Applied() ([]Migration, error) {
	rows, err := m.db.Query("SELECT version, name, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}
	defer rows.Close()

	var applied []Migration
	for rows.Next() {
		var mig Migration
		var appliedAt int64
		err := rows.Scan(&mig.Version, &mig.Name, &appliedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}
		mig.AppliedAt = &appliedAt
		applied = append(applied, mig)
	}

	return applied, nil
}

// Migrate applies all pending migrations.
func (m *Manager) Migrate() ([]Migration, error) {
	pending, err := m.Pending()
	if err != nil {
		return nil, err
	}

	if len(pending) == 0 {
		return nil, nil
	}

	var applied []Migration
	for _, mig := range pending {
		if err := m.applyMigration(mig); err != nil {
			return applied, fmt.Errorf("migration %d (%s) failed: %w", mig.Version, mig.Name, err)
		}
		applied = append(applied, mig)
	}

	return applied, nil
}

// MigrateTo migrates to a specific version.
func (m *Manager) MigrateTo(targetVersion int) error {
	current, err := m.CurrentVersion()
	if err != nil {
		return err
	}

	if targetVersion > current {
		// Migrate up
		for _, mig := range m.migrations {
			if mig.Version > current && mig.Version <= targetVersion {
				if err := m.applyMigration(mig); err != nil {
					return fmt.Errorf("migration %d failed: %w", mig.Version, err)
				}
			}
		}
	} else if targetVersion < current {
		// Migrate down
		sort.Slice(m.migrations, func(i, j int) bool {
			return m.migrations[i].Version > m.migrations[j].Version
		})

		for _, mig := range m.migrations {
			if mig.Version > targetVersion && mig.Version <= current {
				if err := m.rollbackMigration(mig); err != nil {
					return fmt.Errorf("rollback %d failed: %w", mig.Version, err)
				}
			}
		}
	}

	return nil
}

// applyMigration applies a single migration.
func (m *Manager) applyMigration(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the up migration
	if mig.Up != "" {
		if _, err := tx.Exec(mig.Up); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	// Record the migration
	now := time.Now().Unix()
	_, err = tx.Exec(
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
		mig.Version, mig.Name, now,
	)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// rollbackMigration rolls back a single migration.
func (m *Manager) rollbackMigration(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the down migration
	if mig.Down != "" {
		if _, err := tx.Exec(mig.Down); err != nil {
			return fmt.Errorf("failed to execute rollback: %w", err)
		}
	}

	// Remove the migration record
	_, err = tx.Exec("DELETE FROM schema_migrations WHERE version = ?", mig.Version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return tx.Commit()
}

// Rollback rolls back the last migration.
func (m *Manager) Rollback() (*Migration, error) {
	current, err := m.CurrentVersion()
	if err != nil {
		return nil, err
	}

	if current == 0 {
		return nil, ErrNoMigrations
	}

	// Find the current migration
	var currentMig *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == current {
			currentMig = &m.migrations[i]
			break
		}
	}

	if currentMig == nil {
		return nil, ErrMigrationNotFound
	}

	if err := m.rollbackMigration(*currentMig); err != nil {
		return nil, err
	}

	return currentMig, nil
}

// Status returns the migration status.
type Status struct {
	CurrentVersion int         `json:"current_version"`
	PendingCount   int         `json:"pending_count"`
	AppliedCount   int         `json:"applied_count"`
	Pending        []Migration `json:"pending,omitempty"`
	Applied        []Migration `json:"applied,omitempty"`
}

// GetStatus returns the current migration status.
func (m *Manager) GetStatus() (*Status, error) {
	current, err := m.CurrentVersion()
	if err != nil {
		return nil, err
	}

	pending, err := m.Pending()
	if err != nil {
		return nil, err
	}

	applied, err := m.Applied()
	if err != nil {
		return nil, err
	}

	return &Status{
		CurrentVersion: current,
		PendingCount:   len(pending),
		AppliedCount:   len(applied),
		Pending:        pending,
		Applied:        applied,
	}, nil
}

// RegisterDefaultMigrations registers the default schema migrations.
func (m *Manager) RegisterDefaultMigrations() {
	m.Register(1, "add_totp_columns", `
		ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN DEFAULT 0;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_verified_at INTEGER;
	`, `
		ALTER TABLE users DROP COLUMN IF EXISTS totp_secret;
		ALTER TABLE users DROP COLUMN IF EXISTS totp_enabled;
		ALTER TABLE users DROP COLUMN IF EXISTS totp_verified_at;
	`)

	m.Register(2, "create_backup_codes", `
		CREATE TABLE IF NOT EXISTS backup_codes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			code_hash TEXT NOT NULL,
			used_at INTEGER,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_backup_codes_user_id ON backup_codes(user_id);
	`, `
		DROP TABLE IF EXISTS backup_codes;
	`)

	m.Register(3, "create_audit_logs", `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			org_id INTEGER,
			action TEXT NOT NULL,
			resource_type TEXT,
			resource_id TEXT,
			ip_address TEXT,
			user_agent TEXT,
			metadata TEXT,
			checksum TEXT,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
	`, `
		DROP TABLE IF EXISTS audit_logs;
	`)

	m.Register(4, "create_password_reset_tokens", `
		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			expires_at INTEGER NOT NULL,
			used_at INTEGER,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);
	`, `
		DROP TABLE IF EXISTS password_reset_tokens;
	`)

	m.Register(5, "create_ip_rules", `
		CREATE TABLE IF NOT EXISTS ip_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			org_id INTEGER,
			cidr TEXT NOT NULL,
			rule_type TEXT NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at INTEGER NOT NULL,
			expires_at INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_ip_rules_org_id ON ip_rules(org_id);
		CREATE INDEX IF NOT EXISTS idx_ip_rules_rule_type ON ip_rules(rule_type);
	`, `
		DROP TABLE IF EXISTS ip_rules;
	`)

	//  migrations
	m.Register(6, "create_contracts", `
		CREATE TABLE IF NOT EXISTS contracts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			org_id INTEGER,
			name TEXT,
			contract_type TEXT,
			abi TEXT,
			state TEXT,
			deployed_at INTEGER,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_contracts_address ON contracts(address);
		CREATE INDEX IF NOT EXISTS idx_contracts_user_id ON contracts(user_id);
	`, `
		DROP TABLE IF EXISTS contracts;
	`)

	m.Register(7, "create_multisig_tables", `
		CREATE TABLE IF NOT EXISTS multisig_wallets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT UNIQUE NOT NULL,
			org_id INTEGER NOT NULL,
			name TEXT,
			threshold INTEGER NOT NULL,
			signers TEXT NOT NULL,
			is_active BOOLEAN DEFAULT 1,
			created_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS multisig_proposals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			wallet_id INTEGER NOT NULL,
			proposer_id INTEGER NOT NULL,
			action_type TEXT NOT NULL,
			action_data TEXT NOT NULL,
			approvals TEXT DEFAULT '[]',
			rejections TEXT DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'pending',
			tx_hash TEXT,
			created_at INTEGER NOT NULL,
			expires_at INTEGER,
			executed_at INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_multisig_wallets_org_id ON multisig_wallets(org_id);
		CREATE INDEX IF NOT EXISTS idx_multisig_proposals_wallet_id ON multisig_proposals(wallet_id);
	`, `
		DROP TABLE IF EXISTS multisig_proposals;
		DROP TABLE IF EXISTS multisig_wallets;
	`)

	m.Register(8, "create_tx_batches", `
		CREATE TABLE IF NOT EXISTS tx_batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			org_id INTEGER,
			status TEXT NOT NULL DEFAULT 'pending',
			total_amount INTEGER NOT NULL DEFAULT 0,
			tx_count INTEGER NOT NULL DEFAULT 0,
			processed_count INTEGER NOT NULL DEFAULT 0,
			error TEXT,
			created_at INTEGER NOT NULL,
			started_at INTEGER,
			completed_at INTEGER
		);
		ALTER TABLE tx_log ADD COLUMN IF NOT EXISTS batch_id INTEGER;
		CREATE INDEX IF NOT EXISTS idx_tx_batches_user_id ON tx_batches(user_id);
		CREATE INDEX IF NOT EXISTS idx_tx_batches_status ON tx_batches(status);
	`, `
		DROP TABLE IF EXISTS tx_batches;
	`)

	m.Register(9, "enhance_api_keys", `
		ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS org_id INTEGER;
		ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS key_prefix TEXT;
		ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS scopes TEXT DEFAULT '[]';
		ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS rate_limit INTEGER;
		ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS use_count INTEGER DEFAULT 0;
		ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT 1;
	`, `
		-- Cannot easily drop columns in SQLite
	`)

	m.Register(10, "create_jobs", `
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			queue TEXT NOT NULL DEFAULT 'default',
			job_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			priority INTEGER DEFAULT 0,
			attempts INTEGER DEFAULT 0,
			max_attempts INTEGER DEFAULT 3,
			last_error TEXT,
			scheduled_at INTEGER NOT NULL,
			started_at INTEGER,
			completed_at INTEGER,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_jobs_queue_status ON jobs(queue, status);
		CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_at ON jobs(scheduled_at);
	`, `
		DROP TABLE IF EXISTS jobs;
	`)

	m.Register(11, "create_email_verifications", `
		CREATE TABLE IF NOT EXISTS email_verifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			email TEXT NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			expires_at INTEGER NOT NULL,
			verified_at INTEGER,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_email_verifications_user_id ON email_verifications(user_id);
		CREATE INDEX IF NOT EXISTS idx_email_verifications_token_hash ON email_verifications(token_hash);
	`, `
		DROP TABLE IF EXISTS email_verifications;
	`)
}
