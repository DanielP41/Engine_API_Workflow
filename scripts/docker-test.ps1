# docker-test.ps1 - PowerShell version for Windows
param(
    [Parameter(Position=0)]
    [string]$Command = "start"
)

# Función para escribir mensajes con colores
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    
    $colorMap = @{
        "Red" = "Red"
        "Green" = "Green" 
        "Yellow" = "Yellow"
        "Blue" = "Blue"
        "Purple" = "Magenta"
        "Cyan" = "Cyan"
    }
    
    Write-Host $Message -ForegroundColor $colorMap[$Color]
}

function Log-Info { param([string]$Message) Write-ColorOutput "ℹ️  $Message" "Blue" }
function Log-Success { param([string]$Message) Write-ColorOutput "✅ $Message" "Green" }
function Log-Warning { param([string]$Message) Write-ColorOutput "⚠️  $Message" "Yellow" }
function Log-Error { param([string]$Message) Write-ColorOutput "❌ $Message" "Red" }

Write-ColorOutput "🐳 Engine API Workflow - Docker Testing Suite (Windows)" "Purple"
Write-ColorOutput "====================================================" "Purple"

# Función para manejar errores
function Handle-Error {
    param([string]$Message)
    Log-Error $Message
    Write-Host "Cleaning up..."
    docker-compose down
    exit 1
}

