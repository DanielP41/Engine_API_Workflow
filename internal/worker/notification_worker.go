package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"Engine_API_Workflow/internal/services"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// NotificationWorker worker para procesamiento de notificaciones
type NotificationWorker struct {
	notificationService *services.NotificationService
	redisClient         *redis.Client
	logger              *zap.Logger
	workerID            string
	stopChan            chan struct{}
	wg                  sync.WaitGroup

	// Configuración
	config *NotificationWorkerConfig
}

// NotificationWorkerConfig configuración del worker
type NotificationWorkerConfig struct {
	// Configuración de procesamiento
	MaxConcurrentJobs  int           `json:"max_concurrent_jobs"`
	ProcessingInterval time.Duration `json:"processing_interval"`
	RetryInterval      time.Duration `json:"retry_interval"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`

	// Configuración de colas Redis
	QueuePrefix     string `json:"queue_prefix"`
	PendingQueue    string `json:"pending_queue"`
	ProcessingQueue string `json:"processing_queue"`
	FailedQueue     string `json:"failed_queue"`
	RetryQueue      string `json:"retry_queue"`

	// Configuración de timeouts
	JobTimeout  time.Duration `json:"job_timeout"`
	LockTimeout time.Duration `json:"lock_timeout"`

	// Configuración de reintentos
	MaxRetries         int           `json:"max_retries"`
	InitialRetryDelay  time.Duration `json:"initial_retry_delay"`
	MaxRetryDelay      time.Duration `json:"max_retry_delay"`
	RetryBackoffFactor float64       `json:"retry_backoff_factor"`
}

// NotificationJob trabajo de notificación
type NotificationJob struct {
	ID             string                 `json:"id"`
	NotificationID string                 `json:"notification_id"`
	Type           string                 `json:"type"`
	Priority       int                    `json:"priority"`
	Payload        map[string]interface{} `json:"payload"`
	CreatedAt      time.Time              `json:"created_at"`
	ScheduledAt    *time.Time             `json:"scheduled_at,omitempty"`
	Attempts       int                    `json:"attempts"`
	MaxAttempts    int                    `json:"max_attempts"`
	LastError      string                 `json:"last_error,omitempty"`
	LastAttemptAt  *time.Time             `json:"last_attempt_at,omitempty"`
	NextRetryAt    *time.Time             `json:"next_retry_at,omitempty"`
}

// NewNotificationWorker crea un nuevo worker de notificaciones
func NewNotificationWorker(
	notificationService *services.NotificationService,
	redisClient *redis.Client,
	logger *zap.Logger,
	config *NotificationWorkerConfig,
) *NotificationWorker {
	if config == nil {
		config = DefaultNotificationWorkerConfig()
	}

	workerID := fmt.Sprintf("notification-worker-%d", time.Now().UnixNano())

	return &NotificationWorker{
		notificationService: notificationService,
		redisClient:         redisClient,
		logger:              logger,
		workerID:            workerID,
		stopChan:            make(chan struct{}),
		config:              config,
	}
}

// DefaultNotificationWorkerConfig configuración por defecto
func DefaultNotificationWorkerConfig() *NotificationWorkerConfig {
	return &NotificationWorkerConfig{
		MaxConcurrentJobs:  5,
		ProcessingInterval: 10 * time.Second,
		RetryInterval:      30 * time.Second,
		CleanupInterval:    5 * time.Minute,

		QueuePrefix:     "notifications:",
		PendingQueue:    "notifications:pending",
		ProcessingQueue: "notifications:processing",
		FailedQueue:     "notifications:failed",
		RetryQueue:      "notifications:retry",

		JobTimeout:  5 * time.Minute,
		LockTimeout: 10 * time.Minute,

		MaxRetries:         3,
		InitialRetryDelay:  30 * time.Second,
		MaxRetryDelay:      5 * time.Minute,
		RetryBackoffFactor: 2.0,
	}
}

