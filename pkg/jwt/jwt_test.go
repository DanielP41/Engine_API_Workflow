package jwt

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Test constants
const (
	testSecretKey = "test-secret-key-minimum-32-characters"
	testIssuer    = "test-issuer"
	testAudience  = "test-audience"
	testUserID    = "507f1f77bcf86cd799439011"
	testEmail     = "test@example.com"
	testRole      = "user"
)

// TestConfig obtiene configuración de test
func getTestConfig() Config {
	return Config{
		SecretKey:       testSecretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          testIssuer,
		Audience:        testAudience,
	}
}

// Helper para crear ObjectID de test
func getTestUserID() primitive.ObjectID {
	objID, _ := primitive.ObjectIDFromHex(testUserID)
	return objID
}

func TestNewJWTService(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectPanic bool
	}{
		{
			name:        "Valid config",
			config:      getTestConfig(),
			expectPanic: false,
		},
		{
			name: "Short secret key should panic",
			config: Config{
				SecretKey: "short",
			},
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Expected panic for short secret key")
					}
				}()
			}

			service := NewJWTService(tt.config)
			if !tt.expectPanic && service == nil {
				t.Errorf("Expected valid service, got nil")
			}
		})
	}
}

func TestGenerateTokens(t *testing.T) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	tests := []struct {
		name      string
		userID    primitive.ObjectID
		email     string
		role      string
		expectErr bool
	}{
		{
			name:      "Valid token generation",
			userID:    userID,
			email:     testEmail,
			role:      testRole,
			expectErr: false,
		},
		{
			name:      "Empty email should fail",
			userID:    userID,
			email:     "",
			role:      testRole,
			expectErr: true,
		},
		{
			name:      "Empty role should fail",
			userID:    userID,
			email:     testEmail,
			role:      "",
			expectErr: true,
		},
		{
			name:      "Zero ObjectID should fail",
			userID:    primitive.NilObjectID,
			email:     testEmail,
			role:      testRole,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenPair, err := service.GenerateTokens(tt.userID, tt.email, tt.role)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verificar estructura del token
			if tokenPair.AccessToken == "" {
				t.Errorf("Expected access token, got empty string")
			}
			if tokenPair.RefreshToken == "" {
				t.Errorf("Expected refresh token, got empty string")
			}
			if tokenPair.TokenType != "Bearer" {
				t.Errorf("Expected token type 'Bearer', got %s", tokenPair.TokenType)
			}
			if tokenPair.ExpiresIn <= 0 {
				t.Errorf("Expected positive expires_in, got %d", tokenPair.ExpiresIn)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	// Generar token válido para tests
	validTokenPair, err := service.GenerateTokens(userID, testEmail, testRole)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Generar token con secret diferente
	wrongSecretService := NewJWTService(Config{
		SecretKey:       "wrong-secret-key-minimum-32-characters",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          testIssuer,
		Audience:        testAudience,
	})
	wrongSecretToken, _ := wrongSecretService.GenerateTokens(userID, testEmail, testRole)

	tests := []struct {
		name      string
		token     string
		expectErr bool
		errType   error
	}{
		{
			name:      "Valid access token",
			token:     validTokenPair.AccessToken,
			expectErr: false,
		},
		{
			name:      "Valid refresh token",
			token:     validTokenPair.RefreshToken,
			expectErr: false,
		},
		{
			name:      "Empty token",
			token:     "",
			expectErr: true,
			errType:   ErrInvalidToken,
		},
		{
			name:      "Malformed token",
			token:     "invalid.token.format",
			expectErr: true,
			errType:   ErrTokenMalformed,
		},
		{
			name:      "Token with wrong secret",
			token:     wrongSecretToken.AccessToken,
			expectErr: true,
			errType:   ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateToken(tt.token)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("Expected error %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verificar claims
			if claims.UserID != userID.Hex() {
				t.Errorf("Expected UserID %s, got %s", userID.Hex(), claims.UserID)
			}
			if claims.Email != testEmail {
				t.Errorf("Expected email %s, got %s", testEmail, claims.Email)
			}
			if claims.Role != testRole {
				t.Errorf("Expected role %s, got %s", testRole, claims.Role)
			}
		})
	}
}

func TestTokenExpiration(t *testing.T) {
	// Configuración con TTL muy corto para test
	shortTTLConfig := Config{
		SecretKey:       testSecretKey,
		AccessTokenTTL:  1 * time.Millisecond,
		RefreshTokenTTL: 2 * time.Millisecond,
		Issuer:          testIssuer,
		Audience:        testAudience,
	}

	service := NewJWTService(shortTTLConfig)
	userID := getTestUserID()

	// Generar token
	tokenPair, err := service.GenerateTokens(userID, testEmail, testRole)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Esperar a que expire
	time.Sleep(10 * time.Millisecond)

	// Intentar validar token expirado
	_, err = service.ValidateToken(tokenPair.AccessToken)
	if err == nil {
		t.Errorf("Expected error for expired token, got nil")
	}

	// Verificar que es error de expiración
	if err != ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}
}

func TestIsTokenExpired(t *testing.T) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	// Token válido
	validTokenPair, err := service.GenerateTokens(userID, testEmail, testRole)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Token expirado
	expiredConfig := Config{
		SecretKey:       testSecretKey,
		AccessTokenTTL:  1 * time.Millisecond,
		RefreshTokenTTL: 2 * time.Millisecond,
		Issuer:          testIssuer,
		Audience:        testAudience,
	}
	expiredService := NewJWTService(expiredConfig)
	expiredTokenPair, _ := expiredService.GenerateTokens(userID, testEmail, testRole)
	time.Sleep(10 * time.Millisecond)

	tests := []struct {
		name      string
		token     string
		isExpired bool
	}{
		{
			name:      "Valid token is not expired",
			token:     validTokenPair.AccessToken,
			isExpired: false,
		},
		{
			name:      "Expired token is expired",
			token:     expiredTokenPair.AccessToken,
			isExpired: true,
		},
		{
			name:      "Invalid token returns true",
			token:     "invalid.token",
			isExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isExpired := service.IsTokenExpired(tt.token)
			if isExpired != tt.isExpired {
				t.Errorf("Expected isExpired %v, got %v", tt.isExpired, isExpired)
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	// Generar tokens iniciales
	tokenPair, err := service.GenerateTokens(userID, testEmail, testRole)
	if err != nil {
		t.Fatalf("Failed to generate test tokens: %v", err)
	}

	tests := []struct {
		name         string
		refreshToken string
		expectErr    bool
	}{
		{
			name:         "Valid refresh token",
			refreshToken: tokenPair.RefreshToken,
			expectErr:    false,
		},
		{
			name:         "Access token as refresh should fail",
			refreshToken: tokenPair.AccessToken,
			expectErr:    true,
		},
		{
			name:         "Invalid token should fail",
			refreshToken: "invalid.token",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTokenPair, err := service.RefreshToken(tt.refreshToken)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verificar que se generaron nuevos tokens
			if newTokenPair.AccessToken == "" {
				t.Errorf("Expected new access token, got empty")
			}
			if newTokenPair.RefreshToken == "" {
				t.Errorf("Expected new refresh token, got empty")
			}

			// Verificar que el nuevo access token es válido
			claims, err := service.ValidateToken(newTokenPair.AccessToken)
			if err != nil {
				t.Errorf("New access token validation failed: %v", err)
			}
			if claims.UserID != userID.Hex() {
				t.Errorf("New token has wrong UserID")
			}
		})
	}
}

func TestGetTokenClaims(t *testing.T) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	tokenPair, err := service.GenerateTokens(userID, testEmail, testRole)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	tests := []struct {
		name      string
		token     string
		expectErr bool
	}{
		{
			name:      "Valid token",
			token:     tokenPair.AccessToken,
			expectErr: false,
		},
		{
			name:      "Malformed token",
			token:     "invalid.token",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.GetTokenClaims(tt.token)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if claims.UserID != userID.Hex() {
				t.Errorf("Expected UserID %s, got %s", userID.Hex(), claims.UserID)
			}
		})
	}
}

