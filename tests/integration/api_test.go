package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockWorkflowService mocks services.WorkflowService
type MockWorkflowService struct {
	mock.Mock
}

func (m *MockWorkflowService) Create(ctx context.Context, req *models.CreateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	args := m.Called(ctx, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkflowResponse), args.Error(1)
}

func (m *MockWorkflowService) GetByID(ctx context.Context, workflowID primitive.ObjectID) (*models.Workflow, error) {
	args := m.Called(ctx, workflowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}

// Add other methods as needed, stubbing them to satisfy interface
func (m *MockWorkflowService) Search(ctx context.Context, filters repository.WorkflowSearchFilters, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	return nil, 0, nil
}
func (m *MockWorkflowService) Update(ctx context.Context, workflowID primitive.ObjectID, req *models.UpdateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	return nil, nil
}
func (m *MockWorkflowService) Delete(ctx context.Context, workflowID primitive.ObjectID) error {
	return nil
}
func (m *MockWorkflowService) ToggleStatus(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	return nil, nil
}
func (m *MockWorkflowService) Clone(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID, name, description string) (*models.WorkflowResponse, error) {
	return nil, nil
}
func (m *MockWorkflowService) GetUserWorkflows(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	return nil, 0, nil
}
func (m *MockWorkflowService) GetActiveWorkflows(ctx context.Context, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	return nil, 0, nil
}
func (m *MockWorkflowService) ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	return nil
}

// Fix method signatures to match interface closely if strict
// For interface{}, we might need real types from repository package if imported.
// But mostly validation of imports is key.
// Ignoring strict signature match for "interface{}" arguments in this mock for brevity,
// assuming unused in this specific test.

func TestCreateWorkflowAPI(t *testing.T) {
	// Setup
	app := fiber.New()
	mockService := new(MockWorkflowService)

	// Create handler with mock service
	// Note: NewWorkflowHandler needs LogService and Validator too.
	// We can pass nil if key paths don't use them, or mock them.
	// Earlier we saw NewWorkflowHandler(workflowService, logService, validator)
	// We'll pass nil for logService and validator for now assuming basic creation doesn't crash
	// (create usually validates, so validator might be needed).
	// Ideally we should mock validator, but for integration test on API level,
	// we can rely on handler calling service.
	// Create validator
	validator := *utils.NewValidator() // Dereference because NewWorkflowHandler expects value

	// Create handler with mock service
	workflowHandler := handlers.NewWorkflowHandler(mockService, nil, validator)

	// Setup Routes (bypassing Auth for test)
	app.Post("/api/v1/workflows", func(c *fiber.Ctx) error {
		// Mock Auth UserID in context
		c.Locals("userID", primitive.NewObjectID().Hex())
		return workflowHandler.CreateWorkflow(c)
	})

	t.Run("Success", func(t *testing.T) {
		reqBody := models.CreateWorkflowRequest{
			Name: "Integration Test Workflow",
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

		expectedResp := &models.WorkflowResponse{
			ID:     primitive.NewObjectID(),
			Name:   reqBody.Name,
			Status: "draft",
		}

		mockService.On("Create", mock.Anything, mock.AnythingOfType("*models.CreateWorkflowRequest"), mock.AnythingOfType("primitive.ObjectID")).Return(expectedResp, nil)

		bodyPtr, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/workflows", bytes.NewReader(bodyPtr))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)

		assert.Equal(t, 201, resp.StatusCode)
	})
}
