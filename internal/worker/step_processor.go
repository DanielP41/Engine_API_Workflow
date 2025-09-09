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
func (p *StepProcessor) ProcessStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}) (*models.StepExecution, error) {
	startTime := time.Now()

	execution := &models.StepExecution{
		StepID:     step.ID,
		StepName:   step.Name,
		ActionType: models.ActionType(step.Type),
		Status:     models.WorkflowStatus("running"),
		StartedAt:  startTime,
		Input:      step.Config,
		Output:     make(map[string]interface{}),
		RetryCount: 0,
	}

	logger := p.logger.With(
		zap.String("step_id", step.ID),
		zap.String("step_name", step.Name),
		zap.String("step_type", step.Type),
	)

	logger.Info("Processing step")

	// Verificar si el paso está habilitado
	if !step.IsEnabled {
		execution.Status = models.WorkflowStatus("skipped")
		execution.Output["reason"] = "step disabled"
		p.completeExecution(execution, startTime)
		return execution, nil
	}

	// Procesar según el tipo de paso
	var err error
	switch step.Type {
	case "http":
		err = p.processHTTPStep(ctx, step, context, execution, logger)
	case "email":
		err = p.processEmailStep(ctx, step, context, execution, logger)
	case "slack":
		err = p.processSlackStep(ctx, step, context, execution, logger)
	case "webhook":
		err = p.processWebhookStep(ctx, step, context, execution, logger)
	case "delay":
		err = p.processDelayStep(ctx, step, context, execution, logger)
	case "condition":
		err = p.processConditionStep(ctx, step, context, execution, logger)
	case "transform":
		err = p.processTransformStep(ctx, step, context, execution, logger)
	case "database":
		err = p.processDatabaseStep(ctx, step, context, execution, logger)
	case "notification":
		err = p.processNotificationStep(ctx, step, context, execution, logger)
	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
	}

	// Completar la ejecución
	p.completeExecution(execution, startTime)

	if err != nil {
		execution.Status = models.WorkflowStatus("failed")
		execution.ErrorMessage = err.Error()
		logger.Error("Step processing failed", zap.Error(err))
	} else {
		execution.Status = models.WorkflowStatus("completed")
		logger.Info("Step processing completed successfully")
	}

	return execution, err
}

// completeExecution completa los datos de ejecución del paso
func (p *StepProcessor) completeExecution(execution *models.StepExecution, startTime time.Time) {
	completedAt := time.Now()
	duration := completedAt.Sub(startTime)

	execution.CompletedAt = &completedAt
	durationMs := duration.Milliseconds()
	execution.Duration = &durationMs
	execution.ExecutionTime = durationMs
}

// processHTTPStep procesa un paso HTTP
func (p *StepProcessor) processHTTPStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	url, ok := step.Config["url"].(string)
	if !ok {
		return fmt.Errorf("missing url in HTTP step config")
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

	execution.Output = map[string]interface{}{
		"status_code": 200,
		"response":    "OK",
		"url":         url,
		"method":      method,
		"headers":     headers,
	}

	return nil
}

// processEmailStep procesa un paso de email
func (p *StepProcessor) processEmailStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	to, ok := step.Config["to"].(string)
	if !ok {
		return fmt.Errorf("missing 'to' address in email step config")
	}

	subject, _ := step.Config["subject"].(string)
	body, _ := step.Config["body"].(string)
	from, _ := step.Config["from"].(string)

	logger.Info("Processing email step",
		zap.String("to", to),
		zap.String("subject", subject))

	// Simular envío de email
	time.Sleep(200 * time.Millisecond)

	execution.Output = map[string]interface{}{
		"sent":    true,
		"to":      to,
		"subject": subject,
		"from":    from,
	}

	return nil
}

