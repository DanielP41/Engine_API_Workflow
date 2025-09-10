// internal/api/routes/routes.go - VERSIÓN ACTUALIZADA
package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/pkg/jwt"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// RouteConfig configuración para las rutas - ACTUALIZADA
type RouteConfig struct {
	AuthHandler      *handlers.AuthHandler
	WorkflowHandler  *handlers.WorkflowHandler
	LogHandler       *handlers.LogHandler
	TriggerHandler   *handlers.TriggerHandler
	DashboardHandler *handlers.DashboardHandler // 🆕 NUEVO
	JWTService       jwt.JWTService
	Logger           *zap.Logger
}

// SetupRoutes configura todas las rutas de la aplicación - ACTUALIZADA
func SetupRoutes(app *fiber.App, config *RouteConfig) {
	// Middleware global
	app.Use(middleware.CORSMiddleware(middleware.DefaultCORSConfig()))
	app.Use(middleware.SecurityHeadersMiddleware())
	app.Use(middleware.APIResponseHeadersMiddleware())

	// Rutas para servir archivos estáticos del dashboard 🆕
	app.Static("/static", "./web/static")

	// Ruta principal del dashboard (HTML) 🆕
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/dashboard.html")
	})

	// Redirect /dashboard to /
	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.Redirect("/")
	})

	// API v1 routes
	api := app.Group("/api/v1")

	// Health check (público)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"message":   "Engine API Workflow is running",
			"timestamp": time.Now(),
			"version":   "1.0.0",
		})
	})

	// Rutas de autenticación (públicas)
	setupAuthRoutes(api, config)

	// Middleware de autenticación para rutas protegidas
	authMiddleware := middleware.AuthMiddleware(config.JWTService, config.Logger)
	protected := api.Group("", authMiddleware)

	// Rutas protegidas de autenticación
	setupProtectedAuthRoutes(protected, config)

	// Rutas de workflows
	setupWorkflowRoutes(protected, config)

	// Rutas de logs
	setupLogRoutes(protected, config)

	// Rutas de triggers
	setupTriggerRoutes(protected, config)

	// 🆕 Rutas del dashboard (NUEVAS)
	setupDashboardRoutes(protected, config)

	// Rutas de administración (solo admins)
	setupAdminRoutes(protected, config)

	// Manejo de errores 404
	setupNotFoundHandler(app)
}

// setupAuthRoutes configura las rutas públicas de autenticación
func setupAuthRoutes(api fiber.Router, config *RouteConfig) {
	auth := api.Group("/auth")

	auth.Post("/register", config.AuthHandler.Register)
	auth.Post("/login", config.AuthHandler.Login)
	auth.Post("/refresh", config.AuthHandler.RefreshToken)
}

// setupProtectedAuthRoutes configura las rutas protegidas de autenticación
func setupProtectedAuthRoutes(protected fiber.Router, config *RouteConfig) {
	auth := protected.Group("/auth")

	auth.Get("/profile", config.AuthHandler.GetProfile)
	auth.Post("/logout", config.AuthHandler.Logout)
}

// setupWorkflowRoutes configura las rutas de workflows
func setupWorkflowRoutes(protected fiber.Router, config *RouteConfig) {
	workflows := protected.Group("/workflows")

	workflows.Post("/", config.WorkflowHandler.CreateWorkflow)
	workflows.Get("/", config.WorkflowHandler.GetWorkflows)
	workflows.Get("/:id", config.WorkflowHandler.GetWorkflow)
	workflows.Put("/:id", config.WorkflowHandler.UpdateWorkflow)
	workflows.Delete("/:id", config.WorkflowHandler.DeleteWorkflow)
	workflows.Post("/:id/toggle", config.WorkflowHandler.ToggleWorkflowStatus)
	workflows.Get("/:id/stats", config.WorkflowHandler.GetWorkflowStats)
	workflows.Post("/:id/clone", config.WorkflowHandler.CloneWorkflow)
}

// setupLogRoutes configura las rutas de logs
func setupLogRoutes(protected fiber.Router, config *RouteConfig) {
	if config.LogHandler == nil {
		return // Handler opcional
	}

	logs := protected.Group("/logs")

	logs.Get("/", config.LogHandler.GetLogs)
	logs.Get("/:id", config.LogHandler.GetLogByID)
	logs.Get("/stats", config.LogHandler.GetLogStats)
	logs.Delete("/cleanup", config.LogHandler.DeleteOldLogs)
}

