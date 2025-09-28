package services

import (
	"bytes"
	"context"
	"fmt"
	htmlTemplate "html/template"
	"strings"
	"sync"
	textTemplate "text/template"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/email"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// ================================
// INTERFACES Y TIPOS
// ================================

// NotificationService servicio principal para manejo de notificaciones
type NotificationService struct {
	// Repositorios
	notificationRepo repository.NotificationRepository
	templateRepo     repository.TemplateRepository

	// Servicios externos
	emailService email.EmailService

	// Configuración
	logger *zap.Logger
	config *email.Config

	// Control de concurrencia
	processingMux sync.RWMutex
	isProcessing  bool

	// Cache de templates
	templateCache    map[string]*models.EmailTemplate
	templateCacheMux sync.RWMutex
	templateCacheTTL time.Duration

	// Métricas en memoria
	stats    *ServiceStats
	statsMux sync.RWMutex
}

// ServiceStats estadísticas del servicio en memoria
type ServiceStats struct {
	EmailsSentToday    int64         `json:"emails_sent_today"`
	EmailsFailedToday  int64         `json:"emails_failed_today"`
	LastProcessingTime time.Time     `json:"last_processing_time"`
	ProcessingDuration time.Duration `json:"processing_duration"`
	TemplatesCached    int           `json:"templates_cached"`
	ActiveTemplates    int           `json:"active_templates"`
}

// NotificationServiceConfig configuración del servicio
type NotificationServiceConfig struct {
	NotificationRepo repository.NotificationRepository
	TemplateRepo     repository.TemplateRepository
	EmailService     email.EmailService
	Logger           *zap.Logger
	Config           *email.Config
}

// ProcessingResult resultado del procesamiento
type ProcessingResult struct {
	ProcessedCount int               `json:"processed_count"`
	SuccessCount   int               `json:"success_count"`
	ErrorCount     int               `json:"error_count"`
	Errors         []ProcessingError `json:"errors,omitempty"`
	Duration       time.Duration     `json:"duration"`
}

// ProcessingError error durante el procesamiento
type ProcessingError struct {
	NotificationID string `json:"notification_id"`
	Error          string `json:"error"`
	Recoverable    bool   `json:"recoverable"`
}

// ================================
// CONSTRUCTOR
// ================================

// NewNotificationService crea una nueva instancia del servicio
func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	templateRepo repository.TemplateRepository,
	emailService email.EmailService,
	logger *zap.Logger,
	config *email.Config,
) *NotificationService {
	if logger == nil {
		logger = zap.NewNop()
	}

	service := &NotificationService{
		notificationRepo: notificationRepo,
		templateRepo:     templateRepo,
		emailService:     emailService,
		logger:           logger,
		config:           config,
		templateCache:    make(map[string]*models.EmailTemplate),
		templateCacheTTL: time.Hour,
		stats:            &ServiceStats{},
	}

	// Cargar templates activos al cache
	go service.warmUpTemplateCache()

	return service
}

// NewNotificationServiceWithConfig crea el servicio con configuración
func NewNotificationServiceWithConfig(config NotificationServiceConfig) *NotificationService {
	return NewNotificationService(
		config.NotificationRepo,
		config.TemplateRepo,
		config.EmailService,
		config.Logger,
		config.Config,
	)
}

// ================================
// MÉTODOS PRINCIPALES DE ENVÍO
// ================================

