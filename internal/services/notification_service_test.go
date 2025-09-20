package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// ===============================================
// MOCKS
// ===============================================

// MockNotificationRepository mock del repositorio de notificaciones
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, notification *models.EmailNotification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailNotification, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.EmailNotification), args.Error(1)
}

func (m *MockNotificationRepository) Update(ctx context.Context, notification *models.EmailNotification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationRepository) List(ctx context.Context, filters map[string]interface{}, opts *repository.PaginationOptions) ([]*models.EmailNotification, int64, error) {
	args := m.Called(ctx, filters, opts)
	return args.Get(0).([]*models.EmailNotification), args.Get(1).(int64), args.Error(2)
}

func (m *MockNotificationRepository) GetPending(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*models.EmailNotification), args.Error(1)
}

func (m *MockNotificationRepository) GetScheduled(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*models.EmailNotification), args.Error(1)
}

func (m *MockNotificationRepository) GetFailedForRetry(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*models.EmailNotification), args.Error(1)
}

func (m *MockNotificationRepository) GetStats(ctx context.Context, timeRange time.Duration) (*models.NotificationStats, error) {
	args := m.Called(ctx, timeRange)
	return args.Get(0).(*models.NotificationStats), args.Error(1)
}

func (m *MockNotificationRepository) CleanupOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

// MockTemplateRepository mock del repositorio de templates
type MockTemplateRepository struct {
	mock.Mock
}

func (m *MockTemplateRepository) Create(ctx context.Context, template *models.EmailTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockTemplateRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailTemplate, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.EmailTemplate), args.Error(1)
}

func (m *MockTemplateRepository) GetByName(ctx context.Context, name string) (*models.EmailTemplate, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(*models.EmailTemplate), args.Error(1)
}

func (m *MockTemplateRepository) Update(ctx context.Context, template *models.EmailTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockTemplateRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTemplateRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.EmailTemplate, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*models.EmailTemplate), args.Error(1)
}

func (m *MockTemplateRepository) ListByType(ctx context.Context, notificationType models.NotificationType) ([]*models.EmailTemplate, error) {
	args := m.Called(ctx, notificationType)
	return args.Get(0).([]*models.EmailTemplate), args.Error(1)
}

func (m *MockTemplateRepository) CreateVersion(ctx context.Context, template *models.EmailTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockTemplateRepository) GetVersions(ctx context.Context, templateName string) ([]*models.EmailTemplate, error) {
	args := m.Called(ctx, templateName)
	return args.Get(0).([]*models.EmailTemplate), args.Error(1)
}

func (m *MockTemplateRepository) GetLatestVersion(ctx context.Context, templateName string) (*models.EmailTemplate, error) {
	args := m.Called(ctx, templateName)
	return args.Get(0).(*models.EmailTemplate), args.Error(1)
}

func (m *MockTemplateRepository) SetActive(ctx context.Context, id primitive.ObjectID, active bool) error {
	args := m.Called(ctx, id, active)
	return args.Error(0)
}

func (m *MockTemplateRepository) GetActive(ctx context.Context) ([]*models.EmailTemplate, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*models.EmailTemplate), args.Error(1)
}

// MockEmailService mock del servicio de email
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) Send(ctx context.Context, message *email.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockEmailService) SendWithID(ctx context.Context, message *email.Message) (string, error) {
	args := m.Called(ctx, message)
	return args.String(0), args.Error(1)
}

func (m *MockEmailService) SendBatch(ctx context.Context, messages []*email.Message) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *MockEmailService) TestConnection() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockEmailService) GetConfig() *email.Config {
	args := m.Called()
	return args.Get(0).(*email.Config)
}

func (m *MockEmailService) IsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

// ===============================================
// TESTS
// ===============================================

func TestNotificationService_SendEmail(t *testing.T) {
	// Setup
	mockNotificationRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockEmailService := new(MockEmailService)
	logger := zap.NewNop()

	config := &email.Config{
		Enabled: true,
	}

	service := NewNotificationService(
		mockNotificationRepo,
		mockTemplateRepo,
		mockEmailService,
		logger,
		config,
	)

	ctx := context.Background()

	t.Run("successful email send", func(t *testing.T) {
		// Setup request
		req := &models.SendEmailRequest{
			Type:     models.NotificationTypeCustom,
			Priority: models.NotificationPriorityNormal,
			To:       []string{"test@example.com"},
			Subject:  "Test Subject",
			Body:     "Test Body",
			IsHTML:   false,
		}

		// Setup mocks
		mockNotificationRepo.On("Create", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)
		mockEmailService.On("SendWithID", ctx, mock.AnythingOfType("*email.Message")).Return("msg-123", nil)
		mockNotificationRepo.On("Update", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)

		// Execute
		response, err := service.SendEmail(ctx, req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.NotificationTypeCustom, response.Type)
		assert.Equal(t, "Test Subject", response.Subject)
		assert.Equal(t, []string{"test@example.com"}, response.To)

		mockNotificationRepo.AssertExpectations(t)
		mockEmailService.AssertExpectations(t)
	})

	t.Run("validation error - empty recipients", func(t *testing.T) {
		req := &models.SendEmailRequest{
			Type:     models.NotificationTypeCustom,
			Priority: models.NotificationPriorityNormal,
			To:       []string{}, // Empty recipients
			Subject:  "Test Subject",
			Body:     "Test Body",
		}

		response, err := service.SendEmail(ctx, req)

		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "at least one recipient is required")
	})

	t.Run("validation error - missing subject without template", func(t *testing.T) {
		req := &models.SendEmailRequest{
			Type:     models.NotificationTypeCustom,
			Priority: models.NotificationPriorityNormal,
			To:       []string{"test@example.com"},
			Subject:  "", // Empty subject
			Body:     "Test Body",
		}

		response, err := service.SendEmail(ctx, req)

		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "subject is required")
	})
}

