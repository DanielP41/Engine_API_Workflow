// internal/worker/engine.go - MODIFICACIONES NECESARIAS

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// WorkerEngine - ESTRUCTURA ACTUALIZADA
type WorkerEngine struct {
	// Repositorios originales
	queueRepo    repository.QueueRepository
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	userRepo     repository.UserRepository
	logService   services.LogService

	// Componentes originales
	executor *WorkflowExecutor
	logger   *zap.Logger

	// 🆕 NUEVOS COMPONENTES
	pool         *WorkerPool       // Pool dinámico de workers
	retryManager *RetryManager     // Sistema de reintentos
	metrics      *MetricsCollector // Sistema de métricas

	// 🆕 NUEVOS EJECUTORES DE ACCIONES
	httpExecutor    *HTTPActionExecutor
	emailExecutor   *EmailActionExecutor
	slackExecutor   *SlackActionExecutor
	webhookExecutor *WebhookActionExecutor

	// Control de estado
	stopCh    chan struct{}
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.RWMutex
	startTime time.Time // Para métricas de uptime
}

// NewWorkerEngine - CONSTRUCTOR ACTUALIZADO
func NewWorkerEngine(
	queueRepo repository.QueueRepository,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	logService services.LogService,
	logger *zap.Logger,
	config WorkerConfig,
) *WorkerEngine {
	// Configuración por defecto
	if config.Workers <= 0 {
		config.Workers = 3
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 20 // Nuevo: máximo de workers
	}

	// Crear ejecutor original
	executor := NewWorkflowExecutor(logService, logger)

	// 🆕 CREAR NUEVOS COMPONENTES
	retryManager := NewRetryManager(queueRepo, logRepo, logger)
	metrics := NewMetricsCollector(logger, queueRepo)

	// 🆕 CREAR EJECUTORES DE ACCIONES REALES
	httpExecutor := NewHTTPActionExecutor(logger)
	emailExecutor := NewEmailActionExecutor(logger)
	slackExecutor := NewSlackActionExecutor(logger)
	webhookExecutor := NewWebhookActionExecutor(logger)

	engine := &WorkerEngine{
		// Componentes originales
		queueRepo:    queueRepo,
		workflowRepo: workflowRepo,
		logRepo:      logRepo,
		userRepo:     userRepo,
		logService:   logService,
		executor:     executor,
		logger:       logger,
		stopCh:       make(chan struct{}),

		// 🆕 NUEVOS COMPONENTES
		retryManager:    retryManager,
		metrics:         metrics,
		httpExecutor:    httpExecutor,
		emailExecutor:   emailExecutor,
		slackExecutor:   slackExecutor,
		webhookExecutor: webhookExecutor,
		startTime:       time.Now(),
	}

	// 🆕 CREAR WORKER POOL (reemplaza workers simples)
	engine.pool = NewWorkerPool(engine, config.Workers, config.MaxWorkers, logger)

	return engine
}

// Start - MÉTODO ACTUALIZADO
func (e *WorkerEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		return fmt.Errorf("worker engine already running")
	}

	e.logger.Info("Starting advanced worker engine with pool and metrics")

	// 🆕 INICIAR WORKER POOL (en lugar de workers simples)
	if err := e.pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// 🆕 INICIAR LIMPIEZA PERIÓDICA DE MÉTRICAS
	e.wg.Add(1)
	go e.metrics.StartPeriodicCleanup(ctx)

	// 🆕 INICIAR LIMPIEZA DE REINTENTOS ANTIGUOS
	e.wg.Add(1)
	go e.periodicRetryCleanup(ctx)

	e.isRunning = true
	e.logger.Info("Advanced worker engine started successfully")

	return nil
}

// Stop - MÉTODO ACTUALIZADO
func (e *WorkerEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return fmt.Errorf("worker engine not running")
	}

	e.logger.Info("Stopping advanced worker engine...")

	// 🆕 DETENER WORKER POOL
	if err := e.pool.Stop(); err != nil {
		e.logger.Error("Error stopping worker pool", zap.Error(err))
	}

	// Detener procesos internos
	close(e.stopCh)
	e.wg.Wait()

	e.isRunning = false
	e.logger.Info("Advanced worker engine stopped")

	return nil
}

