package middleware

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

type MockJWTService struct {
	shouldFailValidation bool
}

func NewMockJWTService() *MockJWTService {
	return &MockJWTService{shouldFailValidation: false}
}

func (m *MockJWTService) ValidateToken(tokenString string) (*jwt.Claims, error) {
	if m.shouldFailValidation {
		return nil, jwt.ErrInvalidToken
	}
	return &jwt.Claims{
		UserID: "507f1f77bcf86cd799439011",
		Email:  "test@example.com",
		Role:   "user",
		Type:   "access",
	}, nil
}

func (m *MockJWTService) GenerateTokens(userID primitive.ObjectID, email, role string) (*jwt.TokenPair, error) {
	return &jwt.TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(time.Hour),
		ExpiresIn:    3600,
	}, nil
}

func (m *MockJWTService) GenerateTokensWithTTL(userID primitive.ObjectID, email, role string, accessTTL, refreshTTL time.Duration) (*jwt.TokenPair, error) {
	return m.GenerateTokens(userID, email, role)
}

func (m *MockJWTService) RefreshToken(refreshToken string) (*jwt.TokenPair, error) {
	return m.GenerateTokens(primitive.NewObjectID(), "test@example.com", "user")
}

func (m *MockJWTService) RevokeToken(tokenString string) error {
	return nil
}

func (m *MockJWTService) GetTokenClaims(tokenString string) (*jwt.Claims, error) {
	return m.ValidateToken(tokenString)
}

func (m *MockJWTService) IsTokenExpired(tokenString string) bool {
	return false
}

func (m *MockJWTService) SetShouldFailValidation(shouldFail bool) {
	m.shouldFailValidation = shouldFail
}

func TestAuthMiddleware_Basic(t *testing.T) {
	t.Run("create middleware", func(t *testing.T) {
		mockJWT := NewMockJWTService()
		mockLogger := logger.New("debug", "test")

		middleware := NewAuthMiddleware(mockJWT, mockLogger)

		if middleware == nil {
			t.Errorf("Expected middleware to be created, got nil")
		}

		if middleware.jwtService == nil {
			t.Errorf("Expected jwtService to be set, got nil")
		}

		if middleware.logger == nil {
			t.Errorf("Expected logger to be set, got nil")
		}
	})
}

func TestAuthLogic_Basic(t *testing.T) {
	tests := []struct {
		name       string
		userRole   string
		isAdmin    bool
		shouldPass bool
	}{
		{
			name:       "Admin role",
			userRole:   string(models.RoleAdmin),
			isAdmin:    true,
			shouldPass: true,
		},
		{
			name:       "User role",
			userRole:   string(models.RoleUser),
			isAdmin:    false,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualIsAdmin := tt.userRole == string(models.RoleAdmin)

			if actualIsAdmin != tt.isAdmin {
				t.Errorf("Expected isAdmin=%v, got %v", tt.isAdmin, actualIsAdmin)
			}
		})
	}
}
