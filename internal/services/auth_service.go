package services

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

// AuthService interfaz para el servicio de autenticación
type AuthService interface {
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error)
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
	Logout(ctx context.Context, userID primitive.ObjectID) error
	ChangePassword(ctx context.Context, userID primitive.ObjectID, req *ChangePasswordRequest) error
	GetUserProfile(ctx context.Context, userID primitive.ObjectID) (*models.User, error)
	UpdateUserProfile(ctx context.Context, userID primitive.ObjectID, req *UpdateProfileRequest) error
}

// authService implementación del servicio de autenticación
type authService struct {
	userRepo   repository.UserRepository
	jwtManager *jwt.JWTManager
	log        logger.Logger
}

// Estructuras de request/response
type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required,min=2"`
	LastName  string `json:"last_name" validate:"required,min=2"`
	Role      string `json:"role" validate:"required,oneof=admin user viewer"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" validate:"required"`
}

type UpdateProfileRequest struct {
	FirstName string            `json:"first_name,omitempty" validate:"omitempty,min=2"`
	LastName  string            `json:"last_name,omitempty" validate:"omitempty,min=2"`
	Email     string            `json:"email,omitempty" validate:"omitempty,email"`
	Settings  map[string]string `json:"settings,omitempty"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	TokenType    string       `json:"token_type"`
	User         *models.User `json:"user"`
}

type TokenClaims struct {
	UserID    primitive.ObjectID `json:"user_id"`
	Email     string             `json:"email"`
	Role      string             `json:"role"`
	ExpiresAt time.Time          `json:"expires_at"`
}

// NewAuthService crea una nueva instancia del servicio de autenticación
func NewAuthService(userRepo repository.UserRepository, jwtManager *jwt.JWTManager, log logger.Logger) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		log:        log,
	}
}

// Register registra un nuevo usuario
func (s *authService) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	s.log.Info("Starting user registration", "email", req.Email)

	// Validar que las contraseñas coincidan si hay confirmación
	if req.Password == "" || len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters long")
	}

	// Verificar si el usuario ya existe
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.log.Error("Error checking existing user", "error", err)
		return nil, fmt.Errorf("error checking user existence: %w", err)
	}
	if existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Hash de la contraseña
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error("Error hashing password", "error", err)
		return nil, fmt.Errorf("error hashing password: %w", err)
	}

	// Crear el usuario
	user := &models.User{
		ID:           primitive.NewObjectID(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         req.Role,
		IsActive:     true,
		Settings:     make(map[string]string),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Guardar en base de datos
	err = s.userRepo.Create(ctx, user)
	if err != nil {
		s.log.Error("Error creating user", "error", err)
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Generar tokens
	accessToken, refreshToken, expiresIn, err := s.generateTokenPair(user)
	if err != nil {
		s.log.Error("Error generating tokens", "error", err)
		return nil, fmt.Errorf("error generating tokens: %w", err)
	}

	// Actualizar último login
	s.updateLastLogin(ctx, user.ID)

	s.log.Info("User registered successfully", "user_id", user.ID.Hex(), "email", user.Email)

	// Limpiar password hash antes de devolver
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

// Login autentica a un usuario
func (s *authService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	s.log.Info("Starting user login", "email", req.Email)

	// Buscar usuario por email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.log.Error("Error finding user", "error", err)
		return nil, fmt.Errorf("error finding user: %w", err)
	}
	if user == nil {
		s.log.Warn("Login attempt with non-existent email", "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verificar que el usuario esté activo
	if !user.IsActive {
		s.log.Warn("Login attempt with inactive user", "user_id", user.ID.Hex())
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verificar contraseña
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		s.log.Warn("Invalid password attempt", "user_id", user.ID.Hex())
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generar tokens
	accessToken, refreshToken, expiresIn, err := s.generateTokenPair(user)
	if err != nil {
		s.log.Error("Error generating tokens", "error", err)
		return nil, fmt.Errorf("error generating tokens: %w", err)
	}

	// Actualizar último login
	s.updateLastLogin(ctx, user.ID)

	s.log.Info("User logged in successfully", "user_id", user.ID.Hex())

	// Limpiar password hash antes de devolver
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

// RefreshToken renueva el token de acceso
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	s.log.Info("Refreshing token")

	// Validar refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		s.log.Error("Invalid refresh token", "error", err)
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Obtener usuario
	userID, err := primitive.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("Error finding user for token refresh", "error", err)
		return nil, fmt.Errorf("error finding user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, fmt.Errorf("user not found or inactive")
	}

	// Generar nuevos tokens
	newAccessToken, newRefreshToken, expiresIn, err := s.generateTokenPair(user)
	if err != nil {
		s.log.Error("Error generating new tokens", "error", err)
		return nil, fmt.Errorf("error generating tokens: %w", err)
	}

	s.log.Info("Token refreshed successfully", "user_id", user.ID.Hex())

	// Limpiar password hash antes de devolver
	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    expiresIn,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

// ValidateToken valida un token de acceso
func (s *authService) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	userID, err := primitive.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token: %w", err)
	}

	// Verificar que el usuario siga existiendo y activo
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error validating user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, fmt.Errorf("user not found or inactive")
	}

	return &TokenClaims{
		UserID:    userID,
		Email:     user.Email,
		Role:      user.Role,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0),
	}, nil
}

// Logout cierra la sesión del usuario
func (s *authService) Logout(ctx context.Context, userID primitive.ObjectID) error {
	s.log.Info("User logging out", "user_id", userID.Hex())

	// Aquí podrías implementar blacklisting de tokens si fuera necesario
	// Por simplicidad, solo registramos el logout

	// Podrías actualizar último logout si tuvieras ese campo
	// s.userRepo.UpdateLastLogout(ctx, userID, time.Now())

	s.log.Info("User logged out successfully", "user_id", userID.Hex())
	return nil
}

// ChangePassword cambia la contraseña del usuario
func (s *authService) ChangePassword(ctx context.Context, userID primitive.ObjectID, req *ChangePasswordRequest) error {
	s.log.Info("Starting password change", "user_id", userID.Hex())

	// Validar que las contraseñas nuevas coincidan
	if req.NewPassword != req.ConfirmPassword {
		return fmt.Errorf("new password and confirmation do not match")
	}

	// Validar longitud de nueva contraseña
	if len(req.NewPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters long")
	}

	// Obtener usuario actual
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("Error finding user for password change", "error", err)
		return fmt.Errorf("error finding user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// Verificar contraseña actual
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword))
	if err != nil {
		s.log.Warn("Invalid current password in change attempt", "user_id", userID.Hex())
		return fmt.Errorf("current password is incorrect")
	}

	// Hash de la nueva contraseña
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error("Error hashing new password", "error", err)
		return fmt.Errorf("error hashing new password: %w", err)
	}

	// Actualizar contraseña
	user.PasswordHash = string(hashedNewPassword)
	user.UpdatedAt = time.Now()

	err = s.userRepo.Update(ctx, userID, user)
	if err != nil {
		s.log.Error("Error updating password", "error", err)
		return fmt.Errorf("error updating password: %w", err)
	}

	s.log.Info("Password changed successfully", "user_id", userID.Hex())
	return nil
}

// GetUserProfile obtiene el perfil del usuario
func (s *authService) GetUserProfile(ctx context.Context, userID primitive.ObjectID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("Error getting user profile", "error", err, "user_id", userID.Hex())
		return nil, fmt.Errorf("error getting user profile: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Limpiar password hash antes de devolver
	user.PasswordHash = ""
	return user, nil
}

// UpdateUserProfile actualiza el perfil del usuario
func (s *authService) UpdateUserProfile(ctx context.Context, userID primitive.ObjectID, req *UpdateProfileRequest) error {
	s.log.Info("Updating user profile", "user_id", userID.Hex())

	// Obtener usuario actual
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("Error finding user for profile update", "error", err)
		return fmt.Errorf("error finding user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// Verificar si el email ya existe (si se está cambiando)
	if req.Email != "" && req.Email != user.Email {
		existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
		if err != nil {
			s.log.Error("Error checking email availability", "error", err)
			return fmt.Errorf("error checking email availability: %w", err)
		}
		if existingUser != nil {
			return fmt.Errorf("email already in use")
		}
		user.Email = req.Email
	}

	// Actualizar campos si se proporcionan
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Settings != nil {
		// Merge settings
		if user.Settings == nil {
			user.Settings = make(map[string]string)
		}
		for key, value := range req.Settings {
			user.Settings[key] = value
		}
	}

	user.UpdatedAt = time.Now()

	// Guardar cambios
	err = s.userRepo.Update(ctx, userID, user)
	if err != nil {
		s.log.Error("Error updating user profile", "error", err)
		return fmt.Errorf("error updating user profile: %w", err)
	}

	s.log.Info("User profile updated successfully", "user_id", userID.Hex())
	return nil
}

// generateTokenPair genera un par de tokens (access + refresh)
func (s *authService) generateTokenPair(user *models.User) (string, string, int64, error) {
	// Generar access token
	accessToken, err := s.jwtManager.GenerateToken(user.ID.Hex(), user.Email, user.Role)
	if err != nil {
		return "", "", 0, fmt.Errorf("error generating access token: %w", err)
	}

	// Generar refresh token
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID.Hex(), user.Email)
	if err != nil {
		return "", "", 0, fmt.Errorf("error generating refresh token: %w", err)
	}

	// Duración del token (típicamente 15 minutos para access token)
	expiresIn := s.jwtManager.GetTokenDuration().Milliseconds() / 1000

	return accessToken, refreshToken, expiresIn, nil
}

// updateLastLogin actualiza el último login del usuario (método auxiliar)
func (s *authService) updateLastLogin(ctx context.Context, userID primitive.ObjectID) {
	// Ejecutar en goroutine para no bloquear la respuesta
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			s.log.Error("Error getting user for last login update", "error", err)
			return
		}
		if user == nil {
			return
		}

		user.LastLogin = time.Now()
		user.UpdatedAt = time.Now()

		err = s.userRepo.Update(ctx, userID, user)
		if err != nil {
			s.log.Error("Error updating last login", "error", err)
		}
	}()
}
