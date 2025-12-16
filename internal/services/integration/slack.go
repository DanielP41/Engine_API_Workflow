package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// SlackService interface para integración con Slack
type SlackService interface {
	SendMessage(ctx context.Context, channel, message string) error
	SendRichMessage(ctx context.Context, channel string, blocks []SlackBlock) error
	UploadFile(ctx context.Context, channel string, filename string, content []byte, filetype string) error
	GetChannelInfo(ctx context.Context, channelID string) (*SlackChannelInfo, error)
}

// SlackBlock representa un bloque de mensaje rico de Slack
type SlackBlock struct {
	Type string                 `json:"type"`
	Text *SlackText             `json:"text,omitempty"`
	Fields []SlackField         `json:"fields,omitempty"`
	Elements []SlackElement      `json:"elements,omitempty"`
	Accessory *SlackAccessory    `json:"accessory,omitempty"`
	Title *SlackText            `json:"title,omitempty"`
	ImageURL string              `json:"image_url,omitempty"`
	AltText string               `json:"alt_text,omitempty"`
}

// SlackText representa texto en un bloque de Slack
type SlackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// SlackField representa un campo en un bloque de Slack
type SlackField struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Short bool  `json:"short,omitempty"`
}

// SlackElement representa un elemento en un bloque de Slack
type SlackElement struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

// SlackAccessory representa un accesorio en un bloque de Slack
type SlackAccessory struct {
	Type string `json:"type"`
	Text *SlackText `json:"text,omitempty"`
	URL  string     `json:"url,omitempty"`
}

// SlackChannelInfo información de un canal de Slack
type SlackChannelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsChannel bool  `json:"is_channel"`
	IsPrivate bool  `json:"is_private"`
	NumMembers int `json:"num_members"`
}

// SlackConfig configuración del servicio de Slack
type SlackConfig struct {
	BotToken     string
	WebhookURL   string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
}

// slackService implementación del servicio de Slack
type slackService struct {
	client *http.Client
	config SlackConfig
	logger *zap.Logger
}

// NewSlackService crea un nuevo servicio de Slack
func NewSlackService(config SlackConfig, logger *zap.Logger) SlackService {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 1 * time.Second
	}

	return &slackService{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config: config,
		logger: logger,
	}
}

// SendMessage envía un mensaje simple a un canal de Slack
func (s *slackService) SendMessage(ctx context.Context, channel, message string) error {
	if channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	// Si hay webhook URL, usar webhook (más simple)
	if s.config.WebhookURL != "" {
		return s.sendViaWebhook(ctx, channel, message)
	}

	// Si hay bot token, usar Web API
	if s.config.BotToken != "" {
		return s.sendViaAPI(ctx, channel, message)
	}

	return fmt.Errorf("either SLACK_BOT_TOKEN or SLACK_WEBHOOK_URL must be configured")
}

// sendViaWebhook envía mensaje usando webhook de Slack
func (s *slackService) sendViaWebhook(ctx context.Context, channel, message string) error {
	payload := map[string]interface{}{
		"channel": channel,
		"text":    message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	s.logger.Info("Sending Slack message via webhook",
		zap.String("channel", channel))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendViaAPI envía mensaje usando Slack Web API
func (s *slackService) sendViaAPI(ctx context.Context, channel, message string) error {
	payload := map[string]interface{}{
		"channel": channel,
		"text":    message,
	}

	return s.callSlackAPI(ctx, "chat.postMessage", payload)
}

// SendRichMessage envía un mensaje con bloques ricos
func (s *slackService) SendRichMessage(ctx context.Context, channel string, blocks []SlackBlock) error {
	if channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if len(blocks) == 0 {
		return fmt.Errorf("blocks cannot be empty")
	}

	if s.config.BotToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN is required for rich messages")
	}

	payload := map[string]interface{}{
		"channel": channel,
		"blocks":  blocks,
	}

	return s.callSlackAPI(ctx, "chat.postMessage", payload)
}

// UploadFile sube un archivo a un canal de Slack
func (s *slackService) UploadFile(ctx context.Context, channel string, filename string, content []byte, filetype string) error {
	if channel == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if len(content) == 0 {
		return fmt.Errorf("file content cannot be empty")
	}

	if s.config.BotToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN is required for file uploads")
	}

	// Usar files.upload API
	payload := map[string]interface{}{
		"channels": channel,
		"filename": filename,
		"file":     content,
	}

	if filetype != "" {
		payload["filetype"] = filetype
	}

	return s.callSlackFilesAPI(ctx, "files.upload", payload)
}