// SendEmail envía un email directo sin template
func (s *NotificationService) SendEmail(ctx context.Context, req *models.SendEmailRequest) (*models.EmailNotification, error) {
	// Validar request
	if err := s.validateSendEmailRequest(req); err != nil {
		s.logger.Warn("Invalid send email request", zap.Error(err))
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convertir request a notificación
	notification := req.ToEmailNotification()

	// Procesar template si se especifica
	if req.TemplateName != "" {
		if err := s.processTemplate(ctx, notification, req.TemplateName, req.TemplateData); err != nil {
			return nil, fmt.Errorf("template processing failed: %w", err)
		}
	}

	// Guardar en la base de datos
	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		s.logger.Error("Failed to save notification", zap.Error(err))
		return nil, fmt.Errorf("failed to save notification: %w", err)
	}

	s.logger.Info("Notification created",
		zap.String("id", notification.ID.Hex()),
		zap.String("type", string(notification.Type)),
		zap.Strings("to", notification.To))

	// Si no está programada, enviar inmediatamente
	if notification.ShouldProcess() {
		if err := s.sendNotificationNow(ctx, notification); err != nil {
			s.logger.Error("Failed to send notification immediately",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
			// No retornar error aquí porque la notificación se guardó correctamente
		}
	}

	return notification, nil
}

// SendSimpleEmail envía un email simple
func (s *NotificationService) SendSimpleEmail(ctx context.Context, to []string, subject, body string, isHTML bool) (*models.EmailNotification, error) {
	req := &models.SendEmailRequest{
		Type:     models.NotificationTypeCustom,
		Priority: models.NotificationPriorityNormal,
		To:       to,
		Subject:  subject,
		Body:     body,
		IsHTML:   isHTML,
	}

	return s.SendEmail(ctx, req)
}

// SendTemplatedEmail envía un email usando un template
func (s *NotificationService) SendTemplatedEmail(ctx context.Context, templateName string, to []string, data map[string]interface{}) error {
	req := &models.SendEmailRequest{
		Type:         models.NotificationTypeCustom,
		Priority:     models.NotificationPriorityNormal,
		To:           to,
		TemplateName: templateName,
		TemplateData: data,
	}

	_, err := s.SendEmail(ctx, req)
	return err
}

// SendWelcomeEmail envía un email de bienvenida
func (s *NotificationService) SendWelcomeEmail(ctx context.Context, userEmail, userName string) error {
	data := map[string]interface{}{
		"UserName": userName,
		"Email":    userEmail,
		"Date":     time.Now().Format("January 2, 2006"),
	}

	return s.SendTemplatedEmail(ctx, "welcome", []string{userEmail}, data)
}

// SendSystemAlert envía una alerta del sistema
func (s *NotificationService) SendSystemAlert(ctx context.Context, level, message string, recipients []string) error {
	req := &models.SendEmailRequest{
		Type:     models.NotificationTypeSystemAlert,
		Priority: models.NotificationPriorityHigh,
		To:       recipients,
		Subject:  fmt.Sprintf("[%s] System Alert", strings.ToUpper(level)),
		Body:     fmt.Sprintf("System Alert: %s\n\nTime: %s\nLevel: %s", message, time.Now().Format(time.RFC3339), level),
		IsHTML:   false,
	}

	if level == "critical" {
		req.Priority = models.NotificationPriorityCritical
	}

	_, err := s.SendEmail(ctx, req)
	return err
}

// SendPasswordResetEmail envía un email de restablecimiento de contraseña
func (s *NotificationService) SendPasswordResetEmail(ctx context.Context, userEmail, resetToken, resetURL string) error {
	data := map[string]interface{}{
		"ResetToken": resetToken,
		"ResetURL":   resetURL,
		"ExpiresIn":  "24 hours",
	}

	return s.SendTemplatedEmail(ctx, "password-reset", []string{userEmail}, data)
}

// ================================
// PROCESAMIENTO DE NOTIFICACIONES
// ================================

// ProcessPendingNotifications procesa todas las notificaciones pendientes
func (s *NotificationService) ProcessPendingNotifications(ctx context.Context) error {
	s.processingMux.Lock()
	if s.isProcessing {
		s.processingMux.Unlock()
		return fmt.Errorf("processing already in progress")
	}
	s.isProcessing = true
	s.processingMux.Unlock()

	defer func() {
		s.processingMux.Lock()
		s.isProcessing = false
		s.processingMux.Unlock()
	}()

	start := time.Now()
	s.logger.Info("Starting to process pending notifications")

	// Obtener notificaciones pendientes
	notifications, err := s.notificationRepo.GetPending(ctx, 100) // Procesar hasta 100 por vez
	if err != nil {
		return fmt.Errorf("failed to get pending notifications: %w", err)
	}

	if len(notifications) == 0 {
		s.logger.Debug("No pending notifications to process")
		return nil
	}

	s.logger.Info("Processing pending notifications", zap.Int("count", len(notifications)))

	result := &ProcessingResult{
		ProcessedCount: len(notifications),
	}

	// Procesar cada notificación
	for _, notification := range notifications {
		if err := s.sendNotificationNow(ctx, notification); err != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, ProcessingError{
				NotificationID: notification.ID.Hex(),
				Error:          err.Error(),
				Recoverable:    s.isRecoverableError(err),
			})
			s.logger.Error("Failed to send notification",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
		} else {
			result.SuccessCount++
		}
	}

	result.Duration = time.Since(start)

	// Actualizar estadísticas
	s.updateProcessingStats(result)

	s.logger.Info("Finished processing pending notifications",
		zap.Int("processed", result.ProcessedCount),
		zap.Int("success", result.SuccessCount),
		zap.Int("errors", result.ErrorCount),
		zap.Duration("duration", result.Duration))

	return nil
}

