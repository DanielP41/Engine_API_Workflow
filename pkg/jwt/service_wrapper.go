package jwt

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NewService crea el servicio JWT compatible con main.go
// Esta función coincide exactamente con la llamada en main.go:
// jwtService := jwt.NewService(cfg.JWTSecret, "engine-api-workflow")
func NewService(secretKey, issuer string) JWTService {
	config := Config{
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          issuer,
	}

	return NewJWTService(config)
}

// Service alias para compatibilidad
type Service = JWTService

// SimpleJWTService versión simplificada para casos específicos
type SimpleJWTService struct {
	*jwtService
}

// NewSimpleService crea una versión simplificada
func NewSimpleService(secretKey string) *SimpleJWTService {
	config := GetDefaultConfig(secretKey)
	service := NewJWTService(config).(*jwtService)

	return &SimpleJWTService{jwtService: service}
}

// GenerateToken genera solo access token (método simplificado)
func (s *SimpleJWTService) GenerateToken(userID primitive.ObjectID, email, role string) (string, error) {
	tokens, err := s.GenerateTokens(userID, email, role)
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}