// Start inicia el worker
func (w *NotificationWorker) Start(ctx context.Context) error {
	w.logger.Info("Starting notification worker", zap.String("worker_id", w.workerID))

	// Iniciar goroutines de procesamiento
	w.wg.Add(4)

	// Worker principal para procesar notificaciones pendientes
	go w.runPendingProcessor(ctx)

	// Worker para reintentos
	go w.runRetryProcessor(ctx)

	// Worker para limpieza
	go w.runCleanupProcessor(ctx)

	// Worker para notificaciones programadas
	go w.runScheduledProcessor(ctx)

	w.logger.Info("Notification worker started successfully")
	return nil
}

// Stop detiene el worker
func (w *NotificationWorker) Stop() error {
	w.logger.Info("Stopping notification worker", zap.String("worker_id", w.workerID))

	close(w.stopChan)
	w.wg.Wait()

	w.logger.Info("Notification worker stopped")
	return nil
}

// runPendingProcessor procesa notificaciones pendientes
func (w *NotificationWorker) runPendingProcessor(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.ProcessingInterval)
	defer ticker.Stop()

	w.logger.Info("Pending processor started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Pending processor stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Info("Pending processor stopped")
			return
		case <-ticker.C:
			w.processPendingNotifications(ctx)
		}
	}
}

// runRetryProcessor procesa reintentos
func (w *NotificationWorker) runRetryProcessor(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.RetryInterval)
	defer ticker.Stop()

	w.logger.Info("Retry processor started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Retry processor stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Info("Retry processor stopped")
			return
		case <-ticker.C:
			w.processRetryNotifications(ctx)
		}
	}
}

// runCleanupProcessor limpia datos antiguos
func (w *NotificationWorker) runCleanupProcessor(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.CleanupInterval)
	defer ticker.Stop()

	w.logger.Info("Cleanup processor started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Cleanup processor stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Info("Cleanup processor stopped")
			return
		case <-ticker.C:
			w.runCleanupTasks(ctx)
		}
	}
}

// runScheduledProcessor procesa notificaciones programadas
func (w *NotificationWorker) runScheduledProcessor(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(time.Minute) // Verificar cada minuto
	defer ticker.Stop()

	w.logger.Info("Scheduled processor started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Scheduled processor stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Info("Scheduled processor stopped")
			return
		case <-ticker.C:
			w.processScheduledNotifications(ctx)
		}
	}
}

// processPendingNotifications procesa notificaciones pendientes
func (w *NotificationWorker) processPendingNotifications(ctx context.Context) {
	// Procesar usando el servicio de notificaciones directamente
	if err := w.notificationService.ProcessPendingNotifications(ctx); err != nil {
		w.logger.Error("Failed to process pending notifications", zap.Error(err))
	}
}

// processRetryNotifications procesa reintentos
func (w *NotificationWorker) processRetryNotifications(ctx context.Context) {
	if err := w.notificationService.RetryFailedNotifications(ctx); err != nil {
		w.logger.Error("Failed to process retry notifications", zap.Error(err))
	}
}

// processScheduledNotifications procesa notificaciones programadas
func (w *NotificationWorker) processScheduledNotifications(ctx context.Context) {
	// Obtener notificaciones programadas que deben ejecutarse
	now := time.Now()
	script := `
		local scheduled_key = KEYS[1]
		local pending_key = KEYS[2]
		local now = ARGV[1]
		
		-- Obtener trabajos programados que deben ejecutarse
		local jobs = redis.call('ZRANGEBYSCORE', scheduled_key, '-inf', now, 'LIMIT', 0, 100)
		
		for i, job_data in ipairs(jobs) do
			-- Mover a cola pendiente
			redis.call('LPUSH', pending_key, job_data)
			-- Remover de cola programada
			redis.call('ZREM', scheduled_key, job_data)
		end
		
		return #jobs
	`

	scheduledKey := w.config.QueuePrefix + "scheduled"
	pendingKey := w.config.PendingQueue

	result, err := w.redisClient.Eval(ctx, script, []string{scheduledKey, pendingKey}, now.Unix()).Result()
	if err != nil {
		w.logger.Error("Failed to process scheduled notifications", zap.Error(err))
		return
	}

	if count, ok := result.(int64); ok && count > 0 {
		w.logger.Info("Moved scheduled notifications to pending queue", zap.Int64("count", count))
	}
}

