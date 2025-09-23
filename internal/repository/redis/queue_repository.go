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

// ================================
// FUNCIONES AUXILIARES DE CONVERSIÓN
// ================================

// objectIDToString convierte primitive.ObjectID a string
func objectIDToString(id primitive.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}

// stringToObjectIDPtr convierte string a *primitive.ObjectID
func stringToObjectIDPtr(s string) *primitive.ObjectID {
	if s == "" {
		return nil
	}
	if objID, err := primitive.ObjectIDFromHex(s); err == nil {
		return &objID
	}
	return nil
}

// intToTaskPriority convierte int a models.TaskPriority
func intToTaskPriority(priority int) models.TaskPriority {
	return models.TaskPriority(priority)
}

// stringToTaskStatus convierte string a models.TaskStatus
func stringToTaskStatus(status string) models.TaskStatus {
	switch status {
	case "pending":
		return models.TaskStatusPending
	case "processing":
		return models.TaskStatusProcessing
	case "completed":
		return models.TaskStatusCompleted
	case "failed":
		return models.TaskStatusFailed
	case "cancelled":
		return models.TaskStatusCancelled
	case "queued":
		return models.TaskStatusPending // Mapear 'queued' a 'pending'
	default:
		return models.TaskStatusPending
	}
}

// convertQueueItemToTask convierte QueueItem a models.QueueTask
func convertQueueItemToTask(item *QueueItem) *models.QueueTask {
	task := &models.QueueTask{
		ID:           primitive.NewObjectID(), // Generar nuevo ObjectID
		WorkflowID:   item.WorkflowID,
		ExecutionID:  &item.ExecutionID, // Convertir a pointer
		UserID:       item.UserID,
		TaskName:     "workflow_execution",
		TaskType:     "workflow",
		Status:       stringToTaskStatus(item.Status),
		Priority:     intToTaskPriority(item.Priority),
		QueueName:    "main",
		Payload:      item.Payload,
		CreatedAt:    item.CreatedAt,
		StartedAt:    item.ProcessedAt,
		CompletedAt:  item.CompletedAt,
		UpdatedAt:    time.Now(),
		Attempts:     item.RetryCount,
		MaxAttempts:  item.MaxRetries,
		ErrorMessage: item.Error,
	}

	// Mapear campos específicos
	if item.FailedAt != nil {
		task.CompletedAt = item.FailedAt
	}

	return task
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

// GetQueueLength implementa el método de la interfaz
func (r *queueRepository) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	if queueName == "workflow:queue" || queueName == "" || queueName == "main" {
		return r.client.ZCard(ctx, WorkflowQueue).Result()
	}
	return r.client.LLen(ctx, queueName).Result()
}

// Ping implementa el método de la interfaz
func (r *queueRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Workflow-specific queue operations
func (r *queueRepository) Enqueue(ctx context.Context, workflowID primitive.ObjectID, executionID string, userID primitive.ObjectID, payload map[string]interface{}, priority int) error {
	item := QueueItem{
		ID:          fmt.Sprintf("%s_%d", executionID, time.Now().UnixNano()),
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		UserID:      userID,
		Payload:     payload,
		Priority:    priority,
		MaxRetries:  3,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		Status:      "queued",
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
		Score:  float64(-priority),
		Member: string(data),
	})
	pipe.HIncrBy(ctx, StatsKey, "queued", 1)
	pipe.HIncrBy(ctx, StatsKey, "total", 1)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *queueRepository) EnqueueDelayed(ctx context.Context, workflowID primitive.ObjectID, executionID string, userID primitive.ObjectID, payload map[string]interface{}, priority int, delay time.Duration) error {
	item := QueueItem{
		ID:          fmt.Sprintf("%s_%d", executionID, time.Now().UnixNano()),
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
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	score := float64(time.Now().Add(delay).Unix())
	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, DelayedQueue, redis.Z{
		Score:  score,
		Member: string(data),
	})
	pipe.HIncrBy(ctx, StatsKey, "delayed", 1)
	pipe.HIncrBy(ctx, StatsKey, "total", 1)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *queueRepository) EnqueueAt(ctx context.Context, workflowID primitive.ObjectID, executionID string, userID primitive.ObjectID, payload map[string]interface{}, priority int, scheduledAt time.Time) error {
	delay := time.Until(scheduledAt)
	if delay <= 0 {
		return r.Enqueue(ctx, workflowID, executionID, userID, payload, priority)
	}
	return r.EnqueueDelayed(ctx, workflowID, executionID, userID, payload, priority, delay)
}

// MÉTODOS REQUERIDOS POR WORKER ENGINE

