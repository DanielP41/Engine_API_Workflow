package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
)

type logRepository struct {
	collection *mongo.Collection
	db         *mongo.Database
}

// NewLogRepository crea una nueva instancia del repositorio de logs
func NewLogRepository(db *mongo.Database) repository.LogRepository {
	collection := db.Collection("logs")

	// Crear índices
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "workflow_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "level", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "execution_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
	}

	collection.Indexes().CreateMany(ctx, indexes)

	return &logRepository{
		collection: collection,
		db:         db,
	}
}

// Create crea un nuevo log
func (r *logRepository) Create(ctx context.Context, log *models.Log) error {
	if log == nil {
		return fmt.Errorf("log cannot be nil")
	}

	// Validaciones básicas
	if log.WorkflowID == primitive.NilObjectID {
		return fmt.Errorf("workflow_id is required")
	}

	if log.Message == "" {
		return fmt.Errorf("message is required")
	}

	if log.Level == "" {
		log.Level = "info"
	}

	// Establecer timestamps
	now := time.Now()
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}

	// Generar nuevo ID si no existe
	if log.ID == primitive.NilObjectID {
		log.ID = primitive.NewObjectID()
	}

	result, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("failed to create log: %w", err)
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		log.ID = oid
	}

	return nil
}

// GetByID obtiene un log por su ID
func (r *logRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Log, error) {
	if id == primitive.NilObjectID {
		return nil, fmt.Errorf("invalid log ID")
	}

	var log models.Log
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, repository.ErrLogNotFound
		}
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	return &log, nil
}

