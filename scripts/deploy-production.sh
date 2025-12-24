#!/bin/bash

# ============================================
# Engine API Workflow - Production Deployment Script
# ============================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Check if .env.production exists
check_env_file() {
    if [ ! -f .env.production ]; then
        print_error ".env.production file not found!"
        print_info "Creating from .env.example..."
        cp .env.example .env.production
        print_warning "Please edit .env.production with your production values before continuing!"
        exit 1
    fi
    print_success ".env.production file found"
}

# Check Docker installation
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed!"
        exit 1
    fi
    print_success "Docker is installed"
    
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed!"
        exit 1
    fi
    print_success "Docker Compose is installed"
}

# Load environment variables
load_env() {
    export $(cat .env.production | grep -v '^#' | xargs)
    print_success "Environment variables loaded"
}

# Build and start services
deploy_services() {
    print_info "Building and starting services..."
    docker-compose -f docker-compose.prod.yml up -d --build
    print_success "Services started"
}

# Wait for services to be healthy
wait_for_health() {
    print_info "Waiting for services to be healthy..."
    
    # Wait for API
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -f http://localhost:8081/api/v1/health &> /dev/null; then
            print_success "API is healthy"
            return 0
        fi
        attempt=$((attempt + 1))
        echo -n "."
        sleep 2
    done
    
    print_error "API failed to become healthy"
    return 1
}

# Verify deployment
verify_deployment() {
    print_header "Verifying Deployment"
    
    # Check containers
    print_info "Checking containers..."
    if docker ps | grep -q "workflow_api_prod"; then
        print_success "API container is running"
    else
        print_error "API container is not running"
        return 1
    fi
    
    if docker ps | grep -q "workflow_mongodb_prod"; then
        print_success "MongoDB container is running"
    else
        print_error "MongoDB container is not running"
        return 1
    fi
    
    if docker ps | grep -q "workflow_redis_prod"; then
        print_success "Redis container is running"
    else
        print_error "Redis container is not running"
        return 1
    fi
    
    # Check health endpoint
    print_info "Checking API health endpoint..."
    local health_response=$(curl -s http://localhost:8081/api/v1/health)
    
    if echo "$health_response" | grep -q "ok"; then
        print_success "API health check passed"
    else
        print_error "API health check failed"
        return 1
    fi
}

# Show deployment info
show_info() {
    print_header "Deployment Information"
    echo ""
    echo -e "${GREEN}API URL:${NC} http://localhost:8081"
    echo -e "${GREEN}Health Check:${NC} http://localhost:8081/api/v1/health"
    echo -e "${GREEN}Environment:${NC} production"
    echo ""
    echo -e "${YELLOW}Next Steps:${NC}"
    echo "1. Create admin user: curl -X POST http://localhost:8081/api/v1/auth/register ..."
    echo "2. Configure Nginx reverse proxy (optional)"
    echo "3. Set up SSL certificate"
    echo "4. Configure monitoring"
    echo ""
    echo -e "${BLUE}View logs:${NC} docker-compose -f docker-compose.prod.yml logs -f"
    echo -e "${BLUE}Stop services:${NC} docker-compose -f docker-compose.prod.yml down"
    echo ""
}

# Backup before deployment
create_backup() {
    if docker ps | grep -q "workflow_mongodb_prod"; then
        print_info "Creating backup before deployment..."
        local backup_dir="./backups/pre-deployment-$(date +%Y%m%d-%H%M%S)"
        mkdir -p "$backup_dir"
        
        docker exec workflow_mongodb_prod mongodump \
            --username=${MONGO_ROOT_USER} \
            --password=${MONGO_ROOT_PASSWORD} \
            --authenticationDatabase=admin \
            --out=/tmp/backup
        
        docker cp workflow_mongodb_prod:/tmp/backup "$backup_dir"
        print_success "Backup created at $backup_dir"
    else
        print_info "No existing deployment found, skipping backup"
    fi
}

# Main deployment flow
main() {
    print_header "Engine API Workflow - Production Deployment"
    
    # Pre-flight checks
    print_header "Pre-flight Checks"
    check_docker
    check_env_file
    load_env
    
    # Ask for confirmation
    echo ""
    print_warning "This will deploy to PRODUCTION environment"
    read -p "Continue? (yes/no): " confirm
    
    if [ "$confirm" != "yes" ]; then
        print_info "Deployment cancelled"
        exit 0
    fi
    
    # Create backup if exists
    create_backup
    
    # Deploy
    print_header "Deploying Services"
    deploy_services
    
    # Wait for health
    wait_for_health
    
    # Verify
    verify_deployment
    
    # Show info
    show_info
    
    print_success "Deployment completed successfully! 🚀"
}

# Run main function
main
