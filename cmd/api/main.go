package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/repository/redis"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/worker"
	"Engine_API_Workflow/pkg/database"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

var startTime = time.Now() // ✅ Para calcular uptime correctamente

func main() {
	// Cargar configuración (ya incluye validación completa)
	cfg := config.Load()

	// Inicializar logger
	appLogger := logger.New(cfg.LogLevel, cfg.Environment)
	appLogger.Info("Starting Engine API Workflow", "version", "1.0.0", "environment", cfg.Environment)

	// ✅ Log de configuración cargada (sin secrets)
	cfg.LogConfig()

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

	// ✅ Inicializar Token Blacklist (solo si está habilitado)
	var tokenBlacklist *jwt.TokenBlacklist
	if cfg.EnableTokenBlacklist {
		appLogger.Info("Initializing JWT Token Blacklist...")
		blacklistConfig := jwt.BlacklistConfig{
			RedisClient: redisClient,
			KeyPrefix:   "jwt_blacklist:",
		}
		tokenBlacklist = jwt.NewTokenBlacklist(blacklistConfig)

		// Verificar que el blacklist funcione
		if err := tokenBlacklist.HealthCheck(context.Background()); err != nil {
			appLogger.Fatal("Failed to initialize token blacklist", "error", err)
		}
		appLogger.Info("Token blacklist initialized successfully")
	} else {
		appLogger.Info("Token blacklist disabled by configuration")
	}

	// ✅ Usar configuración JWT del config (ya validada)
	jwtConfig := cfg.GetJWTConfig()

	// ✅ Inicializar servicio JWT seguro
	jwtService := jwt.NewJWTService(jwt.Config{
		SecretKey:       jwtConfig.Secret,
		AccessTokenTTL:  jwtConfig.AccessTokenTTL,
		RefreshTokenTTL: jwtConfig.RefreshTokenTTL,
		Issuer:          jwtConfig.Issuer,
	})
	appLogger.Info("JWT service initialized with secure configuration",
		"access_ttl", jwtConfig.AccessTokenTTL,
		"refresh_ttl", jwtConfig.RefreshTokenTTL,
		"issuer", jwtConfig.Issuer)

	// ✅ Inicializar repositorios para workers
	db := mongoClient.Database(cfg.MongoDatabase)
	userRepo := mongodb.NewUserRepository(db)
	workflowRepo := mongodb.NewWorkflowRepository(db)
	logRepo := mongodb.NewLogRepository(db)
	queueRepo := redis.NewQueueRepository(redisClient)

	// ✅ Inicializar servicios de negocio para workers
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)
	queueService := services.NewQueueService(redisClient, appLogger.Zap())

	// ✅ Configurar Worker Engine
	workerConfig := worker.WorkerConfig{
		Workers:           getWorkerCount(cfg), // Número de workers según entorno
		PollInterval:      5 * time.Second,     // Intervalo de polling
		MaxRetries:        3,                   // Máximo reintentos
		RetryDelay:        30 * time.Second,    // Delay entre reintentos
		ProcessingTimeout: 30 * time.Minute,    // Timeout máximo por workflow
	}

	workerEngine := worker.NewWorkerEngine(
		queueRepo,
		workflowRepo,
		logRepo,
		userRepo,
		logService,
		appLogger.Zap(),
		workerConfig,
	)

	// ✅ Configurar Fiber con configuraciones de seguridad de config
	app := fiber.New(fiber.Config{
		ServerHeader:            "Engine-API-Workflow",
		AppName:                 "Engine API Workflow v1.0.0",
		ErrorHandler:            customErrorHandler(appLogger, cfg),
		ReadTimeout:             time.Second * 30,
		WriteTimeout:            time.Second * 30,
		IdleTimeout:             time.Second * 30,
		DisableStartupMessage:   cfg.IsProduction(),        // ✅ Usar método del config
		EnableTrustedProxyCheck: true,                      // ✅ Seguridad proxy
		TrustedProxies:          cfg.TrustedProxies,        // ✅ Usar proxies del config
		ProxyHeader:             fiber.HeaderXForwardedFor, // ✅ Header de proxy confiable
	})

	// ✅ Configurar rutas con configuración completa incluyendo workers
	routeConfig := &routes.RouteConfig{
		DB:             db,
		JWTService:     jwtService,
		TokenBlacklist: tokenBlacklist, // Puede ser nil si está deshabilitado
		Logger:         appLogger,
		Config:         cfg,
		// ✅ Agregar dependencias para workers
		WorkerEngine: workerEngine,
		QueueRepo:    queueRepo,
		QueueService: queueService,
	}
	routes.SetupSecureRoutes(app, routeConfig)

	// ✅ Health check mejorado con verificación de workers
	app.Get("/health", func(c *fiber.Ctx) error {
		return healthCheckHandler(c, mongoClient, redisClient, tokenBlacklist, workerEngine, cfg)
	})

	// ✅ Endpoint de métricas mejorado con stats de workers
	app.Get("/metrics", func(c *fiber.Ctx) error {
		return metricsHandler(c, tokenBlacklist, workerEngine, cfg)
	})

	// ✅ Endpoint de configuración (solo en desarrollo)
	if cfg.IsDevelopment() {
		app.Get("/debug/config", func(c *fiber.Ctx) error {
			return configDebugHandler(c, cfg)
		})
	}

	// Crear contexto principal con cancelación para workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ✅ Iniciar Worker Engine
	appLogger.Info("Starting Worker Engine...", "workers", workerConfig.Workers)
	if err := workerEngine.Start(ctx); err != nil {
		appLogger.Fatal("Failed to start Worker Engine", "error", err)
	}
	appLogger.Info("Worker Engine started successfully")

	// Canal para manejar señales del sistema
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// ✅ Iniciar tareas de background si están habilitadas
	if tokenBlacklist != nil {
		go startBackgroundTasks(tokenBlacklist, appLogger)
	}

	// ✅ Iniciar servidor de estadísticas de workers en puerto separado
	go func() {
		statsAddr := fmt.Sprintf(":%d", getStatsPort(cfg))
		statsApp := fiber.New(fiber.Config{
			AppName:               "Worker Stats API",
			DisableStartupMessage: true,
		})

		// Endpoint de estadísticas de workers
		statsApp.Get("/stats", func(c *fiber.Ctx) error {
			stats, err := workerEngine.GetStats(ctx)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(stats)
		})

		// Health check específico para workers
		statsApp.Get("/health", func(c *fiber.Ctx) error {
			stats, err := workerEngine.GetStats(ctx)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"status": "error",
					"error":  err.Error(),
				})
			}

			isRunning, ok := stats["is_running"].(bool)
			if !ok || !isRunning {
				return c.Status(500).JSON(fiber.Map{
					"status": "error",
					"error":  "workers not running",
				})
			}

			return c.JSON(fiber.Map{
				"status":  "ok",
				"service": "worker-engine",
				"time":    time.Now(),
				"stats":   stats,
			})
		})

		// Endpoint de cola stats
		statsApp.Get("/queue", func(c *fiber.Ctx) error {
			queueStats, err := queueRepo.GetQueueStats(ctx)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
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
			"jwt_access_ttl", cfg.JWTAccessTTL,
			"jwt_refresh_ttl", cfg.JWTRefreshTTL,
			"workers_enabled", true,
			"workers_count", workerConfig.Workers,
			"features_enabled", map[string]bool{
				"blacklist":  cfg.EnableTokenBlacklist,
				"rate_limit": cfg.EnableRateLimit,
				"cors":       cfg.EnableCORS,
				"workers":    true,
			},
		)

		if err := app.Listen(addr); err != nil {
			appLogger.Fatal("Server failed to start", "error", err)
		}
	}()

	// Esperar señal de interrupción
	<-sigChan
	appLogger.Info("Shutting down server...")

	// ✅ Graceful shutdown - Detener Worker Engine primero
	appLogger.Info("Stopping Worker Engine...")
	if err := workerEngine.Stop(); err != nil {
		appLogger.Error("Error stopping Worker Engine", "error", err)
	} else {
		appLogger.Info("Worker Engine stopped successfully")
	}

	// ✅ Cancelar contexto para detener todas las goroutines de workers
	cancel()

	// ✅ Graceful shutdown del servidor HTTP
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Cerrar servidor HTTP
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		appLogger.Error("Server forced to shutdown", "error", err)
	}

	appLogger.Info("Server exited successfully")
}

