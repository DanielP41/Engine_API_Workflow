package tests

import (
	"context"
	"testing"
	"time"

	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// NotificationServiceIntegrationTestSuite suite de tests de integración
type NotificationServiceIntegrationTestSuite struct {
	suite.Suite

	// Dependencias
	mongoClient *mongo.Client
	database    *mongo.Database
	logger      *zap.Logger
	validator   *utils.Validator

	// Repositorios
	notificationRepo repository.NotificationRepository
	templateRepo     repository.TemplateRepository

	// Servicios
	emailService        email.EmailService
	notificationService *services.NotificationService

	// Configuración
	config *config.Config
}

// SetupSuite configura la suite antes de todos los tests
func (suite *NotificationServiceIntegrationTestSuite) SetupSuite() {
	// Configurar logger para tests
	suite.logger = zap.NewNop() // Logger silencioso para tests

	// Configurar validator
	suite.validator = utils.NewValidator()

	// Configurar MongoDB de prueba
	mongoURI := "mongodb://localhost:27017"
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	require.NoError(suite.T(), err)

	suite.mongoClient = client
	suite.database = client.Database("engine_workflow_test_notifications")

	// Configurar repositorios
	suite.notificationRepo = repository.NewMongoNotificationRepository(suite.database, suite.logger)
	suite.templateRepo = repository.NewMongoTemplateRepository(suite.database, suite.logger)

	// Configurar servicio de email (mock para tests)
	emailConfig := &email.Config{
		Development: email.DevelopmentConfig{
			MockMode: true,
		},
		SMTP: email.SMTPConfig{
			Host: "localhost",
			Port: 587,
			DefaultFrom: email.SenderConfig{
				Email: "test@example.com",
				Name:  "Test System",
			},
		},
	}
	suite.emailService = email.NewSMTPService(emailConfig)

	// Configurar servicio de notificaciones
	suite.notificationService = services.NewNotificationService(
		suite.notificationRepo,
		suite.templateRepo,
		suite.emailService,
		suite.logger,
		emailConfig,
	)
}

// TearDownSuite limpia después de todos los tests
func (suite *NotificationServiceIntegrationTestSuite) TearDownSuite() {
	// Limpiar base de datos de prueba
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suite.database.Drop(ctx)
	suite.mongoClient.Disconnect(ctx)
}

// SetupTest configura antes de cada test
func (suite *NotificationServiceIntegrationTestSuite) SetupTest() {
	// Limpiar colecciones antes de cada test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	suite.database.Collection("email_notifications").Drop(ctx)
	suite.database.Collection("email_templates").Drop(ctx)
}

// TestCreateTemplate test de creación de template
func (suite *NotificationServiceIntegrationTestSuite) TestCreateTemplate() {
	ctx := context.Background()

	template := &models.EmailTemplate{
		Name:        "test-template",
		Type:        models.NotificationTypeCustom,
		Language:    "en",
		Subject:     "Test Subject: {{.Name}}",
		BodyText:    "Hello {{.Name}}, this is a test email.",
		BodyHTML:    "<h1>Hello {{.Name}}</h1><p>This is a test email.</p>",
		Description: "Template for testing",
		Tags:        []string{"test", "automated"},
		Variables: []models.TemplateVariable{
			{
				Name:        "Name",
				Type:        "string",
				Description: "User's name",
				Required:    true,
				Example:     "John Doe",
			},
		},
		CreatedBy: "test-system",
	}

	// Crear template
	err := suite.notificationService.CreateTemplate(ctx, template)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), template.ID)
	assert.Equal(suite.T(), 1, template.Version)
	assert.True(suite.T(), template.IsActive)

	// Verificar que se puede obtener
	retrieved, err := suite.notificationService.GetTemplate(ctx, template.ID)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), template.Name, retrieved.Name)
	assert.Equal(suite.T(), template.Subject, retrieved.Subject)
}

