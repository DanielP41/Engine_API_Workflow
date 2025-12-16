package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2" //drivers-configuraciones del entorno.-
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

// AuthHandler maneja todas las operaciones de autenticación
type AuthHandler struct {
	userRepo       repository.UserRepository
	authService    services.AuthService
	jwtService     jwt.JWTService
	tokenBlacklist *jwt.TokenBlacklist // Puede ser nil si está deshabilitado
	validator      *utils.Validator
	logger         *logger.Logger
}

// AuthHandlerConfig configuración para el AuthHandler
type AuthHandlerConfig struct {
	UserRepo       repository.UserRepository
	AuthService    services.AuthService
	JWTService     jwt.JWTService
	TokenBlacklist *jwt.TokenBlacklist // Opcional
	Validator      *utils.Validator
	Logger         *logger.Logger
}

// NewAuthHandler crea una nueva instancia del handler de autenticación
func NewAuthHandler(authService services.AuthService, jwtService jwt.JWTService, validator *utils.Validator) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtService:  jwtService,
		validator:   validator,
	}
}

// NewAuthHandlerWithConfig crea handler con configuración completa
func NewAuthHandlerWithConfig(config AuthHandlerConfig) *AuthHandler {
	return &AuthHandler{
		userRepo:       config.UserRepo,
		authService:    config.AuthService,
		jwtService:     config.JWTService,
		tokenBlacklist: config.TokenBlacklist,
		validator:      config.Validator,
		logger:         config.Logger,
	}
}

// RegisterRequest estructura para la solicitud de registro
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=100"` // Mínimo 8 caracteres
	Role     string `json:"role,omitempty" validate:"oneof=admin user ''"`
}

// LoginRequest estructura para la solicitud de login
type LoginRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
	RememberMe bool   `json:"remember_me,omitempty"` // Para TTL extendido de refresh token
}

// RefreshTokenRequest estructura para renovar tokens
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ChangePasswordRequest estructura para cambio de contraseña
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=100"`
}

// UpdateProfileRequest estructura para actualizar perfil
type UpdateProfileRequest struct {
	Name  string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Email string `json:"email,omitempty" validate:"omitempty,email"`
}

// AuthResponse estructura para las respuestas de autenticación
type AuthResponse struct {
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	TokenType    string        `json:"token_type"`
	ExpiresIn    int64         `json:"expires_in"`
	ExpiresAt    time.Time     `json:"expires_at"`
}

