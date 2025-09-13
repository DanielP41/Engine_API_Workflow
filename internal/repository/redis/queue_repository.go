// internal/repository/redis/queue_repository.go
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
)

type queueRepository struct {
	client *redis.Client
}

// NewQueueRepository creates a new queue repository
func NewQueueRepository(client *redis.Client) repository.QueueRepository {
	return &queueRepository{
		client: client,
	}
}

// Queue names
const (
	WorkflowQueue = "workflow:queue"
	RetryQueue    = "workflow:retry"
	DelayedQueue  = "workflow:delayed"
	ProcessingSet = "workflow:processing"
	CompletedSet  = "workflow:completed"
	FailedSet     = "workflow:failed"
	StatsKey      = "workflow:stats"
)

// QueueItem represents an item in the queue
type QueueItem struct {
	ID          string                 `json:"id"`
	WorkflowID  primitive.ObjectID     `json:"workflow_id"`
	ExecutionID string                 `json:"execution_id"`
	UserID      primitive.ObjectID     `json:"user_id"`
	Payload     map[string]interface{} `json:"payload"`
	Priority    int                    `json:"priority"`
	MaxRetries  int                    `json:"max_retries"`
	RetryCount  int                    `json:"retry_count"`
	CreatedAt   time.Time              `json:"created_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	FailedAt    *time.Time             `json:"failed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Status      string                 `json:"status"`
	DelayUntil  *time.Time             `json:"delay_until,omitempty"`
}

// Basic queue operations
func (r *queueRepository) Push(ctx context.Context, queueName string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	return r.client.LPush(ctx, queueName, jsonData).Err()
}

func (r *queueRepository) Pop(ctx context.Context, queueName string, timeout int) (interface{}, error) {
	result := r.client.BRPop(ctx, time.Duration(timeout)*time.Second, queueName)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, repository.ErrQueueEmpty
		}
		return nil, result.Err()
	}

	items := result.Val()
	if len(items) < 2 {
		return nil, repository.ErrQueueEmpty
	}

	var data interface{}
	if err := json.Unmarshal([]byte(items[1]), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return data, nil
}

func (r *queueRepository) PopBlocking(ctx context.Context, queueName string, timeout int) (interface{}, error) {
	return r.Pop(ctx, queueName, timeout)
}

func (r *queueRepository) Length(ctx context.Context, queueName string) (int64, error) {
	return r.client.LLen(ctx, queueName).Result()
}

func (r *queueRepository) Peek(ctx context.Context, queueName string) (interface{}, error) {
	result := r.client.LIndex(ctx, queueName, -1)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, repository.ErrQueueEmpty
		}
		return nil, result.Err()
	}

	var data interface{}
	if err := json.Unmarshal([]byte(result.Val()), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return data, nil
}

func (r *queueRepository) Clear(ctx context.Context, queueName string) error {
	return r.client.Del(ctx, queueName).Err()
}

func (r *queueRepository) PushPriority(ctx context.Context, queueName string, data interface{}, priority int64) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return r.client.ZAdd(ctx, queueName, redis.Z{
		Score:  float64(-priority),
		Member: jsonData,
	}).Err()
}

func (r *queueRepository) PopPriority(ctx context.Context, queueName string, timeout int) (interface{}, error) {
	result := r.client.ZPopMin(ctx, queueName, 1)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, repository.ErrQueueEmpty
		}
		return nil, result.Err()
	}

	items := result.Val()
	if len(items) == 0 {
		return nil, repository.ErrQueueEmpty
	}

	var data interface{}
	if err := json.Unmarshal([]byte(items[0].Member.(string)), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return data, nil
}

func (r *queueRepository) PushDelayed(ctx context.Context, queueName string, data interface{}, delay int64) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	executeTime := time.Now().Add(time.Duration(delay) * time.Second)
	return r.client.ZAdd(ctx, queueName+"_delayed", redis.Z{
		Score:  float64(executeTime.Unix()),
		Member: jsonData,
	}).Err()
}

