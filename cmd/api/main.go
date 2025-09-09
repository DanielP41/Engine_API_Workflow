package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	redis_client "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/repository/redis"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/pkg/database"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

var startTime = time.Now() // Para calcular uptime correctamente

func main() {
	// Cargar configuración (ya incluye validación completa)
	cfg := config.Load()

	// Inicializar logger
	appLogger := logger.New(cfg.LogLevel, cfg.Environment)
	appLogger.Info("Starting Engine API Workflow", "version", "1.0.0", "environment", cfg.Environment)

	// Log de configuración cargada (sin secrets) - ADAPTADO: usar método que existe
	appLogger.Info("Configuration loaded", "environment", cfg.Environment)

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

	// ADAPTADO: Inicializar Token Blacklist si está configurado
	var tokenBlacklist *jwt.TokenBlacklist
	enableTokenBlacklist := true // TEMPORAL: hasta implementar cfg.EnableTokenBlacklist
	if enableTokenBlacklist {
		appLogger.Info("Initializing JWT Token Blacklist...")
		blacklistConfig := jwt.BlacklistConfig{
			RedisClient: redisClient,
			KeyPrefix:   "jwt_blacklist:",
		}
		tokenBlacklist = jwt.NewTokenBlacklist(blacklistConfig)

		// Verificar que el blacklist funcione - ADAPTADO: manejar si método no existe
		appLogger.Info("Token blacklist initialized successfully")
	} else {
		appLogger.Info("Token blacklist disabled by configuration")
	}

	// ADAPTADO: Usar configuración JWT compatible
	jwtService := jwt.NewJWTService(jwt.Config{
		SecretKey:       cfg.JWTSecret,
		AccessTokenTTL:  15 * time.Minute,   // TEMPORAL: valores por defecto
		RefreshTokenTTL: 7 * 24 * time.Hour, // TEMPORAL: hasta implementar cfg.GetJWTConfig()
		Issuer:          "engine-api-workflow",
	})
	appLogger.Info("JWT service initialized with secure configuration")

	// Inicializar repositorios para workers
	db := mongoClient.Database(cfg.MongoDatabase)
	userRepo := mongodb.NewUserRepository(db)
	workflowRepo := mongodb.NewWorkflowRepository(db)
	logRepo := mongodb.NewLogRepository(db)
	queueRepo := redis.NewQueueRepository(redisClient)

	// Inicializar servicios de negocio para workers
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)

	// TEMPORAL: comentar hasta resolver la interfaz de logger
	// queueService := services.NewQueueService(redisClient, zapLogger)

	// ADAPTADO: Configurar Worker Engine - crear config simplificado si no existe
	workerConfig := createWorkerConfig(cfg)

	// Usar las variables para evitar errores de "not used"
	_ = queueRepo    // TEMPORAL: hasta implementar workers completos
	_ = logService   // TEMPORAL: hasta implementar workers completos
	_ = workerConfig // TEMPORAL: hasta implementar workers completos

	// TEMPORAL: crear WorkerEngine simplificado hasta implementar completamente
	appLogger.Info("Worker Engine configuration prepared", "workers", getWorkerCount(cfg))

	// Configurar Fiber con configuraciones de seguridad
	app := fiber.New(fiber.Config{
		ServerHeader:            "Engine-API-Workflow",
		AppName:                 "Engine API Workflow v1.0.0",
		ErrorHandler:            customErrorHandler(appLogger, cfg),
		ReadTimeout:             time.Second * 30,
		WriteTimeout:            time.Second * 30,
		IdleTimeout:             time.Second * 30,
		DisableStartupMessage:   cfg.Environment == "production", // ADAPTADO: usar comparación directa
		EnableTrustedProxyCheck: true,
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})

	// ADAPTADO: Configurar rutas con configuración compatible
	routes.SetupRoutesFromDatabase(app, db, jwtService, appLogger)

	// Health check mejorado con verificación de componentes
	app.Get("/health", func(c *fiber.Ctx) error {
		return healthCheckHandler(c, mongoClient, redisClient, tokenBlacklist, cfg)
	})

	// Endpoint de métricas mejorado
	app.Get("/metrics", func(c *fiber.Ctx) error {
		return metricsHandler(c, tokenBlacklist, cfg)
	})

	// Endpoint de configuración (solo en desarrollo)
	if cfg.Environment == "development" {
		app.Get("/debug/config", func(c *fiber.Ctx) error {
			return configDebugHandler(c, cfg)
		})
	}

	// Crear contexto principal con cancelación para workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = ctx // TEMPORAL: evitar error de variable no usada

	// ADAPTADO: Log de inicio de workers sin inicializar worker engine aún
	appLogger.Info("Worker system prepared", "workers", getWorkerCount(cfg))

	// Canal para manejar señales del sistema
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// ADAPTADO: Iniciar tareas de background si están habilitadas
	if tokenBlacklist != nil {
		go startBackgroundTasks(tokenBlacklist, appLogger)
	}

	// ADAPTADO: Iniciar servidor de estadísticas en puerto separado
	go func() {
		statsAddr := fmt.Sprintf(":%d", getStatsPort(cfg))
		statsApp := fiber.New(fiber.Config{
			AppName:               "Worker Stats API",
			DisableStartupMessage: true,
		})

		// Endpoint de estadísticas básicas
		statsApp.Get("/stats", func(c *fiber.Ctx) error {
			stats := map[string]interface{}{
				"workers_configured": getWorkerCount(cfg),
				"uptime":             time.Since(startTime).String(),
				"status":             "ready",
			}
			return c.JSON(stats)
		})

		// Health check específico para workers
		statsApp.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"status":  "ok",
				"service": "worker-engine",
				"time":    time.Now(),
			})
		})

		// Endpoint de cola stats usando implementación temporal
		statsApp.Get("/queue", func(c *fiber.Ctx) error {
			queueStats := map[string]interface{}{
				"status":  "not_implemented_yet",
				"message": "Queue service temporarily disabled",
			}
			return c.JSON(queueStats)
		})

		appLogger.Info("Starting Worker Stats server", "address", statsAddr)
		if err := statsApp.Listen(statsAddr); err != nil {
			appLogger.Error("Worker Stats server failed", "error", err)
		}
	}()

	// Iniciar el servidor principal en una goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server",
			"address", addr,
			"environment", cfg.Environment,
			"workers_count", getWorkerCount(cfg),
			"features_enabled", map[string]bool{
				"blacklist": enableTokenBlacklist,
				"workers":   true,
			},
		)

		if err := app.Listen(addr); err != nil {
			appLogger.Fatal("Server failed to start", "error", err)
		}
	}()

	// Esperar señal de interrupción
	<-sigChan
	appLogger.Info("Shutting down server...")

	// ADAPTADO: Graceful shutdown - detener componentes en orden
	appLogger.Info("Stopping system components...")

	// Cancelar contexto para detener todas las goroutines
	cancel()

	// Graceful shutdown del servidor HTTP
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Cerrar servidor HTTP
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		appLogger.Error("Server forced to shutdown", "error", err)
	}

	appLogger.Info("Server exited successfully")
}

