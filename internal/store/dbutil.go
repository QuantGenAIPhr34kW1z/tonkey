package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// DBDriver represents the database driver type
type DBDriver string

const (
	DriverSQLite   DBDriver = "sqlite3"
	DriverPostgres DBDriver = "postgres"
)

// DetectDriver detects the database driver from a sql.DB connection
func DetectDriver(db *sql.DB) DBDriver {
	// Try to detect from driver name
	// This is a bit hacky but works for our use case
	driver := fmt.Sprintf("%T", db.Driver())
	if strings.Contains(driver, "sqlite") {
		return DriverSQLite
	}
	if strings.Contains(driver, "pq") || strings.Contains(driver, "postgres") {
		return DriverPostgres
	}
	// Default to SQLite for safety
	return DriverSQLite
}

// QueryBuilder helps build database-agnostic queries
type QueryBuilder struct {
	driver       DBDriver
	paramCounter int
}

// NewQueryBuilder creates a new query builder for the given database
func NewQueryBuilder(db *sql.DB) *QueryBuilder {
	return &QueryBuilder{
		driver:       DetectDriver(db),
		paramCounter: 0,
	}
}

// Placeholder returns the next parameter placeholder for the current driver
func (qb *QueryBuilder) Placeholder() string {
	qb.paramCounter++
	if qb.driver == DriverPostgres {
		return fmt.Sprintf("$%d", qb.paramCounter)
	}
	return "?"
}

// Reset resets the parameter counter
func (qb *QueryBuilder) Reset() {
	qb.paramCounter = 0
}

// Build builds a query string with the correct placeholders
// Example: Build("SELECT * FROM users WHERE id = ? AND name = ?", 2)
// Returns: "SELECT * FROM users WHERE id = $1 AND name = $2" for Postgres
func (qb *QueryBuilder) Build(query string, numParams int) string {
	qb.Reset()
	if qb.driver == DriverSQLite {
		return query
	}

	// Replace ? with $N for PostgreSQL
	result := query
	for i := 1; i <= numParams; i++ {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", i), 1)
	}
	return result
}
