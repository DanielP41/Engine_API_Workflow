package middleware

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/utils"
	jwtPkg "Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

// AuthMiddleware estructura para el middleware de autenticación
type AuthMiddleware struct {
	jwtService     jwtPkg.JWTService
	tokenBlacklist *jwtPkg.TokenBlacklist // Puede ser nil si está deshabilitado
	logger         *logger.Logger
	config         AuthConfig
}

// AuthConfig configuración para el middleware de autenticación
type AuthConfig struct {
	// SkipPaths rutas que se saltan la autenticación (útil para health checks)
	SkipPaths []string
	// ExtractTokenFunc función personalizada para extraer token (opcional)
	ExtractTokenFunc func(*fiber.Ctx) (string, error)
	// OnAuthSuccess callback ejecutado cuando la autenticación es exitosa
	OnAuthSuccess func(*fiber.Ctx, *jwtPkg.Claims)
	// OnAuthFailure callback ejecutado cuando la autenticación falla
	OnAuthFailure func(*fiber.Ctx, error)
}

// NewAuthMiddleware crea middleware sin blacklist (para compatibilidad)
func NewAuthMiddleware(jwtService jwtPkg.JWTService, logger *logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:     jwtService,
		tokenBlacklist: nil,
		logger:         logger,
		config:         AuthConfig{},
	}
}

// NewAuthMiddlewareWithBlacklist crea middleware con blacklist habilitado
func NewAuthMiddlewareWithBlacklist(jwtService jwtPkg.JWTService, tokenBlacklist *jwtPkg.TokenBlacklist, logger *logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:     jwtService,
		tokenBlacklist: tokenBlacklist,
		logger:         logger,
		config:         AuthConfig{},
	}
}

// NewAuthMiddlewareWithConfig crea middleware con configuración personalizada
func NewAuthMiddlewareWithConfig(jwtService jwtPkg.JWTService, tokenBlacklist *jwtPkg.TokenBlacklist, logger *logger.Logger, config AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:     jwtService,
		tokenBlacklist: tokenBlacklist,
		logger:         logger,
		config:         config,
	}
}

// RequireAuth middleware que requiere autenticación
func (m *AuthMiddleware) RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Verificar si la ruta debe ser saltada
		if m.shouldSkipPath(c.Path()) {
			return c.Next()
		}

		// Extraer token usando función personalizada o por defecto
		token, err := m.extractToken(c)
		if err != nil {
			m.logAuthFailure(c, "token_extraction_failed", err)
			if m.config.OnAuthFailure != nil {
				m.config.OnAuthFailure(c, err)
			}
			return utils.UnauthorizedResponse(c, "Authorization header required")
		}

		// Validar token con JWT service
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			m.logAuthFailure(c, "token_validation_failed", err)
			if m.config.OnAuthFailure != nil {
				m.config.OnAuthFailure(c, err)
			}

			// Proporcionar mensajes de error más específicos
			return m.handleTokenValidationError(c, err)
		}

		// Verificar que sea un access token
		if claims.Type != "access" {
			err := fiber.NewError(fiber.StatusUnauthorized, "Invalid token type")
			m.logAuthFailure(c, "invalid_token_type", err)
			if m.config.OnAuthFailure != nil {
				m.config.OnAuthFailure(c, err)
			}
			return utils.UnauthorizedResponse(c, "Access token required")
		}

		// Verificar blacklist si está habilitado
		if m.tokenBlacklist != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if m.tokenBlacklist.IsBlacklisted(ctx, token) {
				err := fiber.NewError(fiber.StatusUnauthorized, "Token has been revoked")
				m.logAuthFailure(c, "token_blacklisted", err)
				if m.config.OnAuthFailure != nil {
					m.config.OnAuthFailure(c, err)
				}
				return utils.UnauthorizedResponse(c, "Token has been revoked")
			}
		}

		// Convertir user ID a ObjectID para compatibilidad
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			err := fiber.NewError(fiber.StatusUnauthorized, "Invalid user ID format in token")
			m.logAuthFailure(c, "invalid_user_id", err)
			if m.config.OnAuthFailure != nil {
				m.config.OnAuthFailure(c, err)
			}
			return utils.UnauthorizedResponse(c, "Invalid user ID in token")
		}

		// Almacenar información del usuario en el contexto con múltiples formatos para compatibilidad
		m.setUserContext(c, claims, userID)

		// Log de autenticación exitosa
		m.logAuthSuccess(c, claims)

		// Callback de éxito si está configurado
		if m.config.OnAuthSuccess != nil {
			m.config.OnAuthSuccess(c, claims)
		}

		return c.Next()
	}
}

