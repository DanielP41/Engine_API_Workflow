package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
)

// StepProcessor maneja el procesamiento de pasos específicos del workflow
type StepProcessor struct {
	logger *zap.Logger
}

// NewStepProcessor crea una nueva instancia del procesador de pasos
func NewStepProcessor(logger *zap.Logger) *StepProcessor {
	return &StepProcessor{
		logger: logger,
	}
}

// ProcessStep procesa un paso individual del workflow
// CORREGIDO: Cambiar tipo de retorno de *models.StepExecution a map[string]interface{}
func (p *StepProcessor) ProcessStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}) (map[string]interface{}, error) {
	startTime := time.Now()

	logger := p.logger.With(
		zap.String("step_id", step.ID),
		zap.String("step_name", step.Name),
		zap.String("step_type", step.Type),
	)

	logger.Info("Processing step")

	// Verificar si el paso está habilitado
	if !step.IsEnabled {
		return map[string]interface{}{
			"status": "skipped",
			"reason": "step disabled",
		}, nil
	}

	// Procesar según el tipo de paso
	var result map[string]interface{}
	var err error

	switch step.Type {
	case "http":
		result, err = p.processHTTPStep(ctx, step, context, logger)
	case "email":
		result, err = p.processEmailStep(ctx, step, context, logger)
	case "slack":
		result, err = p.processSlackStep(ctx, step, context, logger)
	case "webhook":
		result, err = p.processWebhookStep(ctx, step, context, logger)
	case "delay":
		result, err = p.processDelayStep(ctx, step, context, logger)
	case "condition":
		result, err = p.processConditionStep(ctx, step, context, logger)
	case "transform":
		result, err = p.processTransformStep(ctx, step, context, logger)
	case "database":
		result, err = p.processDatabaseStep(ctx, step, context, logger)
	case "notification":
		result, err = p.processNotificationStep(ctx, step, context, logger)
	default:
		return nil, fmt.Errorf("unknown step type: %s", step.Type)
	}

	if err != nil {
		logger.Error("Step processing failed", zap.Error(err))
		return nil, err
	}

	// Agregar información de timing
	if result == nil {
		result = make(map[string]interface{})
	}
	result["processing_time_ms"] = time.Since(startTime).Milliseconds()
	result["processed_at"] = time.Now()

	logger.Info("Step processing completed successfully")
	return result, nil
}

// processHTTPStep procesa un paso HTTP
func (p *StepProcessor) processHTTPStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	url, ok := step.Config["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing url in HTTP step config")
	}

	method, _ := step.Config["method"].(string)
	if method == "" {
		method = "GET"
	}

	headers, _ := step.Config["headers"].(map[string]interface{})

	logger.Info("Processing HTTP step",
		zap.String("url", url),
		zap.String("method", method))

	// Simular HTTP request
	time.Sleep(100 * time.Millisecond)

	return map[string]interface{}{
		"status_code": 200,
		"response":    "OK",
		"url":         url,
		"method":      method,
		"headers":     headers,
	}, nil
}

// processEmailStep procesa un paso de email
func (p *StepProcessor) processEmailStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	to, ok := step.Config["to"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'to' address in email step config")
	}

	subject, _ := step.Config["subject"].(string)
	// CORREGIDO: Eliminar variable body no utilizada
	from, _ := step.Config["from"].(string)

	logger.Info("Processing email step",
		zap.String("to", to),
		zap.String("subject", subject))

	// Simular envío de email
	time.Sleep(200 * time.Millisecond)

	return map[string]interface{}{
		"sent":    true,
		"to":      to,
		"subject": subject,
		"from":    from,
	}, nil
}

// processSlackStep procesa un paso de Slack
func (p *StepProcessor) processSlackStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	channel, ok := step.Config["channel"].(string)
	if !ok {
		return nil, fmt.Errorf("missing channel in Slack step config")
	}

	message, _ := step.Config["message"].(string)
	username, _ := step.Config["username"].(string)

	logger.Info("Processing Slack step",
		zap.String("channel", channel))

	// Simular llamada a Slack API
	time.Sleep(150 * time.Millisecond)

	return map[string]interface{}{
		"sent":     true,
		"channel":  channel,
		"message":  message,
		"username": username,
	}, nil
}

// processWebhookStep procesa un paso de webhook
func (p *StepProcessor) processWebhookStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	url, ok := step.Config["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing URL in webhook step config")
	}

	method, _ := step.Config["method"].(string)
	if method == "" {
		method = "POST"
	}

	logger.Info("Processing webhook step",
		zap.String("url", url),
		zap.String("method", method))

	// Simular llamada a webhook
	time.Sleep(300 * time.Millisecond)

	return map[string]interface{}{
		"called": true,
		"url":    url,
		"method": method,
		"status": "success",
	}, nil
}

// processDelayStep procesa un paso de delay
func (p *StepProcessor) processDelayStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	delaySeconds, ok := step.Config["delay_seconds"].(float64)
	if !ok {
		delaySeconds = 1.0
	}

	duration := time.Duration(delaySeconds) * time.Second
	logger.Info("Processing delay step", zap.Duration("duration", duration))

	// Esperar con posibilidad de cancelación
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(duration):
		// Continuar
	}

	return map[string]interface{}{
		"delayed_seconds": delaySeconds,
		"completed_at":    time.Now(),
	}, nil
}

