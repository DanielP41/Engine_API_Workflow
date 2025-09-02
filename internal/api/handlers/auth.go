package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
)

// AuthHandler maneja todas las operaciones de autenticación - CORREGIDO
type AuthHandler struct {
	userRepo    repository.UserRepository
	authService services.AuthService // Usar la interfaz del paquete services
}

// NewAuthHandler crea una nueva instancia del handler de autenticación - CORREGIDO
func NewAuthHandler(userRepo repository.UserRepository, authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		authService: authService,
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
	ExpiresIn    int64         `json:"expires_in"` // segundos hasta expiración
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
// POST /auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest

	// Parsear el cuerpo de la petición
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid JSON format", err.Error())
	}

	// Validar los datos de entrada
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err.Error())
	}

	// Normalizar email a minúsculas
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Establecer rol por defecto si no se proporciona
	if req.Role == "" {
		req.Role = "user"
	}

	// Verificar si el usuario ya existe
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	existingUser, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && err.Error() != "user not found" {
		log.Printf("Error checking existing user: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Database error", "Could not check existing user")
	}

	if existingUser != nil {
		return utils.ErrorResponse(c, fiber.StatusConflict, "User already exists", "A user with this email already exists")
	}

	// Hash de la contraseña
	hashedPassword, err := h.authService.HashPassword(req.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Password encryption failed", "Could not process password")
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

	// Aplicar valores por defecto
	user.BeforeCreate()

	// Guardar en la base de datos
	createdUser, err := h.userRepo.Create(ctx, user)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "User creation failed", "Could not create user account")
	}

	// Generar tokens JWT - SIGNATURE CORREGIDA
	tokenPair, err := h.authService.GenerateTokens(
		createdUser.ID,
		createdUser.Email,
		string(createdUser.Role),
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Token generation failed", "Could not generate authentication tokens")
	}

	// Actualizar último login
	if err := h.userRepo.UpdateLastLogin(ctx, createdUser.ID); err != nil {
		log.Printf("Error updating last login: %v", err)
		// No retornamos error porque el usuario ya fue creado exitosamente
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
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    3600, // 1 hora en segundos
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, "User registered successfully", response)
}

// Login maneja el inicio de sesión de usuarios
// POST /auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest

	// Parsear el cuerpo de la petición
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid JSON format", err.Error())
	}

	// Validar los datos de entrada
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err.Error())
	}

	// Normalizar email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Buscar usuario por email
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if err == mongo.ErrNoDocuments || err.Error() == "user not found" {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials", "Email or password is incorrect")
		}
		log.Printf("Error finding user: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Database error", "Could not retrieve user information")
	}

	// Verificar si el usuario está activo
	if !user.IsActive {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Account deactivated", "Your account has been deactivated")
	}

	// Verificar contraseña
	if err := h.authService.CheckPassword(req.Password, user.Password); err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials", "Email or password is incorrect")
	}

	// Generar tokens JWT - SIGNATURE CORREGIDA
	tokenPair, err := h.authService.GenerateTokens(
		user.ID,
		user.Email,
		string(user.Role),
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Token generation failed", "Could not generate authentication tokens")
	}

	// Actualizar último login
	if err := h.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		log.Printf("Error updating last login: %v", err)
		// No retornamos error porque el login fue exitoso
	}

	// Preparar respuesta
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
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    3600, // 1 hora en segundos
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Login successful", response)
}

// RefreshToken maneja la renovación de tokens de acceso
// POST /auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest

	// Parsear el cuerpo de la petición
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid JSON format", err.Error())
	}

	// Validar los datos de entrada
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err.Error())
	}

	// Validar el refresh token
	claims, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid refresh token", "The refresh token is invalid or expired")
	}

	// Convertir userID de string a ObjectID
	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid user ID", "")
	}

	// Buscar usuario para verificar que aún existe y está activo
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments || err.Error() == "user not found" {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not found", "The user associated with this token no longer exists")
		}
		log.Printf("Error finding user: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Database error", "Could not retrieve user information")
	}

	// Verificar si el usuario está activo
	if !user.IsActive {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Account deactivated", "Your account has been deactivated")
	}

	// Generar nuevos tokens - SIGNATURE CORREGIDA
	tokenPair, err := h.authService.GenerateTokens(
		user.ID,
		user.Email,
		string(user.Role),
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Token generation failed", "Could not generate new authentication tokens")
	}

	// Preparar respuesta
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
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    3600, // 1 hora en segundos
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Token refreshed successfully", response)
}

// GetProfile obtiene la información del perfil del usuario autenticado
// GET /auth/profile
func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	// Obtener el ID del usuario del contexto (establecido por el middleware de auth)
	userIDInterface := c.Locals("userID")
	if userIDInterface == nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", "")
	}

	userID, ok := userIDInterface.(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid user ID", "")
	}

	// Buscar usuario
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments || err.Error() == "user not found" {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "User not found", "The user profile could not be found")
		}
		log.Printf("Error finding user: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Database error", "Could not retrieve user profile")
	}

	// Preparar respuesta
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

// Logout maneja el cierre de sesión (opcional, principalmente para invalidar tokens del lado cliente)
// POST /auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// En un sistema JWT stateless, el logout se maneja principalmente del lado cliente

	return utils.SuccessResponse(c, fiber.StatusOK, "Logout successful", map[string]string{
		"message": "Please remove the access and refresh tokens from your client",
	})
}
