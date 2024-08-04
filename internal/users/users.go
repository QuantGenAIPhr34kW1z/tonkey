package users

import (
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"tonkey/internal/store"
)

var (
	ErrUserExists      = errors.New("user already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInactive        = errors.New("user is inactive")
)

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email,omitempty"`
	PasswordHash string `json:"-"` // Never serialize password
	OrgID        int64  `json:"org_id,omitempty"`
	Role         string `json:"role"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    int64  `json:"created_at"`
	LastLogin    int64  `json:"last_login,omitempty"`
}

type Manager struct {
	DB *sql.DB
	qb *store.QueryBuilder
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{
		DB: db,
		qb: store.NewQueryBuilder(db),
	}
}

// CreateUser creates a new user with hashed password
func (m *Manager) CreateUser(username, password, email string, orgID int64, role string) (*User, error) {
	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}

	// Check if user exists
	var exists bool
	query := m.qb.Build("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", 1)
	err := m.DB.QueryRow(query, username).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Default role
	if role == "" {
		role = "user"
	}

	// Insert user
	query = m.qb.Build(`
		INSERT INTO users (username, password_hash, email, org_id, role, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, 7)
	result, err := m.DB.Exec(query, username, string(hash), email, orgID, role, true, time.Now().Unix())
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        id,
		Username:  username,
		Email:     email,
		OrgID:     orgID,
		Role:      role,
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// Authenticate verifies username and password
func (m *Manager) Authenticate(username, password string) (*User, error) {
	var user User
	query := m.qb.Build(`
		SELECT id, username, password_hash, email, org_id, role, is_active, created_at, last_login
		FROM users
		WHERE username = ?
	`, 1)
	err := m.DB.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Email,
		&user.OrgID, &user.Role, &user.IsActive, &user.CreatedAt, &user.LastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrInactive
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidPassword
	}

	// Update last login
	updateQuery := m.qb.Build("UPDATE users SET last_login = ? WHERE id = ?", 2)
	_, _ = m.DB.Exec(updateQuery, time.Now().Unix(), user.ID)

	return &user, nil
}

// GetUser retrieves a user by ID
func (m *Manager) GetUser(id int64) (*User, error) {
	var user User
	query := m.qb.Build(`
		SELECT id, username, email, org_id, role, is_active, created_at, last_login
		FROM users
		WHERE id = ?
	`, 1)
	err := m.DB.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.OrgID,
		&user.Role, &user.IsActive, &user.CreatedAt, &user.LastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (m *Manager) GetUserByUsername(username string) (*User, error) {
	var user User
	query := m.qb.Build(`
		SELECT id, username, email, org_id, role, is_active, created_at, last_login
		FROM users
		WHERE username = ?
	`, 1)
	err := m.DB.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.OrgID,
		&user.Role, &user.IsActive, &user.CreatedAt, &user.LastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers retrieves all users (admin only)
func (m *Manager) ListUsers(orgID int64) ([]*User, error) {
	query := `
		SELECT id, username, email, org_id, role, is_active, created_at, last_login
		FROM users
	`
	args := []interface{}{}

	if orgID > 0 {
		query += " WHERE org_id = ?"
		args = append(args, orgID)
	}

	query += " ORDER BY created_at DESC"
	query = m.qb.Build(query, len(args))

	rows, err := m.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.OrgID,
			&user.Role, &user.IsActive, &user.CreatedAt, &user.LastLogin,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, rows.Err()
}

// UpdateUser updates user details (not password)
func (m *Manager) UpdateUser(id int64, email, role string, isActive bool) error {
	query := m.qb.Build(`
		UPDATE users
		SET email = ?, role = ?, is_active = ?
		WHERE id = ?
	`, 4)
	result, err := m.DB.Exec(query, email, role, isActive, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdatePassword updates user password
func (m *Manager) UpdatePassword(id int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := m.qb.Build(`
		UPDATE users
		SET password_hash = ?
		WHERE id = ?
	`, 2)
	result, err := m.DB.Exec(query, string(hash), id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

// DeleteUser soft-deletes a user by marking as inactive
func (m *Manager) DeleteUser(id int64) error {
	return m.UpdateUser(id, "", "", false)
}
