.PHONY: run build test clean docker-up docker-down

# Variables
APP_NAME=workflow-engine
DOCKER_COMPOSE=docker-compose

# Development commands
run:
	go run cmd/api/main.go

build:
	go build -o bin/$(APP_NAME) cmd/api/main.go

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Docker commands
docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-rebuild:
	$(DOCKER_COMPOSE) up -d --build

# Database commands
db-migrate:
	@echo "Running database migrations..."
	# Agregar comandos de migración aquí

# Cleanup
clean:
	go mod tidy
	go clean
	rm -f bin/$(APP_NAME)

# Install dependencies
deps:
	go mod download
	go mod tidy

# Generate swagger docs
swagger:
	swag init -g cmd/api/main.go -o docs/api