// ProcessScheduledNotifications procesa notificaciones programadas que deben enviarse
func (s *NotificationService) ProcessScheduledNotifications(ctx context.Context) error {
	notifications, err := s.notificationRepo.GetScheduled(ctx, 50)
	if err != nil {
		return fmt.Errorf("failed to get scheduled notifications: %w", err)
	}

	s.logger.Info("Processing scheduled notifications", zap.Int("count", len(notifications)))

	for _, notification := range notifications {
		// Cambiar estado a pendiente para que sea procesada
		notification.Status = models.NotificationStatusPending
		if err := s.notificationRepo.Update(ctx, notification); err != nil {
			s.logger.Error("Failed to update scheduled notification status",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
			continue
		}

		// Enviar inmediatamente
		if err := s.sendNotificationNow(ctx, notification); err != nil {
			s.logger.Error("Failed to send scheduled notification",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
		}
	}

	return nil
}

// RetryFailedNotifications reintenta notificaciones fallidas
func (s *NotificationService) RetryFailedNotifications(ctx context.Context) error {
	notifications, err := s.notificationRepo.GetFailedForRetry(ctx, 25)
	if err != nil {
		return fmt.Errorf("failed to get failed notifications: %w", err)
	}

	if len(notifications) == 0 {
		s.logger.Debug("No failed notifications to retry")
		return nil
	}

	s.logger.Info("Retrying failed notifications", zap.Int("count", len(notifications)))

	for _, notification := range notifications {
		if !notification.CanRetry() {
			continue
		}

		// Incrementar contador de intentos - CORREGIDO: usar Update en lugar de IncrementAttempts
		notification.Attempts++
		nextRetry := notification.CalculateNextRetry()
		notification.NextRetryAt = &nextRetry
		if err := s.notificationRepo.Update(ctx, notification); err != nil {
			s.logger.Error("Failed to increment attempts",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
			continue
		}

		// Reintento
		if err := s.sendNotificationNow(ctx, notification); err != nil {
			s.logger.Warn("Retry failed",
				zap.String("id", notification.ID.Hex()),
				zap.Int("attempt", notification.Attempts),
				zap.Error(err))
		} else {
			s.logger.Info("Retry successful",
				zap.String("id", notification.ID.Hex()),
				zap.Int("attempt", notification.Attempts))
		}
	}

	return nil
}

// ================================
// GESTIÓN DE TEMPLATES
// ================================

// CreateTemplate crea un nuevo template
func (s *NotificationService) CreateTemplate(ctx context.Context, template *models.EmailTemplate) error {
	// Validar template
	if err := template.ValidateTemplate(); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Establecer metadatos
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	if template.Version == 0 {
		template.Version = 1
	}

	// Crear template
	if err := s.templateRepo.Create(ctx, template); err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	// Invalidar cache
	s.invalidateTemplateCache(template.Name)

	s.logger.Info("Template created",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name))

	return nil
}

// UpdateTemplate actualiza un template existente
func (s *NotificationService) UpdateTemplate(ctx context.Context, template *models.EmailTemplate) error {
	if err := template.ValidateTemplate(); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	template.UpdatedAt = time.Now()

	if err := s.templateRepo.Update(ctx, template); err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	// Invalidar cache
	s.invalidateTemplateCache(template.Name)

	s.logger.Info("Template updated",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name))

	return nil
}

// GetTemplate obtiene un template por ID
func (s *NotificationService) GetTemplate(ctx context.Context, id primitive.ObjectID) (*models.EmailTemplate, error) {
	return s.templateRepo.GetByID(ctx, id)
}

// GetTemplateByName obtiene un template por nombre
func (s *NotificationService) GetTemplateByName(ctx context.Context, name string) (*models.EmailTemplate, error) {
	// Verificar cache primero
	if template := s.getTemplateFromCache(name); template != nil {
		return template, nil
	}

	// Obtener de base de datos
	template, err := s.templateRepo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	// Agregar al cache
	s.addTemplateToCache(template)

	return template, nil
}

// ListTemplates lista templates con filtros
func (s *NotificationService) ListTemplates(ctx context.Context, filters map[string]interface{}) ([]*models.EmailTemplate, error) {
	return s.templateRepo.List(ctx, filters)
}

// DeleteTemplate elimina un template
func (s *NotificationService) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	// Obtener template para invalidar cache
	template, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	// Invalidar cache
	s.invalidateTemplateCache(template.Name)

	s.logger.Info("Template deleted",
		zap.String("id", id.Hex()),
		zap.String("name", template.Name))

	return nil
}