// RequireAdmin middleware que requiere permisos de administrador
func (m *AuthMiddleware) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			m.logger.Debug("Admin access denied - no user role in context",
				"path", c.Path(),
				"ip", c.IP())
			return utils.UnauthorizedResponse(c, "Authentication required")
		}

		role, ok := userRole.(string)
		if !ok || role != string(models.RoleAdmin) {
			m.logger.Warn("Admin access denied",
				"user_role", role,
				"path", c.Path(),
				"ip", c.IP(),
				"user_id", c.Locals("userID"))
			return utils.ForbiddenResponse(c, "Admin access required")
		}

		m.logger.Debug("Admin access granted",
			"user_id", c.Locals("userID"),
			"path", c.Path())

		return c.Next()
	}
}

// RequireRole middleware que requiere un rol específico
func (m *AuthMiddleware) RequireRole(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			return utils.UnauthorizedResponse(c, "Authentication required")
		}

		role, ok := userRole.(string)
		if !ok {
			return utils.UnauthorizedResponse(c, "Invalid user role")
		}

		// Verificar si el rol del usuario está en la lista de roles permitidos
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				return c.Next()
			}
		}

		m.logger.Warn("Role access denied",
			"user_role", role,
			"allowed_roles", allowedRoles,
			"path", c.Path(),
			"user_id", c.Locals("userID"))

		return utils.ForbiddenResponse(c, "Insufficient permissions")
	}
}

// OptionalAuth middleware de autenticación opcional
func (m *AuthMiddleware) OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Intentar extraer token
		token, err := m.extractToken(c)
		if err != nil {
			// Si no hay token o es inválido, continuar sin autenticación
			return c.Next()
		}

		// Intentar validar token
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			// Si la validación falla, continuar sin autenticación
			m.logger.Debug("Optional auth failed", "error", err.Error())
			return c.Next()
		}

		// Verificar que sea access token
		if claims.Type != "access" {
			return c.Next()
		}

		// Verificar blacklist si está habilitado
		if m.tokenBlacklist != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			if m.tokenBlacklist.IsBlacklisted(ctx, token) {
				return c.Next()
			}
		}

		// Convertir user ID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			return c.Next()
		}

		// Almacenar información del usuario en el contexto
		m.setUserContext(c, claims, userID)

		return c.Next()
	}
}

// RequireValidRefreshToken middleware para validar refresh tokens
func (m *AuthMiddleware) RequireValidRefreshToken() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token, err := m.extractToken(c)
		if err != nil {
			return utils.UnauthorizedResponse(c, "Authorization header required")
		}

		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			return m.handleTokenValidationError(c, err)
		}

		// Verificar que sea un refresh token
		if claims.Type != "refresh" {
			return utils.UnauthorizedResponse(c, "Refresh token required")
		}

		// Verificar blacklist
		if m.tokenBlacklist != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if m.tokenBlacklist.IsBlacklisted(ctx, token) {
				return utils.UnauthorizedResponse(c, "Refresh token has been revoked")
			}
		}

		// Almacenar claims en contexto
		c.Locals("refresh_claims", claims)
		c.Locals("refresh_token", token)

		return c.Next()
	}
}

// extractToken extrae el token del header Authorization
func (m *AuthMiddleware) extractToken(c *fiber.Ctx) (string, error) {
	// Usar función personalizada si está configurada
	if m.config.ExtractTokenFunc != nil {
		return m.config.ExtractTokenFunc(c)
	}

	// Función por defecto
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Authorization header required")
	}

	// Verificar formato Bearer
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	if token == "" {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Token is required")
	}

	return token, nil
}

// setUserContext almacena información del usuario en el contexto de Fiber
func (m *AuthMiddleware) setUserContext(c *fiber.Ctx, claims *jwtPkg.Claims, userID primitive.ObjectID) {
	// Múltiples formatos para compatibilidad con código existente
	c.Locals("userID", claims.UserID)                             // String
	c.Locals("user_id", userID)                                   // ObjectID
	c.Locals("userEmail", claims.Email)                           // Email
	c.Locals("user_email", claims.Email)                          // Email alternativo
	c.Locals("userRole", claims.Role)                             // Role string
	c.Locals("user_role", claims.Role)                            // Role alternativo
	c.Locals("tokenType", claims.Type)                            // Token type
	c.Locals("token_type", claims.Type)                           // Token type alternativo
	c.Locals("is_admin", claims.Role == string(models.RoleAdmin)) // Boolean admin check
	c.Locals("jwt_claims", claims)                                // Claims completos
	c.Locals("authenticated", true)                               // Flag de autenticación
}

// shouldSkipPath verifica si una ruta debe saltarse la autenticación
func (m *AuthMiddleware) shouldSkipPath(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if path == skipPath || strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// handleTokenValidationError maneja errores específicos de validación de tokens - CORREGIDO para JWT v5
func (m *AuthMiddleware) handleTokenValidationError(c *fiber.Ctx, err error) error {
	// Usar el manejo de errores moderno de JWT v5
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return utils.UnauthorizedResponse(c, "Token has expired")
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return utils.UnauthorizedResponse(c, "Token not valid yet")
	case errors.Is(err, jwt.ErrTokenMalformed):
		return utils.UnauthorizedResponse(c, "Malformed token")
	case errors.Is(err, jwt.ErrSignatureInvalid):
		return utils.UnauthorizedResponse(c, "Invalid token signature")
	default:
		return utils.UnauthorizedResponse(c, "Invalid token")
	}
}

