package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.uber.org/zap"
)

// WorkerPool maneja un pool dinámico de workers
type WorkerPool struct {
	engine       *WorkerEngine
	workers      map[int]*Worker
	workersMutex sync.RWMutex
	maxWorkers   int
	minWorkers   int
	currentLoad  int64
	scalingMutex sync.Mutex
	logger       *zap.Logger
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// Worker representa un worker individual
type Worker struct {
	ID       int
	engine   *WorkerEngine
	tasksCh  chan *models.QueueTask
	stopCh   chan struct{}
	isActive bool
	mutex    sync.RWMutex
	stats    WorkerStats
	logger   *zap.Logger
}

// WorkerStats estadísticas de un worker
type WorkerStats struct {
	TasksProcessed int64
	TasksSucceeded int64
	TasksFailed    int64
	LastTaskTime   time.Time
	TotalRunTime   time.Duration
	IsIdle         bool
}

// NewWorkerPool crea un nuevo pool de workers
func NewWorkerPool(engine *WorkerEngine, minWorkers, maxWorkers int, logger *zap.Logger) *WorkerPool {
	return &WorkerPool{
		engine:     engine,
		workers:    make(map[int]*Worker),
		maxWorkers: maxWorkers,
		minWorkers: minWorkers,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Start inicia el pool de workers
func (p *WorkerPool) Start(ctx context.Context) error {
	p.logger.Info("Starting worker pool",
		zap.Int("min_workers", p.minWorkers),
		zap.Int("max_workers", p.maxWorkers))

	// Iniciar workers mínimos
	for i := 0; i < p.minWorkers; i++ {
		if err := p.createWorker(i); err != nil {
			return fmt.Errorf("failed to create initial worker %d: %w", i, err)
		}
	}

	// Iniciar monitor de escalamiento
	p.wg.Add(1)
	go p.scalingMonitor(ctx)

	// Iniciar dispatcher de tareas
	p.wg.Add(1)
	go p.taskDispatcher(ctx)

	return nil
}

// Stop detiene el pool de workers
func (p *WorkerPool) Stop() error {
	p.logger.Info("Stopping worker pool...")

	close(p.stopCh)

	// Detener todos los workers
	p.workersMutex.Lock()
	for _, worker := range p.workers {
		worker.Stop()
	}
	p.workersMutex.Unlock()

	p.wg.Wait()
	p.logger.Info("Worker pool stopped")
	return nil
}

// createWorker crea un nuevo worker
func (p *WorkerPool) createWorker(id int) error {
	worker := &Worker{
		ID:      id,
		engine:  p.engine,
		tasksCh: make(chan *models.QueueTask, 10),
		stopCh:  make(chan struct{}),
		logger:  p.logger.With(zap.Int("worker_id", id)),
		stats:   WorkerStats{IsIdle: true},
	}

	p.workersMutex.Lock()
	p.workers[id] = worker
	p.workersMutex.Unlock()

	p.wg.Add(1)
	go worker.Run()

	p.logger.Info("Created worker", zap.Int("worker_id", id))
	return nil
}

// removeWorker elimina un worker del pool
func (p *WorkerPool) removeWorker(id int) {
	p.workersMutex.Lock()
	defer p.workersMutex.Unlock()

	if worker, exists := p.workers[id]; exists {
		worker.Stop()
		delete(p.workers, id)
		p.logger.Info("Removed worker", zap.Int("worker_id", id))
	}
}

// scalingMonitor monitorea la carga y escala automáticamente
func (p *WorkerPool) scalingMonitor(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.evaluateScaling()
		}
	}
}

// evaluateScaling evalúa si necesita escalar workers
func (p *WorkerPool) evaluateScaling() {
	p.scalingMutex.Lock()
	defer p.scalingMutex.Unlock()

	currentWorkers := len(p.workers)
	queueLength := p.getQueueLength()
	avgLoad := p.calculateAverageLoad()

	p.logger.Debug("Evaluating scaling",
		zap.Int("current_workers", currentWorkers),
		zap.Int64("queue_length", queueLength),
		zap.Float64("avg_load", avgLoad))

	// Escalar hacia arriba si hay mucha carga
	if queueLength > int64(currentWorkers*5) && currentWorkers < p.maxWorkers {
		newWorkerID := p.getNextWorkerID()
		if err := p.createWorker(newWorkerID); err != nil {
			p.logger.Error("Failed to scale up", zap.Error(err))
		} else {
			p.logger.Info("Scaled up workers",
				zap.Int("new_total", len(p.workers)))
		}
	}

	// Escalar hacia abajo si hay poca carga
	if queueLength == 0 && avgLoad < 0.3 && currentWorkers > p.minWorkers {
		idleWorkerID := p.findIdleWorker()
		if idleWorkerID != -1 {
			p.removeWorker(idleWorkerID)
			p.logger.Info("Scaled down workers",
				zap.Int("new_total", len(p.workers)))
		}
	}
}

// taskDispatcher distribuye tareas a workers disponibles
func (p *WorkerPool) taskDispatcher(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
			// Obtener tarea de la cola
			task, err := p.engine.queueRepo.Dequeue(ctx)
			if err != nil {
				if err != repository.ErrQueueEmpty {
					p.logger.Error("Failed to dequeue task", zap.Error(err))
				}
				time.Sleep(1 * time.Second)
				continue
			}

			if task == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			// Encontrar worker disponible
			worker := p.findAvailableWorker()
			if worker == nil {
				// Reencolar la tarea si no hay workers disponibles
				p.logger.Warn("No available workers, re-queuing task")
				time.Sleep(5 * time.Second)
				continue
			}

			// Enviar tarea al worker
			select {
			case worker.tasksCh <- task:
				atomic.AddInt64(&p.currentLoad, 1)
			case <-time.After(5 * time.Second):
				p.logger.Warn("Worker busy, re-queuing task")
			}
		}
	}
}

