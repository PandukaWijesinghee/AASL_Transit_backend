package main

import (
	"fmt"
	"log"

	"github.com/smarttransit/sms-auth-backend/internal/utils"
)

func main() {
	fmt.Println("===========================================")
	fmt.Println("JWT Secret Generator for SmartTransit")
	fmt.Println("===========================================")
	fmt.Println()

	accessSecret, refreshSecret, err := utils.GenerateJWTSecrets()
	if err != nil {
		log.Fatalf("Failed to generate secrets: %v", err)
	}

	fmt.Println("✅ Secrets generated successfully!")
	fmt.Println()
	fmt.Println("Add these to your .env file or Choreo secrets:")
	fmt.Println()
	fmt.Printf("JWT_SECRET=%s\n", accessSecret)
	fmt.Printf("JWT_REFRESH_SECRET=%s\n", refreshSecret)
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Keep these secrets safe and never commit them to version control!")
	fmt.Println("===========================================")
}
