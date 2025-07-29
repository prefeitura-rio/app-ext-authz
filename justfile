# Justfile for Envoy reCAPTCHA Enterprise External Authorization Service

# Default recipe
default:
    @echo "🚀 Envoy reCAPTCHA Enterprise External Authorization Service"
    @echo "Available commands:"
    @just --list

# Build the binary
build:
    @echo "🔨 Building binary..."
    @go build -o bin/recaptcha-authz cmd/main.go

# Run locally
run:
    @echo "🚀 Running locally..."
    @go run cmd/main.go

# Run in mock mode (bypasses Google API)
run-mock:
    @echo "🎭 Running in mock mode..."
    @MOCK_MODE=true go run cmd/main.go

# Run in production mode (real Google API)
run-prod:
    @echo "🚀 Running in production mode..."
    @MOCK_MODE=false go run cmd/main.go

# Build Docker image
docker-build:
    @echo "🐳 Building Docker image..."
    @docker build -t recaptcha-authz:latest .

# Run with Docker Compose
docker-compose:
    @echo "🐳 Running with Docker Compose..."
    @docker-compose up --build

# Clean up
clean:
    @echo "🧹 Cleaning up..."
    @rm -rf bin/
    @go clean -cache

# Install dependencies
install:
    @echo "📦 Installing dependencies..."
    @go mod download

# Tidy dependencies
tidy:
    @echo "🧹 Tidying dependencies..."
    @go mod tidy
    @go mod verify

# Lint code
lint:
    @echo "🔍 Linting code..."
    @golangci-lint run

# Format code
fmt:
    @echo "✨ Formatting code..."
    @go fmt ./...
    @goimports -w .

# Show help
help:
    @echo "Available commands:"
    @just --list

# Test the service with curl (HTTP mode)
test-curl-http:
    @echo "🌐 Testing service with curl (HTTP mode)..."
    @curl -X POST http://localhost:8000 \
        -H "X-Recaptcha-Token: test_token" \
        -v

# Test the service with curl (gRPC mode - requires grpcurl)
test-curl-grpc:
    @echo "🌐 Testing service with grpcurl (gRPC mode)..."
    @grpcurl -plaintext -d '{"attributes": {"request": {"http": {"headers": {"x-recaptcha-token": "test_token"}}}}}' localhost:9000 envoy.service.auth.v3.Authorization/Check 