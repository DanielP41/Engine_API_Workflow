package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

// MockJWTService para tests de middleware
type MockJWTService struct {
	shouldFailValidation bool
	mockClaims           *jwt.Claims
}

func NewMockJWTService() *MockJWTService {
	return &MockJWTService{
		mockClaims: &jwt.Claims{
			UserID: "507f1f77bcf86cd799439011",
			Email:  "test@example.com",
			Role:   "user",
			Type:   "access",
		},
	}
}

func (m *MockJWTService) GenerateTokens(userID primitive.ObjectID, email, role string) (*jwt.TokenPair, error) {
	return &jwt.TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
		ExpiresIn:    900,
	}, nil
}

func (m *MockJWTService) GenerateTokensWithTTL(userID primitive.ObjectID, email, role string, accessTTL, refreshTTL time.Duration) (*jwt.TokenPair, error) {
	return m.GenerateTokens(userID, email, role)
}

func (m *MockJWTService) ValidateToken(tokenString string) (*jwt.Claims, error) {
	if m.shouldFailValidation {
		return nil, jwt.ErrInvalidToken
	}
	return m.mockClaims, nil
}

func (m *MockJWTService) RefreshToken(refreshToken string) (*jwt.TokenPair, error) {
	return m.GenerateTokens(primitive.NewObjectID(), "refresh@example.com", "user")
}

func (m *MockJWTService) RevokeToken(tokenString string) error {
	return nil
}

func (m *MockJWTService) GetTokenClaims(tokenString string) (*jwt.Claims, error) {
	return m.ValidateToken(tokenString)
}

func (m *MockJWTService) IsTokenExpired(tokenString string) bool {
	return m.shouldFailValidation
}

func (m *MockJWTService) SetShouldFailValidation(shouldFail bool) {
	m.shouldFailValidation = shouldFail
}

func (m *MockJWTService) SetMockClaims(claims *jwt.Claims) {
	m.mockClaims = claims
}

// MockTokenBlacklist para tests
type MockTokenBlacklist struct {
	blacklistedTokens map[string]bool
	shouldFail        bool
}

func NewMockTokenBlacklist() *MockTokenBlacklist {
	return &MockTokenBlacklist{
		blacklistedTokens: make(map[string]bool),
		shouldFail:        false,
	}
}

func (m *MockTokenBlacklist) IsBlacklisted(ctx interface{}, token string) bool {
	if m.shouldFail {
		return true
	}
	return m.blacklistedTokens[token]
}

func (m *MockTokenBlacklist) BlacklistToken(ctx interface{}, token string) error {
	if m.shouldFail {
		return jwt.ErrInvalidToken
	}
	m.blacklistedTokens[token] = true
	return nil
}

func (m *MockTokenBlacklist) BlacklistTokenWithReason(ctx interface{}, token, reason string) error {
	return m.BlacklistToken(ctx, token)
}

func (m *MockTokenBlacklist) SetShouldFail(shouldFail bool) {
	m.shouldFail = shouldFail
}

func (m *MockTokenBlacklist) AddToBlacklist(token string) {
	m.blacklistedTokens[token] = true
}

// Helper para crear app de test
func createTestAppForMiddleware() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})
}

// Helper para crear middleware de test
func createTestAuthMiddleware() (*AuthMiddleware, *MockJWTService, *MockTokenBlacklist) {
	mockJWT := NewMockJWTService()
	mockBlacklist := NewMockTokenBlacklist()
	mockLogger := logger.New(logger.Config{Level: "debug"})

	middleware := NewAuthMiddlewareWithBlacklist(mockJWT, mockBlacklist, mockLogger)
	return middleware, mockJWT, mockBlacklist
}