// PreviewTemplate genera una vista previa de un template con datos
func (s *NotificationService) PreviewTemplate(ctx context.Context, templateName string, data map[string]interface{}) (*models.TemplatePreview, error) {
	template, err := s.GetTemplateByName(ctx, templateName)
	if err != nil {
		return nil, err
	}

	preview := &models.TemplatePreview{}

	// Renderizar subject
	if subject, err := s.renderTextTemplate(template.Subject, data); err != nil {
		preview.Variables = data
		return preview, fmt.Errorf("subject rendering failed: %w", err)
	} else {
		preview.Subject = subject
	}

	// Renderizar body
	if template.BodyHTML != "" {
		if body, err := s.renderHTMLTemplate(template.BodyHTML, data); err != nil {
			return preview, fmt.Errorf("HTML body rendering failed: %w", err)
		} else {
			preview.Body = body
			preview.IsHTML = true
		}
	} else {
		if body, err := s.renderTextTemplate(template.BodyText, data); err != nil {
			return preview, fmt.Errorf("text body rendering failed: %w", err)
		} else {
			preview.Body = body
			preview.IsHTML = false
		}
	}

	preview.Variables = data
	return preview, nil
}

// ================================
// GESTIÓN DE NOTIFICACIONES
// ================================

// GetNotification obtiene una notificación por ID
func (s *NotificationService) GetNotification(ctx context.Context, id primitive.ObjectID) (*models.EmailNotification, error) {
	return s.notificationRepo.GetByID(ctx, id)
}

// GetNotifications obtiene una lista de notificaciones con filtros
func (s *NotificationService) GetNotifications(ctx context.Context, filters map[string]interface{}, opts *repository.PaginationOptions) ([]*models.EmailNotification, int64, error) {
	return s.notificationRepo.List(ctx, filters, opts)
}

