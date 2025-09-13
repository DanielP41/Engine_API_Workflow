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
	"github.com/gofiber/fiber/v2/middleware/limiter"
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
	appLogger := logger.New(cfg.LogLevel, cfg.Environment)
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
		// CORREGIDO: Comentar handlers que no existen
		// User:      handlers.NewUserHandler(servicesContainer.User, appLogger),
		Workflow: handlers.NewWorkflowHandler(servicesContainer.Workflow, appLogger),
		// Log:       handlers.NewLogHandler(servicesContainer.Log, appLogger),
		Worker:    handlers.NewWorkerHandler(repos.Queue, workerEngine, appLogger),
		Dashboard: handlers.NewDashboardHandler(servicesContainer.Dashboard, appLogger),
	}

	// Inicializar Fiber con configuración básica
	app := fiber.New(fiber.Config{
		ServerHeader: "Engine-API-Workflow",
		AppName:      "Engine API Workflow v2.0.0",
		ErrorHandler: customErrorHandler(appLogger),
		ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		BodyLimit:    getEnvAsBytes("API_MAX_REQUEST_SIZE", 10*1024*1024), // 10MB
	})

	// Middlewares globales
	app.Use(recover.New())

	// CORS básico
	if cfg.EnableCORS {
		corsConfig := cfg.GetCORSConfig()
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(corsConfig.AllowedOrigins, ","),
			AllowMethods:     strings.Join(corsConfig.AllowedMethods, ","),
			AllowHeaders:     strings.Join(corsConfig.AllowedHeaders, ","),
			AllowCredentials: corsConfig.AllowCredentials,
		}))
		appLogger.Info("CORS enabled")
	}

	// Rate limiting básico
	if cfg.EnableRateLimit {
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.RateLimitRequests,
			Expiration: cfg.RateLimitWindow,
		}))
		appLogger.Info("Rate limiting enabled")
	}

	// Archivos estáticos
	if cfg.EnableWebInterface {
		app.Static("/static", "./web/static")
		app.Static("/", "./web")
		appLogger.Info("Web interface enabled")
	}

	// Request logging middleware
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		appLogger.Info("HTTP Request",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", duration))

		return err
	})

	// Configurar rutas
	routeConfig := &routes.RouteConfig{
		AuthHandler:      handlersContainer.Auth,
		WorkflowHandler:  handlersContainer.Workflow,
		TriggerHandler:   nil,
		DashboardHandler: handlersContainer.Dashboard,
		WorkerHandler:    handlersContainer.Worker,
		JWTService:       jwtService,
		Logger:           appLogger,
	}

	// Validar configuración de rutas
	if err := routes.ValidateRouteConfig(routeConfig); err != nil {
		appLogger.Fatal("Invalid route configuration", zap.Error(err))
	}

	// Configurar rutas web si está habilitado
	if cfg.EnableWebInterface {
		setupWebRoutes(app, handlersContainer, authMiddleware, appLogger)
	}

	// Configurar todas las rutas
	routes.SetupAllRoutes(app, routeConfig)

	// Log de rutas configuradas
	routeInfo := routes.GetRouteInfo()
	workerRouteInfo := routes.GetWorkerRouteInfo()
	appLogger.Info("Routes configured successfully",
		zap.Any("basic_info", routeInfo),
		zap.Any("worker_routes", workerRouteInfo))

	// Verificar directorio web
	if cfg.EnableWebInterface {
		if err := ensureWebDirectoryExists(); err != nil {
			appLogger.Warn("Web directory setup failed", zap.Error(err))
		}
	}

	// Contexto para shutdown graceful
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Iniciar Worker Engine
	appLogger.Info("Starting advanced worker engine...")
	if err := workerEngine.Start(ctx); err != nil {
		appLogger.Fatal("Failed to start worker engine", zap.Error(err))
	}

	// Canal para señales del sistema
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Iniciar servicios en background
	go startBackgroundServices(servicesContainer.Dashboard, appLogger)

	// Iniciar servidor
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server",
			zap.String("address", addr),
			zap.String("health_check", fmt.Sprintf("http://localhost:%s/api/v1/health", cfg.ServerPort)))

		if err := app.Listen(addr); err != nil {
			appLogger.Error("Server failed to start", zap.Error(err))
			sigChan <- syscall.SIGTERM
		}
	}()

	// Esperar señal de interrupción
	<-sigChan
	appLogger.Info("Shutting down server...")

	// Shutdown del Worker Engine
	if err := workerEngine.Stop(); err != nil {
		appLogger.Error("Error stopping worker engine", zap.Error(err))
	}

	// Shutdown del servidor HTTP
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		appLogger.Error("Server forced to shutdown", zap.Error(err))
	}

	appLogger.Info("Server exited gracefully")
}

// setupWebRoutes configura rutas web básicas
func setupWebRoutes(app *fiber.App, handlers *handlers.Handlers, authMiddleware middleware.AuthMiddleware, logger *zap.Logger) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/dashboard")
	})

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/dashboard.html")
	})

	app.Get("/api/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "online",
			"version":   "2.0.0",
			"timestamp": time.Now(),
		})
	})

	logger.Info("Web routes configured")
}

// customErrorHandler maneja errores de la aplicación
func customErrorHandler(appLogger *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError

		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		appLogger.Error("HTTP Error",
			zap.Error(err),
			zap.String("path", c.Path()),
			zap.Int("status", code))

		return c.Status(code).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
			"status":  code,
		})
	}
}

// ensureWebDirectoryExists crea directorios web básicos
func ensureWebDirectoryExists() error {
	dirs := []string{"./web", "./web/templates", "./web/static"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Crear HTML básico
	indexPath := "./web/templates/dashboard.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		basicHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Engine API Workflow Dashboard</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: white; padding: 30px; border-radius: 8px; margin-bottom: 20px; }
        h1 { color: #333; margin: 0; }
        .status { color: #28a745; font-weight: bold; }
        .links { display: flex; gap: 10px; margin: 20px 0; }
        .links a { background: #007bff; color: white; padding: 10px 15px; text-decoration: none; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Engine API Workflow v2.0</h1>
            <p class="status">Sistema Operacional</p>
            <div class="links">
                <a href="/api/v1/health">Health Check</a>
                <a href="/api/v1/workers/stats">Workers</a>
                <a href="/api/status">API Status</a>
            </div>
        </div>
    </div>
</body>
</html>`
		if err := os.WriteFile(indexPath, []byte(basicHTML), 0644); err != nil {
			return err
		}
	}
	return nil
}

// startBackgroundServices inicia servicios en background
func startBackgroundServices(dashboardService services.DashboardService, appLogger *zap.Logger) {
	appLogger.Info("Starting background services")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := dashboardService.RefreshDashboardData(ctx); err != nil {
				appLogger.Debug("Failed to refresh dashboard data", zap.Error(err))
			}
			cancel()
		}
	}
}

// Helper functions
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
