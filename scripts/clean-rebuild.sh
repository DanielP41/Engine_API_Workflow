# clean-rebuild.ps1 - Script para limpiar y reconstruir todo

function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    $colorMap = @{ "Red" = "Red"; "Green" = "Green"; "Yellow" = "Yellow"; "Blue" = "Blue"; "Purple" = "Magenta"; "Cyan" = "Cyan" }
    Write-Host $Message -ForegroundColor $colorMap[$Color]
}

Write-ColorOutput "🧹 Engine API Workflow - Clean & Rebuild" "Purple"
Write-ColorOutput "=======================================" "Purple"

# 1. Parar y limpiar contenedores existentes
Write-ColorOutput "🛑 Stopping and cleaning existing containers..." "Yellow"
docker-compose down -v --remove-orphans 2>$null
docker system prune -f 2>$null

# 2. Limpiar módulos Go
Write-ColorOutput "📦 Cleaning Go modules..." "Blue"
if (Test-Path "go.sum") { Remove-Item "go.sum" -Force }
go clean -modcache 2>$null
go mod tidy

# 3. Verificar archivos corregidos
Write-ColorOutput "🔍 Verifying corrected files..." "Blue"
$filesToCheck = @(
    "internal/repository/interfaces.go",
    "internal/utils/response.go", 
    "internal/utils/errors.go",
    "internal/utils/validation.go",
    "Dockerfile"
)

foreach ($file in $filesToCheck) {
    if (Test-Path $file) {
        Write-ColorOutput "✅ $file exists" "Green"
    } else {
        Write-ColorOutput "❌ $file missing - please create it first" "Red"
        exit 1
    }
}

# 4. Crear directorios necesarios
Write-ColorOutput "📁 Creating necessary directories..." "Blue"
@("config", "scripts", "tmp") | ForEach-Object {
    if (-not (Test-Path $_)) {
        New-Item -ItemType Directory -Path $_ -Force
        Write-ColorOutput "✅ Created directory: $_" "Green"
    }
}

# 5. Crear .env si no existe
if (-not (Test-Path ".env")) {
    if (Test-Path ".env.example") {
        Copy-Item ".env.example" ".env"
        Write-ColorOutput "✅ Created .env from template" "Green"
    } else {
        Write-ColorOutput "⚠️  .env.example not found, creating basic .env" "Yellow"
        @"
# Basic configuration
PORT=8081
ENV=development
MONGODB_URI=mongodb://admin:password123@localhost:27017/engine_workflow?authSource=admin
MONGODB_DATABASE=engine_workflow
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production
LOG_LEVEL=debug
"@ | Out-File -FilePath ".env" -Encoding UTF8
    }
}

# 6. Verificar sintaxis Go
Write-ColorOutput "🔍 Checking Go syntax..." "Blue"
$goCheck = go build -o tmp/syntax-check cmd/api/main.go 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-ColorOutput "✅ Go syntax is valid" "Green"
    Remove-Item "tmp/syntax-check" -ErrorAction SilentlyContinue
} else {
    Write-ColorOutput "❌ Go syntax errors found:" "Red"
    Write-Host $goCheck
    exit 1
}

# 7. Construir imagen Docker
Write-ColorOutput "🐳 Building Docker image..." "Blue"
$buildOutput = docker build -t engine-api-workflow . 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-ColorOutput "✅ Docker image built successfully" "Green"
} else {
    Write-ColorOutput "❌ Docker build failed:" "Red"
    Write-Host $buildOutput
    exit 1
}

# 8. Iniciar servicios
Write-ColorOutput "🚀 Starting services..." "Blue"
docker-compose up -d

# 9. Esperar que los servicios estén listos
Write-ColorOutput "⏳ Waiting for services to start..." "Yellow"
Start-Sleep -Seconds 20

# 10. Verificar estado
Write-ColorOutput "📊 Checking service status..." "Blue"
docker-compose ps

# 11. Probar health endpoint
Write-ColorOutput "🔍 Testing health endpoint..." "Blue"
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/health" -Method Get -TimeoutSec 10
    Write-ColorOutput "✅ API is responding!" "Green"
    $response | ConvertTo-Json
} catch {
    Write-ColorOutput "⚠️  API not ready yet, checking logs..." "Yellow"
    docker-compose logs --tail=20 api
}

Write-ColorOutput "" ""
Write-ColorOutput "🎉 Clean & Rebuild completed!" "Green"
Write-ColorOutput "🌐 Services available at:" "Blue"
Write-ColorOutput "   - API:            http://localhost:8081" "White"
Write-ColorOutput "   - Health:         http://localhost:8081/api/v1/health" "White"  
Write-ColorOutput "   - Mongo Express:  http://localhost:8082" "White"
Write-ColorOutput "   - Redis Commander: http://localhost:8083" "White"

Write-ColorOutput "" ""
Write-ColorOutput "📋 Next steps:" "Purple"
Write-ColorOutput "   - Test endpoints: .\scripts\docker-test.ps1 test" "White"
Write-ColorOutput "   - View logs:      docker-compose logs -f api" "White"
Write-ColorOutput "   - Stop services:  docker-compose down" "White"