// ✅ getWorkerCount devuelve el número de workers según el entorno
func getWorkerCount(cfg *config.Config) int {
	if cfg.IsProduction() {
		return 5 // Más workers en producción
	}
	return 3 // Workers en desarrollo/testing
}

// ✅ getStatsPort devuelve el puerto para el servidor de estadísticas
func getStatsPort(cfg *config.Config) int {
	if cfg.IsProduction() {
		return 8083 // Puerto diferente en producción
	}
	return 8082 // Puerto para desarrollo
}

// ✅ healthCheckHandler mejorado con verificación de workers
func healthCheckHandler(c *fiber.Ctx, mongoClient *mongo.Client, redisClient *redis.Client, tokenBlacklist *jwt.TokenBlacklist, workerEngine *worker.WorkerEngine, cfg *config.Config) error {
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

	// ✅ Check Workers
	if workerStats, err := workerEngine.GetStats(context.Background()); err != nil {
		healthStatus["workers"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		healthStatus["status"] = "degraded"
	} else {
		isRunning, ok := workerStats["is_running"].(bool)
		if !ok || !isRunning {
			healthStatus["workers"] = map[string]interface{}{
				"status": "error",
				"error":  "workers not running",
			}
			healthStatus["status"] = "degraded"
		} else {
			healthStatus["workers"] = map[string]interface{}{
				"status":          "ok",
				"workers_running": workerStats["workers_running"],
				"queue_stats":     workerStats["queue_stats"],
			}
		}
	}

	// Check Token Blacklist (si está habilitado)
	if tokenBlacklist != nil {
		if err := tokenBlacklist.HealthCheck(context.Background()); err != nil {
			healthStatus["blacklist"] = map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			}
			healthStatus["status"] = "degraded"
		} else {
			healthStatus["blacklist"] = map[string]interface{}{
				"status":  "ok",
				"enabled": true,
			}
		}
	} else {
		healthStatus["blacklist"] = map[string]interface{}{
			"status":  "disabled",
			"enabled": false,
		}
	}

	// Features status
	healthStatus["features"] = map[string]bool{
		"token_blacklist": cfg.EnableTokenBlacklist,
		"rate_limit":      cfg.EnableRateLimit,
		"cors":            cfg.EnableCORS,
		"workers":         true, // ✅ Workers siempre habilitados
	}

	statusCode := fiber.StatusOK
	if healthStatus["status"] == "degraded" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(healthStatus)
}

