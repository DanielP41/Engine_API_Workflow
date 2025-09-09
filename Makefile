.PHONY: run build test clean docker-up docker-down worker-test

# Variables
APP_NAME=workflow-engine
DOCKER_COMPOSE=docker-compose
WORKER_PORT=8082

# Development commands
run:
	@powershell -Command "& { . .\.env; go run cmd/api/main.go }"

dev:
	@echo "Starting development mode with hot reload..."
	@powershell -Command "& { . .\.env; air }" || go run cmd/api/main.go

build:
	go build -o bin/$(APP_NAME) cmd/api/main.go

# Testing commands
test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

test-workers:
	@echo "Testing worker functionality..."
	@echo "1. Checking worker health..."
	curl -s http://localhost:$(WORKER_PORT)/health || echo "Worker stats server not running"
	@echo "\n2. Checking worker stats..."
	curl -s http://localhost:$(WORKER_PORT)/stats || echo "Worker stats not available"

# Docker commands
docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-logs-api:
	$(DOCKER_COMPOSE) logs -f api

docker-rebuild:
	$(DOCKER_COMPOSE) up -d --build

# API Testing commands
test-api:
	@echo "Testing API endpoints..."
	@echo "1. Health check:"
	curl -s http://localhost:8081/api/v1/health | jq . || echo "API not responding"
	@echo "\n2. Worker health:"
	curl -s http://localhost:8081/api/v1/workers/health | jq . || echo "Workers not available"

test-auth:
	@echo "Testing authentication..."
	@echo "1. Register test user:"
	curl -X POST http://localhost:8081/api/v1/auth/register \
		-H "Content-Type: application/json" \
		-d '{"name":"Test User","email":"test@example.com","password":"password123","role":"user"}' \
		| jq . || echo "Registration failed"

test-workflow:
	@echo "Testing workflow creation (requires auth token)..."
	@echo "Please get auth token first with 'make test-auth'"

# Queue testing
test-queue:
	@echo "Testing queue operations..."
	curl -s http://localhost:8081/api/v1/workers/queue/stats | jq . || echo "Queue stats not available"

# Database commands
db-migrate:
	@echo "Running database migrations..."
	# Agregar comandos de migración aquí

db-seed:
	@echo "Seeding database with test data..."
	# Agregar comandos para datos de prueba

# Cleanup
clean:
	go mod tidy
	go clean
	rm -f bin/$(APP_NAME)
	rm -f coverage.out

clean-docker:
	docker-compose down -v
	docker system prune -f

# Install dependencies
deps:
	go mod download
	go mod tidy

# Development tools
install-air:
	go install github.com/cosmtrek/air@latest

install-tools: install-air
	@echo "Development tools installed"

# Generate swagger docs
swagger:
	swag init -g cmd/api/main.go -o docs/api

# Monitoring and logs
logs-live:
	$(DOCKER_COMPOSE) logs -f api

logs-workers:
	curl -s http://localhost:$(WORKER_PORT)/stats | jq .

logs-queue:
	curl -s http://localhost:8081/api/v1/workers/queue/stats | jq .

# Performance testing
benchmark:
	@echo "Running performance tests..."
	go test -bench=. -benchmem ./...

load-test:
	@echo "Running load test on health endpoint..."
	@for i in {1..10}; do \
		curl -s http://localhost:8081/api/v1/health > /dev/null && echo "Request $$i: OK" || echo "Request $$i: FAILED"; \
	done

# Production commands
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/$(APP_NAME) cmd/api/main.go

docker-prod:
	$(DOCKER_COMPOSE) -f docker-compose.prod.yml up -d

# Help
help:
	@echo "Available commands:"
	@echo "  run              - Run the application locally"
	@echo "  dev              - Run with hot reload (requires air)"
	@echo "  build            - Build the application"
	@echo "  test             - Run all tests"
	@echo "  test-workers     - Test worker functionality"
	@echo "  test-api         - Test API endpoints"
	@echo "  test-auth        - Test authentication"
	@echo "  docker-up        - Start Docker containers"
	@echo "  docker-down      - Stop Docker containers"
	@echo "  docker-rebuild   - Rebuild and start containers"
	@echo "  clean            - Clean build artifacts"
	@echo "  swagger          - Generate API documentation"
	@echo "  help             - Show this help message"