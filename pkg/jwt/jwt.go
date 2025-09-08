package jwt

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// JWTService interfaz para el servicio JWT
type JWTService interface {
	GenerateTokens(userID primitive.ObjectID, email, role string) (*TokenPair, error)
	GenerateTokensWithTTL(userID primitive.ObjectID, email, role string, accessTTL, refreshTTL time.Duration) (*TokenPair, error)
	ValidateToken(tokenString string) (*Claims, error)
	RefreshToken(refreshToken string) (*TokenPair, error)
	RevokeToken(tokenString string) error
	GetTokenClaims(tokenString string) (*Claims, error)
	IsTokenExpired(tokenString string) bool
}

// jwtService implementación del servicio JWT
type jwtService struct {
	secretKey       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	issuer          string
	audience        string
}

// TokenPair par de tokens (access y refresh)
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int64     `json:"expires_in"` // Segundos hasta expiración
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
	Audience        string
}

// Validation errors
var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token has expired")
	ErrTokenNotValidYet = errors.New("token not valid yet")
	ErrTokenMalformed   = errors.New("malformed token")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrInvalidClaims    = errors.New("invalid token claims")
	ErrTokenRevoked     = errors.New("token has been revoked")
)

// NewJWTService crea una nueva instancia del servicio JWT
func NewJWTService(config Config) JWTService {
	// Validar configuración
	if len(config.SecretKey) < 32 {
		panic("JWT secret key must be at least 32 characters")
	}
	if config.AccessTokenTTL == 0 {
		config.AccessTokenTTL = 15 * time.Minute
	}
	if config.RefreshTokenTTL == 0 {
		config.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if config.Issuer == "" {
		config.Issuer = "engine-api-workflow"
	}
	if config.Audience == "" {
		config.Audience = "engine-api"
	}

	return &jwtService{
		secretKey:       []byte(config.SecretKey),
		accessTokenTTL:  config.AccessTokenTTL,
		refreshTokenTTL: config.RefreshTokenTTL,
		issuer:          config.Issuer,
		audience:        config.Audience,
	}
}

// GenerateTokens genera un par de tokens (access y refresh) con TTL por defecto
func (s *jwtService) GenerateTokens(userID primitive.ObjectID, email, role string) (*TokenPair, error) {
	return s.GenerateTokensWithTTL(userID, email, role, s.accessTokenTTL, s.refreshTokenTTL)
}

// GenerateTokensWithTTL genera tokens con TTL personalizado
func (s *jwtService) GenerateTokensWithTTL(userID primitive.ObjectID, email, role string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	if userID.IsZero() {
		return nil, errors.New("user ID cannot be empty")
	}
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}
	if role == "" {
		return nil, errors.New("role cannot be empty")
	}

	now := time.Now()
	userIDStr := userID.Hex()

	// Access Token
	accessClaims := Claims{
		UserID: userIDStr,
		Email:  email,
		Role:   role,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.issuer,
			Subject:   userIDStr,
			Audience:  []string{s.audience},
			ID:        generateJTI(), // JWT ID para tracking
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(s.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh Token
	refreshClaims := Claims{
		UserID: userIDStr,
		Email:  email,
		Role:   role,
		Type:   "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.issuer,
			Subject:   userIDStr,
			Audience:  []string{s.audience},
			ID:        generateJTI(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(s.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	expiresAt := now.Add(accessTTL)
	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt,
		ExpiresIn:    int64(accessTTL.Seconds()),
	}, nil
}

// ValidateToken valida un token JWT y retorna sus claims
func (s *jwtService) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verificar método de firma para prevenir algorithm confusion attacks
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, s.mapJWTError(err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	// Validaciones adicionales
	if err := s.validateClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// GetTokenClaims obtiene claims sin validar completamente (útil para parsing)
func (s *jwtService) GetTokenClaims(tokenString string) (*Claims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, ErrTokenMalformed
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// IsTokenExpired verifica si un token ha expirado
func (s *jwtService) IsTokenExpired(tokenString string) bool {
	claims, err := s.GetTokenClaims(tokenString)
	if err != nil {
		return true
	}

	if claims.ExpiresAt == nil {
		return true
	}

	return claims.ExpiresAt.Time.Before(time.Now())
}

// RefreshToken genera nuevos tokens usando el refresh token
func (s *jwtService) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	if claims.Type != "refresh" {
		return nil, ErrInvalidTokenType
	}

	// Convertir user ID de string a ObjectID
	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token: %w", err)
	}

	// Generar nuevos tokens
	return s.GenerateTokens(userID, claims.Email, claims.Role)
}

// RevokeToken revoca un token (implementación básica)
func (s *jwtService) RevokeToken(tokenString string) error {
	// En una implementación real, agregarías el token a una blacklist
	// Por ahora, solo validamos que el token sea válido
	_, err := s.ValidateToken(tokenString)
	if err != nil {
		return fmt.Errorf("cannot revoke invalid token: %w", err)
	}

	// TODO: Implementar blacklist en Redis
	return nil
}

// validateClaims valida claims específicos del negocio
func (s *jwtService) validateClaims(claims *Claims) error {
	// Validar issuer
	if claims.Issuer != s.issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", s.issuer, claims.Issuer)
	}

	// Validar audience - CORREGIDO para JWT v5
	if !slices.Contains(claims.Audience, s.audience) {
		return fmt.Errorf("invalid audience: expected %s", s.audience)
	}

	// Validar tipo de token
	if claims.Type != "access" && claims.Type != "refresh" {
		return fmt.Errorf("invalid token type: %s", claims.Type)
	}

	// Validar que tenga user ID
	if claims.UserID == "" {
		return errors.New("token missing user ID")
	}

	// Validar formato de user ID
	if _, err := primitive.ObjectIDFromHex(claims.UserID); err != nil {
		return fmt.Errorf("invalid user ID format: %s", claims.UserID)
	}

	// Validar email
	if claims.Email == "" {
		return errors.New("token missing email")
	}

	// Validar role
	if claims.Role == "" {
		return errors.New("token missing role")
	}

	return nil
}

// mapJWTError mapea errores de JWT a errores personalizados - CORREGIDO para JWT v5
func (s *jwtService) mapJWTError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return ErrTokenExpired
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return ErrTokenNotValidYet
	case errors.Is(err, jwt.ErrTokenMalformed):
		return ErrTokenMalformed
	case errors.Is(err, jwt.ErrSignatureInvalid):
		return ErrInvalidSignature
	default:
		return ErrInvalidToken
	}
}

// generateJTI genera un JWT ID único
func generateJTI() string {
	return primitive.NewObjectID().Hex()
}

// ExtractTokenFromHeader extrae el token del header Authorization
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is empty")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", errors.New("invalid authorization header format")
	}

	token := authHeader[len(bearerPrefix):]
	if token == "" {
		return "", errors.New("token is empty")
	}

	return token, nil
}

