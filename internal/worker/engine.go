package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
)

// WorkerEngine maneja la ejecución de workflows
type WorkerEngine struct {
	queueRepo    repository.QueueRepository
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	userRepo     repository.UserRepository
	logService   services.LogService
	executor     *WorkflowExecutor
	logger       *zap.Logger
	workers      int
	stopCh       chan struct{}
	wg           sync.WaitGroup
	isRunning    bool
	mu           sync.RWMutex
}

// WorkerConfig configuración del worker
type WorkerConfig struct {
	Workers           int           `json:"workers"`
	PollInterval      time.Duration `json:"poll_interval"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	ProcessingTimeout time.Duration `json:"processing_timeout"`
}

// TaskData estructura de datos del task
type TaskData struct {
	LogID     string                 `json:"log_id"`
	Workflow  *models.Workflow       `json:"workflow"`
	TriggerBy string                 `json:"trigger_by"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata"`
	UserID    string                 `json:"user_id"`
}

// NewWorkerEngine crea una nueva instancia del worker engine
func NewWorkerEngine(
	queueRepo repository.QueueRepository,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	logService services.LogService,
	logger *zap.Logger,
	config WorkerConfig,
) *WorkerEngine {
	if config.Workers <= 0 {
		config.Workers = 3
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 30 * time.Second
	}
	if config.ProcessingTimeout <= 0 {
		config.ProcessingTimeout = 30 * time.Minute
	}

	executor := NewWorkflowExecutor(logService, logger)

	return &WorkerEngine{
		queueRepo:    queueRepo,
		workflowRepo: workflowRepo,
		logRepo:      logRepo,
		userRepo:     userRepo,
		logService:   logService,
		executor:     executor,
		logger:       logger,
		workers:      config.Workers,
		stopCh:       make(chan struct{}),
	}
}

// Start inicia los workers
func (e *WorkerEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		return fmt.Errorf("worker engine already running")
	}

	e.logger.Info("Starting worker engine", zap.Int("workers", e.workers))

	// Iniciar workers
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}

	// Iniciar cleanup de tareas obsoletas
	e.wg.Add(1)
	go e.cleanupWorker(ctx)

	e.isRunning = true
	e.logger.Info("Worker engine started successfully")

	return nil
}

// Stop detiene todos los workers
func (e *WorkerEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return fmt.Errorf("worker engine not running")
	}

	e.logger.Info("Stopping worker engine...")

	close(e.stopCh)
	e.wg.Wait()

	e.isRunning = false
	e.logger.Info("Worker engine stopped")

	return nil
}

