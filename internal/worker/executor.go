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

	// 🆕 EJECUTORES REALES DE ACCIONES (ACTUALIZADOS)
	httpExecutor      *HTTPActionExecutor
	emailExecutor     *EmailActionExecutor
	slackExecutor     *SlackActionExecutor
	webhookExecutor   *WebhookActionExecutor
	transformExecutor *TransformActionExecutor // 🆕 NUEVO EJECUTOR
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

// 🆕 MÉTODO ACTUALIZADO: Configurar ejecutores reales (incluye Transform)
func (e *WorkflowExecutor) SetActionExecutors(
	httpExec *HTTPActionExecutor,
	emailExec *EmailActionExecutor,
	slackExec *SlackActionExecutor,
	webhookExec *WebhookActionExecutor,
) {
	e.httpExecutor = httpExec
	e.emailExecutor = emailExec
	e.slackExecutor = slackExec
	e.webhookExecutor = webhookExec

	e.logger.Info("Action executors configured",
		zap.Bool("http_enabled", httpExec != nil),
		zap.Bool("email_enabled", emailExec != nil),
		zap.Bool("slack_enabled", slackExec != nil),
		zap.Bool("webhook_enabled", webhookExec != nil))
}

// 🆕 NUEVO MÉTODO: Configurar TransformActionExecutor
func (e *WorkflowExecutor) SetTransformExecutor(transformExec *TransformActionExecutor) {
	e.transformExecutor = transformExec
	e.logger.Info("Transform executor configured", zap.Bool("transform_enabled", transformExec != nil))
}

// Execute compatible con la interfaz del worker engine
func (e *WorkflowExecutor) Execute(ctx context.Context, workflow *models.Workflow, userID primitive.ObjectID, logID primitive.ObjectID, triggerData map[string]interface{}) error {
	execCtx := &ExecutionContext{
		WorkflowID:  workflow.ID,
		LogID:       logID,
		UserID:      userID,
		TriggerData: triggerData,
		Metadata:    make(map[string]interface{}),
		Variables:   make(map[string]interface{}),
		Logger:      e.logger.With(zap.String("workflow_id", workflow.ID.Hex())),
	}

	result, err := e.ExecuteWorkflow(ctx, workflow, execCtx)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("workflow execution failed: %s", result.ErrorMessage)
	}

	return nil
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

