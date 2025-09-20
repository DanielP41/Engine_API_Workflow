package notificationclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Client cliente para el sistema de notificaciones
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
	UserAgent  string
}

// ClientConfig configuración del cliente
type ClientConfig struct {
	BaseURL   string
	Token     string
	Timeout   time.Duration
	UserAgent string
}

// NewClient crea un nuevo cliente de notificaciones
func NewClient(config ClientConfig) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.UserAgent == "" {
		config.UserAgent = "NotificationClient/1.0"
	}

	return &Client{
		BaseURL:   config.BaseURL,
		Token:     config.Token,
		UserAgent: config.UserAgent,
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Response respuesta genérica de la API
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// NotificationType tipos de notificación
type NotificationType string

const (
	NotificationTypeCustom       NotificationType = "custom"
	NotificationTypeSystemAlert  NotificationType = "system_alert"
	NotificationTypeWorkflow     NotificationType = "workflow"
	NotificationTypeUserActivity NotificationType = "user_activity"
	NotificationTypeBackup       NotificationType = "backup"
	NotificationTypeScheduled    NotificationType = "scheduled"
)

// NotificationPriority prioridades de notificación
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"
	NotificationPriorityNormal   NotificationPriority = "normal"
	NotificationPriorityHigh     NotificationPriority = "high"
	NotificationPriorityCritical NotificationPriority = "critical"
	NotificationPriorityUrgent   NotificationPriority = "urgent"
)

// NotificationStatus estados de notificación
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"
	NotificationStatusSending   NotificationStatus = "sending"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusFailed    NotificationStatus = "failed"
	NotificationStatusCancelled NotificationStatus = "cancelled"
	NotificationStatusScheduled NotificationStatus = "scheduled"
)

// SendEmailRequest solicitud para enviar email
type SendEmailRequest struct {
	Type        NotificationType     `json:"type"`
	Priority    NotificationPriority `json:"priority"`
	To          []string             `json:"to"`
	CC          []string             `json:"cc,omitempty"`
	BCC         []string             `json:"bcc,omitempty"`
	Subject     string               `json:"subject,omitempty"`
	Body        string               `json:"body,omitempty"`
	IsHTML      bool                 `json:"is_html,omitempty"`
	ScheduledAt *time.Time           `json:"scheduled_at,omitempty"`
	MaxAttempts int                  `json:"max_attempts,omitempty"`
	WorkflowID  *primitive.ObjectID  `json:"workflow_id,omitempty"`
	ExecutionID *string              `json:"execution_id,omitempty"`
}

