package services

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/email"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// NotificationService servicio para manejo de notificaciones por email
type NotificationService struct {
	notificationRepo repository.NotificationRepository
	templateRepo     repository.TemplateRepository
	emailService     email.Service
	logger           *zap.Logger
	config           *email.Config
}

// NotificationServiceConfig configuración del servicio
type NotificationServiceConfig struct {
	NotificationRepo repository.NotificationRepository
	TemplateRepo     repository.TemplateRepository
	EmailService     email.Service
	Logger           *zap.Logger
	Config           *email.Config
}

// NewNotificationService crea una nueva instancia del servicio
func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	templateRepo repository.TemplateRepository,
	emailService email.Service,
	logger *zap.Logger,
	config *email.Config,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		templateRepo:     templateRepo,
		emailService:     emailService,
		logger:           logger,
		config:           config,
	}
}

// NewNotificationServiceWithConfig crea el servicio con configuración
func NewNotificationServiceWithConfig(config NotificationServiceConfig) *NotificationService {
	return &NotificationService{
		notificationRepo: config.NotificationRepo,
		templateRepo:     config.TemplateRepo,
		emailService:     config.EmailService,
		logger:           config.Logger,
		config:           config.Config,
	}
}

// SendEmail envía un email directo sin template
func (s *NotificationService) SendEmail(ctx context.Context, req *models.SendEmailRequest) (*models.EmailNotification, error) {
	// Validar request
	if err := s.validateSendEmailRequest(req); err != nil {
		s.logger.Warn("Invalid send email request", zap.Error(err))
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Crear notificación
	notification := &models.EmailNotification{
		ID:          primitive.NewObjectID(),
		Type:        req.Type,
		Status:      models.NotificationStatusPending,
		Priority:    req.Priority,
		To:          req.To,
		CC:          req.CC,
		BCC:         req.BCC,
		Subject:     req.Subject,
		Body:        req.Body,
		IsHTML:      req.IsHTML,
		UserID:      req.UserID,
		WorkflowID:  req.WorkflowID,
		ExecutionID: req.ExecutionID,
		ScheduledAt: req.ScheduledAt,
		Attempts:    0,
		MaxAttempts: req.MaxAttempts,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Establecer valores por defecto
	if notification.MaxAttempts == 0 {
		notification.MaxAttempts = 3
	}

	// Validar la notificación
	if err := notification.Validate(); err != nil {
		return nil, fmt.Errorf("invalid notification: %w", err)
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
	if req.ScheduledAt == nil || req.ScheduledAt.Before(time.Now()) {
		if err := s.sendNotificationNow(ctx, notification); err != nil {
			s.logger.Error("Failed to send notification immediately",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
			// No retornar error aquí porque la notificación se guardó correctamente
			// y puede ser procesada más tarde
		}
	}

	return notification, nil
}

// SendTemplatedEmail envía un email usando un template
func (s *NotificationService) SendTemplatedEmail(ctx context.Context, templateName string, to []string, data map[string]interface{}) error {
	// Obtener el template
	template, err := s.templateRepo.GetByName(ctx, templateName)
	if err != nil {
		s.logger.Error("Failed to get template",
			zap.String("template", templateName),
			zap.Error(err))
		return fmt.Errorf("failed to get template %s: %w", templateName, err)
	}

	if !template.IsActive {
		return fmt.Errorf("template %s is not active", templateName)
	}

	// Renderizar el template
	subject, err := s.renderTemplate(template.Subject, data)
	if err != nil {
		return fmt.Errorf("failed to render subject template: %w", err)
	}

	body, err := s.renderTemplate(template.BodyText, data)
	if err != nil {
		return fmt.Errorf("failed to render body template: %w", err)
	}

	htmlBody := ""
	if template.BodyHTML != "" {
		htmlBody, err = s.renderTemplate(template.BodyHTML, data)
		if err != nil {
			return fmt.Errorf("failed to render HTML body template: %w", err)
		}
	}

	// Crear request de email
	req := &models.SendEmailRequest{
		Type:         template.Type,
		Priority:     models.NotificationPriorityNormal,
		To:           to,
		Subject:      subject,
		Body:         body,
		IsHTML:       htmlBody != "",
		TemplateName: templateName,
		TemplateData: data,
	}

	if htmlBody != "" {
		req.Body = htmlBody
		req.IsHTML = true
	}

	// Enviar el email
	_, err = s.SendEmail(ctx, req)
	return err
}

// SendSystemAlert envía una alerta del sistema
func (s *NotificationService) SendSystemAlert(ctx context.Context, message, level string, recipients []string) error {
	var priority models.NotificationPriority

	switch strings.ToLower(level) {
	case "critical":
		priority = models.NotificationPriorityCritical
	case "high", "error":
		priority = models.NotificationPriorityHigh
	case "warning", "warn":
		priority = models.NotificationPriorityHigh
	case "info", "low":
		priority = models.NotificationPriorityNormal
	default:
		priority = models.NotificationPriorityNormal
	}

	req := &models.SendEmailRequest{
		Type:     models.NotificationTypeSystemAlert,
		Priority: priority,
		To:       recipients,
		Subject:  fmt.Sprintf("[%s] System Alert", strings.ToUpper(level)),
		Body:     fmt.Sprintf("System Alert: %s\n\nLevel: %s\nTimestamp: %s", message, level, time.Now().Format(time.RFC3339)),
		IsHTML:   false,
	}

	_, err := s.SendEmail(ctx, req)
	return err
}

// ProcessPendingNotifications procesa las notificaciones pendientes
func (s *NotificationService) ProcessPendingNotifications(ctx context.Context) error {
	// Obtener notificaciones pendientes
	filters := map[string]interface{}{
		"status": models.NotificationStatusPending,
	}

	// También incluir notificaciones programadas que ya deben enviarse
	scheduledFilters := map[string]interface{}{
		"status": models.NotificationStatusScheduled,
		"scheduled_at": map[string]interface{}{
			"$lte": time.Now(),
		},
	}

	pendingNotifications, _, err := s.notificationRepo.List(ctx, filters, &repository.PaginationOptions{
		Limit: 100, // Procesar en lotes
	})
	if err != nil {
		return fmt.Errorf("failed to get pending notifications: %w", err)
	}

	scheduledNotifications, _, err := s.notificationRepo.List(ctx, scheduledFilters, &repository.PaginationOptions{
		Limit: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to get scheduled notifications: %w", err)
	}

	// Combinar todas las notificaciones
	allNotifications := append(pendingNotifications, scheduledNotifications...)

	s.logger.Info("Processing notifications", zap.Int("count", len(allNotifications)))

	// Procesar cada notificación
	for _, notification := range allNotifications {
		if err := s.sendNotificationNow(ctx, notification); err != nil {
			s.logger.Error("Failed to send notification",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
		}
	}

	return nil
}

// TestEmailConfiguration prueba la configuración de email
func (s *NotificationService) TestEmailConfiguration(ctx context.Context) error {
	if !s.emailService.IsEnabled() {
		return fmt.Errorf("email service is disabled")
	}

	// Crear mensaje de prueba
	message := email.NewMessage()
	message.From = email.NewAddress(s.config.FromEmail, s.config.FromName)
	message.To = []email.Address{email.NewAddress(s.config.FromEmail, "Test")}
	message.Subject = "Email Configuration Test"
	message.TextBody = "This is a test email to verify the email configuration is working correctly."

	// Enviar mensaje de prueba
	return s.emailService.Send(ctx, message)
}

// GetNotificationStats obtiene estadísticas de notificaciones
func (s *NotificationService) GetNotificationStats(ctx context.Context, timeRange time.Duration) (*models.NotificationStats, error) {
	return s.notificationRepo.GetStats(ctx, timeRange)
}

// RetryFailedNotifications reintenta notificaciones fallidas
func (s *NotificationService) RetryFailedNotifications(ctx context.Context) error {
	failedNotifications, err := s.notificationRepo.GetFailedForRetry(ctx, 50)
	if err != nil {
		return fmt.Errorf("failed to get failed notifications: %w", err)
	}

	s.logger.Info("Retrying failed notifications", zap.Int("count", len(failedNotifications)))

	for _, notification := range failedNotifications {
		// Verificar si puede reintentarse
		if !notification.CanRetry() {
			continue
		}

		// Verificar si es tiempo de reintentar
		if notification.NextRetryAt != nil && notification.NextRetryAt.After(time.Now()) {
			continue
		}

		if err := s.sendNotificationNow(ctx, notification); err != nil {
			s.logger.Error("Failed to retry notification",
				zap.String("id", notification.ID.Hex()),
				zap.Error(err))
		}
	}

	return nil
}

// CleanupOldNotifications limpia notificaciones antiguas
func (s *NotificationService) CleanupOldNotifications(ctx context.Context, olderThan time.Duration) (int64, error) {
	deletedCount, err := s.notificationRepo.CleanupOld(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old notifications: %w", err)
	}

	s.logger.Info("Cleaned up old notifications", zap.Int64("count", deletedCount))
	return deletedCount, nil
}

// sendNotificationNow envía una notificación inmediatamente
func (s *NotificationService) sendNotificationNow(ctx context.Context, notification *models.EmailNotification) error {
	// Marcar como enviándose
	notification.MarkAsSending()
	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		return fmt.Errorf("failed to update notification status: %w", err)
	}

	// Crear mensaje de email
	message := s.buildEmailMessage(notification)

	// Enviar el email
	messageID, err := s.emailService.SendWithID(ctx, message)
	if err != nil {
		// Marcar como fallida
		notification.AddError("SEND_FAILED", err.Error(), "", true)
		notification.MarkAsFailed()

		if updateErr := s.notificationRepo.Update(ctx, notification); updateErr != nil {
			s.logger.Error("Failed to update notification after send failure",
				zap.String("id", notification.ID.Hex()),
				zap.Error(updateErr))
		}

		return fmt.Errorf("failed to send email: %w", err)
	}

	// Marcar como enviada exitosamente
	notification.MarkAsSent(messageID)
	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		s.logger.Error("Failed to update notification after successful send",
			zap.String("id", notification.ID.Hex()),
			zap.Error(err))
		// No retornar error porque el email se envió exitosamente
	}

	s.logger.Info("Notification sent successfully",
		zap.String("id", notification.ID.Hex()),
		zap.String("message_id", messageID),
		zap.Strings("to", notification.To))

	return nil
}

// buildEmailMessage construye un mensaje de email desde una notificación
func (s *NotificationService) buildEmailMessage(notification *models.EmailNotification) *email.Message {
	message := email.NewMessage()

	// From
	message.From = email.NewAddress(s.config.FromEmail, s.config.FromName)

	// To
	for _, addr := range notification.To {
		message.To = append(message.To, email.NewAddress(addr, ""))
	}

	// CC
	for _, addr := range notification.CC {
		message.CC = append(message.CC, email.NewAddress(addr, ""))
	}

	// BCC
	for _, addr := range notification.BCC {
		message.BCC = append(message.BCC, email.NewAddress(addr, ""))
	}

	// Subject y body
	message.Subject = notification.Subject

	if notification.IsHTML {
		message.HTMLBody = notification.Body
	} else {
		message.TextBody = notification.Body
	}

	// Prioridad
	switch notification.Priority {
	case models.NotificationPriorityCritical:
		message.Priority = email.PriorityCritical
	case models.NotificationPriorityHigh:
		message.Priority = email.PriorityHigh
	case models.NotificationPriorityLow:
		message.Priority = email.PriorityLow
	default:
		message.Priority = email.PriorityNormal
	}

	// Headers personalizados
	message.Headers["X-Notification-ID"] = notification.ID.Hex()
	message.Headers["X-Notification-Type"] = string(notification.Type)

	if notification.WorkflowID != nil {
		message.Headers["X-Workflow-ID"] = notification.WorkflowID.Hex()
	}

	if notification.ExecutionID != nil {
		message.Headers["X-Execution-ID"] = *notification.ExecutionID
	}

	// Tags para categorización
	message.Tags = []string{
		string(notification.Type),
		string(notification.Priority),
	}

	// Metadata
	message.Metadata = map[string]interface{}{
		"notification_id": notification.ID.Hex(),
		"type":            string(notification.Type),
		"priority":        string(notification.Priority),
		"attempt":         notification.Attempts + 1,
	}

	return message
}

// renderTemplate renderiza un template con los datos proporcionados
func (s *NotificationService) renderTemplate(templateStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("notification").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// validateSendEmailRequest valida una solicitud de envío de email
func (s *NotificationService) validateSendEmailRequest(req *models.SendEmailRequest) error {
	if len(req.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	// Si no usa template, validar contenido directo
	if req.TemplateName == "" {
		if req.Subject == "" {
			return fmt.Errorf("subject is required when not using a template")
		}
		if req.Body == "" {
			return fmt.Errorf("body is required when not using a template")
		}
	}

	// Validar prioridad
	switch req.Priority {
	case models.NotificationPriorityLow, models.NotificationPriorityNormal,
		models.NotificationPriorityHigh, models.NotificationPriorityCritical,
		models.NotificationPriorityUrgent:
		// Válidas
	default:
		return fmt.Errorf("invalid priority: %s", req.Priority)
	}

	// Validar tipo
	switch req.Type {
	case models.NotificationTypeCustom, models.NotificationTypeSystemAlert,
		models.NotificationTypeWorkflow, models.NotificationTypeUserActivity,
		models.NotificationTypeBackup, models.NotificationTypeScheduled:
		// Válidas
	default:
		return fmt.Errorf("invalid notification type: %s", req.Type)
	}

	return nil
}

// Template Management Methods

// CreateTemplate crea un nuevo template
func (s *NotificationService) CreateTemplate(ctx context.Context, template *models.EmailTemplate) error {
	// Validar template
	if err := template.ValidateTemplate(); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Establecer metadatos
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	template.Version = 1
	template.IsActive = true

	// Crear template
	if err := s.templateRepo.Create(ctx, template); err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

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
	return s.templateRepo.GetByName(ctx, name)
}

// ListTemplates lista templates con filtros
func (s *NotificationService) ListTemplates(ctx context.Context, filters map[string]interface{}) ([]*models.EmailTemplate, error) {
	return s.templateRepo.List(ctx, filters)
}

// DeleteTemplate elimina un template
func (s *NotificationService) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	s.logger.Info("Template deleted", zap.String("id", id.Hex()))
	return nil
}

// PreviewTemplate genera una vista previa del template con datos de prueba
func (s *NotificationService) PreviewTemplate(ctx context.Context, templateName string, data map[string]interface{}) (*models.EmailNotification, error) {
	template, err := s.templateRepo.GetByName(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Renderizar template
	subject, err := s.renderTemplate(template.Subject, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render subject: %w", err)
	}

	body, err := s.renderTemplate(template.BodyText, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render body: %w", err)
	}

	// Crear notificación de prueba (no guardar)
	notification := &models.EmailNotification{
		Type:         template.Type,
		Status:       models.NotificationStatusPending,
		Priority:     models.NotificationPriorityNormal,
		To:           []string{"preview@example.com"},
		Subject:      subject,
		Body:         body,
		IsHTML:       template.HasHTMLVersion(),
		TemplateName: templateName,
		TemplateData: data,
	}

	return notification, nil
}

// GetNotifications obtiene notificaciones con filtros y paginación
func (s *NotificationService) GetNotifications(ctx context.Context, filters map[string]interface{}, opts *repository.PaginationOptions) ([]*models.EmailNotification, int64, error) {
	return s.notificationRepo.List(ctx, filters, opts)
}

// GetNotification obtiene una notificación por ID
func (s *NotificationService) GetNotification(ctx context.Context, id primitive.ObjectID) (*models.EmailNotification, error) {
	return s.notificationRepo.GetByID(ctx, id)
}

// RetryNotification reintenta una notificación específica
func (s *NotificationService) RetryNotification(ctx context.Context, id primitive.ObjectID) error {
	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get notification: %w", err)
	}

	if !notification.CanRetry() {
		return fmt.Errorf("notification cannot be retried")
	}

	// Resetear estado para reintento
	notification.Status = models.NotificationStatusPending
	notification.UpdatedAt = time.Now()

	if err := s.notificationRepo.Update(ctx, notification); err != nil {
		return fmt.Errorf("failed to update notification for retry: %w", err)
	}

	// Enviar inmediatamente
	return s.sendNotificationNow(ctx, notification)
}

// CancelNotification cancela una notificación
func (s *NotificationService) CancelNotification(ctx context.Context, id primitive.ObjectID) error {
	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get notification: %w", err)
	}

	if notification.Status == models.NotificationStatusSent {
		return fmt.Errorf("cannot cancel notification that has already been sent")
	}

	notification.Status = models.NotificationStatusCancelled
	notification.UpdatedAt = time.Now()

	return s.notificationRepo.Update(ctx, notification)
}

// SearchTemplates busca templates por texto
func (s *NotificationService) SearchTemplates(ctx context.Context, searchTerm string, limit int) ([]*models.EmailTemplate, error) {
	// Si el repositorio implementa búsqueda (necesitaríamos extender la interfaz)
	// Por ahora, implementamos búsqueda básica usando filtros
	filters := map[string]interface{}{
		"$or": []map[string]interface{}{
			{"name": map[string]interface{}{"$regex": searchTerm, "$options": "i"}},
			{"description": map[string]interface{}{"$regex": searchTerm, "$options": "i"}},
			{"subject": map[string]interface{}{"$regex": searchTerm, "$options": "i"}},
		},
	}

	templates, err := s.templateRepo.List(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Limitar resultados
	if len(templates) > limit {
		templates = templates[:limit]
	}

	return templates, nil
}
