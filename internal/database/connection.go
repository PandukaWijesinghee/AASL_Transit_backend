package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/smarttransit/sms-auth-backend/internal/config"
)

// DB interface defines database operations
type DB interface {
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Ping() error
	Close() error
}

// PostgresDB implements the DB interface using sqlx
type PostgresDB struct {
	*sqlx.DB
}

// maskPassword masks the password in a database URL for safe logging
func maskPassword(url string) string {
	// Replace password in postgres://user:password@host format
	re := regexp.MustCompile(`(postgres(?:ql)?://[^:]+:)([^@]+)(@.+)`)
	return re.ReplaceAllString(url, "${1}****${3}")
}

// NewConnection creates a new database connection
func NewConnection(cfg config.DatabaseConfig) (DB, error) {
	// Parse and validate connection string
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	// Add connection pooler compatibility parameters
	// For lib/pq driver with Supavisor/PgBouncer, we need binary_parameters=no
	// This disables binary encoding which causes "bind message" errors with pooled connections
	connectionURL := cfg.URL
	
	fmt.Printf("INFO: Original database URL: %s\n", maskPassword(cfg.URL))
	
	// Add sslmode if not present (required for Supabase)
	if !strings.Contains(connectionURL, "sslmode") {
		separator := "?"
		if strings.Contains(connectionURL, "?") {
			separator = "&"
		}
		connectionURL = connectionURL + separator + "sslmode=require"
		fmt.Printf("INFO: Added sslmode=require\n")
	}
	
	// Disable binary parameters to fix bind message errors with connection poolers
	// binary_parameters=no forces text protocol which doesn't have prepared statement issues
	if !strings.Contains(connectionURL, "binary_parameters") {
		separator := "?"
		if strings.Contains(connectionURL, "?") {
			separator = "&"
		}
		connectionURL = connectionURL + separator + "binary_parameters=no"
		fmt.Printf("INFO: Added binary_parameters=no (fixes connection pooler issues)\n")
	}
	
	fmt.Printf("INFO: Final connection URL: %s\n", maskPassword(connectionURL))

	// Connect to database
	db, err := sqlx.Connect("postgres", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for better stability with connection poolers
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Add idle timeout to prevent stale connections
	db.SetConnMaxIdleTime(cfg.ConnMaxLifetime / 2) // Half of max lifetime

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{DB: db}, nil
}

// Get wraps sqlx.Get
func (db *PostgresDB) Get(dest interface{}, query string, args ...interface{}) error {
	return db.DB.Get(dest, query, args...)
}

// Select wraps sqlx.Select
func (db *PostgresDB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.DB.Select(dest, query, args...)
}

// Exec wraps sqlx.Exec
func (db *PostgresDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(query, args...)
}

// QueryRow wraps sqlx.QueryRow
func (db *PostgresDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRow(query, args...)
}

// Query wraps sqlx.Query
func (db *PostgresDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.Query(query, args...)
}

// Ping wraps sqlx.Ping
func (db *PostgresDB) Ping() error {
	return db.DB.Ping()
}

// Close wraps sqlx.Close
func (db *PostgresDB) Close() error {
	return db.DB.Close()
}
