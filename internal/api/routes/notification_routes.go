package routes

import (
	"fmt"
	"time"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/timeout"
)

// ================================
// CONFIGURACIÓN DE RUTAS
// ================================

// NotificationRoutesConfig configuración para las rutas de notificaciones
type NotificationRoutesConfig struct {
	// Configuración básica
	Prefix          string        `json:"prefix"`
	AuthMiddleware  fiber.Handler `json:"-"`
	AdminMiddleware fiber.Handler `json:"-"`

	// Límites de rate limiting
	RateLimit       int           `json:"rate_limit"`        // requests por ventana
	RateLimitWindow time.Duration `json:"rate_limit_window"` // ventana de tiempo
	BurstLimit      int           `json:"burst_limit"`       // pico permitido

	// Timeouts
	RequestTimeout time.Duration `json:"request_timeout"`
	UploadTimeout  time.Duration `json:"upload_timeout"`

	// Habilitación de funcionalidades
	EnableSending    bool `json:"enable_sending"`
	EnableManagement bool `json:"enable_management"`
	EnableTemplates  bool `json:"enable_templates"`
	EnableStats      bool `json:"enable_stats"`
	EnableAdmin      bool `json:"enable_admin"`
	EnableWebhooks   bool `json:"enable_webhooks"`
	EnableHealth     bool `json:"enable_health"`

	// Configuración CORS
	CORSConfig *CORSConfig `json:"cors_config"`

	// Configuración de validación
	MaxRequestSize  int64 `json:"max_request_size"`
	MaxEmailsPerReq int   `json:"max_emails_per_request"`
	MaxTemplateSize int64 `json:"max_template_size"`

	// Headers de seguridad
	SecurityHeaders bool `json:"security_headers"`
}

// CORSConfig configuración de CORS específica
type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

// ================================
// CONFIGURACIONES PREDEFINIDAS
// ================================

// DefaultNotificationRoutesConfig configuración por defecto
func DefaultNotificationRoutesConfig() NotificationRoutesConfig {
	return NotificationRoutesConfig{
		Prefix:           "/api/v1/notifications",
		RateLimit:        60, // 60 requests por minuto
		RateLimitWindow:  time.Minute,
		BurstLimit:       10, // Permitir picos de hasta 10 requests
		RequestTimeout:   30 * time.Second,
		UploadTimeout:    60 * time.Second,
		EnableSending:    true,
		EnableManagement: true,
		EnableTemplates:  true,
		EnableStats:      true,
		EnableAdmin:      true,
		EnableWebhooks:   false, // Deshabilitado por defecto
		EnableHealth:     true,
		MaxRequestSize:   10 * 1024 * 1024, // 10MB
		MaxEmailsPerReq:  100,              // Máximo 100 emails por request
		MaxTemplateSize:  1024 * 1024,      // 1MB para templates
		SecurityHeaders:  true,
		CORSConfig: &CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization", "Accept"},
			AllowCredentials: true,
			MaxAge:           3600,
		},
	}
}

// ProductionNotificationRoutesConfig configuración para producción
func ProductionNotificationRoutesConfig() NotificationRoutesConfig {
	config := DefaultNotificationRoutesConfig()

	// Configuraciones más restrictivas para producción
	config.RateLimit = 30       // Menos requests por minuto
	config.BurstLimit = 5       // Menos picos permitidos
	config.MaxEmailsPerReq = 50 // Menos emails por request
	config.SecurityHeaders = true
	config.EnableWebhooks = true // Habilitar webhooks en producción

	// CORS más restrictivo
	config.CORSConfig.AllowedOrigins = []string{
		"https://yourdomain.com",
		"https://app.yourdomain.com",
	}

	return config
}

// DevelopmentNotificationRoutesConfig configuración para desarrollo
func DevelopmentNotificationRoutesConfig() NotificationRoutesConfig {
	config := DefaultNotificationRoutesConfig()

	// Configuraciones más permisivas para desarrollo
	config.RateLimit = 120         // Más requests por minuto
	config.BurstLimit = 20         // Más picos permitidos
	config.MaxEmailsPerReq = 200   // Más emails por request
	config.SecurityHeaders = false // Headers de seguridad opcionales

	// CORS permisivo para desarrollo
	config.CORSConfig.AllowedOrigins = []string{"*"}

	return config
}

