package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
)

// AuthHandler maneja todas las operaciones de autenticación
type AuthHandler struct {
	userRepo    repository.UserRepository
	authService services.AuthService
	jwtService  jwt.JWTService
	validator   *utils.Validator
}

// NewAuthHandler crea una nueva instancia del handler de autenticación
func NewAuthHandler(authService services.AuthService, jwtService jwt.JWTService, validator *utils.Validator) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtService:  jwtService,
		validator:   validator,
	}
}

// RegisterRequest estructura para la solicitud de registro
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6,max=100"`
	Role     string `json:"role,omitempty" validate:"oneof=admin user ''"`
}

// LoginRequest estructura para la solicitud de login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshTokenRequest estructura para renovar tokens
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// AuthResponse estructura para las respuestas de autenticación
type AuthResponse struct {
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int64         `json:"expires_in"`
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

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Role == "" {
		req.Role = "user"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verificar si el usuario ya existe
	existingUser, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && err.Error() != "user not found" {
		log.Printf("Error checking existing user: %v", err)
		return utils.InternalServerErrorResponse(c, "Database error", err)
	}

	if existingUser != nil {
		return utils.ErrorResponseFunc(c, fiber.StatusConflict, "User already exists", "A user with this email already exists")
	}

	// Hash de la contraseña
	hashedPassword, err := h.authService.HashPassword(req.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return utils.InternalServerErrorResponse(c, "Password encryption failed", err)
	}

	// Crear nuevo usuario
	user := &models.User{
		ID:        primitive.NewObjectID(),
		Name:      strings.TrimSpace(req.Name),
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      models.Role(req.Role),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Guardar en la base de datos
	createdUser, err := h.userRepo.Create(ctx, user)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return utils.InternalServerErrorResponse(c, "User creation failed", err)
	}

	// Generar tokens JWT
	accessToken, refreshToken, err := h.authService.GenerateTokens(
		createdUser.ID.Hex(),
		createdUser.Email,
		string(createdUser.Role),
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		return utils.InternalServerErrorResponse(c, "Token generation failed", err)
	}

	// Actualizar último login
	if err := h.userRepo.UpdateLastLoginString(ctx, createdUser.ID.Hex()); err != nil {
		log.Printf("Error updating last login: %v", err)
	}

	// Preparar respuesta
	response := &AuthResponse{
		User: &UserResponse{
			ID:        createdUser.ID.Hex(),
			Name:      createdUser.Name,
			Email:     createdUser.Email,
			Role:      string(createdUser.Role),
			IsActive:  createdUser.IsActive,
			CreatedAt: createdUser.CreatedAt,
			UpdatedAt: createdUser.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, "User registered successfully", response)
}

// Login maneja el inicio de sesión de usuarios
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if err.Error() == "user not found" {
			return utils.UnauthorizedResponse(c, "Invalid credentials")
		}
		log.Printf("Error finding user: %v", err)
		return utils.InternalServerErrorResponse(c, "Database error", err)
	}

	if !user.IsActive {
		return utils.UnauthorizedResponse(c, "Account deactivated")
	}

	if err := h.authService.CheckPassword(req.Password, user.Password); err != nil {
		return utils.UnauthorizedResponse(c, "Invalid credentials")
	}

	accessToken, refreshToken, err := h.authService.GenerateTokens(
		user.ID.Hex(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		return utils.InternalServerErrorResponse(c, "Token generation failed", err)
	}

	if err := h.userRepo.UpdateLastLoginString(ctx, user.ID.Hex()); err != nil {
		log.Printf("Error updating last login: %v", err)
	}

	response := &AuthResponse{
		User: &UserResponse{
			ID:        user.ID.Hex(),
			Name:      user.Name,
			Email:     user.Email,
			Role:      string(user.Role),
			IsActive:  user.IsActive,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Login successful", response)
}

// RefreshToken maneja la renovación de tokens de acceso
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	claims, err := h.authService.ValidateToken(req.RefreshToken)
	if err != nil {
		return utils.UnauthorizedResponse(c, "Invalid refresh token")
	}

	if claims.Type != "refresh" {
		return utils.UnauthorizedResponse(c, "Invalid token type")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByIDString(ctx, claims.UserID)
	if err != nil {
		if err.Error() == "user not found" {
			return utils.UnauthorizedResponse(c, "User not found")
		}
		log.Printf("Error finding user: %v", err)
		return utils.InternalServerErrorResponse(c, "Database error", err)
	}

	if !user.IsActive {
		return utils.UnauthorizedResponse(c, "Account deactivated")
	}

	accessToken, refreshToken, err := h.authService.GenerateTokens(
		user.ID.Hex(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		return utils.InternalServerErrorResponse(c, "Token generation failed", err)
	}

	response := &AuthResponse{
		User: &UserResponse{
			ID:        user.ID.Hex(),
			Name:      user.Name,
			Email:     user.Email,
			Role:      string(user.Role),
			IsActive:  user.IsActive,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Token refreshed successfully", response)
}

// GetProfile obtiene la información del perfil del usuario autenticado
func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByIDString(ctx, userID)
	if err != nil {
		if err.Error() == "user not found" {
			return utils.NotFoundResponse(c, "User not found")
		}
		log.Printf("Error finding user: %v", err)
		return utils.InternalServerErrorResponse(c, "Database error", err)
	}

	response := &UserResponse{
		ID:        user.ID.Hex(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Profile retrieved successfully", response)
}

// Logout maneja el cierre de sesión
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", map[string]string{
		"message": "Please remove the access and refresh tokens from your client",
	})
}