// runCleanupTasks ejecuta tareas de limpieza
func (w *NotificationWorker) runCleanupTasks(ctx context.Context) {
	// Limpiar trabajos completados antiguos
	w.cleanupCompletedJobs(ctx)

	// Limpiar notificaciones antiguas en la base de datos
	deletedCount, err := w.notificationService.CleanupOldNotifications(ctx, 90*24*time.Hour)
	if err != nil {
		w.logger.Error("Failed to cleanup old notifications", zap.Error(err))
	} else if deletedCount > 0 {
		w.logger.Info("Cleaned up old notifications", zap.Int64("count", deletedCount))
	}
}

// cleanupCompletedJobs limpia trabajos completados de Redis
func (w *NotificationWorker) cleanupCompletedJobs(ctx context.Context) {
	// Limpiar trabajos fallidos antiguos (más de 7 días)
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()

	script := `
		local failed_key = KEYS[1]
		local cutoff = ARGV[1]
		
		-- Obtener trabajos antiguos
		local old_jobs = redis.call('ZRANGEBYSCORE', failed_key, '-inf', cutoff)
		
		-- Remover trabajos antiguos
		if #old_jobs > 0 then
			redis.call('ZREMRANGEBYSCORE', failed_key, '-inf', cutoff)
		end
		
		return #old_jobs
	`

	failedKey := w.config.FailedQueue

	result, err := w.redisClient.Eval(ctx, script, []string{failedKey}, cutoff).Result()
	if err != nil {
		w.logger.Error("Failed to cleanup completed jobs", zap.Error(err))
		return
	}

	if count, ok := result.(int64); ok && count > 0 {
		w.logger.Info("Cleaned up old failed jobs", zap.Int64("count", count))
	}
}