// TestingNotificationRoutesConfig configuración para testing
func TestingNotificationRoutesConfig() NotificationRoutesConfig {
	config := DefaultNotificationRoutesConfig()

	// Sin límites para testing
	config.RateLimit = 1000
	config.BurstLimit = 100
	config.MaxEmailsPerReq = 1000
	config.RequestTimeout = 5 * time.Second
	config.SecurityHeaders = false

	return config
}

// ================================
// CONFIGURACIÓN PRINCIPAL DE RUTAS
// ================================

// SetupNotificationRoutes configura las rutas básicas con configuración por defecto
func SetupNotificationRoutes(app *fiber.App, handler *handlers.NotificationHandler, authMiddleware fiber.Handler) {
	config := DefaultNotificationRoutesConfig()
	config.AuthMiddleware = authMiddleware
	SetupNotificationRoutesWithConfig(app, handler, config)
}

// SetupNotificationRoutesWithConfig configura las rutas con configuración personalizada
func SetupNotificationRoutesWithConfig(app *fiber.App, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	// ================================
	// MIDDLEWARE GLOBAL PARA NOTIFICACIONES
	// ================================

	// Grupo principal
	notifications := app.Group(config.Prefix)

	// CORS si está habilitado
	if config.CORSConfig != nil {
		notifications.Use(cors.New(cors.Config{
			AllowOrigins:     joinStrings(config.CORSConfig.AllowedOrigins),
			AllowMethods:     joinStrings(config.CORSConfig.AllowedMethods),
			AllowHeaders:     joinStrings(config.CORSConfig.AllowedHeaders),
			AllowCredentials: config.CORSConfig.AllowCredentials,
			MaxAge:           config.CORSConfig.MaxAge,
		}))
	}

	// Headers de seguridad
	if config.SecurityHeaders {
		notifications.Use(func(c *fiber.Ctx) error {
			c.Set("X-Content-Type-Options", "nosniff")
			c.Set("X-Frame-Options", "DENY")
			c.Set("X-XSS-Protection", "1; mode=block")
			c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			return c.Next()
		})
	}

	// Rate limiting global
	if config.RateLimit > 0 {
		notifications.Use(middleware.NewRateLimiter(middleware.RateLimiterConfig{
			Max:      config.RateLimit,
			Duration: config.RateLimitWindow,
			KeyFunc:  middleware.DefaultKeyFunc,
		}))
	}

	// Timeout global
	if config.RequestTimeout > 0 {
		notifications.Use(timeout.New(handler.HandleTimeout, config.RequestTimeout))
	}

	// Middleware de autenticación para rutas protegidas
	protected := notifications.Group("/")
	if config.AuthMiddleware != nil {
		protected.Use(config.AuthMiddleware)
	}

	// ================================
	// RUTAS DE ENVÍO DE EMAILS
	// ================================

	if config.EnableSending {
		setupSendingRoutes(protected, handler, config)
	}

	// ================================
	// RUTAS DE GESTIÓN DE NOTIFICACIONES
	// ================================

	if config.EnableManagement {
		setupManagementRoutes(protected, handler, config)
	}

	// ================================
	// RUTAS DE GESTIÓN DE TEMPLATES
	// ================================

	if config.EnableTemplates {
		setupTemplateRoutes(protected, handler, config)
	}

	// ================================
	// RUTAS DE ESTADÍSTICAS
	// ================================

	if config.EnableStats {
		setupStatsRoutes(protected, handler, config)
	}

	// ================================
	// RUTAS DE ADMINISTRACIÓN
	// ================================

	if config.EnableAdmin {
		setupAdminRoutes(protected, handler, config)
	}

	// ================================
	// RUTAS DE SALUD Y ESTADO
	// ================================

	if config.EnableHealth {
		setupHealthRoutes(app, handler, config)
	}

	// ================================
	// WEBHOOKS (SIN AUTENTICACIÓN)
	// ================================

	if config.EnableWebhooks {
		setupWebhookRoutes(app, handler, config)
	}
}

