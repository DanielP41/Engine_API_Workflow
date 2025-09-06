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

	// Obtener base de datos
	db := mongoClient.Database(cfg.MongoDatabase)

	// ✅ Configurar rutas con configuración completa
	routeConfig := &routes.RouteConfig{
		DB:             db,
		JWTService:     jwtService,
		TokenBlacklist: tokenBlacklist, // Puede ser nil si está deshabilitado
		Logger:         appLogger,
		Config:         cfg,
	}
	routes.SetupSecureRoutes(app, routeConfig)

	// ✅ Health check mejorado con más verificaciones
	app.Get("/health", func(c *fiber.Ctx) error {
		return healthCheckHandler(c, mongoClient, redisClient, tokenBlacklist, cfg)
	})

	// ✅ Endpoint de métricas mejorado
	app.Get("/metrics", func(c *fiber.Ctx) error {
		return metricsHandler(c, tokenBlacklist, cfg)
	})

	// ✅ Endpoint de configuración (solo en desarrollo)
	if cfg.IsDevelopment() {
		app.Get("/debug/config", func(c *fiber.Ctx) error {
			return configDebugHandler(c, cfg)
		})
	}

	// Canal para manejar señales del sistema
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// ✅ Iniciar tareas de background si están habilitadas
	if tokenBlacklist != nil {
		go startBackgroundTasks(tokenBlacklist, appLogger)
	}

	// Iniciar el servidor en una goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		appLogger.Info("Starting HTTP server",
			"address", addr,
			"environment", cfg.Environment,
			"jwt_access_ttl", cfg.JWTAccessTTL,
			"jwt_refresh_ttl", cfg.JWTRefreshTTL,
			"features_enabled", map[string]bool{
				"blacklist":  cfg.EnableTokenBlacklist,
				"rate_limit": cfg.EnableRateLimit,
				"cors":       cfg.EnableCORS,
			},
		)

		if err := app.Listen(addr); err != nil {
			appLogger.Fatal("Server failed to start", "error", err)
		}
	}()

	// Esperar señal de interrupción
	<-sigChan
	appLogger.Info("Shutting down server...")

	// ✅ Graceful shutdown con cleanup de recursos
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Cerrar servidor HTTP
	if err := app.ShutdownWithContext(ctx); err != nil {
		appLogger.Error("Server forced to shutdown", "error", err)
	}

	appLogger.Info("Server exited successfully")
}

// ✅ healthCheckHandler maneja el endpoint de health check
func healthCheckHandler(c *fiber.Ctx, mongoClient *mongo.Client, redisClient *redis.Client, tokenBlacklist *jwt.TokenBlacklist, cfg *config.Config) error {
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
	}

	statusCode := fiber.StatusOK
	if healthStatus["status"] == "degraded" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(healthStatus)
}

// ✅ metricsHandler maneja el endpoint de métricas
func metricsHandler(c *fiber.Ctx, tokenBlacklist *jwt.TokenBlacklist, cfg *config.Config) error {
	metrics := map[string]interface{}{
		"uptime_seconds": time.Since(startTime).Seconds(),
		"environment":    cfg.Environment,
		"go_version":     "1.21+",
		"timestamp":      time.Now().UTC(),
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
		},
		"rate_limit": map[string]interface{}{
			"requests": cfg.RateLimitRequests,
			"window":   cfg.RateLimitWindow.String(),
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