// executeStep ejecuta un paso individual del workflow - ACTUALIZADO CON TRANSFORM REAL
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

	// 🆕 EJECUTAR CON EJECUTORES REALES (INCLUYE TRANSFORM)
	var err error
	switch models.ActionType(step.Type) {
	case models.ActionTypeHTTP:
		if e.httpExecutor != nil {
			stepResult, err = e.httpExecutor.Execute(stepCtx, step, execCtx)
		} else {
			err = e.executeHTTPActionSimulated(stepCtx, step, stepResult, execCtx)
		}

	case models.ActionTypeEmail:
		if e.emailExecutor != nil {
			stepResult, err = e.emailExecutor.Execute(stepCtx, step, execCtx)
		} else {
			err = e.executeEmailActionSimulated(stepCtx, step, stepResult, execCtx)
		}

	case models.ActionTypeSlack:
		if e.slackExecutor != nil {
			stepResult, err = e.slackExecutor.Execute(stepCtx, step, execCtx)
		} else {
			err = e.executeSlackActionSimulated(stepCtx, step, stepResult, execCtx)
		}

	case models.ActionTypeWebhook:
		if e.webhookExecutor != nil {
			stepResult, err = e.webhookExecutor.Execute(stepCtx, step, execCtx)
		} else {
			err = e.executeWebhookActionSimulated(stepCtx, step, stepResult, execCtx)
		}

	case models.ActionTypeTransform:
		// 🚀 NUEVO: USAR EXECUTOR REAL DE TRANSFORM
		if e.transformExecutor != nil {
			stepResult, err = e.transformExecutor.Execute(stepCtx, step, execCtx)
		} else {
			err = e.executeTransformActionSimulated(stepCtx, step, stepResult, execCtx)
		}

	case models.ActionTypeCondition:
		err = e.executeConditionAction(stepCtx, step, stepResult, execCtx)
	case models.ActionTypeDelay:
		err = e.executeDelayAction(stepCtx, step, stepResult, execCtx)
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

// 🆕 MÉTODO SIMULADO FALLBACK para Transform (mantener compatibilidad)
func (e *WorkflowExecutor) executeTransformActionSimulated(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Transform action (simulated fallback)", zap.String("step_id", step.ID))

	result.Output["transform_completed"] = true
	result.Output["message"] = "Transform action simulated (fallback) - real executor not configured"
	result.Output["mode"] = "simulated"

	return nil
}

// Los métodos simulados existentes se mantienen para compatibilidad
func (e *WorkflowExecutor) executeHTTPActionSimulated(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing HTTP action (simulated)", zap.String("step_id", step.ID))
	time.Sleep(100 * time.Millisecond)
	result.Output["http_response"] = map[string]interface{}{
		"status":      "success",
		"status_code": 200,
		"message":     "HTTP action simulated successfully",
		"timestamp":   time.Now(),
		"mode":        "simulated",
	}
	return nil
}

func (e *WorkflowExecutor) executeEmailActionSimulated(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Email action (simulated)", zap.String("step_id", step.ID))
	time.Sleep(50 * time.Millisecond)
	result.Output["email_sent"] = true
	result.Output["message"] = "Email action simulated successfully"
	result.Output["timestamp"] = time.Now()
	result.Output["mode"] = "simulated"
	return nil
}

func (e *WorkflowExecutor) executeSlackActionSimulated(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Slack action (simulated)", zap.String("step_id", step.ID))
	time.Sleep(75 * time.Millisecond)
	result.Output["slack_sent"] = true
	result.Output["message"] = "Slack action simulated successfully"
	result.Output["timestamp"] = time.Now()
	result.Output["mode"] = "simulated"
	return nil
}

func (e *WorkflowExecutor) executeWebhookActionSimulated(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Webhook action (simulated)", zap.String("step_id", step.ID))
	time.Sleep(150 * time.Millisecond)
	result.Output["webhook_called"] = true
	result.Output["message"] = "Webhook action simulated successfully"
	result.Output["timestamp"] = time.Now()
	result.Output["mode"] = "simulated"
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

// Los otros métodos permanecen igual por ahora (Database, Integration, Notification)
func (e *WorkflowExecutor) executeDatabaseAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Database action", zap.String("step_id", step.ID))
	time.Sleep(200 * time.Millisecond)
	result.Output["database_operation"] = "completed"
	result.Output["message"] = "Database action simulated successfully"
	return nil
}

func (e *WorkflowExecutor) executeIntegrationAction(ctx context.Context, step *models.WorkflowStep, result *StepResult, execCtx *ExecutionContext) error {
	e.logger.Info("Executing Integration action", zap.String("step_id", step.ID))
	time.Sleep(300 * time.Millisecond)
	result.Output["integration_completed"] = true
	result.Output["message"] = "Integration action simulated successfully"
	return nil
}

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
	if currentStep.NextStepID != nil && *currentStep.NextStepID != "" {
		return *currentStep.NextStepID
	}

	if len(currentStep.Conditions) > 0 {
		return currentStep.Conditions[0].NextStep
	}

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
		}
	}

	return ""
}

// HasRealExecutors verifica si ejecutores reales están configurados
func (e *WorkflowExecutor) HasRealExecutors() bool {
	return e.httpExecutor != nil || e.emailExecutor != nil || e.slackExecutor != nil || e.webhookExecutor != nil || e.transformExecutor != nil
}

// GetExecutorInfo obtiene información de ejecutores configurados
func (e *WorkflowExecutor) GetExecutorInfo() map[string]bool {
	return map[string]bool{
		"http":      e.httpExecutor != nil,
		"email":     e.emailExecutor != nil,
		"slack":     e.slackExecutor != nil,
		"webhook":   e.webhookExecutor != nil,
		"transform": e.transformExecutor != nil, // 🆕 NUEVO
	}
}

// GetExecutorModes obtiene los modos de ejecución por tipo de acción
func (e *WorkflowExecutor) GetExecutorModes() map[string]string {
	modes := make(map[string]string)

	if e.httpExecutor != nil {
		modes["http"] = "real"
	} else {
		modes["http"] = "simulated"
	}

	if e.emailExecutor != nil {
		modes["email"] = "real"
	} else {
		modes["email"] = "simulated"
	}

	if e.slackExecutor != nil {
		modes["slack"] = "real"
	} else {
		modes["slack"] = "simulated"
	}

	if e.webhookExecutor != nil {
		modes["webhook"] = "real"
	} else {
		modes["webhook"] = "simulated"
	}

	// 🆕 NUEVO: Transform mode
	if e.transformExecutor != nil {
		modes["transform"] = "real"
	} else {
		modes["transform"] = "simulated"
	}

	// Siempre simulados (por ahora)
	modes["condition"] = "native"
	modes["delay"] = "native"
	modes["database"] = "simulated"
	modes["integration"] = "simulated"
	modes["notification"] = "simulated"

	return modes
}
