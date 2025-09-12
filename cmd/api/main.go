package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/repository/redis"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/database"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

func main() {
	// Cargar configuración
	cfg := config.Load()

	// Inicializar logger
	appLogger := logger.New(cfg.LogLevel, cfg.Environment)

	// Crear un zap logger directo para servicios que lo requieren
	var zapLogger *zap.Logger
	if cfg.Environment == "production" {
		zapLogger, _ = zap.NewProduction()
	} else {
		zapLogger, _ = zap.NewDevelopment()
	}

	appLogger.Info("Starting Engine API Workflow", "version", "1.0.0", "environment", cfg.Environment)

	// Conectar a MongoDB
	appLogger.Info("Connecting to MongoDB...")
	mongoClient, err := database.NewMongoConnection(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		appLogger.Fatal("Failed to connect to MongoDB", "error", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			appLogger.Error("Error disconnecting from MongoDB", "error", err)
		}
	}()
	appLogger.Info("Connected to MongoDB successfully")

	// Conectar a Redis
	appLogger.Info("Connecting to Redis...")
	redisClient, err := database.NewRedisConnection(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		appLogger.Fatal("Failed to connect to Redis", "error", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			appLogger.Error("Error disconnecting from Redis", "error", err)
		}
	}()
	appLogger.Info("Connected to Redis successfully")

	// Inicializar base de datos
	db := mongoClient.Database(cfg.MongoDatabase)

	// Inicializar servicios JWT
	jwtService := jwt.NewService(cfg.JWTSecret, "engine-api-workflow")

	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(db)
	workflowRepo := mongodb.NewWorkflowRepository(db)
	logRepo := mongodb.NewLogRepository(db)
	queueRepo := redis.NewQueueRepository(redisClient)

	// Inicializar servicios - CORREGIDO: usar zapLogger donde se requiere *zap.Logger
	authService := services.NewAuthService(userRepo)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)
	queueService := services.NewQueueService(redisClient, zapLogger) // usar zapLogger

	// Inicializar servicios del dashboard
	metricsService := services.NewMetricsService(userRepo, workflowRepo, logRepo, queueRepo)
	dashboardService := services.NewDashboardService(metricsService, workflowRepo, logRepo, userRepo, queueRepo)

	// Inicializar validator
	validator := &utils.Validator{}

	// Inicializar handlers - CORREGIDO: usar zapLogger donde se requiere *zap.Logger
	authHandler := handlers.NewAuthHandler(authService, jwtService, validator)
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService, *validator)
	triggerHandler := handlers.NewTriggerHandler(workflowRepo, logRepo, queueService, zapLogger) // usar zapLogger
	dashboardHandler := handlers.NewDashboardHandler(dashboardService, zapLogger)                // usar zapLogger

	// Verificar que el directorio web existe
	if err := ensureWebDirectoryExists(); err != nil {
		appLogger.Warn("Web directory not found, dashboard UI will not be available", "error", err)
	}

	// Configurar Fiber
	app := fiber.New(fiber.Config{
		ServerHeader: "Engine-API-Workflow",
		AppName:      "Engine API Workflow v1.0.0",
		ErrorHandler: customErrorHandler(appLogger),
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 30,
	})

	// Configurar rutas con todos los handlers - CORREGIDO: usar zapLogger en RouteConfig
	routeConfig := &routes.RouteConfig{
		AuthHandler:      authHandler,
		WorkflowHandler:  workflowHandler,
		TriggerHandler:   triggerHandler,
		DashboardHandler: dashboardHandler,
		JWTService:       jwtService,
		Logger:           zapLogger, // usar zapLogger en lugar de appLogger
	}

	// Validar configuración de rutas
	if err := routes.ValidateRouteConfig(routeConfig); err != nil {
		appLogger.Fatal("Invalid route configuration", "error", err)
	}

	// Configurar todas las rutas
	routes.SetupRoutes(app, routeConfig)

	// Log de rutas configuradas
	routeInfo := routes.GetRouteInfo()
	appLogger.Info("Routes configured", "info", routeInfo)

	// Inicializar servicios en background (opcional)
	go startBackgroundServices(dashboardService, appLogger)

	// Canal para manejar señales del sistema
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Iniciar el servidor en una goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server", "address", addr)
		appLogger.Info("Dashboard available at", "url", fmt.Sprintf("http://localhost:%s", cfg.ServerPort))
		appLogger.Info("API documentation at", "url", fmt.Sprintf("http://localhost:%s/api/v1/health", cfg.ServerPort))

		if err := app.Listen(addr); err != nil {
			appLogger.Fatal("Server failed to start", "error", err)
		}
	}()

	// Esperar señal de interrupción
	<-c
	appLogger.Info("Shutting down server...")

	// Apagar el servidor gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		appLogger.Error("Server forced to shutdown", "error", err)
	}

	appLogger.Info("Server exited")
}

// customErrorHandler maneja los errores de la aplicación
func customErrorHandler(log *logger.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Código de estado por defecto
		code := fiber.StatusInternalServerError

		// Verificar si es un error de Fiber
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		// Log del error
		log.Error("HTTP Error",
			"error", err.Error(),
			"path", c.Path(),
			"method", c.Method(),
			"status", code,
		)

		// Enviar respuesta de error
		return c.Status(code).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
			"path":    c.Path(),
			"status":  code,
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
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; margin-bottom: 20px; }
        .status { background: #e8f5e8; padding: 15px; border-radius: 4px; margin: 20px 0; }
        .api-links { display: flex; flex-wrap: wrap; gap: 10px; margin: 20px 0; }
        .api-links a { background: #007bff; color: white; padding: 10px 15px; text-decoration: none; border-radius: 4px; }
        .api-links a:hover { background: #0056b3; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Engine API Workflow Dashboard</h1>
        
        <div class="status">
            <h3>✅ Sistema Operacional</h3>
            <p><strong>Estado:</strong> Operacional</p>
        </div>
        
        <h3>📊 API Endpoints Disponibles:</h3>
        <div class="api-links">
            <a href="/api/v1/health">🔍 Health Check</a>
            <a href="/api/v1/dashboard/summary">📈 Dashboard Summary</a>
            <a href="/api/v1/dashboard/health">💚 System Health</a>
            <a href="/api/v1/dashboard/stats">📊 Quick Stats</a>
        </div>
        
        <p><em>Dashboard UI completo próximamente...</em></p>
    </div>
</body>
</html>`
		if err := os.WriteFile(indexPath, []byte(basicHTML), 0644); err != nil {
			return fmt.Errorf("failed to create basic HTML file: %w", err)
		}
	}

	return nil
}

// startBackgroundServices inicia servicios en background para el dashboard
func startBackgroundServices(dashboardService services.DashboardService, appLogger *logger.Logger) {
	appLogger.Info("Starting background services for dashboard")

	// Ticker para refrescar datos del dashboard cada 30 segundos
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Refrescar datos del dashboard en background
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := dashboardService.RefreshDashboardData(ctx); err != nil {
				appLogger.Error("Failed to refresh dashboard data in background", "error", err)
			}
			cancel()
		}
	}
}