func (r *queueRepository) ProcessDelayedJobs(ctx context.Context, queueName string) error {
	now := time.Now().Unix()
	delayedQueue := queueName + "_delayed"

	result := r.client.ZRangeByScore(ctx, delayedQueue, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", now),
	})

	if result.Err() != nil || len(result.Val()) == 0 {
		return result.Err()
	}

	pipe := r.client.Pipeline()
	for _, member := range result.Val() {
		pipe.ZRem(ctx, delayedQueue, member)
		pipe.LPush(ctx, queueName, member)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *queueRepository) SetJobStatus(ctx context.Context, jobID string, status string) error {
	return r.client.HSet(ctx, "job_status", jobID, status).Err()
}

func (r *queueRepository) GetJobStatus(ctx context.Context, jobID string) (string, error) {
	return r.client.HGet(ctx, "job_status", jobID).Result()
}

// CORREGIDO: Renombrado método para coincidir con interface
func (r *queueRepository) GetStats(ctx context.Context, queueName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get basic queue length if queueName provided
	if queueName != "" {
		length, err := r.client.LLen(ctx, queueName).Result()
		if err == nil {
			stats["length"] = length
			stats["queue_name"] = queueName
		}
	}

	// Get additional stats from hash
	hashStats := r.client.HGetAll(ctx, StatsKey)
	if hashStats.Err() == nil {
		for key, value := range hashStats.Val() {
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				stats[key] = val
			} else {
				stats[key] = value
			}
		}
	}

	// Get queue lengths for different types
	if zLen := r.client.ZCard(ctx, WorkflowQueue); zLen.Err() == nil {
		stats["workflow_queue_length"] = zLen.Val()
	}

	if zLen := r.client.ZCard(ctx, DelayedQueue); zLen.Err() == nil {
		stats["delayed_queue_length"] = zLen.Val()
	}

	if sLen := r.client.SCard(ctx, ProcessingSet); sLen.Err() == nil {
		stats["processing_count"] = sLen.Val()
	}

	return stats, nil
}

func (r *queueRepository) GetAllQueueNames(ctx context.Context) ([]string, error) {
	pattern := "*queue*"
	return r.client.Keys(ctx, pattern).Result()
}

func (r *queueRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *queueRepository) GetProcessingTasks(ctx context.Context) ([]*models.QueueTask, error) {
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return nil, members.Err()
	}

	var tasks []*models.QueueTask

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue // Skip malformed items
		}

		task := &models.QueueTask{
			ID:          item.ID,
			WorkflowID:  item.WorkflowID,
			ExecutionID: item.ExecutionID,
			UserID:      item.UserID,
			Payload:     item.Payload,
			Priority:    item.Priority,
			RetryCount:  item.RetryCount,
			MaxRetries:  item.MaxRetries,
			Status:      models.QueueTaskStatus(item.Status),
			LastError:   item.Error,
			ScheduledAt: item.DelayUntil,
			CreatedAt:   item.CreatedAt,
			ProcessedAt: item.ProcessedAt,
			CompletedAt: item.CompletedAt,
			FailedAt:    item.FailedAt,
			Error:       item.Error,
		}

		tasks = append(tasks, task)
		count++
	}

	return tasks, nil
}

// UpdateTaskStatus actualiza el estado de una tarea
func (r *queueRepository) UpdateTaskStatus(ctx context.Context, taskID string, status string, errorMsg string) error {
	// Esta implementación es básica - buscar en todos los sets
	sets := []string{ProcessingSet, CompletedSet, FailedSet}
	
	for _, setName := range sets {
		members := r.client.SMembers(ctx, setName)
		if members.Err() != nil {
			continue
		}

		for _, member := range members.Val() {
			var item QueueItem
			if err := json.Unmarshal([]byte(member), &item); err != nil {
				continue
			}

			if item.ID == taskID {
				// Update the item
				item.Status = status
				if errorMsg != "" {
					item.Error = errorMsg
				}

				updatedData, err := json.Marshal(item)
				if err != nil {
					return fmt.Errorf("failed to marshal updated item: %w", err)
				}

				pipe := r.client.Pipeline()
				pipe.SRem(ctx, setName, member)
				pipe.SAdd(ctx, setName, string(updatedData))

				_, err = pipe.Exec(ctx)
				return err
			}
		}
	}

	return fmt.Errorf("task not found: %s", taskID)
}