// EnqueueNotification encola una notificación para procesamiento
func (w *NotificationWorker) EnqueueNotification(ctx context.Context, notificationID string, priority int) error {
	job := &NotificationJob{
		ID:             fmt.Sprintf("notif-%s-%d", notificationID, time.Now().UnixNano()),
		NotificationID: notificationID,
		Type:           "email",
		Priority:       priority,
		CreatedAt:      time.Now(),
		Attempts:       0,
		MaxAttempts:    w.config.MaxRetries,
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Agregar a cola pendiente con prioridad
	score := float64(time.Now().Unix())
	if priority > 0 {
		score -= float64(priority * 1000) // Prioridades más altas se procesan primero
	}

	err = w.redisClient.ZAdd(ctx, w.config.PendingQueue, &redis.Z{
		Score:  score,
		Member: string(jobData),
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to enqueue notification: %w", err)
	}

	w.logger.Debug("Notification enqueued",
		zap.String("job_id", job.ID),
		zap.String("notification_id", notificationID),
		zap.Int("priority", priority))

	return nil
}

// EnqueueScheduledNotification programa una notificación para envío futuro
func (w *NotificationWorker) EnqueueScheduledNotification(ctx context.Context, notificationID string, scheduledAt time.Time, priority int) error {
	job := &NotificationJob{
		ID:             fmt.Sprintf("scheduled-%s-%d", notificationID, time.Now().UnixNano()),
		NotificationID: notificationID,
		Type:           "email",
		Priority:       priority,
		CreatedAt:      time.Now(),
		ScheduledAt:    &scheduledAt,
		Attempts:       0,
		MaxAttempts:    w.config.MaxRetries,
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled job: %w", err)
	}

	// Agregar a cola programada
	scheduledKey := w.config.QueuePrefix + "scheduled"
	score := float64(scheduledAt.Unix())

	err = w.redisClient.ZAdd(ctx, scheduledKey, &redis.Z{
		Score:  score,
		Member: string(jobData),
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to enqueue scheduled notification: %w", err)
	}

	w.logger.Debug("Scheduled notification enqueued",
		zap.String("job_id", job.ID),
		zap.String("notification_id", notificationID),
		zap.Time("scheduled_at", scheduledAt))

	return nil
}

// GetQueueStats obtiene estadísticas de las colas
func (w *NotificationWorker) GetQueueStats(ctx context.Context) (map[string]int64, error) {
	pipe := w.redisClient.Pipeline()

	pendingCmd := pipe.ZCard(ctx, w.config.PendingQueue)
	processingCmd := pipe.LLen(ctx, w.config.ProcessingQueue)
	failedCmd := pipe.ZCard(ctx, w.config.FailedQueue)
	retryCmd := pipe.ZCard(ctx, w.config.RetryQueue)
	scheduledCmd := pipe.ZCard(ctx, w.config.QueuePrefix+"scheduled")

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	stats := map[string]int64{
		"pending":    pendingCmd.Val(),
		"processing": processingCmd.Val(),
		"failed":     failedCmd.Val(),
		"retry":      retryCmd.Val(),
		"scheduled":  scheduledCmd.Val(),
	}

	return stats, nil
}

// HealthCheck verifica el estado del worker
func (w *NotificationWorker) HealthCheck(ctx context.Context) error {
	// Verificar conexión Redis
	if err := w.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	// Verificar que el servicio de notificaciones esté disponible
	if w.notificationService == nil {
		return fmt.Errorf("notification service is not available")
	}

	return nil
}

// WorkerStats estadísticas del worker
type WorkerStats struct {
	WorkerID        string           `json:"worker_id"`
	Status          string           `json:"status"`
	StartTime       time.Time        `json:"start_time"`
	ProcessedJobs   int64            `json:"processed_jobs"`
	FailedJobs      int64            `json:"failed_jobs"`
	QueueStats      map[string]int64 `json:"queue_stats"`
	LastProcessedAt *time.Time       `json:"last_processed_at,omitempty"`
}

// GetStats obtiene estadísticas del worker
func (w *NotificationWorker) GetStats(ctx context.Context) (*WorkerStats, error) {
	queueStats, err := w.GetQueueStats(ctx)
	if err != nil {
		return nil, err
	}

	stats := &WorkerStats{
		WorkerID:   w.workerID,
		Status:     "running",
		QueueStats: queueStats,
	}

	return stats, nil
}

// NotificationWorkerManager gestor de múltiples workers
type NotificationWorkerManager struct {
	workers []*NotificationWorker
	logger  *zap.Logger
	config  *NotificationWorkerConfig
}

// NewNotificationWorkerManager crea un nuevo gestor de workers
func NewNotificationWorkerManager(logger *zap.Logger, config *NotificationWorkerConfig) *NotificationWorkerManager {
	return &NotificationWorkerManager{
		workers: make([]*NotificationWorker, 0),
		logger:  logger,
		config:  config,
	}
}

// AddWorker agrega un worker al gestor
func (m *NotificationWorkerManager) AddWorker(worker *NotificationWorker) {
	m.workers = append(m.workers, worker)
}

// StartAll inicia todos los workers
func (m *NotificationWorkerManager) StartAll(ctx context.Context) error {
	for i, worker := range m.workers {
		if err := worker.Start(ctx); err != nil {
			m.logger.Error("Failed to start worker", zap.Int("worker_index", i), zap.Error(err))
			return err
		}
	}

	m.logger.Info("All notification workers started", zap.Int("count", len(m.workers)))
	return nil
}

// StopAll detiene todos los workers
func (m *NotificationWorkerManager) StopAll() error {
	for i, worker := range m.workers {
		if err := worker.Stop(); err != nil {
			m.logger.Error("Failed to stop worker", zap.Int("worker_index", i), zap.Error(err))
		}
	}

	m.logger.Info("All notification workers stopped")
	return nil
}

// GetAggregatedStats obtiene estadísticas agregadas de todos los workers
func (m *NotificationWorkerManager) GetAggregatedStats(ctx context.Context) (map[string]interface{}, error) {
	totalStats := map[string]interface{}{
		"worker_count": len(m.workers),
		"queues":       make(map[string]int64),
	}

	aggregatedQueues := make(map[string]int64)

	for _, worker := range m.workers {
		stats, err := worker.GetStats(ctx)
		if err != nil {
			continue
		}

		for queueName, count := range stats.QueueStats {
			aggregatedQueues[queueName] += count
		}
	}

	totalStats["queues"] = aggregatedQueues
	return totalStats, nil
}