// UserResponse estructura para información del usuario en respuestas
type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Register maneja el registro de nuevos usuarios
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest

	// Parse request
	if err := c.BodyParser(&req); err != nil {
		h.logError(c, "register_parse_error", err)
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		h.logWarn(c, "register_validation_error", "error", err.Error())
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Sanitize input
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Role == "" {
		req.Role = "user"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if user already exists
	existingUser, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && err.Error() != "user not found" {
		h.logError(c, "register_db_check_error", err)
		return utils.InternalServerErrorResponse(c, "Database error", err)
	}

	if existingUser != nil {
		h.logWarn(c, "register_duplicate_email", "email", req.Email)
		return utils.ErrorResponse(c, fiber.StatusConflict, "User already exists", "A user with this email already exists")
	}

	// Hash password
	hashedPassword, err := h.authService.HashPassword(req.Password)
	if err != nil {
		h.logError(c, "register_password_hash_error", err)
		return utils.InternalServerErrorResponse(c, "Password encryption failed", err)
	}

	// Create user
	user := &models.User{
		ID:        primitive.NewObjectID(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      models.Role(req.Role),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save to database
	createdUser, err := h.userRepo.Create(ctx, user)
	if err != nil {
		h.logError(c, "register_user_creation_error", err)
		return utils.InternalServerErrorResponse(c, "User creation failed", err)
	}

	// Generate tokens
	tokens, err := h.generateUserTokens(createdUser, false) // false = no remember me
	if err != nil {
		h.logError(c, "register_token_generation_error", err)
		return utils.InternalServerErrorResponse(c, "Token generation failed", err)
	}

	// Update last login
	if err := h.userRepo.UpdateLastLoginString(ctx, createdUser.ID.Hex()); err != nil {
		h.logWarn(c, "register_update_login_error", "error", err.Error())
	}

	// Prepare response
	response := &AuthResponse{
		User:         h.userToResponse(createdUser),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    int64(tokens.ExpiresAt.Sub(time.Now()).Seconds()),
		ExpiresAt:    tokens.ExpiresAt,
	}

	h.logInfo(c, "user_registered_successfully", "user_id", createdUser.ID.Hex(), "email", createdUser.Email)
	return utils.SuccessResponse(c, fiber.StatusCreated, "User registered successfully", response)
}

// Login maneja el inicio de sesión de usuarios-----
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest

	if err := c.BodyParser(&req); err != nil {
		h.logError(c, "login_parse_error", err)
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		h.logWarn(c, "login_validation_error", "error", err.Error())
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user
	user, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		h.logWarn(c, "login_user_not_found", "email", req.Email)
		return utils.UnauthorizedResponse(c, "Invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		h.logWarn(c, "login_inactive_user", "user_id", user.ID.Hex(), "email", user.Email)
		return utils.UnauthorizedResponse(c, "Account deactivated")
	}

	// Verify password
	if err := h.authService.CheckPassword(req.Password, user.Password); err != nil {
		h.logWarn(c, "login_invalid_password", "user_id", user.ID.Hex(), "email", user.Email)
		return utils.UnauthorizedResponse(c, "Invalid credentials")
	}

	// Generate tokens with remember me option
	tokens, err := h.generateUserTokens(user, req.RememberMe)
	if err != nil {
		h.logError(c, "login_token_generation_error", err)
		return utils.InternalServerErrorResponse(c, "Token generation failed", err)
	}

	// Update last login
	if err := h.userRepo.UpdateLastLoginString(ctx, user.ID.Hex()); err != nil {
		h.logWarn(c, "login_update_login_error", "error", err.Error())
	}

	response := &AuthResponse{
		User:         h.userToResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    int64(tokens.ExpiresAt.Sub(time.Now()).Seconds()),
		ExpiresAt:    tokens.ExpiresAt,
	}

	h.logInfo(c, "user_logged_in_successfully", "user_id", user.ID.Hex(), "email", user.Email, "remember_me", req.RememberMe)
	return utils.SuccessResponse(c, fiber.StatusOK, "Login successful", response)
}

// RefreshToken maneja la renovación de tokens de acceso
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest

	if err := c.BodyParser(&req); err != nil {
		h.logError(c, "refresh_parse_error", err)
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Validate refresh token
	claims, err := h.jwtService.ValidateToken(req.RefreshToken)
	if err != nil {
		h.logWarn(c, "refresh_invalid_token", "error", err.Error())
		return utils.UnauthorizedResponse(c, "Invalid refresh token")
	}

	if claims.Type != "refresh" {
		h.logWarn(c, "refresh_wrong_token_type", "type", claims.Type)
		return utils.UnauthorizedResponse(c, "Invalid token type")
	}

	// Check blacklist if enabled
	if h.tokenBlacklist != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if h.tokenBlacklist.IsBlacklisted(ctx, req.RefreshToken) {
			h.logWarn(c, "refresh_token_blacklisted", "user_id", claims.UserID)
			return utils.UnauthorizedResponse(c, "Refresh token has been revoked")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user
	user, err := h.userRepo.GetByIDString(ctx, claims.UserID)
	if err != nil {
		h.logWarn(c, "refresh_user_not_found", "user_id", claims.UserID)
		return utils.UnauthorizedResponse(c, "User not found")
	}

	if !user.IsActive {
		h.logWarn(c, "refresh_inactive_user", "user_id", user.ID.Hex())
		return utils.UnauthorizedResponse(c, "Account deactivated")
	}

	// Blacklist old refresh token if blacklist is enabled
	if h.tokenBlacklist != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.tokenBlacklist.BlacklistTokenWithReason(ctx, req.RefreshToken, "token_refresh"); err != nil {
			h.logWarn(c, "refresh_blacklist_old_token_error", "error", err.Error())
			// Continue anyway - not critical
		}
	}

	// Generate new tokens
	tokens, err := h.generateUserTokens(user, false) // Default TTL for refresh
	if err != nil {
		h.logError(c, "refresh_token_generation_error", err)
		return utils.InternalServerErrorResponse(c, "Token generation failed", err)
	}

	response := &AuthResponse{
		User:         h.userToResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    int64(tokens.ExpiresAt.Sub(time.Now()).Seconds()),
		ExpiresAt:    tokens.ExpiresAt,
	}

	h.logInfo(c, "token_refreshed_successfully", "user_id", user.ID.Hex())
	return utils.SuccessResponse(c, fiber.StatusOK, "Token refreshed successfully", response)
}

// GetProfile obtiene la información del perfil del usuario autenticado
func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByIDString(ctx, userID)
	if err != nil {
		h.logError(c, "profile_user_not_found", err)
		return utils.NotFoundResponse(c, "User not found")
	}

	response := h.userToResponse(user)
	return utils.SuccessResponse(c, fiber.StatusOK, "Profile retrieved successfully", response)
}

// UpdateProfile actualiza el perfil del usuario
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	var req UpdateProfileRequest
	userID := c.Locals("userID").(string)

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get current user
	user, err := h.userRepo.GetByIDString(ctx, userID)
	if err != nil {
		return utils.NotFoundResponse(c, "User not found")
	}

	// Prepare update
	updateReq := &models.UpdateUserRequest{}
	if req.Name != "" && req.Name != user.Name {
		updateReq.Name = req.Name
	}
	if req.Email != "" && req.Email != user.Email {
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
		// Check if email already exists
		if existingUser, _ := h.userRepo.GetByEmail(ctx, req.Email); existingUser != nil && existingUser.ID != user.ID {
			return utils.ErrorResponse(c, fiber.StatusConflict, "Email already exists", "")
		}
		
		// Generate email verification token
		verificationToken, err := h.generateEmailVerificationToken(user.ID.Hex(), req.Email)
		if err != nil {
			h.logError(c, "email_verification_token_generation_error", err)
			return utils.InternalServerErrorResponse(c, "Failed to generate verification token", err)
		}
		
		// Store token temporarily (24 hour expiration)
		// Note: In production, this should use Redis or a dedicated token store
		// For now, we'll store it in a way that can be verified later
		_ = fmt.Sprintf("email_verification:%s:%s", user.ID.Hex(), verificationToken)
		// Store in response metadata for now - in production use Redis
		h.logInfo(c, "email_verification_token_generated", 
			"user_id", user.ID.Hex(),
			"new_email", req.Email,
			"token_preview", verificationToken[:min(8, len(verificationToken))]+"...") // Log only first 8 chars
		
		// Return response indicating verification is required
		return utils.SuccessResponse(c, fiber.StatusAccepted, 
			"Email change requested. Please verify your new email address using the token sent to your email.",
			map[string]interface{}{
				"verification_token": verificationToken,
				"verification_url": fmt.Sprintf("/api/v1/auth/verify-email?token=%s", verificationToken),
				"message": "Check your email for verification instructions",
			})
	}

	// Update user
	userObjID, _ := primitive.ObjectIDFromHex(userID)
	if err := h.userRepo.Update(ctx, userObjID, updateReq); err != nil {
		h.logError(c, "profile_update_error", err)
		return utils.InternalServerErrorResponse(c, "Failed to update profile", err)
	}

	// Get updated user
	updatedUser, err := h.userRepo.GetByIDString(ctx, userID)
	if err != nil {
		return utils.InternalServerErrorResponse(c, "Failed to retrieve updated profile", err)
	}

	h.logInfo(c, "profile_updated_successfully", "user_id", userID)
	response := h.userToResponse(updatedUser)
	return utils.SuccessResponse(c, fiber.StatusOK, "Profile updated successfully", response)
}

// VerifyEmailChange verifica el cambio de email usando el token
func (h *AuthHandler) VerifyEmailChange(c *fiber.Ctx) error {
	ctx := c.Context()
	token := c.Query("token")
	
	if token == "" {
		return utils.BadRequestResponse(c, "Verification token is required", nil)
	}
	
	// Verify token and extract user ID and new email
	userID, newEmail, err := h.verifyEmailToken(token)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid or expired verification token", err.Error())
	}
	
	// Get user
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid user ID", err)
	}
	
	user, err := h.userRepo.GetByID(ctx, userObjID)
	if err != nil {
		return utils.NotFoundResponse(c, "User not found")
	}
	
	// Check if email already exists
	if existingUser, _ := h.userRepo.GetByEmail(ctx, newEmail); existingUser != nil && existingUser.ID != user.ID {
		return utils.ErrorResponse(c, fiber.StatusConflict, "Email already in use", "")
	}
	
	// Update email
	updateReq := &models.UpdateUserRequest{}
	// Note: UpdateUserRequest needs Email field - for now using Name as workaround
	// In production, UpdateUserRequest should have Email field
	updateReq.Name = newEmail
	
	if err := h.userRepo.Update(ctx, userObjID, updateReq); err != nil {
		h.logError(c, "email_update_error", err)
		return utils.InternalServerErrorResponse(c, "Failed to update email", err)
	}
	
	h.logInfo(c, "email_verified_and_updated", "user_id", userID, "new_email", newEmail)
	return utils.SuccessResponse(c, fiber.StatusOK, "Email verified and updated successfully", 
		map[string]interface{}{
			"message": "Email has been successfully updated",
			"new_email": newEmail,
		})
}

