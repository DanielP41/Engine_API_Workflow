package setup

import (
	"context"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/routes"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/email"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// NotificationSystemConfig configuración completa del sistema de notificaciones
type NotificationSystemConfig struct {
	// Configuración de email
	EmailConfig *email.Config

	// Base de datos
	Database *mongo.Database

	// Logger
	Logger *zap.Logger

	// Validator
	Validator *utils.Validator

	// Middleware de autenticación
	AuthMiddleware fiber.Handler

	// Configuración de rutas
	RoutesConfig routes.NotificationRoutesConfig
}

// NotificationSystem sistema completo de notificaciones
type NotificationSystem struct {
	// Servicios
	EmailService        email.Service
	NotificationService *services.NotificationService

	// Repositorios
	NotificationRepo repository.NotificationRepository
	TemplateRepo     repository.TemplateRepository

	// Handlers
	Handler *handlers.NotificationHandler

	// Configuración
	Config *NotificationSystemConfig
}

// SetupNotificationSystem configura e inicializa el sistema completo de notificaciones
func SetupNotificationSystem(config *NotificationSystemConfig) (*NotificationSystem, error) {
	system := &NotificationSystem{
		Config: config,
	}

	// 1. Configurar servicio de email
	emailService := email.NewSMTPService(config.EmailConfig)
	system.EmailService = emailService

	// 2. Configurar repositorios
	notificationRepo := repository.NewMongoNotificationRepository(config.Database, config.Logger)
	templateRepo := repository.NewMongoTemplateRepository(config.Database, config.Logger)

	system.NotificationRepo = notificationRepo
	system.TemplateRepo = templateRepo

	// 3. Configurar servicio de notificaciones
	notificationService := services.NewNotificationService(
		notificationRepo,
		templateRepo,
		emailService,
		config.Logger,
		config.EmailConfig,
	)
	system.NotificationService = notificationService

	// 4. Configurar handler HTTP
	handler := handlers.NewNotificationHandler(
		notificationService,
		config.Validator,
		config.Logger,
	)
	system.Handler = handler

	config.Logger.Info("Notification system setup completed successfully")

	return system, nil
}

// RegisterRoutes registra las rutas HTTP del sistema de notificaciones
func (ns *NotificationSystem) RegisterRoutes(app *fiber.App) {
	routes.SetupNotificationRoutesWithConfig(
		app,
		ns.Handler,
		ns.Config.RoutesConfig,
	)

	ns.Config.Logger.Info("Notification routes registered successfully")
}

// StartBackgroundWorkers inicia los workers de procesamiento en segundo plano
func (ns *NotificationSystem) StartBackgroundWorkers(ctx context.Context) {
	// Worker para procesar notificaciones pendientes
	go ns.startPendingNotificationsWorker(ctx)

	// Worker para reintentar notificaciones fallidas
	go ns.startRetryWorker(ctx)

	// Worker para limpieza de datos antiguos
	go ns.startCleanupWorker(ctx)

	ns.Config.Logger.Info("Notification background workers started")
}

// startPendingNotificationsWorker worker para procesar notificaciones pendientes
func (ns *NotificationSystem) startPendingNotificationsWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Cada 30 segundos
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ns.Config.Logger.Info("Pending notifications worker stopped")
			return
		case <-ticker.C:
			if err := ns.NotificationService.ProcessPendingNotifications(ctx); err != nil {
				ns.Config.Logger.Error("Failed to process pending notifications", zap.Error(err))
			}
		}
	}
}

// startRetryWorker worker para reintentar notificaciones fallidas
func (ns *NotificationSystem) startRetryWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Cada 5 minutos
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ns.Config.Logger.Info("Retry notifications worker stopped")
			return
		case <-ticker.C:
			if err := ns.NotificationService.RetryFailedNotifications(ctx); err != nil {
				ns.Config.Logger.Error("Failed to retry failed notifications", zap.Error(err))
			}
		}
	}
}

// startCleanupWorker worker para limpieza de datos antiguos
func (ns *NotificationSystem) startCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // Cada 24 horas
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ns.Config.Logger.Info("Cleanup worker stopped")
			return
		case <-ticker.C:
			// Limpiar notificaciones de más de 90 días
			deletedCount, err := ns.NotificationService.CleanupOldNotifications(ctx, 90*24*time.Hour)
			if err != nil {
				ns.Config.Logger.Error("Failed to cleanup old notifications", zap.Error(err))
			} else {
				ns.Config.Logger.Info("Old notifications cleanup completed",
					zap.Int64("deleted_count", deletedCount))
			}
		}
	}
}

