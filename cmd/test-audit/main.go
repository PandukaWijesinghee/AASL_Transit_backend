package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/config"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

func main() {
	fmt.Println("=== Audit Logging Test ===\n")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	fmt.Println("Connecting to database...")
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("✅ Database connected")

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Database ping successful\n")

	// Initialize audit service
	auditService := services.NewAuditService(db)
	fmt.Println("✅ Audit service initialized\n")

	// Test 1: Log OTP Request
	fmt.Println("TEST 1: Logging OTP request...")
	err = auditService.LogOTPRequest(
		"+94771234567",
		"203.94.123.45",
		"TestAgent/1.0",
		true,
		"",
	)
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ SUCCESS: OTP request logged")
	}

	// Test 2: Log Login
	fmt.Println("\nTEST 2: Logging login event...")
	testUserID := uuid.New()
	err = auditService.LogLogin(
		testUserID,
		"+94771234567",
		"203.94.123.45",
		"TestAgent/1.0",
		"test-device-123",
		"android",
	)
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ SUCCESS: Login logged")
	}

	// Test 3: Verify data in database
	fmt.Println("\nTEST 3: Checking if data was inserted...")
	var count int
	query := "SELECT COUNT(*) FROM audit_logs"
	err = db.QueryRow(query).Scan(&count)
	if err != nil {
		fmt.Printf("❌ FAILED to query audit_logs: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d records in audit_logs table\n", count)
	}

	// Test 4: Show recent records
	if count > 0 {
		fmt.Println("\nTEST 4: Recent audit log entries:")
		rows, err := db.Query(`
			SELECT action, entity_type, ip_address, created_at
			FROM audit_logs
			ORDER BY created_at DESC
			LIMIT 5
		`)
		if err != nil {
			fmt.Printf("❌ FAILED to query recent logs: %v\n", err)
		} else {
			defer rows.Close()
			fmt.Println("----------------------------------------------")
			for rows.Next() {
				var action, entityType, ipAddress string
				var createdAt string
				if err := rows.Scan(&action, &entityType, &ipAddress, &createdAt); err != nil {
					fmt.Printf("❌ Error scanning row: %v\n", err)
					continue
				}
				fmt.Printf("- %s | %s | %s | %s\n", action, entityType, ipAddress, createdAt)
			}
			fmt.Println("----------------------------------------------")
		}
	}

	fmt.Println("\n=== Test Complete ===")
}
