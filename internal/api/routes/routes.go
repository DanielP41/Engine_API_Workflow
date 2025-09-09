package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/internal/worker"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"go.mongodb.org/mongo-driver/mongo"
)

// RouteConfig estructura compatible con main.go - ACTUALIZADA con Workers
type RouteConfig struct {
	DB             *mongo.Database
	JWTService     jwt.JWTService
	TokenBlacklist *jwt.TokenBlacklist
	Logger         *logger.Logger
	Config         *config.Config
	// Dependencias para workers
	WorkerEngine *worker.WorkerEngine
	QueueRepo    repository.QueueRepository
	QueueService *services.QueueService
}

// SetupSecureRoutes función principal que main.go espera - ACTUALIZADA
func SetupSecureRoutes(app *fiber.App, config *RouteConfig) {
	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(config.DB)
	workflowRepo := mongodb.NewWorkflowRepository(config.DB)
	logRepo := mongodb.NewLogRepository(config.DB)

	// Inicializar servicios
	authService := services.NewAuthService(userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)

	// Crear validador
	validator := utils.NewValidator()

	// Inicializar handlers principales
	authHandler := handlers.NewAuthHandler(authService, config.JWTService, validator)
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService, validator)
	triggerHandler := handlers.NewTriggerHandler(workflowRepo, logRepo, config.QueueService, config.Logger.Zap())
	logHandler := handlers.NewLogHandler(logRepo, config.Logger.Zap())

	// ✅ Inicializar handler de workers (si está disponible)
	var workerHandler *handlers.WorkerHandler
	if config.WorkerEngine != nil && config.QueueRepo != nil {
		workerHandler = handlers.NewWorkerHandler(config.WorkerEngine, config.QueueRepo, config.Logger.Zap())
	}

	// Configurar middlewares de seguridad
	setupSecurityMiddlewares(app, config)

	// Configurar rutas principales
	setupMainRoutes(app, config, authHandler, workflowHandler, triggerHandler, logHandler, workerHandler)

	config.Logger.Info("Secure routes configured successfully")
}

// setupSecurityMiddlewares configura middlewares de seguridad
func setupSecurityMiddlewares(app *fiber.App, config *RouteConfig) {
	// CORS (si está habilitado)
	if config.Config.EnableCORS {
		origins := config.Config.GetCORSOrigins()
		originString := "http://localhost:3000,http://localhost:3001"
		if len(origins) > 0 {
			originString = origins[0]
			for i := 1; i < len(origins); i++ {
				originString += "," + origins[i]
			}
		}

		corsConfig := cors.Config{
			AllowOrigins:     originString,
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With,X-Request-ID",
			AllowCredentials: true,
			MaxAge:           86400,
		}
		app.Use(cors.New(corsConfig))
		config.Logger.Info("CORS middleware enabled")
	}

	// Rate Limiting (si está habilitado)
	if config.Config.EnableRateLimit {
		rateLimitConfig := limiter.Config{
			Max:        config.Config.RateLimitRequests,
			Expiration: config.Config.RateLimitWindow,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"success":     false,
					"message":     "Too many requests",
					"retry_after": config.Config.RateLimitWindow.Seconds(),
				})
			},
		}
		app.Use(limiter.New(rateLimitConfig))
		config.Logger.Info("Rate limiting middleware enabled")
	}

	// Security headers
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if config.Config.IsProduction() {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	})
}