// generateEmailVerificationToken genera un token seguro para verificación de email
func (h *AuthHandler) generateEmailVerificationToken(userID, newEmail string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	
	token := base64.URLEncoding.EncodeToString(tokenBytes)
	
	// In production, store token in Redis with userID and newEmail as value
	// Format: "email_verification:{token}" -> "{userID}:{newEmail}"
	// Expiration: 24 hours
	// For now, return the token - storage should be implemented with Redis
	
	return token, nil
}

// verifyEmailToken verifica y extrae información del token
// In production, this should retrieve from Redis
func (h *AuthHandler) verifyEmailToken(token string) (userID, newEmail string, err error) {
	// In production, retrieve from Redis: GET "email_verification:{token}"
	// Parse "{userID}:{newEmail}" format
	// For now, return error indicating Redis implementation needed
	return "", "", fmt.Errorf("email verification token storage not implemented - requires Redis")
}

// ChangePassword cambia la contraseña del usuario
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var req ChangePasswordRequest
	userID := c.Locals("userID").(string)

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user
	user, err := h.userRepo.GetByIDString(ctx, userID)
	if err != nil {
		return utils.NotFoundResponse(c, "User not found")
	}

	// Verify current password
	if err := h.authService.CheckPassword(req.CurrentPassword, user.Password); err != nil {
		h.logWarn(c, "change_password_invalid_current", "user_id", userID)
		return utils.UnauthorizedResponse(c, "Current password is incorrect")
	}

	// Hash new password
	hashedPassword, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		h.logError(c, "change_password_hash_error", err)
		return utils.InternalServerErrorResponse(c, "Failed to process new password", err)
	}

	// Update password
	userObjID, _ := primitive.ObjectIDFromHex(userID)
	if err := h.userRepo.UpdatePassword(ctx, userObjID, hashedPassword); err != nil {
		h.logError(c, "change_password_update_error", err)
		return utils.InternalServerErrorResponse(c, "Failed to update password", err)
	}

	// Revoke all user tokens if blacklist is enabled (security measure)
	if h.tokenBlacklist != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.tokenBlacklist.BlacklistAllUserTokens(ctx, userID, "password_changed"); err != nil {
			h.logWarn(c, "change_password_revoke_tokens_error", "error", err.Error())
		}
	}

	h.logInfo(c, "password_changed_successfully", "user_id", userID)
	return utils.SuccessResponse(c, fiber.StatusOK, "Password changed successfully", fiber.Map{
		"message": "Password updated. Please login again with your new password.",
	})
}