// TestSendEmail test de envío de email directo
func (suite *NotificationServiceIntegrationTestSuite) TestSendEmail() {
	ctx := context.Background()

	req := &models.SendEmailRequest{
		Type:     models.NotificationTypeCustom,
		Priority: models.NotificationPriorityNormal,
		To:       []string{"test@example.com"},
		Subject:  "Test Email",
		Body:     "This is a test email body.",
		IsHTML:   false,
	}

	// Enviar email (no se enviará realmente porque el servicio está deshabilitado)
	notification, err := suite.notificationService.SendEmail(ctx, req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), notification)
	assert.NotEmpty(suite.T(), notification.ID)
	assert.Equal(suite.T(), models.NotificationStatusPending, notification.Status)
	assert.Equal(suite.T(), req.To, notification.To)
	assert.Equal(suite.T(), req.Subject, notification.Subject)
	assert.Equal(suite.T(), req.Body, notification.Body)
}

// TestSendTemplatedEmail test de envío con template
func (suite *NotificationServiceIntegrationTestSuite) TestSendTemplatedEmail() {
	ctx := context.Background()

	// Crear template primero
	template := &models.EmailTemplate{
		Name:        "welcome-template",
		Type:        models.NotificationTypeUserActivity,
		Language:    "en",
		Subject:     "Welcome {{.UserName}}!",
		BodyText:    "Welcome to our system, {{.UserName}}! Your account is ready.",
		BodyHTML:    "<h1>Welcome {{.UserName}}!</h1><p>Your account is ready.</p>",
		Description: "Welcome email template",
		IsActive:    true,
		CreatedBy:   "test-system",
	}

	err := suite.notificationService.CreateTemplate(ctx, template)
	require.NoError(suite.T(), err)

	// Enviar email con template
	templateData := map[string]interface{}{
		"UserName": "John Doe",
	}

	err = suite.notificationService.SendTemplatedEmail(
		ctx,
		"welcome-template",
		[]string{"john@example.com"},
		templateData,
	)
	assert.NoError(suite.T(), err)

	// Verificar que la notificación se creó
	notifications, _, err := suite.notificationService.GetNotifications(ctx, map[string]interface{}{
		"template_name": "welcome-template",
	}, &repository.PaginationOptions{PageSize: 10})

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), notifications, 1)
	assert.Equal(suite.T(), "Welcome John Doe!", notifications[0].Subject)
	assert.Contains(suite.T(), notifications[0].Body, "John Doe")
}

// TestPreviewTemplate test de vista previa de template
func (suite *NotificationServiceIntegrationTestSuite) TestPreviewTemplate() {
	ctx := context.Background()

	// Crear template
	template := &models.EmailTemplate{
		Name:        "preview-template",
		Type:        models.NotificationTypeCustom,
		Language:    "en",
		Subject:     "Hello {{.Name}} from {{.Company}}",
		BodyText:    "Dear {{.Name}}, welcome to {{.Company}}! Your role is {{.Role}}.",
		BodyHTML:    "<h1>Hello {{.Name}}</h1><p>Welcome to <strong>{{.Company}}</strong>!</p>",
		Description: "Template for preview testing",
		IsActive:    true,
		CreatedBy:   "test-system",
	}

	err := suite.notificationService.CreateTemplate(ctx, template)
	require.NoError(suite.T(), err)

	// Generar vista previa
	previewData := map[string]interface{}{
		"Name":    "Alice Smith",
		"Company": "ACME Corp",
		"Role":    "Developer",
	}

	preview, err := suite.notificationService.PreviewTemplate(ctx, "preview-template", previewData)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), preview)
	assert.Equal(suite.T(), "Hello Alice Smith from ACME Corp", preview.Subject)
	assert.Contains(suite.T(), preview.Body, "Alice Smith")
	assert.Contains(suite.T(), preview.Body, "ACME Corp")
	assert.Contains(suite.T(), preview.Body, "Developer")
}