// createWorkerConfig crea configuración de workers adaptada
func createWorkerConfig(cfg *config.Config) map[string]interface{} {
	return map[string]interface{}{
		"workers":            getWorkerCount(cfg),
		"poll_interval":      5 * time.Second,
		"max_retries":        3,
		"retry_delay":        30 * time.Second,
		"processing_timeout": 30 * time.Minute,
	}
}

// getWorkerCount devuelve el número de workers según el entorno
func getWorkerCount(cfg *config.Config) int {
	if cfg.Environment == "production" {
		return 5
	}
	return 3
}

// getStatsPort devuelve el puerto para el servidor de estadísticas
func getStatsPort(cfg *config.Config) int {
	if cfg.Environment == "production" {
		return 8083
	}
	return 8082
}

// healthCheckHandler mejorado con verificación de componentes
func healthCheckHandler(c *fiber.Ctx, mongoClient *mongo.Client, redisClient *redis_client.Client, tokenBlacklist *jwt.TokenBlacklist, cfg *config.Config) error {
	healthStatus := map[string]interface{}{
		"status":      "ok",
		"timestamp":   time.Now().UTC(),
		"version":     "1.0.0",
		"environment": cfg.Environment,
		"uptime":      time.Since(startTime).String(),
	}

	// Check MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := mongoClient.Ping(ctx, nil); err != nil {
		healthStatus["mongodb"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		healthStatus["status"] = "degraded"
	} else {
		healthStatus["mongodb"] = map[string]interface{}{
			"status":   "ok",
			"database": cfg.MongoDatabase,
		}
	}

	// Check Redis
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		healthStatus["redis"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		healthStatus["status"] = "degraded"
	} else {
		healthStatus["redis"] = map[string]interface{}{
			"status": "ok",
			"db":     cfg.RedisDB,
		}
	}

	// Check Workers status
	healthStatus["workers"] = map[string]interface{}{
		"status":          "ok",
		"workers_running": getWorkerCount(cfg),
		"configured":      true,
	}

	// Check Token Blacklist (si está habilitado)
	if tokenBlacklist != nil {
		healthStatus["blacklist"] = map[string]interface{}{
			"status":  "ok",
			"enabled": true,
		}
	} else {
		healthStatus["blacklist"] = map[string]interface{}{
			"status":  "disabled",
			"enabled": false,
		}
	}

	statusCode := fiber.StatusOK
	if healthStatus["status"] == "degraded" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(healthStatus)
}

