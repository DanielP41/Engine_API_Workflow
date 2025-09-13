package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"Engine_API_Workflow/internal/models"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

// HTTPActionExecutor ejecuta acciones HTTP reales
type HTTPActionExecutor struct {
	client *http.Client
	logger *zap.Logger
}

// NewHTTPActionExecutor crea un nuevo ejecutor de acciones HTTP
func NewHTTPActionExecutor(logger *zap.Logger) *HTTPActionExecutor {
	return &HTTPActionExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Execute ejecuta una acción HTTP real
func (h *HTTPActionExecutor) Execute(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	result := &StepResult{
		Success: false,
		Output:  make(map[string]interface{}),
	}

	// Extraer configuración HTTP del step
	httpConfig, err := h.parseHTTPConfig(step.Config)
	if err != nil {
		return result, fmt.Errorf("invalid HTTP configuration: %w", err)
	}

	// Reemplazar variables en la configuración
	err = h.replaceVariables(httpConfig, execCtx.Variables)
	if err != nil {
		return result, fmt.Errorf("failed to replace variables: %w", err)
	}

	// Crear request HTTP
	req, err := h.createHTTPRequest(ctx, httpConfig)
	if err != nil {
		return result, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Ejecutar request
	h.logger.Info("Executing HTTP request",
		zap.String("method", httpConfig.Method),
		zap.String("url", httpConfig.URL))

	startTime := time.Now()
	resp, err := h.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		result.ErrorMessage = err.Error()
		result.Output["error"] = err.Error()
		result.Output["duration_ms"] = duration.Milliseconds()
		return result, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Leer respuesta
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.ErrorMessage = "Failed to read response body"
		result.Output["error"] = result.ErrorMessage
		return result, err
	}

	// Procesar respuesta
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	result.Output["status_code"] = resp.StatusCode
	result.Output["status"] = resp.Status
	result.Output["duration_ms"] = duration.Milliseconds()
	result.Output["response_size"] = len(body)

	// Intentar parsear JSON
	var jsonResponse interface{}
	if err := json.Unmarshal(body, &jsonResponse); err == nil {
		result.Output["response"] = jsonResponse
	} else {
		result.Output["response"] = string(body)
	}

	// Headers de respuesta
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	result.Output["headers"] = headers

	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("HTTP request failed with status %d", resp.StatusCode)
		return result, fmt.Errorf(result.ErrorMessage)
	}

	return result, nil
}

// HTTPConfig configuración para acciones HTTP
type HTTPConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
	Timeout int               `json:"timeout_seconds"`
}

func (h *HTTPActionExecutor) parseHTTPConfig(config map[string]interface{}) (*HTTPConfig, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var httpConfig HTTPConfig
	err = json.Unmarshal(configBytes, &httpConfig)
	if err != nil {
		return nil, err
	}

	// Validaciones
	if httpConfig.Method == "" {
		httpConfig.Method = "GET"
	}
	if httpConfig.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if httpConfig.Timeout == 0 {
		httpConfig.Timeout = 30
	}

	return &httpConfig, nil
}

func (h *HTTPActionExecutor) replaceVariables(config *HTTPConfig, variables map[string]interface{}) error {
	// Reemplazar variables en URL
	config.URL = h.replaceVariablesInString(config.URL, variables)

	// Reemplazar variables en headers
	for key, value := range config.Headers {
		config.Headers[key] = h.replaceVariablesInString(value, variables)
	}

	// Reemplazar variables en body si es string
	if bodyStr, ok := config.Body.(string); ok {
		config.Body = h.replaceVariablesInString(bodyStr, variables)
	}

	return nil
}

func (h *HTTPActionExecutor) replaceVariablesInString(text string, variables map[string]interface{}) string {
	result := text
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		valueStr := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}
	return result
}

func (h *HTTPActionExecutor) createHTTPRequest(ctx context.Context, config *HTTPConfig) (*http.Request, error) {
	var body io.Reader

	if config.Body != nil {
		switch v := config.Body.(type) {
		case string:
			body = strings.NewReader(v)
		default:
			jsonData, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(jsonData)
		}
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, body)
	if err != nil {
		return nil, err
	}

	// Establecer headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Headers por defecto
	if req.Header.Get("Content-Type") == "" && config.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Engine-API-Workflow/1.0")
	}

	return req, nil
}

// SMTP CONFIG
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
}

// EmailActionExecutor ejecuta acciones de email REALES
type EmailActionExecutor struct {
	logger     *zap.Logger
	smtpConfig *SMTPConfig
}

func NewEmailActionExecutor(logger *zap.Logger) *EmailActionExecutor {
	// Cargar configuración SMTP desde variables de entorno
	smtpConfig := &SMTPConfig{
		Host:      getEnvOrDefault("SMTP_HOST", "smtp.gmail.com"),
		Port:      getEnvIntOrDefault("SMTP_PORT", 587),
		Username:  getEnvOrDefault("SMTP_USERNAME", ""),
		Password:  getEnvOrDefault("SMTP_PASSWORD", ""),
		FromName:  getEnvOrDefault("SMTP_FROM_NAME", "Engine API Workflow"),
		FromEmail: getEnvOrDefault("SMTP_FROM_EMAIL", ""),
	}

	// Si FromEmail no está configurado, usar Username
	if smtpConfig.FromEmail == "" {
		smtpConfig.FromEmail = smtpConfig.Username
	}

	return &EmailActionExecutor{
		logger:     logger,
		smtpConfig: smtpConfig,
	}
}

