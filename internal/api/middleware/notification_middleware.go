package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// RateLimiter estructura para rate limiting
type RateLimiter struct {
	requests map[string][]time.Time
	mutex    sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter crea un nuevo rate limiter
func NewRateLimiter(limit int, windowSeconds int) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   time.Duration(windowSeconds) * time.Second,
	}
}

// IsAllowed verifica si una IP está permitida
func (rl *RateLimiter) IsAllowed(ip string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Obtener requests para esta IP
	requests, exists := rl.requests[ip]
	if !exists {
		rl.requests[ip] = []time.Time{now}
		return true
	}

	// Filtrar requests dentro de la ventana de tiempo
	validRequests := []time.Time{}
	for _, reqTime := range requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Verificar límite
	if len(validRequests) >= rl.limit {
		rl.requests[ip] = validRequests
		return false
	}

	// Agregar nueva request
	validRequests = append(validRequests, now)
	rl.requests[ip] = validRequests
	return true
}

// Cleanup limpia requests antiguas
func (rl *RateLimiter) Cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	for ip, requests := range rl.requests {
		validRequests := []time.Time{}
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				validRequests = append(validRequests, reqTime)
			}
		}

		if len(validRequests) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = validRequests
		}
	}
}

// Global rate limiters
var (
	rateLimiters = make(map[string]*RateLimiter)
	limiterMutex sync.RWMutex
)

// RateLimit middleware para limitación de velocidad
func RateLimit(limit int, windowSeconds int) fiber.Handler {
	// Reemplazar fiber.Utils.UnsafeString() con fmt.Sprintf()
	limiterKey := fmt.Sprintf("limiter_%d_%d", limit, windowSeconds)

	limiterMutex.Lock()
	limiter, exists := rateLimiters[limiterKey]
	if !exists {
		limiter = NewRateLimiter(limit, windowSeconds)
		rateLimiters[limiterKey] = limiter

		// Iniciar cleanup automático
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				limiter.Cleanup()
			}
		}()
	}
	limiterMutex.Unlock()

	return func(c *fiber.Ctx) error {
		ip := c.IP()

		if !limiter.IsAllowed(ip) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":   "Too many requests",
				"message": "Rate limit exceeded. Please try again later.",
				"limit":   limit,
				"window":  windowSeconds,
			})
		}

		return c.Next()
	}
}

// RequireRole middleware para verificar roles de usuario
func RequireRole(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("user_role")
		if userRole == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "Unauthorized",
				"message": "User role not found",
			})
		}

		role, ok := userRole.(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "Unauthorized",
				"message": "Invalid user role format",
			})
		}

		// Verificar si el usuario tiene el rol requerido
		if !hasRequiredRole(role, requiredRole) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":         "Forbidden",
				"message":       "Insufficient permissions",
				"required_role": requiredRole,
				"user_role":     role,
			})
		}

		return c.Next()
	}
}

// hasRequiredRole verifica si el usuario tiene el rol requerido
func hasRequiredRole(userRole, requiredRole string) bool {
	// Jerarquía de roles: admin > user
	roleHierarchy := map[string]int{
		"admin": 2,
		"user":  1,
	}

	userLevel, userExists := roleHierarchy[userRole]
	requiredLevel, requiredExists := roleHierarchy[requiredRole]

	if !userExists || !requiredExists {
		return false
	}

	return userLevel >= requiredLevel
}

// RequirePermission middleware para verificar permisos específicos
func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userPermissions := c.Locals("user_permissions")
		if userPermissions == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Forbidden",
				"message": "No permissions found",
			})
		}

		permissions, ok := userPermissions.([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Forbidden",
				"message": "Invalid permissions format",
			})
		}

		// Verificar si el usuario tiene el permiso requerido
		for _, perm := range permissions {
			if perm == permission {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":               "Forbidden",
			"message":             "Insufficient permissions",
			"required_permission": permission,
		})
	}
}

