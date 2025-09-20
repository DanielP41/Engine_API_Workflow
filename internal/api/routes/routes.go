package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupNotificationRoutes configura las rutas para el sistema de notificaciones
func SetupNotificationRoutes(app *fiber.App, handler *handlers.NotificationHandler, authMiddleware fiber.Handler) {
	// Grupo principal de notificaciones
	notifications := app.Group("/api/v1/notifications")

	// Aplicar middleware de autenticación a todas las rutas
	notifications.Use(authMiddleware)

	// ================================
	// ENVÍO DE EMAILS
	// ================================

	// Enviar email directo
	notifications.Post("/send",
		middleware.RateLimit(10, 60), // 10 requests por minuto
		handler.SendEmail)

	// Enviar email con template
	notifications.Post("/send/template",
		middleware.RateLimit(10, 60),
		handler.SendTemplatedEmail)

	// ================================
	// GESTIÓN DE NOTIFICACIONES
	// ================================

	// Obtener lista de notificaciones
	notifications.Get("/", handler.GetNotifications)

	// Obtener notificación específica
	notifications.Get("/:id", handler.GetNotification)

	// Reintentar notificación fallida
	notifications.Post("/:id/retry", handler.RetryNotification)

	// Cancelar notificación
	notifications.Post("/:id/cancel", handler.CancelNotification)

	// ================================
	// GESTIÓN DE TEMPLATES
	// ================================

	templates := notifications.Group("/templates")

	// CRUD de templates
	templates.Post("/", handler.CreateTemplate)
	templates.Get("/", handler.GetTemplates)
	templates.Get("/:id", handler.GetTemplate)
	templates.Put("/:id", handler.UpdateTemplate)
	templates.Delete("/:id", handler.DeleteTemplate)

	// Vista previa de template
	templates.Post("/:name/preview", handler.PreviewTemplate)

	// ================================
	// ESTADÍSTICAS Y MONITOREO
	// ================================

	stats := notifications.Group("/stats")

	// Estadísticas de notificaciones
	stats.Get("/", handler.GetNotificationStats)

	// ================================
	// ADMINISTRACIÓN Y MANTENIMIENTO
	// ================================

	admin := notifications.Group("/admin")

	// Requiere permisos de administrador
	admin.Use(middleware.RequireRole("admin"))

	// Procesar notificaciones pendientes manualmente
	admin.Post("/process-pending", handler.ProcessPendingNotifications)

	// Reintentar notificaciones fallidas
	admin.Post("/retry-failed", handler.RetryFailedNotifications)

	// Limpiar notificaciones antiguas
	admin.Delete("/cleanup", handler.CleanupOldNotifications)

	// Probar configuración de email
	admin.Post("/test-config", handler.TestEmailConfiguration)

	// ================================
	// ENDPOINTS PÚBLICOS (SIN AUTH)
	// ================================

	// Endpoint para webhooks o sistemas externos
	webhooks := app.Group("/api/v1/webhooks/notifications")

	// Webhook para recibir eventos de proveedores de email
	// webhooks.Post("/delivery/:provider", handler.HandleDeliveryWebhook)

	// ================================
	// RUTAS DE SALUD Y ESTADO
	// ================================

	health := app.Group("/api/v1/health")

	// Estado del sistema de notificaciones
	health.Get("/notifications", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"service":   "email-notifications",
			"timestamp": fiber.Map{},
		})
	})
}

