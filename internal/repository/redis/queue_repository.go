package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"Engine_API_Workflow/internal/repository"
)

type queueRepository struct {
	client *redis.Client
}

// NewQueueRepository crea una nueva instancia del repositorio de cola
func NewQueueRepository(client *redis.Client) repository.QueueRepository {
	return &queueRepository{
		client: client,
	}
}

// Push añade un mensaje a la cola
func (r *queueRepository) Push(ctx context.Context, queueName string, message interface{}) error {
	if queueName == "" {
		return fmt.Errorf("queue name cannot be empty")
	}
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// Serializar el mensaje a JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Añadir el mensaje al final de la lista (cola FIFO)
	err = r.client.LPush(ctx, queueName, messageJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to push message to queue %s: %w", queueName, err)
	}

	return nil
}

// Pop obtiene y elimina un mensaje de la cola (blocking)
func (r *queueRepository) Pop(ctx context.Context, queueName string, timeout time.Duration) ([]byte, error) {
	if queueName == "" {
		return nil, fmt.Errorf("queue name cannot be empty")
	}

	// BRPOP es blocking right pop - espera hasta timeout si la cola está vacía
	result, err := r.client.BRPop(ctx, timeout, queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Timeout, no hay mensajes
		}
		return nil, fmt.Errorf("failed to pop message from queue %s: %w", queueName, err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BRPOP result")
	}

	// result[0] es el nombre de la cola, result[1] es el mensaje
	return []byte(result[1]), nil
}

// PopNoBlock obtiene y elimina un mensaje sin bloquear
func (r *queueRepository) PopNoBlock(ctx context.Context, queueName string) ([]byte, error) {
	if queueName == "" {
		return nil, fmt.Errorf("queue name cannot be empty")
	}

	result, err := r.client.RPop(ctx, queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cola vacía
		}
		return nil, fmt.Errorf("failed to pop message from queue %s: %w", queueName, err)
	}

	return []byte(result), nil
}

// Peek obtiene un mensaje sin eliminarlo de la cola
func (r *queueRepository) Peek(ctx context.Context, queueName string) ([]byte, error) {
	if queueName == "" {
		return nil, fmt.Errorf("queue name cannot be empty")
	}

	// Obtener el último elemento (el siguiente a ser procesado) sin eliminarlo
	result, err := r.client.LIndex(ctx, queueName, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cola vacía
		}
		return nil, fmt.Errorf("failed to peek message from queue %s: %w", queueName, err)
	}

	return []byte(result), nil
}

// Length obtiene el número de mensajes en la cola
func (r *queueRepository) Length(ctx context.Context, queueName string) (int64, error) {
	if queueName == "" {
		return 0, fmt.Errorf("queue name cannot be empty")
	}

	length, err := r.client.LLen(ctx, queueName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length for %s: %w", queueName, err)
	}

	return length, nil
}

// Clear elimina todos los mensajes de la cola
func (r *queueRepository) Clear(ctx context.Context, queueName string) error {
	if queueName == "" {
		return fmt.Errorf("queue name cannot be empty")
	}

	err := r.client.Del(ctx, queueName).Err()
	if err != nil {
		return fmt.Errorf("failed to clear queue %s: %w", queueName, err)
	}

	return nil
}

// Exists verifica si una cola existe y tiene mensajes
func (r *queueRepository) Exists(ctx context.Context, queueName string) (bool, error) {
	if queueName == "" {
		return false, fmt.Errorf("queue name cannot be empty")
	}

	exists, err := r.client.Exists(ctx, queueName).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if queue %s exists: %w", queueName, err)
	}

	return exists > 0, nil
}

// PushWithDelay añade un mensaje que será procesado después de un delay
func (r *queueRepository) PushWithDelay(ctx context.Context, queueName string, message interface{}, delay time.Duration) error {
	if queueName == "" {
		return fmt.Errorf("queue name cannot be empty")
	}
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// Serializar el mensaje
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Usar sorted set para manejar delay - score es el timestamp cuando debe procesarse
	processAt := time.Now().Add(delay).Unix()
	delayedQueueName := fmt.Sprintf("%s:delayed", queueName)

	err = r.client.ZAdd(ctx, delayedQueueName, &redis.Z{
		Score:  float64(processAt),
		Member: messageJSON,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to push delayed message to queue %s: %w", queueName, err)
	}

	return nil
}

// ProcessDelayedMessages mueve mensajes listos desde la cola con delay a la cola principal
func (r *queueRepository) ProcessDelayedMessages(ctx context.Context, queueName string) (int, error) {
	if queueName == "" {
		return 0, fmt.Errorf("queue name cannot be empty")
	}

	delayedQueueName := fmt.Sprintf("%s:delayed", queueName)
	now := time.Now().Unix()

	// Obtener mensajes que ya deben procesarse (score <= now)
	messages, err := r.client.ZRangeByScoreWithScores(ctx, delayedQueueName, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get delayed messages: %w", err)
	}

	processed := 0
	for _, msg := range messages {
		// Mover mensaje a la cola principal
		err = r.client.LPush(ctx, queueName, msg.Member).Err()
		if err != nil {
			continue // Log error pero continúa con otros mensajes
		}

		// Eliminar de la cola con delay
		err = r.client.ZRem(ctx, delayedQueueName, msg.Member).Err()
		if err != nil {
			continue // Log error pero continúa
		}

		processed++
	}

	return processed, nil
}

// GetQueueStats obtiene estadísticas de la cola
func (r *queueRepository) GetQueueStats(ctx context.Context, queueName string) (map[string]interface{}, error) {
	if queueName == "" {
		return nil, fmt.Errorf("queue name cannot be empty")
	}

	stats := make(map[string]interface{})

	// Longitud de la cola principal
	length, err := r.Length(ctx, queueName)
	if err != nil {
		return nil, err
	}
	stats["queue_length"] = length

	// Longitud de la cola con delay
	delayedQueueName := fmt.Sprintf("%s:delayed", queueName)
	delayedLength, err := r.client.ZCard(ctx, delayedQueueName).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get delayed queue length: %w", err)
	}
	stats["delayed_queue_length"] = delayedLength

	// Información adicional
	stats["queue_name"] = queueName
	stats["timestamp"] = time.Now().Unix()

	return stats, nil
}

// ListQueues obtiene una lista de todas las colas disponibles
func (r *queueRepository) ListQueues(ctx context.Context, pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}

	return keys, nil
}

// BatchPush añade múltiples mensajes a la cola de forma atómica
func (r *queueRepository) BatchPush(ctx context.Context, queueName string, messages []interface{}) error {
	if queueName == "" {
		return fmt.Errorf("queue name cannot be empty")
	}
	if len(messages) == 0 {
		return nil // No hay nada que hacer
	}

	// Usar pipeline para operación atómica
	pipe := r.client.Pipeline()

	for _, message := range messages {
		if message == nil {
			continue
		}

		messageJSON, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		pipe.LPush(ctx, queueName, messageJSON)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to batch push messages: %w", err)
	}

	return nil
}

// Health verifica la salud de la conexión Redis
func (r *queueRepository) Health(ctx context.Context) error {
	_, err := r.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	return nil
}
