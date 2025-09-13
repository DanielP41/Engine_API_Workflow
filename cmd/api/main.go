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
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/helmet"
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
		if intValue, err := strconv.Atoi(cleanValue); err == nil {
			return intValue * multiplier
		}
	}
	return defaultValue
}zap.String("version", "2.0.0"),
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
		ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),  // 🔧 CORREGIDO
		WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second), // 🔧 CORREGIDO
		IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", 120*time.Second), // 🔧 CORREGIDO
		BodyLimit:    getEnvAsBytes("API_MAX_REQUEST_SIZE", 10*1024*1024), // 10MB
		// 🆕 Configuraciones adicionales para mejor performance
		Prefork:                 false, // Disable prefork para desarrollo
		DisableHeaderNormalizing: false,
		StrictRouting:           false,
		CaseSensitive:          false,
		ETag:                   true,
		// 🆕 Configuración de proxy
		ProxyHeader: fiber.HeaderXForwardedFor,
		TrustedProxies: cfg.TrustedProxies,
	})

	// 🆕 MIDDLEWARES GLOBALES MEJORADOS
	
	// Recovery middleware (debe ir primero)
	app.Use(recover.New(recover.Config{
		EnableStackTrace: cfg.IsDevelopment(),
	}))

	// 🆕 Security headers
	if !cfg.IsDevelopment() {
		app.Use(helmet.New(helmet.Config{
			XSSProtection:         "1; mode=block",
			ContentTypeNosniff:    "nosniff",
			XFrameOptions:         "DENY",
			ReferrerPolicy:        "no-referrer",
			CrossOriginEmbedderPolicy: "require-corp",
			CrossOriginOpenerPolicy:   "same-origin",
			CrossOriginResourcePolicy: "cross-origin",
			OriginAgentCluster:        "?1",
		}))
	}

	// 🆕 CORS MIDDLEWARE COMPLETO
	if cfg.EnableCORS {
		corsConfig := cfg.GetCORSConfig()
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(corsConfig.AllowedOrigins, ","),
			AllowMethods:     strings.Join(corsConfig.AllowedMethods, ","),
			AllowHeaders:     strings.Join(corsConfig.AllowedHeaders, ","),
			AllowCredentials: corsConfig.AllowCredentials,
			ExposeHeaders:    strings.Join(corsConfig.ExposedHeaders, ","),
			MaxAge:          corsConfig.MaxAge,
			// 🆕 Configuraciones adicionales
			Next: func(c *fiber.Ctx) bool {
				// Skip CORS para rutas internas
				return strings.HasPrefix(c.Path(), "/internal/")
			},
		}))

		// Log CORS configuration
		appLogger.Info("CORS enabled",
			zap.Strings("allowed_origins", corsConfig.AllowedOrigins),
			zap.Bool("allow_credentials", corsConfig.AllowCredentials),
			zap.Strings("allowed_methods", corsConfig.AllowedMethods))
	}

	// 🆕 Rate Limiting
	if cfg.EnableRateLimit {
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.RateLimitRequests,
			Expiration: cfg.RateLimitWindow,
			KeyGenerator: func(c *fiber.Ctx) string {
				// Rate limit por IP + User Agent para mejor control
				return c.IP() + c.Get("User-Agent")
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error":   "Rate limit exceeded",
					"message": fmt.Sprintf("Maximum %d requests per %v", cfg.RateLimitRequests, cfg.RateLimitWindow),
					"retry_after": cfg.RateLimitWindow.Seconds(),
				})
			},
			SkipFailedRequests: true,
			SkipSuccessfulRequests: false,
			// Skip rate limiting para health checks
			Skip: func(c *fiber.Ctx) bool {
				return strings.HasPrefix(c.Path(), "/api/v1/health") ||
					   strings.HasPrefix(c.Path(), "/monitoring/")
			},
		}))

		appLogger.Info("Rate limiting enabled",
			zap.Int("max_requests", cfg.RateLimitRequests),
			zap.Duration("window", cfg.RateLimitWindow))
	}

	// 🆕 ARCHIVOS ESTÁTICOS PARA FRONTEND WEB
	if cfg.EnableWebInterface {
		// Servir archivos estáticos (CSS, JS, imágenes)
		app.Static("/static", cfg.StaticFilesPath, fiber.Static{
			Compress:  true,
			ByteRange: true,
			Browse:    cfg.IsDevelopment(), // Solo en desarrollo
			MaxAge:   3600, // 1 hora de cache
		})

		// Servir archivos del directorio web como fallback
		app.Static("/", "./web", fiber.Static{
			Compress: true,
			MaxAge:   1800, // 30 minutos de cache
		})

		appLogger.Info("Web interface enabled",
			zap.String("static_path", cfg.StaticFilesPath),
			zap.String("templates_path", cfg.TemplatesPath))
	}

	// 🆕 REQUEST LOGGING MIDDLEWARE
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()

		// Continuar con la request
		err := c.Next()

		// Log después de procesar
		duration := time.Since(start)
		
		// Solo log requests que no sean health checks en desarrollo
		if cfg.IsDevelopment() && strings.HasPrefix(c.Path(), "/api/v1/health") {
			return err
		}

		logLevel := zap.InfoLevel
		if c.Response().StatusCode() >= 400 {
			logLevel = zap.WarnLevel
		}
		if c.Response().StatusCode() >= 500 {
			logLevel = zap.ErrorLevel
		}

		appLogger.Log(logLevel, "HTTP Request",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", duration),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
			zap.Int("size", len(c.Response().Body())))

		return err
	})

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

	// 🆕 RUTAS WEB ADICIONALES (para frontend)
	if cfg.EnableWebInterface {
		setupWebRoutes(app, handlersContainer, authMiddleware, appLogger)
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
	if cfg.EnableWebInterface {
		if err := ensureWebDirectoryExists(); err != nil {
			appLogger.Warn("Web directory setup failed, dashboard UI may be limited", zap.Error(err))
		}
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
			zap.String("worker_stats", fmt.Sprintf("http://localhost:%s/api/v1/workers/advanced/stats", cfg.ServerPort)),
			zap.Bool("cors_enabled", cfg.EnableCORS),
			zap.Bool("web_interface", cfg.EnableWebInterface),
			zap.Bool("rate_limiting", cfg.EnableRateLimit))

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

// 🆕 setupWebRoutes configura rutas específicas para el frontend web
func setupWebRoutes(app *fiber.App, handlers *handlers.Handlers, authMiddleware middleware.AuthMiddleware, logger *zap.Logger) {
	// Ruta raíz redirige al dashboard
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/dashboard")
	})

	// Dashboard página principal
	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/dashboard.html")
	})

	// 🆕 Rutas adicionales para SPA support
	spaRoutes := []string{"/workflows", "/users", "/logs", "/settings"}
	for _, route := range spaRoutes {
		app.Get(route, func(c *fiber.Ctx) error {
			return c.SendFile("./web/templates/dashboard.html")
		})
	}

	// 🆕 API Status endpoint para frontend
	app.Get("/api/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "online",
			"version":   "2.0.0",
			"timestamp": time.Now(),
			"features": fiber.Map{
				"cors_enabled":      true,
				"rate_limiting":     true,
				"web_interface":     true,
				"worker_engine":     "advanced",
				"authentication":    "jwt",
			},
		})
	})

	logger.Info("Web routes configured successfully")
}