// processSlackStep procesa un paso de Slack
func (p *StepProcessor) processSlackStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	channel, ok := step.Config["channel"].(string)
	if !ok {
		return fmt.Errorf("missing channel in Slack step config")
	}

	message, _ := step.Config["message"].(string)
	username, _ := step.Config["username"].(string)

	logger.Info("Processing Slack step",
		zap.String("channel", channel))

	// Simular llamada a Slack API
	time.Sleep(150 * time.Millisecond)

	execution.Output = map[string]interface{}{
		"sent":     true,
		"channel":  channel,
		"message":  message,
		"username": username,
	}

	return nil
}

// processWebhookStep procesa un paso de webhook
func (p *StepProcessor) processWebhookStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	url, ok := step.Config["url"].(string)
	if !ok {
		return fmt.Errorf("missing URL in webhook step config")
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

	execution.Output = map[string]interface{}{
		"called": true,
		"url":    url,
		"method": method,
		"status": "success",
	}

	return nil
}

// processDelayStep procesa un paso de delay
func (p *StepProcessor) processDelayStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	delaySeconds, ok := step.Config["delay_seconds"].(float64)
	if !ok {
		delaySeconds = 1.0
	}

	duration := time.Duration(delaySeconds) * time.Second
	logger.Info("Processing delay step", zap.Duration("duration", duration))

	// Esperar con posibilidad de cancelación
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		// Continuar
	}

	execution.Output = map[string]interface{}{
		"delayed_seconds": delaySeconds,
		"completed_at":    time.Now(),
	}

	return nil
}

// processConditionStep procesa un paso condicional
func (p *StepProcessor) processConditionStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	condition, ok := step.Config["condition"].(string)
	if !ok {
		return fmt.Errorf("missing condition in condition step config")
	}

	variable, _ := step.Config["variable"].(string)
	operator, _ := step.Config["operator"].(string)
	value, _ := step.Config["value"]

	logger.Info("Processing condition step",
		zap.String("condition", condition))

	// Evaluar condición
	result := p.evaluateCondition(condition, variable, operator, value, context)

	execution.Output = map[string]interface{}{
		"condition": condition,
		"variable":  variable,
		"operator":  operator,
		"value":     value,
		"result":    result,
	}

	return nil
}

// processTransformStep procesa un paso de transformación
func (p *StepProcessor) processTransformStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	transform, ok := step.Config["transform"].(string)
	if !ok {
		return fmt.Errorf("missing transform in transform step config")
	}

	inputField, _ := step.Config["input_field"].(string)
	outputField, _ := step.Config["output_field"].(string)

	logger.Info("Processing transform step",
		zap.String("transform", transform))

	// Ejecutar transformación
	result := p.executeTransformation(transform, inputField, outputField, context)

	execution.Output = map[string]interface{}{
		"transform":    transform,
		"input_field":  inputField,
		"output_field": outputField,
		"result":       result,
	}

	return nil
}

// processDatabaseStep procesa un paso de base de datos
func (p *StepProcessor) processDatabaseStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	operation, ok := step.Config["operation"].(string)
	if !ok {
		return fmt.Errorf("missing operation in database step config")
	}

	table, _ := step.Config["table"].(string)

	logger.Info("Processing database step",
		zap.String("operation", operation),
		zap.String("table", table))

	// Simular operación de base de datos
	time.Sleep(50 * time.Millisecond)

	execution.Output = map[string]interface{}{
		"operation":     operation,
		"table":         table,
		"success":       true,
		"rows_affected": 1,
	}

	return nil
}

// processNotificationStep procesa un paso de notificación
func (p *StepProcessor) processNotificationStep(ctx context.Context, step models.WorkflowStep, context map[string]interface{}, execution *models.StepExecution, logger *zap.Logger) error {
	notificationType, ok := step.Config["type"].(string)
	if !ok {
		return fmt.Errorf("missing type in notification step config")
	}

	recipient, _ := step.Config["recipient"].(string)
	message, _ := step.Config["message"].(string)

	logger.Info("Processing notification step",
		zap.String("type", notificationType))

	// Simular envío de notificación
	time.Sleep(100 * time.Millisecond)

	execution.Output = map[string]interface{}{
		"type":      notificationType,
		"recipient": recipient,
		"message":   message,
		"sent":      true,
	}

	return nil
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
