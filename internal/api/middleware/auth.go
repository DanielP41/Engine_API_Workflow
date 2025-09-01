package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/utils"
	jwtPkg "Engine_API_Workflow/pkg/jwt"
)

// AuthMiddleware validates JWT tokens - CORREGIDO: usar función con fiber.Ctx
func AuthMiddleware(jwtService jwtPkg.JWTService, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			logger.Debug("Missing authorization header")
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Authorization header required", "")
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			logger.Debug("Invalid authorization header format")
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid authorization header format", "Use 'Bearer <token>'")
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			logger.Debug("Empty token")
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Token is required", "")
		}

		// Validar token usando la interfaz correctamente
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			logger.Debug("Invalid token", zap.Error(err))

			// Check if token is expired
			if ve, ok := err.(*jwt.ValidationError); ok {
				if ve.Errors&jwt.ValidationErrorExpired != 0 {
					return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Token expired", "Please refresh your token")
				}
			}

			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid token", "")
		}

		// Convert user ID string to ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			logger.Error("Invalid user ID in token", zap.Error(err), zap.String("user_id", claims.UserID))
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid token", "")
		}

		// Store user info in context
		c.Locals("userID", userID)
		c.Locals("userRole", claims.Role)
		c.Locals("tokenType", claims.Type)

		return c.Next()
	}
}

// OptionalAuthMiddleware validates JWT tokens but doesn't require them
func OptionalAuthMiddleware(jwtService jwtPkg.JWTService, logger *zap.Logger) fiber.Handler {
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

		// Validar token usando la interfaz correctamente
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			// Invalid token but optional, continue without auth
			logger.Debug("Invalid optional token", zap.Error(err))
			return c.Next()
		}

		// Convert user ID string to ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			logger.Debug("Invalid user ID in optional token", zap.Error(err))
			return c.Next()
		}

		// Store user info in context
		c.Locals("userID", userID)
		c.Locals("userRole", claims.Role)
		c.Locals("tokenType", claims.Type)

		return c.Next()
	}
}

// AdminRequiredMiddleware ensures the user has admin role
func AdminRequiredMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Authentication required", "")
		}

		role, ok := userRole.(string)
		if !ok || role != string(models.RoleAdmin) {
			return utils.ErrorResponseWithCode(c, fiber.StatusForbidden, "Admin access required", "")
		}

		return c.Next()
	}
}

// RoleRequiredMiddleware ensures the user has one of the specified roles
func RoleRequiredMiddleware(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")
		if userRole == nil {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Authentication required", "")
		}

		role, ok := userRole.(string)
		if !ok {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid user role", "")
		}

		// Check if user role is in allowed roles
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				return c.Next()
			}
		}

		return utils.ErrorResponseWithCode(c, fiber.StatusForbidden, "Insufficient permissions", "")
	}
}

// RefreshTokenMiddleware validates refresh tokens
func RefreshTokenMiddleware(jwtService jwtPkg.JWTService, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Authorization header required", "")
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid authorization header format", "Use 'Bearer <token>'")
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Token is required", "")
		}

		// Validate refresh token
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			logger.Debug("Invalid refresh token", zap.Error(err))
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid refresh token", "")
		}

		// Check if it's actually a refresh token
		if claims.Type != "refresh" {
			logger.Debug("Token is not a refresh token", zap.String("type", claims.Type))
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid token type", "Refresh token required")
		}

		// Convert user ID string to ObjectID
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			logger.Error("Invalid user ID in refresh token", zap.Error(err))
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid token", "")
		}

		// Store user info in context
		c.Locals("userID", userID)
		c.Locals("userRole", claims.Role)
		c.Locals("tokenType", claims.Type)

		return c.Next()
	}
}

// UserOwnershipMiddleware ensures the user can only access their own resources
func UserOwnershipMiddleware(userRepo repository.UserRepository, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get current user ID from context
		userID := c.Locals("userID")
		if userID == nil {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Authentication required", "")
		}

		currentUserID, ok := userID.(primitive.ObjectID)
		if !ok {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid user ID", "")
		}

		// Get user role
		userRole := c.Locals("userRole")
		role, ok := userRole.(string)
		if !ok {
			return utils.ErrorResponseWithCode(c, fiber.StatusUnauthorized, "Invalid user role", "")
		}

		// Admins can access any resource
		if role == string(models.RoleAdmin) {
			return c.Next()
		}

		// Get target user ID from URL params
		targetUserIDParam := c.Params("userID")
		if targetUserIDParam == "" {
			// If no user ID in params, user can access their own resources
			c.Locals("targetUserID", currentUserID)
			return c.Next()
		}

		// Convert target user ID to ObjectID
		targetUserID, err := primitive.ObjectIDFromHex(targetUserIDParam)
		if err != nil {
			return utils.ErrorResponseWithCode(c, fiber.StatusBadRequest, "Invalid user ID format", "")
		}

		// Check if user is trying to access their own resource
		if currentUserID != targetUserID {
			return utils.ErrorResponseWithCode(c, fiber.StatusForbidden, "Access denied", "You can only access your own resources")
		}

		c.Locals("targetUserID", targetUserID)
		return c.Next()
	}
}

// RateLimitByUserMiddleware implements per-user rate limiting
func RateLimitByUserMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// This is a placeholder for rate limiting implementation
		// You would typically use Redis or an in-memory store to track requests per user

		// For now, just pass through
		// TODO: Implement actual rate limiting logic
		return c.Next()
	}
}

// GetCurrentUserID extracts the current user ID from the request context
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

// GetCurrentUserRole extracts the current user role from the request context
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

// IsAdmin checks if the current user is an admin
func IsAdmin(c *fiber.Ctx) bool {
	role, err := GetCurrentUserRole(c)
	if err != nil {
		return false
	}
	return role == string(models.RoleAdmin)
}

// IsAuthenticated checks if the request is authenticated
func IsAuthenticated(c *fiber.Ctx) bool {
	_, err := GetCurrentUserID(c)
	return err == nil
}