// GetOldestPendingTask obtiene la tarea pendiente más antigua
func (r *queueRepository) GetOldestPendingTask(ctx context.Context) (*models.QueueTask, error) {
	// Get oldest item from main queue (highest score = oldest)
	result := r.client.ZRangeWithScores(ctx, WorkflowQueue, 0, 0)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, repository.ErrQueueEmpty
		}
		return nil, result.Err()
	}

	items := result.Val()
	if len(items) == 0 {
		return nil, repository.ErrQueueEmpty
	}

	var item QueueItem
	if err := json.Unmarshal([]byte(items[0].Member.(string)), &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
	}

	task := &models.QueueTask{
		ID:          item.ID,
		WorkflowID:  item.WorkflowID,
		ExecutionID: item.ExecutionID,
		UserID:      item.UserID,
		Payload:     item.Payload,
		Priority:    item.Priority,
		RetryCount:  item.RetryCount,
		MaxRetries:  item.MaxRetries,
		Status:      models.QueueTaskStatus(item.Status),
		LastError:   item.Error,
		ScheduledAt: item.DelayUntil,
		CreatedAt:   item.CreatedAt,
		ProcessedAt: item.ProcessedAt,
	}

	return task, nil
}

// GetTaskMetrics obtiene métricas agregadas de tareas
func (r *queueRepository) GetTaskMetrics(ctx context.Context, since time.Time) (map[string]int64, error) {
	metrics := make(map[string]int64)

	// Get counts from hash stats
	hashStats := r.client.HGetAll(ctx, StatsKey)
	if hashStats.Err() == nil {
		for key, value := range hashStats.Val() {
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				metrics[key] = val
			}
		}
	}

	// Get current queue lengths
	if queueLen := r.client.ZCard(ctx, WorkflowQueue); queueLen.Err() == nil {
		metrics["current_queued"] = queueLen.Val()
	}

	if processingLen := r.client.SCard(ctx, ProcessingSet); processingLen.Err() == nil {
		metrics["current_processing"] = processingLen.Val()
	}

	if completedLen := r.client.SCard(ctx, CompletedSet); completedLen.Err() == nil {
		metrics["current_completed"] = completedLen.Val()
	}

	if failedLen := r.client.SCard(ctx, FailedSet); failedLen.Err() == nil {
		metrics["current_failed"] = failedLen.Val()
	}

	if delayedLen := r.client.ZCard(ctx, DelayedQueue); delayedLen.Err() == nil {
		metrics["current_delayed"] = delayedLen.Val()
	}

	return metrics, nil
}

// RescheduleTask reagenda una tarea para ejecución posterior
func (r *queueRepository) RescheduleTask(ctx context.Context, taskID string, newScheduledTime time.Time) error {
	// Find the task in processing set
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return members.Err()
	}

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		if item.ID == taskID {
			// Update scheduling info
			item.DelayUntil = &newScheduledTime
			item.Status = "pending"
			item.ProcessedAt = nil

			rescheduledData, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal rescheduled item: %w", err)
			}

			pipe := r.client.Pipeline()
			// Remove from processing
			pipe.SRem(ctx, ProcessingSet, member)
			// Add to delayed queue
			score := float64(newScheduledTime.Unix())
			pipe.ZAdd(ctx, DelayedQueue, redis.Z{
				Score:  score,
				Member: string(rescheduledData),
			})
			// Update stats
			pipe.HIncrBy(ctx, StatsKey, "processing", -1)
			pipe.HIncrBy(ctx, StatsKey, "scheduled", 1)

			_, err = pipe.Exec(ctx)
			return err
		}
	}

	return fmt.Errorf("task not found in processing: %s", taskID)
}

// HELPER METHODS

