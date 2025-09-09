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

func (r *queueRepository) GetQueueStats(ctx context.Context, queueName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get basic queue length
	length, err := r.client.LLen(ctx, queueName).Result()
	if err != nil {
		return nil, err
	}
	stats["length"] = length
	stats["queue_name"] = queueName

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

// AGREGADO: Método faltante GetProcessingTasks
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
			CreatedAt:   item.CreatedAt,
			ProcessedAt: item.ProcessedAt,
			CompletedAt: item.CompletedAt,
			FailedAt:    item.FailedAt,
			Error:       item.Error,
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}
