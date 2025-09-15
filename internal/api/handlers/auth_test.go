package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
)

// MockAuthService simplificado para tests
type MockAuthService struct {
	shouldFailMap map[string]bool
}

func NewMockAuthService() *MockAuthService {
	return &MockAuthService{
		shouldFailMap: make(map[string]bool),
	}
}

func (m *MockAuthService) HashPassword(password string) (string, error) {
	if m.shouldFailMap["HashPassword"] {
		return "", jwt.ErrInvalidToken
	}
	return "hashed_" + password, nil
}

func (m *MockAuthService) CheckPassword(password, hash string) error {
	if m.shouldFailMap["CheckPassword"] {
		return jwt.ErrInvalidToken
	}
	expectedHash := "hashed_" + password
	if hash != expectedHash {
		return jwt.ErrInvalidToken
	}
	return nil
}

func (m *MockAuthService) GenerateTokens(userID string, email string, role string) (accessToken, refreshToken string, err error) {
	if m.shouldFailMap["GenerateTokens"] {
		return "", "", jwt.ErrInvalidToken
	}
	return "mock_access_token_" + userID, "mock_refresh_token_" + userID, nil
}

func (m *MockAuthService) ValidateToken(token string) (*jwt.Claims, error) {
	if m.shouldFailMap["ValidateToken"] {
		return nil, jwt.ErrInvalidToken
	}

	// Parse mock token to extract user ID
	userID := "507f1f77bcf86cd799439011" // Default test user ID
	if len(token) > 17 {
		userID = token[17:] // Extract after "mock_access_token_"
	}

	return &jwt.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   "user",
		Type:   "access",
	}, nil
}

func (m *MockAuthService) SetShouldFail(method string, shouldFail bool) {
	m.shouldFailMap[method] = shouldFail
}

// Helper para crear app de test
func createTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})
	return app
}

// Helper para crear handler de test simplificado
func createTestAuthHandler() (*AuthHandler, *MockAuthService) {
	mockAuthService := NewMockAuthService()
	mockValidator := utils.NewValidator()

	// Crear handler simplificado sin JWT service y sin user repo
	handler := &AuthHandler{
		authService: mockAuthService,
		validator:   mockValidator,
	}

	return handler, mockAuthService
}

func TestRegister_Simple(t *testing.T) {
	tests := []struct {
		name           string
		payload        RegisterRequest
		setupMocks     func(*MockAuthService)
		expectedStatus int
		shouldContain  string
	}{
		{
			name: "Valid registration",
			payload: RegisterRequest{
				Name:     "Test User",
				Email:    "test@example.com",
				Password: "password123",
				Role:     "user",
			},
			setupMocks: func(auth *MockAuthService) {
				// No setup needed for success case
			},
			expectedStatus: 201,
			shouldContain:  "registered",
		},
		{
			name: "Invalid email format",
			payload: RegisterRequest{
				Name:     "Test User",
				Email:    "invalid-email",
				Password: "password123",
				Role:     "user",
			},
			setupMocks:     func(auth *MockAuthService) {},
			expectedStatus: 400,
			shouldContain:  "validation",
		},
		{
			name: "Password too short",
			payload: RegisterRequest{
				Name:     "Test User",
				Email:    "test@example.com",
				Password: "123",
				Role:     "user",
			},
			setupMocks:     func(auth *MockAuthService) {},
			expectedStatus: 400,
			shouldContain:  "validation",
		},
		{
			name: "Password hashing fails",
			payload: RegisterRequest{
				Name:     "Test User",
				Email:    "test@example.com",
				Password: "password123",
				Role:     "user",
			},
			setupMocks: func(auth *MockAuthService) {
				auth.SetShouldFail("HashPassword", true)
			},
			expectedStatus: 500,
			shouldContain:  "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestApp()
			handler, mockAuth := createTestAuthHandler()

			// Setup mocks
			tt.setupMocks(mockAuth)

			// Setup route - usar un handler simplificado que no requiera repositorio
			app.Post("/register", func(c *fiber.Ctx) error {
				var req RegisterRequest
				if err := c.BodyParser(&req); err != nil {
					return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
				}

				// Validar request
				if err := handler.validator.Validate(&req); err != nil {
					return c.Status(400).JSON(fiber.Map{"error": "validation failed"})
				}

				// Hash password
				_, err := handler.authService.HashPassword(req.Password)
				if err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "Failed to process registration"})
				}

				return c.Status(201).JSON(fiber.Map{"message": "User registered successfully"})
			})

			// Create request
			payload, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")

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
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			responseStr := string(body[:n])

			if tt.shouldContain != "" && !contains(responseStr, tt.shouldContain) {
				t.Errorf("Expected response to contain '%s', got: %s", tt.shouldContain, responseStr)
			}
		})
	}
}