// worker es el loop principal de procesamiento
func (e *WorkerEngine) worker(ctx context.Context, workerID int) {
	defer e.wg.Done()

	logger := e.logger.With(zap.Int("worker_id", workerID))
	logger.Info("Worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Worker stopping due to context cancellation")
			return
		case <-e.stopCh:
			logger.Info("Worker stopping due to stop signal")
			return
		default:
			if err := e.processNextTask(ctx, logger); err != nil {
				if err != repository.ErrQueueEmpty {
					logger.Error("Error processing task", zap.Error(err))
				}
				// Esperar antes del siguiente intento
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// processNextTask procesa la siguiente tarea de la cola
func (e *WorkerEngine) processNextTask(ctx context.Context, logger *zap.Logger) error {
	// Obtener tarea de la cola
	task, err := e.queueRepo.Dequeue(ctx)
	if err != nil {
		return err
	}

	if task == nil {
		return repository.ErrQueueEmpty
	}

	logger = logger.With(zap.String("task_id", task.ID))
	logger.Info("Processing task", zap.String("workflow_id", task.WorkflowID.Hex()))

	// Parsear datos del task
	var taskData TaskData
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", task.Payload)), &taskData); err != nil {
		logger.Error("Failed to unmarshal task data", zap.Error(err))
		return e.markTaskFailed(ctx, task.ID, fmt.Errorf("invalid task data: %w", err))
	}

	// Ejecutar workflow con timeout
	taskCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	err = e.executeWorkflowTask(taskCtx, task, &taskData, logger)
	if err != nil {
		logger.Error("Workflow execution failed", zap.Error(err))
		return e.markTaskFailed(ctx, task.ID, err)
	}

	// Marcar como completado
	if err := e.queueRepo.MarkCompleted(ctx, task.ID); err != nil {
		logger.Error("Failed to mark task as completed", zap.Error(err))
		return err
	}

	logger.Info("Task completed successfully")
	return nil
}

// executeWorkflowTask ejecuta un workflow específico
func (e *WorkerEngine) executeWorkflowTask(ctx context.Context, task *models.QueueTask, taskData *TaskData, logger *zap.Logger) error {
	// Convertir LogID a ObjectID
	logID, err := primitive.ObjectIDFromHex(taskData.LogID)
	if err != nil {
		return fmt.Errorf("invalid log ID: %w", err)
	}

	// Convertir UserID a ObjectID
	userID, err := primitive.ObjectIDFromHex(taskData.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Obtener el log existente
	workflowLog, err := e.logService.GetLogByID(ctx, logID)
	if err != nil {
		return fmt.Errorf("failed to get workflow log: %w", err)
	}

	// Actualizar estado a "running"
	if err := e.logService.UpdateLogStatus(ctx, logID, models.WorkflowStatus("running"), ""); err != nil {
		return fmt.Errorf("failed to update log status: %w", err)
	}

	logger.Info("Starting workflow execution",
		zap.String("workflow_name", taskData.Workflow.Name),
		zap.String("trigger_by", taskData.TriggerBy))

	// Crear contexto de ejecución
	execContext := &ExecutionContext{
		WorkflowID:  task.WorkflowID,
		LogID:       logID,
		UserID:      userID,
		TriggerData: taskData.Data,
		Metadata:    taskData.Metadata,
		Variables:   make(map[string]interface{}),
		Logger:      logger,
	}

	// Ejecutar workflow
	result, err := e.executor.ExecuteWorkflow(ctx, taskData.Workflow, execContext)
	if err != nil {
		// Actualizar log con error
		if updateErr := e.logService.CompleteWorkflowLog(ctx, logID, false, err.Error()); updateErr != nil {
			logger.Error("Failed to update log with error", zap.Error(updateErr))
		}
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	// Completar log con éxito
	if err := e.logService.CompleteWorkflowLog(ctx, logID, result.Success, result.ErrorMessage); err != nil {
		logger.Error("Failed to complete workflow log", zap.Error(err))
		return fmt.Errorf("failed to complete workflow log: %w", err)
	}

	// Actualizar contexto del log con variables finales
	updateData := map[string]interface{}{
		"context":    execContext.Variables,
		"result":     result,
		"updated_at": time.Now(),
	}

	if err := e.logRepo.Update(ctx, logID, updateData); err != nil {
		logger.Error("Failed to update log context", zap.Error(err))
		// No fallar por esto
	}

	logger.Info("Workflow execution completed successfully",
		zap.Bool("success", result.Success),
		zap.Int("steps_executed", len(result.StepsExecuted)))

	return nil
}

// markTaskFailed marca una tarea como fallida
func (e *WorkerEngine) markTaskFailed(ctx context.Context, taskID string, err error) error {
	if markErr := e.queueRepo.MarkFailed(ctx, taskID, err); markErr != nil {
		e.logger.Error("Failed to mark task as failed",
			zap.String("task_id", taskID),
			zap.Error(markErr))
		return markErr
	}
	return err
}

// cleanupWorker limpia tareas obsoletas periódicamente
func (e *WorkerEngine) cleanupWorker(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			if err := e.cleanupStaleTasks(ctx); err != nil {
				e.logger.Error("Failed to cleanup stale tasks", zap.Error(err))
			}
		}
	}
}

// cleanupStaleTasks limpia tareas que han estado procesándose demasiado tiempo
func (e *WorkerEngine) cleanupStaleTasks(ctx context.Context) error {
	// Obtener tareas en procesamiento
	processingTasks, err := e.queueRepo.GetProcessingTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get processing tasks: %w", err)
	}

	staleTimeout := 1 * time.Hour
	now := time.Now()
	cleanedCount := 0

	for _, task := range processingTasks {
		if task.ProcessedAt != nil && now.Sub(*task.ProcessedAt) > staleTimeout {
			// Marcar como fallida por timeout
			if err := e.queueRepo.MarkFailed(ctx, task.ID, fmt.Errorf("task timeout after %v", staleTimeout)); err != nil {
				e.logger.Error("Failed to mark stale task as failed",
					zap.String("task_id", task.ID),
					zap.Error(err))
				continue
			}
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		e.logger.Info("Cleaned up stale tasks", zap.Int("count", cleanedCount))
	}

	return nil
}

// GetStats devuelve estadísticas del worker engine
func (e *WorkerEngine) GetStats(ctx context.Context) (map[string]interface{}, error) {
	queueStats, err := e.queueRepo.GetQueueStats(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"workers_running": e.workers,
		"is_running":      e.isRunning,
		"queue_stats":     queueStats,
		"uptime":          time.Now(), // Simplificado
	}

	return stats, nil
}
