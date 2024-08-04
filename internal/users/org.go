package users

import (
	"database/sql"
	"errors"
	"time"

	"tonkey/internal/store"
)

var (
	ErrOrgExists   = errors.New("organization already exists")
	ErrOrgNotFound = errors.New("organization not found")
)

type Organization struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Plan      string `json:"plan"`
	IsActive  bool   `json:"is_active"`
	CreatedAt int64  `json:"created_at"`
}

// CreateOrganization creates a new organization
func (m *Manager) CreateOrganization(name, slug, plan string) (*Organization, error) {
	if name == "" || slug == "" {
		return nil, errors.New("name and slug are required")
	}

	// Check if org exists
	var exists bool
	query := m.qb.Build("SELECT EXISTS(SELECT 1 FROM organizations WHERE slug = ?)", 1)
	err := m.DB.QueryRow(query, slug).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrOrgExists
	}

	if plan == "" {
		plan = "free"
	}

	query = m.qb.Build(`
		INSERT INTO organizations (name, slug, plan, is_active, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, 5)
	result, err := m.DB.Exec(query, name, slug, plan, true, time.Now().Unix())
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Organization{
		ID:        id,
		Name:      name,
		Slug:      slug,
		Plan:      plan,
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// GetOrganization retrieves an organization by ID
func (m *Manager) GetOrganization(id int64) (*Organization, error) {
	var org Organization
	query := m.qb.Build(`
		SELECT id, name, slug, plan, is_active, created_at
		FROM organizations
		WHERE id = ?
	`, 1)
	err := m.DB.QueryRow(query, id).Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.IsActive, &org.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// GetOrganizationBySlug retrieves an organization by slug
func (m *Manager) GetOrganizationBySlug(slug string) (*Organization, error) {
	var org Organization
	query := m.qb.Build(`
		SELECT id, name, slug, plan, is_active, created_at
		FROM organizations
		WHERE slug = ?
	`, 1)
	err := m.DB.QueryRow(query, slug).Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.IsActive, &org.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// ListOrganizations retrieves all organizations
func (m *Manager) ListOrganizations() ([]*Organization, error) {
	rows, err := m.DB.Query(`
		SELECT id, name, slug, plan, is_active, created_at
		FROM organizations
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		var org Organization
		err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.IsActive, &org.CreatedAt)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, &org)
	}
	return orgs, rows.Err()
}

// UpdateOrganization updates organization details
func (m *Manager) UpdateOrganization(id int64, name, plan string, isActive bool) error {
	query := m.qb.Build(`
		UPDATE organizations
		SET name = ?, plan = ?, is_active = ?
		WHERE id = ?
	`, 4)
	result, err := m.DB.Exec(query, name, plan, isActive, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOrgNotFound
	}
	return nil
}