// GetChannelInfo obtiene información de un canal
func (s *slackService) GetChannelInfo(ctx context.Context, channelID string) (*SlackChannelInfo, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channel ID cannot be empty")
	}

	if s.config.BotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is required to get channel info")
	}

	payload := map[string]interface{}{
		"channel": channelID,
	}

	var result struct {
		OK      bool              `json:"ok"`
		Channel *SlackChannelInfo `json:"channel"`
		Error   string            `json:"error,omitempty"`
	}

	if err := s.callSlackAPIRaw(ctx, "conversations.info", payload, &result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("slack API error: %s", result.Error)
	}

	return result.Channel, nil
}

// callSlackAPI llama a la API de Slack
func (s *slackService) callSlackAPI(ctx context.Context, method string, payload map[string]interface{}) error {
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	return s.callSlackAPIRaw(ctx, method, payload, &result)
}

// callSlackAPIRaw llama a la API de Slack y parsea la respuesta
func (s *slackService) callSlackAPIRaw(ctx context.Context, method string, payload map[string]interface{}, result interface{}) error {
	url := fmt.Sprintf("https://slack.com/api/%s", method)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.BotToken)
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Calling Slack API",
		zap.String("method", method))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Slack API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// callSlackFilesAPI llama a la API de archivos de Slack (multipart/form-data)
func (s *slackService) callSlackFilesAPI(ctx context.Context, method string, payload map[string]interface{}) error {
	url := fmt.Sprintf("https://slack.com/api/%s", method)

	var body bytes.Buffer
	writer := &multipartWriter{body: &body}

	// Agregar campos del formulario
	for key, value := range payload {
		if key == "file" {
			// El archivo se maneja de forma especial
			continue
		}
		if err := writer.WriteField(key, fmt.Sprintf("%v", value)); err != nil {
			return fmt.Errorf("failed to write field: %w", err)
		}
	}

	// Agregar archivo si existe
	if fileContent, ok := payload["file"].([]byte); ok {
		if err := writer.WriteFile("file", payload["filename"].(string), fileContent); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.BotToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	s.logger.Debug("Calling Slack Files API",
		zap.String("method", method))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Slack Files API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("slack API error: %s", result.Error)
	}

	return nil
}

// multipartWriter es un writer simple para multipart/form-data
type multipartWriter struct {
	body   *bytes.Buffer
	boundary string
	first  bool
}

func (w *multipartWriter) WriteField(name, value string) error {
	if w.boundary == "" {
		w.boundary = fmt.Sprintf("----WebKitFormBoundary%d", time.Now().UnixNano())
		w.body.WriteString("--" + w.boundary + "\r\n")
		w.first = true
	}

	if !w.first {
		w.body.WriteString("\r\n")
	}
	w.first = false

	w.body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"\r\n\r\n%s", name, value))
	return nil
}

func (w *multipartWriter) WriteFile(fieldName, filename string, content []byte) error {
	if w.boundary == "" {
		w.boundary = fmt.Sprintf("----WebKitFormBoundary%d", time.Now().UnixNano())
		w.body.WriteString("--" + w.boundary + "\r\n")
		w.first = true
	}

	if !w.first {
		w.body.WriteString("\r\n")
	}
	w.first = false

	w.body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldName, filename))
	w.body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	w.body.Write(content)
	return nil
}

func (w *multipartWriter) Close() {
	if w.boundary != "" {
		w.body.WriteString("\r\n--" + w.boundary + "--\r\n")
	}
}

func (w *multipartWriter) FormDataContentType() string {
	if w.boundary == "" {
		w.boundary = fmt.Sprintf("----WebKitFormBoundary%d", time.Now().UnixNano())
	}
	return "multipart/form-data; boundary=" + w.boundary
}