// 🆕 NUEVO MÉTODO: Procesar tarea con reintentos y métricas
func (e *WorkerEngine) ProcessTaskWithRetries(ctx context.Context, task *models.QueueTask, logger *zap.Logger) error {
	startTime := time.Now()

	// Obtener el workflow
	var taskData TaskData
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", task.Payload)), &taskData); err != nil {
		return fmt.Errorf("invalid task data: %w", err)
	}

	// Ejecutar workflow
	err := e.executeWorkflowTaskAdvanced(ctx, task, &taskData, logger)
	duration := time.Since(startTime)

	// 🆕 REGISTRAR MÉTRICAS
	actionType := "workflow" // Se puede extraer del workflow
	if taskData.Workflow != nil && len(taskData.Workflow.Steps) > 0 {
		actionType = taskData.Workflow.Steps[0].Type
	}

	success := err == nil
	e.metrics.RecordTaskProcessed(actionType, duration, success)

	if err != nil {
		// 🆕 USAR SISTEMA DE REINTENTOS
		workflow := taskData.Workflow
		if workflow != nil {
			policy := e.retryManager.GetWorkflowRetryPolicy(workflow)
			return e.retryManager.ScheduleRetry(ctx, task, err, policy)
		}
		return e.markTaskFailed(ctx, task.ID, err)
	}

	// Marcar como completado
	return e.queueRepo.MarkCompleted(ctx, task.ID)
}

// 🆕 NUEVO MÉTODO: Ejecutar workflow con nuevos ejecutores
func (e *WorkerEngine) executeWorkflowTaskAdvanced(ctx context.Context, task *models.QueueTask, taskData *TaskData, logger *zap.Logger) error {
	// Usar el executor original pero con nuevos ejecutores de acciones
	logID, err := primitive.ObjectIDFromHex(taskData.LogID)
	if err != nil {
		return fmt.Errorf("invalid log ID: %w", err)
	}

	userID, err := primitive.ObjectIDFromHex(taskData.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// 🆕 INTEGRAR EJECUTORES REALES
	e.executor.SetActionExecutors(
		e.httpExecutor,
		e.emailExecutor,
		e.slackExecutor,
		e.webhookExecutor,
	)

	// Ejecutar usando el executor mejorado
	return e.executor.Execute(ctx, taskData.Workflow, userID, logID, taskData.Data)
}

// 🆕 NUEVO MÉTODO: Limpieza periódica de reintentos
func (e *WorkerEngine) periodicRetryCleanup(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			if err := e.retryManager.CleanupOldRetries(ctx, 24*time.Hour); err != nil {
				e.logger.Error("Failed to cleanup old retries", zap.Error(err))
			}
		}
	}
}

// 🆕 NUEVOS MÉTODOS: Exponer métricas y estado
func (e *WorkerEngine) GetAdvancedStats(ctx context.Context) (map[string]interface{}, error) {
	// Combinar estadísticas existentes con nuevas métricas
	poolStats := e.pool.GetStats()
	metricsStats := e.metrics.GetMetrics()
	retryStats, err := e.retryManager.GetRetryStats(ctx)
	if err != nil {
		retryStats = map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{
		"pool":    poolStats,
		"metrics": metricsStats,
		"retries": retryStats,
		"uptime":  time.Since(e.startTime),
		"engine": map[string]interface{}{
			"is_running": e.isRunning,
			"start_time": e.startTime,
		},
	}, nil
}

func (e *WorkerEngine) GetHealthStatus(ctx context.Context) (*WorkerHealthStatus, error) {
	return e.metrics.CheckHealth(ctx, e.pool, e.startTime)
}

// Métodos para acceder a componentes
func (e *WorkerEngine) GetMetricsCollector() *MetricsCollector {
	return e.metrics
}

func (e *WorkerEngine) GetRetryManager() *RetryManager {
	return e.retryManager
}

func (e *WorkerEngine) GetWorkerPool() *WorkerPool {
	return e.pool
}