// setupMainRoutes configura las rutas principales de la API - ACTUALIZADA con Workers
func setupMainRoutes(
	app *fiber.App,
	config *RouteConfig,
	authHandler *handlers.AuthHandler,
	workflowHandler *handlers.WorkflowHandler,
	triggerHandler *handlers.TriggerHandler,
	logHandler *handlers.LogHandler,
	workerHandler *handlers.WorkerHandler,
) {
	// Grupo principal de la API
	api := app.Group("/api/v1")

	// Health check interno (mejorado)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(utils.SuccessResponse("Engine API Workflow is running", fiber.Map{
			"status":  "ok",
			"version": "1.0.0",
			"workers": config.WorkerEngine != nil,
		}))
	})

	// ✅ Health check público específico para workers
	if workerHandler != nil {
		api.Get("/workers/health", workerHandler.HealthCheck)
		api.Get("/workers/stats", workerHandler.GetWorkerStats)
	}

	// ===== RUTAS PÚBLICAS =====

	// Rutas de autenticación (públicas)
	setupAuthRoutes(api, authHandler, config)

	// Webhooks públicos (sin autenticación para triggers externos)
	setupWebhookRoutes(api, triggerHandler)

	// ===== RUTAS PROTEGIDAS =====

	// Crear instancia del middleware de autenticación
	authMiddlewareInstance := middleware.NewAuthMiddlewareWithBlacklist(config.JWTService, config.TokenBlacklist, config.Logger)

	// Grupo de rutas protegidas
	protected := api.Group("", authMiddlewareInstance.RequireAuth())

	// Configurar rutas protegidas
	setupProtectedRoutes(protected, config, authHandler, workflowHandler, triggerHandler, logHandler, workerHandler)

	// Configurar rutas de administración
	setupAdminRoutes(protected, config, workerHandler, logHandler)

	// 404 handler
	app.Use("*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse(
			"Endpoint not found",
			"The requested endpoint does not exist",
		))
	})

	config.Logger.Info("Routes configured successfully")
}

// setupAuthRoutes configura las rutas de autenticación
func setupAuthRoutes(api fiber.Router, authHandler *handlers.AuthHandler, config *RouteConfig) {
	auth := api.Group("/auth")

	// Rutas públicas de autenticación
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)

	// Rutas protegidas de autenticación (configuradas en setupProtectedRoutes)
}

// setupWebhookRoutes configura las rutas de webhooks (públicas)
func setupWebhookRoutes(api fiber.Router, triggerHandler *handlers.TriggerHandler) {
	webhooks := api.Group("/webhooks")

	// Webhook triggers (públicos - sin autenticación)
	if triggerHandler != nil {
		webhooks.Post("/:webhook_id", triggerHandler.TriggerWebhook)
	} else {
		webhooks.Post("/:webhook_id", func(c *fiber.Ctx) error {
			return c.JSON(utils.ErrorResponse("Webhook handler not available", ""))
		})
	}

	// Health check específico para webhooks
	webhooks.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "webhooks",
			"message": "Webhook service is running",
		})
	})
}

// setupProtectedRoutes configura las rutas que requieren autenticación
func setupProtectedRoutes(
	protected fiber.Router,
	config *RouteConfig,
	authHandler *handlers.AuthHandler,
	workflowHandler *handlers.WorkflowHandler,
	triggerHandler *handlers.TriggerHandler,
	logHandler *handlers.LogHandler,
	workerHandler *handlers.WorkerHandler,
) {
	// Rutas de autenticación protegidas
	protected.Get("/auth/profile", authHandler.GetProfile)
	protected.Post("/auth/logout", func(c *fiber.Ctx) error {
		return c.JSON(utils.SuccessResponse("Logout successful", fiber.Map{
			"message": "Please remove the access and refresh tokens from your client",
		}))
	})

	// ===== RUTAS DE WORKFLOWS =====
	if workflowHandler != nil {
		workflows := protected.Group("/workflows")
		workflows.Post("/", workflowHandler.CreateWorkflow)
		workflows.Get("/", workflowHandler.GetWorkflows)
		workflows.Get("/:id", workflowHandler.GetWorkflow)
		workflows.Put("/:id", workflowHandler.UpdateWorkflow)
		workflows.Delete("/:id", workflowHandler.DeleteWorkflow)
		workflows.Post("/:id/toggle", workflowHandler.ToggleWorkflowStatus)
		workflows.Get("/:id/stats", workflowHandler.GetWorkflowStats)
		workflows.Post("/:id/clone", workflowHandler.CloneWorkflow)
	} else {
		// Placeholder para workflows
		workflows := protected.Group("/workflows")
		workflows.Get("/", func(c *fiber.Ctx) error {
			return c.JSON(utils.SuccessResponse("Workflows endpoint", fiber.Map{
				"message": "Workflow handler not available",
				"status":  "placeholder",
			}))
		})
	}

	// ===== RUTAS DE TRIGGERS =====
	if triggerHandler != nil {
		triggers := protected.Group("/triggers")
		triggers.Post("/workflow", triggerHandler.TriggerWorkflow)
		triggers.Post("/scheduled/:workflow_id", triggerHandler.TriggerScheduled)
		triggers.Get("/status/:log_id", triggerHandler.GetTriggerStatus)
		triggers.Post("/cancel/:log_id", triggerHandler.CancelTrigger)
	} else {
		// Placeholder para triggers
		triggers := protected.Group("/triggers")
		triggers.Post("/", func(c *fiber.Ctx) error {
			return c.JSON(utils.SuccessResponse("Triggers endpoint", fiber.Map{
				"message": "Trigger handler not available",
				"status":  "placeholder",
			}))
		})
	}

	// ===== RUTAS DE LOGS =====
	if logHandler != nil {
		logs := protected.Group("/logs")
		logs.Get("/", logHandler.GetLogs)
		logs.Get("/:id", logHandler.GetLogByID)
		logs.Get("/stats", logHandler.GetLogStats)
	} else {
		// Placeholder para logs
		logs := protected.Group("/logs")
		logs.Get("/", func(c *fiber.Ctx) error {
			return c.JSON(utils.SuccessResponse("Logs endpoint", fiber.Map{
				"message": "Log handler not available",
				"status":  "placeholder",
			}))
		})
	}

	// ===== RUTAS DE WORKERS (NUEVAS) =====
	if workerHandler != nil {
		workers := protected.Group("/workers")
		workers.Get("/stats", workerHandler.GetWorkerStats)
		workers.Get("/health", workerHandler.HealthCheck)
		workers.Get("/queue/stats", workerHandler.GetQueueStats)
		workers.Get("/processing", workerHandler.GetProcessingTasks)
		workers.Get("/failed", workerHandler.GetFailedTasks)
		workers.Post("/retry/:task_id", workerHandler.RetryFailedTask)
	} else {
		// Placeholder para workers
		workers := protected.Group("/workers")
		workers.Get("/stats", func(c *fiber.Ctx) error {
			return c.JSON(utils.SuccessResponse("Worker stats", fiber.Map{
				"message": "Worker engine not available",
				"status":  "disabled",
			}))
		})
	}
}

