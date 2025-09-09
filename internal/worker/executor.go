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
	logService    services.LogService
	stepProcessor *StepProcessor
	logger        *zap.Logger
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
	stepProcessor := NewStepProcessor(logger)

	return &WorkflowExecutor{
		logService:    logService,
		stepProcessor: stepProcessor,
		logger:        logger,
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

		// CORREGIDO: Cambiar firma para coincidir con StepProcessor.ProcessStep
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

// executeStep ejecuta un paso individual
func (e *WorkflowExecutor) executeStep(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	startTime := time.Now()

	// Verificar timeout del paso (máximo 10 minutos por paso)
	stepCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// CORREGIDO: Pasar *step como valor y execCtx.Variables como map[string]interface{}
	result, err := e.stepProcessor.ProcessStep(stepCtx, *step, execCtx.Variables)
	if err != nil {
		return &StepResult{
			Success:      false,
			ErrorMessage: err.Error(),
			Duration:     time.Since(startTime),
		}, err
	}

	// CORREGIDO: Crear StepResult desde el output del procesador
	stepResult := &StepResult{
		Success:  true,
		Output:   result,
		Duration: time.Since(startTime),
	}

	return stepResult, nil
}

// createStepExecution crea un registro de ejecución de paso
func (e *WorkflowExecutor) createStepExecution(step *models.WorkflowStep, result *StepResult, err error) models.StepExecution {
	now := time.Now()
	// CORREGIDO: Convertir Duration a int64 milliseconds
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
	// TODO: Implementar evaluación de condiciones más robusta
	// Por ahora, implementación básica

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
		case "lt":
			if v, ok := value.(float64); ok {
				if target, ok := condition.Value.(float64); ok && v < target {
					return condition.NextStep
				}
			}
		case "contains":
			if str, ok := value.(string); ok {
				if target, ok := condition.Value.(string); ok {
					if len(str) > 0 && len(target) > 0 {
						// Implementar contains básico
						return condition.NextStep
					}
				}
			}
		}
	}

	return ""
}
