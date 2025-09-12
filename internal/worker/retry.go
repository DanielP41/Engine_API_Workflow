package worker

import (
	"context"
	"fmt"
	"math"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.uber.org/zap"
)

// RetryManager maneja la lógica de reintentos
type RetryManager struct {
	queueRepo repository.QueueRepository
	logRepo   repository.LogRepository
	logger    *zap.Logger
}

// RetryPolicy política de reintentos
type RetryPolicy struct {
	MaxAttempts     int             `json:"max_attempts"`
	InitialDelay    time.Duration   `json:"initial_delay"`
	MaxDelay        time.Duration   `json:"max_delay"`
	BackoffStrategy BackoffStrategy `json:"backoff_strategy"`
	RetryableErrors []string        `json:"retryable_errors"`
	ExponentialBase float64         `json:"exponential_base"`
}

// BackoffStrategy estrategias de backoff
type BackoffStrategy string

const (
	BackoffFixed       BackoffStrategy = "fixed"
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
	BackoffCustom      BackoffStrategy = "custom"
)

// RetryableError errores que pueden ser reintentados
type RetryableError struct {
	Type        string `json:"type"`
	Pattern     string `json:"pattern"`
	MaxRetries  int    `json:"max_retries"`
	RetryDelay  int    `json:"retry_delay_ms"`
	Description string `json:"description"`
}

// NewRetryManager crea un nuevo manager de reintentos
func NewRetryManager(queueRepo repository.QueueRepository, logRepo repository.LogRepository, logger *zap.Logger) *RetryManager {
	return &RetryManager{
		queueRepo: queueRepo,
		logRepo:   logRepo,
		logger:    logger,
	}
}

// GetDefaultRetryPolicy obtiene la política de reintentos por defecto
func GetDefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:     3,
		InitialDelay:    30 * time.Second,
		MaxDelay:        10 * time.Minute,
		BackoffStrategy: BackoffExponential,
		ExponentialBase: 2.0,
		RetryableErrors: []string{
			"timeout",
			"connection_error",
			"rate_limit",
			"temporary_failure",
			"network_error",
			"service_unavailable",
		},
	}
}

// GetWorkflowRetryPolicy obtiene política específica del workflow
func (r *RetryManager) GetWorkflowRetryPolicy(workflow *models.Workflow) RetryPolicy {
	policy := GetDefaultRetryPolicy()

	// Sobrescribir con configuración del workflow
	if workflow.RetryAttempts > 0 {
		policy.MaxAttempts = workflow.RetryAttempts
	}

	if workflow.RetryDelayMs > 0 {
		policy.InitialDelay = time.Duration(workflow.RetryDelayMs) * time.Millisecond
	}

	return policy
}

// ShouldRetry determina si una tarea debe ser reintentada
func (r *RetryManager) ShouldRetry(task *models.QueueTask, err error, policy RetryPolicy) bool {
	// Verificar número máximo de intentos
	if task.RetryCount >= policy.MaxAttempts {
		r.logger.Info("Max retry attempts reached",
			zap.String("task_id", task.ID),
			zap.Int("retry_count", task.RetryCount),
			zap.Int("max_attempts", policy.MaxAttempts))
		return false
	}

	// Verificar si el error es reintentable
	if !r.isRetryableError(err, policy.RetryableErrors) {
		r.logger.Info("Error is not retryable",
			zap.String("task_id", task.ID),
			zap.String("error", err.Error()))
		return false
	}

	return true
}

// ScheduleRetry programa un reintento para una tarea
func (r *RetryManager) ScheduleRetry(ctx context.Context, task *models.QueueTask, err error, policy RetryPolicy) error {
	if !r.ShouldRetry(task, err, policy) {
		return r.markTaskAsFinallyFailed(ctx, task, err)
	}

	// Calcular delay para el próximo intento
	delay := r.calculateRetryDelay(task.RetryCount, policy)
	nextRunTime := time.Now().Add(delay)

	// Actualizar información del task
	task.RetryCount++
	task.ScheduledAt = &nextRunTime
	task.Status = models.QueueTaskStatusPending
	task.LastError = err.Error()
	task.ProcessedAt = nil

	// Reencolar la tarea
	err = r.queueRepo.EnqueueAt(ctx, task, nextRunTime)
	if err != nil {
		return fmt.Errorf("failed to schedule retry: %w", err)
	}

	r.logger.Info("Task scheduled for retry",
		zap.String("task_id", task.ID),
		zap.Int("retry_count", task.RetryCount),
		zap.Duration("delay", delay),
		zap.Time("next_run", nextRunTime))

	return nil
}

// calculateRetryDelay calcula el delay para el próximo intento
func (r *RetryManager) calculateRetryDelay(retryCount int, policy RetryPolicy) time.Duration {
	var delay time.Duration

	switch policy.BackoffStrategy {
	case BackoffFixed:
		delay = policy.InitialDelay

	case BackoffLinear:
		delay = policy.InitialDelay * time.Duration(retryCount+1)

	case BackoffExponential:
		multiplier := math.Pow(policy.ExponentialBase, float64(retryCount))
		delay = time.Duration(float64(policy.InitialDelay) * multiplier)

	case BackoffCustom:
		// Implementar lógica personalizada
		delay = r.customBackoffCalculation(retryCount, policy)

	default:
		delay = policy.InitialDelay
	}

	// Aplicar jitter para evitar thundering herd
	jitter := time.Duration(float64(delay) * 0.1 * (0.5 - float64(time.Now().UnixNano()%1000)/1000))
	delay += jitter

	// Respetar delay máximo
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	return delay
}

