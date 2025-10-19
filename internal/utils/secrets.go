package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateSecret generates a cryptographically secure random secret
func GenerateSecret(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateJWTSecrets generates two different JWT secrets (access and refresh)
func GenerateJWTSecrets() (accessSecret, refreshSecret string, err error) {
	accessSecret, err = GenerateSecret(32) // 256-bit
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access secret: %w", err)
	}

	refreshSecret, err = GenerateSecret(32) // 256-bit
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh secret: %w", err)
	}

	return accessSecret, refreshSecret, nil
}
