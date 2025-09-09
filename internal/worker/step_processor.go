package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
)

// StepProcessor procesa diferentes tipos de pasos
type StepProcessor struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewStepProcessor crea un nuevo procesador de pasos
func NewStepProcessor(logger *zap.Logger) *StepProcessor {
	return &StepProcessor{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ProcessStep procesa un paso según su tipo
func (p *StepProcessor) ProcessStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	p.logger.Info("Processing step",
		zap.String("step_id", step.ID),
		zap.String("step_type", step.Type))

	switch models.ActionType(step.Type) {
	case models.ActionTypeHTTP:
		return p.processHTTPStep(ctx, step, execCtx)
	case models.ActionTypeEmail:
		return p.processEmailStep(ctx, step, execCtx)
	case models.ActionTypeSlack:
		return p.processSlackStep(ctx, step, execCtx)
	case models.ActionTypeWebhook:
		return p.processWebhookStep(ctx, step, execCtx)
	case models.ActionTypeCondition:
		return p.processConditionStep(ctx, step, execCtx)
	case models.ActionTypeDelay:
		return p.processDelayStep(ctx, step, execCtx)
	case models.ActionTypeTransform:
		return p.processTransformStep(ctx, step, execCtx)
	case models.ActionTypeNotification:
		return p.processNotificationStep(ctx, step, execCtx)
	default:
		return nil, fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

// processHTTPStep procesa pasos de HTTP request
func (p *StepProcessor) processHTTPStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	// Extraer configuración HTTP
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("HTTP step missing required 'url' field")
	}

	method, ok := config["method"].(string)
	if !ok {
		method = "GET"
	}

	// Preparar headers
	headers := make(map[string]string)
	if h, ok := config["headers"].(map[string]interface{}); ok {
		for key, value := range h {
			if strVal, ok := value.(string); ok {
				headers[key] = strVal
			}
		}
	}

	// Preparar body
	var body []byte
	if bodyData, ok := config["body"]; ok && bodyData != nil {
		if bodyStr, ok := bodyData.(string); ok {
			body = []byte(bodyStr)
		} else {
			// Convertir a JSON si no es string
			jsonBody, err := json.Marshal(bodyData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal HTTP body: %w", err)
			}
			body = jsonBody
			headers["Content-Type"] = "application/json"
		}
	}

	// Crear request
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Agregar headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Ejecutar request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Leer respuesta
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response: %w", err)
	}

	// Verificar status code
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	result := &StepResult{
		Success: success,
		Output: map[string]interface{}{
			"status_code": resp.StatusCode,
			"headers":     resp.Header,
			"body":        string(respBody),
			"url":         url,
			"method":      method,
		},
	}

	if !success {
		result.ErrorMessage = fmt.Sprintf("HTTP request failed with status %d", resp.StatusCode)
	}

	return result, nil
}

// processEmailStep procesa pasos de envío de email
func (p *StepProcessor) processEmailStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	to, ok := config["to"].(string)
	if !ok || to == "" {
		return nil, fmt.Errorf("email step missing required 'to' field")
	}

	subject, ok := config["subject"].(string)
	if !ok {
		subject = "Workflow Notification"
	}

	body, ok := config["body"].(string)
	if !ok {
		body = "This is an automated message from your workflow."
	}

	// Por ahora, simular envío de email
	p.logger.Info("Simulating email send",
		zap.String("to", to),
		zap.String("subject", subject))

	// TODO: Implementar envío real de email con servicio como SendGrid, SES, etc.

	return &StepResult{
		Success: true,
		Output: map[string]interface{}{
			"to":      to,
			"subject": subject,
			"body":    body,
			"sent_at": time.Now(),
			"status":  "simulated", // Cambiar a "sent" cuando se implemente
		},
	}, nil
}

