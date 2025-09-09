package worker

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/services"
)

// WorkflowExecutor ejecuta workflows paso a paso
type WorkflowExecutor struct {
	logService services.LogService
	logger     *zap.Logger
}

// ExecutionContext contexto de ejecución del workflow
type ExecutionContext struct {
	WorkflowID  primitive.ObjectID     `json:"workflow_id"`
	LogID       primitive.ObjectID     `json:"log_id"`
	UserID      primitive.ObjectID     `json:"user_id"`
	TriggerData map[string]interface{} `json:"trigger_data"`
	Metadata    map[string]interface{} `json:"metadata"`
	Variables   map[string]interface{} `json:"variables"`
	Logger      *zap.Logger            `json:"-"`
}

// ExecutionResult resultado de la ejecución
type ExecutionResult struct {
	Success        bool                   `json:"success"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	StepsExecuted  []models.StepExecution `json:"steps_executed"`
	FinalVariables map[string]interface{} `json:"final_variables"`
	Duration       time.Duration          `json:"duration"`
	CompletedAt    time.Time              `json:"completed_at"`
}

// StepResult resultado de un paso individual
type StepResult struct {
	Success      bool                   `json:"success"`
	Output       map[string]interface{} `json:"output"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	NextStepID   string                 `json:"next_step_id,omitempty"`
	Duration     time.Duration          `json:"duration"`
}

// NewWorkflowExecutor crea un nuevo ejecutor de workflows
func NewWorkflowExecutor(logService services.LogService, logger *zap.Logger) *WorkflowExecutor {
	return &WorkflowExecutor{
		logService: logService,
		logger:     logger,
	}
}

