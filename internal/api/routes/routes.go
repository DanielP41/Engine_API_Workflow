package routes

import (
	"time"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"go.mongodb.org/mongo-driver/mongo"
)

// ✅ RouteConfig actualizada con todas las dependencias necesarias
type RouteConfig struct {
	DB             *mongo.Database
	JWTService     jwt.JWTService
	TokenBlacklist *jwt.TokenBlacklist // Puede ser nil si está deshabilitado
	Logger         *logger.Logger
	Config         *config.Config
}

// ✅ SetupSecureRoutes configura todas las rutas con seguridad mejorada
func SetupSecureRoutes(app *fiber.App, config *RouteConfig) {
	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(config.DB)
	workflowRepo := mongodb.NewWorkflowRepository(config.DB)
	logRepo := mongodb.NewLogRepository(config.DB)

	// Inicializar servicios
	authService := services.NewAuthService(userRepo)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)

	// Inicializar validador
	validator := utils.NewValidator()

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(authService, config.JWTService, validator)
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService, validator)
	logHandler := handlers.NewLogHandler(logRepo, config.Logger.GetZapLogger()) // Assuming GetZapLogger method

	// ✅ Configurar middlewares globales de seguridad
	setupGlobalMiddlewares(app, config)

	// Grupo principal de la API
	api := app.Group("/api/v1")

	// ✅ Configurar rutas públicas
	setupPublicRoutes(api, authHandler, config)

	// ✅ Configurar middleware de autenticación con blacklist
	authMiddleware := createAuthMiddleware(config)

	// ✅ Configurar rutas protegidas
	setupProtectedRoutes(api, authHandler, workflowHandler, logHandler, authMiddleware, config)

	// ✅ Configurar rutas de administración
	setupAdminRoutes(api, authMiddleware, config)

	// ✅ Configurar webhooks públicos
	setupWebhookRoutes(api, config)

	// ✅ Ruta catch-all para 404
	setupNotFoundHandler(app, config)
}

// ✅ setupGlobalMiddlewares configura middlewares globales
func setupGlobalMiddlewares(app *fiber.App, config *RouteConfig) {
	// CORS (si está habilitado)
	if config.Config.EnableCORS {
		corsConfig := middleware.CORSConfig{
			AllowOrigins:     config.Config.GetCORSOrigins(),
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Request-ID"},
			AllowCredentials: true,
			MaxAge:           86400, // 24 horas
		}

		if config.Config.IsProduction() {
			app.Use(middleware.DynamicCORSMiddleware(corsConfig))
		} else {
			app.Use(middleware.CORSMiddleware(corsConfig))
		}
	}

	// Rate limiting (si está habilitado)
	if config.Config.EnableRateLimit {
		app.Use(limiter.New(limiter.Config{
			Max:        config.Config.RateLimitRequests,
			Expiration: config.Config.RateLimitWindow,
			KeyGenerator: func(c *fiber.Ctx) string {
				// Rate limit por IP, pero considerar headers de proxy
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				config.Logger.Warn("Rate limit exceeded",
					"ip", c.IP(),
					"path", c.Path(),
					"user_agent", c.Get("User-Agent"),
				)
				return c.Status(fiber.StatusTooManyRequests).JSON(utils.ErrorResponse("Rate limit exceeded", "Too many requests"))
			},
			SkipFailedRequests:     false,
			SkipSuccessfulRequests: false,
		}))
	}

	// Headers de seguridad
	app.Use(middleware.SecurityHeadersMiddleware())
	app.Use(middleware.APIResponseHeadersMiddleware())

	// Request ID para trazabilidad
	app.Use(func(c *fiber.Ctx) error {
		if c.Get("X-Request-ID") == "" {
			// Generar request ID simple si no existe
			requestID := generateRequestID()
			c.Set("X-Request-ID", requestID)
		}
		return c.Next()
	})
}

// ✅ setupPublicRoutes configura rutas que no requieren autenticación
func setupPublicRoutes(api fiber.Router, authHandler *handlers.AuthHandler, config *RouteConfig) {
	// Health check básico (público)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"message":   "Engine API Workflow is running",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})

	// Información de la API (público)
	api.Get("/info", func(c *fiber.Ctx) error {
		info := fiber.Map{
			"name":         "Engine API Workflow",
			"version":      "1.0.0",
			"environment":  config.Config.Environment,
			"api_version":  "v1",
			"documentation": "/docs", // Para Swagger futuro
		}

		// Solo mostrar features en desarrollo
		if config.Config.IsDevelopment() {
			info["features"] = fiber.Map{
				"jwt_blacklist": config.Config.EnableTokenBlacklist,
				"rate_limiting": config.Config.EnableRateLimit,
				"cors":          config.Config.EnableCORS,
			}
		}

		return c.JSON(info)
	})

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)

	// Status de la API (público pero con info limitada)
	api.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"api":         "operational",
			"timestamp":   time.Now().UTC(),
			"environment": config.Config.Environment,
		})
	})
}

