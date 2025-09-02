.PHONY: run build test clean docker-up docker-down dev-setup

# Variables
APP_NAME=engine-api-workflow
DOCKER_COMPOSE=docker-compose
GO_VERSION=1.23.5
MAIN_PATH=cmd/api/main.go

# Colors for output
RED=\033[31m
GREEN=\033[32m
YELLOW=\033[33m
BLUE=\033[34m
RESET=\033[0m

# Development commands
run:
	@echo "$(GREEN)Starting application...$(RESET)"
	@if [ -f .env ]; then \
		export $$(cat .env | xargs) && go run $(MAIN_PATH); \
	else \
		echo "$(YELLOW)Warning: .env file not found, using defaults$(RESET)"; \
		go run $(MAIN_PATH); \
	fi

dev:
	@echo "$(BLUE)Starting in development mode with hot reload...$(RESET)"
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "$(YELLOW)Air not found. Install with: go install github.com/cosmtrek/air@latest$(RESET)"; \
		make run; \
	fi

build:
	@echo "$(GREEN)Building application...$(RESET)"
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/$(APP_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build completed: bin/$(APP_NAME)$(RESET)"

build-local:
	@echo "$(GREEN)Building for local OS...$(RESET)"
	@go build -o bin/$(APP_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Local build completed: bin/$(APP_NAME)$(RESET)"

# Testing commands
test:
	@echo "$(BLUE)Running tests...$(RESET)"
	@go test -v ./...

test-coverage:
	@echo "$(BLUE)Running tests with coverage...$(RESET)"
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(RESET)"

test-integration:
	@echo "$(BLUE)Running integration tests...$(RESET)"
	@go test -v -tags=integration ./tests/integration/...

# Docker commands
docker-build:
	@echo "$(BLUE)Building Docker image...$(RESET)"
	@docker build -t $(APP_NAME):latest .

docker-up:
	@echo "$(BLUE)Starting services with Docker Compose...$(RESET)"
	@$(DOCKER_COMPOSE) up -d
	@echo "$(GREEN)Services started successfully$(RESET)"

docker-down:
	@echo "$(YELLOW)Stopping Docker services...$(RESET)"
	@$(DOCKER_COMPOSE) down

docker-logs:
	@echo "$(BLUE)Showing Docker logs...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f

docker-rebuild:
	@echo "$(BLUE)Rebuilding and restarting services...$(RESET)"
	@$(DOCKER_COMPOSE) down
	@$(DOCKER_COMPOSE) up -d --build
	@echo "$(GREEN)Services rebuilt and restarted$(RESET)"

docker-clean:
	@echo "$(YELLOW)Cleaning Docker resources...$(RESET)"
	@$(DOCKER_COMPOSE) down -v --remove-orphans
	@docker system prune -f

# Database commands
db-up:
	@echo "$(BLUE)Starting only database services...$(RESET)"
	@$(DOCKER_COMPOSE) up -d mongodb redis

db-migrate:
	@echo "$(BLUE)Running database migrations...$(RESET)"
	@echo "$(YELLOW)Migration system not implemented yet$(RESET)"

db-seed:
	@echo "$(BLUE)Seeding database...$(RESET)"
	@echo "$(YELLOW)Seeding system not implemented yet$(RESET)"

# Development setup
dev-setup: clean deps docker-up
	@echo "$(GREEN)Development environment setup complete!$(RESET)"
	@echo "$(BLUE)You can now run 'make run' to start the application$(RESET)"

# Dependency management
deps:
	@echo "$(BLUE)Installing dependencies...$(RESET)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Dependencies installed$(RESET)"

deps-update:
	@echo "$(BLUE)Updating dependencies...$(RESET)"
	@go get -u ./...
	@go mod tidy
	@echo "$(GREEN)Dependencies updated$(RESET)"

# Cleanup commands
clean:
	@echo "$(YELLOW)Cleaning up...$(RESET)"
	@go mod tidy
	@go clean
	@rm -f bin/$(APP_NAME)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)Cleanup completed$(RESET)"

clean-all: clean docker-clean
	@echo "$(GREEN)Full cleanup completed$(RESET)"

# Code quality
lint:
	@echo "$(BLUE)Running linter...$(RESET)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(RESET)"; \
	fi

format:
	@echo "$(BLUE)Formatting code...$(RESET)"
	@go fmt ./...
	@echo "$(GREEN)Code formatted$(RESET)"

# Generate documentation
docs:
	@echo "$(BLUE)Generating documentation...$(RESET)"
	@if command -v swag > /dev/null; then \
		swag init -g $(MAIN_PATH) -o docs/api; \
	else \
		echo "$(YELLOW)swag not found. Install with: go install github.com/swaggo/swag/cmd/swag@latest$(RESET)"; \
	fi

# Environment setup
env-copy:
	@if [ ! -f .env ]; then \
		echo "$(BLUE)Copying .env.example to .env...$(RESET)"; \
		cp .env.example .env; \
		echo "$(GREEN).env file created. Please review and update values as needed.$(RESET)"; \
	else \
		echo "$(YELLOW).env file already exists$(RESET)"; \
	fi

# Health checks
health-check:
	@echo "$(BLUE)Checking application health...$(RESET)"
	@curl -f http://localhost:8081/api/v1/health || echo "$(RED)Health check failed$(RESET)"

# Show help
help:
	@echo "$(GREEN)Available commands:$(RESET)"
	@echo "  $(BLUE)Development:$(RESET)"
	@echo "    run          - Run the application"
	@echo "    dev          - Run with hot reload (requires air)"
	@echo "    build        - Build the application for Linux"
	@echo "    build-local  - Build the application for current OS"
	@echo "    dev-setup    - Setup complete development environment"
	@echo ""
	@echo "  $(BLUE)Testing:$(RESET)"
	@echo "    test         - Run unit tests"
	@echo "    test-coverage - Run tests with coverage report"
	@echo "    test-integration - Run integration tests"
	@echo ""
	@echo "  $(BLUE)Docker:$(RESET)"
	@echo "    docker-build   - Build Docker image"
	@echo "    docker-up      - Start services with Docker Compose"
	@echo "    docker-down    - Stop Docker services"
	@echo "    docker-logs    - Show Docker logs"
	@echo "    docker-rebuild - Rebuild and restart services"
	@echo "    docker-clean   - Clean Docker resources"
	@echo ""
	@echo "  $(BLUE)Database:$(RESET)"
	@echo "    db-up        - Start only database services"
	@echo "    db-migrate   - Run database migrations"
	@echo "    db-seed      - Seed database with test data"
	@echo ""
	@echo "  $(BLUE)Utilities:$(RESET)"
	@echo "    clean        - Clean build artifacts"
	@echo "    clean-all    - Clean everything including Docker"
	@echo "    deps         - Install dependencies"
	@echo "    deps-update  - Update all dependencies"
	@echo "    lint         - Run linter"
	@echo "    format       - Format code"
	@echo "    docs         - Generate API documentation"
	@echo "    env-copy     - Copy .env.example to .env"
	@echo "    health-check - Check application health"