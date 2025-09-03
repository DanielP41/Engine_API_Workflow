#!/bin/bash

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Engine API Workflow - Build and Run Script${NC}"
echo "=================================================="

# Función para manejar errores
handle_error() {
    echo -e "${RED}❌ Error: $1${NC}"
    exit 1
}

# Función para mostrar éxito
show_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# Función para mostrar advertencia
show_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Verificar si Go está instalado
if ! command -v go &> /dev/null; then
    handle_error "Go is not installed. Please install Go 1.23+ first."
fi

# Verificar versión de Go
GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
REQUIRED_VERSION="1.21"

if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    handle_error "Go version $GO_VERSION is too old. Please upgrade to Go 1.21 or higher."
fi

show_success "Go version $GO_VERSION is compatible"

# Crear archivo .env si no existe
if [ ! -f .env ]; then
    echo "📋 Creating .env file from template..."
    cp .env.example .env
    show_success "Created .env file"
else
    show_warning ".env file already exists"
fi

# Limpiar módulos y descargar dependencias
echo "📦 Installing dependencies..."
go mod tidy
if [ $? -ne 0 ]; then
    handle_error "Failed to tidy Go modules"
fi

go mod download
if [ $? -ne 0 ]; then
    handle_error "Failed to download Go modules"
fi

show_success "Dependencies installed"

# Verificar Docker si se pasa el parámetro --docker
if [ "$1" == "--docker" ]; then
    echo "🐳 Starting services with Docker..."
    
    # Verificar si Docker está instalado
    if ! command -v docker &> /dev/null; then
        handle_error "Docker is not installed"
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        handle_error "Docker Compose is not installed"
    fi
    
    # Parar contenedores existentes
    docker-compose down
    
    # Iniciar servicios
    docker-compose up -d mongodb redis
    if [ $? -ne 0 ]; then
        handle_error "Failed to start Docker services"
    fi
    
    show_success "Docker services started"
    
    # Esperar que los servicios estén listos
    echo "⏳ Waiting for services to be ready..."
    sleep 10
fi

# Compilar la aplicación
echo "🔨 Building application..."
go build -o bin/engine-api-workflow cmd/api/main.go
if [ $? -ne 0 ]; then
    handle_error "Failed to build application"
fi

show_success "Application built successfully"

# Ejecutar la aplicación
echo "🚀 Starting Engine API Workflow..."
echo "=================================================="

if [ "$1" == "--background" ] || [ "$2" == "--background" ]; then
    echo "Starting in background mode..."
    nohup ./bin/engine-api-workflow > app.log 2>&1 &
    PID=$!
    echo $PID > app.pid
    show_success "Application started in background (PID: $PID)"
    echo "📝 Logs available in app.log"
    echo "🛑 Stop with: kill $PID or ./scripts/stop.sh"
else
    echo "Starting in foreground mode..."
    echo "📝 Press Ctrl+C to stop"
    echo "🌐 API will be available at: http://localhost:8081"
    echo "🔍 Health check: http://localhost:8081/api/v1/health"
    echo "=================================================="
    
    ./bin/engine-api-workflow
fi