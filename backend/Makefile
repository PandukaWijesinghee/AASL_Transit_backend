# Makefile for SmartTransit SMS Authentication Backend

.PHONY: help build run test clean docker-build docker-run generate-secrets install-deps dev db-clear

# Variables
APP_NAME=sms-auth-backend
MAIN_PATH=./cmd/server
DOCKER_IMAGE=$(APP_NAME):latest
PORT=8080

# Default target
help:
	@echo "SmartTransit SMS Authentication Backend - Make Commands"
	@echo ""
	@echo "Available commands:"
	@echo "  make install-deps    - Install Go dependencies"
	@echo "  make generate-secrets - Generate JWT secrets"
	@echo "  make build           - Build the application"
	@echo "  make run             - Run the application"
	@echo "  make dev             - Run in development mode with hot reload"
	@echo "  make test            - Run tests"
	@echo "  make test-coverage   - Run tests with coverage"
	@echo "  make docker-build    - Build Docker image"
	@echo "  make docker-run      - Run Docker container"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make db-clear        - TRUNCATE all data (requires DATABASE_URL)"
	@echo "  make lint            - Run linter"
	@echo ""

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate JWT secrets
generate-secrets:
	@echo "Generating JWT secrets..."
	go run cmd/generate-secrets/main.go

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: bin/$(APP_NAME)"

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_PATH)

# Run in development mode with hot reload (requires air)
dev:
	@echo "Starting development server..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "air not installed. Install with: go install github.com/air-verse/air@latest"; \
		echo "Running without hot reload..."; \
		go run $(MAIN_PATH); \
	fi

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
	fi

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p $(PORT):$(PORT) --env-file .env $(DOCKER_IMAGE)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Clear all application data from the database (preserves schema)
db-clear:
	@if [ -z "$$DATABASE_URL" ]; then \
		echo "ERROR: DATABASE_URL is not set. Export it first."; \
		exit 1; \
	fi
	@echo "Clearing all data from application tables..."
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/clear_all_data.sql
	@echo "All data cleared successfully."

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "All checks passed!"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development tools installed!"
