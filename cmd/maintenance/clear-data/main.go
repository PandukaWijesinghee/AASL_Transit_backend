package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/smarttransit/sms-auth-backend/internal/config"
	"github.com/smarttransit/sms-auth-backend/internal/database"
)

func main() {
	var dbURLFlag string
	flag.StringVar(&dbURLFlag, "database-url", "", "PostgreSQL connection string (overrides DATABASE_URL)")
	flag.Parse()

	// Try loading .env from current working directory (optional)
	// This avoids having to pass secrets on the command line.
	_ = godotenv.Load()

	dbURL := dbURLFlag
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set and -database-url was not provided")
	}

	// Build minimal database config without loading full app config
	dbCfg := config.DatabaseConfig{
		URL:                dbURL,
		MaxConnections:     5,
		MaxIdleConnections: 2,
		// ConnMaxLifetime left as zero (driver default)
	}

	db, err := database.NewConnection(dbCfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Connected to database. Truncating tables...")

	truncateSQL := `
TRUNCATE TABLE
    audit_logs,
    bus_staff,
    bus_owners,
    lounge_owners,
    otp_verifications,
    otp_rate_limits,
    refresh_tokens,
    user_sessions,
    users
RESTART IDENTITY CASCADE;`

	if _, err := db.Exec(truncateSQL); err != nil {
		log.Fatalf("failed to truncate tables: %v", err)
	}

	fmt.Println("All data cleared successfully (tables truncated, identities reset).")

	// Verify by printing row counts for each table
	tables := []string{
		"audit_logs",
		"bus_staff",
		"bus_owners",
		"lounge_owners",
		"otp_verifications",
		"otp_rate_limits",
		"refresh_tokens",
		"user_sessions",
		"users",
	}

	fmt.Println("Post-clear row counts:")
	for _, t := range tables {
		var count int
		if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&count); err != nil {
			fmt.Printf("  %s: error: %v\n", t, err)
			continue
		}
		fmt.Printf("  %s: %d\n", t, count)
	}
}