func (r *queueRepository) moveDelayedToQueue(ctx context.Context) error {
	now := time.Now().Unix()

	// Get items that are ready to be processed
	items := r.client.ZRangeByScore(ctx, DelayedQueue, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", now),
	})

	if items.Err() != nil || len(items.Val()) == 0 {
		return items.Err()
	}

	pipe := r.client.Pipeline()

	for _, member := range items.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue // Skip malformed items
		}

		// Remove from delayed queue
		pipe.ZRem(ctx, DelayedQueue, member)

		// Update status and add to main queue
		item.Status = "queued"
		item.DelayUntil = nil

		updatedData, err := json.Marshal(item)
		if err != nil {
			continue
		}

		// Add to main queue with original priority
		score := float64(-item.Priority)
		pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
			Score:  score,
			Member: string(updatedData),
		})

		// Update stats
		pipe.HIncrBy(ctx, StatsKey, "delayed", -1)
		pipe.HIncrBy(ctx, StatsKey, "queued", 1)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *queueRepository) moveFromProcessing(ctx context.Context, taskID string, targetSet string, status string) error {
	// Get all processing items
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return members.Err()
	}

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		if item.ID == taskID {
			// Update item status
			item.Status = status
			if status == "completed" {
				item.CompletedAt = timePtr(time.Now())
			} else if status == "failed" {
				item.FailedAt = timePtr(time.Now())
			}

			updatedData, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal updated item: %w", err)
			}

			pipe := r.client.Pipeline()
			pipe.SRem(ctx, ProcessingSet, member)
			pipe.SAdd(ctx, targetSet, string(updatedData))
			pipe.HIncrBy(ctx, StatsKey, "processing", -1)
			pipe.HIncrBy(ctx, StatsKey, status, 1)

			_, execErr := pipe.Exec(ctx)
			return execErr
		}
	}

	return repository.ErrTaskNotFound
}

func (r *queueRepository) requeueForRetry(ctx context.Context, item *QueueItem, originalData string, errorMsg string) error {
	item.RetryCount++
	item.Status = "queued"
	item.Error = errorMsg
	item.ProcessedAt = nil

	retryData, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal retry item: %w", err)
	}

	// Calculate delay for retry (exponential backoff)
	retryDelay := time.Duration(item.RetryCount*item.RetryCount) * time.Second

	pipe := r.client.Pipeline()
	pipe.SRem(ctx, ProcessingSet, originalData)

	if retryDelay > 0 {
		// Add to delayed queue with retry delay
		score := float64(time.Now().Add(retryDelay).Unix())
		pipe.ZAdd(ctx, DelayedQueue, redis.Z{
			Score:  score,
			Member: string(retryData),
		})
		pipe.HIncrBy(ctx, StatsKey, "delayed", 1)
	} else {
		// Add back to main queue
		score := float64(-item.Priority)
		pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
			Score:  score,
			Member: string(retryData),
		})
		pipe.HIncrBy(ctx, StatsKey, "queued", 1)
	}

	pipe.HIncrBy(ctx, StatsKey, "processing", -1)
	pipe.HIncrBy(ctx, StatsKey, "retries", 1)

	_, execErr := pipe.Exec(ctx)
	return execErr
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

// GetQueueLength obtiene la longitud de una cola específica
func (r *queueRepository) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	// Para workflow queue, usar el método específico
	if queueName == "workflow:queue" || queueName == "" {
		return r.client.ZCard(ctx, WorkflowQueue).Result()
	}

	// Para otras colas, usar LLen
	length, err := r.client.LLen(ctx, queueName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length for %s: %w", queueName, err)
	}

	return length, nil
} = append(tasks, task)
	}

	return tasks, nil
}

// MÉTODOS REQUERIDOS POR WORKER ENGINE

// Dequeue implementa el método requerido por worker engine
func (r *queueRepository) Dequeue(ctx context.Context) (*models.QueueTask, error) {
	// First, move any ready delayed items to main queue
	if err := r.moveDelayedToQueue(ctx); err != nil {
		// Log error but don't fail the dequeue
		fmt.Printf("Warning: failed to move delayed items: %v\n", err)
	}

	// Get highest priority item from main queue
	result := r.client.ZPopMin(ctx, WorkflowQueue, 1)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, repository.ErrQueueEmpty
		}
		return nil, result.Err()
	}

	items := result.Val()
	if len(items) == 0 {
		return nil, repository.ErrQueueEmpty
	}

	var item QueueItem
	if err := json.Unmarshal([]byte(items[0].Member.(string)), &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
	}

	// Move to processing set
	item.Status = "processing"
	item.ProcessedAt = timePtr(time.Now())

	processedData, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal processing item: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.SAdd(ctx, ProcessingSet, string(processedData))
	pipe.HIncrBy(ctx, StatsKey, "queued", -1)
	pipe.HIncrBy(ctx, StatsKey, "processing", 1)

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to move item to processing: %w", err)
	}

	// Convert to QueueTask
	task := &models.QueueTask{
		ID:          item.ID,
		WorkflowID:  item.WorkflowID,
		ExecutionID: item.ExecutionID,
		UserID:      item.UserID,
		Payload:     item.Payload,
		Priority:    item.Priority,
		RetryCount:  item.RetryCount,
		MaxRetries:  item.MaxRetries,
		Status:      models.QueueTaskStatus(item.Status),
		LastError:   item.Error,
		ScheduledAt: item.DelayUntil,
		CreatedAt:   item.CreatedAt,
		ProcessedAt: item.ProcessedAt,
	}

	return task, nil
}