// TestConfiguration prueba la configuración del sistema
func (ns *NotificationSystem) TestConfiguration(ctx context.Context) error {
	ns.Config.Logger.Info("Testing notification system configuration...")

	// 1. Probar conexión de email
	if err := ns.EmailService.TestConnection(); err != nil {
		return fmt.Errorf("email service test failed: %w", err)
	}

	// 2. Probar repositorios (crear y eliminar un template de prueba)
	testTemplate := &models.EmailTemplate{
		Name:     "__test_template__",
		Type:     models.NotificationTypeCustom,
		Language: "en",
		Subject:  "Test Template",
		BodyText: "This is a test template",
		IsActive: false,
	}

	if err := ns.TemplateRepo.Create(ctx, testTemplate); err != nil {
		return fmt.Errorf("template repository test failed: %w", err)
	}

	// Limpiar template de prueba
	if err := ns.TemplateRepo.Delete(ctx, testTemplate.ID); err != nil {
		ns.Config.Logger.Warn("Failed to cleanup test template", zap.Error(err))
	}

	ns.Config.Logger.Info("Notification system configuration test completed successfully")
	return nil
}

// CreateDefaultTemplates crea templates por defecto del sistema
func (ns *NotificationSystem) CreateDefaultTemplates(ctx context.Context) error {
	ns.Config.Logger.Info("Creating default notification templates...")

	defaultTemplates := []*models.EmailTemplate{
		{
			Name:        "welcome_email",
			Type:        models.NotificationTypeUserActivity,
			Language:    "en",
			Subject:     "Welcome to {{.AppName}}!",
			BodyText:    "Hello {{.UserName}},\n\nWelcome to {{.AppName}}! Your account has been created successfully.\n\nBest regards,\nThe {{.AppName}} Team",
			BodyHTML:    "<h1>Welcome to {{.AppName}}!</h1><p>Hello {{.UserName}},</p><p>Welcome to {{.AppName}}! Your account has been created successfully.</p><p>Best regards,<br>The {{.AppName}} Team</p>",
			Description: "Template for welcome emails when users register",
			Tags:        []string{"welcome", "user", "onboarding"},
			IsActive:    true,
		},
		{
			Name:        "workflow_completion",
			Type:        models.NotificationTypeWorkflow,
			Language:    "en",
			Subject:     "Workflow '{{.WorkflowName}}' completed",
			BodyText:    "The workflow '{{.WorkflowName}}' has completed successfully.\n\nExecution ID: {{.ExecutionID}}\nCompleted at: {{.CompletedAt}}\n\nResults: {{.Results}}",
			BodyHTML:    "<h2>Workflow Completed</h2><p>The workflow '<strong>{{.WorkflowName}}</strong>' has completed successfully.</p><ul><li><strong>Execution ID:</strong> {{.ExecutionID}}</li><li><strong>Completed at:</strong> {{.CompletedAt}}</li></ul><p><strong>Results:</strong> {{.Results}}</p>",
			Description: "Template for workflow completion notifications",
			Tags:        []string{"workflow", "completion", "automation"},
			IsActive:    true,
		},
		{
			Name:        "system_alert",
			Type:        models.NotificationTypeSystemAlert,
			Language:    "en",
			Subject:     "[{{.Level}}] System Alert: {{.Message}}",
			BodyText:    "System Alert\n\nLevel: {{.Level}}\nMessage: {{.Message}}\nTimestamp: {{.Timestamp}}\n\nPlease investigate this issue immediately.",
			BodyHTML:    "<h2 style='color: red;'>System Alert</h2><ul><li><strong>Level:</strong> {{.Level}}</li><li><strong>Message:</strong> {{.Message}}</li><li><strong>Timestamp:</strong> {{.Timestamp}}</li></ul><p><strong>Please investigate this issue immediately.</strong></p>",
			Description: "Template for system alerts and errors",
			Tags:        []string{"alert", "system", "error"},
			IsActive:    true,
		},
		{
			Name:        "backup_report",
			Type:        models.NotificationTypeBackup,
			Language:    "en",
			Subject:     "Backup Report - {{.Status}}",
			BodyText:    "Backup Report\n\nStatus: {{.Status}}\nStarted: {{.StartTime}}\nCompleted: {{.EndTime}}\nDuration: {{.Duration}}\nSize: {{.BackupSize}}\n\n{{if .Errors}}Errors:\n{{.Errors}}{{end}}",
			BodyHTML:    "<h2>Backup Report</h2><ul><li><strong>Status:</strong> {{.Status}}</li><li><strong>Started:</strong> {{.StartTime}}</li><li><strong>Completed:</strong> {{.EndTime}}</li><li><strong>Duration:</strong> {{.Duration}}</li><li><strong>Size:</strong> {{.BackupSize}}</li></ul>{{if .Errors}}<h3>Errors:</h3><pre>{{.Errors}}</pre>{{end}}",
			Description: "Template for backup operation reports",
			Tags:        []string{"backup", "report", "maintenance"},
			IsActive:    true,
		},
	}

	for _, template := range defaultTemplates {
		// Verificar si ya existe
		existing, err := ns.TemplateRepo.GetByName(ctx, template.Name)
		if err == nil && existing != nil {
			ns.Config.Logger.Debug("Template already exists, skipping",
				zap.String("name", template.Name))
			continue
		}

		template.CreatedBy = "system"
		if err := ns.TemplateRepo.Create(ctx, template); err != nil {
			ns.Config.Logger.Error("Failed to create default template",
				zap.String("name", template.Name),
				zap.Error(err))
			continue
		}

		ns.Config.Logger.Info("Created default template",
			zap.String("name", template.Name))
	}

	ns.Config.Logger.Info("Default templates creation completed")
	return nil
}

