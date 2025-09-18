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
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/repository/redis"
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

	// Crear logger zap para componentes que lo requieren
	zapLogger := zap.NewNop()

	appLogger.Info("Starting Engine API Workflow",
		"version", "2.0.0",
		"environment", cfg.Environment)

	// Conectar a MongoDB
	appLogger.Info("Connecting to MongoDB...")
	mongoClient, err := database.NewMongoConnection(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		appLogger.Fatal("Failed to connect to MongoDB", "error", err)
	}
	defer func() {
		if err := database.DisconnectMongoDB(mongoClient); err != nil {
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

	// Inicializar base de datos MongoDB
	mongoDB := mongoClient.Database(cfg.MongoDatabase)

	// Inicializar repositorios individuales
	userRepo := mongodb.NewUserRepository(mongoDB)
	workflowRepo := mongodb.NewWorkflowRepository(mongoDB)
	logRepo := mongodb.NewLogRepository(mongoDB)
	queueRepo := redis.NewQueueRepository(redisClient)

	// Inicializar servicios JWT usando la configuración correcta
	jwtConfig := cfg.GetJWTConfig()

	// Convertir JWTConfig a jwt.Config
	jwtServiceConfig := jwt.Config{
		SecretKey:       jwtConfig.Secret,
		AccessTokenTTL:  jwtConfig.AccessTokenTTL,
		RefreshTokenTTL: jwtConfig.RefreshTokenTTL,
		Issuer:          jwtConfig.Issuer,
		Audience:        jwtConfig.Audience,
	}
	jwtService := jwt.NewJWTService(jwtServiceConfig)

	// Inicializar TokenBlacklist si está habilitado
	var tokenBlacklist *jwt.TokenBlacklist
	if cfg.EnableTokenBlacklist {
		blacklistConfig := jwt.BlacklistConfig{
			RedisClient: redisClient,
			KeyPrefix:   "jwt_blacklist:",
		}
		tokenBlacklist = jwt.NewTokenBlacklist(blacklistConfig)
	}

	// Inicializar MetricsService primero
	metricsService := services.NewMetricsService(
		userRepo,
		workflowRepo,
		logRepo,
		queueRepo,
	)

	// Inicializar servicios individuales
	authService := services.NewAuthService(userRepo)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)
	queueService := services.NewQueueService(redisClient, zapLogger)

	// Inicializar BackupService
	appLogger.Info("Initializing backup service...")
	backupService := services.NewBackupService(
		cfg,
		zapLogger,
		userRepo,
		workflowRepo,
		logRepo,
		queueRepo,
	)

	// Inicializar DashboardService con MetricsService
	dashboardService := services.NewDashboardService(
		metricsService,
		workflowRepo,
		logRepo,
		userRepo,
		queueRepo,
	)

	// Inicializar Worker Engine
	appLogger.Info("Initializing worker engine...")
	workerConfig := worker.WorkerConfig{
		Workers:           getEnvAsInt("WORKER_POOL_MIN_SIZE", 3),
		MaxWorkers:        getEnvAsInt("WORKER_POOL_MAX_SIZE", 20),
		PollInterval:      getEnvAsDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		MaxRetries:        getEnvAsInt("DEFAULT_MAX_RETRIES", 3),
		RetryDelay:        getEnvAsDuration("DEFAULT_RETRY_DELAY", 30*time.Second),
		ProcessingTimeout: getEnvAsDuration("TASK_EXECUTION_TIMEOUT", 30*time.Minute),
	}

	workerEngine := worker.NewWorkerEngine(
		queueRepo,
		workflowRepo,
		logRepo,
		userRepo,
		logService,
		zapLogger,
		workerConfig,
	)

	// Inicializar middlewares
	authMiddleware := middleware.NewAuthMiddlewareWithBlacklist(jwtService, tokenBlacklist, appLogger)

	// Inicializar validator
	validator := utils.NewValidator()

	// Inicializar handlers individuales
	authHandler := handlers.NewAuthHandlerWithConfig(handlers.AuthHandlerConfig{
		UserRepo:       userRepo,
		AuthService:    authService,
		JWTService:     jwtService,
		TokenBlacklist: tokenBlacklist,
		Validator:      validator,
		Logger:         appLogger,
	})

	// Crear otros handlers
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService, *validator)
	dashboardHandler := handlers.NewDashboardHandler(dashboardService, zapLogger)
	workerHandler := handlers.NewWorkerHandler(queueRepo, workerEngine, zapLogger)
	triggerHandler := handlers.NewTriggerHandler(workflowRepo, logRepo, queueService, zapLogger)

	// Crear BackupHandler
	backupHandler := handlers.NewBackupHandler(backupService, zapLogger)

	// Crear WebHandler
	webHandler := handlers.NewWebHandler(
		userRepo,
		workflowService,
		logService,
		authService,
		jwtService,
	)

	// Inicializar Fiber
	app := fiber.New(fiber.Config{
		ServerHeader: "Engine-API-Workflow",
		AppName:      "Engine API Workflow v2.0.0",
		ErrorHandler: customErrorHandler(zapLogger),
		ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		BodyLimit:    getEnvAsBytes("API_MAX_REQUEST_SIZE", 10*1024*1024),
	})

	// Middlewares globales
	app.Use(recover.New())

	// CORS
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

	// Rate limiting
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

	// Request logging usando el logger personalizado
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		appLogger.Info("HTTP Request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", duration.Milliseconds())

		return err
	})

	// Configurar rutas básicas
	setupBasicRoutes(app, appLogger)
	setupWebHandlerRoutes(app, webHandler, authMiddleware)
	setupAPIRoutes(app, authHandler, workflowHandler, dashboardHandler, workerHandler, triggerHandler, backupHandler, authMiddleware, appLogger)

	appLogger.Info("Routes configured successfully")

	// Crear directorios web si es necesario
	if cfg.EnableWebInterface {
		if err := ensureWebDirectoryExists(); err != nil {
			appLogger.Warn("Web directory setup failed", "error", err)
		}
	}

	// Crear directorios de backup si es necesario
	if cfg.BackupEnabled {
		if err := ensureBackupDirectoryExists(cfg.BackupStoragePath); err != nil {
			appLogger.Warn("Backup directory setup failed", "error", err)
		}
	}

	// Contexto para shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Iniciar Worker Engine
	appLogger.Info("Starting worker engine...")
	if err := workerEngine.Start(ctx); err != nil {
		appLogger.Fatal("Failed to start worker engine", "error", err)
	}

	// Iniciar backups automatizados si está habilitado
	if cfg.BackupEnabled && cfg.BackupAutoEnabled {
		appLogger.Info("Starting automated backups...")
		if err := backupService.StartAutomatedBackups(ctx); err != nil {
			appLogger.Error("Failed to start automated backups", "error", err)
		} else {
			appLogger.Info("Automated backups started successfully")
		}
	}

	// Canal para señales
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Iniciar servicios en background
	go startBackgroundServices(dashboardService, backupService, appLogger, cfg)

	// Iniciar servidor
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server",
			"address", addr,
			"health_check", fmt.Sprintf("http://localhost:%s/api/v1/health", cfg.ServerPort))

		if err := app.Listen(addr); err != nil {
			appLogger.Error("Server failed to start", "error", err)
			sigChan <- syscall.SIGTERM
		}
	}()

	// Esperar señal
	<-sigChan
	appLogger.Info("Shutting down server...")

	// Parar backups automatizados
	if cfg.BackupEnabled && cfg.BackupAutoEnabled {
		appLogger.Info("Stopping automated backups...")
		if err := backupService.StopAutomatedBackups(); err != nil {
			appLogger.Error("Error stopping automated backups", "error", err)
		}
	}

	// Shutdown Worker Engine
	if err := workerEngine.Stop(); err != nil {
		appLogger.Error("Error stopping worker engine", "error", err)
	}

	// Shutdown servidor HTTP
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		appLogger.Error("Server forced to shutdown", "error", err)
	}

	appLogger.Info("Server exited gracefully")
}