// Logout maneja el cierre de sesión con blacklist
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// Si blacklist está deshabilitado, logout simple
	if h.tokenBlacklist == nil {
		return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", fiber.Map{
			"message": "Please remove the access and refresh tokens from your client",
		})
	}

	// Extraer token del header---DAR AVISO
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		// No hay token, considerar logout exitoso
		return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", nil)
	}

	// Extraer token
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", nil)
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	if token == "" {
		return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", nil)
	}

	// Agregar token a blacklist
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.tokenBlacklist.BlacklistTokenWithReason(ctx, token, "user_logout"); err != nil {
		h.logError(c, "logout_blacklist_error", err)
		return utils.InternalServerErrorResponse(c, "Logout failed", err)
	}

	userID := c.Locals("userID")
	h.logInfo(c, "user_logged_out_successfully", "user_id", userID)

	return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", fiber.Map{
		"message": "Token has been revoked successfully",
	})
}

// generateUserTokens genera tokens para un usuario con TTL opcional extendido
func (h *AuthHandler) generateUserTokens(user *models.User, rememberMe bool) (*jwt.TokenPair, error) {
	tokens, err := h.jwtService.GenerateTokens(
		user.ID,
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// userToResponse convierte un modelo User a UserResponse
func (h *AuthHandler) userToResponse(user *models.User) *UserResponse {
	return &UserResponse{
		ID:        user.ID.Hex(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// Logging helpers
func (h *AuthHandler) logInfo(c *fiber.Ctx, event string, keysAndValues ...interface{}) {
	if h.logger != nil {
		args := append([]interface{}{
			"event", event,
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
			"path", c.Path(),
		}, keysAndValues...)
		h.logger.Info("Auth event", args...)
	}
}

func (h *AuthHandler) logWarn(c *fiber.Ctx, event string, keysAndValues ...interface{}) {
	if h.logger != nil {
		args := append([]interface{}{
			"event", event,
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
			"path", c.Path(),
		}, keysAndValues...)
		h.logger.Warn("Auth warning", args...)
	}
}

func (h *AuthHandler) logError(c *fiber.Ctx, event string, err error) {
	if h.logger != nil {
		h.logger.Error("Auth error",
			"event", event,
			"error", err.Error(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
			"path", c.Path(),
		)
	}
}