// MarkCompleted implementa el método requerido por worker engine
func (r *queueRepository) MarkCompleted(ctx context.Context, taskID string) error {
	return r.moveFromProcessing(ctx, taskID, CompletedSet, "completed")
}

// MarkFailed implementa el método requerido por worker engine
func (r *queueRepository) MarkFailed(ctx context.Context, taskID string, err error) error {
	// First get the item from processing
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return members.Err()
	}

	var targetItem *QueueItem
	var targetData string

	for _, member := range members.Val() {
		var item QueueItem
		if jsonErr := json.Unmarshal([]byte(member), &item); jsonErr != nil {
			continue
		}

		if item.ID == taskID {
			targetItem = &item
			targetData = member
			break
		}
	}

	if targetItem == nil {
		return repository.ErrTaskNotFound
	}

	// Check if we should retry
	if targetItem.RetryCount < targetItem.MaxRetries {
		return r.requeueForRetry(ctx, targetItem, targetData, err.Error())
	}

	// Mark as permanently failed
	targetItem.Status = "failed"
	targetItem.FailedAt = timePtr(time.Now())
	targetItem.Error = err.Error()

	failedData, marshalErr := json.Marshal(targetItem)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal failed item: %w", marshalErr)
	}

	pipe := r.client.Pipeline()
	pipe.SRem(ctx, ProcessingSet, targetData)
	pipe.SAdd(ctx, FailedSet, string(failedData))
	pipe.HIncrBy(ctx, StatsKey, "processing", -1)
	pipe.HIncrBy(ctx, StatsKey, "failed", 1)

	_, execErr := pipe.Exec(ctx)
	return execErr
}

// WORKFLOW-SPECIFIC QUEUE OPERATIONS

func (r *queueRepository) Enqueue(ctx context.Context, workflowID primitive.ObjectID, executionID string, userID primitive.ObjectID, payload map[string]interface{}, priority int) error {
	item := QueueItem{
		ID:          primitive.NewObjectID().Hex(),
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		UserID:      userID,
		Payload:     payload,
		Priority:    priority,
		MaxRetries:  3, // Default max retries
		RetryCount:  0,
		CreatedAt:   time.Now(),
		Status:      "queued",
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	// Use sorted set for priority queue (higher priority = lower score for Redis)
	score := float64(-priority) // Negative for higher priority first

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
		Score:  score,
		Member: string(data),
	})

	// Update stats
	pipe.HIncrBy(ctx, StatsKey, "queued", 1)
	pipe.HIncrBy(ctx, StatsKey, "total", 1)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *queueRepository) EnqueueDelayed(ctx context.Context, workflowID primitive.ObjectID, executionID string, userID primitive.ObjectID, payload map[string]interface{}, priority int, delay time.Duration) error {
	item := QueueItem{
		ID:          primitive.NewObjectID().Hex(),
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		UserID:      userID,
		Payload:     payload,
		Priority:    priority,
		MaxRetries:  3,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		Status:      "delayed",
		DelayUntil:  timePtr(time.Now().Add(delay)),
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal delayed queue item: %w", err)
	}

	// Use sorted set with timestamp as score for delayed processing
	score := float64(time.Now().Add(delay).Unix())

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, DelayedQueue, redis.Z{
		Score:  score,
		Member: string(data),
	})

	// Update stats
	pipe.HIncrBy(ctx, StatsKey, "delayed", 1)
	pipe.HIncrBy(ctx, StatsKey, "total", 1)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *queueRepository) CleanupStaleProcessing(ctx context.Context, timeout time.Duration) error {
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return members.Err()
	}

	staleThreshold := time.Now().Add(-timeout)
	pipe := r.client.Pipeline()
	staleCount := 0

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		// Check if processing started too long ago
		if item.ProcessedAt != nil && item.ProcessedAt.Before(staleThreshold) {
			// Move back to queue for retry
			item.Status = "queued"
			item.ProcessedAt = nil
			item.RetryCount++

			requeueData, err := json.Marshal(item)
			if err != nil {
				continue
			}

			pipe.SRem(ctx, ProcessingSet, member)

			if item.RetryCount < item.MaxRetries {
				// Add back to queue
				score := float64(-item.Priority)
				pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
					Score:  score,
					Member: string(requeueData),
				})
				pipe.HIncrBy(ctx, StatsKey, "queued", 1)
			} else {
				// Mark as failed
				item.Status = "failed"
				item.FailedAt = timePtr(time.Now())
				item.Error = "Processing timeout - maximum retries exceeded"

				failedData, err := json.Marshal(item)
				if err != nil {
					continue
				}

				pipe.SAdd(ctx, FailedSet, string(failedData))
				pipe.HIncrBy(ctx, StatsKey, "failed", 1)
			}

			pipe.HIncrBy(ctx, StatsKey, "processing", -1)
			staleCount++
		}
	}

	if staleCount > 0 {
		_, err := pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to cleanup %d stale tasks: %w", staleCount, err)
		}
	}

	return nil
}