// setupBasicRoutes configura rutas básicas
func setupBasicRoutes(app *fiber.App, logger *logger.Logger) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Engine API Workflow",
			"version": "2.0.0",
			"status":  "running",
		})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now(),
			"version":   "2.0.0",
		})
	})

	logger.Info("Basic routes configured")
}

// setupWebHandlerRoutes configura rutas del WebHandler
func setupWebHandlerRoutes(app *fiber.App, webHandler *handlers.WebHandler, authMiddleware *middleware.AuthMiddleware) {
	// Rutas públicas
	app.Get("/", webHandler.Index)
	app.Get("/login", webHandler.ShowLogin)
	app.Post("/login", webHandler.HandleLogin)

	// Rutas protegidas (requieren autenticación)
	protected := app.Group("/")
	protected.Use(authMiddleware.RequireAuth())
	protected.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/dashboard.html")
	})
}

// setupAPIRoutes configura rutas de la API (actualizada con backup routes)
func setupAPIRoutes(app *fiber.App,
	authHandler *handlers.AuthHandler,
	workflowHandler *handlers.WorkflowHandler,
	dashboardHandler *handlers.DashboardHandler,
	workerHandler *handlers.WorkerHandler,
	triggerHandler *handlers.TriggerHandler,
	backupHandler *handlers.BackupHandler,
	authMiddleware *middleware.AuthMiddleware,
	logger *logger.Logger) {

	// API v1 group
	api := app.Group("/api/v1")

	// Health check (público)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now(),
			"version":   "2.0.0",
		})
	})

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)

	// Rutas protegidas
	protected := api.Group("/")
	protected.Use(authMiddleware.RequireAuth())

	// Auth protegidas
	protected.Post("/auth/logout", authHandler.Logout)
	protected.Get("/auth/profile", authHandler.GetProfile)
	protected.Put("/auth/profile", authHandler.UpdateProfile)
	protected.Put("/auth/change-password", authHandler.ChangePassword)

	// Workflows
	workflows := protected.Group("/workflows")
	workflows.Post("/", workflowHandler.CreateWorkflow)
	workflows.Get("/", func(c *fiber.Ctx) error {
		// Implementación básica para ListWorkflows
		return c.JSON(fiber.Map{"message": "List workflows - coming soon"})
	})
	workflows.Get("/:id", workflowHandler.GetWorkflow)
	workflows.Put("/:id", workflowHandler.UpdateWorkflow)
	workflows.Delete("/:id", workflowHandler.DeleteWorkflow)
	workflows.Post("/:id/clone", workflowHandler.CloneWorkflow)
	workflows.Post("/:id/execute", func(c *fiber.Ctx) error {
		// Implementación básica para ExecuteWorkflow
		return c.JSON(fiber.Map{"message": "Execute workflow - coming soon"})
	})

	// Dashboard
	dashboard := protected.Group("/dashboard")
	dashboard.Get("/", dashboardHandler.GetDashboard)
	dashboard.Get("/stats", func(c *fiber.Ctx) error {
		// Implementación básica para GetStats
		return c.JSON(fiber.Map{"message": "Dashboard stats - coming soon"})
	})
	dashboard.Get("/summary", func(c *fiber.Ctx) error {
		// Implementación básica para GetSummary
		return c.JSON(fiber.Map{"message": "Dashboard summary - coming soon"})
	})
	dashboard.Get("/health", func(c *fiber.Ctx) error {
		// Implementación básica para GetSystemHealth
		return c.JSON(fiber.Map{"status": "healthy", "timestamp": time.Now()})
	})
	dashboard.Get("/recent-activity", dashboardHandler.GetRecentActivity)

	// Workers
	workers := protected.Group("/workers")
	workers.Get("/stats", func(c *fiber.Ctx) error {
		// Implementación básica para worker stats
		return c.JSON(fiber.Map{
			"active_workers": 3,
			"queue_length":   0,
			"processed":      0,
			"timestamp":      time.Now(),
		})
	})
	workers.Get("/status", func(c *fiber.Ctx) error {
		// Implementación básica para worker status
		return c.JSON(fiber.Map{
			"status": "running",
			"workers": map[string]interface{}{
				"total":  3,
				"active": 3,
				"idle":   0,
			},
		})
	})
	workers.Post("/start", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Workers started"})
	})
	workers.Post("/stop", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Workers stopped"})
	})

	// Backup routes (protegidas)
	backups := protected.Group("/backups")
	backups.Post("/", backupHandler.CreateBackup)
	backups.Get("/", backupHandler.ListBackups)
	backups.Get("/:id", backupHandler.GetBackup)
	backups.Delete("/:id", backupHandler.DeleteBackup)
	backups.Post("/:id/restore", backupHandler.RestoreBackup)
	backups.Post("/:id/validate", backupHandler.ValidateBackup)
	backups.Get("/status", backupHandler.GetBackupStatus)

	// Backup management
	backups.Post("/automated/start", backupHandler.StartAutomatedBackups)
	backups.Post("/automated/stop", backupHandler.StopAutomatedBackups)
	backups.Post("/cleanup", backupHandler.CleanupOldBackups)

	// Triggers (públicos para webhooks)
	triggers := api.Group("/triggers")
	triggers.Post("/webhook/:webhook_id", func(c *fiber.Ctx) error {
		// Implementación básica para HandleWebhook
		webhookID := c.Params("webhook_id")
		return c.JSON(fiber.Map{
			"message":    "Webhook received",
			"webhook_id": webhookID,
			"timestamp":  time.Now(),
		})
	})

	// Triggers protegidos
	protectedTriggers := protected.Group("/triggers")
	protectedTriggers.Post("/manual", func(c *fiber.Ctx) error {
		// Implementación básica para TriggerManual
		return c.JSON(fiber.Map{"message": "Manual trigger - coming soon"})
	})

	logger.Info("API routes configured with backup endpoints")
}

