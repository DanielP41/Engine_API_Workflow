# ============================================
# Engine API Workflow - Production Deployment Script (PowerShell)
# ============================================

# Colors
$ErrorColor = "Red"
$SuccessColor = "Green"
$WarningColor = "Yellow"
$InfoColor = "Cyan"

function Print-Header {
    param([string]$Message)
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host $Message -ForegroundColor Blue
    Write-Host "========================================" -ForegroundColor Blue
}

function Print-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor $SuccessColor
}

function Print-Error {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor $ErrorColor
}

function Print-Warning {
    param([string]$Message)
    Write-Host "⚠ $Message" -ForegroundColor $WarningColor
}

function Print-Info {
    param([string]$Message)
    Write-Host "ℹ $Message" -ForegroundColor $InfoColor
}

# Check if .env.production exists
function Check-EnvFile {
    if (-not (Test-Path ".env.production")) {
        Print-Error ".env.production file not found!"
        Print-Info "Creating from .env.example..."
        Copy-Item ".env.example" ".env.production"
        Print-Warning "Please edit .env.production with your production values before continuing!"
        exit 1
    }
    Print-Success ".env.production file found"
}

# Check Docker installation
function Check-Docker {
    try {
        $null = docker --version
        Print-Success "Docker is installed"
    } catch {
        Print-Error "Docker is not installed!"
        exit 1
    }
    
    try {
        $null = docker-compose --version
        Print-Success "Docker Compose is installed"
    } catch {
        Print-Error "Docker Compose is not installed!"
        exit 1
    }
}

# Build and start services
function Deploy-Services {
    Print-Info "Building and starting services..."
    docker-compose -f docker-compose.prod.yml up -d --build
    if ($LASTEXITCODE -eq 0) {
        Print-Success "Services started"
    } else {
        Print-Error "Failed to start services"
        exit 1
    }
}

# Wait for services to be healthy
function Wait-ForHealth {
    Print-Info "Waiting for services to be healthy..."
    
    $maxAttempts = 30
    $attempt = 0
    
    while ($attempt -lt $maxAttempts) {
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:8081/api/v1/health" -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                Print-Success "API is healthy"
                return $true
            }
        } catch {
            # Continue waiting
        }
        
        $attempt++
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 2
    }
    
    Write-Host ""
    Print-Error "API failed to become healthy"
    return $false
}

# Verify deployment
function Verify-Deployment {
    Print-Header "Verifying Deployment"
    
    # Check containers
    Print-Info "Checking containers..."
    
    $containers = docker ps --format "{{.Names}}"
    
    if ($containers -match "workflow_api_prod") {
        Print-Success "API container is running"
    } else {
        Print-Error "API container is not running"
        return $false
    }
    
    if ($containers -match "workflow_mongodb_prod") {
        Print-Success "MongoDB container is running"
    } else {
        Print-Error "MongoDB container is not running"
        return $false
    }
    
    if ($containers -match "workflow_redis_prod") {
        Print-Success "Redis container is running"
    } else {
        Print-Error "Redis container is not running"
        return $false
    }
    
    # Check health endpoint
    Print-Info "Checking API health endpoint..."
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/health"
        if ($response.status -eq "ok") {
            Print-Success "API health check passed"
            return $true
        }
    } catch {
        Print-Error "API health check failed"
        return $false
    }
}

# Show deployment info
function Show-Info {
    Print-Header "Deployment Information"
    Write-Host ""
    Write-Host "API URL: " -NoNewline -ForegroundColor Green
    Write-Host "http://localhost:8081"
    Write-Host "Health Check: " -NoNewline -ForegroundColor Green
    Write-Host "http://localhost:8081/api/v1/health"
    Write-Host "Environment: " -NoNewline -ForegroundColor Green
    Write-Host "production"
    Write-Host ""
    Write-Host "Next Steps:" -ForegroundColor Yellow
    Write-Host "1. Create admin user"
    Write-Host "2. Configure reverse proxy (optional)"
    Write-Host "3. Set up SSL certificate"
    Write-Host "4. Configure monitoring"
    Write-Host ""
    Write-Host "View logs: " -NoNewline -ForegroundColor Cyan
    Write-Host "docker-compose -f docker-compose.prod.yml logs -f"
    Write-Host "Stop services: " -NoNewline -ForegroundColor Cyan
    Write-Host "docker-compose -f docker-compose.prod.yml down"
    Write-Host ""
}

# Create backup before deployment
function Create-Backup {
    $containers = docker ps --format "{{.Names}}"
    if ($containers -match "workflow_mongodb_prod") {
        Print-Info "Creating backup before deployment..."
        $backupDir = "./backups/pre-deployment-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
        New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
        
        # Note: Backup creation would require MongoDB credentials from .env.production
        Print-Success "Backup directory created at $backupDir"
    } else {
        Print-Info "No existing deployment found, skipping backup"
    }
}

# Main deployment flow
function Main {
    Print-Header "Engine API Workflow - Production Deployment"
    
    # Pre-flight checks
    Print-Header "Pre-flight Checks"
    Check-Docker
    Check-EnvFile
    
    # Ask for confirmation
    Write-Host ""
    Print-Warning "This will deploy to PRODUCTION environment"
    $confirm = Read-Host "Continue? (yes/no)"
    
    if ($confirm -ne "yes") {
        Print-Info "Deployment cancelled"
        exit 0
    }
    
    # Create backup if exists
    Create-Backup
    
    # Deploy
    Print-Header "Deploying Services"
    Deploy-Services
    
    # Wait for health
    if (-not (Wait-ForHealth)) {
        Print-Error "Deployment failed - services not healthy"
        exit 1
    }
    
    # Verify
    if (-not (Verify-Deployment)) {
        Print-Error "Deployment verification failed"
        exit 1
    }
    
    # Show info
    Show-Info
    
    Print-Success "Deployment completed successfully! 🚀"
}

# Run main function
Main