// ExecuteWorkflow ejecuta un workflow completo
func (e *WorkflowExecutor) ExecuteWorkflow(ctx context.Context, workflow *models.Workflow, execCtx *ExecutionContext) (*ExecutionResult, error) {
	startTime := time.Now()

	execCtx.Logger.Info("Starting workflow execution",
		zap.String("workflow_name", workflow.Name),
		zap.Int("total_steps", len(workflow.Steps)))

	result := &ExecutionResult{
		Success:        true,
		StepsExecuted:  []models.StepExecution{},
		FinalVariables: make(map[string]interface{}),
		CompletedAt:    time.Now(),
	}

	// Inicializar variables con trigger data
	if execCtx.Variables == nil {
		execCtx.Variables = make(map[string]interface{})
	}

	// Agregar trigger data a variables
	execCtx.Variables["trigger_data"] = execCtx.TriggerData
	execCtx.Variables["metadata"] = execCtx.Metadata

	// Validar que el workflow tenga pasos
	if len(workflow.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps to execute")
	}

	// Obtener el primer paso
	currentStepID := workflow.Steps[0].ID
	stepsExecuted := 0
	maxSteps := len(workflow.Steps) * 2 // Prevenir loops infinitos

	// Ejecutar pasos secuencialmente
	for currentStepID != "" && stepsExecuted < maxSteps {
		// Verificar timeout del contexto
		select {
		case <-ctx.Done():
			result.Success = false
			result.ErrorMessage = "Workflow execution timeout"
			break
		default:
		}

		// Encontrar el paso actual
		var currentStep *models.WorkflowStep
		for i := range workflow.Steps {
			if workflow.Steps[i].ID == currentStepID {
				currentStep = &workflow.Steps[i]
				break
			}
		}

		if currentStep == nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Step with ID '%s' not found", currentStepID)
			break
		}

		// Verificar si el paso está habilitado
		if !currentStep.IsEnabled {
			execCtx.Logger.Info("Skipping disabled step", zap.String("step_id", currentStep.ID))

			// Crear registro de paso omitido
			stepExec := models.StepExecution{
				StepID:        currentStep.ID,
				StepName:      currentStep.Name,
				ActionType:    models.ActionType(currentStep.Type),
				Status:        models.WorkflowStatus("skipped"),
				StartedAt:     time.Now(),
				CompletedAt:   &[]time.Time{time.Now()}[0],
				Duration:      &[]int64{0}[0],
				Input:         currentStep.Config,
				Output:        map[string]interface{}{"skipped": true},
				RetryCount:    0,
				ExecutionTime: 0,
			}

			result.StepsExecuted = append(result.StepsExecuted, stepExec)

			// Ir al siguiente paso
			currentStepID = e.getNextStepID(currentStep)
			stepsExecuted++
			continue
		}

		execCtx.Logger.Info("Executing step",
			zap.String("step_id", currentStep.ID),
			zap.String("step_name", currentStep.Name),
			zap.String("step_type", currentStep.Type))

		// Ejecutar el paso
		stepResult, err := e.executeStep(ctx, currentStep, execCtx)
		if err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Step '%s' failed: %v", currentStep.ID, err)

			// Registrar paso fallido
			stepExec := e.createStepExecution(currentStep, stepResult, err)
			result.StepsExecuted = append(result.StepsExecuted, stepExec)

			// Agregar al log en tiempo real
			if logErr := e.logService.AddStepExecution(ctx, execCtx.LogID, stepExec); logErr != nil {
				execCtx.Logger.Error("Failed to add step execution to log", zap.Error(logErr))
			}

			break
		}

		// Crear registro de ejecución del paso
		stepExec := e.createStepExecution(currentStep, stepResult, nil)
		result.StepsExecuted = append(result.StepsExecuted, stepExec)

		// Agregar al log en tiempo real
		if err := e.logService.AddStepExecution(ctx, execCtx.LogID, stepExec); err != nil {
			execCtx.Logger.Error("Failed to add step execution to log", zap.Error(err))
		}

		// Actualizar variables con output del paso
		if stepResult.Output != nil {
			for key, value := range stepResult.Output {
				execCtx.Variables[fmt.Sprintf("step_%s_%s", currentStep.ID, key)] = value
			}
			// También guardar output del último paso
			execCtx.Variables["last_step_output"] = stepResult.Output
		}

		// Si el paso falló, terminar ejecución
		if !stepResult.Success {
			result.Success = false
			result.ErrorMessage = stepResult.ErrorMessage
			break
		}

		// Determinar siguiente paso
		if stepResult.NextStepID != "" {
			currentStepID = stepResult.NextStepID
		} else {
			currentStepID = e.getNextStepID(currentStep)
		}

		stepsExecuted++

		execCtx.Logger.Info("Step completed successfully",
			zap.String("step_id", currentStep.ID),
			zap.String("next_step_id", currentStepID))
	}

	// Verificar si se alcanzó el máximo de pasos (posible loop)
	if stepsExecuted >= maxSteps {
		result.Success = false
		result.ErrorMessage = "Maximum steps exceeded - possible infinite loop detected"
	}

	// Calcular duración total
	result.Duration = time.Since(startTime)
	result.FinalVariables = execCtx.Variables

	execCtx.Logger.Info("Workflow execution completed",
		zap.Bool("success", result.Success),
		zap.Int("steps_executed", len(result.StepsExecuted)),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// executeStep ejecuta un paso individual del workflow
func (e *WorkflowExecutor) executeStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	startTime := time.Now()

	stepResult := &StepResult{
		Success:      true,
		Output:       make(map[string]interface{}),
		ErrorMessage: "",
		Duration:     0,
	}

	// Verificar timeout del paso (máximo 10 minutos por paso)
	stepCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Ejecutar según el tipo de paso
	var err error
	switch models.ActionType(step.Type) {
	case models.ActionTypeHTTP:
		err = e.executeHTTPAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeEmail:
		err = e.executeEmailAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeSlack:
		err = e.executeSlackAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeWebhook:
		err = e.executeWebhookAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeCondition:
		err = e.executeConditionAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeDelay:
		err = e.executeDelayAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeTransform:
		err = e.executeTransformAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeDatabase:
		err = e.executeDatabaseAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeIntegration:
		err = e.executeIntegrationAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeNotification:
		err = e.executeNotificationAction(stepCtx, step, stepResult, execCtx)
	default:
		err = fmt.Errorf("unsupported action type: %s", step.Type)
	}

	stepResult.Duration = time.Since(startTime)

	if err != nil {
		stepResult.Success = false
		stepResult.ErrorMessage = err.Error()
		return stepResult, err
	}

	return stepResult, nil
}