// TestNotificationStats test de estadísticas
func (suite *NotificationServiceIntegrationTestSuite) TestNotificationStats() {
	ctx := context.Background()

	// Crear varias notificaciones de prueba
	notifications := []*models.EmailNotification{
		{
			Type:     models.NotificationTypeCustom,
			Status:   models.NotificationStatusSent,
			Priority: models.NotificationPriorityNormal,
			To:       []string{"user1@example.com"},
			Subject:  "Test 1",
			Body:     "Body 1",
		},
		{
			Type:     models.NotificationTypeSystemAlert,
			Status:   models.NotificationStatusSent,
			Priority: models.NotificationPriorityHigh,
			To:       []string{"admin@example.com"},
			Subject:  "Alert 1",
			Body:     "Alert body",
		},
		{
			Type:     models.NotificationTypeCustom,
			Status:   models.NotificationStatusFailed,
			Priority: models.NotificationPriorityNormal,
			To:       []string{"user2@example.com"},
			Subject:  "Test 2",
			Body:     "Body 2",
		},
	}

	// Guardar notificaciones
	for _, notification := range notifications {
		err := suite.notificationRepo.Create(ctx, notification)
		require.NoError(suite.T(), err)
	}

	// Obtener estadísticas
	stats, err := suite.notificationService.GetNotificationStats(ctx, 24*time.Hour)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), stats)
	assert.Equal(suite.T(), int64(3), stats.TotalNotifications)
	assert.Equal(suite.T(), int64(2), stats.ByStatus[models.NotificationStatusSent])
	assert.Equal(suite.T(), int64(1), stats.ByStatus[models.NotificationStatusFailed])
	assert.Equal(suite.T(), int64(2), stats.ByType[models.NotificationTypeCustom])
	assert.Equal(suite.T(), int64(1), stats.ByType[models.NotificationTypeSystemAlert])
}

// TestTemplateVersioning test de versionado de templates
func (suite *NotificationServiceIntegrationTestSuite) TestTemplateVersioning() {
	ctx := context.Background()

	// Crear template inicial
	template := &models.EmailTemplate{
		Name:        "versioned-template",
		Type:        models.NotificationTypeCustom,
		Language:    "en",
		Subject:     "Version 1 Subject",
		BodyText:    "Version 1 body",
		Description: "Original version",
		IsActive:    true,
		CreatedBy:   "test-system",
	}

	err := suite.notificationService.CreateTemplate(ctx, template)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, template.Version)

	originalID := template.ID

	// Crear nueva versión
	newVersion := &models.EmailTemplate{
		Name:        "versioned-template",
		Type:        models.NotificationTypeCustom,
		Language:    "en",
		Subject:     "Version 2 Subject",
		BodyText:    "Version 2 body with improvements",
		Description: "Updated version",
		IsActive:    true,
		CreatedBy:   "test-system",
	}

	err = suite.templateRepo.CreateVersion(ctx, newVersion)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, newVersion.Version)
	assert.NotEqual(suite.T(), originalID, newVersion.ID)

	// Verificar que GetByName retorna la versión más reciente
	latest, err := suite.notificationService.GetTemplateByName(ctx, "versioned-template")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, latest.Version)
	assert.Equal(suite.T(), "Version 2 Subject", latest.Subject)

	// Verificar que se pueden obtener todas las versiones
	versions, err := suite.templateRepo.GetVersions(ctx, "versioned-template")
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), versions, 2)
	assert.Equal(suite.T(), 2, versions[0].Version) // Ordenadas por versión descendente
	assert.Equal(suite.T(), 1, versions[1].Version)
}

// TestNotificationRetry test de reintento de notificaciones
func (suite *NotificationServiceIntegrationTestSuite) TestNotificationRetry() {
	ctx := context.Background()

	// Crear notificación fallida
	notification := &models.EmailNotification{
		Type:        models.NotificationTypeCustom,
		Status:      models.NotificationStatusFailed,
		Priority:    models.NotificationPriorityNormal,
		To:          []string{"failed@example.com"},
		Subject:     "Failed Email",
		Body:        "This email failed",
		Attempts:    1,
		MaxAttempts: 3,
	}

	err := suite.notificationRepo.Create(ctx, notification)
	require.NoError(suite.T(), err)

	// Verificar que puede reintentarse
	assert.True(suite.T(), notification.CanRetry())

	// Reintentar notificación
	err = suite.notificationService.RetryNotification(ctx, notification.ID)
	assert.NoError(suite.T(), err)

	// Verificar que el estado cambió
	updated, err := suite.notificationService.GetNotification(ctx, notification.ID)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), models.NotificationStatusPending, updated.Status)
}