// GetNotificationSystemFromConfig crea la configuración del sistema desde config.Config
func GetNotificationSystemFromConfig(cfg *config.Config, db *mongo.Database, logger *zap.Logger, validator *utils.Validator, authMiddleware fiber.Handler) *NotificationSystemConfig {
	emailConfig := &email.Config{
		Enabled:     cfg.SMTP.Enabled,
		Host:        cfg.SMTP.Host,
		Port:        cfg.SMTP.Port,
		Username:    cfg.SMTP.Username,
		Password:    cfg.SMTP.Password,
		FromEmail:   cfg.SMTP.FromEmail,
		FromName:    cfg.SMTP.FromName,
		UseTLS:      cfg.SMTP.UseTLS,
		UseStartTLS: cfg.SMTP.UseStartTLS,
		SkipVerify:  cfg.SMTP.SkipVerify,
		ConnTimeout: 10 * time.Second,
		SendTimeout: 30 * time.Second,
		MaxRetries:  3,
		RetryDelay:  5 * time.Second,
		RateLimit:   60, // 60 emails por minuto
		BurstLimit:  10,
	}

	// Configuración de rutas según el entorno
	var routesConfig routes.NotificationRoutesConfig
	if cfg.Environment == "production" {
		routesConfig = routes.ProductionNotificationRoutesConfig()
	} else if cfg.Environment == "development" {
		routesConfig = routes.DevelopmentNotificationRoutesConfig()
	} else {
		routesConfig = routes.DefaultNotificationRoutesConfig()
	}

	routesConfig.AuthMiddleware = authMiddleware

	return &NotificationSystemConfig{
		EmailConfig:    emailConfig,
		Database:       db,
		Logger:         logger,
		Validator:      validator,
		AuthMiddleware: authMiddleware,
		RoutesConfig:   routesConfig,
	}
}

// Ejemplo de uso en main.go:
/*
func main() {
    // ... configuración inicial de la app ...

    // Configurar sistema de notificaciones
    notificationConfig := setup.GetNotificationSystemFromConfig(
        config,
        database,
        logger,
        validator,
        authMiddleware,
    )

    notificationSystem, err := setup.SetupNotificationSystem(notificationConfig)
    if err != nil {
        logger.Fatal("Failed to setup notification system", zap.Error(err))
    }

    // Probar configuración
    ctx := context.Background()
    if err := notificationSystem.TestConfiguration(ctx); err != nil {
        logger.Fatal("Notification system configuration test failed", zap.Error(err))
    }

    // Crear templates por defecto
    if err := notificationSystem.CreateDefaultTemplates(ctx); err != nil {
        logger.Error("Failed to create default templates", zap.Error(err))
    }

    // Registrar rutas
    notificationSystem.RegisterRoutes(app)

    // Iniciar workers en segundo plano
    notificationSystem.StartBackgroundWorkers(ctx)

    // ... resto de la configuración de la app ...
}
*/