// GetDefaultConfig retorna configuración por defecto para desarrollo
func GetDefaultConfig(secretKey string) Config {
	return Config{
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "engine-api-workflow",
		Audience:        "engine-api",
	}
}

// Helper methods para Claims

// IsAccessToken verifica si las claims corresponden a un access token
func (c *Claims) IsAccessToken() bool {
	return c.Type == "access"
}

// IsRefreshToken verifica si las claims corresponden a un refresh token
func (c *Claims) IsRefreshToken() bool {
	return c.Type == "refresh"
}

// GetUserObjectID convierte el UserID string a primitive.ObjectID
func (c *Claims) GetUserObjectID() (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(c.UserID)
}

// IsExpired verifica si el token ha expirado
func (c *Claims) IsExpired() bool {
	if c.ExpiresAt == nil {
		return true
	}
	return c.ExpiresAt.Time.Before(time.Now())
}

// TimeUntilExpiry retorna el tiempo hasta la expiración
func (c *Claims) TimeUntilExpiry() time.Duration {
	if c.ExpiresAt == nil {
		return 0
	}
	return time.Until(c.ExpiresAt.Time)
}

// IsValidFor verifica si el token es válido para una audiencia específica - CORREGIDO para JWT v5
func (c *Claims) IsValidFor(audience string) bool {
	return slices.Contains(c.Audience, audience)
}

// TokenInfo estructura con información resumida del token
type TokenInfo struct {
	UserID    string        `json:"user_id"`
	Email     string        `json:"email"`
	Role      string        `json:"role"`
	Type      string        `json:"type"`
	ExpiresAt time.Time     `json:"expires_at"`
	ExpiresIn time.Duration `json:"expires_in"`
	IssuedAt  time.Time     `json:"issued_at"`
	Issuer    string        `json:"issuer"`
	Audience  []string      `json:"audience"`
}

// GetTokenInfo extrae información resumida de las claims
func (c *Claims) GetTokenInfo() TokenInfo {
	info := TokenInfo{
		UserID: c.UserID,
		Email:  c.Email,
		Role:   c.Role,
		Type:   c.Type,
		Issuer: c.Issuer,
	}

	if c.ExpiresAt != nil {
		info.ExpiresAt = c.ExpiresAt.Time
		info.ExpiresIn = time.Until(c.ExpiresAt.Time)
	}

	if c.IssuedAt != nil {
		info.IssuedAt = c.IssuedAt.Time
	}

	if len(c.Audience) > 0 {
		info.Audience = c.Audience
	}

	return info
}
