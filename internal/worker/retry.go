package worker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.uber.org/zap"
)

// RetryManager maneja la lógica de reintentos
type RetryManager struct {
	queueRepo     repository.QueueRepository
	logRepo       repository.LogRepository
	logger        *zap.Logger
	policies      map[string]RetryPolicy
	defaultPolicy RetryPolicy
	mu            sync.RWMutex
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// RetryPolicy política de reintentos
type RetryPolicy struct {
	MaxAttempts     int             `json:"max_attempts"`
	BaseDelay       time.Duration   `json:"base_delay"`
	MaxDelay        time.Duration   `json:"max_delay"`
	BackoffStrategy BackoffStrategy `json:"backoff_strategy"`
	RetryableErrors []string        `json:"retryable_errors"`
	Multiplier      float64         `json:"multiplier"`
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
	defaultPolicy := GetDefaultRetryPolicy()

	return &RetryManager{
		queueRepo:     queueRepo,
		logRepo:       logRepo,
		logger:        logger,
		policies:      make(map[string]RetryPolicy),
		defaultPolicy: defaultPolicy,
		stopCh:        make(chan struct{}),
	}
}

// GetDefaultRetryPolicy obtiene la política de reintentos por defecto
func GetDefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:     3,
		BaseDelay:       30 * time.Second,
		MaxDelay:        10 * time.Minute,
		BackoffStrategy: BackoffExponential,
		Multiplier:      2.0,
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

// StartProcessing inicia el procesamiento de reintentos
func (r *RetryManager) StartProcessing(ctx context.Context) {
	r.logger.Info("Starting retry manager processing")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Retry manager stopping due to context cancellation")
			return
		case <-r.stopCh:
			r.logger.Info("Retry manager stopping")
			return
		case <-ticker.C:
			if err := r.ProcessFailedTasks(ctx); err != nil {
				r.logger.Error("Failed to process failed tasks", zap.Error(err))
			}

			if err := r.CleanupOldRetries(ctx, 7*24*time.Hour); err != nil {
				r.logger.Error("Failed to cleanup old retries", zap.Error(err))
			}
		}
	}
}

// Stop detiene el retry manager
func (r *RetryManager) Stop() {
	r.logger.Info("Stopping retry manager")
	close(r.stopCh)
	r.wg.Wait()
}

// GetWorkflowRetryPolicy obtiene política específica del workflow
func (r *RetryManager) GetWorkflowRetryPolicy(workflow *models.Workflow) RetryPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workflowKey := workflow.ID.Hex()

	if policy, exists := r.policies[workflowKey]; exists {
		return policy
	}

	if len(workflow.Tags) > 0 {
		for _, tag := range workflow.Tags {
			if policy, exists := r.policies[tag]; exists {
				return policy
			}
		}
	}

	policy := r.defaultPolicy

	if workflow.RetryAttempts > 0 {
		policy.MaxAttempts = workflow.RetryAttempts
	}

	if workflow.RetryDelayMs > 0 {
		policy.BaseDelay = time.Duration(workflow.RetryDelayMs) * time.Millisecond
	}

	if workflow.Priority >= 8 {
		policy.MaxAttempts = 5
		policy.BaseDelay = 10 * time.Second
		policy.MaxDelay = 5 * time.Minute
	} else if workflow.Priority >= 5 {
		policy.MaxAttempts = 3
		policy.BaseDelay = 30 * time.Second
		policy.MaxDelay = 10 * time.Minute
	}

	return policy
}

// ShouldRetry determina si una tarea debe ser reintentada
func (r *RetryManager) ShouldRetry(task *models.QueueTask, err error, policy RetryPolicy) bool {
	if task.RetryCount >= policy.MaxAttempts {
		r.logger.Info("Max retry attempts reached",
			zap.String("task_id", task.ID.Hex()),
			zap.Int("retry_count", task.RetryCount),
			zap.Int("max_attempts", policy.MaxAttempts))
		return false
	}

	if !r.isRetryableError(err, policy.RetryableErrors) {
		r.logger.Info("Error is not retryable",
			zap.String("task_id", task.ID.Hex()),
			zap.String("error", err.Error()))
		return false
	}

	return true
}

// ScheduleRetry programa un reintento para una tarea
func (r *RetryManager) ScheduleRetry(ctx context.Context, task *models.QueueTask, err error, policy RetryPolicy) error {
	if !r.ShouldRetry(task, err, policy) {
		return r.handleMaxRetriesExceeded(ctx, task, err)
	}

	retryTime := r.calculateRetryTime(policy, task.RetryCount)
	errorMsg := err.Error()

	r.logger.Info("Scheduling retry",
		zap.String("task_id", task.ID.Hex()),
		zap.Int("retry_count", task.RetryCount+1),
		zap.Time("retry_time", retryTime),
		zap.String("error", errorMsg))

	var executionID string
	if task.ExecutionID != nil {
		executionID = *task.ExecutionID
	}

	// CORREGIDO: Convertir TaskPriority a int
	priorityInt := int(task.Priority)

	return r.queueRepo.EnqueueAt(ctx, task.WorkflowID, executionID, task.UserID, task.Payload, priorityInt, retryTime)
}

// calculateRetryTime calcula cuándo debe ejecutarse el reintento
func (r *RetryManager) calculateRetryTime(policy RetryPolicy, currentRetryCount int) time.Time {
	delay := r.calculateRetryDelay(currentRetryCount, policy)
	return time.Now().Add(delay)
}

