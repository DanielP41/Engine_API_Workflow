package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/internal/worker"
	"Engine_API_Workflow/pkg/database"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

func main() {
	// Cargar configuración
	cfg := config.Load()
	cfg.LogConfig()

	// Inicializar logger
	appLogger := logger.NewLogger(cfg.LogLevel, cfg.Environment)
	defer appLogger.Sync()

	appLogger.Info("Starting Engine API Workflow",
		zap.String("version", "2.0.0"),
		zap.String("environment", cfg.Environment))

	// Conectar a MongoDB
	appLogger.Info("Connecting to MongoDB...")
	mongoClient, err := database.NewMongoConnection(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		appLogger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := database.DisconnectMongoDB(mongoClient); err != nil {
			appLogger.Error("Error disconnecting from MongoDB", zap.Error(err))
		}
	}()
	appLogger.Info("Connected to MongoDB successfully")

	// Conectar a Redis
	appLogger.Info("Connecting to Redis...")
	redisClient, err := database.NewRedisConnection(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		appLogger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			appLogger.Error("Error disconnecting from Redis", zap.Error(err))
		}
	}()
	appLogger.Info("Connected to Redis successfully")

	// Inicializar repositorios
	repos := &repository.Repositories{
		User:     repository.NewUserRepository(mongoClient, cfg.MongoDatabase, appLogger),
		Workflow: repository.NewWorkflowRepository(mongoClient, cfg.MongoDatabase, appLogger),
		Log:      repository.NewLogRepository(mongoClient, cfg.MongoDatabase, appLogger),
		Queue:    repository.NewQueueRepository(redisClient, appLogger),
	}

	// Inicializar servicios JWT
	jwtConfig := cfg.GetJWTConfig()
	jwtService := jwt.NewJWTService(jwtConfig)

	var tokenBlacklist *jwt.TokenBlacklist
	if cfg.EnableTokenBlacklist {
		tokenBlacklist = jwt.NewTokenBlacklist(redisClient, appLogger)
	}

	// Inicializar servicios de negocio
	servicesContainer := &services.Services{
		Auth:      services.NewAuthService(repos.User, jwtService, appLogger),
		User:      services.NewUserService(repos.User, appLogger),
		Workflow:  services.NewWorkflowService(repos.Workflow, repos.Queue, appLogger),
		Log:       services.NewLogService(repos.Log, appLogger),
		Queue:     services.NewQueueService(repos.Queue, appLogger),
		Dashboard: services.NewDashboardService(repos.Workflow, repos.Log, repos.Queue, appLogger),
	}

	// Inicializar Worker Engine avanzado
	appLogger.Info("Initializing advanced worker engine...")
	workerConfig := worker.WorkerConfig{
		Workers:           getEnvAsInt("WORKER_POOL_MIN_SIZE", 3),
		MaxWorkers:        getEnvAsInt("WORKER_POOL_MAX_SIZE", 20),
		PollInterval:      getEnvAsDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		MaxRetries:        getEnvAsInt("DEFAULT_MAX_RETRIES", 3),
		RetryDelay:        getEnvAsDuration("DEFAULT_RETRY_DELAY", 30*time.Second),
		ProcessingTimeout: getEnvAsDuration("TASK_EXECUTION_TIMEOUT", 30*time.Minute),
	}

	workerEngine := worker.NewWorkerEngine(
		repos.Queue,
		repos.Workflow,
		repos.Log,
		repos.User,
		servicesContainer.Log,
		appLogger,
		workerConfig,
	)

	// Inicializar middlewares
	authMiddleware := middleware.NewAuthMiddlewareWithBlacklist(jwtService, tokenBlacklist, appLogger)

	// Inicializar validator
	validator := utils.NewValidator()

	// Inicializar handlers
	handlersContainer := &handlers.Handlers{
		Auth: handlers.NewAuthHandlerWithConfig(handlers.AuthHandlerConfig{
			UserRepo:       repos.User,
			AuthService:    servicesContainer.Auth,
			JWTService:     jwtService,
			TokenBlacklist: tokenBlacklist,
			Validator:      validator,
			Logger:         appLogger,
		}),
		User:      handlers.NewUserHandler(servicesContainer.User, appLogger),
		Workflow:  handlers.NewWorkflowHandler(servicesContainer.Workflow, appLogger),
		Log:       handlers.NewLogHandler(servicesContainer.Log, appLogger),
		Worker:    handlers.NewWorkerHandler(repos.Queue, workerEngine, appLogger),
		Dashboard: handlers.NewDashboardHandler(servicesContainer.Dashboard, appLogger),
	}

	// Inicializar Fiber con configuración avanzada
	app := fiber.New(fiber.Config{
		ServerHeader: "Engine-API-Workflow",
		AppName:      "Engine API Workflow v2.0.0",
		ErrorHandler: customErrorHandler(appLogger),
		ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  getEnvAsDuration("IDLE_TIMEOUT", 120*time.Second),
		BodyLimit:    getEnvAsBytes("API_MAX_REQUEST_SIZE", 10*1024*1024), // 10MB
	})

	// Middlewares globales
	app.Use(recover.New())

	if cfg.EnableCORS {
		app.Use(cors.New(cors.Config{
			AllowOrigins: strings.Join(cfg.GetCORSOrigins(), ","),
			AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
			AllowHeaders: "Content-Type,Authorization",
		}))
	}

	// Configurar rutas con todos los handlers
	routeConfig := &routes.RouteConfig{
		AuthHandler:      handlersContainer.Auth,
		WorkflowHandler:  handlersContainer.Workflow,
		TriggerHandler:   nil, // Mantener compatibilidad, puede ser nil
		DashboardHandler: handlersContainer.Dashboard,
		WorkerHandler:    handlersContainer.Worker,
		UserHandler:      handlersContainer.User,
		LogHandler:       handlersContainer.Log,
		JWTService:       jwtService,
		Logger:           appLogger,
	}

	// Validar configuración de rutas
	if err := routes.ValidateRouteConfig(routeConfig); err != nil {
		appLogger.Fatal("Invalid route configuration", zap.Error(err))
	}

	// Configurar todas las rutas (básicas + avanzadas + monitoreo + admin)
	routes.SetupAllRoutes(app, routeConfig)

	// Log de rutas configuradas
	routeInfo := routes.GetRouteInfo()
	workerRouteInfo := routes.GetWorkerRouteInfo()
	appLogger.Info("Routes configured successfully",
		zap.Any("basic_info", routeInfo),
		zap.Any("worker_routes", workerRouteInfo))

	// Verificar que el directorio web existe (opcional)
	if err := ensureWebDirectoryExists(); err != nil {
		appLogger.Warn("Web directory setup failed, dashboard UI may be limited", zap.Error(err))
	}

	// Contexto para shutdown graceful
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Iniciar Worker Engine avanzado
	appLogger.Info("Starting advanced worker engine...")
	if err := workerEngine.Start(ctx); err != nil {
		appLogger.Fatal("Failed to start worker engine", zap.Error(err))
	}

	// Canal para manejar señales del sistema
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Iniciar servicios en background
	go startBackgroundServices(servicesContainer.Dashboard, appLogger)

	// Iniciar el servidor en una goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server",
			zap.String("address", addr),
			zap.String("dashboard_url", fmt.Sprintf("http://localhost:%s", cfg.ServerPort)),
			zap.String("health_check", fmt.Sprintf("http://localhost:%s/api/v1/health", cfg.ServerPort)),
			zap.String("worker_stats", fmt.Sprintf("http://localhost:%s/api/v1/workers/advanced/stats", cfg.ServerPort)))

		if err := app.Listen(addr); err != nil {
			appLogger.Error("Server failed to start", zap.Error(err))
			sigChan <- syscall.SIGTERM
		}
	}()

	// Esperar señal de interrupción
	<-sigChan
	appLogger.Info("Shutting down server...")

	// Shutdown graceful del Worker Engine
	appLogger.Info("Stopping worker engine...")
	if err := workerEngine.Stop(); err != nil {
		appLogger.Error("Error stopping worker engine", zap.Error(err))
	}

	// Shutdown del servidor HTTP
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(),
		getEnvAsDuration("GRACEFUL_SHUTDOWN_TIMEOUT", 15*time.Second))
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		appLogger.Error("Server forced to shutdown", zap.Error(err))
	}

	appLogger.Info("Server exited gracefully")
}