// ✅ metricsHandler mejorado con métricas de workers
func metricsHandler(c *fiber.Ctx, tokenBlacklist *jwt.TokenBlacklist, workerEngine *worker.WorkerEngine, cfg *config.Config) error {
	metrics := map[string]interface{}{
		"uptime_seconds": time.Since(startTime).Seconds(),
		"environment":    cfg.Environment,
		"go_version":     "1.23+",
		"timestamp":      time.Now().UTC(),
	}

	// ✅ Métricas de Workers
	if workerStats, err := workerEngine.GetStats(context.Background()); err == nil {
		metrics["workers"] = workerStats
	} else {
		metrics["workers"] = map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Métricas de JWT blacklist (si está habilitado)
	if tokenBlacklist != nil {
		if blacklistStats, err := tokenBlacklist.GetStats(context.Background()); err == nil {
			metrics["jwt_blacklist"] = blacklistStats
		} else {
			metrics["jwt_blacklist"] = map[string]interface{}{
				"error": err.Error(),
			}
		}
	} else {
		metrics["jwt_blacklist"] = map[string]interface{}{
			"enabled": false,
		}
	}

	// Métricas de configuración
	metrics["config"] = map[string]interface{}{
		"jwt_access_ttl":  cfg.JWTAccessTTL.String(),
		"jwt_refresh_ttl": cfg.JWTRefreshTTL.String(),
		"features": map[string]bool{
			"blacklist":  cfg.EnableTokenBlacklist,
			"rate_limit": cfg.EnableRateLimit,
			"cors":       cfg.EnableCORS,
			"workers":    true,
		},
	}

	return c.JSON(metrics)
}

// ✅ configDebugHandler muestra configuración (solo en desarrollo)
func configDebugHandler(c *fiber.Ctx, cfg *config.Config) error {
	if cfg.IsProduction() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Not found",
		})
	}

	debugInfo := map[string]interface{}{
		"environment":     cfg.Environment,
		"server_port":     cfg.ServerPort,
		"log_level":       cfg.LogLevel,
		"mongo_database":  cfg.MongoDatabase,
		"redis_host":      cfg.RedisHost,
		"redis_port":      cfg.RedisPort,
		"redis_db":        cfg.RedisDB,
		"jwt_issuer":      cfg.JWTIssuer,
		"jwt_audience":    cfg.JWTAudience,
		"jwt_access_ttl":  cfg.JWTAccessTTL.String(),
		"jwt_refresh_ttl": cfg.JWTRefreshTTL.String(),
		"cors_origins":    cfg.GetCORSOrigins(),
		"trusted_proxies": cfg.TrustedProxies,
		"features": map[string]bool{
			"token_blacklist": cfg.EnableTokenBlacklist,
			"rate_limit":      cfg.EnableRateLimit,
			"cors":            cfg.EnableCORS,
			"workers":         true, // ✅ Workers siempre habilitados
		},
		"rate_limit": map[string]interface{}{
			"requests": cfg.RateLimitRequests,
			"window":   cfg.RateLimitWindow.String(),
		},
		"workers": map[string]interface{}{
			"count":      getWorkerCount(cfg),
			"stats_port": getStatsPort(cfg),
		},
	}

	return c.JSON(debugInfo)
}

