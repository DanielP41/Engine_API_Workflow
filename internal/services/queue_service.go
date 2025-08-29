package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// QueueService handles task queuing operations
type QueueService struct {
	redis  *redis.Client
	logger *zap.Logger
}

// QueueTask represents a task in the queue
type QueueTask struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
	CreatedAt   time.Time              `json:"created_at"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
}

// Task types
const (
	TaskTypeWorkflowExecution = "workflow_execution"
	TaskTypeNotification      = "notification"
	TaskTypeWebhookCall       = "webhook_call"
	TaskTypeSlackMessage      = "slack_message"
)

// Queue names
const (
	QueueHigh      = "queue:high"
	QueueDefault   = "queue:default"
	QueueLow       = "queue:low"
	QueueScheduled = "queue:scheduled"
	QueueFailed    = "queue:failed"
)

// NewQueueService creates a new queue service
func NewQueueService(redis *redis.Client, logger *zap.Logger) *QueueService {
	return &QueueService{
		redis:  redis,
		logger: logger,
	}
}

// EnqueueTask adds a task to a specific queue
func (s *QueueService) EnqueueTask(ctx context.Context, queueName string, task *QueueTask) error {
	if task.ID == "" {
		task.ID = fmt.Sprintf("%s_%d", task.Type, time.Now().UnixNano())
	}

	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		s.logger.Error("Failed to marshal task", zap.Error(err), zap.String("task_id", task.ID))
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	if err := s.redis.LPush(ctx, queueName, taskJSON).Err(); err != nil {
		s.logger.Error("Failed to enqueue task", zap.Error(err),
			zap.String("queue", queueName), zap.String("task_id", task.ID))
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	s.logger.Info("Task enqueued successfully",
		zap.String("queue", queueName),
		zap.String("task_id", task.ID),
		zap.String("task_type", task.Type))

	return nil
}

// DequeueTask retrieves and removes a task from the queue
func (s *QueueService) DequeueTask(ctx context.Context, queueNames ...string) (*QueueTask, error) {
	if len(queueNames) == 0 {
		queueNames = []string{QueueHigh, QueueDefault, QueueLow}
	}

	result, err := s.redis.BRPop(ctx, 30*time.Second, queueNames...).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No tasks available
		}
		s.logger.Error("Failed to dequeue task", zap.Error(err))
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("invalid result from queue")
	}

	var task QueueTask
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		s.logger.Error("Failed to unmarshal task", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	s.logger.Info("Task dequeued successfully",
		zap.String("queue", result[0]),
		zap.String("task_id", task.ID),
		zap.String("task_type", task.Type))

	return &task, nil
}

// EnqueueWorkflowExecution adds a workflow execution task
func (s *QueueService) EnqueueWorkflowExecution(ctx context.Context, workflowID string, triggerData map[string]interface{}) error {
	task := &QueueTask{
		Type: TaskTypeWorkflowExecution,
		Payload: map[string]interface{}{
			"workflow_id":  workflowID,
			"trigger_data": triggerData,
		},
	}

	return s.EnqueueTask(ctx, QueueHigh, task)
}

// EnqueueNotification adds a notification task
func (s *QueueService) EnqueueNotification(ctx context.Context, notificationType string, recipient string, message string, metadata map[string]interface{}) error {
	task := &QueueTask{
		Type: TaskTypeNotification,
		Payload: map[string]interface{}{
			"notification_type": notificationType,
			"recipient":         recipient,
			"message":           message,
			"metadata":          metadata,
		},
	}

	return s.EnqueueTask(ctx, QueueDefault, task)
}

// EnqueueSlackMessage adds a Slack message task
func (s *QueueService) EnqueueSlackMessage(ctx context.Context, channel string, message string, attachments interface{}) error {
	task := &QueueTask{
		Type: TaskTypeSlackMessage,
		Payload: map[string]interface{}{
			"channel":     channel,
			"message":     message,
			"attachments": attachments,
		},
	}

	return s.EnqueueTask(ctx, QueueDefault, task)
}

// EnqueueWebhookCall adds a webhook call task
func (s *QueueService) EnqueueWebhookCall(ctx context.Context, url string, method string, headers map[string]string, payload interface{}) error {
	task := &QueueTask{
		Type: TaskTypeWebhookCall,
		Payload: map[string]interface{}{
			"url":     url,
			"method":  method,
			"headers": headers,
			"payload": payload,
		},
	}

	return s.EnqueueTask(ctx, QueueDefault, task)
}

// ScheduleTask schedules a task for later execution
func (s *QueueService) ScheduleTask(ctx context.Context, task *QueueTask, executeAt time.Time) error {
	task.ScheduledAt = &executeAt

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled task: %w", err)
	}

	score := float64(executeAt.Unix())
	if err := s.redis.ZAdd(ctx, QueueScheduled, redis.Z{
		Score:  score,
		Member: taskJSON,
	}).Err(); err != nil {
		return fmt.Errorf("failed to schedule task: %w", err)
	}

	s.logger.Info("Task scheduled successfully",
		zap.String("task_id", task.ID),
		zap.Time("execute_at", executeAt))

	return nil
}

// ProcessScheduledTasks moves scheduled tasks that are ready to execute to the appropriate queues
func (s *QueueService) ProcessScheduledTasks(ctx context.Context) error {
	now := time.Now().Unix()

	result, err := s.redis.ZRangeByScore(ctx, QueueScheduled, &redis.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%d", now),
		Count: 100,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get scheduled tasks: %w", err)
	}

	for _, taskJSON := range result {
		var task QueueTask
		if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
			s.logger.Error("Failed to unmarshal scheduled task", zap.Error(err))
			continue
		}

		// Remove from scheduled queue
		if err := s.redis.ZRem(ctx, QueueScheduled, taskJSON).Err(); err != nil {
			s.logger.Error("Failed to remove scheduled task", zap.Error(err), zap.String("task_id", task.ID))
			continue
		}

		// Add to appropriate queue
		queueName := QueueDefault
		if task.Type == TaskTypeWorkflowExecution {
			queueName = QueueHigh
		}

		if err := s.EnqueueTask(ctx, queueName, &task); err != nil {
			s.logger.Error("Failed to move scheduled task to queue", zap.Error(err), zap.String("task_id", task.ID))
			continue
		}

		s.logger.Info("Scheduled task moved to queue", zap.String("task_id", task.ID), zap.String("queue", queueName))
	}

	return nil
}

// RetryFailedTask retries a failed task
func (s *QueueService) RetryFailedTask(ctx context.Context, task *QueueTask) error {
	if task.RetryCount >= task.MaxRetries {
		s.logger.Warn("Task exceeded max retries", zap.String("task_id", task.ID), zap.Int("retry_count", task.RetryCount))
		return s.EnqueueTask(ctx, QueueFailed, task)
	}

	task.RetryCount++
	queueName := QueueLow // Lower priority for retries

	return s.EnqueueTask(ctx, queueName, task)
}

// GetQueueStats returns statistics about the queues
func (s *QueueService) GetQueueStats(ctx context.Context) (map[string]int64, error) {
	queues := []string{QueueHigh, QueueDefault, QueueLow, QueueFailed}
	stats := make(map[string]int64)

	for _, queue := range queues {
		length, err := s.redis.LLen(ctx, queue).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get length of queue %s: %w", queue, err)
		}
		stats[queue] = length
	}

	// Get scheduled tasks count
	scheduledCount, err := s.redis.ZCard(ctx, QueueScheduled).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled tasks count: %w", err)
	}
	stats[QueueScheduled] = scheduledCount

	return stats, nil
}

// ClearQueue removes all tasks from a specific queue
func (s *QueueService) ClearQueue(ctx context.Context, queueName string) error {
	if err := s.redis.Del(ctx, queueName).Err(); err != nil {
		return fmt.Errorf("failed to clear queue %s: %w", queueName, err)
	}

	s.logger.Info("Queue cleared", zap.String("queue", queueName))
	return nil
}