func TestRequireAuth(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		setupMocks     func(*MockJWTService, *MockTokenBlacklist)
		expectedStatus int
		shouldContain  string
	}{
		{
			name:       "Valid token",
			authHeader: "Bearer valid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
			},
			expectedStatus: 200,
			shouldContain:  "success",
		},
		{
			name:           "Missing authorization header",
			authHeader:     "",
			setupMocks:     func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {},
			expectedStatus: 401,
			shouldContain:  "Authorization header required",
		},
		{
			name:           "Invalid authorization format",
			authHeader:     "InvalidFormat token",
			setupMocks:     func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {},
			expectedStatus: 401,
			shouldContain:  "Authorization header required",
		},
		{
			name:       "Invalid token",
			authHeader: "Bearer invalid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(true)
			},
			expectedStatus: 401,
			shouldContain:  "Invalid token",
		},
		{
			name:       "Blacklisted token",
			authHeader: "Bearer blacklisted_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
				blacklist.AddToBlacklist("blacklisted_token")
			},
			expectedStatus: 401,
			shouldContain:  "Token has been revoked",
		},
		{
			name:       "Refresh token instead of access token",
			authHeader: "Bearer refresh_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
				jwt.SetMockClaims(&jwt.Claims{
					UserID: "507f1f77bcf86cd799439011",
					Email:  "test@example.com",
					Role:   "user",
					Type:   "refresh", // Wrong token type
				})
			},
			expectedStatus: 401,
			shouldContain:  "Access token required",
		},
		{
			name:       "Invalid user ID in token",
			authHeader: "Bearer valid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
				jwt.SetMockClaims(&jwt.Claims{
					UserID: "invalid_object_id",
					Email:  "test@example.com",
					Role:   "user",
					Type:   "access",
				})
			},
			expectedStatus: 401,
			shouldContain:  "Invalid user ID in token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestAppForMiddleware()
			middleware, mockJWT, mockBlacklist := createTestAuthMiddleware()

			// Setup mocks
			tt.setupMocks(mockJWT, mockBlacklist)

			// Setup protected route
			app.Use(middleware.RequireAuth())
			app.Get("/protected", func(c *fiber.Ctx) error {
				return c.JSON(fiber.Map{"message": "success"})
			})

			// Create request
			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Check response content if needed
			if tt.shouldContain != "" {
				body := make([]byte, 1024)
				n, _ := resp.Body.Read(body)
				responseStr := string(body[:n])

				if !contains(responseStr, tt.shouldContain) {
					t.Errorf("Expected response to contain '%s', got: %s", tt.shouldContain, responseStr)
				}
			}
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name           string
		userRole       string
		hasAuth        bool
		expectedStatus int
		shouldContain  string
	}{
		{
			name:           "Valid admin access",
			userRole:       string(models.RoleAdmin),
			hasAuth:        true,
			expectedStatus: 200,
			shouldContain:  "success",
		},
		{
			name:           "User role trying admin access",
			userRole:       string(models.RoleUser),
			hasAuth:        true,
			expectedStatus: 403,
			shouldContain:  "Admin access required",
		},
		{
			name:           "No authentication",
			userRole:       "",
			hasAuth:        false,
			expectedStatus: 401,
			shouldContain:  "Authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestAppForMiddleware()
			middleware, _, _ := createTestAuthMiddleware()

			// Setup admin route
			app.Use(middleware.RequireAdmin())
			app.Get("/admin", func(c *fiber.Ctx) error {
				return c.JSON(fiber.Map{"message": "success"})
			})

			// Create request with simulated auth context
			req := httptest.NewRequest("GET", "/admin", nil)

			// Test with middleware simulation
			if tt.hasAuth {
				app.Use(func(c *fiber.Ctx) error {
					c.Locals("userRole", tt.userRole)
					return c.Next()
				})
			}

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name           string
		allowedRoles   []string
		userRole       string
		hasAuth        bool
		expectedStatus int
		shouldContain  string
	}{
		{
			name:           "User with allowed role",
			allowedRoles:   []string{"user", "admin"},
			userRole:       "user",
			hasAuth:        true,
			expectedStatus: 200,
			shouldContain:  "success",
		},
		{
			name:           "Admin with allowed role",
			allowedRoles:   []string{"admin"},
			userRole:       "admin",
			hasAuth:        true,
			expectedStatus: 200,
			shouldContain:  "success",
		},
		{
			name:           "User with disallowed role",
			allowedRoles:   []string{"admin"},
			userRole:       "user",
			hasAuth:        true,
			expectedStatus: 403,
			shouldContain:  "Insufficient permissions",
		},
		{
			name:           "No authentication",
			allowedRoles:   []string{"user"},
			userRole:       "",
			hasAuth:        false,
			expectedStatus: 401,
			shouldContain:  "Authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestAppForMiddleware()
			middleware, _, _ := createTestAuthMiddleware()

			// Setup role-based route
			app.Use(middleware.RequireRole(tt.allowedRoles...))
			app.Get("/role-protected", func(c *fiber.Ctx) error {
				return c.JSON(fiber.Map{"message": "success"})
			})

			// Simulate auth context if needed
			if tt.hasAuth {
				app.Use(func(c *fiber.Ctx) error {
					c.Locals("userRole", tt.userRole)
					return c.Next()
				})
			}

			// Create request
			req := httptest.NewRequest("GET", "/role-protected", nil)

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestOptionalAuth(t *testing.T) {
	tests := []struct {
		name         string
		authHeader   string
		setupMocks   func(*MockJWTService, *MockTokenBlacklist)
		shouldSetCtx bool
	}{
		{
			name:       "Valid token sets context",
			authHeader: "Bearer valid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
			},
			shouldSetCtx: true,
		},
		{
			name:       "Invalid token doesn't set context",
			authHeader: "Bearer invalid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(true)
			},
			shouldSetCtx: false,
		},
		{
			name:         "No token doesn't set context",
			authHeader:   "",
			setupMocks:   func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {},
			shouldSetCtx: false,
		},
		{
			name:       "Blacklisted token doesn't set context",
			authHeader: "Bearer blacklisted_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
				blacklist.AddToBlacklist("blacklisted_token")
			},
			shouldSetCtx: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestAppForMiddleware()
			middleware, mockJWT, mockBlacklist := createTestAuthMiddleware()

			// Setup mocks
			tt.setupMocks(mockJWT, mockBlacklist)

			// Setup optional auth route
			app.Use(middleware.OptionalAuth())
			app.Get("/optional", func(c *fiber.Ctx) error {
				userID := c.Locals("userID")
				hasAuth := userID != nil
				return c.JSON(fiber.Map{
					"has_auth": hasAuth,
					"user_id":  userID,
				})
			})

			// Create request
			req := httptest.NewRequest("GET", "/optional", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Should always return 200 for optional auth
			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Check if context was set correctly
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			responseStr := string(body[:n])

			if tt.shouldSetCtx {
				if !contains(responseStr, `"has_auth":true`) {
					t.Errorf("Expected auth context to be set, but it wasn't")
				}
			} else {
				if !contains(responseStr, `"has_auth":false`) {
					t.Errorf("Expected auth context not to be set, but it was")
				}
			}
		})
	}
}