// ✅ startBackgroundTasks inicia tareas de limpieza en background
func startBackgroundTasks(blacklist *jwt.TokenBlacklist, logger *logger.Logger) {
	logger.Info("Starting background cleanup tasks")

	// Cleanup de tokens expirados cada hora
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			deleted, err := blacklist.CleanupExpiredTokens(ctx)
			if err != nil {
				logger.Error("Failed to cleanup expired tokens", "error", err)
			} else if deleted > 0 {
				logger.Info("Cleaned up expired tokens", "deleted_count", deleted)
			}

			cancel()
		}
	}
}

// ✅ customErrorHandler mejorado con configuración
func customErrorHandler(log *logger.Logger, cfg *config.Config) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Código de estado por defecto
		code := fiber.StatusInternalServerError

		// Verificar si es un error de Fiber
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		// ✅ Log mejorado con más contexto
		log.Error("HTTP Error",
			"status", code,
			"method", c.Method(),
			"path", c.Path(),
			"error", err.Error(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
			"referer", c.Get("Referer"),
			"request_id", c.Get("X-Request-ID", "unknown"),
		)

		// ✅ Response estructurada
		errorResponse := fiber.Map{
			"success":   false,
			"message":   getErrorMessage(code),
			"timestamp": time.Now().UTC(),
			"path":      c.Path(),
		}

		// ✅ Solo incluir detalles del error en desarrollo
		if cfg.IsDevelopment() {
			errorResponse["error"] = err.Error()
			errorResponse["method"] = c.Method()
			errorResponse["stack"] = fmt.Sprintf("%+v", err) // Stack trace en desarrollo
		}

		// ✅ Headers de seguridad adicionales
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
