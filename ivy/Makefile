.PHONY: dev build test clean vendor tidy lint

# Development
dev:
	@echo "Starting Ivy in development mode..."
	@cd cmd && air

# Build
build:
	@echo "Building Ivy..."
	@go build -o bin/ivy ./cmd

# Test
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean
clean:
	@echo "Cleaning..."
	@rm -rf bin/ tmp/ vendor/

# Vendor dependencies
vendor:
	@echo "Vendoring dependencies..."
	@go mod vendor

# Tidy
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Lint
lint:
	@echo "Linting..."
	@golangci-lint run ./...

# Database
db-up:
	@echo "Running migrations..."
	@go run ./cmd migrate up

db-down:
	@echo "Rolling back migrations..."
	@go run ./cmd migrate down

# Docker
docker-build:
	@echo "Building Docker image..."
	@docker build -t ivy:latest .

docker-run:
	@echo "Running Docker container..."
	@docker run -p 3002:3002 --env-file .env ivy:latest