// CancelNotification cancela una notificación pendiente o programada
func (s *NotificationService) CancelNotification(ctx context.Context, id primitive.ObjectID) error {
	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if notification.Status != models.NotificationStatusPending && notification.Status != models.NotificationStatusScheduled {
		return fmt.Errorf("cannot cancel notification with status: %s", notification.Status)
	}

	// CORREGIDO: usar Update en lugar de UpdateStatus
	notification.Status = models.NotificationStatusCancelled
	return s.notificationRepo.Update(ctx, notification)
}

// ResendNotification reenvía una notificación
func (s *NotificationService) ResendNotification(ctx context.Context, id primitive.ObjectID) error {
	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Resetear para reenvío
	notification.Status = models.NotificationStatusPending
	notification.Attempts = 0
	notification.Errors = nil
	notification.NextRetryAt = nil
	notification.UpdatedAt = time.Now()

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		return err
	}

	return s.sendNotificationNow(ctx, notification)
}

// ================================
// ESTADÍSTICAS Y MONITOREO
// ================================

// GetNotificationStats obtiene estadísticas de notificaciones
func (s *NotificationService) GetNotificationStats(ctx context.Context, timeRange time.Duration) (*models.NotificationStats, error) {
	return s.notificationRepo.GetStats(ctx, timeRange)
}

// GetServiceStats obtiene estadísticas del servicio
func (s *NotificationService) GetServiceStats() *ServiceStats {
	s.statsMux.RLock()
	defer s.statsMux.RUnlock()

	// Crear copia para thread safety
	stats := *s.stats

	// Agregar información de cache
	s.templateCacheMux.RLock()
	stats.TemplatesCached = len(s.templateCache)
	s.templateCacheMux.RUnlock()

	return &stats
}

// ================================
// ADMINISTRACIÓN Y MANTENIMIENTO
// ================================

// TestEmailConfiguration prueba la configuración de email
func (s *NotificationService) TestEmailConfiguration(ctx context.Context) error {
	// Crear mensaje de prueba - CORREGIDO: usar SMTP.DefaultFrom.Email y TextBody
	testMessage := &email.Message{
		To:       []email.Address{{Email: s.config.SMTP.DefaultFrom.Email}}, // CORREGIDO
		Subject:  "Test Email Configuration",
		TextBody: "This is a test email to verify the SMTP configuration is working correctly.", // CORREGIDO
	}

	return s.emailService.Send(ctx, testMessage)
}

// CleanupOldNotifications limpia notificaciones antiguas
func (s *NotificationService) CleanupOldNotifications(ctx context.Context, olderThan time.Duration) (int64, error) {
	deletedCount, err := s.notificationRepo.CleanupOld(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("cleanup failed: %w", err)
	}

	s.logger.Info("Old notifications cleaned up",
		zap.Int64("deleted_count", deletedCount),
		zap.Duration("older_than", olderThan))

	return deletedCount, nil
}

// GetHealthStatus verifica el estado de salud del servicio
func (s *NotificationService) GetHealthStatus(ctx context.Context) map[string]interface{} {
	status := map[string]interface{}{
		"service":         "healthy",
		"email_service":   "unknown",
		"database":        "unknown",
		"template_cache":  len(s.templateCache),
		"last_processing": s.stats.LastProcessingTime,
	}

	// Verificar servicio de email
	if s.emailService.IsEnabled() {
		status["email_service"] = "healthy"
	} else {
		status["email_service"] = "disabled"
	}

	// Verificar base de datos (si el repo lo soporta)
	if validator, ok := s.notificationRepo.(interface{ ValidateConnection(context.Context) error }); ok {
		if err := validator.ValidateConnection(ctx); err != nil {
			status["database"] = "unhealthy"
			status["service"] = "degraded"
		} else {
			status["database"] = "healthy"
		}
	}

	return status
}