func TestNotificationService_SendTemplatedEmail(t *testing.T) {
	// Setup
	mockNotificationRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockEmailService := new(MockEmailService)
	logger := zap.NewNop()

	config := &email.Config{
		Enabled: true,
	}

	service := NewNotificationService(
		mockNotificationRepo,
		mockTemplateRepo,
		mockEmailService,
		logger,
		config,
	)

	ctx := context.Background()

	t.Run("successful templated email send", func(t *testing.T) {
		// Setup template
		template := &models.EmailTemplate{
			ID:       primitive.NewObjectID(),
			Name:     "test-template",
			Type:     models.NotificationTypeCustom,
			Subject:  "Hello {{.Name}}",
			BodyText: "Welcome {{.Name}}!",
			IsActive: true,
		}

		// Setup mocks
		mockTemplateRepo.On("GetByName", ctx, "test-template").Return(template, nil)
		mockNotificationRepo.On("Create", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)
		mockEmailService.On("SendWithID", ctx, mock.AnythingOfType("*email.Message")).Return("msg-123", nil)
		mockNotificationRepo.On("Update", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)

		// Execute
		err := service.SendTemplatedEmail(ctx, "test-template", []string{"test@example.com"}, map[string]interface{}{
			"Name": "John Doe",
		})

		// Assert
		assert.NoError(t, err)

		mockTemplateRepo.AssertExpectations(t)
		mockNotificationRepo.AssertExpectations(t)
		mockEmailService.AssertExpectations(t)
	})

	t.Run("template not found", func(t *testing.T) {
		// Setup mocks
		mockTemplateRepo.On("GetByName", ctx, "nonexistent-template").Return((*models.EmailTemplate)(nil), repository.ErrTemplateNotFound)

		// Execute
		err := service.SendTemplatedEmail(ctx, "nonexistent-template", []string{"test@example.com"}, nil)

		// Assert
		assert.Error(t, err)

		mockTemplateRepo.AssertExpectations(t)
	})
}

func TestNotificationService_SendSystemAlert(t *testing.T) {
	// Setup
	mockNotificationRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockEmailService := new(MockEmailService)
	logger := zap.NewNop()

	config := &email.Config{
		Enabled: true,
	}

	service := NewNotificationService(
		mockNotificationRepo,
		mockTemplateRepo,
		mockEmailService,
		logger,
		config,
	)

	ctx := context.Background()

	t.Run("critical alert", func(t *testing.T) {
		// Setup mocks
		mockNotificationRepo.On("Create", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)
		mockEmailService.On("SendWithID", ctx, mock.AnythingOfType("*email.Message")).Return("msg-123", nil)
		mockNotificationRepo.On("Update", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)

		// Execute
		err := service.SendSystemAlert(ctx, "System is down", "critical", []string{"admin@example.com"})

		// Assert
		assert.NoError(t, err)

		// Verificar que se llamó con la prioridad correcta
		mockNotificationRepo.AssertCalled(t, "Create", ctx, mock.MatchedBy(func(n *models.EmailNotification) bool {
			return n.Priority == models.NotificationPriorityCritical &&
				n.Type == models.NotificationTypeSystemAlert
		}))

		mockNotificationRepo.AssertExpectations(t)
		mockEmailService.AssertExpectations(t)
	})

	t.Run("warning alert", func(t *testing.T) {
		// Setup mocks
		mockNotificationRepo.On("Create", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)
		mockEmailService.On("SendWithID", ctx, mock.AnythingOfType("*email.Message")).Return("msg-123", nil)
		mockNotificationRepo.On("Update", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)

		// Execute
		err := service.SendSystemAlert(ctx, "High CPU usage", "warning", []string{"admin@example.com"})

		// Assert
		assert.NoError(t, err)

		// Verificar que se llamó con la prioridad correcta
		mockNotificationRepo.AssertCalled(t, "Create", ctx, mock.MatchedBy(func(n *models.EmailNotification) bool {
			return n.Priority == models.NotificationPriorityHigh
		}))

		mockNotificationRepo.AssertExpectations(t)
		mockEmailService.AssertExpectations(t)
	})
}