// ================================
// CONFIGURACIÓN DE GRUPOS DE RUTAS
// ================================

// setupSendingRoutes configura las rutas de envío de emails
func setupSendingRoutes(group fiber.Router, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	// Rate limiting específico para envío (más restrictivo)
	sendingLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		Max:      config.RateLimit / 2, // Mitad del límite general
		Duration: config.RateLimitWindow,
		KeyFunc:  middleware.DefaultKeyFunc,
	})

	// Validación de tamaño de request
	sizeValidator := middleware.NewRequestSizeValidator(config.MaxRequestSize)

	// Envío general
	group.Post("/send",
		sendingLimiter,
		sizeValidator,
		middleware.ValidateContentType("application/json"),
		handler.SendEmail)

	// Envío simple
	group.Post("/send/simple",
		sendingLimiter,
		sizeValidator,
		middleware.ValidateContentType("application/json"),
		handler.SendSimpleEmail)

	// Envío con template
	group.Post("/send/template",
		sendingLimiter,
		sizeValidator,
		middleware.ValidateContentType("application/json"),
		handler.SendTemplatedEmail)

	// Envío de bienvenida
	group.Post("/send/welcome",
		sendingLimiter,
		sizeValidator,
		middleware.ValidateContentType("application/json"),
		handler.SendWelcomeEmail)

	// Envío de alertas del sistema
	group.Post("/send/alert",
		sendingLimiter,
		sizeValidator,
		middleware.ValidateContentType("application/json"),
		middleware.RequireRole("admin"),
		handler.SendSystemAlert)
}

// setupManagementRoutes configura las rutas de gestión de notificaciones
func setupManagementRoutes(group fiber.Router, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	// Listar notificaciones con cache
	group.Get("/",
		middleware.Cache(5*time.Minute), // Cache por 5 minutos
		handler.GetNotifications)

	// Obtener notificación específica
	group.Get("/:id",
		middleware.ValidateObjectID("id"),
		middleware.Cache(2*time.Minute),
		handler.GetNotification)

	// Cancelar notificación
	group.Post("/:id/cancel",
		middleware.ValidateObjectID("id"),
		middleware.ValidateContentType("application/json"),
		handler.CancelNotification)

	// Reenviar notificación
	group.Post("/:id/resend",
		middleware.ValidateObjectID("id"),
		middleware.ValidateContentType("application/json"),
		middleware.RequirePermission("notifications:resend"),
		handler.ResendNotification)
}

// setupTemplateRoutes configura las rutas de gestión de templates
func setupTemplateRoutes(group fiber.Router, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	templates := group.Group("/templates")

	// Validador de tamaño para templates
	templateSizeValidator := middleware.NewRequestSizeValidator(config.MaxTemplateSize)

	// Crear template
	templates.Post("/",
		templateSizeValidator,
		middleware.ValidateContentType("application/json"),
		middleware.RequirePermission("templates:create"),
		handler.CreateTemplate)

	// Listar templates
	templates.Get("/",
		middleware.Cache(10*time.Minute), // Cache más largo para templates
		handler.GetTemplates)

	// Obtener template específico
	templates.Get("/:id",
		middleware.ValidateObjectID("id"),
		middleware.Cache(10*time.Minute),
		handler.GetTemplate)

	// Actualizar template
	templates.Put("/:id",
		middleware.ValidateObjectID("id"),
		templateSizeValidator,
		middleware.ValidateContentType("application/json"),
		middleware.RequirePermission("templates:update"),
		handler.UpdateTemplate)

	// Eliminar template
	templates.Delete("/:id",
		middleware.ValidateObjectID("id"),
		middleware.RequirePermission("templates:delete"),
		handler.DeleteTemplate)

	// Vista previa de template
	templates.Post("/:name/preview",
		templateSizeValidator,
		middleware.ValidateContentType("application/json"),
		handler.PreviewTemplate)
}

