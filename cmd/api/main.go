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
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
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

	// Inicializar servicios JWT
	jwtService := jwt.NewService(cfg.JWTSecret, "engine-api-workflow")

	// Inicializar repositorios
	db := mongoClient.Database(cfg.MongoDatabase)
	userRepo := mongodb.NewUserRepository(db)
	workflowRepo := mongodb.NewWorkflowRepository(db)
	logRepo := mongodb.NewLogRepository(db)

	// Inicializar servicios de aplicación
	authService := services.NewAuthService(jwtService)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)
	queueService := services.NewQueueService(redisClient, appLogger.(*logger.Logger))

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(userRepo, authService)
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService)
	triggerHandler := handlers.NewTriggerHandler(workflowRepo, logRepo, queueService, appLogger.(*logger.Logger))

	// Configurar Fiber
	app := fiber.New(fiber.Config{
		ServerHeader: "Engine-API-Workflow",
		AppName:      "Engine API Workflow v1.0.0",
		ErrorHandler: customErrorHandler(appLogger),
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 30,
	})

	// Configurar middlewares
	corsConfig := middleware.DefaultCORSConfig()
	if cfg.Environment == "production" {
		corsConfig = middleware.ProductionCORSConfig([]string{})
	}

	app.Use(middleware.CORSMiddleware(corsConfig))
	app.Use(middleware.SecurityHeadersMiddleware())
	app.Use(middleware.APIResponseHeadersMiddleware())
	middleware.SetupMiddleware(app, appLogger)

	// Configurar rutas
	routeConfig := &routes.RouteConfig{
		AuthHandler:     authHandler,
		WorkflowHandler: workflowHandler,
		TriggerHandler:  triggerHandler,
		JWTService:      jwtService,
		Logger:          appLogger,
	}
	routes.SetupRoutes(app, routeConfig)

	// Canal para manejar señales del sistema
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Iniciar el servidor en una goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server", "address", addr)

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
		)

		// Enviar respuesta de error personalizada
		return c.Status(code).JSON(fiber.Map{
			"status":    "error",
			"message":   getErrorMessage(code),
			"error":     err.Error(),
			"timestamp": time.Now().UTC(),
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