// setupTriggerRoutes configura las rutas de triggers
func setupTriggerRoutes(protected fiber.Router, config *RouteConfig) {
	if config.TriggerHandler == nil {
		return // Handler opcional
	}

	triggers := protected.Group("/triggers")

	triggers.Post("/workflow", config.TriggerHandler.TriggerWorkflow)
	triggers.Post("/scheduled/:workflow_id", config.TriggerHandler.TriggerScheduled)
	triggers.Get("/status/:log_id", config.TriggerHandler.GetTriggerStatus)
	triggers.Post("/cancel/:log_id", config.TriggerHandler.CancelTrigger)

	// Rutas públicas para webhooks (sin autenticación)
	api := protected.(*fiber.Group).Group.(*fiber.App).Group("/api/v1")
	api.Post("/triggers/webhook/:webhook_id", config.TriggerHandler.TriggerWebhook)
}

// 🆕 setupDashboardRoutes configura las rutas del dashboard (NUEVAS)
func setupDashboardRoutes(protected fiber.Router, config *RouteConfig) {
	if config.DashboardHandler == nil {
		config.Logger.Warn("Dashboard handler not provided, skipping dashboard routes")
		return
	}

	dashboard := protected.Group("/dashboard")

	// Rutas principales del dashboard
	dashboard.Get("/", config.DashboardHandler.GetDashboard)
	dashboard.Get("/summary", config.DashboardHandler.GetDashboardSummary)

	// Rutas de componentes específicos
	dashboard.Get("/health", config.DashboardHandler.GetSystemHealth)
	dashboard.Get("/stats", config.DashboardHandler.GetQuickStats)
	dashboard.Get("/activity", config.DashboardHandler.GetRecentActivity)
	dashboard.Get("/workflows", config.DashboardHandler.GetWorkflowStatus)
	dashboard.Get("/queue", config.DashboardHandler.GetQueueStatus)
	dashboard.Get("/performance", config.DashboardHandler.GetPerformanceData)
	dashboard.Get("/alerts", config.DashboardHandler.GetActiveAlerts)
	dashboard.Get("/metrics", config.DashboardHandler.GetMetrics)

	// Rutas específicas de workflows
	dashboard.Get("/workflows/:id/health", config.DashboardHandler.GetWorkflowHealth)

	// Rutas administrativas del dashboard (solo admins)
	adminMiddleware := middleware.AdminRequiredMiddleware()
	dashboard.Post("/refresh", adminMiddleware, config.DashboardHandler.RefreshDashboard)

	config.Logger.Info("Dashboard routes configured successfully")
}

// setupAdminRoutes configura las rutas de administración
func setupAdminRoutes(protected fiber.Router, config *RouteConfig) {
	adminMiddleware := middleware.AdminRequiredMiddleware()
	admin := protected.Group("/admin", adminMiddleware)

	// Rutas de administración de usuarios
	admin.Get("/users", func(c *fiber.Ctx) error {
		// TODO: Implementar handler de usuarios para admins
		return c.JSON(fiber.Map{
			"message": "Admin users endpoint - Coming soon",
		})
	})

	// Estadísticas del sistema para admins
	admin.Get("/stats", func(c *fiber.Ctx) error {
		// TODO: Implementar estadísticas del sistema
		return c.JSON(fiber.Map{
			"message": "Admin stats endpoint - Coming soon",
		})
	})

	// Configuración del sistema
	admin.Get("/config", func(c *fiber.Ctx) error {
		// TODO: Implementar configuración del sistema
		return c.JSON(fiber.Map{
			"message": "Admin config endpoint - Coming soon",
		})
	})

	// Logs del sistema
	admin.Get("/system-logs", func(c *fiber.Ctx) error {
		// TODO: Implementar logs del sistema
		return c.JSON(fiber.Map{
			"message": "Admin system logs endpoint - Coming soon",
		})
	})
}

// setupNotFoundHandler configura el manejo de rutas no encontradas
func setupNotFoundHandler(app *fiber.App) {
	app.Use("*", func(c *fiber.Ctx) error {
		// Para requests de API, retornar JSON
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "API endpoint not found",
				"path":    c.Path(),
			})
		}

		// Para otras rutas, redirigir al dashboard
		return c.Redirect("/")
	})
}