// customBackoffCalculation implementa backoff personalizado
func (r *RetryManager) customBackoffCalculation(retryCount int, policy RetryPolicy) time.Duration {
	// Fibonacci-like backoff: cada intento toma más tiempo basado en los anteriores
	if retryCount == 0 {
		return policy.InitialDelay
	}
	if retryCount == 1 {
		return policy.InitialDelay * 2
	}

	// Aproximación de Fibonacci para delay
	prev2 := policy.InitialDelay
	prev1 := policy.InitialDelay * 2

	for i := 2; i <= retryCount; i++ {
		current := prev1 + prev2
		prev2 = prev1
		prev1 = current
	}

	return prev1
}

// isRetryableError verifica si un error puede ser reintentado
func (r *RetryManager) isRetryableError(err error, retryableErrors []string) bool {
	errorMsg := err.Error()

	for _, retryablePattern := range retryableErrors {
		if r.containsPattern(errorMsg, retryablePattern) {
			return true
		}
	}

	return false
}

// containsPattern verifica si el mensaje de error contiene un patrón específico
func (r *RetryManager) containsPattern(errorMsg, pattern string) bool {
	// Implementación simple de matching
	// Se puede expandir para usar regex si es necesario
	return len(errorMsg) > 0 && len(pattern) > 0 &&
		r.simpleContains(errorMsg, pattern)
}

func (r *RetryManager) simpleContains(str, substr string) bool {
	if len(substr) > len(str) {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// markTaskAsFinallyFailed marca una tarea como fallida definitivamente
func (r *RetryManager) markTaskAsFinallyFailed(ctx context.Context, task *models.QueueTask, err error) error {
	task.Status = models.QueueTaskStatusFailed
	task.FailedAt = &time.Time{}
	*task.FailedAt = time.Now()
	task.LastError = fmt.Sprintf("Max retries exceeded: %s", err.Error())

	updateErr := r.queueRepo.MarkFailed(ctx, task.ID, fmt.Errorf(task.LastError))
	if updateErr != nil {
		r.logger.Error("Failed to mark task as finally failed",
			zap.String("task_id", task.ID),
			zap.Error(updateErr))
		return updateErr
	}

	r.logger.Info("Task marked as finally failed",
		zap.String("task_id", task.ID),
		zap.Int("final_retry_count", task.RetryCount),
		zap.String("final_error", task.LastError))

	return nil
}

// GetRetryStats obtiene estadísticas de reintentos
func (r *RetryManager) GetRetryStats(ctx context.Context) (map[string]interface{}, error) {
	// Obtener tareas pendientes de reintento
	pendingRetries, err := r.queueRepo.GetPendingRetries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending retries: %w", err)
	}

	// Agrupar por número de intentos
	retryDistribution := make(map[int]int)
	for _, task := range pendingRetries {
		retryDistribution[task.RetryCount]++
	}

	stats := map[string]interface{}{
		"total_pending_retries": len(pendingRetries),
		"retry_distribution":    retryDistribution,
		"timestamp":             time.Now(),
	}

	return stats, nil
}

// CleanupOldRetries limpia reintentos muy antiguos
func (r *RetryManager) CleanupOldRetries(ctx context.Context, maxAge time.Duration) error {
	cutoffTime := time.Now().Add(-maxAge)

	oldTasks, err := r.queueRepo.GetTasksOlderThan(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to get old tasks: %w", err)
	}

	cleanedCount := 0
	for _, task := range oldTasks {
		if task.Status == models.QueueTaskStatusPending && task.RetryCount > 0 {
			err := r.markTaskAsFinallyFailed(ctx, task, fmt.Errorf("task expired after %v", maxAge))
			if err != nil {
				r.logger.Error("Failed to cleanup old retry task",
					zap.String("task_id", task.ID),
					zap.Error(err))
				continue
			}
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		r.logger.Info("Cleaned up old retry tasks", zap.Int("count", cleanedCount))
	}

	return nil
}

// ProcessFailedTasks procesa tareas fallidas para posibles reintentos
func (r *RetryManager) ProcessFailedTasks(ctx context.Context) error {
	failedTasks, err := r.queueRepo.GetFailedTasks(ctx, 100) // Límite de 100
	if err != nil {
		return fmt.Errorf("failed to get failed tasks: %w", err)
	}

	for _, task := range failedTasks {
		// Solo procesar tareas que fallaron recientemente y pueden ser reintentadas
		if task.FailedAt != nil && time.Since(*task.FailedAt) < 24*time.Hour {
			// Aquí se podría implementar lógica para reactivar reintentos
			r.logger.Debug("Reviewing failed task for potential retry",
				zap.String("task_id", task.ID),
				zap.Int("retry_count", task.RetryCount))
		}
	}

	return nil
}