// GetByWorkflowID obtiene logs por workflow ID con paginación
func (r *logRepository) GetByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, limit, offset int) ([]*models.Log, error) {
	if workflowID == primitive.NilObjectID {
		return nil, fmt.Errorf("invalid workflow ID")
	}

	if limit <= 0 {
		limit = 50
	}

	filter := bson.M{"workflow_id": workflowID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find logs by workflow: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	for cursor.Next(ctx) {
		var log models.Log
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// GetByUserID obtiene logs por user ID con paginación
func (r *logRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, offset int) ([]*models.Log, error) {
	if userID == primitive.NilObjectID {
		return nil, fmt.Errorf("invalid user ID")
	}

	if limit <= 0 {
		limit = 50
	}

	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find logs by user: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	for cursor.Next(ctx) {
		var log models.Log
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// GetByExecutionID obtiene logs por execution ID
func (r *logRepository) GetByExecutionID(ctx context.Context, executionID string) ([]*models.Log, error) {
	if executionID == "" {
		return nil, fmt.Errorf("execution_id is required")
	}

	filter := bson.M{"execution_id": executionID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find logs by execution: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	for cursor.Next(ctx) {
		var log models.Log
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// GetByLevel obtiene logs por nivel con paginación
func (r *logRepository) GetByLevel(ctx context.Context, level string, limit, offset int) ([]*models.Log, error) {
	if level == "" {
		return nil, fmt.Errorf("level is required")
	}

	if limit <= 0 {
		limit = 50
	}

	filter := bson.M{"level": level}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find logs by level: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	for cursor.Next(ctx) {
		var log models.Log
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// GetByDateRange obtiene logs en un rango de fechas
func (r *logRepository) GetByDateRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*models.Log, error) {
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("start and end dates are required")
	}

	if start.After(end) {
		return nil, fmt.Errorf("start date cannot be after end date")
	}

	if limit <= 0 {
		limit = 50
	}

	filter := bson.M{
		"created_at": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find logs by date range: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	for cursor.Next(ctx) {
		var log models.Log
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// Search busca logs con filtros avanzados
func (r *logRepository) Search(ctx context.Context, filter repository.LogSearchFilter) ([]*models.Log, error) {
	query := bson.M{}

	// Aplicar filtros
	if filter.WorkflowID != nil && *filter.WorkflowID != primitive.NilObjectID {
		query["workflow_id"] = *filter.WorkflowID
	}

	if filter.UserID != nil && *filter.UserID != primitive.NilObjectID {
		query["user_id"] = *filter.UserID
	}

	if filter.Level != nil && *filter.Level != "" {
		query["level"] = *filter.Level
	}

	if filter.Status != nil && *filter.Status != "" {
		query["status"] = *filter.Status
	}

	if filter.ExecutionID != nil && *filter.ExecutionID != "" {
		query["execution_id"] = *filter.ExecutionID
	}

	if filter.Message != nil && *filter.Message != "" {
		query["message"] = bson.M{"$regex": *filter.Message, "$options": "i"}
	}

	// Filtro de fechas
	if filter.StartDate != nil || filter.EndDate != nil {
		dateFilter := bson.M{}
		if filter.StartDate != nil {
			dateFilter["$gte"] = *filter.StartDate
		}
		if filter.EndDate != nil {
			dateFilter["$lte"] = *filter.EndDate
		}
		query["created_at"] = dateFilter
	}

	// Opciones de consulta
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(filter.Limit)).
		SetSkip(int64(filter.Offset))

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	for cursor.Next(ctx) {
		var log models.Log
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// Delete elimina un log por ID
func (r *logRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if id == primitive.NilObjectID {
		return fmt.Errorf("invalid log ID")
	}

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete log: %w", err)
	}

	if result.DeletedCount == 0 {
		return repository.ErrLogNotFound
	}

	return nil
}

// DeleteByWorkflowID elimina logs por workflow ID
func (r *logRepository) DeleteByWorkflowID(ctx context.Context, workflowID primitive.ObjectID) error {
	if workflowID == primitive.NilObjectID {
		return fmt.Errorf("invalid workflow ID")
	}

	_, err := r.collection.DeleteMany(ctx, bson.M{"workflow_id": workflowID})
	if err != nil {
		return fmt.Errorf("failed to delete logs by workflow: %w", err)
	}

	return nil
}

// DeleteByUserID elimina logs por user ID
func (r *logRepository) DeleteByUserID(ctx context.Context, userID primitive.ObjectID) error {
	if userID == primitive.NilObjectID {
		return fmt.Errorf("invalid user ID")
	}

	_, err := r.collection.DeleteMany(ctx, bson.M{"user_id": userID})
	if err != nil {
		return fmt.Errorf("failed to delete logs by user: %w", err)
	}

	return nil
}

// DeleteOldLogs elimina logs anteriores a la fecha especificada
func (r *logRepository) DeleteOldLogs(ctx context.Context, before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, fmt.Errorf("before date is required")
	}

	result, err := r.collection.DeleteMany(ctx, bson.M{
		"created_at": bson.M{"$lt": before},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %w", err)
	}

	return result.DeletedCount, nil
}

// Count cuenta el total de logs
func (r *logRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}
	return count, nil
}

// CountByWorkflow cuenta logs por workflow
func (r *logRepository) CountByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (int64, error) {
	if workflowID == primitive.NilObjectID {
		return 0, fmt.Errorf("invalid workflow ID")
	}

	count, err := r.collection.CountDocuments(ctx, bson.M{"workflow_id": workflowID})
	if err != nil {
		return 0, fmt.Errorf("failed to count logs by workflow: %w", err)
	}
	return count, nil
}

// GetStats obtiene estadísticas de logs
func (r *logRepository) GetStats(ctx context.Context, workflowID *primitive.ObjectID) (map[string]interface{}, error) {
	matchStage := bson.M{}
	if workflowID != nil && *workflowID != primitive.NilObjectID {
		matchStage["workflow_id"] = *workflowID
	}

	pipeline := []bson.M{
		{"$match": matchStage},
		{
			"$group": bson.M{
				"_id":    "$level",
				"count":  bson.M{"$sum": 1},
				"latest": bson.M{"$max": "$created_at"},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get log stats: %w", err)
	}
	defer cursor.Close(ctx)

	stats := make(map[string]interface{})
	levelStats := make(map[string]map[string]interface{})

	for cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode stats: %w", err)
		}

		level := result["_id"].(string)
		levelStats[level] = map[string]interface{}{
			"count":  result["count"],
			"latest": result["latest"],
		}
	}

	stats["by_level"] = levelStats

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, matchStage)
	if err != nil {
		return nil, fmt.Errorf("failed to count total logs: %w", err)
	}
	stats["total"] = totalCount

	return stats, nil
}
