// ACTUALIZADo CON DASHBOARD
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/repository/redis"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/pkg/database"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

func main() {
	// Cargar configuración
	cfg := config.Load()

	// Inicializar logger
	appLogger := logger.New(cfg.LogLevel, cfg.Environment)
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
	queueRepo := redis.NewQueueRepository(redisClient) // 🆕 Repository de cola

	// Inicializar servicios
	authService := services.NewAuthService(jwtService)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)
	queueService := services.NewQueueService(redisClient, appLogger.Logger) // 🆕 Servicio de cola

	// 🆕 Inicializar servicios del dashboard
	metricsService := services.NewMetricsService(userRepo, workflowRepo, logRepo, queueRepo)
	dashboardService := services.NewDashboardService(metricsService, workflowRepo, logRepo, userRepo, queueRepo)

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(userRepo, authService)
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService, appLogger)
	logHandler := handlers.NewLogHandler(logRepo, appLogger.Logger)
	triggerHandler := handlers.NewTriggerHandler(workflowRepo, logRepo, queueService, appLogger.Logger)

	// 🆕 Inicializar handler del dashboard
	dashboardHandler := handlers.NewDashboardHandler(dashboardService, appLogger.Logger)

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

	// Configurar rutas con todos los handlers
	routeConfig := &routes.RouteConfig{
		AuthHandler:      authHandler,
		WorkflowHandler:  workflowHandler,
		LogHandler:       logHandler,
		TriggerHandler:   triggerHandler,
		DashboardHandler: dashboardHandler, // 🆕 Handler del dashboard
		JWTService:       jwtService,
		Logger:           appLogger.Logger,
	}

	// Validar configuración de rutas
	if err := routes.ValidateRouteConfig(routeConfig); err != nil {
		appLogger.Fatal("Invalid route configuration", "error", err)
	}

	// Configurar middlewares adicionales
	routes.SetupMiddlewares(app, routeConfig)

	// Configurar health checks
	routes.SetupHealthChecks(app, routeConfig)

	// Configurar todas las rutas
	routes.SetupRoutes(app, routeConfig)

	// Log de rutas configuradas
	routeInfo := routes.GetRouteInfo()
	appLogger.Info("Routes configured", "info", routeInfo)

	// 🆕 Inicializar servicios en background (opcional)
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
			"status", code,
			"method", c.Method(),
			"path", c.Path(),
			"error", err.Error(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		)

		// Enviar respuesta de error personalizada
		return c.Status(code).JSON(fiber.Map{
			"status":    "error",
			"message":   getErrorMessage(code),
			"error":     err.Error(),
			"timestamp": time.Now().UTC(),
			"path":      c.Path(),
		})
	}
}

// getErrorMessage devuelve un mensaje apropiado según el código de estado
func getErrorMessage(code int) string {
	switch code {
	case fiber.StatusBadRequest:
		return "Bad Request"
	case fiber.StatusUnauthorized:
		return "Unauthorized"
	case fiber.StatusForbidden:
		return "Forbidden"
	case fiber.StatusNotFound:
		return "Not Found"
	case fiber.StatusMethodNotAllowed:
		return "Method Not Allowed"
	case fiber.StatusTooManyRequests:
		return "Too Many Requests"
	case fiber.StatusInternalServerError:
		return "Internal Server Error"
	case fiber.StatusServiceUnavailable:
		return "Service Unavailable"
	default:
		return "Unknown Error"
	}
}

// ensureWebDirectoryExists verifica que el directorio web exista
func ensureWebDirectoryExists() error {
	webDirs := []string{
		"./web",
		"./web/static",
		"./web/templates",
		"./web/static/css",
		"./web/static/js",
		"./web/static/images",
	}

	for _, dir := range webDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	// Crear archivo index básico si no existe
	indexPath := "./web/templates/dashboard.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		basicHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Engine API Workflow - Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; text-align: center; }
        .status { background: #e8f5e8; padding: 20px; border-radius: 4px; margin: 20px 0; }
        .api-links { margin: 20px 0; }
        .api-links a { display: block; margin: 5px 0; color: #007bff; text-decoration: none; }
        .api-links a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Engine API Workflow</h1>
        <div class="status">
            <h3>✅ Sistema Activo</h3>
            <p>El dashboard del sistema está funcionando correctamente.</p>
            <p><strong>Versión:</strong> 1.0.0</p>
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
