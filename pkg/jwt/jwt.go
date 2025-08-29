package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// JWTService interfaz para el servicio JWT
type JWTService interface {
	GenerateTokens(userID primitive.ObjectID, email, role string) (*TokenPair, error)
	ValidateToken(tokenString string) (*Claims, error)
	RefreshToken(refreshToken string) (*TokenPair, error)
	RevokeToken(tokenString string) error
}

// jwtService implementación del servicio JWT
type jwtService struct {
	secretKey       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	issuer          string
}

// TokenPair par de tokens (access y refresh)
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Claims estructura de claims JWT personalizada
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"` // "access" o "refresh"
	jwt.RegisteredClaims
}

// Config configuración para JWT Service
type Config struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
}

// NewJWTService crea una nueva instancia del servicio JWT
func NewJWTService(config Config) JWTService {
	return &jwtService{
		secretKey:       []byte(config.SecretKey),
		accessTokenTTL:  config.AccessTokenTTL,
		refreshTokenTTL: config.RefreshTokenTTL,
		issuer:          config.Issuer,
	}
}

// GenerateTokens genera un par de tokens (access y refresh)
func (s *jwtService) GenerateTokens(userID primitive.ObjectID, email, role string) (*TokenPair, error) {
	now := time.Now()

	// Access Token
	accessClaims := Claims{
		UserID: userID.Hex(),
		Email:  email,
		Role:   role,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.issuer,
			Subject:   userID.Hex(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(s.secretKey)
	if err != nil {
		return nil, err
	}

	// Refresh Token
	refreshClaims := Claims{
		UserID: userID.Hex(),
		Email:  email,
		Role:   role,
		Type:   "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.issuer,
			Subject:   userID.Hex(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(s.secretKey)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresAt:    now.Add(s.accessTokenTTL),
	}, nil
}

// ValidateToken valida un token JWT
func (s *jwtService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de firma inválido")
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token inválido")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("claims inválidos")
	}

	return claims, nil
}

// RefreshToken genera un nuevo par de tokens usando el refresh token
func (s *jwtService) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != "refresh" {
		return nil, errors.New("token no es de tipo refresh")
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, errors.New("ID de usuario inválido")
	}

	return s.GenerateTokens(userID, claims.Email, claims.Role)
}

// RevokeToken revoca un token (implementación básica - en producción usar blacklist)
func (s *jwtService) RevokeToken(tokenString string) error {
	// En una implementación real, agregarías el token a una blacklist en Redis
	// Por ahora, solo validamos que el token sea válido
	_, err := s.ValidateToken(tokenString)
	return err
}

// ExtractTokenFromHeader extrae el token del header Authorization
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("header de autorización vacío")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", errors.New("formato de token inválido")
	}

	return authHeader[len(bearerPrefix):], nil
}

// GetDefaultConfig retorna configuración por defecto para desarrollo
func GetDefaultConfig(secretKey string) Config {
	return Config{
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour, // 7 días
		Issuer:          "Engine_API_Workflow",
	}
}