// executeHTTPAction ejecuta una acción HTTP
func (e *WorkflowExecutor) executeHTTPAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	// TODO: Implementar llamada HTTP real
	// Por ahora, simulamos una ejecución exitosa

	e.logger.Info("Executing HTTP action", zap.String("step_id", step.ID))

	// Simular tiempo de procesamiento
	time.Sleep(100 * time.Millisecond)

	result.Output["http_response"] = map[string]interface{}{
		"status":      "success",
		"status_code": 200,
		"message":     "HTTP action simulated successfully",
		"timestamp":   time.Now(),
	}

	return nil
}

// executeEmailAction ejecuta una acción de email
func (e *WorkflowExecutor) executeEmailAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Email action", zap.String("step_id", step.ID))

	time.Sleep(50 * time.Millisecond)

	result.Output["email_sent"] = true
	result.Output["message"] = "Email action simulated successfully"
	result.Output["timestamp"] = time.Now()

	return nil
}

// executeSlackAction ejecuta una acción de Slack
func (e *WorkflowExecutor) executeSlackAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Slack action", zap.String("step_id", step.ID))

	time.Sleep(75 * time.Millisecond)

	result.Output["slack_sent"] = true
	result.Output["message"] = "Slack action simulated successfully"
	result.Output["timestamp"] = time.Now()

	return nil
}

// executeWebhookAction ejecuta una acción de webhook
func (e *WorkflowExecutor) executeWebhookAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Webhook action", zap.String("step_id", step.ID))

	time.Sleep(150 * time.Millisecond)

	result.Output["webhook_called"] = true
	result.Output["message"] = "Webhook action simulated successfully"
	result.Output["timestamp"] = time.Now()

	return nil
}

// executeConditionAction ejecuta una acción condicional
func (e *WorkflowExecutor) executeConditionAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Condition action", zap.String("step_id", step.ID))

	// Evaluar condiciones y determinar siguiente paso
	if len(step.Conditions) > 0 {
		nextStep := e.evaluateConditions(step.Conditions, execCtx.Variables)
		if nextStep != "" {
			result.NextStepID = nextStep
		}
	}

	result.Output["condition_evaluated"] = true
	result.Output["next_step"] = result.NextStepID
	result.Output["message"] = "Condition action evaluated successfully"

	return nil
}

// executeDelayAction ejecuta una acción de delay
func (e *WorkflowExecutor) executeDelayAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	// Obtener duración del delay de la configuración
	delayMs, ok := step.Config["delay_ms"]
	if !ok {
		delayMs = float64(1000) // Default 1 segundo
	}

	delay := time.Duration(delayMs.(float64)) * time.Millisecond

	e.logger.Info("Executing delay", zap.Duration("delay", delay))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		result.Output["delay_completed"] = true
		result.Output["delay_duration_ms"] = delayMs
		result.Output["message"] = "Delay action completed successfully"
		return nil
	}
}

// executeTransformAction ejecuta una acción de transformación de datos
func (e *WorkflowExecutor) executeTransformAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Transform action", zap.String("step_id", step.ID))

	// TODO: Implementar transformación de datos real
	result.Output["transform_completed"] = true
	result.Output["message"] = "Transform action simulated successfully"
	result.Output["input_variables"] = len(execCtx.Variables)

	return nil
}

// executeDatabaseAction ejecuta una acción de base de datos
func (e *WorkflowExecutor) executeDatabaseAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Database action", zap.String("step_id", step.ID))

	time.Sleep(200 * time.Millisecond)

	result.Output["database_operation"] = "completed"
	result.Output["message"] = "Database action simulated successfully"

	return nil
}

// executeIntegrationAction ejecuta una acción de integración
func (e *WorkflowExecutor) executeIntegrationAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Integration action", zap.String("step_id", step.ID))

	time.Sleep(300 * time.Millisecond)

	result.Output["integration_completed"] = true
	result.Output["message"] = "Integration action simulated successfully"

	return nil
}