// SendTemplatedEmailRequest solicitud para enviar email con template
type SendTemplatedEmailRequest struct {
	TemplateName string                 `json:"template_name"`
	To           []string               `json:"to"`
	CC           []string               `json:"cc,omitempty"`
	BCC          []string               `json:"bcc,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	ScheduledAt  *time.Time             `json:"scheduled_at,omitempty"`
	Priority     NotificationPriority   `json:"priority,omitempty"`
}

// EmailNotification notificación de email
type EmailNotification struct {
	ID          string               `json:"id"`
	Type        NotificationType     `json:"type"`
	Status      NotificationStatus   `json:"status"`
	Priority    NotificationPriority `json:"priority"`
	To          []string             `json:"to"`
	CC          []string             `json:"cc,omitempty"`
	BCC         []string             `json:"bcc,omitempty"`
	Subject     string               `json:"subject"`
	Body        string               `json:"body"`
	IsHTML      bool                 `json:"is_html"`
	ScheduledAt *time.Time           `json:"scheduled_at,omitempty"`
	SentAt      *time.Time           `json:"sent_at,omitempty"`
	Attempts    int                  `json:"attempts"`
	MaxAttempts int                  `json:"max_attempts"`
	MessageID   string               `json:"message_id,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// SendEmailResponse respuesta del envío de email
type SendEmailResponse struct {
	NotificationID string             `json:"notification_id"`
	Status         NotificationStatus `json:"status"`
	CreatedAt      time.Time          `json:"created_at"`
}

// ListNotificationsOptions opciones para listar notificaciones
type ListNotificationsOptions struct {
	Status      NotificationStatus   `json:"status,omitempty"`
	Type        NotificationType     `json:"type,omitempty"`
	Priority    NotificationPriority `json:"priority,omitempty"`
	UserID      string               `json:"user_id,omitempty"`
	WorkflowID  string               `json:"workflow_id,omitempty"`
	ExecutionID string               `json:"execution_id,omitempty"`
	ToEmail     string               `json:"to_email,omitempty"`
	Page        int                  `json:"page,omitempty"`
	Limit       int                  `json:"limit,omitempty"`
}

// NotificationStats estadísticas de notificaciones
type NotificationStats struct {
	TotalNotifications int64                          `json:"total_notifications"`
	ByStatus           map[NotificationStatus]int64   `json:"by_status"`
	ByType             map[NotificationType]int64     `json:"by_type"`
	ByPriority         map[NotificationPriority]int64 `json:"by_priority"`
	SuccessRate        float64                        `json:"success_rate"`
	AverageRetries     float64                        `json:"average_retries"`
	LastProcessed      *time.Time                     `json:"last_processed,omitempty"`
	Period             string                         `json:"period"`
}

// HealthStatus estado de salud del sistema
type HealthStatus struct {
	Status     string           `json:"status"`
	Service    string           `json:"service"`
	Email      string           `json:"email"`
	Worker     string           `json:"worker"`
	QueueStats map[string]int64 `json:"queue_stats"`
	Timestamp  time.Time        `json:"timestamp"`
}

// ===========================================
// MÉTODOS DEL CLIENTE
// ===========================================

// makeRequest realiza una petición HTTP
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body interface{}) (*Response, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &response, fmt.Errorf("API error (%d): %s", resp.StatusCode, response.Message)
	}

	return &response, nil
}

// SendEmail envía un email directo
func (c *Client) SendEmail(ctx context.Context, req *SendEmailRequest) (*SendEmailResponse, error) {
	response, err := c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req)
	if err != nil {
		return nil, err
	}

	var result SendEmailResponse
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling send email response: %w", err)
	}

	return &result, nil
}

// SendTemplatedEmail envía un email usando un template
func (c *Client) SendTemplatedEmail(ctx context.Context, req *SendTemplatedEmailRequest) error {
	_, err := c.makeRequest(ctx, "POST", "/api/v1/notifications/send/template", req)
	return err
}

// GetNotification obtiene una notificación por ID
func (c *Client) GetNotification(ctx context.Context, id string) (*EmailNotification, error) {
	response, err := c.makeRequest(ctx, "GET", "/api/v1/notifications/"+id, nil)
	if err != nil {
		return nil, err
	}

	var notification EmailNotification
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &notification); err != nil {
		return nil, fmt.Errorf("error unmarshaling notification: %w", err)
	}

	return &notification, nil
}