// processConditionStep procesa un paso condicional
func (p *StepProcessor) processConditionStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	condition, ok := step.Config["condition"].(string)
	if !ok {
		return nil, fmt.Errorf("missing condition in condition step config")
	}

	variable, _ := step.Config["variable"].(string)
	operator, _ := step.Config["operator"].(string)
	value, _ := step.Config["value"]

	logger.Info("Processing condition step",
		zap.String("condition", condition))

	// Evaluar condición
	result := p.evaluateCondition(condition, variable, operator, value, context)

	return map[string]interface{}{
		"condition": condition,
		"variable":  variable,
		"operator":  operator,
		"value":     value,
		"result":    result,
	}, nil
}

// processTransformStep procesa un paso de transformación
func (p *StepProcessor) processTransformStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	transform, ok := step.Config["transform"].(string)
	if !ok {
		return nil, fmt.Errorf("missing transform in transform step config")
	}

	inputField, _ := step.Config["input_field"].(string)
	outputField, _ := step.Config["output_field"].(string)

	logger.Info("Processing transform step",
		zap.String("transform", transform))

	// Ejecutar transformación
	result := p.executeTransformation(transform, inputField, outputField, context)

	return map[string]interface{}{
		"transform":    transform,
		"input_field":  inputField,
		"output_field": outputField,
		"result":       result,
	}, nil
}

// processDatabaseStep procesa un paso de base de datos
func (p *StepProcessor) processDatabaseStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	operation, ok := step.Config["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("missing operation in database step config")
	}

	table, _ := step.Config["table"].(string)

	logger.Info("Processing database step",
		zap.String("operation", operation),
		zap.String("table", table))

	// Simular operación de base de datos
	time.Sleep(50 * time.Millisecond)

	return map[string]interface{}{
		"operation":     operation,
		"table":         table,
		"success":       true,
		"rows_affected": 1,
	}, nil
}

// processNotificationStep procesa un paso de notificación
func (p *StepProcessor) processNotificationStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, logger *zap.Logger) (map[string]interface{}, error) {
	notificationType, ok := step.Config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing type in notification step config")
	}

	recipient, _ := step.Config["recipient"].(string)
	message, _ := step.Config["message"].(string)

	logger.Info("Processing notification step",
		zap.String("type", notificationType))

	// Simular envío de notificación
	time.Sleep(100 * time.Millisecond)

	return map[string]interface{}{
		"type":      notificationType,
		"recipient": recipient,
		"message":   message,
		"sent":      true,
	}, nil
}

// evaluateCondition evalúa una condición simple
func (p *StepProcessor) evaluateCondition(condition, variable, operator string, value interface{}, context map[string]interface{}) bool {
	if condition == "true" {
		return true
	}
	if condition == "false" {
		return false
	}

	// Si hay variable y operador, evaluar
	if variable != "" && operator != "" {
		varValue, exists := context[variable]
		if !exists {
			return false
		}

		switch operator {
		case "eq", "equals":
			return varValue == value
		case "neq", "not_equals":
			return varValue != value
		case "gt", "greater_than":
			return p.compareValues(varValue, value) > 0
		case "lt", "less_than":
			return p.compareValues(varValue, value) < 0
		case "contains":
			return p.containsValue(varValue, value)
		}
	}

	return true
}

// compareValues compara dos valores
func (p *StepProcessor) compareValues(a, b interface{}) int {
	if aFloat, ok := a.(float64); ok {
		if bFloat, ok := b.(float64); ok {
			if aFloat > bFloat {
				return 1
			} else if aFloat < bFloat {
				return -1
			}
			return 0
		}
	}

	if aStr, ok := a.(string); ok {
		if bStr, ok := b.(string); ok {
			if aStr > bStr {
				return 1
			} else if aStr < bStr {
				return -1
			}
			return 0
		}
	}

	return 0
}

// containsValue verifica si un valor contiene otro
func (p *StepProcessor) containsValue(container, value interface{}) bool {
	if containerStr, ok := container.(string); ok {
		if valueStr, ok := value.(string); ok {
			return strings.Contains(containerStr, valueStr)
		}
	}

	if containerArr, ok := container.([]interface{}); ok {
		for _, item := range containerArr {
			if item == value {
				return true
			}
		}
	}

	return false
}

// executeTransformation ejecuta una transformación
func (p *StepProcessor) executeTransformation(transform, inputField, outputField string, context map[string]interface{}) interface{} {
	var input interface{}
	if inputField != "" {
		input = context[inputField]
	}

	switch transform {
	case "uppercase":
		if str, ok := input.(string); ok {
			result := strings.ToUpper(str)
			if outputField != "" {
				context[outputField] = result
			}
			return result
		}
	case "lowercase":
		if str, ok := input.(string); ok {
			result := strings.ToLower(str)
			if outputField != "" {
				context[outputField] = result
			}
			return result
		}
	case "trim":
		if str, ok := input.(string); ok {
			result := strings.TrimSpace(str)
			if outputField != "" {
				context[outputField] = result
			}
			return result
		}
	case "length":
		if str, ok := input.(string); ok {
			result := len(str)
			if outputField != "" {
				context[outputField] = result
			}
			return result
		}
	}

	return input
}