// processSlackStep procesa pasos de notificación Slack
func (p *StepProcessor) processSlackStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("slack step missing required 'webhook_url' field")
	}

	message, ok := config["message"].(string)
	if !ok {
		message = "Workflow notification"
	}

	channel, _ := config["channel"].(string)
	username, _ := config["username"].(string)

	// Preparar payload de Slack
	payload := map[string]interface{}{
		"text": message,
	}

	if channel != "" {
		payload["channel"] = channel
	}

	if username != "" {
		payload["username"] = username
	}

	// Convertir a JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// Enviar a Slack
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	success := resp.StatusCode == 200

	result := &StepResult{
		Success: success,
		Output: map[string]interface{}{
			"message":     message,
			"channel":     channel,
			"status_code": resp.StatusCode,
			"sent_at":     time.Now(),
		},
	}

	if !success {
		result.ErrorMessage = fmt.Sprintf("Slack API returned status %d", resp.StatusCode)
	}

	return result, nil
}

// processWebhookStep procesa pasos de webhook saliente
func (p *StepProcessor) processWebhookStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	// Similar a HTTP pero con configuración específica de webhook
	return p.processHTTPStep(ctx, step, execCtx)
}

// processConditionStep procesa pasos condicionales
func (p *StepProcessor) processConditionStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	// Extraer configuración de condición
	field, ok := config["field"].(string)
	if !ok {
		return nil, fmt.Errorf("condition step missing required 'field' field")
	}

	operator, ok := config["operator"].(string)
	if !ok {
		return nil, fmt.Errorf("condition step missing required 'operator' field")
	}

	expectedValue := config["value"]
	trueStepID, _ := config["true_step"].(string)
	falseStepID, _ := config["false_step"].(string)

	// Obtener valor del campo de las variables
	actualValue, exists := execCtx.Variables[field]
	if !exists {
		// Buscar en trigger data
		if triggerData, ok := execCtx.Variables["trigger_data"].(map[string]interface{}); ok {
			actualValue = triggerData[field]
		}
	}

	// Evaluar condición
	conditionMet := p.evaluateCondition(actualValue, operator, expectedValue)

	nextStepID := ""
	if conditionMet && trueStepID != "" {
		nextStepID = trueStepID
	} else if !conditionMet && falseStepID != "" {
		nextStepID = falseStepID
	}

	return &StepResult{
		Success:    true,
		NextStepID: nextStepID,
		Output: map[string]interface{}{
			"field":          field,
			"operator":       operator,
			"expected_value": expectedValue,
			"actual_value":   actualValue,
			"condition_met":  conditionMet,
			"next_step":      nextStepID,
		},
	}, nil
}

// processDelayStep procesa pasos de delay
func (p *StepProcessor) processDelayStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	// Extraer duración del delay
	var duration time.Duration

	if durationStr, ok := config["duration"].(string); ok {
		var err error
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			return nil, fmt.Errorf("invalid duration format: %w", err)
		}
	} else if seconds, ok := config["seconds"].(float64); ok {
		duration = time.Duration(seconds) * time.Second
	} else {
		return nil, fmt.Errorf("delay step missing duration configuration")
	}

	p.logger.Info("Executing delay step", zap.Duration("duration", duration))

	// Ejecutar delay con posibilidad de cancelación
	select {
	case <-time.After(duration):
		return &StepResult{
			Success: true,
			Output: map[string]interface{}{
				"duration":   duration.String(),
				"delayed_at": time.Now(),
			},
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("delay step cancelled due to context cancellation")
	}
}