func TestGenerateTokensWithTTL(t *testing.T) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	customAccessTTL := 30 * time.Minute
	customRefreshTTL := 14 * 24 * time.Hour

	tokenPair, err := service.GenerateTokensWithTTL(
		userID, testEmail, testRole,
		customAccessTTL, customRefreshTTL,
	)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	// Verificar que el token tiene el TTL correcto
	expectedExpiresIn := int64(customAccessTTL.Seconds())
	if tokenPair.ExpiresIn != expectedExpiresIn {
		t.Errorf("Expected ExpiresIn %d, got %d", expectedExpiresIn, tokenPair.ExpiresIn)
	}

	// Verificar que el token es válido
	claims, err := service.ValidateToken(tokenPair.AccessToken)
	if err != nil {
		t.Errorf("Generated token validation failed: %v", err)
	}

	// Verificar tiempo de expiración en claims
	expectedExpiry := time.Now().Add(customAccessTTL)
	actualExpiry := claims.ExpiresAt.Time

	// Permitir diferencia de 1 segundo por timing
	if actualExpiry.Sub(expectedExpiry) > time.Second || expectedExpiry.Sub(actualExpiry) > time.Second {
		t.Errorf("Token expiry time incorrect. Expected around %v, got %v", expectedExpiry, actualExpiry)
	}
}

// Benchmark tests
func BenchmarkGenerateTokens(b *testing.B) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GenerateTokens(userID, testEmail, testRole)
		if err != nil {
			b.Fatalf("Token generation failed: %v", err)
		}
	}
}

func BenchmarkValidateToken(b *testing.B) {
	service := NewJWTService(getTestConfig())
	userID := getTestUserID()

	tokenPair, err := service.GenerateTokens(userID, testEmail, testRole)
	if err != nil {
		b.Fatalf("Failed to generate test token: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ValidateToken(tokenPair.AccessToken)
		if err != nil {
			b.Fatalf("Token validation failed: %v", err)
		}
	}
}
