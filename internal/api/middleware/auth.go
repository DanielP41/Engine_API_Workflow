package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/utils"
	jwtPkg "Engine_API_Workflow/pkg/jwt"
)

// AuthMiddleware estructura para el middleware de autenticación - CORREGIDO
type AuthMiddleware struct {
	jwtService jwtPkg.JWTService // Usar la interfaz correcta
	logger     *zap.Logger
}

// NewAuthMiddleware crea una nueva instancia del middleware de autenticación - CORREGIDO
func NewAuthMiddleware(jwtService jwtPkg.JWTService) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		logger:     zap.NewNop(), // Logger por defecto, puede ser configurado
	}
}

// RequireAuth middleware que requiere autenticación - CORREGIDO
func (m *AuthMiddleware) RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			if m.logger != nil {
				m.logger.Debug("Missing authorization header")
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Authorization header required", "")
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			if m.logger != nil {
				m.logger.Debug("Invalid authorization header format")
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid authorization header format", "Use 'Bearer <token>'")
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			if m.logger != nil {
				m.logger.Debug("Empty token")
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Token is required", "")
		}

		// Validate token
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			if m.logger != nil {
				m.logger.Debug("Invalid token", zap.Error(err))
			}

			// Check if token is expired
			if ve, ok := err.(*jwt.ValidationError); ok {
				if ve.Errors&jwt.ValidationErrorExpired != 0 {
					return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Token expired", "Please refresh your token")
				}
			}

			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid token", "")
		}

		// Convert user ID string to ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("Invalid user ID in token", zap.Error(err), zap.String("user_id", claims.UserID))
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid token", "")
		}

		// Store user info in context
		c.Locals("userID", userID)
		c.Locals("userRole", claims.Role)
		c.Locals("tokenType", claims.Type)
		c.Locals("user_id", userID) // Alias para compatibilidad
		c.Locals("is_admin", claims.Role == models.RoleAdmin)

		return c.Next()
	}
}

// RequireAdmin middleware que requiere rol de admin - CORREGIDO
func (m *AuthMiddleware) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Authentication required", "")
		}

		role, ok := userRole.(string)
		if !ok || role != string(models.RoleAdmin) {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Admin access required", "")
		}

		return c.Next()
	}
}

// OptionalAuth middleware que permite autenticación opcional - CORREGIDO
func (m *AuthMiddleware) OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			// No auth header is fine for optional auth
			return c.Next()
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			// Invalid format but optional, continue without auth
			return c.Next()
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			// Empty token but optional, continue without auth
			return c.Next()
		}

		// Validate token
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			// Invalid token but optional, continue without auth
			if m.logger != nil {
				m.logger.Debug("Invalid optional token", zap.Error(err))
			}
			return c.Next()
		}

		// Convert user ID string to ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			if m.logger != nil {
				m.logger.Debug("Invalid user ID in optional token", zap.Error(err))
			}
			return c.Next()
		}

		// Store user info in context
		c.Locals("userID", userID)
		c.Locals("userRole", claims.Role)
		c.Locals("tokenType", claims.Type)
		c.Locals("user_id", userID) // Alias para compatibilidad
		c.Locals("is_admin", claims.Role == string(models.RoleAdmin))

		return c.Next()
	}
}

// RoleRequired middleware que requiere roles específicos - CORREGIDO
func (m *AuthMiddleware) RoleRequired(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Authentication required", "")
		}

		role, ok := userRole.(string)
		if !ok {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid user role", "")
		}

		// Check if user role is in allowed roles
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				return c.Next()
			}
		}

		return utils.ErrorResponse(c, fiber.StatusForbidden, "Insufficient permissions", "")
	}
}

// RefreshTokenRequired middleware que valida refresh tokens - CORREGIDO
func (m *AuthMiddleware) RefreshTokenRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Authorization header required", "")
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid authorization header format", "Use 'Bearer <token>'")
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Token is required", "")
		}

		// Validate refresh token
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			if m.logger != nil {
				m.logger.Debug("Invalid refresh token", zap.Error(err))
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid refresh token", "")
		}

		// Check if it's actually a refresh token
		if claims.Type != "refresh" {
			if m.logger != nil {
				m.logger.Debug("Token is not a refresh token", zap.String("type", claims.Type))
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid token type", "Refresh token required")
		}

		// Convert user ID string to ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("Invalid user ID in refresh token", zap.Error(err))
			}
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid token", "")
		}

		// Store user info in context
		c.Locals("userID", userID)
		c.Locals("userRole", claims.Role)
		c.Locals("tokenType", claims.Type)
		c.Locals("user_id", userID) // Alias para compatibilidad

		return c.Next()
	}
}

// Helper functions para extraer información del contexto

// GetCurrentUserID extrae el ID del usuario actual del contexto
func GetCurrentUserID(c *fiber.Ctx) (primitive.ObjectID, error) {
	userID := c.Locals("userID")
	if userID == nil {
		return primitive.NilObjectID, utils.ErrUnauthorized
	}

	objID, ok := userID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, utils.ErrUnauthorized
	}

	return objID, nil
}

// GetCurrentUserRole extrae el rol del usuario actual del contexto
func GetCurrentUserRole(c *fiber.Ctx) (string, error) {
	userRole := c.Locals("userRole")
	if userRole == nil {
		return "", utils.ErrUnauthorized
	}

	role, ok := userRole.(string)
	if !ok {
		return "", utils.ErrUnauthorized
	}

	return role, nil
}

// IsAdmin verifica si el usuario actual es admin
func IsAdmin(c *fiber.Ctx) bool {
	role, err := GetCurrentUserRole(c)
	if err != nil {
		return false
	}
	return role == string(models.RoleAdmin)
}

// IsAuthenticated verifica si la petición está autenticada
func IsAuthenticated(c *fiber.Ctx) bool {
	_, err := GetCurrentUserID(c)
	return err == nil
}

// SetLogger configura el logger para el middleware
func (m *AuthMiddleware) SetLogger(logger *zap.Logger) {
	m.logger = logger
}