// CreateDefaultTemplates crea templates por defecto del sistema
func (s *NotificationService) CreateDefaultTemplates(ctx context.Context) error {
	defaultTemplates := s.getDefaultTemplates()

	for _, template := range defaultTemplates {
		// Verificar si ya existe
		existing, err := s.templateRepo.GetByName(ctx, template.Name)
		if err == nil && existing != nil {
			s.logger.Info("Default template already exists, skipping",
				zap.String("name", template.Name))
			continue
		}

		if err := s.CreateTemplate(ctx, template); err != nil {
			s.logger.Error("Failed to create default template",
				zap.String("name", template.Name),
				zap.Error(err))
			return err
		}

		s.logger.Info("Default template created",
			zap.String("name", template.Name))
	}

	return nil
}

// ================================
// MÉTODOS INTERNOS
// ================================

// sendNotificationNow envía una notificación inmediatamente
func (s *NotificationService) sendNotificationNow(ctx context.Context, notification *models.EmailNotification) error {
	// Actualizar estado a procesando - CORREGIDO: usar Update en lugar de UpdateStatus
	notification.Status = models.NotificationStatusProcessing
	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		return fmt.Errorf("failed to update status to processing: %w", err)
	}

	// Crear mensaje de email - CORREGIDO: usar TextBody/HTMLBody según el tipo
	emailMessage := &email.Message{
		To:      s.convertStringSliceToAddresses(notification.To),
		CC:      s.convertStringSliceToAddresses(notification.CC),
		BCC:     s.convertStringSliceToAddresses(notification.BCC),
		Subject: notification.Subject,
	}

	// Configurar contenido según el tipo - CORREGIDO
	if notification.IsHTML {
		emailMessage.HTMLBody = notification.Body
	} else {
		emailMessage.TextBody = notification.Body
	}

	// Configurar prioridad
	switch notification.Priority {
	case models.NotificationPriorityHigh:
		emailMessage.Priority = email.PriorityHigh
	case models.NotificationPriorityCritical:
		emailMessage.Priority = email.PriorityCritical
	case models.NotificationPriorityLow:
		emailMessage.Priority = email.PriorityLow
	default:
		emailMessage.Priority = email.PriorityNormal
	}

	// Enviar email
	if err := s.emailService.Send(ctx, emailMessage); err != nil {
		// Manejar error
		s.handleSendError(ctx, notification, err)
		return err
	}

	// Actualizar estado a enviado - CORREGIDO: usar Update en lugar de UpdateStatus
	notification.Status = models.NotificationStatusSent
	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		s.logger.Error("Failed to update status to sent",
			zap.String("id", notification.ID.Hex()),
			zap.Error(err))
	}

	// Actualizar estadísticas - CORREGIDO: statsMux en lugar de statsMutex
	s.statsMux.Lock()
	s.stats.EmailsSentToday++
	s.statsMux.Unlock()

	s.logger.Info("Notification sent successfully",
		zap.String("id", notification.ID.Hex()),
		zap.Strings("to", notification.To))

	return nil
}

// convertStringSliceToAddresses convierte slice de strings a slice de email.Address
func (s *NotificationService) convertStringSliceToAddresses(emails []string) []email.Address {
	addresses := make([]email.Address, len(emails))
	for i, emailStr := range emails {
		addresses[i] = email.Address{Email: emailStr}
	}
	return addresses
}

// processTemplate procesa un template y actualiza la notificación
func (s *NotificationService) processTemplate(ctx context.Context, notification *models.EmailNotification, templateName string, data map[string]interface{}) error {
	template, err := s.GetTemplateByName(ctx, templateName)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	// Renderizar subject
	if subject, err := s.renderTextTemplate(template.Subject, data); err != nil {
		return fmt.Errorf("failed to render subject: %w", err)
	} else {
		notification.Subject = subject
	}

	// Renderizar body
	if template.BodyHTML != "" {
		if body, err := s.renderHTMLTemplate(template.BodyHTML, data); err != nil {
			return fmt.Errorf("failed to render HTML body: %w", err)
		} else {
			notification.Body = body
			notification.IsHTML = true
		}
	} else {
		if body, err := s.renderTextTemplate(template.BodyText, data); err != nil {
			return fmt.Errorf("failed to render text body: %w", err)
		} else {
			notification.Body = body
			notification.IsHTML = false
		}
	}

	// Establecer metadatos
	notification.TemplateName = templateName
	notification.TemplateData = data

	return nil
}