// ListNotifications lista notificaciones con filtros
func (c *Client) ListNotifications(ctx context.Context, opts *ListNotificationsOptions) ([]*EmailNotification, error) {
	endpoint := "/api/v1/notifications"

	if opts != nil {
		params := url.Values{}

		if opts.Status != "" {
			params.Add("status", string(opts.Status))
		}
		if opts.Type != "" {
			params.Add("type", string(opts.Type))
		}
		if opts.Priority != "" {
			params.Add("priority", string(opts.Priority))
		}
		if opts.UserID != "" {
			params.Add("user_id", opts.UserID)
		}
		if opts.WorkflowID != "" {
			params.Add("workflow_id", opts.WorkflowID)
		}
		if opts.ExecutionID != "" {
			params.Add("execution_id", opts.ExecutionID)
		}
		if opts.ToEmail != "" {
			params.Add("to_email", opts.ToEmail)
		}
		if opts.Page > 0 {
			params.Add("page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			params.Add("limit", strconv.Itoa(opts.Limit))
		}

		if len(params) > 0 {
			endpoint += "?" + params.Encode()
		}
	}

	response, err := c.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Extraer las notificaciones de la respuesta
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	notificationsData, ok := data["notifications"]
	if !ok {
		return nil, fmt.Errorf("notifications not found in response")
	}

	dataBytes, err := json.Marshal(notificationsData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling notifications data: %w", err)
	}

	var notifications []*EmailNotification
	if err := json.Unmarshal(dataBytes, &notifications); err != nil {
		return nil, fmt.Errorf("error unmarshaling notifications: %w", err)
	}

	return notifications, nil
}

// RetryNotification reintenta una notificación fallida
func (c *Client) RetryNotification(ctx context.Context, id string) error {
	_, err := c.makeRequest(ctx, "POST", "/api/v1/notifications/"+id+"/retry", nil)
	return err
}

// CancelNotification cancela una notificación
func (c *Client) CancelNotification(ctx context.Context, id string) error {
	_, err := c.makeRequest(ctx, "POST", "/api/v1/notifications/"+id+"/cancel", nil)
	return err
}

// GetStats obtiene estadísticas de notificaciones
func (c *Client) GetStats(ctx context.Context, timeRange time.Duration) (*NotificationStats, error) {
	endpoint := fmt.Sprintf("/api/v1/notifications/stats?time_range=%s", timeRange.String())

	response, err := c.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var stats NotificationStats
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &stats); err != nil {
		return nil, fmt.Errorf("error unmarshaling stats: %w", err)
	}

	return &stats, nil
}

// GetHealth verifica el estado de salud del sistema
func (c *Client) GetHealth(ctx context.Context) (*HealthStatus, error) {
	response, err := c.makeRequest(ctx, "GET", "/health/notifications", nil)
	if err != nil {
		return nil, err
	}

	var health HealthStatus
	dataBytes, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("error marshaling response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &health); err != nil {
		return nil, fmt.Errorf("error unmarshaling health status: %w", err)
	}

	return &health, nil
}

// ===========================================
// MÉTODOS DE CONVENIENCIA
// ===========================================

// SendSimpleEmail envía un email simple con parámetros básicos
func (c *Client) SendSimpleEmail(ctx context.Context, to []string, subject, body string) (*SendEmailResponse, error) {
	req := &SendEmailRequest{
		Type:     NotificationTypeCustom,
		Priority: NotificationPriorityNormal,
		To:       to,
		Subject:  subject,
		Body:     body,
		IsHTML:   false,
	}

	return c.SendEmail(ctx, req)
}

// SendHTMLEmail envía un email HTML
func (c *Client) SendHTMLEmail(ctx context.Context, to []string, subject, htmlBody string) (*SendEmailResponse, error) {
	req := &SendEmailRequest{
		Type:     NotificationTypeCustom,
		Priority: NotificationPriorityNormal,
		To:       to,
		Subject:  subject,
		Body:     htmlBody,
		IsHTML:   true,
	}

	return c.SendEmail(ctx, req)
}

// SendWelcomeEmail envía un email de bienvenida usando template
func (c *Client) SendWelcomeEmail(ctx context.Context, to string, userName, appName string) error {
	req := &SendTemplatedEmailRequest{
		TemplateName: "welcome_email",
		To:           []string{to},
		Data: map[string]interface{}{
			"UserName":  userName,
			"AppName":   appName,
			"UserEmail": to,
		},
		Priority: NotificationPriorityNormal,
	}

	return c.SendTemplatedEmail(ctx, req)
}

// SendPasswordResetEmail envía email de restablecimiento de contraseña
func (c *Client) SendPasswordResetEmail(ctx context.Context, to, userName, resetLink string) error {
	req := &SendTemplatedEmailRequest{
		TemplateName: "password_reset",
		To:           []string{to},
		Data: map[string]interface{}{
			"UserName":       userName,
			"ResetLink":      resetLink,
			"ExpirationTime": "24 horas",
		},
		Priority: NotificationPriorityHigh,
	}

	return c.SendTemplatedEmail(ctx, req)
}