// setupStatsRoutes configura las rutas de estadísticas
func setupStatsRoutes(group fiber.Router, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	stats := group.Group("/stats")

	// Estadísticas de notificaciones
	stats.Get("/",
		middleware.Cache(2*time.Minute), // Cache corto para stats
		handler.GetNotificationStats)

	// Estadísticas del servicio
	stats.Get("/service",
		middleware.Cache(30*time.Second), // Cache muy corto para stats en tiempo real
		handler.GetServiceStats)

	// Estadísticas por usuario (solo para admins o el propio usuario)
	stats.Get("/user/:user_id",
		middleware.ValidateObjectID("user_id"),
		middleware.RequireOwnershipOrRole("admin"),
		middleware.Cache(5*time.Minute),
		handler.GetUserNotificationStats)

	// Estadísticas por template
	stats.Get("/template/:template_name",
		middleware.Cache(10*time.Minute),
		handler.GetTemplateStats)
}

// setupAdminRoutes configura las rutas de administración
func setupAdminRoutes(group fiber.Router, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	admin := group.Group("/admin")

	// Middleware de administrador
	if config.AdminMiddleware != nil {
		admin.Use(config.AdminMiddleware)
	} else {
		admin.Use(middleware.RequireRole("admin"))
	}

	// Rate limiting más restrictivo para operaciones de admin
	adminLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		Max:      config.RateLimit / 4, // Cuarto del límite general
		Duration: config.RateLimitWindow,
		KeyFunc:  middleware.DefaultKeyFunc,
	})

	admin.Use(adminLimiter)

	// Procesar notificaciones pendientes
	admin.Post("/process-pending",
		middleware.ValidateContentType("application/json"),
		handler.ProcessPendingNotifications)

	// Reintentar notificaciones fallidas
	admin.Post("/retry-failed",
		middleware.ValidateContentType("application/json"),
		handler.RetryFailedNotifications)

	// Limpiar notificaciones antiguas
	admin.Delete("/cleanup",
		middleware.ValidateContentType("application/json"),
		handler.CleanupOldNotifications)

	// Probar configuración de email
	admin.Post("/test-config",
		middleware.ValidateContentType("application/json"),
		handler.TestEmailConfiguration)

	// Crear templates por defecto
	admin.Post("/default-templates",
		middleware.ValidateContentType("application/json"),
		handler.CreateDefaultTemplates)

	// Estadísticas avanzadas de administrador
	admin.Get("/stats/advanced",
		middleware.Cache(1*time.Minute),
		handler.GetAdvancedStats)

	// Logs del sistema de notificaciones
	admin.Get("/logs",
		middleware.RequirePermission("logs:read"),
		handler.GetSystemLogs)

	// Configuración del sistema
	admin.Get("/config",
		middleware.RequirePermission("config:read"),
		handler.GetSystemConfig)

	admin.Put("/config",
		middleware.ValidateContentType("application/json"),
		middleware.RequirePermission("config:write"),
		handler.UpdateSystemConfig)
}

// setupHealthRoutes configura las rutas de salud (sin autenticación)
func setupHealthRoutes(app *fiber.App, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	health := app.Group("/api/v1/health")

	// Health check básico
	health.Get("/notifications", handler.GetHealthStatus)

	// Health check detallado (con autenticación)
	health.Get("/notifications/detailed",
		config.AuthMiddleware,
		middleware.RequireRole("admin"),
		handler.GetDetailedHealthStatus)

	// Métricas para monitoring (Prometheus, etc.)
	health.Get("/notifications/metrics",
		middleware.RequirePermission("metrics:read"),
		handler.GetMetrics)
}