// ValidateRouteConfig valida la configuración de rutas
func ValidateRouteConfig(config *RouteConfig) error {
	if config == nil {
		return fmt.Errorf("route config cannot be nil")
	}

	if config.AuthHandler == nil {
		return fmt.Errorf("auth handler is required")
	}

	if config.JWTService == nil {
		return fmt.Errorf("JWT service is required")
	}

	if config.Logger == nil {
		return fmt.Errorf("logger is required")
	}

	// Handlers opcionales - solo advertir si no están presentes
	if config.WorkflowHandler == nil {
		config.Logger.Warn("Workflow handler not provided")
	}

	if config.DashboardHandler == nil {
		config.Logger.Warn("Dashboard handler not provided")
	}

	if config.LogHandler == nil {
		config.Logger.Warn("Log handler not provided")
	}

	if config.TriggerHandler == nil {
		config.Logger.Warn("Trigger handler not provided")
	}

	return nil
}

// GetRouteInfo retorna información sobre las rutas configuradas
func GetRouteInfo() map[string]interface{} {
	return map[string]interface{}{
		"version": "1.0.0",
		"routes": map[string]interface{}{
			"public": []string{
				"GET /",
				"GET /dashboard",
				"GET /static/*",
				"GET /api/v1/health",
				"POST /api/v1/auth/register",
				"POST /api/v1/auth/login",
				"POST /api/v1/auth/refresh",
				"POST /api/v1/triggers/webhook/:webhook_id",
			},
			"protected": []string{
				"GET /api/v1/auth/profile",
				"POST /api/v1/auth/logout",
				"CRUD /api/v1/workflows",
				"GET /api/v1/logs",
				"POST /api/v1/triggers",
				"GET /api/v1/dashboard/*",
			},
			"admin": []string{
				"POST /api/v1/dashboard/refresh",
				"GET /api/v1/admin/*",
			},
		},
		"dashboard_endpoints": []string{
			"GET /api/v1/dashboard/",
			"GET /api/v1/dashboard/summary",
			"GET /api/v1/dashboard/health",
			"GET /api/v1/dashboard/stats",
			"GET /api/v1/dashboard/activity",
			"GET /api/v1/dashboard/workflows",
			"GET /api/v1/dashboard/queue",
			"GET /api/v1/dashboard/performance",
			"GET /api/v1/dashboard/alerts",
			"GET /api/v1/dashboard/metrics",
			"GET /api/v1/dashboard/workflows/:id/health",
			"POST /api/v1/dashboard/refresh",
		},
	}
}

// SetupMiddlewares configura middlewares globales adicionales
func SetupMiddlewares(app *fiber.App, config *RouteConfig) {
	// Rate limiting básico (implementación simple)
	app.Use(func(c *fiber.Ctx) error {
		// TODO: Implementar rate limiting real
		return c.Next()
	})

	// Request ID para tracing
	app.Use(func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("X-Request-ID", requestID)
		c.Locals("request_id", requestID)
		return c.Next()
	})

	// Logging de requests para el dashboard
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()

		// Procesar request
		err := c.Next()

		// Log después del request
		duration := time.Since(start)
		config.Logger.Info("HTTP Request",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", duration),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
			zap.String("request_id", c.Locals("request_id").(string)),
		)

		return err
	})
}

// generateRequestID genera un ID único para el request
func generateRequestID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(6))
}

// randomString genera una cadena aleatoria de la longitud especificada
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// SetupHealthChecks configura health checks avanzados
func SetupHealthChecks(app *fiber.App, config *RouteConfig) {
	health := app.Group("/health")

	// Health check básico
	health.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now(),
			"uptime":    time.Since(startTime).String(),
		})
	})

	// Health check detallado
	health.Get("/detailed", func(c *fiber.Ctx) error {
		healthStatus := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now(),
			"uptime":    time.Since(startTime).String(),
			"version":   "1.0.0",
			"services": map[string]string{
				"api":       "healthy",
				"database":  "healthy", // TODO: verificar conexión real
				"redis":     "healthy", // TODO: verificar conexión real
				"dashboard": "healthy",
			},
		}

		return c.JSON(healthStatus)
	})

	// Ready check para Kubernetes
	health.Get("/ready", func(c *fiber.Ctx) error {
		// TODO: Verificar que todos los servicios estén listos
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// Live check para Kubernetes
	health.Get("/live", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "alive"})
	})
}

// Variables globales para health checks
var startTime = time.Now()

// Imports adicionales necesarios
