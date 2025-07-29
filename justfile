# Justfile for Envoy reCAPTCHA Authz Service

# Default target
default:
    @just --list

# Run the service locally
run:
    @echo "🚀 Starting recaptcha-authz service..."
    @go run cmd/main.go

# Run with hot reload (requires air)
dev:
    @echo "🔄 Starting development server with hot reload..."
    @air

# Build the binary
build:
    @echo "🔨 Building binary..."
    @go build -o bin/recaptcha-authz cmd/main.go

# Run tests
test:
    @echo "🧪 Running tests..."
    @go test -v ./...

# Run tests with coverage
test-coverage:
    @echo "📊 Running tests with coverage..."
    @go test -v -coverprofile=coverage.out ./...
    @go tool cover -html=coverage.out -o coverage.html
    @echo "📄 Coverage report generated: coverage.html"

# Run integration tests
test-integration:
    @echo "🔗 Running integration tests..."
    @go test -v -tags=integration ./...

# Run load tests
test-load:
    @echo "⚡ Running load tests..."
    @go test -v -tags=load ./test/load/

# Run benchmarks
bench:
    @echo "🏃 Running benchmarks..."
    @go test -bench=. ./...

# Lint code
lint:
    @echo "🔍 Linting code..."
    @golangci-lint run

# Format code
fmt:
    @echo "✨ Formatting code..."
    @go fmt ./...
    @goimports -w .

# Tidy dependencies
tidy:
    @echo "🧹 Tidying dependencies..."
    @go mod tidy
    @go mod verify

# Generate mocks
mocks:
    @echo "🎭 Generating mocks..."
    @mockgen -source=internal/recaptcha/client.go -destination=internal/recaptcha/mocks.go

# Build Docker image
docker-build:
    @echo "🐳 Building Docker image..."
    @docker build -t recaptcha-authz:latest .

# Clean up
clean:
    @echo "🧹 Cleaning up..."
    @rm -rf bin/
    @rm -f coverage.out coverage.html
    @go clean -cache

# Install dependencies
install:
    @echo "📦 Installing dependencies..."
    @go mod download
    @go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    @go install github.com/vektra/mockery/v2@latest
    @go install github.com/cosmtrek/air@latest

# Show help
help:
    @echo "Available commands:"
    @just --list

# Run with mock mode (bypasses Google API)
run-mock:
    @echo "🎭 Running in mock mode..."
    @MOCK_MODE=true go run cmd/main.go

# Test the service with curl
test-curl:
    @echo "🌐 Testing service with curl..."
    @curl -X POST http://localhost:8080/authz \
        -H "X-Recaptcha-Token: test_token" \
        -v

# Health check
health:
    @echo "🏥 Checking service health..."
    @curl -X GET http://localhost:8080/health

# Metrics
metrics:
    @echo "📊 Getting metrics..."
    @curl -X GET http://localhost:8080/metrics 