// handleSendError maneja errores de envío
func (s *NotificationService) handleSendError(ctx context.Context, notification *models.EmailNotification, sendErr error) {
	// Crear error de notificación
	notifError := models.NotificationError{
		Timestamp:   time.Now(),
		Code:        "SEND_ERROR",
		Message:     sendErr.Error(),
		Recoverable: s.isRecoverableError(sendErr),
	}

	// Agregar error a la notificación - CORREGIDO: usar Update en lugar de AddError
	notification.Errors = append(notification.Errors, notifError)
	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		s.logger.Error("Failed to add error to notification",
			zap.String("id", notification.ID.Hex()),
			zap.Error(err))
	}

	// Determinar próximo estado
	if notification.Attempts >= notification.MaxAttempts {
		// Máximo de intentos alcanzado
		notification.Status = models.NotificationStatusFailed
		s.notificationRepo.Update(ctx, notification)
	} else if notifError.Recoverable {
		// Programar reintento
		nextRetry := notification.CalculateNextRetry()
		notification.NextRetryAt = &nextRetry
		notification.Attempts++
		notification.Status = models.NotificationStatusFailed
		s.notificationRepo.Update(ctx, notification)
	} else {
		// Error no recuperable
		notification.Status = models.NotificationStatusFailed
		s.notificationRepo.Update(ctx, notification)
	}

	// Actualizar estadísticas
	s.statsMux.Lock()
	s.stats.EmailsFailedToday++
	s.statsMux.Unlock()
}

// validateSendEmailRequest valida una solicitud de envío
func (s *NotificationService) validateSendEmailRequest(req *models.SendEmailRequest) error {
	if len(req.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	if req.Subject == "" && req.TemplateName == "" {
		return fmt.Errorf("subject or template name is required")
	}

	if req.Body == "" && req.TemplateName == "" {
		return fmt.Errorf("body or template name is required")
	}

	// Validar tipo
	if !req.Type.IsValid() {
		return fmt.Errorf("invalid notification type: %s", req.Type)
	}

	// Validar prioridad
	if !req.Priority.IsValid() {
		return fmt.Errorf("invalid notification priority: %s", req.Priority)
	}

	return nil
}

// isRecoverableError determina si un error es recuperable
func (s *NotificationService) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Errores temporales
	recoverableKeywords := []string{
		"timeout", "temporary", "network", "connection",
		"dial", "read", "write", "broken pipe",
		"connection reset", "i/o timeout",
	}

	for _, keyword := range recoverableKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// ================================
// GESTIÓN DE CACHE DE TEMPLATES
// ================================

// getTemplateFromCache obtiene un template del cache
func (s *NotificationService) getTemplateFromCache(name string) *models.EmailTemplate {
	s.templateCacheMux.RLock()
	defer s.templateCacheMux.RUnlock()

	if template, exists := s.templateCache[name]; exists {
		return template
	}
	return nil
}

// addTemplateToCache agrega un template al cache
func (s *NotificationService) addTemplateToCache(template *models.EmailTemplate) {
	s.templateCacheMux.Lock()
	defer s.templateCacheMux.Unlock()

	s.templateCache[template.Name] = template
}

// invalidateTemplateCache invalida un template del cache
func (s *NotificationService) invalidateTemplateCache(name string) {
	s.templateCacheMux.Lock()
	defer s.templateCacheMux.Unlock()

	delete(s.templateCache, name)
}

// warmUpTemplateCache carga templates activos al cache
func (s *NotificationService) warmUpTemplateCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	templates, err := s.templateRepo.GetActive(ctx)
	if err != nil {
		s.logger.Error("Failed to warm up template cache", zap.Error(err))
		return
	}

	s.templateCacheMux.Lock()
	for _, template := range templates {
		s.templateCache[template.Name] = template
	}
	s.templateCacheMux.Unlock()

	s.logger.Info("Template cache warmed up", zap.Int("count", len(templates)))
}