// handleMaxRetriesExceeded maneja cuando se exceden los reintentos
func (r *RetryManager) handleMaxRetriesExceeded(ctx context.Context, task *models.QueueTask, err error) error {
	errorMsg := fmt.Sprintf("Max retries exceeded (%d): %v", task.RetryCount, err)

	r.logger.Error("Task failed permanently",
		zap.String("task_id", task.ID.Hex()),
		zap.Int("retry_count", task.RetryCount),
		zap.String("error", errorMsg))

	// CORREGIDO: Convertir task.ID a string
	return r.queueRepo.MarkFailed(ctx, task.ID.Hex(), fmt.Errorf(errorMsg))
}

// calculateRetryDelay calcula el delay para el próximo intento
func (r *RetryManager) calculateRetryDelay(retryCount int, policy RetryPolicy) time.Duration {
	var delay time.Duration

	switch policy.BackoffStrategy {
	case BackoffFixed:
		delay = policy.BaseDelay

	case BackoffLinear:
		delay = policy.BaseDelay * time.Duration(retryCount+1)

	case BackoffExponential:
		multiplier := math.Pow(policy.Multiplier, float64(retryCount))
		delay = time.Duration(float64(policy.BaseDelay) * multiplier)

	case BackoffCustom:
		delay = r.customBackoffCalculation(retryCount, policy)

	default:
		delay = policy.BaseDelay
	}

	jitter := time.Duration(float64(delay) * 0.1 * (0.5 - float64(time.Now().UnixNano()%1000)/1000))
	delay += jitter

	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	return delay
}

// customBackoffCalculation implementa backoff personalizado
func (r *RetryManager) customBackoffCalculation(retryCount int, policy RetryPolicy) time.Duration {
	if retryCount == 0 {
		return policy.BaseDelay
	}
	if retryCount == 1 {
		return policy.BaseDelay * 2
	}

	prev2 := policy.BaseDelay
	prev1 := policy.BaseDelay * 2

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

// markTaskAsFinallyFailed marca una tarea como fallida permanentemente
func (r *RetryManager) markTaskAsFinallyFailed(ctx context.Context, task *models.QueueTask, err error) error {
	errorMsg := fmt.Sprintf("Max retries exceeded: %s", err.Error())

	// CORREGIDO: Convertir task.ID a string
	updateErr := r.queueRepo.MarkFailed(ctx, task.ID.Hex(), fmt.Errorf(errorMsg))
	if updateErr != nil {
		r.logger.Error("Failed to mark task as finally failed",
			zap.String("task_id", task.ID.Hex()),
			zap.Error(updateErr))
		return updateErr
	}

	r.logger.Info("Task marked as finally failed",
		zap.String("task_id", task.ID.Hex()),
		zap.Int("final_retry_count", task.RetryCount),
		zap.String("final_error", errorMsg))

	return nil
}

// SetWorkflowPolicy establece una política específica para un workflow
func (r *RetryManager) SetWorkflowPolicy(workflowID string, policy RetryPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies[workflowID] = policy
}

// RemoveWorkflowPolicy elimina una política específica de workflow
func (r *RetryManager) RemoveWorkflowPolicy(workflowID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.policies, workflowID)
}

// GetRetryStats obtiene estadísticas de reintentos
func (r *RetryManager) GetRetryStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})

	retryingTasks, err := r.queueRepo.GetRetryingTasksCount(ctx)
	if err != nil {
		r.logger.Error("Failed to get retrying tasks count", zap.Error(err))
		retryingTasks = 0
	}

	failedTasks, err := r.queueRepo.GetFailedTasksCount(ctx)
	if err != nil {
		r.logger.Error("Failed to get failed tasks count", zap.Error(err))
		failedTasks = 0
	}

	stats["retrying_tasks"] = retryingTasks
	stats["permanently_failed_tasks"] = failedTasks
	stats["retry_policies_count"] = len(r.policies)
	stats["timestamp"] = time.Now()

	return stats
}

// CleanupOldRetries limpia reintentos muy antiguos
func (r *RetryManager) CleanupOldRetries(ctx context.Context, maxAge time.Duration) error {
	cleanedCount, err := r.queueRepo.CleanupStaleProcessingTasks(ctx, maxAge)
	if err != nil {
		return fmt.Errorf("failed to cleanup old retries: %w", err)
	}

	if cleanedCount > 0 {
		r.logger.Info("Cleaned up old retry tasks", zap.Int64("count", cleanedCount))
	}

	return nil
}

// ProcessFailedTasks procesa tareas fallidas para posibles reintentos
func (r *RetryManager) ProcessFailedTasks(ctx context.Context) error {
	failedTasks, err := r.queueRepo.GetFailedTasks(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to get failed tasks: %w", err)
	}

	for _, task := range failedTasks {
		// CORREGIDO: Usar CompletedAt en lugar de FailedAt
		if task.CompletedAt != nil && time.Since(*task.CompletedAt) < 24*time.Hour {
			r.logger.Debug("Reviewing failed task for potential retry",
				zap.String("task_id", task.ID.Hex()),
				zap.Int("retry_count", task.RetryCount))
		}
	}

	return nil
}

// shouldRetryError determina si un error justifica un reintento
func (r *RetryManager) shouldRetryError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()

	nonRetryableErrors := []string{
		"invalid workflow",
		"user not found",
		"workflow not found",
		"invalid payload",
		"permission denied",
		"unauthorized",
		"forbidden",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if r.simpleContains(errorMsg, nonRetryable) {
			return false
		}
	}

	return true
}