// findAvailableWorker encuentra un worker disponible
func (p *WorkerPool) findAvailableWorker() *Worker {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	for _, worker := range p.workers {
		worker.mutex.RLock()
		isActive := worker.isActive
		isIdle := worker.stats.IsIdle
		worker.mutex.RUnlock()

		if isActive && isIdle {
			return worker
		}
	}
	return nil
}

// Helper methods
func (p *WorkerPool) getQueueLength() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	length, err := p.engine.queueRepo.GetQueueLength(ctx, "workflow:queue")
	if err != nil {
		return 0
	}
	return length
}

func (p *WorkerPool) calculateAverageLoad() float64 {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	if len(p.workers) == 0 {
		return 0
	}

	totalLoad := 0
	for _, worker := range p.workers {
		worker.mutex.RLock()
		if !worker.stats.IsIdle {
			totalLoad++
		}
		worker.mutex.RUnlock()
	}

	return float64(totalLoad) / float64(len(p.workers))
}

func (p *WorkerPool) findIdleWorker() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	for id, worker := range p.workers {
		worker.mutex.RLock()
		isIdle := worker.stats.IsIdle
		worker.mutex.RUnlock()

		if isIdle {
			return id
		}
	}
	return -1
}

func (p *WorkerPool) getNextWorkerID() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	maxID := -1
	for id := range p.workers {
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

// GetWorkerCount obtiene el número actual de workers (MÉTODO AGREGADO)
func (p *WorkerPool) GetWorkerCount() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()
	return len(p.workers)
}

// Worker methods

// Run ejecuta el loop principal del worker
func (w *Worker) Run() {
	defer func() {
		w.engine.wg.Done()
		w.logger.Info("Worker stopped")
	}()

	w.mutex.Lock()
	w.isActive = true
	w.mutex.Unlock()

	w.logger.Info("Worker started")

	for {
		select {
		case <-w.stopCh:
			return
		case task := <-w.tasksCh:
			w.processTask(task)
		}
	}
}

// processTask procesa una tarea específica
func (w *Worker) processTask(task *models.QueueTask) {
	startTime := time.Now()

	w.mutex.Lock()
	w.stats.IsIdle = false
	w.stats.LastTaskTime = startTime
	w.mutex.Unlock()

	defer func() {
		duration := time.Since(startTime)
		w.mutex.Lock()
		w.stats.IsIdle = true
		w.stats.TasksProcessed++
		w.stats.TotalRunTime += duration
		w.mutex.Unlock()

		atomic.AddInt64(&w.engine.currentLoad, -1)
	}()

	w.logger.Info("Processing task", zap.String("task_id", task.ID))

	// Usar el executor existente del engine
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Aquí integramos con el processTask existente
	err := w.engine.processTask(ctx, task, w.logger)

	w.mutex.Lock()
	if err != nil {
		w.stats.TasksFailed++
		w.logger.Error("Task failed", zap.Error(err))
	} else {
		w.stats.TasksSucceeded++
		w.logger.Info("Task completed successfully")
	}
	w.mutex.Unlock()
}

// Stop detiene el worker
func (w *Worker) Stop() {
	w.mutex.Lock()
	w.isActive = false
	w.mutex.Unlock()

	close(w.stopCh)
}

// GetStats obtiene estadísticas del worker
func (w *Worker) GetStats() WorkerStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.stats
}

// GetStats obtiene estadísticas del pool (MÉTODO RENOMBRADO para evitar conflicto)
func (p *WorkerPool) GetStats() map[string]interface{} {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	stats := map[string]interface{}{
		"total_workers":  len(p.workers),
		"min_workers":    p.minWorkers,
		"max_workers":    p.maxWorkers,
		"current_load":   atomic.LoadInt64(&p.currentLoad),
		"average_load":   p.calculateAverageLoad(),
		"queue_length":   p.getQueueLength(),
		"worker_details": make([]map[string]interface{}, 0),
	}

	for id, worker := range p.workers {
		workerStats := worker.GetStats()
		stats["worker_details"] = append(stats["worker_details"].([]map[string]interface{}), map[string]interface{}{
			"id":              id,
			"tasks_processed": workerStats.TasksProcessed,
			"tasks_succeeded": workerStats.TasksSucceeded,
			"tasks_failed":    workerStats.TasksFailed,
			"is_idle":         workerStats.IsIdle,
			"last_task_time":  workerStats.LastTaskTime,
			"total_run_time":  workerStats.TotalRunTime,
		})
	}

	return stats
}