// SendSystemAlert envía una alerta del sistema
func (c *Client) SendSystemAlert(ctx context.Context, to []string, level, message string) (*SendEmailResponse, error) {
	var priority NotificationPriority
	switch level {
	case "critical":
		priority = NotificationPriorityCritical
	case "high", "error":
		priority = NotificationPriorityHigh
	case "warning":
		priority = NotificationPriorityHigh
	default:
		priority = NotificationPriorityNormal
	}

	req := &SendEmailRequest{
		Type:     NotificationTypeSystemAlert,
		Priority: priority,
		To:       to,
		Subject:  fmt.Sprintf("[%s] System Alert", strings.ToUpper(level)),
		Body:     fmt.Sprintf("System Alert: %s\n\nLevel: %s\nTimestamp: %s", message, level, time.Now().Format(time.RFC3339)),
		IsHTML:   false,
	}

	return c.SendEmail(ctx, req)
}

// ScheduleEmail programa un email para envío futuro
func (c *Client) ScheduleEmail(ctx context.Context, to []string, subject, body string, scheduledAt time.Time) (*SendEmailResponse, error) {
	req := &SendEmailRequest{
		Type:        NotificationTypeScheduled,
		Priority:    NotificationPriorityNormal,
		To:          to,
		Subject:     subject,
		Body:        body,
		IsHTML:      false,
		ScheduledAt: &scheduledAt,
	}

	return c.SendEmail(ctx, req)
}

// GetFailedNotifications obtiene notificaciones fallidas
func (c *Client) GetFailedNotifications(ctx context.Context, limit int) ([]*EmailNotification, error) {
	opts := &ListNotificationsOptions{
		Status: NotificationStatusFailed,
		Limit:  limit,
	}

	return c.ListNotifications(ctx, opts)
}

// GetPendingNotifications obtiene notificaciones pendientes
func (c *Client) GetPendingNotifications(ctx context.Context, limit int) ([]*EmailNotification, error) {
	opts := &ListNotificationsOptions{
		Status: NotificationStatusPending,
		Limit:  limit,
	}

	return c.ListNotifications(ctx, opts)
}

// ===========================================
// EJEMPLO DE USO
// ===========================================

/*
Ejemplo de uso del cliente:

```go
package main

import (
	"context"
	"log"
	"time"
	"yourapp/pkg/notificationclient"
)

func main() {
	// Crear cliente
	client := notificationclient.NewClient(notificationclient.ClientConfig{
		BaseURL: "http://localhost:8081",
		Token:   "your-jwt-token",
		Timeout: 30 * time.Second,
	})

	ctx := context.Background()

	// Enviar email simple
	response, err := client.SendSimpleEmail(
		ctx,
		[]string{"usuario@ejemplo.com"},
		"Prueba de email",
		"Este es un email de prueba.",
	)
	if err != nil {
		log.Fatalf("Error enviando email: %v", err)
	}

	log.Printf("Email enviado con ID: %s", response.NotificationID)

	// Enviar email de bienvenida
	err = client.SendWelcomeEmail(
		ctx,
		"nuevo@ejemplo.com",
		"Juan Pérez",
		"Mi Sistema",
	)
	if err != nil {
		log.Fatalf("Error enviando bienvenida: %v", err)
	}

	// Obtener estadísticas
	stats, err := client.GetStats(ctx, 7*24*time.Hour)
	if err != nil {
		log.Fatalf("Error obteniendo estadísticas: %v", err)
	}

	log.Printf("Total notificaciones (7 días): %d", stats.TotalNotifications)
	log.Printf("Tasa de éxito: %.2f%%", stats.SuccessRate*100)

	// Verificar salud del sistema
	health, err := client.GetHealth(ctx)
	if err != nil {
		log.Fatalf("Error verificando salud: %v", err)
	}

	log.Printf("Estado del sistema: %s", health.Status)
}
```
*/