// ✅ createAuthMiddleware crea middleware de autenticación con blacklist
func createAuthMiddleware(config *RouteConfig) *middleware.AuthMiddleware {
	if config.TokenBlacklist != nil {
		// Middleware con blacklist habilitado
		return middleware.NewAuthMiddlewareWithBlacklist(config.JWTService, config.TokenBlacklist, config.Logger)
	} else {
		// Middleware sin blacklist
		return middleware.NewAuthMiddleware(config.JWTService, config.Logger)
	}
}

// ✅ setupProtectedRoutes configura rutas que requieren autenticación
func setupProtectedRoutes(api fiber.Router, authHandler *handlers.AuthHandler, workflowHandler *handlers.WorkflowHandler, logHandler *handlers.LogHandler, authMiddleware *middleware.AuthMiddleware, config *RouteConfig) {
	// Rutas protegidas
	protected := api.Group("", authMiddleware.RequireAuth())

	// Rutas de autenticación protegidas
	auth := protected.Group("/auth")
	auth.Get("/profile", authHandler.GetProfile)
	auth.Post("/logout", authHandler.Logout)
	auth.Put("/profile", authHandler.UpdateProfile) // Si existe
	auth.Post("/change-password", authHandler.ChangePassword) // Si existe

	// ✅ Rutas de workflows completamente implementadas
	workflows := protected.Group("/workflows")
	workflows.Post("/", workflowHandler.CreateWorkflow)
	workflows.Get("/", workflowHandler.GetWorkflows)
	workflows.Get("/:id", workflowHandler.GetWorkflow)
	workflows.Put("/:id", workflowHandler.UpdateWorkflow)
	workflows.Delete("/:id", workflowHandler.DeleteWorkflow)
	workflows.Post("/:id/toggle", workflowHandler.ToggleWorkflowStatus)
	workflows.Get("/:id/stats", workflowHandler.GetWorkflowStats)
	workflows.Post("/:id/clone", workflowHandler.CloneWorkflow)
	workflows.Post("/:id/execute", workflowHandler.ExecuteWorkflow) // Si existe

	// ✅ Rutas de logs completamente implementadas
	logs := protected.Group("/logs")
	logs.Get("/", logHandler.GetLogs)
	logs.Get("/:id", logHandler.GetLogByID)
	logs.Get("/stats", logHandler.GetLogStats)
	logs.Delete("/cleanup", logHandler.DeleteOldLogs) // Solo admin

	// ✅ Rutas de triggers para ejecución de workflows
	triggers := protected.Group("/triggers")
	triggers.Post("/workflow", func(c *fiber.Ctx) error {
		// TODO: Implementar trigger de workflow manual
		return c.JSON(fiber.Map{
			"message": "Manual workflow trigger - Coming soon",
			"status":  "not_implemented",
		})
	})

	// ✅ Rutas de usuario para gestión de perfil
	user := protected.Group("/user")
	user.Get("/workflows", func(c *fiber.Ctx) error {
		// Obtener workflows del usuario actual
		return workflowHandler.GetUserWorkflows(c)
	})
	user.Get("/logs", func(c *fiber.Ctx) error {
		// Obtener logs del usuario actual
		return logHandler.GetUserLogs(c)
	})
	user.Get("/stats", func(c *fiber.Ctx) error {
		// Estadísticas del usuario actual
		return c.JSON(fiber.Map{
			"message": "User stats - Coming soon",
			"status":  "not_implemented",
		})
	})
}