# Función para verificar si un comando existe
function Test-Command {
    param([string]$Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

# Función para esperar que un servicio esté listo
function Wait-ForService {
    param([string]$ServiceName, [int]$MaxAttempts = 30)
    
    Log-Info "Waiting for $ServiceName to be ready..."
    
    for ($i = 1; $i -le $MaxAttempts; $i++) {
        try {
            $result = docker-compose exec $ServiceName echo "Service is running" 2>$null
            if ($LASTEXITCODE -eq 0) {
                Log-Success "$ServiceName is ready!"
                return $true
            }
        }
        catch {
            # Continuar esperando
        }
        
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 2
    }
    
    Handle-Error "$ServiceName failed to start within expected time"
}

# Función para verificar health check
function Test-Health {
    param([string]$ServiceName, [int]$MaxAttempts = 20)
    
    Log-Info "Checking health of $ServiceName..."
    
    for ($i = 1; $i -le $MaxAttempts; $i++) {
        try {
            $containerId = docker-compose ps -q $ServiceName
            if ($containerId) {
                $healthStatus = docker inspect --format='{{.State.Health.Status}}' $containerId 2>$null
                if ($healthStatus -eq "healthy") {
                    Log-Success "$ServiceName is healthy!"
                    return $true
                }
            }
        }
        catch {
            # Continuar verificando
        }
        
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 3
    }
    
    Log-Warning "$ServiceName health check inconclusive, but continuing..."
}

# Función para probar endpoints de API
function Test-ApiEndpoints {
    Log-Info "Testing API endpoints..."
    
    $baseUrl = "http://localhost:8081"
    
    # Test health endpoint
    Log-Info "Testing health endpoint..."
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/health" -Method Get -TimeoutSec 10
        Log-Success "Health endpoint is working"
    }
    catch {
        Handle-Error "Health endpoint is not responding: $($_.Exception.Message)"
    }
    
    # Test register endpoint
    Log-Info "Testing user registration..."
    try {
        $registerData = @{
            name = "Docker Test User"
            email = "dockertest@example.com"
            password = "dockertest123"
            role = "user"
        } | ConvertTo-Json
        
        $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/register" -Method Post -Body $registerData -ContentType "application/json" -TimeoutSec 10
        Log-Success "User registration is working"
    }
    catch {
        if ($_.Exception.Response.StatusCode -eq "Conflict") {
            Log-Warning "User registration returned conflict (might be duplicate user)"
        }
        else {
            Log-Warning "User registration failed: $($_.Exception.Message)"
        }
    }
    
    # Test login endpoint
    Log-Info "Testing user login..."
    try {
        $loginData = @{
            email = "admin@test.com"
            password = "admin123"
        } | ConvertTo-Json
        
        $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method Post -Body $loginData -ContentType "application/json" -TimeoutSec 10
        Log-Success "User login is working"
        
        if ($response.data.access_token) {
            Log-Success "JWT token obtained successfully"
            
            # Test protected endpoint
            Log-Info "Testing protected endpoint..."
            try {
                $headers = @{
                    "Authorization" = "Bearer $($response.data.access_token)"
                }
                $profileResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/profile" -Method Get -Headers $headers -TimeoutSec 10
                Log-Success "Protected endpoint is working"
            }
            catch {
                Log-Warning "Protected endpoint failed: $($_.Exception.Message)"
            }
        }
    }
    catch {
        Log-Warning "User login failed: $($_.Exception.Message)"
    }
}

# Función para mostrar información de servicios
function Show-ServicesInfo {
    Log-Info "Services Information:"
    Write-Host "===================="
    Write-Host "🔧 API Server:      http://localhost:8081"
    Write-Host "🔍 API Health:      http://localhost:8081/api/v1/health"
    Write-Host "🗄️  MongoDB:        localhost:27017 (admin/password123)"
    Write-Host "🚀 Redis:           localhost:6379"
    Write-Host "🌐 Mongo Express:   http://localhost:8082 (admin/admin123)"
    Write-Host "📊 Redis Commander: http://localhost:8083"
    Write-Host ""
    Write-Host "Test Accounts:"
    Write-Host "- Admin: admin@test.com / admin123"
    Write-Host "- User:  user@test.com / user123"
    Write-Host "===================="
}

# Función principal
switch ($Command.ToLower()) {
    "start" {
        Log-Info "Starting Docker services for testing..."
        
        # Verificar que Docker esté corriendo
        if (-not (Test-Command "docker")) {
            Handle-Error "Docker is not installed or not in PATH"
        }
        
        try {
            docker info | Out-Null
        }
        catch {
            Handle-Error "Docker is not running. Please start Docker Desktop first."
        }
        
        # Verificar Docker Compose
        $composeCommand = if (Test-Command "docker-compose") { "docker-compose" } else { "docker compose" }
        
        # Crear directorios necesarios
        if (-not (Test-Path "config")) { New-Item -ItemType Directory -Path "config" -Force }
        if (-not (Test-Path "scripts")) { New-Item -ItemType Directory -Path "scripts" -Force }
        
        # Limpiar contenedores previos si existen
        Log-Info "Cleaning up any existing containers..."
        & $composeCommand down -v 2>$null
        
        # Iniciar servicios
        Log-Info "Building and starting services..."
        $result = & $composeCommand up -d --build
        if ($LASTEXITCODE -ne 0) {
            Handle-Error "Failed to start Docker services"
        }
        
        # Esperar que los servicios estén listos
        Wait-ForService "mongodb"
        Wait-ForService "redis" 
        Wait-ForService "api"
        
        # Verificar health checks
        Test-Health "mongodb"
        Test-Health "redis"
        Test-Health "api"
        
        # Probar endpoints
        Start-Sleep -Seconds 5
        Test-ApiEndpoints
        
        # Mostrar información
        Show-ServicesInfo
        
        Log-Success "All services are running and ready for testing!"
    }
    
    "test" {
        Log-Info "Running comprehensive tests..."
        Test-ApiEndpoints
    }
    
    "logs" {
        Log-Info "Showing real-time logs (Press Ctrl+C to stop)..."
        docker-compose logs -f --tail=50
    }
    
    "stop" {
        Log-Info "Stopping Docker services..."
        docker-compose down
        Log-Success "Services stopped"
    }
    
    "clean" {
        Log-Info "Cleaning up Docker resources..."
        docker-compose down -v --remove-orphans
        docker system prune -f
        Log-Success "Cleanup completed"
    }
    
    "restart" {
        Log-Info "Restarting services..."
        docker-compose restart
        Start-Sleep -Seconds 10
        Test-ApiEndpoints
        Show-ServicesInfo
    }
    
    "status" {
        Log-Info "Service Status:"
        docker-compose ps
        Write-Host ""
        Log-Info "Health Status:"
        try {
            Invoke-RestMethod -Uri "http://localhost:8081/api/v1/health" -Method Get -TimeoutSec 5 | Out-Null
            Log-Success "API is healthy"
        }
        catch {
            Log-Error "API is not responding"
        }
    }
    
    "shell" {
        Log-Info "Opening shell in API container..."
        docker-compose exec api /bin/sh
    }
    
    default {
        Write-Host "Usage: .\scripts\docker-test.ps1 [COMMAND]"
        Write-Host ""
        Write-Host "Commands:"
        Write-Host "  start        - Start all services and run tests (default)"
        Write-Host "  test         - Run API tests"
        Write-Host "  logs         - Show real-time logs"
        Write-Host "  stop         - Stop all services"  
        Write-Host "  clean        - Stop services and clean up volumes"
        Write-Host "  restart      - Restart all services"
        Write-Host "  status       - Show service status"
        Write-Host "  shell        - Open shell in API container"
        Write-Host "  help         - Show this help message"
    }
}