// SetupNotificationRoutesWithConfig configura las rutas con configuración personalizada
func SetupNotificationRoutesWithConfig(app *fiber.App, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	// Grupo principal
	notifications := app.Group(config.Prefix)

	if config.AuthMiddleware != nil {
		notifications.Use(config.AuthMiddleware)
	}

	// Rate limiting personalizado
	if config.RateLimit > 0 {
		notifications.Use(middleware.RateLimit(config.RateLimit, config.RateLimitWindow))
	}

	// Aplicar rutas según configuración
	if config.EnableSending {
		notifications.Post("/send", handler.SendEmail)
		notifications.Post("/send/template", handler.SendTemplatedEmail)
	}

	if config.EnableManagement {
		notifications.Get("/", handler.GetNotifications)
		notifications.Get("/:id", handler.GetNotification)
		notifications.Post("/:id/retry", handler.RetryNotification)
		notifications.Post("/:id/cancel", handler.CancelNotification)
	}

	if config.EnableTemplates {
		templates := notifications.Group("/templates")
		templates.Post("/", handler.CreateTemplate)
		templates.Get("/", handler.GetTemplates)
		templates.Get("/:id", handler.GetTemplate)
		templates.Put("/:id", handler.UpdateTemplate)
		templates.Delete("/:id", handler.DeleteTemplate)
		templates.Post("/:name/preview", handler.PreviewTemplate)
	}

	if config.EnableStats {
		stats := notifications.Group("/stats")
		stats.Get("/", handler.GetNotificationStats)
	}

	if config.EnableAdmin {
		admin := notifications.Group("/admin")
		if config.AdminMiddleware != nil {
			admin.Use(config.AdminMiddleware)
		}

		admin.Post("/process-pending", handler.ProcessPendingNotifications)
		admin.Post("/retry-failed", handler.RetryFailedNotifications)
		admin.Delete("/cleanup", handler.CleanupOldNotifications)
		admin.Post("/test-config", handler.TestEmailConfiguration)
	}
}

// NotificationRoutesConfig configuración para las rutas de notificaciones
type NotificationRoutesConfig struct {
	Prefix          string        `json:"prefix"`
	AuthMiddleware  fiber.Handler `json:"-"`
	AdminMiddleware fiber.Handler `json:"-"`

	// Límites de velocidad
	RateLimit       int `json:"rate_limit"`
	RateLimitWindow int `json:"rate_limit_window"` // en segundos

	// Características habilitadas
	EnableSending    bool `json:"enable_sending"`
	EnableManagement bool `json:"enable_management"`
	EnableTemplates  bool `json:"enable_templates"`
	EnableStats      bool `json:"enable_stats"`
	EnableAdmin      bool `json:"enable_admin"`
	EnableWebhooks   bool `json:"enable_webhooks"`
}

// DefaultNotificationRoutesConfig configuración por defecto
func DefaultNotificationRoutesConfig() NotificationRoutesConfig {
	return NotificationRoutesConfig{
		Prefix:           "/api/v1/notifications",
		RateLimit:        10,
		RateLimitWindow:  60,
		EnableSending:    true,
		EnableManagement: true,
		EnableTemplates:  true,
		EnableStats:      true,
		EnableAdmin:      true,
		EnableWebhooks:   false, // Deshabilitado por defecto por seguridad
	}
}

// ProductionNotificationRoutesConfig configuración para producción
func ProductionNotificationRoutesConfig() NotificationRoutesConfig {
	return NotificationRoutesConfig{
		Prefix:           "/api/v1/notifications",
		RateLimit:        5, // Más restrictivo en producción
		RateLimitWindow:  60,
		EnableSending:    true,
		EnableManagement: true,
		EnableTemplates:  true,
		EnableStats:      true,
		EnableAdmin:      false, // Admin endpoints separados en producción
		EnableWebhooks:   true,
	}
}

// DevelopmentNotificationRoutesConfig configuración para desarrollo
func DevelopmentNotificationRoutesConfig() NotificationRoutesConfig {
	return NotificationRoutesConfig{
		Prefix:           "/api/v1/notifications",
		RateLimit:        50, // Más permisivo en desarrollo
		RateLimitWindow:  60,
		EnableSending:    true,
		EnableManagement: true,
		EnableTemplates:  true,
		EnableStats:      true,
		EnableAdmin:      true,
		EnableWebhooks:   true,
	}
}