// ✅ setupAdminRoutes configura rutas de administración
func setupAdminRoutes(api fiber.Router, authMiddleware *middleware.AuthMiddleware, config *RouteConfig) {
	// Rutas de administración (solo admins)
	admin := api.Group("/admin", authMiddleware.RequireAuth(), authMiddleware.RequireAdmin())

	// Gestión de usuarios
	users := admin.Group("/users")
	users.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin users listing - Coming soon",
			"status":  "not_implemented",
		})
	})
	users.Get("/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin user details - Coming soon",
			"status":  "not_implemented",
		})
	})
	users.Put("/:id/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin change user status - Coming soon",
			"status":  "not_implemented",
		})
	})

	// Estadísticas del sistema
	admin.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "System stats - Coming soon",
			"status":  "not_implemented",
		})
	})

	// Gestión de workflows globales
	admin.Get("/workflows", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin workflows listing - Coming soon",
			"status":  "not_implemented",
		})
	})

	// ✅ Gestión de blacklist de tokens (si está habilitado)
	if config.TokenBlacklist != nil {
		blacklist := admin.Group("/blacklist")
		blacklist.Get("/stats", func(c *fiber.Ctx) error {
			stats, err := config.TokenBlacklist.GetStats(c.Context())
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get blacklist stats", err.Error()))
			}
			return c.JSON(utils.SuccessResponse("Blacklist stats retrieved", stats))
		})
		
		blacklist.Post("/user/:user_id/revoke", func(c *fiber.Ctx) error {
			userID := c.Params("user_id")
			reason := c.Query("reason", "admin_revocation")
			
			err := config.TokenBlacklist.BlacklistAllUserTokens(c.Context(), userID, reason)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to revoke user tokens", err.Error()))
			}
			
			config.Logger.Info("Admin revoked all user tokens", "user_id", userID, "reason", reason)
			return c.JSON(utils.SuccessResponse("All user tokens revoked", nil))
		})
		
		blacklist.Delete("/cleanup", func(c *fiber.Ctx) error {
			deleted, err := config.TokenBlacklist.CleanupExpiredTokens(c.Context())
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to cleanup tokens", err.Error()))
			}
			return c.JSON(utils.SuccessResponse("Expired tokens cleaned up", fiber.Map{"deleted": deleted}))
		})
	}
}

// ✅ setupWebhookRoutes configura webhooks públicos
func setupWebhookRoutes(api fiber.Router, config *RouteConfig) {
	// Webhooks públicos (sin autenticación pero con validación)
	webhooks := api.Group("/webhooks")
	
	// Webhook genérico por ID
	webhooks.Post("/:id", func(c *fiber.Ctx) error {
		webhookID := c.Params("id")
		
		// Log del webhook recibido
		config.Logger.Info("Webhook received",
			"webhook_id", webhookID,
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
			"content_type", c.Get("Content-Type"),
		)
		
		// TODO: Implementar procesamiento de webhook
		return c.JSON(fiber.Map{
			"message":    "Webhook received successfully",
			"webhook_id": webhookID,
			"status":     "processed",
			"timestamp":  time.Now().UTC(),
		})
	})

	// Webhook de Slack específico
	webhooks.Post("/slack/:id", func(c *fiber.Ctx) error {
		webhookID := c.Params("id")
		
		// TODO: Validar signature de Slack
		// TODO: Procesar evento de Slack
		
		return c.JSON(fiber.Map{
			"message":    "Slack webhook processed",
			"webhook_id": webhookID,
			"status":     "processed",
		})
	})
}

// ✅ setupNotFoundHandler configura el handler para rutas no encontradas
func setupNotFoundHandler(app *fiber.App, config *RouteConfig) {
	app.Use("*", func(c *fiber.Ctx) error {
		config.Logger.Debug("Route not found",
			"path", c.Path(),
			"method", c.Method(),
			"ip", c.IP(),
		)

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success":   false,
			"message":   "Endpoint not found",
			"path":      c.Path(),
			"method":    c.Method(),
			"timestamp": time.Now().UTC(),
		})
	})
}

// ✅ generateRequestID genera un ID único para la request
func generateRequestID() string {
	// Implementación simple - en producción usar UUID
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// ✅ Mantener compatibilidad con función anterior (DEPRECATED)
func SetupRoutesComplete(app *fiber.App, db *mongo.Database, jwtService jwt.JWTService, appLogger *logger.Logger) {
	// Esta función se mantiene por compatibilidad pero está deprecated
	appLogger.Warn("SetupRoutesComplete is deprecated, use SetupSecureRoutes instead")
	
	// Crear configuración mínima para compatibilidad
	config := &RouteConfig{
		DB:         db,
		JWTService: jwtService,
		Logger:     appLogger,
		Config: &config.Config{
			Environment:          "development",
			EnableTokenBlacklist: false,
			EnableRateLimit:      false,
			EnableCORS:           true,
		},
	}
	
	SetupSecureRoutes(app, config)
}

// ✅ Helper function - falta el import fmt
import "fmt"