// ================================
// RENDERIZADO DE TEMPLATES
// ================================

// renderTextTemplate renderiza un template de texto
func (s *NotificationService) renderTextTemplate(templateText string, data map[string]interface{}) (string, error) {
	tmpl, err := textTemplate.New("template").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buffer.String(), nil
}

// renderHTMLTemplate renderiza un template HTML
func (s *NotificationService) renderHTMLTemplate(templateHTML string, data map[string]interface{}) (string, error) {
	tmpl, err := htmlTemplate.New("template").Parse(templateHTML)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return buffer.String(), nil
}

// ================================
// UTILIDADES
// ================================

// updateProcessingStats actualiza las estadísticas de procesamiento
func (s *NotificationService) updateProcessingStats(result *ProcessingResult) {
	s.statsMux.Lock()
	defer s.statsMux.Unlock()

	s.stats.LastProcessingTime = time.Now()
	s.stats.ProcessingDuration = result.Duration
	s.stats.EmailsSentToday += int64(result.SuccessCount)
	s.stats.EmailsFailedToday += int64(result.ErrorCount)
}

// getDefaultTemplates devuelve los templates por defecto del sistema
func (s *NotificationService) getDefaultTemplates() []*models.EmailTemplate {
	return []*models.EmailTemplate{
		{
			Name:        "welcome",
			Type:        models.NotificationTypeWelcome,
			Language:    "en",
			Subject:     "Welcome to {{.SystemName | default \"our system\"}}!",
			BodyText:    "Hi {{.UserName}},\n\nWelcome to our system! Your account has been created successfully.\n\nEmail: {{.Email}}\nDate: {{.Date}}\n\nBest regards,\nThe Team",
			BodyHTML:    "<h1>Welcome {{.UserName}}!</h1><p>Your account has been created successfully.</p><p><strong>Email:</strong> {{.Email}}</p><p><strong>Date:</strong> {{.Date}}</p><p>Best regards,<br>The Team</p>",
			Description: "Welcome email for new users",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			Name:        "password-reset",
			Type:        models.NotificationTypePasswordReset,
			Language:    "en",
			Subject:     "Password Reset Request",
			BodyText:    "You requested a password reset. Click the following link to reset your password:\n\n{{.ResetURL}}\n\nThis link expires in {{.ExpiresIn}}.\n\nIf you didn't request this, please ignore this email.",
			BodyHTML:    "<h2>Password Reset Request</h2><p>You requested a password reset. Click the link below to reset your password:</p><p><a href=\"{{.ResetURL}}\">Reset Password</a></p><p>This link expires in {{.ExpiresIn}}.</p><p>If you didn't request this, please ignore this email.</p>",
			Description: "Password reset email template",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			Name:        "system-alert",
			Type:        models.NotificationTypeSystemAlert,
			Language:    "en",
			Subject:     "[{{.Level | upper}}] System Alert: {{.Title}}",
			BodyText:    "System Alert\n\nLevel: {{.Level}}\nTitle: {{.Title}}\nMessage: {{.Message}}\n\nTime: {{.Timestamp}}\nServer: {{.Server | default \"Unknown\"}}",
			BodyHTML:    "<h2>🚨 System Alert</h2><p><strong>Level:</strong> {{.Level}}</p><p><strong>Title:</strong> {{.Title}}</p><p><strong>Message:</strong> {{.Message}}</p><p><strong>Time:</strong> {{.Timestamp}}</p><p><strong>Server:</strong> {{.Server | default \"Unknown\"}}</p>",
			Description: "System alert notification template",
			IsActive:    true,
			CreatedBy:   "system",
		},
	}
}