// LogNotificationActivity middleware para logging de actividad
func LogNotificationActivity(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Procesar request
		err := c.Next()

		// Log de la actividad
		duration := time.Since(start)

		fields := []zap.Field{
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", duration),
		}

		// Agregar información del usuario si está disponible
		if userID := c.Locals("user_id"); userID != nil {
			fields = append(fields, zap.Any("user_id", userID))
		}

		if userName := c.Locals("user_name"); userName != nil {
			fields = append(fields, zap.String("user_name", userName.(string)))
		}

		// Agregar parámetros de la ruta si existen
		if c.Params("id") != "" {
			fields = append(fields, zap.String("resource_id", c.Params("id")))
		}

		if c.Params("name") != "" {
			fields = append(fields, zap.String("resource_name", c.Params("name")))
		}

		// Log según el nivel del resultado
		if err != nil {
			logger.Error("Notification API request failed", append(fields, zap.Error(err))...)
		} else if c.Response().StatusCode() >= 400 {
			logger.Warn("Notification API request completed with error", fields...)
		} else {
			logger.Info("Notification API request completed", fields...)
		}

		return err
	}
}

// ValidateContentType middleware para validar content-type
func ValidateContentType(contentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "PATCH" {
			if c.Get("Content-Type") != contentType {
				return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{
					"error":   "Unsupported Media Type",
					"message": "Content-Type must be " + contentType,
				})
			}
		}
		return c.Next()
	}
}

// RequireJSONContent middleware para requerir JSON
func RequireJSONContent() fiber.Handler {
	return ValidateContentType("application/json")
}

// AddSecurityHeaders middleware para agregar headers de seguridad
func AddSecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Agregar headers de seguridad
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// CORS headers si no se configuraron en otro lugar
		if c.Get("Access-Control-Allow-Origin") == "" {
			c.Set("Access-Control-Allow-Origin", "*")
			c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		return c.Next()
	}
}

// RequestSizeLimit middleware para limitar el tamaño de requests
func RequestSizeLimit(maxSizeBytes int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if len(c.Body()) > maxSizeBytes {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error":    "Request Entity Too Large",
				"message":  "Request body exceeds maximum allowed size",
				"max_size": maxSizeBytes,
			})
		}
		return c.Next()
	}
}

// NotificationRateLimitConfig configuración específica para notificaciones
type NotificationRateLimitConfig struct {
	EmailSending RateLimitRule `json:"email_sending"`
	TemplateOps  RateLimitRule `json:"template_ops"`
	Management   RateLimitRule `json:"management"`
	AdminOps     RateLimitRule `json:"admin_ops"`
}

// RateLimitRule regla de rate limiting
type RateLimitRule struct {
	Limit  int `json:"limit"`
	Window int `json:"window"` // segundos
}

// DefaultNotificationRateLimits límites por defecto para notificaciones
func DefaultNotificationRateLimits() NotificationRateLimitConfig {
	return NotificationRateLimitConfig{
		EmailSending: RateLimitRule{Limit: 10, Window: 60}, // 10 emails por minuto
		TemplateOps:  RateLimitRule{Limit: 20, Window: 60}, // 20 operaciones de template por minuto
		Management:   RateLimitRule{Limit: 50, Window: 60}, // 50 consultas por minuto
		AdminOps:     RateLimitRule{Limit: 5, Window: 300}, // 5 operaciones admin por 5 minutos
	}
}

// ProductionNotificationRateLimits límites para producción
func ProductionNotificationRateLimits() NotificationRateLimitConfig {
	return NotificationRateLimitConfig{
		EmailSending: RateLimitRule{Limit: 5, Window: 60}, // Más restrictivo
		TemplateOps:  RateLimitRule{Limit: 10, Window: 60},
		Management:   RateLimitRule{Limit: 30, Window: 60},
		AdminOps:     RateLimitRule{Limit: 3, Window: 600}, // Muy restrictivo para admin
	}
}

// CreateNotificationRateLimiters crea middlewares de rate limiting para notificaciones
func CreateNotificationRateLimiters(config NotificationRateLimitConfig) map[string]fiber.Handler {
	return map[string]fiber.Handler{
		"email_sending": RateLimit(config.EmailSending.Limit, config.EmailSending.Window),
		"template_ops":  RateLimit(config.TemplateOps.Limit, config.TemplateOps.Window),
		"management":    RateLimit(config.Management.Limit, config.Management.Window),
		"admin_ops":     RateLimit(config.AdminOps.Limit, config.AdminOps.Window),
	}
}
