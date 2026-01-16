# Multi-stage Dockerfile for Go application
# Optimized for small image size and fast builds

# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

# Set build metadata
LABEL stage=builder
LABEL service="sms-auth-backend"

# Set working directory
WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies with caching
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sms-auth-backend ./cmd/server

# Stage 2: Production stage
FROM alpine:latest

# Install security updates and certificates
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# Set metadata
LABEL service="sms-auth-backend"
LABEL version="1.0.0"
LABEL description="SmartTransit SMS Authentication Backend API Service"

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/sms-auth-backend .

# Create non-root user for security compliance (Choreo requirement)
RUN adduser -D -s /bin/sh -u 10001 smsauth
USER 10001

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Command to run the SMS Authentication backend
CMD ["./sms-auth-backend"]
