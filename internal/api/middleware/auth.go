package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
)

// AuthMiddleware estructura para el middleware de autenticación
type AuthMiddleware struct {
	jwtService jwt.JWTService
}

// NewAuthMiddleware crea una nueva instancia del middleware de autenticación
func NewAuthMiddleware(jwtService jwt.JWTService) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
	}
}

// RequireAuth middleware que requiere autenticación
func (m *AuthMiddleware) RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Obtener el header de autorización
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return utils.UnauthorizedResponse(c, "Authorization header required")
		}

		// Verificar formato Bearer
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return utils.UnauthorizedResponse(c, "Invalid authorization header format")
		}

		// Extraer token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return utils.UnauthorizedResponse(c, "Token is required")
		}

		// Validar token
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			return utils.UnauthorizedResponse(c, "Invalid token")
		}

		// Verificar que sea un access token
		if claims.Type != "access" {
			return utils.UnauthorizedResponse(c, "Invalid token type")
		}

		// Convertir user ID a ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			return utils.UnauthorizedResponse(c, "Invalid user ID in token")
		}

		// Almacenar información del usuario en el contexto
		c.Locals("userID", claims.UserID) // String para compatibilidad
		c.Locals("user_id", userID)       // ObjectID para handlers que lo necesiten
		c.Locals("userRole", claims.Role)
		c.Locals("user_role", claims.Role)
		c.Locals("tokenType", claims.Type)
		c.Locals("is_admin", claims.Role == string(models.RoleAdmin))

		return c.Next()
	}
}

// RequireAdmin middleware que requiere permisos de administrador
func (m *AuthMiddleware) RequireAdmin() fiber.Handler {
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

// OptionalAuth middleware de autenticación opcional
func (m *AuthMiddleware) OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Next()
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return c.Next()
		}

		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			return c.Next()
		}

		if claims.Type == "access" {
			userID, err := primitive.ObjectIDFromHex(claims.UserID)
			if err == nil {
				c.Locals("userID", claims.UserID)
				c.Locals("user_id", userID)
				c.Locals("userRole", claims.Role)
				c.Locals("user_role", claims.Role)
				c.Locals("is_admin", claims.Role == string(models.RoleAdmin))
			}
		}

		return c.Next()
	}
}