// customErrorHandler maneja los errores de la aplicación
func customErrorHandler(appLogger *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Código de estado por defecto
		code := fiber.StatusInternalServerError

		// Verificar si es un error de Fiber
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		// Log del error
		appLogger.Error("HTTP Error",
			zap.Error(err),
			zap.String("path", c.Path()),
			zap.String("method", c.Method()),
			zap.Int("status", code),
			zap.String("ip", c.IP()))

		// Enviar respuesta de error
		return c.Status(code).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
			"path":    c.Path(),
			"status":  code,
			"version": "2.0",
		})
	}
}

// ensureWebDirectoryExists verifica y crea el directorio web si no existe
func ensureWebDirectoryExists() error {
	webDir := "./web"
	templatesDir := "./web/templates"
	staticDir := "./web/static"

	// Crear directorios si no existen
	dirs := []string{webDir, templatesDir, staticDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Crear archivo index básico si no existe
	indexPath := "./web/templates/dashboard.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		basicHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Engine API Workflow Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { background: rgba(255,255,255,0.95); padding: 30px; border-radius: 12px; margin-bottom: 20px; backdrop-filter: blur(10px); }
        .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: rgba(255,255,255,0.95); padding: 25px; border-radius: 12px; backdrop-filter: blur(10px); }
        h1 { color: #333; margin: 0 0 10px 0; font-size: 2.5em; }
        .status { color: #28a745; font-weight: bold; }
        .api-links { display: flex; flex-wrap: wrap; gap: 12px; margin: 20px 0; }
        .api-links a { background: linear-gradient(45deg, #007bff, #0056b3); color: white; padding: 12px 20px; text-decoration: none; border-radius: 8px; transition: transform 0.2s; }
        .api-links a:hover { transform: translateY(-2px); box-shadow: 0 4px 12px rgba(0,123,255,0.3); }
        .version { background: #f8f9fa; padding: 8px 16px; border-radius: 20px; display: inline-block; font-size: 0.9em; margin-left: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Engine API Workflow <span class="version">v2.0</span></h1>
            <p class="status">Sistema Operacional - Workers Avanzados Activos</p>
        </div>
        
        <div class="cards">
            <div class="card">
                <h3>🚀 API Endpoints Básicos</h3>
                <div class="api-links">
                    <a href="/api/v1/health">Health Check</a>
                    <a href="/api/v1/dashboard/overview">Dashboard</a>
                    <a href="/api/v1/workflows">Workflows</a>
                </div>
            </div>
            
            <div class="card">
                <h3>⚙️ Sistema de Workers Avanzado</h3>
                <div class="api-links">
                    <a href="/api/v1/workers/advanced/stats">Stats Avanzadas</a>
                    <a href="/api/v1/workers/health/detailed">Health Detallado</a>
                    <a href="/api/v1/workers/pool/stats">Pool de Workers</a>
                    <a href="/api/v1/workers/metrics/detailed">Métricas</a>
                </div>
            </div>
            
            <div class="card">
                <h3>📊 Monitoreo</h3>
                <div class="api-links">
                    <a href="/monitoring/health">Health Simple</a>
                    <a href="/monitoring/status">Status Sistema</a>
                    <a href="/api/v1/workers/retries/stats">Reintentos</a>
                </div>
            </div>
        </div>
    </div>
</body>
</html>`
		if err := os.WriteFile(indexPath, []byte(basicHTML), 0644); err != nil {
			return fmt.Errorf("failed to create basic HTML file: %w", err)
		}
	}

	return nil
}

// startBackgroundServices inicia servicios en background
func startBackgroundServices(dashboardService services.DashboardService, appLogger *zap.Logger) {
	appLogger.Info("Starting background services")

	ticker := time.NewTicker(getEnvAsDuration("DASHBOARD_REFRESH_INTERVAL", 30*time.Second))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Refrescar datos del dashboard en background
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := dashboardService.RefreshDashboardData(ctx); err != nil {
				appLogger.Error("Failed to refresh dashboard data", zap.Error(err))
			}
			cancel()
		}
	}
}

// Helper functions para configuración
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvAsBytes(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Parse formato como "10MB", "1GB", etc.
		multiplier := 1
		cleanValue := strings.ToUpper(value)

		if strings.HasSuffix(cleanValue, "KB") {
			multiplier = 1024
			cleanValue = strings.TrimSuffix(cleanValue, "KB")
		} else if strings.HasSuffix(cleanValue, "MB") {
			multiplier = 1024 * 1024
			cleanValue = strings.TrimSuffix(cleanValue, "MB")
		} else if strings.HasSuffix(cleanValue, "GB") {
			multiplier = 1024 * 1024 * 1024
			cleanValue = strings.TrimSuffix(cleanValue, "GB")
		}

		if intValue, err := strconv.Atoi(cleanValue); err == nil {
			return intValue * multiplier
		}
	}
	return defaultValue
}