func (r *queueRepository) Dequeue(ctx context.Context) (*models.QueueTask, error) {
	if err := r.moveDelayedToQueue(ctx); err != nil {
		fmt.Printf("Warning: failed to move delayed items: %v\n", err)
	}

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

	item.Status = "processing"
	item.ProcessedAt = timePtr(time.Now())

	processedData, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal processed item: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.SAdd(ctx, ProcessingSet, string(processedData))
	pipe.HIncrBy(ctx, StatsKey, "queued", -1)
	pipe.HIncrBy(ctx, StatsKey, "processing", 1)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to move item to processing: %w", err)
	}

	// Convertir QueueItem a models.QueueTask usando función auxiliar
	task := convertQueueItemToTask(&item)

	return task, nil
}

func (r *queueRepository) MarkCompleted(ctx context.Context, taskID string) error {
	return r.moveFromProcessing(ctx, taskID, CompletedSet, "completed")
}

func (r *queueRepository) MarkFailed(ctx context.Context, taskID string, err error) error {
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return members.Err()
	}

	for _, member := range members.Val() {
		var item QueueItem
		if jsonErr := json.Unmarshal([]byte(member), &item); jsonErr != nil {
			continue
		}

		if item.ID == taskID {
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			}

			if item.RetryCount < item.MaxRetries {
				return r.requeueForRetry(ctx, &item, member, errorMsg)
			} else {
				return r.moveFromProcessing(ctx, taskID, FailedSet, "failed")
			}
		}
	}

	return repository.ErrTaskNotFound
}

func (r *queueRepository) GetProcessingTasks(ctx context.Context) ([]*models.QueueTask, error) {
	return r.getTasksByStatus(ctx, "processing")
}

func (r *queueRepository) GetProcessingTasksCount(ctx context.Context) (int64, error) {
	return r.client.SCard(ctx, ProcessingSet).Result()
}

func (r *queueRepository) CleanupStaleProcessing(ctx context.Context, timeout time.Duration) error {
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return members.Err()
	}

	staleThreshold := time.Now().Add(-timeout)
	pipe := r.client.Pipeline()
	staleCount := int64(0)

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		if item.ProcessedAt != nil && item.ProcessedAt.Before(staleThreshold) {
			item.Status = "queued"
			item.ProcessedAt = nil
			item.RetryCount++

			requeueData, err := json.Marshal(item)
			if err != nil {
				continue
			}

			pipe.SRem(ctx, ProcessingSet, member)

			if item.RetryCount < item.MaxRetries {
				score := float64(-item.Priority)
				pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
					Score:  score,
					Member: string(requeueData),
				})
				pipe.HIncrBy(ctx, StatsKey, "queued", 1)
			} else {
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

// CleanupStaleProcessingTasks - Método requerido por la interfaz que devuelve int64
func (r *queueRepository) CleanupStaleProcessingTasks(ctx context.Context, timeout time.Duration) (int64, error) {
	members := r.client.SMembers(ctx, ProcessingSet)
	if members.Err() != nil {
		return 0, members.Err()
	}

	staleThreshold := time.Now().Add(-timeout)
	pipe := r.client.Pipeline()
	staleCount := int64(0)

	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		if item.ProcessedAt != nil && item.ProcessedAt.Before(staleThreshold) {
			item.Status = "queued"
			item.ProcessedAt = nil
			item.RetryCount++

			requeueData, err := json.Marshal(item)
			if err != nil {
				continue
			}

			pipe.SRem(ctx, ProcessingSet, member)

			if item.RetryCount < item.MaxRetries {
				score := float64(-item.Priority)
				pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
					Score:  score,
					Member: string(requeueData),
				})
				pipe.HIncrBy(ctx, StatsKey, "queued", 1)
			} else {
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
			return 0, fmt.Errorf("failed to cleanup %d stale tasks: %w", staleCount, err)
		}
	}

	return staleCount, nil
}

func (r *queueRepository) GetFailedTasks(ctx context.Context, limit int64) ([]*models.QueueTask, error) {
	return r.getTasksByStatus(ctx, "failed")
}

func (r *queueRepository) GetFailedTasksCount(ctx context.Context) (int64, error) {
	return r.client.SCard(ctx, FailedSet).Result()
}

func (r *queueRepository) GetCompletedTasksCount(ctx context.Context) (int64, error) {
	return r.client.SCard(ctx, CompletedSet).Result()
}

func (r *queueRepository) GetQueuedTasksCount(ctx context.Context) (int64, error) {
	return r.client.ZCard(ctx, WorkflowQueue).Result()
}

func (r *queueRepository) GetRetryingTasksCount(ctx context.Context) (int64, error) {
	return r.client.ZCard(ctx, DelayedQueue).Result()
}