// customErrorHandler maneja los errores de la aplicación (MEJORADO)
func customErrorHandler(appLogger *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Código de estado por defecto
		code := fiber.StatusInternalServerError

		// Verificar si es un error de Fiber
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		// 🆕 Determinar si es una request de API o web
		isAPIRequest := strings.HasPrefix(c.Path(), "/api/")
		
		// Log del error
		appLogger.Error("HTTP Error",
			zap.Error(err),
			zap.String("path", c.Path()),
			zap.String("method", c.Method()),
			zap.Int("status", code),
			zap.String("ip", c.IP()),
			zap.Bool("is_api", isAPIRequest))

		// 🆕 Respuesta diferenciada para API vs Web
		if isAPIRequest {
			// Respuesta JSON para API
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
				"path":    c.Path(),
				"status":  code,
				"version": "2.0",
				"timestamp": time.Now(),
			})
		} else {
			// Respuesta HTML para web (si está habilitado)
			if code == fiber.StatusNotFound {
				return c.Status(code).SendFile("./web/templates/404.html")
			}
			// Para otros errores, retornar JSON también
			return c.Status(code).JSON(fiber.Map{
				"error":   err.Error(),
				"status":  code,
				"message": "An error occurred",
			})
		}
	}
}

// ensureWebDirectoryExists verifica y crea el directorio web si no existe (MEJORADO)
func ensureWebDirectoryExists() error {
	webDir := "./web"
	templatesDir := "./web/templates"
	staticDir := "./web/static"
	cssDir := "./web/static/css"
	jsDir := "./web/static/js"

	// Crear directorios si no existen
	dirs := []string{webDir, templatesDir, staticDir, cssDir, jsDir}
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
        .cors-info { background: #e3f2fd; padding: 15px; border-radius: 8px; margin: 15px 0; border-left: 4px solid #2196f3; }
        .api-links { display: flex; flex-wrap: wrap; gap: 12px; margin: 20px 0; }
        .api-links a { background: linear-gradient(45deg, #007bff, #0056b3); color: white; padding: 12px 20px; text-decoration: none; border-radius: 8px; transition: transform 0.2s; }
        .api-links a:hover { transform: translateY(-2px); box-shadow: 0 4px 12px rgba(0,123,255,0.3); }
        .version { background: #f8f9fa; padding: 8px 16px; border-radius: 20px; display: inline-block; font-size: 0.9em; margin-left: 10px; }
        .feature-badge { background: #28a745; color: white; padding: 4px 8px; border-radius: 12px; font-size: 0.8em; margin: 0 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Engine API Workflow <span class="version">v2.0</span></h1>
            <p class="status">
                Sistema Operacional - Workers Avanzados Activos
                <span class="feature-badge">CORS</span>
                <span class="feature-badge">Web UI</span>
                <span class="feature-badge">Rate Limiting</span>
            </p>
            <div class="cors-info">
                <strong>✅ CORS Configurado:</strong> Frontend web completamente funcional con autenticación por cookies
            </div>
        </div>
        
        <div class="cards">
            <div class="card">
                <h3>🚀 API Endpoints Básicos</h3>
                <div class="api-links">
                    <a href="/api/v1/health">Health Check</a>
                    <a href="/api/v1/dashboard/overview">Dashboard</a>
                    <a href="/api/v1/workflows">Workflows</a>
                    <a href="/api/status">API Status</a>
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

	// 🆕 Crear archivo 404.html básico
	notFoundPath := "./web/templates/404.html"
	if _, err := os.Stat(notFoundPath); os.IsNotExist(err) {
		notFoundHTML := `<!DOCTYPE html>
<html>
<head>
    <title>404 - Page Not Found</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; display: flex; align-items: center; justify-content: center; }
        .container { text-align: center; background: rgba(255,255,255,0.95); padding: 60px; border-radius: 12px; backdrop-filter: blur(10px); }
        h1 { font-size: 4em; margin: 0; color: #333; }
        p { font-size: 1.2em; color: #666; margin: 20px 0; }
        a { background: linear-gradient(45deg, #007bff, #0056b3); color: white; padding: 12px 24px; text-decoration: none; border-radius: 8px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>404</h1>
        <p>Página no encontrada</p>
        <a href="/dashboard">Volver al Dashboard</a>
    </div>
</body>
</html>`
		if err := os.WriteFile(notFoundPath, []byte(notFoundHTML), 0644); err != nil {
			return fmt.Errorf("failed to create 404 HTML file: %w", err)
		}
	}

	return nil
}

// startBackgroundServices inicia servicios en background (MEJORADO)
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
				appLogger.Debug("Failed to refresh dashboard data", zap.Error(err)) // Changed to Debug
			}
			cancel()
		}
	}
}

// Helper functions para configuración (MANTENIDAS)
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