package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager maneja la generación y validación de tokens JWT
type JWTManager struct {
	secretKey            string
	tokenDuration        time.Duration
	refreshTokenDuration time.Duration
}

// Claims estructura personalizada para los claims del JWT
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshClaims estructura para refresh tokens (más simples)
type RefreshClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// NewJWTManager crea una nueva instancia del JWTManager
func NewJWTManager(secretKey string, tokenDuration time.Duration, refreshTokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:            secretKey,
		tokenDuration:        tokenDuration,
		refreshTokenDuration: refreshTokenDuration,
	}
}

// GenerateToken genera un access token JWT
func (manager *JWTManager) GenerateToken(userID, email, role string) (string, error) {
	if userID == "" || email == "" || role == "" {
		return "", fmt.Errorf("userID, email and role are required")
	}

	now := time.Now()
	expirationTime := now.Add(manager.tokenDuration)

	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "engine-api-workflow",
			Audience:  []string{"engine-api-workflow"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// GenerateRefreshToken genera un refresh token JWT
func (manager *JWTManager) GenerateRefreshToken(userID, email string) (string, error) {
	if userID == "" || email == "" {
		return "", fmt.Errorf("userID and email are required")
	}

	now := time.Now()
	expirationTime := now.Add(manager.refreshTokenDuration)

	claims := &RefreshClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "engine-api-workflow",
			Audience:  []string{"engine-api-workflow-refresh"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// ValidateToken valida y parsea un access token
func (manager *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token string is required")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			// Verificar que el método de firma sea el esperado
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(manager.secretKey), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verificar que el token no haya expirado
	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token has expired")
	}

	// Verificar que el token ya sea válido (NotBefore)
	if time.Now().Before(claims.NotBefore.Time) {
		return nil, fmt.Errorf("token not valid yet")
	}

	return claims, nil
}

// ValidateRefreshToken valida y parsea un refresh token
func (manager *JWTManager) ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token string is required")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&RefreshClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// Verificar que el método de firma sea el esperado
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(manager.secretKey)
		},
	)

	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok {
		return nil, fmt.Errorf("invalid refresh token claims")
	}

	// Verificar que el token no haya expirado
	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("refresh token has expired")
	}

	// Verificar que el token ya sea válido (NotBefore)
	if time.Now().Before(claims.NotBefore.Time) {
		return nil, fmt.Errorf("refresh token not valid yet")
	}

	return claims, nil
}

// ExtractTokenFromHeader extrae el token del header Authorization
func (manager *JWTManager) ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is required")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", fmt.Errorf("invalid authorization header format")
	}

	token := authHeader[len(bearerPrefix):]
	if token == "" {
		return "", fmt.Errorf("token is required")
	}

	return token, nil
}

// GetTokenDuration devuelve la duración del access token
func (manager *JWTManager) GetTokenDuration() time.Duration {
	return manager.tokenDuration
}

// GetRefreshTokenDuration devuelve la duración del refresh token
func (manager *JWTManager) GetRefreshTokenDuration() time.Duration {
	return manager.refreshTokenDuration
}

// IsTokenExpired verifica si un token ha expirado sin validar la firma
func (manager *JWTManager) IsTokenExpired(tokenString string) bool {
	token, _ := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(manager.secretKey), nil
		},
	)

	if token == nil {
		return true
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return true
	}

	return time.Now().After(claims.ExpiresAt.Time)
}

// GetClaims obtiene los claims de un token sin validar la firma
// Útil para debugging o logs
func (manager *JWTManager) GetClaims(tokenString string) (*Claims, error) {
	token, _ := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(manager.secretKey), nil
		},
	)

	if token == nil {
		return nil, fmt.Errorf("invalid token format")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// RevokeToken añade un token a una lista negra (implementación básica)
// En un entorno de producción, esto se haría con Redis o una base de datos
type TokenBlacklist interface {
	Add(tokenString string, expiration time.Time) error
	IsBlacklisted(tokenString string) bool
}

// SimpleInMemoryBlacklist implementación simple en memoria para desarrollo
type SimpleInMemoryBlacklist struct {
	tokens map[string]time.Time
}

// NewSimpleInMemoryBlacklist crea una nueva instancia de blacklist en memoria
func NewSimpleInMemoryBlacklist() *SimpleInMemoryBlacklist {
	return &SimpleInMemoryBlacklist{
		tokens: make(map[string]time.Time),
	}
}

// Add añade un token a la blacklist
func (bl *SimpleInMemoryBlacklist) Add(tokenString string, expiration time.Time) error {
	bl.tokens[tokenString] = expiration
	return nil
}

// IsBlacklisted verifica si un token está en la blacklist
func (bl *SimpleInMemoryBlacklist) IsBlacklisted(tokenString string) bool {
	expiration, exists := bl.tokens[tokenString]
	if !exists {
		return false
	}

	// Si el token ya expiró naturalmente, lo removemos de la blacklist
	if time.Now().After(expiration) {
		delete(bl.tokens, tokenString)
		return false
	}

	return true
}

// Cleanup limpia tokens expirados de la blacklist
func (bl *SimpleInMemoryBlacklist) Cleanup() {
	now := time.Now()
	for token, expiration := range bl.tokens {
		if now.After(expiration) {
			delete(bl.tokens, token)
		}
	}
}
