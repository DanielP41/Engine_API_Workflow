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
	policies      map[string]RetryPolicy // 🆕 AGREGADO: Políticas por workflow
	defaultPolicy RetryPolicy            // 🆕 AGREGADO: Política por defecto
	mu            sync.RWMutex           // 🆕 AGREGADO: Mutex para concurrencia
}

// RetryPolicy política de reintentos
type RetryPolicy struct {
	MaxAttempts     int             `json:"max_attempts"`
	BaseDelay       time.Duration   `json:"base_delay"` // 🔧 RENOMBRADO: InitialDelay -> BaseDelay
	MaxDelay        time.Duration   `json:"max_delay"`
	BackoffStrategy BackoffStrategy `json:"backoff_strategy"`
	RetryableErrors []string        `json:"retryable_errors"`
	Multiplier      float64         `json:"multiplier"` // 🔧 RENOMBRADO: ExponentialBase -> Multiplier
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
		policies:      make(map[string]RetryPolicy), // 🆕 INICIALIZADO
		defaultPolicy: defaultPolicy,                // 🆕 INICIALIZADO
	}
}

// GetDefaultRetryPolicy obtiene la política de reintentos por defecto
func GetDefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:     3,
		BaseDelay:       30 * time.Second, // 🔧 CORREGIDO: InitialDelay -> BaseDelay
		MaxDelay:        10 * time.Minute,
		BackoffStrategy: BackoffExponential,
		Multiplier:      2.0, // 🔧 CORREGIDO: ExponentialBase -> Multiplier
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