// setupWebhookRoutes configura las rutas de webhooks (sin autenticación)
func setupWebhookRoutes(app *fiber.App, handler *handlers.NotificationHandler, config NotificationRoutesConfig) {
	webhooks := app.Group("/api/v1/webhooks/notifications")

	// Rate limiting para webhooks
	webhookLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		Max:      config.RateLimit * 2, // Más permisivo para webhooks
		Duration: config.RateLimitWindow,
		KeyFunc:  middleware.DefaultKeyFunc,
	})

	webhooks.Use(webhookLimiter)

	// Webhook de entrega de emails (para proveedores como SendGrid, Mailgun)
	webhooks.Post("/delivery/:provider",
		middleware.ValidateWebhookSignature(),
		middleware.ValidateContentType("application/json"),
		handler.HandleDeliveryWebhook)

	// Webhook de rebotes
	webhooks.Post("/bounce/:provider",
		middleware.ValidateWebhookSignature(),
		middleware.ValidateContentType("application/json"),
		handler.HandleBounceWebhook)

	// Webhook de quejas
	webhooks.Post("/complaint/:provider",
		middleware.ValidateWebhookSignature(),
		middleware.ValidateContentType("application/json"),
		handler.HandleComplaintWebhook)
}

// ================================
// MÉTODOS AUXILIARES ESPECIALIZADOS
// ================================

// SetupDevelopmentRoutes configura rutas específicas para desarrollo
func SetupDevelopmentRoutes(app *fiber.App, handler *handlers.NotificationHandler, authMiddleware fiber.Handler) {
	config := DevelopmentNotificationRoutesConfig()
	config.AuthMiddleware = authMiddleware

	// Rutas adicionales para desarrollo
	dev := app.Group("/api/v1/dev/notifications")

	if authMiddleware != nil {
		dev.Use(authMiddleware)
	}

	// Simular envío de emails (sin enviar realmente)
	dev.Post("/simulate",
		middleware.ValidateContentType("application/json"),
		handler.SimulateEmail)

	// Limpiar todos los datos de testing
	dev.Delete("/cleanup-test-data",
		middleware.RequireRole("admin"),
		handler.CleanupTestData)

	// Generar datos de prueba
	dev.Post("/generate-test-data",
		middleware.ValidateContentType("application/json"),
		middleware.RequireRole("admin"),
		handler.GenerateTestData)

	// Configurar rutas principales
	SetupNotificationRoutesWithConfig(app, handler, config)
}

// SetupProductionRoutes configura rutas específicas para producción
func SetupProductionRoutes(app *fiber.App, handler *handlers.NotificationHandler, authMiddleware fiber.Handler, adminMiddleware fiber.Handler) {
	config := ProductionNotificationRoutesConfig()
	config.AuthMiddleware = authMiddleware
	config.AdminMiddleware = adminMiddleware

	// Configurar rutas principales
	SetupNotificationRoutesWithConfig(app, handler, config)
}

// ================================
// FUNCIONES AUXILIARES
// ================================

// joinStrings une un slice de strings con comas
func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += "," + strs[i]
	}
	return result
}

// ValidateRouteConfig valida la configuración de rutas
func ValidateRouteConfig(config NotificationRoutesConfig) error {
	if config.Prefix == "" {
		return fmt.Errorf("prefix cannot be empty")
	}

	if config.RateLimit < 0 {
		return fmt.Errorf("rate_limit must be non-negative")
	}

	if config.RateLimitWindow <= 0 {
		config.RateLimitWindow = time.Minute
	}

	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}

	if config.MaxRequestSize <= 0 {
		config.MaxRequestSize = 10 * 1024 * 1024 // 10MB
	}

	if config.MaxEmailsPerReq <= 0 {
		config.MaxEmailsPerReq = 100
	}

	return nil
}

// GetRouteInfo devuelve información sobre las rutas configuradas
func GetRouteInfo(config NotificationRoutesConfig) map[string]interface{} {
	return map[string]interface{}{
		"prefix":            config.Prefix,
		"rate_limit":        config.RateLimit,
		"rate_limit_window": config.RateLimitWindow.String(),
		"request_timeout":   config.RequestTimeout.String(),
		"features": map[string]bool{
			"sending":    config.EnableSending,
			"management": config.EnableManagement,
			"templates":  config.EnableTemplates,
			"stats":      config.EnableStats,
			"admin":      config.EnableAdmin,
			"webhooks":   config.EnableWebhooks,
			"health":     config.EnableHealth,
		},
		"limits": map[string]interface{}{
			"max_request_size":   config.MaxRequestSize,
			"max_emails_per_req": config.MaxEmailsPerReq,
			"max_template_size":  config.MaxTemplateSize,
		},
	}
}