func (e *EmailActionExecutor) Execute(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	result := &StepResult{
		Success: false,
		Output:  make(map[string]interface{}),
	}

	// Extraer configuración de email
	emailConfig, err := e.parseEmailConfig(step.Config)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Invalid email configuration: %v", err)
		return result, fmt.Errorf("invalid email configuration: %w", err)
	}

	// Reemplazar variables
	e.replaceVariables(emailConfig, execCtx.Variables)

	// Verificar configuración SMTP
	if !e.isSMTPConfigured() {
		e.logger.Warn("SMTP not configured, simulating email send")
		return e.simulateEmail(emailConfig)
	}

	// Enviar email real
	e.logger.Info("Sending real email",
		zap.String("to", emailConfig.To),
		zap.String("subject", emailConfig.Subject),
		zap.String("smtp_host", e.smtpConfig.Host))

	startTime := time.Now()
	err = e.sendRealEmail(emailConfig)
	duration := time.Since(startTime)

	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to send email: %v", err)
		result.Output["error"] = err.Error()
		result.Output["duration_ms"] = duration.Milliseconds()
		return result, fmt.Errorf("failed to send email: %w", err)
	}

	// Email enviado exitosamente
	result.Success = true
	result.Output["email_sent"] = true
	result.Output["to"] = emailConfig.To
	result.Output["subject"] = emailConfig.Subject
	result.Output["from"] = e.smtpConfig.FromEmail
	result.Output["message"] = "Email sent successfully"
	result.Output["duration_ms"] = duration.Milliseconds()
	result.Output["timestamp"] = time.Now()
	result.Output["smtp_host"] = e.smtpConfig.Host

	e.logger.Info("Email sent successfully",
		zap.String("to", emailConfig.To),
		zap.Duration("duration", duration))

	return result, nil
}

// sendRealEmail envía un email real usando SMTP
func (e *EmailActionExecutor) sendRealEmail(config *EmailConfig) error {
	// Crear mensaje
	m := gomail.NewMessage()

	// Configurar remitente
	fromAddress := e.smtpConfig.FromEmail
	if e.smtpConfig.FromName != "" {
		fromAddress = fmt.Sprintf("%s <%s>", e.smtpConfig.FromName, e.smtpConfig.FromEmail)
	}
	m.SetHeader("From", fromAddress)

	// Configurar destinatario
	m.SetHeader("To", config.To)

	// Configurar CC si existe
	if len(config.CC) > 0 {
		m.SetHeader("Cc", config.CC...)
	}

	// Configurar BCC si existe
	if len(config.BCC) > 0 {
		m.SetHeader("Bcc", config.BCC...)
	}

	// Configurar asunto
	m.SetHeader("Subject", config.Subject)

	// Configurar cuerpo
	if config.IsHTML {
		m.SetBody("text/html", config.Body)
	} else {
		m.SetBody("text/plain", config.Body)
	}

	// Si hay archivos adjuntos
	for _, attachment := range config.Attachments {
		if attachment != "" {
			m.Attach(attachment)
		}
	}

	// Crear dialer SMTP
	d := gomail.NewDialer(
		e.smtpConfig.Host,
		e.smtpConfig.Port,
		e.smtpConfig.Username,
		e.smtpConfig.Password,
	)

	// Configurar TLS para Gmail y otros proveedores
	d.TLSConfig = &tls.Config{InsecureSkipVerify: false}

	// Enviar email
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	return nil
}

// simulateEmail simula el envío cuando SMTP no está configurado
func (e *EmailActionExecutor) simulateEmail(config *EmailConfig) (*StepResult, error) {
	result := &StepResult{
		Success: true,
		Output:  make(map[string]interface{}),
	}

	time.Sleep(500 * time.Millisecond)

	result.Output["email_sent"] = true
	result.Output["to"] = config.To
	result.Output["subject"] = config.Subject
	result.Output["message"] = "Email sent successfully (simulated - SMTP not configured)"
	result.Output["timestamp"] = time.Now()
	result.Output["mode"] = "simulated"

	return result, nil
}

// isSMTPConfigured verifica si SMTP está configurado
func (e *EmailActionExecutor) isSMTPConfigured() bool {
	return e.smtpConfig.Host != "" &&
		e.smtpConfig.Username != "" &&
		e.smtpConfig.Password != ""
}