// executeNotificationAction ejecuta una acción de notificación
func (e *WorkflowExecutor) executeNotificationAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Notification action", zap.String("step_id", step.ID))

	time.Sleep(100 * time.Millisecond)

	result.Output["notification_sent"] = true
	result.Output["message"] = "Notification action simulated successfully"

	return nil
}

// createStepExecution crea un registro de ejecución de paso
func (e *WorkflowExecutor) createStepExecution(step *models.WorkflowStep, result *StepResult, err error) models.StepExecution {
	now := time.Now()
	duration := int64(result.Duration.Milliseconds())

	status := models.WorkflowStatus("completed")
	errorMessage := ""

	if err != nil || !result.Success {
		status = models.WorkflowStatus("failed")
		if err != nil {
			errorMessage = err.Error()
		} else {
			errorMessage = result.ErrorMessage
		}
	}

	stepExec := models.StepExecution{
		StepID:        step.ID,
		StepName:      step.Name,
		ActionType:    models.ActionType(step.Type),
		Status:        status,
		StartedAt:     now.Add(-result.Duration),
		CompletedAt:   &now,
		Duration:      &duration,
		Input:         step.Config,
		ErrorMessage:  errorMessage,
		RetryCount:    0, // TODO: implementar reintentos por paso
		ExecutionTime: duration,
	}

	if result.Output != nil {
		stepExec.Output = result.Output
	} else {
		stepExec.Output = make(map[string]interface{})
	}

	return stepExec
}

// getNextStepID determina el siguiente paso basado en la configuración
func (e *WorkflowExecutor) getNextStepID(currentStep *models.WorkflowStep) string {
	// Si el paso tiene un next_step_id definido, usarlo
	if currentStep.NextStepID != nil && *currentStep.NextStepID != "" {
		return *currentStep.NextStepID
	}

	// Si hay condiciones, evaluarlas (simplificado por ahora)
	if len(currentStep.Conditions) > 0 {
		// TODO: implementar evaluación de condiciones
		// Por ahora, tomar el primer paso de la primera condición
		return currentStep.Conditions[0].NextStep
	}

	// Si no hay configuración específica, terminar
	return ""
}

// evaluateConditions evalúa las condiciones de un paso
func (e *WorkflowExecutor) evaluateConditions(conditions []models.WorkflowCondition, variables map[string]interface{}) string {
	for _, condition := range conditions {
		value, exists := variables[condition.Field]
		if !exists {
			continue
		}

		switch condition.Operator {
		case "eq":
			if value == condition.Value {
				return condition.NextStep
			}
		case "neq":
			if value != condition.Value {
				return condition.NextStep
			}
		case "gt":
			if v, ok := value.(float64); ok {
				if target, ok := condition.Value.(float64); ok && v > target {
					return condition.NextStep
				}
			}
		case "gte":
			if v, ok := value.(float64); ok {
				if target, ok := condition.Value.(float64); ok && v >= target {
					return condition.NextStep
				}
			}
		case "lt":
			if v, ok := value.(float64); ok {
				if target, ok := condition.Value.(float64); ok && v < target {
					return condition.NextStep
				}
			}
		case "lte":
			if v, ok := value.(float64); ok {
				if target, ok := condition.Value.(float64); ok && v <= target {
					return condition.NextStep
				}
			}
		case "contains":
			if str, ok := value.(string); ok {
				if target, ok := condition.Value.(string); ok {
					if len(str) > 0 && len(target) > 0 {
						// Implementación básica de contains
						for i := 0; i <= len(str)-len(target); i++ {
							if str[i:i+len(target)] == target {
								return condition.NextStep
							}
						}
					}
				}
			}
		case "not_contains":
			if str, ok := value.(string); ok {
				if target, ok := condition.Value.(string); ok {
					found := false
					for i := 0; i <= len(str)-len(target); i++ {
						if str[i:i+len(target)] == target {
							found = true
							break
						}
					}
					if !found {
						return condition.NextStep
					}
				}
			}
		}
	}

	return ""
}