// TestSearchTemplates test de búsqueda de templates
func (suite *NotificationServiceIntegrationTestSuite) TestSearchTemplates() {
	ctx := context.Background()

	// Crear varios templates
	templates := []*models.EmailTemplate{
		{
			Name:        "user-welcome",
			Type:        models.NotificationTypeUserActivity,
			Language:    "en",
			Subject:     "Welcome to our platform",
			BodyText:    "Welcome message for new users",
			Description: "Welcome email for new user registration",
			Tags:        []string{"welcome", "user", "onboarding"},
			IsActive:    true,
			CreatedBy:   "test-system",
		},
		{
			Name:        "password-reset",
			Type:        models.NotificationTypeUserActivity,
			Language:    "en",
			Subject:     "Reset your password",
			BodyText:    "Instructions to reset password",
			Description: "Password reset email template",
			Tags:        []string{"password", "security", "reset"},
			IsActive:    true,
			CreatedBy:   "test-system",
		},
		{
			Name:        "system-maintenance",
			Type:        models.NotificationTypeSystemAlert,
			Language:    "en",
			Subject:     "Scheduled maintenance notification",
			BodyText:    "System will be under maintenance",
			Description: "Maintenance notification template",
			Tags:        []string{"maintenance", "system", "downtime"},
			IsActive:    true,
			CreatedBy:   "test-system",
		},
	}

	for _, template := range templates {
		err := suite.notificationService.CreateTemplate(ctx, template)
		require.NoError(suite.T(), err)
	}

	// Buscar templates por término
	results, err := suite.notificationService.SearchTemplates(ctx, "welcome", 10)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 1)
	assert.Equal(suite.T(), "user-welcome", results[0].Name)

	// Buscar por término en descripción
	results, err = suite.notificationService.SearchTemplates(ctx, "password", 10)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 1)
	assert.Equal(suite.T(), "password-reset", results[0].Name)

	// Buscar por término que aparece en múltiples templates
	results, err = suite.notificationService.SearchTemplates(ctx, "system", 10)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 1)
	assert.Equal(suite.T(), "system-maintenance", results[0].Name)
}

// TestNotificationValidation test de validación de notificaciones
func (suite *NotificationServiceIntegrationTestSuite) TestNotificationValidation() {
	ctx := context.Background()

	// Test con destinatarios vacíos
	req := &models.SendEmailRequest{
		Type:     models.NotificationTypeCustom,
		Priority: models.NotificationPriorityNormal,
		To:       []string{}, // Vacío - debe fallar
		Subject:  "Test Subject",
		Body:     "Test Body",
	}

	_, err := suite.notificationService.SendEmail(ctx, req)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "at least one recipient is required")

	// Test sin subject ni template
	req = &models.SendEmailRequest{
		Type:     models.NotificationTypeCustom,
		Priority: models.NotificationPriorityNormal,
		To:       []string{"test@example.com"},
		Subject:  "", // Vacío - debe fallar
		Body:     "Test Body",
	}

	_, err = suite.notificationService.SendEmail(ctx, req)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "subject is required")
}