// setupAdminRoutes configura las rutas de administración
func setupAdminRoutes(
	protected fiber.Router,
	config *RouteConfig,
	workerHandler *handlers.WorkerHandler,
	logHandler *handlers.LogHandler,
) {
	// Middleware para verificar permisos de admin
	adminMiddleware := middleware.AdminRequiredMiddleware()

	admin := protected.Group("/admin", adminMiddleware)

	// Estadísticas del sistema
	admin.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(utils.SuccessResponse("System statistics", fiber.Map{
			"message": "System stats endpoint - Coming soon",
			"workers": config.WorkerEngine != nil,
		}))
	})

	// Gestión de usuarios (solo admins)
	users := admin.Group("/users")
	users.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(utils.SuccessResponse("Users list", fiber.Map{
			"message": "Users management endpoint - Coming soon",
		}))
	})

	// ===== GESTIÓN DE WORKERS (SOLO ADMINS) =====
	if workerHandler != nil {
		// Gestión de colas (solo admins)
		queue := admin.Group("/queue")
		queue.Post("/clear", workerHandler.ClearQueue)
		queue.Get("/failed", workerHandler.GetFailedTasks)

		// Gestión avanzada de workers
		workerAdmin := admin.Group("/workers")
		workerAdmin.Get("/stats", workerHandler.GetWorkerStats)
		workerAdmin.Get("/processing", workerHandler.GetProcessingTasks)
		workerAdmin.Post("/queue/clear", workerHandler.ClearQueue)
	}

	// ===== GESTIÓN DE LOGS (SOLO ADMINS) =====
	if logHandler != nil {
		// Limpieza de logs antiguos (solo admins)
		admin.Delete("/logs/cleanup", logHandler.DeleteOldLogs)
	}

	// Debug endpoints (solo en desarrollo)
	if config.Config.IsDevelopment() {
		debug := admin.Group("/debug")
		debug.Get("/config", func(c *fiber.Ctx) error {
			return c.JSON(utils.SuccessResponse("Debug config", fiber.Map{
				"environment": config.Config.Environment,
				"workers":     config.WorkerEngine != nil,
				"features": fiber.Map{
					"blacklist":  config.Config.EnableTokenBlacklist,
					"rate_limit": config.Config.EnableRateLimit,
					"cors":       config.Config.EnableCORS,
					"workers":    config.WorkerEngine != nil,
				},
			}))
		})
	}
}