// EmailConfig configuración de email MEJORADA
type EmailConfig struct {
	To          string   `json:"to"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	From        string   `json:"from"`        // Opcional, usa configuración por defecto
	IsHTML      bool     `json:"is_html"`     // Soporte para HTML
	Attachments []string `json:"attachments"` // Archivos adjuntos
	CC          []string `json:"cc"`          // Copia
	BCC         []string `json:"bcc"`         // Copia oculta
}

func (e *EmailActionExecutor) parseEmailConfig(config map[string]interface{}) (*EmailConfig, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var emailConfig EmailConfig
	err = json.Unmarshal(configBytes, &emailConfig)
	if err != nil {
		return nil, err
	}

	// Validaciones
	if emailConfig.To == "" {
		return nil, fmt.Errorf("'to' field is required")
	}
	if emailConfig.Subject == "" {
		return nil, fmt.Errorf("'subject' field is required")
	}
	if emailConfig.Body == "" {
		return nil, fmt.Errorf("'body' field is required")
	}

	return &emailConfig, nil
}

func (e *EmailActionExecutor) replaceVariables(config *EmailConfig, variables map[string]interface{}) {
	config.To = e.replaceInString(config.To, variables)
	config.Subject = e.replaceInString(config.Subject, variables)
	config.Body = e.replaceInString(config.Body, variables)
	config.From = e.replaceInString(config.From, variables)
}

func (e *EmailActionExecutor) replaceInString(text string, variables map[string]interface{}) string {
	result := text
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		valueStr := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}
	return result
}

// Funciones helper para variables de entorno
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// SlackActionExecutor ejecuta acciones de Slack
type SlackActionExecutor struct {
	client *http.Client
	logger *zap.Logger
}

func NewSlackActionExecutor(logger *zap.Logger) *SlackActionExecutor {
	return &SlackActionExecutor{
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

func (s *SlackActionExecutor) Execute(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	result := &StepResult{
		Success: false,
		Output:  make(map[string]interface{}),
	}

	// Extraer configuración de Slack
	slackConfig, err := s.parseSlackConfig(step.Config)
	if err != nil {
		return result, fmt.Errorf("invalid Slack configuration: %w", err)
	}

	// Reemplazar variables
	s.replaceVariables(slackConfig, execCtx.Variables)

	// Crear payload para Slack
	payload := map[string]interface{}{
		"text":    slackConfig.Message,
		"channel": slackConfig.Channel,
	}

	if slackConfig.Username != "" {
		payload["username"] = slackConfig.Username
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return result, fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// Enviar a Slack
	req, err := http.NewRequestWithContext(ctx, "POST", slackConfig.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return result, fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	s.logger.Info("Sending Slack message",
		zap.String("channel", slackConfig.Channel),
		zap.String("message", slackConfig.Message))

	resp, err := s.client.Do(req)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	result.Success = resp.StatusCode == 200
	result.Output["slack_sent"] = result.Success
	result.Output["status_code"] = resp.StatusCode
	result.Output["channel"] = slackConfig.Channel
	result.Output["message"] = slackConfig.Message

	if !result.Success {
		bodyBytes, _ := io.ReadAll(resp.Body)
		result.ErrorMessage = fmt.Sprintf("Slack API returned status %d: %s", resp.StatusCode, string(bodyBytes))
		return result, fmt.Errorf(result.ErrorMessage)
	}

	return result, nil
}

type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel"`
	Message    string `json:"message"`
	Username   string `json:"username"`
}

func (s *SlackActionExecutor) parseSlackConfig(config map[string]interface{}) (*SlackConfig, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var slackConfig SlackConfig
	err = json.Unmarshal(configBytes, &slackConfig)
	if err != nil {
		return nil, err
	}

	if slackConfig.WebhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}
	if slackConfig.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	return &slackConfig, nil
}

func (s *SlackActionExecutor) replaceVariables(config *SlackConfig, variables map[string]interface{}) {
	config.Message = s.replaceInString(config.Message, variables)
	config.Channel = s.replaceInString(config.Channel, variables)
	config.Username = s.replaceInString(config.Username, variables)
}

func (s *SlackActionExecutor) replaceInString(text string, variables map[string]interface{}) string {
	result := text
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		valueStr := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}
	return result
}

// WebhookActionExecutor ejecuta webhooks
type WebhookActionExecutor struct {
	httpExecutor *HTTPActionExecutor
	logger       *zap.Logger
}

func NewWebhookActionExecutor(logger *zap.Logger) *WebhookActionExecutor {
	return &WebhookActionExecutor{
		httpExecutor: NewHTTPActionExecutor(logger),
		logger:       logger,
	}
}

func (w *WebhookActionExecutor) Execute(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	// Los webhooks son esencialmente llamadas HTTP POST
	// Convertir configuración de webhook a configuración HTTP
	httpConfig := map[string]interface{}{
		"method": "POST",
		"url":    step.Config["url"],
		"body":   step.Config["payload"],
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}

	// Agregar headers personalizados si existen
	if customHeaders, ok := step.Config["headers"].(map[string]interface{}); ok {
		headers := httpConfig["headers"].(map[string]string)
		for key, value := range customHeaders {
			headers[key] = fmt.Sprintf("%v", value)
		}
	}

	// Usar el ejecutor HTTP
	step.Config = httpConfig
	result, err := w.httpExecutor.Execute(ctx, step, execCtx)

	if err == nil {
		result.Output["webhook_sent"] = true
		result.Output["type"] = "webhook"
	}

	return result, err
}