func TestNotificationService_ProcessPendingNotifications(t *testing.T) {
	// Setup
	mockNotificationRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockEmailService := new(MockEmailService)
	logger := zap.NewNop()

	config := &email.Config{
		Enabled: true,
	}

	service := NewNotificationService(
		mockNotificationRepo,
		mockTemplateRepo,
		mockEmailService,
		logger,
		config,
	)

	ctx := context.Background()

	t.Run("process pending notifications", func(t *testing.T) {
		// Setup pending notifications
		pendingNotifications := []*models.EmailNotification{
			{
				ID:      primitive.NewObjectID(),
				Status:  models.NotificationStatusPending,
				To:      []string{"test1@example.com"},
				Subject: "Test 1",
				Body:    "Body 1",
			},
			{
				ID:      primitive.NewObjectID(),
				Status:  models.NotificationStatusPending,
				To:      []string{"test2@example.com"},
				Subject: "Test 2",
				Body:    "Body 2",
			},
		}

		// Setup mocks
		mockNotificationRepo.On("List", ctx, mock.AnythingOfType("map[string]interface {}"), mock.AnythingOfType("*repository.PaginationOptions")).Return(pendingNotifications, int64(2), nil)

		for _, notification := range pendingNotifications {
			mockNotificationRepo.On("Update", ctx, mock.MatchedBy(func(n *models.EmailNotification) bool {
				return n.ID == notification.ID && n.Status == models.NotificationStatusSending
			})).Return(nil)

			mockEmailService.On("SendWithID", ctx, mock.AnythingOfType("*email.Message")).Return("msg-123", nil)

			mockNotificationRepo.On("Update", ctx, mock.MatchedBy(func(n *models.EmailNotification) bool {
				return n.ID == notification.ID && n.Status == models.NotificationStatusSent
			})).Return(nil)
		}

		// Execute
		err := service.ProcessPendingNotifications(ctx)

		// Assert
		assert.NoError(t, err)

		mockNotificationRepo.AssertExpectations(t)
		mockEmailService.AssertExpectations(t)
	})
}

func TestNotificationService_TestEmailConfiguration(t *testing.T) {
	// Setup
	mockNotificationRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockEmailService := new(MockEmailService)
	logger := zap.NewNop()

	config := &email.Config{
		Enabled:   true,
		FromEmail: "test@example.com",
	}

	service := NewNotificationService(
		mockNotificationRepo,
		mockTemplateRepo,
		mockEmailService,
		logger,
		config,
	)

	ctx := context.Background()

	t.Run("successful configuration test", func(t *testing.T) {
		// Setup mocks
		mockEmailService.On("Send", ctx, mock.AnythingOfType("*email.Message")).Return(nil)

		// Execute
		err := service.TestEmailConfiguration(ctx)

		// Assert
		assert.NoError(t, err)

		mockEmailService.AssertExpectations(t)
	})

	t.Run("configuration test failure", func(t *testing.T) {
		// Setup mocks
		mockEmailService.On("Send", ctx, mock.AnythingOfType("*email.Message")).Return(fmt.Errorf("SMTP connection failed"))

		// Execute
		err := service.TestEmailConfiguration(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SMTP connection failed")

		mockEmailService.AssertExpectations(t)
	})
}

// ===============================================
// BENCHMARK TESTS
// ===============================================

func BenchmarkNotificationService_SendEmail(b *testing.B) {
	// Setup
	mockNotificationRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockEmailService := new(MockEmailService)
	logger := zap.NewNop()

	config := &email.Config{
		Enabled: true,
	}

	service := NewNotificationService(
		mockNotificationRepo,
		mockTemplateRepo,
		mockEmailService,
		logger,
		config,
	)

	ctx := context.Background()

	// Setup mocks (will be called many times)
	mockNotificationRepo.On("Create", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)
	mockEmailService.On("SendWithID", ctx, mock.AnythingOfType("*email.Message")).Return("msg-123", nil)
	mockNotificationRepo.On("Update", ctx, mock.AnythingOfType("*models.EmailNotification")).Return(nil)

	req := &models.SendEmailRequest{
		Type:     models.NotificationTypeCustom,
		Priority: models.NotificationPriorityNormal,
		To:       []string{"test@example.com"},
		Subject:  "Benchmark Test",
		Body:     "This is a benchmark test",
		IsHTML:   false,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := service.SendEmail(ctx, req)
		if err != nil {
			b.Fatalf("SendEmail failed: %v", err)
		}
	}
}

// ===============================================
// HELPER FUNCTIONS
// ===============================================

func createTestNotification() *models.EmailNotification {
	return &models.EmailNotification{
		ID:       primitive.NewObjectID(),
		Type:     models.NotificationTypeCustom,
		Status:   models.NotificationStatusPending,
		Priority: models.NotificationPriorityNormal,
		To:       []string{"test@example.com"},
		Subject:  "Test Subject",
		Body:     "Test Body",
		IsHTML:   false,
	}
}

func createTestTemplate() *models.EmailTemplate {
	return &models.EmailTemplate{
		ID:       primitive.NewObjectID(),
		Name:     "test-template",
		Type:     models.NotificationTypeCustom,
		Subject:  "Test Template",
		BodyText: "This is a test template",
		IsActive: true,
		Language: "en",
		Version:  1,
	}
}