// TestSystemAlert test de alertas del sistema
func (suite *NotificationServiceIntegrationTestSuite) TestSystemAlert() {
	ctx := context.Background()

	// Enviar alerta crítica
	err := suite.notificationService.SendSystemAlert(
		ctx,
		"Database connection lost",
		"critical",
		[]string{"admin@example.com", "ops@example.com"},
	)
	assert.NoError(suite.T(), err)

	// Verificar que se creó la notificación
	notifications, _, err := suite.notificationService.GetNotifications(ctx, map[string]interface{}{
		"type": models.NotificationTypeSystemAlert,
	}, &repository.PaginationOptions{PageSize: 10})

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), notifications, 1)
	assert.Equal(suite.T(), models.NotificationPriorityCritical, notifications[0].Priority)
	assert.Equal(suite.T(), "[CRITICAL] System Alert", notifications[0].Subject)
	assert.Contains(suite.T(), notifications[0].Body, "Database connection lost")
	assert.Equal(suite.T(), []string{"admin@example.com", "ops@example.com"}, notifications[0].To)
}

// Ejecutar la suite de tests
func TestNotificationServiceIntegrationSuite(t *testing.T) {
	// Solo ejecutar si MongoDB está disponible
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(NotificationServiceIntegrationTestSuite))
}

// ========================================
// TESTS UNITARIOS ADICIONALES
// ========================================

// TestNotificationModel test de métodos del modelo
func TestNotificationModel(t *testing.T) {
	notification := &models.EmailNotification{
		Status:      models.NotificationStatusFailed,
		Attempts:    2,
		MaxAttempts: 3,
	}

	// Test CanRetry
	assert.True(t, notification.CanRetry())

	// Test cuando se alcanza el máximo de intentos
	notification.Attempts = 3
	assert.False(t, notification.CanRetry())

	// Test IsDelivered
	notification.Status = models.NotificationStatusSent
	assert.True(t, notification.IsDelivered())

	notification.Status = models.NotificationStatusPending
	assert.False(t, notification.IsDelivered())

	// Test AddError
	notification.AddError("SMTP_ERROR", "SMTP connection failed", "Connection timeout", true)
	assert.Len(t, notification.Errors, 1)
	assert.Equal(t, "SMTP_ERROR", notification.Errors[0].Code)
	assert.Equal(t, "SMTP connection failed", notification.Errors[0].Message)
	assert.True(t, notification.Errors[0].Recoverable)

	// Test GetLastError
	lastError := notification.GetLastError()
	assert.NotNil(t, lastError)
	assert.Equal(t, "SMTP_ERROR", lastError.Code)
}

// TestTemplateModel test de métodos del template
func TestTemplateModel(t *testing.T) {
	template := &models.EmailTemplate{
		Name:     "test-template",
		Version:  2,
		BodyHTML: "<h1>Test</h1>",
	}

	// Test GetFullName
	fullName := template.GetFullName()
	assert.Equal(t, "test-template_v2", fullName)

	// Test HasHTMLVersion
	assert.True(t, template.HasHTMLVersion())

	template.BodyHTML = ""
	assert.False(t, template.HasHTMLVersion())

	// Test ValidateTemplate
	template.Name = ""
	err := template.ValidateTemplate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template name is required")

	template.Name = "test-template"
	template.Subject = ""
	err = template.ValidateTemplate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template subject is required")

	template.Subject = "Test Subject"
	template.BodyText = ""
	err = template.ValidateTemplate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template body is required")

	template.BodyText = "Test body"
	err = template.ValidateTemplate()
	assert.NoError(t, err)
	assert.Equal(t, "en", template.Language) // Se establece por defecto
	assert.Equal(t, 1, template.Version)     // Se establece por defecto
}

// TestEmailValidation test de validación de emails
func TestEmailValidation(t *testing.T) {
	notification := &models.EmailNotification{}

	// Test con datos válidos
	notification.To = []string{"test@example.com"}
	notification.Subject = "Test Subject"
	notification.Body = "Test Body"

	err := notification.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 3, notification.MaxAttempts) // Se establece por defecto

	// Test sin destinatarios
	notification.To = []string{}
	err = notification.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one recipient is required")

	// Test sin subject y sin template
	notification.To = []string{"test@example.com"}
	notification.Subject = ""
	notification.TemplateName = ""
	err = notification.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subject is required")

	// Test sin body y sin template
	notification.Subject = "Test Subject"
	notification.Body = ""
	notification.TemplateName = ""
	err = notification.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "body is required")
}