// customErrorHandler maneja errores
func customErrorHandler(zapLogger *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError

		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		zapLogger.Error("HTTP Error",
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

// ensureWebDirectoryExists crea directorios web
func ensureWebDirectoryExists() error {
	dirs := []string{"./web", "./web/templates", "./web/static"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// HTML básico
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
                <a href="/api/v1/dashboard/stats">Dashboard</a>
                <a href="/api/v1/backups/status">Backup Status</a>
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

// ensureBackupDirectoryExists crea el directorio de backups
func ensureBackupDirectoryExists(backupPath string) error {
	if backupPath == "" {
		backupPath = "./backups"
	}

	return os.MkdirAll(backupPath, 0755)
}

// startBackgroundServices servicios en background (actualizado con backup)
func startBackgroundServices(dashboardService services.DashboardService, backupService services.BackupService, appLogger *logger.Logger, cfg *config.Config) {
	appLogger.Info("Starting background services")

	// Ticker para servicios de dashboard
	dashboardTicker := time.NewTicker(30 * time.Second)
	defer dashboardTicker.Stop()

	// Ticker para limpieza de backups (cada hora)
	var backupCleanupTicker *time.Ticker
	if cfg.BackupEnabled {
		backupCleanupTicker = time.NewTicker(1 * time.Hour)
		defer backupCleanupTicker.Stop()
	}

	for {
		select {
		case <-dashboardTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := dashboardService.RefreshDashboardData(ctx); err != nil {
				appLogger.Debug("Failed to refresh dashboard data", "error", err)
			}
			cancel()

		case <-func() <-chan time.Time {
			if backupCleanupTicker != nil {
				return backupCleanupTicker.C
			}
			// Return a channel that never sends if backup is disabled
			return make(<-chan time.Time)
		}():
			if cfg.BackupEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if err := backupService.CleanupOldBackups(ctx); err != nil {
					appLogger.Debug("Failed to cleanup old backups", "error", err)
				}
				cancel()
			}
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