func TestLogin_Simple(t *testing.T) {
	tests := []struct {
		name           string
		payload        LoginRequest
		setupMocks     func(*MockAuthService)
		expectedStatus int
		shouldContain  string
	}{
		{
			name: "Valid login",
			payload: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMocks: func(auth *MockAuthService) {
				// No setup needed for success case
			},
			expectedStatus: 200,
			shouldContain:  "token",
		},
		{
			name: "Invalid email format",
			payload: LoginRequest{
				Email:    "invalid-email",
				Password: "password123",
			},
			setupMocks:     func(auth *MockAuthService) {},
			expectedStatus: 400,
			shouldContain:  "validation",
		},
		{
			name: "Password check fails",
			payload: LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			setupMocks: func(auth *MockAuthService) {
				auth.SetShouldFail("CheckPassword", true)
			},
			expectedStatus: 401,
			shouldContain:  "Invalid",
		},
		{
			name: "Token generation fails",
			payload: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMocks: func(auth *MockAuthService) {
				auth.SetShouldFail("GenerateTokens", true)
			},
			expectedStatus: 500,
			shouldContain:  "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestApp()
			handler, mockAuth := createTestAuthHandler()

			// Setup mocks
			tt.setupMocks(mockAuth)

			// Setup route - handler simplificado para login
			app.Post("/login", func(c *fiber.Ctx) error {
				var req LoginRequest
				if err := c.BodyParser(&req); err != nil {
					return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
				}

				// Validar request
				if err := handler.validator.Validate(&req); err != nil {
					return c.Status(400).JSON(fiber.Map{"error": "validation failed"})
				}

				// Simular verificación de password
				hash := "hashed_" + req.Password
				err := handler.authService.CheckPassword(req.Password, hash)
				if err != nil {
					return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
				}

				// Generar tokens
				userID := primitive.NewObjectID().Hex()
				accessToken, refreshToken, err := handler.authService.GenerateTokens(userID, req.Email, "user")
				if err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "Failed to generate tokens"})
				}

				return c.Status(200).JSON(fiber.Map{
					"message": "Login successful",
					"data": fiber.Map{
						"access_token":  accessToken,
						"refresh_token": refreshToken,
					},
				})
			})

			// Create request
			payload, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")

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
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			responseStr := string(body[:n])

			if tt.shouldContain != "" && !contains(responseStr, tt.shouldContain) {
				t.Errorf("Expected response to contain '%s', got: %s", tt.shouldContain, responseStr)
			}
		})
	}
}

func TestValidateToken_Simple(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		setupMocks    func(*MockAuthService)
		expectedValid bool
	}{
		{
			name:  "Valid token",
			token: "mock_access_token_123",
			setupMocks: func(auth *MockAuthService) {
				// Success case
			},
			expectedValid: true,
		},
		{
			name:  "Invalid token",
			token: "invalid_token",
			setupMocks: func(auth *MockAuthService) {
				auth.SetShouldFail("ValidateToken", true)
			},
			expectedValid: false,
		},
		{
			name:          "Empty token",
			token:         "",
			setupMocks:    func(auth *MockAuthService) {},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockAuth := createTestAuthHandler()

			// Setup mocks
			tt.setupMocks(mockAuth)

			// Test token validation
			claims, err := handler.authService.ValidateToken(tt.token)

			if tt.expectedValid {
				if err != nil {
					t.Errorf("Expected valid token, got error: %v", err)
				}
				if claims == nil {
					t.Errorf("Expected claims, got nil")
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for invalid token, got nil")
				}
			}
		})
	}
}

// Helper function
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
func BenchmarkRegister_Simple(b *testing.B) {
	app := createTestApp()
	handler, _ := createTestAuthHandler()

	app.Post("/register", func(c *fiber.Ctx) error {
		var req RegisterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		if err := handler.validator.Validate(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "validation failed"})
		}

		_, err := handler.authService.HashPassword(req.Password)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to process registration"})
		}

		return c.Status(201).JSON(fiber.Map{"message": "User registered successfully"})
	})

	payload := RegisterRequest{
		Name:     "Benchmark User",
		Email:    "bench@example.com",
		Password: "password123",
		Role:     "user",
	}
	payloadBytes, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		app.Test(req)
	}
}
