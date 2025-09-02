package services

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"Engine_API_Workflow/pkg/jwt"
)

// AuthService define la interfaz del servicio de autenticación - CORREGIDO
type AuthService interface {
	HashPassword(password string) (string, error)
	CheckPassword(password, hash string) error
	GenerateTokens(userID primitive.ObjectID, email string, role string) (*jwt.TokenPair, error)
	ValidateToken(token string) (*jwt.Claims, error)
	ValidateAccessToken(token string) (*jwt.Claims, error)
	ValidateRefreshToken(token string) (*jwt.Claims, error)
	RefreshTokens(refreshToken string) (*jwt.TokenPair, error)
	RevokeToken(token string) error
}

// AuthServiceImpl implementa el servicio de autenticación
type AuthServiceImpl struct {
	jwtService jwt.JWTService
}

// NewAuthService crea una nueva instancia del servicio de autenticación
func NewAuthService(jwtService jwt.JWTService) AuthService {
	return &AuthServiceImpl{
		jwtService: jwtService,
	}
}

// HashPassword genera un hash seguro de la contraseña
func (s *AuthServiceImpl) HashPassword(password string) (string, error) {
	// Generar hash con costo 12 (recomendado para producción)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
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

// GenerateTokens genera tanto el access token como el refresh token - SIGNATURE CORREGIDA
func (s *AuthServiceImpl) GenerateTokens(userID primitive.ObjectID, email, role string) (*jwt.TokenPair, error) {
	// Usar el método correcto de la interfaz JWTService
	tokenPair, err := s.jwtService.GenerateTokens(userID, email, role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return tokenPair, nil
}

// ValidateToken valida un token y retorna sus claims
func (s *AuthServiceImpl) ValidateToken(token string) (*jwt.Claims, error) {
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return claims, nil
}

// ValidateAccessToken valida específicamente un access token
func (s *AuthServiceImpl) ValidateAccessToken(token string) (*jwt.Claims, error) {
	claims, err := s.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	if claims.Type != "access" {
		return nil, fmt.Errorf("invalid token type, expected access token")
	}

	return claims, nil
}

// ValidateRefreshToken valida específicamente un refresh token
func (s *AuthServiceImpl) ValidateRefreshToken(token string) (*jwt.Claims, error) {
	claims, err := s.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	if claims.Type != "refresh" {
		return nil, fmt.Errorf("invalid token type, expected refresh token")
	}

	return claims, nil
}

// RefreshTokens genera nuevos tokens usando el refresh token
func (s *AuthServiceImpl) RefreshTokens(refreshToken string) (*jwt.TokenPair, error) {
	tokenPair, err := s.jwtService.RefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh tokens: %w", err)
	}

	return tokenPair, nil
}

// RevokeToken revoca un token
func (s *AuthServiceImpl) RevokeToken(token string) error {
	err := s.jwtService.RevokeToken(token)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return nil
}