// processTransformStep procesa pasos de transformación de datos
func (p *StepProcessor) processTransformStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	// Extraer configuración de transformación
	sourceField, ok := config["source_field"].(string)
	if !ok {
		return nil, fmt.Errorf("transform step missing required 'source_field' field")
	}

	targetField, ok := config["target_field"].(string)
	if !ok {
		targetField = sourceField + "_transformed"
	}

	operation, ok := config["operation"].(string)
	if !ok {
		operation = "copy"
	}

	// Obtener valor del campo fuente
	sourceValue := execCtx.Variables[sourceField]

	// Realizar transformación según la operación
	var transformedValue interface{}

	switch operation {
	case "copy":
		transformedValue = sourceValue
	case "uppercase":
		if str, ok := sourceValue.(string); ok {
			transformedValue = strings.ToUpper(str)
		} else {
			transformedValue = sourceValue
		}
	case "lowercase":
		if str, ok := sourceValue.(string); ok {
			transformedValue = strings.ToLower(str)
		} else {
			transformedValue = sourceValue
		}
	case "json_parse":
		if str, ok := sourceValue.(string); ok {
			var parsed interface{}
			if err := json.Unmarshal([]byte(str), &parsed); err == nil {
				transformedValue = parsed
			} else {
				return nil, fmt.Errorf("failed to parse JSON: %w", err)
			}
		} else {
			transformedValue = sourceValue
		}
	case "json_stringify":
		jsonBytes, err := json.Marshal(sourceValue)
		if err != nil {
			return nil, fmt.Errorf("failed to stringify to JSON: %w", err)
		}
		transformedValue = string(jsonBytes)
	default:
		return nil, fmt.Errorf("unsupported transformation operation: %s", operation)
	}

	// Guardar valor transformado en variables
	execCtx.Variables[targetField] = transformedValue

	return &StepResult{
		Success: true,
		Output: map[string]interface{}{
			"source_field":      sourceField,
			"target_field":      targetField,
			"operation":         operation,
			"source_value":      sourceValue,
			"transformed_value": transformedValue,
		},
	}, nil
}

// processNotificationStep procesa pasos de notificación genérica
func (p *StepProcessor) processNotificationStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	config := step.Config

	notificationType, ok := config["type"].(string)
	if !ok {
		notificationType = "general"
	}

	message, ok := config["message"].(string)
	if !ok {
		message = "Workflow notification"
	}

	// Por ahora, registrar la notificación
	p.logger.Info("Processing notification",
		zap.String("type", notificationType),
		zap.String("message", message))

	// TODO: Implementar diferentes tipos de notificaciones
	// - push notifications
	// - SMS
	// - webhook notifications
	// etc.

	return &StepResult{
		Success: true,
		Output: map[string]interface{}{
			"type":         notificationType,
			"message":      message,
			"processed_at": time.Now(),
			"status":       "processed",
		},
	}, nil
}

// evaluateCondition evalúa una condición específica
func (p *StepProcessor) evaluateCondition(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "eq", "equals":
		return actual == expected
	case "neq", "not_equals":
		return actual != expected
	case "gt", "greater_than":
		return p.compareNumbers(actual, expected, ">")
	case "gte", "greater_than_or_equal":
		return p.compareNumbers(actual, expected, ">=")
	case "lt", "less_than":
		return p.compareNumbers(actual, expected, "<")
	case "lte", "less_than_or_equal":
		return p.compareNumbers(actual, expected, "<=")
	case "contains":
		if actualStr, ok := actual.(string); ok {
			if expectedStr, ok := expected.(string); ok {
				return strings.Contains(actualStr, expectedStr)
			}
		}
		return false
	case "not_contains":
		if actualStr, ok := actual.(string); ok {
			if expectedStr, ok := expected.(string); ok {
				return !strings.Contains(actualStr, expectedStr)
			}
		}
		return false
	case "empty":
		return actual == nil || actual == ""
	case "not_empty":
		return actual != nil && actual != ""
	default:
		p.logger.Warn("Unknown condition operator", zap.String("operator", operator))
		return false
	}
}

// compareNumbers compara dos valores numéricos
func (p *StepProcessor) compareNumbers(actual, expected interface{}, operator string) bool {
	actualFloat := p.toFloat64(actual)
	expectedFloat := p.toFloat64(expected)

	if actualFloat == nil || expectedFloat == nil {
		return false
	}

	switch operator {
	case ">":
		return *actualFloat > *expectedFloat
	case ">=":
		return *actualFloat >= *expectedFloat
	case "<":
		return *actualFloat < *expectedFloat
	case "<=":
		return *actualFloat <= *expectedFloat
	default:
		return false
	}
}

// toFloat64 convierte un valor a float64 si es posible
func (p *StepProcessor) toFloat64(value interface{}) *float64 {
	switch v := value.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	default:
		return nil
	}
}