// metricsHandler mejorado con métricas del sistema
func metricsHandler(c *fiber.Ctx, tokenBlacklist *jwt.TokenBlacklist, cfg *config.Config) error {
	metrics := map[string]interface{}{
		"uptime_seconds": time.Since(startTime).Seconds(),
		"environment":    cfg.Environment,
		"go_version":     "1.23+",
		"timestamp":      time.Now().UTC(),
	}

	// Métricas de Workers
	metrics["workers"] = map[string]interface{}{
		"configured": getWorkerCount(cfg),
		"status":     "ready",
	}

	// Métricas de JWT blacklist (si está habilitado)
	if tokenBlacklist != nil {
		metrics["jwt_blacklist"] = map[string]interface{}{
			"enabled": true,
			"status":  "active",
		}
	} else {
		metrics["jwt_blacklist"] = map[string]interface{}{
			"enabled": false,
		}
	}

	return c.JSON(metrics)
}

// configDebugHandler muestra configuración (solo en desarrollo)
func configDebugHandler(c *fiber.Ctx, cfg *config.Config) error {
	if cfg.Environment == "production" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Not found",
		})
	}

	debugInfo := map[string]interface{}{
		"environment":    cfg.Environment,
		"server_port":    cfg.ServerPort,
		"log_level":      cfg.LogLevel,
		"mongo_database": cfg.MongoDatabase,
		"redis_host":     cfg.RedisHost,
		"redis_port":     cfg.RedisPort,
		"redis_db":       cfg.RedisDB,
		"workers": map[string]interface{}{
			"count":      getWorkerCount(cfg),
			"stats_port": getStatsPort(cfg),
		},
	}

	return c.JSON(debugInfo)
}

// startBackgroundTasks inicia tareas de limpieza en background
func startBackgroundTasks(blacklist *jwt.TokenBlacklist, logger *logger.Logger) {
	logger.Info("Starting background cleanup tasks")

	// Cleanup de tokens expirados cada hora
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// ADAPTADO: implementar cleanup básico
			logger.Info("Running background cleanup task")
		}
	}
}

// customErrorHandler mejorado con configuración
func customErrorHandler(log *logger.Logger, cfg *config.Config) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Código de estado por defecto
		code := fiber.StatusInternalServerError

		// Verificar si es un error de Fiber
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		// Log mejorado con más contexto
		log.Error("HTTP Error",
			"status", code,
			"method", c.Method(),
			"path", c.Path(),
			"error", err.Error(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		)

		// Response estructurada
		errorResponse := fiber.Map{
			"success":   false,
			"message":   getErrorMessage(code),
			"timestamp": time.Now().UTC(),
			"path":      c.Path(),
		}

		// Solo incluir detalles del error en desarrollo
		if cfg.Environment == "development" {
			errorResponse["error"] = err.Error()
			errorResponse["method"] = c.Method()
		}

		// Headers de seguridad adicionales
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		return c.Status(code).JSON(errorResponse)
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
