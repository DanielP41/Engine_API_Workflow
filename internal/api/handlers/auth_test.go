package handlers

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"
)

// MockUserRepository implementación completa para tests
type MockUserRepository struct {
	users         map[string]*models.User
	shouldFailMap map[string]bool
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:         make(map[string]*models.User),
		shouldFailMap: make(map[string]bool),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	if m.shouldFailMap["Create"] {
		return nil, repository.ErrUserAlreadyExists
	}

	for _, existingUser := range m.users {
		if existingUser.Email == user.Email {
			return nil, repository.ErrUserAlreadyExists
		}
	}

	user.ID = primitive.NewObjectID()
	m.users[user.ID.Hex()] = user
	return user, nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	if m.shouldFailMap["GetByID"] {
		return nil, repository.ErrUserNotFound
	}

	user, exists := m.users[id.Hex()]
	if !exists {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.shouldFailMap["GetByEmail"] {
		return nil, repository.ErrUserNotFound
	}

	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (m *MockUserRepository) Update(ctx context.Context, id primitive.ObjectID, update *models.UpdateUserRequest) error {
	if m.shouldFailMap["Update"] {
		return repository.ErrUserNotFound
	}

	user, exists := m.users[id.Hex()]
	if !exists {
		return repository.ErrUserNotFound
	}

	if update.FirstName != "" {
		user.FirstName = update.FirstName
	}
	if update.LastName != "" {
		user.LastName = update.LastName
	}
	if update.Role != "" {
		user.Role = models.Role(update.Role)
	}
	if update.IsActive != nil {
		user.IsActive = *update.IsActive
	}

	return nil
}

func (m *MockUserRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if m.shouldFailMap["Delete"] {
		return repository.ErrUserNotFound
	}

	if _, exists := m.users[id.Hex()]; !exists {
		return repository.ErrUserNotFound
	}

	delete(m.users, id.Hex())
	return nil
}

func (m *MockUserRepository) List(ctx context.Context, page, pageSize int) (*models.UserListResponse, error) {
	if m.shouldFailMap["List"] {
		return nil, repository.ErrUserNotFound
	}

	users := make([]models.User, 0, len(m.users))
	for _, user := range m.users {
		if user.IsActive {
			users = append(users, *user)
		}
	}

	return &models.UserListResponse{
		Users:      users,
		Total:      int64(len(users)),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 1,
	}, nil
}

func (m *MockUserRepository) ListByRole(ctx context.Context, role models.Role, page, pageSize int) (*models.UserListResponse, error) {
	if m.shouldFailMap["ListByRole"] {
		return nil, repository.ErrUserNotFound
	}

	users := make([]models.User, 0)
	for _, user := range m.users {
		if user.IsActive && user.Role == role {
			users = append(users, *user)
		}
	}

	return &models.UserListResponse{
		Users:      users,
		Total:      int64(len(users)),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 1,
	}, nil
}

func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, id primitive.ObjectID) error {
	if m.shouldFailMap["UpdateLastLogin"] {
		return repository.ErrUserNotFound
	}

	if _, exists := m.users[id.Hex()]; !exists {
		return repository.ErrUserNotFound
	}

	return nil
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, id primitive.ObjectID, hashedPassword string) error {
	if m.shouldFailMap["UpdatePassword"] {
		return repository.ErrUserNotFound
	}

	user, exists := m.users[id.Hex()]
	if !exists {
		return repository.ErrUserNotFound
	}

	user.Password = hashedPassword
	return nil
}

func (m *MockUserRepository) SetActiveStatus(ctx context.Context, id primitive.ObjectID, isActive bool) error {
	if m.shouldFailMap["SetActiveStatus"] {
		return repository.ErrUserNotFound
	}

	user, exists := m.users[id.Hex()]
	if !exists {
		return repository.ErrUserNotFound
	}

	user.IsActive = isActive
	return nil
}

func (m *MockUserRepository) Count(ctx context.Context) (int64, error) {
	if m.shouldFailMap["Count"] {
		return 0, repository.ErrUserNotFound
	}

	count := int64(0)
	for _, user := range m.users {
		if user.IsActive {
			count++
		}
	}
	return count, nil
}

func (m *MockUserRepository) CountByRole(ctx context.Context, role models.Role) (int64, error) {
	if m.shouldFailMap["CountByRole"] {
		return 0, repository.ErrUserNotFound
	}

	count := int64(0)
	for _, user := range m.users {
		if user.IsActive && user.Role == role {
			count++
		}
	}
	return count, nil
}

func (m *MockUserRepository) CountUsers(ctx context.Context) (int64, error) {
	if m.shouldFailMap["CountUsers"] {
		return 0, repository.ErrUserNotFound
	}

	return int64(len(m.users)), nil
}

func (m *MockUserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	if m.shouldFailMap["EmailExists"] {
		return false, repository.ErrUserNotFound
	}

	for _, user := range m.users {
		if user.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockUserRepository) EmailExistsExcludeID(ctx context.Context, email string, excludeID primitive.ObjectID) (bool, error) {
	if m.shouldFailMap["EmailExistsExcludeID"] {
		return false, repository.ErrUserNotFound
	}

	for _, user := range m.users {
		if user.Email == email && user.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockUserRepository) GetByIDString(ctx context.Context, id string) (*models.User, error) {
	if m.shouldFailMap["GetByIDString"] {
		return nil, repository.ErrUserNotFound
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, repository.ErrUserNotFound
	}

	return m.GetByID(ctx, objectID)
}

func (m *MockUserRepository) UpdateLastLoginString(ctx context.Context, id string) error {
	if m.shouldFailMap["UpdateLastLoginString"] {
		return repository.ErrUserNotFound
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return repository.ErrUserNotFound
	}

	return m.UpdateLastLogin(ctx, objectID)
}

func (m *MockUserRepository) Search(ctx context.Context, query string, page, pageSize int) (*models.UserListResponse, error) {
	if m.shouldFailMap["Search"] {
		return nil, repository.ErrUserNotFound
	}

	users := make([]models.User, 0)
	for _, user := range m.users {
		if user.IsActive && (user.FirstName == query || user.LastName == query || user.Email == query) {
			users = append(users, *user)
		}
	}

	return &models.UserListResponse{
		Users:      users,
		Total:      int64(len(users)),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 1,
	}, nil
}

func (m *MockUserRepository) SetShouldFail(method string, shouldFail bool) {
	m.shouldFailMap[method] = shouldFail
}

// MockAuthService para tests
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

	userID := "507f1f77bcf86cd799439011"
	if len(token) > 17 {
		userID = token[17:]
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

// MockJWTService para tests
type MockJWTService struct {
	shouldFailMap map[string]bool
}

func NewMockJWTService() *MockJWTService {
	return &MockJWTService{
		shouldFailMap: make(map[string]bool),
	}
}

func (m *MockJWTService) GenerateTokens(userID primitive.ObjectID, email, role string) (*jwt.TokenPair, error) {
	if m.shouldFailMap["GenerateTokens"] {
		return nil, jwt.ErrInvalidToken
	}

	return &jwt.TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(time.Hour),
		ExpiresIn:    3600,
	}, nil
}

func (m *MockJWTService) GenerateTokensWithTTL(userID primitive.ObjectID, email, role string, accessTTL, refreshTTL time.Duration) (*jwt.TokenPair, error) {
	if m.shouldFailMap["GenerateTokensWithTTL"] {
		return nil, jwt.ErrInvalidToken
	}

	return m.GenerateTokens(userID, email, role)
}

func (m *MockJWTService) ValidateToken(tokenString string) (*jwt.Claims, error) {
	if m.shouldFailMap["ValidateToken"] {
		return nil, jwt.ErrInvalidToken
	}

	return &jwt.Claims{
		UserID: "507f1f77bcf86cd799439011",
		Email:  "test@example.com",
		Role:   "user",
		Type:   "access",
	}, nil
}

func (m *MockJWTService) RefreshToken(refreshToken string) (*jwt.TokenPair, error) {
	if m.shouldFailMap["RefreshToken"] {
		return nil, jwt.ErrInvalidToken
	}

	return m.GenerateTokens(primitive.NewObjectID(), "test@example.com", "user")
}

func (m *MockJWTService) RevokeToken(tokenString string) error {
	if m.shouldFailMap["RevokeToken"] {
		return jwt.ErrInvalidToken
	}
	return nil
}

func (m *MockJWTService) GetTokenClaims(tokenString string) (*jwt.Claims, error) {
	if m.shouldFailMap["GetTokenClaims"] {
		return nil, jwt.ErrInvalidToken
	}

	return &jwt.Claims{
		UserID: "507f1f77bcf86cd799439011",
		Email:  "test@example.com",
		Role:   "user",
		Type:   "access",
	}, nil
}

func (m *MockJWTService) IsTokenExpired(tokenString string) bool {
	return m.shouldFailMap["IsTokenExpired"]
}

func (m *MockJWTService) SetShouldFail(method string, shouldFail bool) {
	m.shouldFailMap[method] = shouldFail
}

// Helper para crear handler de test con configuración completa
func createTestAuthHandler() (*AuthHandler, *MockAuthService, *MockUserRepository) {
	mockAuthService := NewMockAuthService()
	mockUserRepo := NewMockUserRepository()
	mockJWTService := NewMockJWTService()
	mockValidator := utils.NewValidator()
	mockLogger := logger.New("debug", "test")

	handler := NewAuthHandlerWithConfig(AuthHandlerConfig{
		UserRepo:    mockUserRepo,
		AuthService: mockAuthService,
		JWTService:  mockJWTService,
		Validator:   mockValidator,
		Logger:      mockLogger,
	})

	return handler, mockAuthService, mockUserRepo
}

// Tests básicos sin HTTP
func TestRegister_Basic(t *testing.T) {
	t.Run("simple register test", func(t *testing.T) {
		mockAuth := NewMockAuthService()
		mockRepo := NewMockUserRepository()

		hashedPassword, err := mockAuth.HashPassword("password123")
		if err != nil {
			t.Errorf("Expected password hash to succeed, got error: %v", err)
		}
		if hashedPassword == "" {
			t.Errorf("Expected hashed password, got empty string")
		}

		user := &models.User{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Password:  hashedPassword,
			Role:      models.RoleUser,
			IsActive:  true,
		}

		createdUser, err := mockRepo.Create(context.Background(), user)
		if err != nil {
			t.Errorf("Expected user creation to succeed, got error: %v", err)
		}
		if createdUser == nil {
			t.Errorf("Expected created user, got nil")
		}
	})
}

func TestLogin_Basic(t *testing.T) {
	t.Run("simple login test", func(t *testing.T) {
		mockAuth := NewMockAuthService()
		mockRepo := NewMockUserRepository()

		user := &models.User{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Password:  "hashed_password123",
			Role:      models.RoleUser,
			IsActive:  true,
		}
		createdUser, err := mockRepo.Create(context.Background(), user)
		if err != nil {
			t.Errorf("Failed to create test user: %v", err)
			return
		}

		foundUser, err := mockRepo.GetByEmail(context.Background(), "test@example.com")
		if err != nil {
			t.Errorf("Expected to find user, got error: %v", err)
		}
		if foundUser.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", foundUser.Email)
		}

		err = mockAuth.CheckPassword("password123", "hashed_password123")
		if err != nil {
			t.Errorf("Expected password check to succeed, got error: %v", err)
		}

		accessToken, refreshToken, err := mockAuth.GenerateTokens(createdUser.ID.Hex(), createdUser.Email, string(createdUser.Role))
		if err != nil {
			t.Errorf("Expected token generation to succeed, got error: %v", err)
		}
		if accessToken == "" || refreshToken == "" {
			t.Errorf("Expected tokens to be generated, got empty tokens")
		}
	})
}

func TestAuthHandler_Creation(t *testing.T) {
	t.Run("test handler creation", func(t *testing.T) {
		handler, _, _ := createTestAuthHandler()

		if handler == nil {
			t.Errorf("Expected handler to be created, got nil")
		}

		if handler.authService == nil {
			t.Errorf("Expected authService to be set, got nil")
		}

		if handler.userRepo == nil {
			t.Errorf("Expected userRepo to be set, got nil")
		}

		if handler.validator == nil {
			t.Errorf("Expected validator to be set, got nil")
		}

		if handler.logger == nil {
			t.Errorf("Expected logger to be set, got nil")
		}

		if handler.jwtService == nil {
			t.Errorf("Expected jwtService to be set, got nil")
		}
	})
}

func TestGenerateUserTokens(t *testing.T) {
	t.Run("test generateUserTokens method", func(t *testing.T) {
		handler, _, _ := createTestAuthHandler()

		user := &models.User{
			ID:        primitive.NewObjectID(),
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Role:      models.RoleUser,
			IsActive:  true,
		}

		tokens, err := handler.generateUserTokens(user, false)

		if err != nil {
			t.Errorf("Expected generateUserTokens to succeed, got error: %v", err)
		}

		if tokens == nil {
			t.Errorf("Expected tokens, got nil")
		}

		if tokens.AccessToken == "" {
			t.Errorf("Expected access token, got empty")
		}

		if tokens.RefreshToken == "" {
			t.Errorf("Expected refresh token, got empty")
		}
	})
}
