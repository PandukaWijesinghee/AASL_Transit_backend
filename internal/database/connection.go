package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
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

	// Check if using transaction mode pooler (port 6543)
	usingPooler := strings.Contains(connectionURL, ":6543")
	if usingPooler {
		fmt.Printf("INFO: Detected Supabase Transaction Mode (port 6543) - enabling simple protocol\n")
	}

	fmt.Printf("INFO: Final connection URL: %s\n", maskPassword(connectionURL))

	// Parse pgx config from connection URL
	pgxConfig, err := pgx.ParseConfig(connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Enable simple protocol mode for connection poolers (Supavisor/PgBouncer)
	// This disables prepared statements which cause "unnamed prepared statement does not exist" errors
	// with transaction-mode pooling
	if usingPooler {
		pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		fmt.Printf("INFO: Using QueryExecModeSimpleProtocol (fixes pooler prepared statement issues)\n")
	}

	// Register the pgx driver with our config
	connStr := stdlib.RegisterConnConfig(pgxConfig)

	// Connect using sqlx with the registered pgx driver
	db, err := sqlx.Connect("pgx", connStr)
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