// logAuthSuccess registra autenticación exitosa
func (m *AuthMiddleware) logAuthSuccess(c *fiber.Ctx, claims *jwtPkg.Claims) {
	m.logger.Debug("Authentication successful",
		"user_id", claims.UserID,
		"user_email", claims.Email,
		"user_role", claims.Role,
		"path", c.Path(),
		"method", c.Method(),
		"ip", c.IP(),
		"user_agent", c.Get("User-Agent"),
	)
}

// logAuthFailure registra fallo de autenticación
func (m *AuthMiddleware) logAuthFailure(c *fiber.Ctx, reason string, err error) {
	m.logger.Warn("Authentication failed",
		"reason", reason,
		"error", err.Error(),
		"path", c.Path(),
		"method", c.Method(),
		"ip", c.IP(),
		"user_agent", c.Get("User-Agent"),
		"auth_header_present", c.Get("Authorization") != "",
	)
}

// GetCurrentUser extrae información del usuario actual del contexto
func GetCurrentUser(c *fiber.Ctx) (*UserContext, error) {
	userID := c.Locals("userID")
	if userID == nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "No authenticated user")
	}

	userRole := c.Locals("userRole")
	userEmail := c.Locals("userEmail")
	isAdmin := c.Locals("is_admin")

	user := &UserContext{
		ID:    userID.(string),
		Role:  userRole.(string),
		Email: userEmail.(string),
	}

	if admin, ok := isAdmin.(bool); ok {
		user.IsAdmin = admin
	}

	return user, nil
}

// GetCurrentUserID extrae solo el ID del usuario del contexto
func GetCurrentUserID(c *fiber.Ctx) (primitive.ObjectID, error) {
	userID := c.Locals("user_id")
	if userID == nil {
		return primitive.NilObjectID, fiber.NewError(fiber.StatusUnauthorized, "No authenticated user")
	}

	if objID, ok := userID.(primitive.ObjectID); ok {
		return objID, nil
	}

	return primitive.NilObjectID, fiber.NewError(fiber.StatusInternalServerError, "Invalid user ID format")
}

// IsAuthenticated verifica si la request está autenticada
func IsAuthenticated(c *fiber.Ctx) bool {
	authenticated := c.Locals("authenticated")
	if auth, ok := authenticated.(bool); ok {
		return auth
	}
	return false
}

// IsAdmin verifica si el usuario actual es administrador
func IsAdmin(c *fiber.Ctx) bool {
	isAdmin := c.Locals("is_admin")
	if admin, ok := isAdmin.(bool); ok {
		return admin
	}
	return false
}

// UserContext estructura que contiene información del usuario autenticado
type UserContext struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	IsAdmin bool   `json:"is_admin"`
}

// CreateLogoutHandler crea un handler para logout que usa blacklist - CORREGIDO
func (m *AuthMiddleware) CreateLogoutHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Solo proceder si hay blacklist habilitado
		if m.tokenBlacklist == nil {
			return utils.SuccessResponse(c, fiber.StatusOK, "Logged out successfully", fiber.Map{
				"message": "Token blacklisting disabled - please remove token from client",
			})
		}

		// Extraer token
		token, err := m.extractToken(c)
		if err != nil {
			// Si no hay token, considerar logout exitoso
			return utils.SuccessResponse(c, fiber.StatusOK, "Logged out successfully", nil)
		}

		// Agregar token a blacklist
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.tokenBlacklist.BlacklistTokenWithReason(ctx, token, "user_logout"); err != nil {
			m.logger.Error("Failed to blacklist token during logout", "error", err)
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Logout failed", "Could not revoke token")
		}

		m.logger.Info("User logged out successfully",
			"user_id", c.Locals("userID"),
			"ip", c.IP())

		return utils.SuccessResponse(c, fiber.StatusOK, "Logged out successfully", nil)
	}
}

// AdminRequiredMiddleware función standalone para requerir admin
func AdminRequiredMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			return utils.UnauthorizedResponse(c, "Authentication required")
		}

		role, ok := userRole.(string)
		if !ok || role != string(models.RoleAdmin) {
			return utils.ForbiddenResponse(c, "Admin access required")
		}

		return c.Next()
	}
}

// GetCurrentUserRole extrae el rol del usuario del contexto
func GetCurrentUserRole(c *fiber.Ctx) (string, error) {
	userRole := c.Locals("userRole")
	if userRole == nil {
		return "", errors.New("no user role in context")
	}

	if role, ok := userRole.(string); ok {
		return role, nil
	}

	return "", errors.New("invalid user role format")
}
