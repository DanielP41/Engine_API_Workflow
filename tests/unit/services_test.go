package unit

import (
	"context"
	"testing"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockWorkflowRepository mocks repository.WorkflowRepository
type MockWorkflowRepository struct {
	mock.Mock
}

func (m *MockWorkflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func (m *MockWorkflowRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Workflow, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) Update(ctx context.Context, workflowID primitive.ObjectID, update map[string]interface{}) error {
	args := m.Called(ctx, workflowID, update)
	return args.Error(0)
}

func (m *MockWorkflowRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockWorkflowRepository) List(ctx context.Context, page, pageSize int) (*models.WorkflowListResponse, error) {
	args := m.Called(ctx, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkflowListResponse), args.Error(1)
}
func (m *MockWorkflowRepository) GetByName(ctx context.Context, name string) (*models.Workflow, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}
func (m *MockWorkflowRepository) Search(ctx context.Context, query string, page, pageSize int) (*models.WorkflowListResponse, error) {
	args := m.Called(ctx, query, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkflowListResponse), args.Error(1)
}
func (m *MockWorkflowRepository) ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.WorkflowListResponse, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkflowListResponse), args.Error(1)
}
func (m *MockWorkflowRepository) GetActiveWorkflows(ctx context.Context) ([]*models.Workflow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*models.Workflow), args.Error(1)
}

// Missing methods for WorkflowRepository interface
func (m *MockWorkflowRepository) ListByStatus(ctx context.Context, status models.WorkflowStatus, page, pageSize int) (*models.WorkflowListResponse, error) {
	return nil, nil
}
func (m *MockWorkflowRepository) SearchByUser(ctx context.Context, userID primitive.ObjectID, query string, page, pageSize int) (*models.WorkflowListResponse, error) {
	return nil, nil
}
func (m *MockWorkflowRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.WorkflowStatus) error {
	return nil
}
func (m *MockWorkflowRepository) UpdateRunStats(ctx context.Context, id primitive.ObjectID, success bool) error {
	return nil
}
func (m *MockWorkflowRepository) GetWorkflowsByTriggerType(ctx context.Context, triggerType models.TriggerType) ([]*models.Workflow, error) {
	return nil, nil
}
func (m *MockWorkflowRepository) CreateVersion(ctx context.Context, workflow *models.Workflow) error {
	return nil
}
func (m *MockWorkflowRepository) GetVersions(ctx context.Context, workflowID primitive.ObjectID) ([]*models.Workflow, error) {
	return nil, nil
}
func (m *MockWorkflowRepository) ListByTags(ctx context.Context, tags []string, page, pageSize int) (*models.WorkflowListResponse, error) {
	return nil, nil
}
func (m *MockWorkflowRepository) GetAllTags(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *MockWorkflowRepository) Count(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *MockWorkflowRepository) CountByUser(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	return 0, nil
}
func (m *MockWorkflowRepository) CountByStatus(ctx context.Context, status models.WorkflowStatus) (int64, error) {
	return 0, nil
}
func (m *MockWorkflowRepository) CountWorkflows(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *MockWorkflowRepository) CountActiveWorkflows(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *MockWorkflowRepository) NameExistsForUser(ctx context.Context, name string, userID primitive.ObjectID) (bool, error) {
	return false, nil
}
func (m *MockWorkflowRepository) NameExistsForUserExcludeID(ctx context.Context, name string, userID primitive.ObjectID, excludeID primitive.ObjectID) (bool, error) {
	return false, nil
}

// MockUserRepository mocks repository.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	args := m.Called(ctx, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, id primitive.ObjectID, update *models.UpdateUserRequest) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}
func (m *MockUserRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockUserRepository) List(ctx context.Context, limit, offset int) (*models.UserListResponse, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserListResponse), args.Error(1)
}
func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// Missing methods for UserRepository interface
func (m *MockUserRepository) GetByIDString(ctx context.Context, id string) (*models.User, error) {
	return nil, nil
}
func (m *MockUserRepository) UpdateLastLoginString(ctx context.Context, id string) error {
	return nil
}
func (m *MockUserRepository) Search(ctx context.Context, query string, page, pageSize int) (*models.UserListResponse, error) {
	return nil, nil
}
func (m *MockUserRepository) ListByRole(ctx context.Context, role models.Role, page, pageSize int) (*models.UserListResponse, error) {
	return nil, nil
}
func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, id primitive.ObjectID) error {
	return nil
}
func (m *MockUserRepository) UpdatePassword(ctx context.Context, id primitive.ObjectID, hashedPassword string) error {
	return nil
}
func (m *MockUserRepository) SetActiveStatus(ctx context.Context, id primitive.ObjectID, isActive bool) error {
	return nil
}
func (m *MockUserRepository) Count(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *MockUserRepository) CountByRole(ctx context.Context, role models.Role) (int64, error) {
	return 0, nil
}
func (m *MockUserRepository) CountUsers(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *MockUserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	return false, nil
}
func (m *MockUserRepository) EmailExistsExcludeID(ctx context.Context, email string, excludeID primitive.ObjectID) (bool, error) {
	return false, nil
}

// TestWorkflowService_Create tests the creation of a workflow
func TestWorkflowService_Create(t *testing.T) {
	// Setup
	mockRepo := new(MockWorkflowRepository)
	mockUserRepo := new(MockUserRepository)

	// Crear instancia del servicio corregida
	service := services.NewWorkflowService(mockRepo, mockUserRepo)

	ctx := context.TODO()
	userID := primitive.NewObjectID()
	user := &models.User{
		ID:        userID,
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("Success", func(t *testing.T) {
		req := &models.CreateWorkflowRequest{
			Name: "Test Workflow",
			Steps: []models.WorkflowStep{
				{
					ID:   "step-1",
					Name: "Step 1",
					Type: "http",
				},
			},
			Triggers: []models.WorkflowTrigger{
				{Type: "manual"},
			},
		}

		// Configurar mocks
		mockUserRepo.On("GetByID", ctx, userID).Return(user, nil)
		mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Workflow")).Return(nil)

		resp, err := service.Create(ctx, req, userID)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, req.Name, resp.Name)
		assert.NotEmpty(t, resp.ID)

		mockRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		req := &models.CreateWorkflowRequest{
			Name: "", // Inválido
		}

		mockUserRepo.On("GetByID", ctx, userID).Return(user, nil)

		resp, err := service.Create(ctx, req, userID)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