func (r *queueRepository) GetTasksByStatus(ctx context.Context, status string, limit int64) ([]*models.QueueTask, error) {
	return r.getTasksByStatus(ctx, status)
}

func (r *queueRepository) UpdateTaskStatus(ctx context.Context, taskID string, status string, errorMsg string) error {
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

func (r *queueRepository) GetOldestPendingTask(ctx context.Context) (*models.QueueTask, error) {
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

	// Convertir usando función auxiliar
	task := convertQueueItemToTask(&item)

	return task, nil
}

func (r *queueRepository) GetTaskMetrics(ctx context.Context, since time.Time) (map[string]int64, error) {
	stats := make(map[string]int64)

	// Get stats from hash
	hashStats := r.client.HGetAll(ctx, StatsKey)
	if hashStats.Err() == nil {
		for key, value := range hashStats.Val() {
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				stats[key] = val
			}
		}
	}

	return stats, nil
}

func (r *queueRepository) RescheduleTask(ctx context.Context, taskID string, newScheduledTime time.Time) error {
	// Implementation would depend on finding the task and moving it to delayed queue
	return fmt.Errorf("reschedule task not implemented")
}

func (r *queueRepository) RequeueFailedTask(ctx context.Context, taskID string) error {
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
			item.Status = "queued"
			item.RetryCount = 0
			item.Error = ""
			item.FailedAt = nil
			item.ProcessedAt = nil

			requeueData, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal requeue item: %w", err)
			}

			pipe := r.client.Pipeline()
			pipe.SRem(ctx, FailedSet, member)
			pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
				Score:  float64(-item.Priority),
				Member: string(requeueData),
			})
			pipe.HIncrBy(ctx, StatsKey, "failed", -1)
			pipe.HIncrBy(ctx, StatsKey, "queued", 1)

			_, err = pipe.Exec(ctx)
			return err
		}
	}

	return repository.ErrTaskNotFound
}

// Job tracking
func (r *queueRepository) SetJobStatus(ctx context.Context, jobID string, status string) error {
	return r.client.HSet(ctx, "job_status", jobID, status).Err()
}

func (r *queueRepository) GetJobStatus(ctx context.Context, jobID string) (string, error) {
	return r.client.HGet(ctx, "job_status", jobID).Result()
}

// Statistics
func (r *queueRepository) GetStats(ctx context.Context, queueName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	if queueName != "" {
		length, err := r.client.LLen(ctx, queueName).Result()
		if err == nil {
			stats["length"] = length
			stats["queue_name"] = queueName
		}
	}

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

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

func (r *queueRepository) moveDelayedToQueue(ctx context.Context) error {
	now := time.Now().Unix()
	result := r.client.ZRangeByScore(ctx, DelayedQueue, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", now),
	})

	if result.Err() != nil || len(result.Val()) == 0 {
		return result.Err()
	}

	pipe := r.client.Pipeline()
	for _, member := range result.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		item.Status = "queued"
		item.DelayUntil = nil

		updatedData, err := json.Marshal(item)
		if err != nil {
			continue
		}

		score := float64(-item.Priority)
		pipe.ZRem(ctx, DelayedQueue, member)
		pipe.ZAdd(ctx, WorkflowQueue, redis.Z{
			Score:  score,
			Member: string(updatedData),
		})
		pipe.HIncrBy(ctx, StatsKey, "delayed", -1)
		pipe.HIncrBy(ctx, StatsKey, "queued", 1)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *queueRepository) moveFromProcessing(ctx context.Context, taskID string, targetSet string, status string) error {
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

	retryDelay := time.Duration(item.RetryCount*item.RetryCount) * time.Second

	pipe := r.client.Pipeline()
	pipe.SRem(ctx, ProcessingSet, originalData)

	if retryDelay > 0 {
		score := float64(time.Now().Add(retryDelay).Unix())
		pipe.ZAdd(ctx, DelayedQueue, redis.Z{
			Score:  score,
			Member: string(retryData),
		})
		pipe.HIncrBy(ctx, StatsKey, "delayed", 1)
	} else {
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

func (r *queueRepository) getTasksByStatus(ctx context.Context, status string) ([]*models.QueueTask, error) {
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
	for _, member := range members.Val() {
		var item QueueItem
		if err := json.Unmarshal([]byte(member), &item); err != nil {
			continue
		}

		// Usar función auxiliar de conversión
		task := convertQueueItemToTask(&item)

		// Mapear campos específicos que no están en la función auxiliar
		task.LastError = item.Error
		if item.DelayUntil != nil {
			task.ScheduledAt = item.DelayUntil
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}
