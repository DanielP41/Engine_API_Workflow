package services

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/jwt"
)

// AuthService define la interfaz para el servicio de autenticación
type AuthService interface {
	HashPassword(password string) (string, error)
	CheckPassword(password, hash string) error
	GenerateTokens(userID string, email string, role string) (accessToken, refreshToken string, err error)
	ValidateToken(token string) (*jwt.Claims, error)
}

// AuthServiceImpl implementa el servicio de autenticación
type AuthServiceImpl struct {
	jwtService jwt.JWTService
	userRepo   repository.UserRepository
}

// NewAuthService crea una nueva instancia del servicio de autenticación
func NewAuthService(userRepo repository.UserRepository, jwtService jwt.JWTService) AuthService {
	return &AuthServiceImpl{
		jwtService: jwtService,
		userRepo:   userRepo,
	}
}

// HashPassword genera un hash seguro de la contraseña
func (s *AuthServiceImpl) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifica si la contraseña coincide con el hash
func (s *AuthServiceImpl) CheckPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return fmt.Errorf("password verification failed: %w", err)
	}
	return nil
}

// GenerateTokens genera tokens de acceso y refresh
func (s *AuthServiceImpl) GenerateTokens(userID string, email string, role string) (accessToken, refreshToken string, err error) {
	// Convertir string ID a ObjectID
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return "", "", fmt.Errorf("invalid user ID: %w", err)
	}

	// Generar tokens usando el servicio JWT
	tokenPair, err := s.jwtService.GenerateTokens(objectID, email, role)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	return tokenPair.AccessToken, tokenPair.RefreshToken, nil
}

// ValidateToken valida un token y retorna sus claims
func (s *AuthServiceImpl) ValidateToken(token string) (*jwt.Claims, error) {
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return claims, nil
}
