package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// WebhookService interface para envío de webhooks
type WebhookService interface {
	SendWebhook(ctx context.Context, url string, payload interface{}, headers map[string]string) error
	SendWebhookWithMethod(ctx context.Context, method, webhookURL string, payload interface{}, headers map[string]string) error
	ValidateWebhookURL(webhookURL string) error
	RetryWebhook(ctx context.Context, webhookURL string, payload interface{}, headers map[string]string, maxRetries int) error
}

// WebhookConfig configuración del servicio de webhooks
type WebhookConfig struct {
	Timeout           time.Duration
	MaxRetries        int
	RetryBackoff      time.Duration
	InsecureSSL       bool
	FollowRedirects   bool
	MaxRedirects      int
	UserAgent         string
	DefaultHeaders    map[string]string
}

// webhookService implementación del servicio de webhooks
type webhookService struct {
	client *http.Client
	config WebhookConfig
	logger *zap.Logger
}

// NewWebhookService crea un nuevo servicio de webhooks
func NewWebhookService(config WebhookConfig, logger *zap.Logger) WebhookService {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 1 * time.Second
	}
	if config.MaxRedirects == 0 {
		config.MaxRedirects = 5
	}
	if config.UserAgent == "" {
		config.UserAgent = "Engine-API-Workflow/2.0"
	}

	// Configurar transporte HTTP
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSSL,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Configurar cliente HTTP
	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	// Configurar redirects
	if !config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", config.MaxRedirects)
			}
			return nil
		}
	}

	return &webhookService{
		client: client,
		config: config,
		logger: logger,
	}
}

// ValidateWebhookURL valida que la URL del webhook sea válida y segura
func (w *webhookService) ValidateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL format: %w", err)
	}

	// Validar esquema
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	// Advertir sobre HTTP en producción (pero permitirlo)
	if parsedURL.Scheme == "http" {
		w.logger.Warn("Webhook URL uses HTTP instead of HTTPS",
			zap.String("url", webhookURL))
	}

	// Validar host
	if parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must have a valid host")
	}

	// Validar que no sea localhost en producción (si se puede detectar)
	if strings.Contains(parsedURL.Host, "localhost") || strings.Contains(parsedURL.Host, "127.0.0.1") {
		w.logger.Warn("Webhook URL points to localhost",
			zap.String("url", webhookURL))
	}

	// Validar que no sea una IP privada (básico)
	if strings.HasPrefix(parsedURL.Host, "192.168.") ||
		strings.HasPrefix(parsedURL.Host, "10.") ||
		strings.HasPrefix(parsedURL.Host, "172.") {
		w.logger.Warn("Webhook URL points to private IP range",
			zap.String("url", webhookURL))
	}

	return nil
}

// SendWebhook envía un webhook usando POST por defecto
func (w *webhookService) SendWebhook(ctx context.Context, webhookURL string, payload interface{}, headers map[string]string) error {
	return w.SendWebhookWithMethod(ctx, http.MethodPost, webhookURL, payload, headers)
}

// SendWebhookWithMethod envía un webhook con método HTTP específico
func (w *webhookService) SendWebhookWithMethod(ctx context.Context, method, webhookURL string, payload interface{}, headers map[string]string) error {
	// Validar URL
	if err := w.ValidateWebhookURL(webhookURL); err != nil {
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// Validar método HTTP
	method = strings.ToUpper(method)
	validMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch}
	isValid := false
	for _, validMethod := range validMethods {
		if method == validMethod {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid HTTP method: %s (allowed: POST, PUT, PATCH)", method)
	}

	// Preparar body
	var body io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	}

	// Crear request
	req, err := http.NewRequestWithContext(ctx, method, webhookURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Agregar headers por defecto
	req.Header.Set("User-Agent", w.config.UserAgent)
	req.Header.Set("Content-Type", "application/json")

	// Agregar headers personalizados
	if w.config.DefaultHeaders != nil {
		for key, value := range w.config.DefaultHeaders {
			req.Header.Set(key, value)
		}
	}

	// Agregar headers del request
	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	// Log request
	w.logger.Info("Sending webhook",
		zap.String("method", method),
		zap.String("url", webhookURL),
		zap.Int("headers_count", len(req.Header)))

	// Ejecutar request
	startTime := time.Now()
	resp, err := w.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		w.logger.Error("Webhook request failed",
			zap.String("url", webhookURL),
			zap.Error(err),
			zap.Duration("duration", duration))
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	// Leer respuesta
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		w.logger.Error("Failed to read webhook response",
			zap.String("url", webhookURL),
			zap.Error(err))
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Verificar status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.logger.Warn("Webhook returned non-success status",
			zap.String("url", webhookURL),
			zap.Int("status_code", resp.StatusCode),
			zap.String("status", resp.Status),
			zap.String("response_body", string(respBody)),
			zap.Duration("duration", duration))
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, resp.Status)
	}

	// Log success
	w.logger.Info("Webhook sent successfully",
		zap.String("url", webhookURL),
		zap.Int("status_code", resp.StatusCode),
		zap.Int("response_size", len(respBody)),
		zap.Duration("duration", duration))

	return nil
}

// RetryWebhook envía un webhook con reintentos automáticos
func (w *webhookService) RetryWebhook(ctx context.Context, webhookURL string, payload interface{}, headers map[string]string, maxRetries int) error {
	if maxRetries <= 0 {
		maxRetries = w.config.MaxRetries
	}

	var lastErr error
	backoff := w.config.RetryBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Esperar antes de reintentar
			w.logger.Info("Retrying webhook",
				zap.String("url", webhookURL),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				// Exponential backoff
				backoff = time.Duration(float64(backoff) * 1.5)
			}
		}

		err := w.SendWebhook(ctx, webhookURL, payload, headers)
		if err == nil {
			if attempt > 0 {
				w.logger.Info("Webhook succeeded after retry",
					zap.String("url", webhookURL),
					zap.Int("attempts", attempt+1))
			}
			return nil
		}

		lastErr = err
		w.logger.Warn("Webhook attempt failed",
			zap.String("url", webhookURL),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", maxRetries),
			zap.Error(err))
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", maxRetries+1, lastErr)
}

