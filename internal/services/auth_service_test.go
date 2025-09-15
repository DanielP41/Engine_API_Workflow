package services

import (
	"testing"
	"time"

	"Engine_API_Workflow/pkg/jwt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Helper para crear AuthService de test con JWT service configurado
func createTestAuthService() *AuthServiceImpl {
	// Crear JWT service con configuración de test
	jwtConfig := jwt.Config{
		SecretKey:       "test-secret-key-minimum-32-characters",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test-issuer",
		Audience:        "test-audience",
	}
	jwtService := jwt.NewJWTService(jwtConfig)

	return &AuthServiceImpl{
		jwtService: jwtService,
		userRepo:   nil, // No necesitamos repo para estos tests
	}
}

// Test simplificado que se enfoca en la lógica del AuthService sin usar repositorio
func TestAuthService_HashPassword_Simple(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		expectErr bool
	}{
		{
			name:      "Valid password",
			password:  "validpassword123",
			expectErr: false,
		},
		{
			name:      "Empty password",
			password:  "",
			expectErr: false, // bcrypt puede hashear string vacío
		},
		{
			name:      "Long password",
			password:  "very-long-password-with-special-characters-!@#$%^&*()",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Usar helper para crear AuthService configurado
			authService := createTestAuthService()

			hash, err := authService.HashPassword(tt.password)

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

			// Verificar que el hash no está vacío
			if hash == "" {
				t.Errorf("Expected non-empty hash, got empty string")
			}

			// Verificar que el hash es diferente a la password original
			if hash == tt.password {
				t.Errorf("Hash should be different from original password")
			}

			// Verificar que es un hash bcrypt válido
			err = authService.CheckPassword(tt.password, hash)
			if err != nil {
				t.Errorf("Generated hash is not valid bcrypt hash: %v", err)
			}
		})
	}
}

func TestAuthService_CheckPassword_Simple(t *testing.T) {
	authService := createTestAuthService()

	// Crear password hash válido
	password := "testpassword123"
	hash, err := authService.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to create test hash: %v", err)
	}

	tests := []struct {
		name      string
		password  string
		hash      string
		expectErr bool
	}{
		{
			name:      "Correct password",
			password:  password,
			hash:      hash,
			expectErr: false,
		},
		{
			name:      "Incorrect password",
			password:  "wrongpassword",
			hash:      hash,
			expectErr: true,
		},
		{
			name:      "Empty password",
			password:  "",
			hash:      hash,
			expectErr: true,
		},
		{
			name:      "Invalid hash",
			password:  password,
			hash:      "invalid-hash",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authService.CheckPassword(tt.password, tt.hash)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Test que no requiere repositorio - solo testing de tokens
func TestAuthService_ValidateToken_Simple(t *testing.T) {
	authService := createTestAuthService()

	// Generar token válido usando el servicio directamente
	testUserID := primitive.NewObjectID()
	testEmail := "test@example.com"
	testRole := "user"

	accessToken, _, err := authService.GenerateTokens(testUserID.Hex(), testEmail, testRole)
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
			token:     accessToken,
			expectErr: false,
		},
		{
			name:      "Empty token",
			token:     "",
			expectErr: true,
		},
		{
			name:      "Invalid token",
			token:     "invalid.token.format",
			expectErr: true,
		},
		{
			name:      "Malformed token",
			token:     "header.payload",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := authService.ValidateToken(tt.token)

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

			// Verificar claims válidos
			if claims.UserID != testUserID.Hex() {
				t.Errorf("Expected UserID %s, got %s", testUserID.Hex(), claims.UserID)
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

// Benchmark tests simplificados
func BenchmarkAuthService_HashPassword(b *testing.B) {
	authService := createTestAuthService()
	password := "benchmark-password-123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := authService.HashPassword(password)
		if err != nil {
			b.Fatalf("Password hashing failed: %v", err)
		}
	}
}

func BenchmarkAuthService_CheckPassword(b *testing.B) {
	authService := createTestAuthService()
	password := "benchmark-password-123"

	hash, err := authService.HashPassword(password)
	if err != nil {
		b.Fatalf("Failed to create test hash: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := authService.CheckPassword(password, hash)
		if err != nil {
			b.Fatalf("Password verification failed: %v", err)
		}
	}
}

func BenchmarkAuthService_GenerateTokens(b *testing.B) {
	authService := createTestAuthService()

	userID := primitive.NewObjectID()
	email := "bench@example.com"
	role := "user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := authService.GenerateTokens(userID.Hex(), email, role)
		if err != nil {
			b.Fatalf("Token generation failed: %v", err)
		}
	}
}