func (r *queueRepository) GetFailedTasks(ctx context.Context, limit int64) ([]*models.QueueTask, error) {
	members := r.client.SMembers(ctx, FailedSet)
	if members.Err() != nil {
		return nil, members.Err()
	}

	var tasks []*models.QueueTask
	count := int64(0)

	for _, member := range members.Val() {
		if limit > 0 && count >= limit {
			break
		}

		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		task := &models.QueueTask{
			ID:          item.ID,
			WorkflowID:  item.WorkflowID,
			ExecutionID: item.ExecutionID,
			UserID:      item.UserID,
			Payload:     item.Payload,
			Priority:    item.Priority,
			RetryCount:  item.RetryCount,
			MaxRetries:  item.MaxRetries,
			Status:      models.QueueTaskStatus(item.Status),
			LastError:   item.Error,
			ScheduledAt: item.DelayUntil,
			CreatedAt:   item.CreatedAt,
			ProcessedAt: item.ProcessedAt,
			CompletedAt: item.CompletedAt,
			FailedAt:    item.FailedAt,
			Error:       item.Error,
		}

		tasks = append(tasks, task)
		count++
	}

	return tasks, nil
}

func (r *queueRepository) RequeueFailedTask(ctx context.Context, taskID string) error {
	// Find the failed task
	members := r.client.SMembers(ctx, FailedSet)
	if members.Err() != nil {
		return members.Err()
	}

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		if item.ID == taskID {
			// Reset retry count and error
			item.RetryCount = 0
			item.Error = ""
			item.Status = "queued"
			item.ProcessedAt = nil
			item.FailedAt = nil

			requeueData, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal requeue item: %w", err)
			}

			pipe := r.client.Pipeline()
			pipe.SRem(ctx, FailedSet, member)

			// Add back to main queue
			score := float64(-item.Priority)
			pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
				Score:  score,
				Member: string(requeueData),
			})

			pipe.HIncrBy(ctx, StatsKey, "failed", -1)
			pipe.HIncrBy(ctx, StatsKey, "queued", 1)

			_, execErr := pipe.Exec(ctx)
			return execErr
		}
	}

	return repository.ErrTaskNotFound
}

// MÉTODOS AGREGADOS PARA COMPLETAR LA IMPLEMENTACIÓN

// GetFailedTasksCount obtiene el conteo de tareas fallidas
func (r *queueRepository) GetFailedTasksCount(ctx context.Context) (int64, error) {
	count := r.client.SCard(ctx, FailedSet)
	if count.Err() != nil {
		return 0, count.Err()
	}
	return count.Val(), nil
}

// EnqueueAt programa una tarea para ejecución en un tiempo específico
func (r *queueRepository) EnqueueAt(ctx context.Context, workflowID primitive.ObjectID, executionID string, userID primitive.ObjectID, payload map[string]interface{}, priority int, scheduledAt time.Time) error {
	item := QueueItem{
		ID:          primitive.NewObjectID().Hex(),
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		UserID:      userID,
		Payload:     payload,
		Priority:    priority,
		MaxRetries:  3, // Default
		RetryCount:  0,
		CreatedAt:   time.Now(),
		Status:      "pending",
		DelayUntil:  &scheduledAt, // Usar DelayUntil para el tiempo programado
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled item: %w", err)
	}

	// Usar timestamp como score en sorted set para delayed queue
	score := float64(scheduledAt.Unix())

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, DelayedQueue, redis.Z{
		Score:  score,
		Member: string(data),
	})

	// Update stats
	pipe.HIncrBy(ctx, StatsKey, "scheduled", 1)
	pipe.HIncrBy(ctx, StatsKey, "total", 1)

	_, err = pipe.Exec(ctx)
	return err
}

