.PHONY: help run build test clean docker-up docker-down docker-rebuild install deps fmt lint vet

# Variables
APP_NAME=engine-api-workflow
DOCKER_COMPOSE=docker-compose
GO_VERSION=1.23
BINARY_PATH=bin/$(APP_NAME)

# Default target
help: ## Show this help message
	@echo "Engine API Workflow - Available commands:"
	@echo "==========================================="
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development commands
run: deps ## Run the application locally
	@echo "🚀 Starting Engine API Workflow..."
	@if [ ! -f .env ]; then cp .env.example .env; echo "📋 Created .env file"; fi
	go run cmd/api/main.go

build: deps ## Build the application
	@echo "🔨 Building $(APP_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o $(BINARY_PATH) cmd/api/main.go
	@echo "✅ Build completed: $(BINARY_PATH)"

build-local: deps ## Build for local OS
	@echo "🔨 Building $(APP_NAME) for local system..."
	@mkdir -p bin
	go build -o $(BINARY_PATH) cmd/api/main.go
	@echo "✅ Build completed: $(BINARY_PATH)"

install: ## Install the application globally
	go install cmd/api/main.go

# Testing
test: ## Run tests
	@echo "🧪 Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "🧪 Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

test-race: ## Run tests with race detector
	@echo "🧪 Running tests with race detector..."
	go test -race -v ./...

# Docker commands
docker-up: ## Start all services with Docker
	@echo "🐳 Starting Docker services..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Docker services started"

docker-down: ## Stop all Docker services
	@echo "🐳 Stopping Docker services..."
	$(DOCKER_COMPOSE) down
	@echo "✅ Docker services stopped"

docker-logs: ## Show Docker logs
	$(DOCKER_COMPOSE) logs -f

docker-rebuild: ## Rebuild and start Docker services
	@echo "🐳 Rebuilding Docker services..."
	$(DOCKER_COMPOSE) up -d --build
	@echo "✅ Docker services rebuilt and started"

docker-clean: ## Clean Docker containers and volumes
	@echo "🧹 Cleaning Docker resources..."
	$(DOCKER_COMPOSE) down -v --remove-orphans
	docker system prune -f
	@echo "✅ Docker cleanup completed"

# Database commands
db-up: ## Start only database services
	@echo "🗄️ Starting database services..."
	$(DOCKER_COMPOSE) up -d mongodb redis
	@echo "✅ Database services started"

db-down: ## Stop database services
	@echo "🗄️ Stopping database services..."
	$(DOCKER_COMPOSE) stop mongodb redis
	@echo "✅ Database services stopped"

db-reset: ## Reset database (WARNING: This will delete all data)
	@echo "⚠️  Resetting database - all data will be lost!"
	@read -p "Are you sure? [y/N]: " confirm && [ "$$confirm" = "y" ]
	$(DOCKER_COMPOSE) down mongodb redis
	docker volume rm $$(docker volume ls -q | grep workflow) 2>/dev/null || true
	$(DOCKER_COMPOSE) up -d mongodb redis
	@echo "✅ Database reset completed"

# Code quality
deps: ## Download and tidy dependencies
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

fmt: ## Format Go code
	@echo "🎨 Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted"

lint: ## Run golangci-lint
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2"; \
	fi

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	go vet ./...
	@echo "✅ Vet completed"

security: ## Run gosec security scanner
	@echo "🔒 Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "⚠️  gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Cleanup
clean: ## Clean build artifacts and dependencies
	@echo "🧹 Cleaning up..."
	go clean
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f app.log app.pid
	@echo "✅ Cleanup completed"

clean-all: clean docker-clean ## Clean everything including Docker resources

# Environment setup
setup: ## Initial project setup
	@echo "🚀 Setting up Engine API Workflow..."
	@if [ ! -f .env ]; then cp .env.example .env; echo "📋 Created .env file"; fi
	@chmod +x scripts/*.sh 2>/dev/null || true
	$(MAKE) deps
	$(MAKE) fmt
	@echo "✅ Setup completed"

# Development helpers
dev: db-up ## Start development environment
	@echo "🛠️ Starting development environment..."
	@sleep 5  # Wait for databases to be ready
	$(MAKE) run

dev-reset: db-reset ## Reset development environment
	@echo "🔄 Resetting development environment..."
	$(MAKE) clean
	$(MAKE) deps
	@echo "✅ Development environment reset"

# Production helpers
prod-build: ## Build production binary
	@echo "🏭 Building production binary..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s -X main.version=$$(git describe --tags --always --dirty)" \
		-o $(BINARY_PATH) cmd/api/main.go
	@echo "✅ Production build completed"

# Health checks
health: ## Check application health
	@echo "🏥 Checking application health..."
	@curl -f http://localhost:8081/api/v1/health || echo "❌ Application is not responding"

check-deps: ## Verify system dependencies
	@echo "🔍 Checking system dependencies..."
	@echo "Go version: $$(go version)"
	@docker --version 2>/dev/null || echo "⚠️  Docker not found"
	@docker-compose --version 2>/dev/null || docker compose version 2>/dev/null || echo "⚠️  Docker Compose not found"
	@echo "✅ Dependency check completed"

# Show application info
info: ## Show application information
	@echo "Engine API Workflow Information"
	@echo "==============================="
	@echo "App Name: $(APP_NAME)"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Binary Path: $(BINARY_PATH)"
	@echo "Docker Compose: $(DOCKER_COMPOSE)"
	@if [ -f $(BINARY_PATH) ]; then echo "Binary Size: $$(du -h $(BINARY_PATH) | cut -f1)"; fi