func TestCreateLogoutHandler(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		setupMocks     func(*MockJWTService, *MockTokenBlacklist)
		expectedStatus int
		shouldContain  string
	}{
		{
			name:       "Successful logout with blacklist",
			authHeader: "Bearer valid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
				blacklist.SetShouldFail(false)
			},
			expectedStatus: 200,
			shouldContain:  "Logged out successfully",
		},
		{
			name:           "Logout without token",
			authHeader:     "",
			setupMocks:     func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {},
			expectedStatus: 200,
			shouldContain:  "Logged out successfully",
		},
		{
			name:       "Blacklist failure",
			authHeader: "Bearer valid_token",
			setupMocks: func(jwt *MockJWTService, blacklist *MockTokenBlacklist) {
				jwt.SetShouldFailValidation(false)
				blacklist.SetShouldFail(true)
			},
			expectedStatus: 500,
			shouldContain:  "Logout failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestAppForMiddleware()
			middleware, mockJWT, mockBlacklist := createTestAuthMiddleware()

			// Setup mocks
			tt.setupMocks(mockJWT, mockBlacklist)

			// Setup logout route
			app.Post("/logout", middleware.CreateLogoutHandler())

			// Create request
			req := httptest.NewRequest("POST", "/logout", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Check response content
			if tt.shouldContain != "" {
				body := make([]byte, 1024)
				n, _ := resp.Body.Read(body)
				responseStr := string(body[:n])

				if !contains(responseStr, tt.shouldContain) {
					t.Errorf("Expected response to contain '%s', got: %s", tt.shouldContain, responseStr)
				}
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("GetCurrentUserID", func(t *testing.T) {
		app := createTestAppForMiddleware()

		app.Get("/test", func(c *fiber.Ctx) error {
			testObjID := primitive.NewObjectID()
			c.Locals("userID", testObjID)

			userID, err := GetCurrentUserID(c)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if userID != testObjID {
				return c.Status(500).JSON(fiber.Map{"error": "userID mismatch"})
			}

			return c.JSON(fiber.Map{"success": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("IsAuthenticated", func(t *testing.T) {
		app := createTestAppForMiddleware()

		app.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("authenticated", true)
			isAuth := IsAuthenticated(c)
			return c.JSON(fiber.Map{"is_authenticated": isAuth})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		responseStr := string(body[:n])

		if !contains(responseStr, `"is_authenticated":true`) {
			t.Errorf("Expected is_authenticated to be true")
		}
	})

	t.Run("IsAdmin", func(t *testing.T) {
		app := createTestAppForMiddleware()

		app.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("is_admin", true)
			isAdminResult := IsAdmin(c)
			return c.JSON(fiber.Map{"is_admin": isAdminResult})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		responseStr := string(body[:n])

		if !contains(responseStr, `"is_admin":true`) {
			t.Errorf("Expected is_admin to be true")
		}
	})

	t.Run("GetCurrentUserRole", func(t *testing.T) {
		app := createTestAppForMiddleware()

		app.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("userRole", "admin")
			role, err := GetCurrentUserRole(c)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"role": role})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		responseStr := string(body[:n])

		if !contains(responseStr, `"role":"admin"`) {
			t.Errorf("Expected role to be admin")
		}
	})
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findInString(s, substr)
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests
func BenchmarkRequireAuth(b *testing.B) {
	app := createTestAppForMiddleware()
	middleware, mockJWT, _ := createTestAuthMiddleware()

	mockJWT.SetShouldFailValidation(false)

	app.Use(middleware.RequireAuth())
	app.Get("/protected", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid_token")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Test(req)
	}
}

func BenchmarkOptionalAuth(b *testing.B) {
	app := createTestAppForMiddleware()
	middleware, mockJWT, _ := createTestAuthMiddleware()

	mockJWT.SetShouldFailValidation(false)

	app.Use(middleware.OptionalAuth())
	app.Get("/optional", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	req := httptest.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer valid_token")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Test(req)
	}
}