// GetProcessingTasksCount obtiene el conteo de tareas en procesamiento
func (r *queueRepository) GetProcessingTasksCount(ctx context.Context) (int64, error) {
	count := r.client.SCard(ctx, ProcessingSet)
	if count.Err() != nil {
		return 0, count.Err()
	}
	return count.Val(), nil
}

// GetCompletedTasksCount obtiene el conteo de tareas completadas
func (r *queueRepository) GetCompletedTasksCount(ctx context.Context) (int64, error) {
	count := r.client.SCard(ctx, CompletedSet)
	if count.Err() != nil {
		return 0, count.Err()
	}
	return count.Val(), nil
}

// GetQueuedTasksCount obtiene el conteo de tareas en cola
func (r *queueRepository) GetQueuedTasksCount(ctx context.Context) (int64, error) {
	count := r.client.ZCard(ctx, WorkflowQueue)
	if count.Err() != nil {
		return 0, count.Err()
	}
	return count.Val(), nil
}

// GetRetryingTasksCount obtiene el conteo de tareas reintentándose
func (r *queueRepository) GetRetryingTasksCount(ctx context.Context) (int64, error) {
	count := r.client.ZCard(ctx, DelayedQueue)
	if count.Err() != nil {
		return 0, count.Err()
	}
	return count.Val(), nil
}

// CleanupStaleProcessingTasks limpia tareas en procesamiento obsoletas
func (r *queueRepository) CleanupStaleProcessingTasks(ctx context.Context, timeout time.Duration) (int64, error) {
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return 0, members.Err()
	}

	cutoff := time.Now().Add(-timeout)
	var staleItems []string
	var cleanedCount int64

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		// Check if task is stale
		if item.ProcessedAt != nil && item.ProcessedAt.Before(cutoff) {
			staleItems = append(staleItems, member)
		}
	}

	// Remove stale items
	if len(staleItems) > 0 {
		pipe := r.client.Pipeline()
		for _, staleItem := range staleItems {
			pipe.SRem(ctx, ProcessingSet, staleItem)
			pipe.SAdd(ctx, FailedSet, staleItem) // Move to failed set
			cleanedCount++
		}
		pipe.HIncrBy(ctx, StatsKey, "processing", -cleanedCount)
		pipe.HIncrBy(ctx, StatsKey, "failed", cleanedCount)

		_, err := pipe.Exec(ctx)
		if err != nil {
			return 0, err
		}
	}

	return cleanedCount, nil
}

// GetTasksByStatus obtiene tareas por estado
func (r *queueRepository) GetTasksByStatus(ctx context.Context, status string, limit int64) ([]*models.QueueTask, error) {
	var setName string
	
	switch status {
	case "processing":
		setName = ProcessingSet
	case "completed":
		setName = CompletedSet
	case "failed":
		setName = FailedSet
	default:
		return nil, fmt.Errorf("unsupported status: %s", status)
	}

	members := r.client.SMembers(ctx, setName)
	if members.Err() != nil {
		return nil, members.Err()
	}

	var tasks []*models.QueueTask
	count := int64(0)

	for _, member := range members.Val() {
		if limit > 0 && count >= limit {
			break
		}

		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		task := &models.QueueTask{
			ID:          item.ID,
			WorkflowID:  item.WorkflowID,
			ExecutionID: item.ExecutionID,
			UserID:      item.UserID,
			Payload:     item.Payload,
			Priority:    item.Priority,
			RetryCount:  item.RetryCount,
			MaxRetries:  item.MaxRetries,
			Status:      models.QueueTaskStatus(item.Status),
			LastError:   item.Error,
			ScheduledAt: item.DelayUntil,
			CreatedAt:   item.CreatedAt,
			ProcessedAt: item.ProcessedAt,
			CompletedAt: item.CompletedAt,
			FailedAt:    item.FailedAt,
			Error:       item.Error,
		}

		tasks