// GetWorkflowRetryPolicy obtiene política específica del workflow - CORREGIDO PARA SATISFACER INTERFAZ
func (r *RetryManager) GetWorkflowRetryPolicy(workflow *models.Workflow) RetryPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workflowKey := workflow.ID.Hex()

	// Buscar política específica para el workflow
	if policy, exists := r.policies[workflowKey]; exists {
		return policy
	}

	// Buscar política por tipo/categoría usando tags
	if len(workflow.Tags) > 0 {
		for _, tag := range workflow.Tags {
			if policy, exists := r.policies[tag]; exists {
				return policy
			}
		}
	}

	// Política por defecto basada en configuración del workflow
	policy := r.defaultPolicy

	// Sobrescribir con configuración del workflow si está disponible
	if workflow.RetryAttempts > 0 {
		policy.MaxAttempts = workflow.RetryAttempts
	}

	if workflow.RetryDelayMs > 0 {
		policy.BaseDelay = time.Duration(workflow.RetryDelayMs) * time.Millisecond
	}

	// Política por defecto basada en prioridad del workflow
	if workflow.Priority >= 8 { // Alta prioridad
		policy.MaxAttempts = 5
		policy.BaseDelay = 10 * time.Second
		policy.MaxDelay = 5 * time.Minute
	} else if workflow.Priority >= 5 { // Prioridad media
		policy.MaxAttempts = 3
		policy.BaseDelay = 30 * time.Second
		policy.MaxDelay = 10 * time.Minute
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

// ScheduleRetry programa un reintento para una tarea - CORREGIDO COMPLETAMENTE
func (r *RetryManager) ScheduleRetry(ctx context.Context, task *models.QueueTask, err error, policy RetryPolicy) error {
	if !r.ShouldRetry(task, err, policy) {
		return r.handleMaxRetriesExceeded(ctx, task, err)
	}

	// Calcular el tiempo de reintento
	retryTime := r.calculateRetryTime(policy, task.RetryCount)
	errorMsg := err.Error()

	// Log del reintento
	r.logger.Info("Scheduling retry",
		zap.String("task_id", task.ID),
		zap.Int("retry_count", task.RetryCount+1),
		zap.Time("retry_time", retryTime),
		zap.String("error", errorMsg))

	// 🔧 CORREGIDO: Encolar nuevamente usando EnqueueAt con parámetros correctos
	return r.queueRepo.EnqueueAt(ctx, task.WorkflowID, task.ExecutionID, task.UserID, task.Payload, task.Priority, retryTime)
}

// 🆕 MÉTODO AGREGADO: calculateRetryTime calcula cuándo debe ejecutarse el reintento
func (r *RetryManager) calculateRetryTime(policy RetryPolicy, currentRetryCount int) time.Time {
	delay := r.calculateRetryDelay(currentRetryCount, policy)
	return time.Now().Add(delay)
}

// 🆕 MÉTODO AGREGADO: handleMaxRetriesExceeded maneja cuando se exceden los reintentos
func (r *RetryManager) handleMaxRetriesExceeded(ctx context.Context, task *models.QueueTask, err error) error {
	errorMsg := fmt.Sprintf("Max retries exceeded (%d): %v", task.RetryCount, err)

	r.logger.Error("Task failed permanently",
		zap.String("task_id", task.ID),
		zap.Int("retry_count", task.RetryCount),
		zap.String("error", errorMsg))

	// 🔧 CORREGIDO: Usar el método MarkFailed del repositorio
	return r.queueRepo.MarkFailed(ctx, task.ID, fmt.Errorf(errorMsg))
}

// calculateRetryDelay calcula el delay para el próximo intento
func (r *RetryManager) calculateRetryDelay(retryCount int, policy RetryPolicy) time.Duration {
	var delay time.Duration

	switch policy.BackoffStrategy {
	case BackoffFixed:
		delay = policy.BaseDelay // 🔧 CORREGIDO: InitialDelay -> BaseDelay

	case BackoffLinear:
		delay = policy.BaseDelay * time.Duration(retryCount+1) // 🔧 CORREGIDO

	case BackoffExponential:
		multiplier := math.Pow(policy.Multiplier, float64(retryCount)) // 🔧 CORREGIDO
		delay = time.Duration(float64(policy.BaseDelay) * multiplier)  // 🔧 CORREGIDO

	case BackoffCustom:
		delay = r.customBackoffCalculation(retryCount, policy)

	default:
		delay = policy.BaseDelay // 🔧 CORREGIDO
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
		return policy.BaseDelay // 🔧 CORREGIDO
	}
	if retryCount == 1 {
		return policy.BaseDelay * 2 // 🔧 CORREGIDO
	}

	// Aproximación de Fibonacci para delay
	prev2 := policy.BaseDelay     // 🔧 CORREGIDO
	prev1 := policy.BaseDelay * 2 // 🔧 CORREGIDO

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

// 🔧 MÉTODO SIMPLIFICADO Y CORREGIDO
func (r *RetryManager) markTaskAsFinallyFailed(ctx context.Context, task *models.QueueTask, err error) error {
	errorMsg := fmt.Sprintf("Max retries exceeded: %s", err.Error())

	updateErr := r.queueRepo.MarkFailed(ctx, task.ID, fmt.Errorf(errorMsg))
	if updateErr != nil {
		r.logger.Error("Failed to mark task as finally failed",
			zap.String("task_id", task.ID),
			zap.Error(updateErr))
		return updateErr
	}

	r.logger.Info("Task marked as finally failed",
		zap.String("task_id", task.ID),
		zap.Int("final_retry_count", task.RetryCount),
		zap.String("final_error", errorMsg))

	return nil
}

// 🆕 MÉTODOS ADICIONALES PARA COMPLETAR LA FUNCIONALIDAD

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

// GetRetryStats obtiene estadísticas de reintentos - CORREGIDO PARA USAR MÉTODOS EXISTENTES
func (r *RetryManager) GetRetryStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})

	// Obtener tareas que están siendo reintentadas
	retryingTasks, err := r.queueRepo.GetRetryingTasksCount(ctx)
	if err != nil {
		r.logger.Error("Failed to get retrying tasks count", zap.Error(err))
		retryingTasks = 0
	}

	// Obtener tareas fallidas permanentemente
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

// CleanupOldRetries limpia reintentos muy antiguos - SIMPLIFICADO PARA USAR MÉTODOS EXISTENTES
func (r *RetryManager) CleanupOldRetries(ctx context.Context, maxAge time.Duration) error {
	// Usar el método de limpieza existente del repositorio
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
	failedTasks, err := r.queueRepo.GetFailedTasks(ctx, 100) // Límite de 100
	if err != nil {
		return fmt.Errorf("failed to get failed tasks: %w", err)
	}

	for _, task := range failedTasks {
		// Solo procesar tareas que fallaron recientemente y pueden ser reintentadas
		if task.FailedAt != nil && time.Since(*task.FailedAt) < 24*time.Hour {
			r.logger.Debug("Reviewing failed task for potential retry",
				zap.String("task_id", task.ID),
				zap.Int("retry_count", task.RetryCount))
		}
	}

	return nil
}

// 🆕 MÉTODOS ADICIONALES ÚTILES

// shouldRetryError determina si un error justifica un reintento
func (r *RetryManager) shouldRetryError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()

	// Errores que NO deben reintentarse
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

	// Por defecto, reintentar errores temporales
	return true
}
