#!/bin/bash

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🛑 Engine API Workflow - Stop Script${NC}"
echo "============================================"

# Función para mostrar éxito
show_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# Función para mostrar advertencia
show_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Parar aplicación en background si existe PID file
if [ -f app.pid ]; then
    PID=$(cat app.pid)
    if ps -p $PID > /dev/null 2>&1; then
        echo "🔄 Stopping application (PID: $PID)..."
        kill $PID
        
        # Esperar que el proceso termine
        sleep 2
        
        if ps -p $PID > /dev/null 2>&1; then
            echo "🔨 Force killing application..."
            kill -9 $PID
        fi
        
        show_success "Application stopped"
    else
        show_warning "Application with PID $PID is not running"
    fi
    
    rm -f app.pid
else
    show_warning "No PID file found. Application might not be running in background."
fi

# Parar contenedores Docker si están corriendo
if command -v docker-compose &> /dev/null || docker compose version &> /dev/null; then
    if [ -f docker-compose.yml ]; then
        echo "🐳 Stopping Docker services..."
        docker-compose down
        show_success "Docker services stopped"
    fi
fi

# Limpiar archivos temporales
if [ -f app.log ]; then
    echo "🧹 Cleaning up log file..."
    rm -f app.log
    show_success "Log file cleaned"
fi

echo "============================================"
show_success "Cleanup completed"