package mongodb

import (
	"context"
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
}

// NewLogRepository creates a new log repository
func NewLogRepository(db *mongo.Database) repository.LogRepository {
	collection := db.Collection("workflow_logs") // Nombre consistente

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "workflow_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("workflow_created_idx"),
		},
		{
			Keys: bson.D{
				{Key: "execution_id", Value: 1},
			},
			Options: options.Index().SetName("execution_id_idx"),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("status_created_idx"),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("user_created_idx"),
		},
	}

	collection.Indexes().CreateMany(ctx, indexes)

	return &logRepository{
		collection: collection,
	}
}

func (r *logRepository) Create(ctx context.Context, log *models.WorkflowLog) error {
	if log.ID.IsZero() {
		log.ID = primitive.NewObjectID()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	log.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return err
	}

	return nil
}

func (r *logRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.WorkflowLog, error) {
	var log models.WorkflowLog

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, repository.ErrLogNotFound
		}
		return nil, err
	}

	return &log, nil
}

func (r *logRepository) GetByExecutionID(ctx context.Context, executionID string) (*models.WorkflowLog, error) {
	var log models.WorkflowLog

	err := r.collection.FindOne(ctx, bson.M{"execution_id": executionID}).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, repository.ErrLogNotFound
		}
		return nil, err
	}

	return &log, nil
}

// Update actualiza un log 
func (r *logRepository) Update(ctx context.Context, id primitive.ObjectID, update map[string]interface{}) error {
	if len(update) == 0 {
		return nil
	}

	update["updated_at"] = time.Now()

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return repository.ErrLogNotFound
	}

	return nil
}

func (r *logRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return repository.ErrLogNotFound
	}

	return nil
}

// GetByWorkflowID con signature unificada - CORREGIDO
func (r *logRepository) GetByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, opts repository.PaginationOptions) ([]*models.WorkflowLog, int64, error) {
	filter := bson.M{"workflow_id": workflowID}

	// Contar total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Configurar paginación
	page := opts.Page
	pageSize := opts.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize

	// Configurar ordenamiento
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := -1 // desc por defecto
	if opts.SortDesc {
		sortOrder = -1
	}

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: sortBy, Value: sortOrder}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// Search con signature unificada - CORREGIDO
func (r *logRepository) Search(ctx context.Context, filter repository.LogSearchFilter, opts repository.PaginationOptions) ([]*models.WorkflowLog, int64, error) {
	query := bson.M{}

	// Build query based on filters
	if filter.WorkflowID != nil {
		query["workflow_id"] = *filter.WorkflowID
	}

	if filter.UserID != nil {
		query["user_id"] = *filter.UserID
	}

	if filter.ExecutionID != nil {
		query["execution_id"] = *filter.ExecutionID
	}

	if filter.Status != nil {
		query["status"] = *filter.Status
	}

	if filter.Level != nil {
		query["level"] = *filter.Level
	}

	// Date range filter
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

	// Message contains filter
	if filter.MessageContains != nil && *filter.MessageContains != "" {
		query["error_message"] = bson.M{"$regex": *filter.MessageContains, "$options": "i"}
	}

	// Contar total
	total, err := r.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	// Configurar paginación
	page := opts.Page
	pageSize := opts.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize

	// Configurar ordenamiento
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := -1 // desc por defecto
	if opts.SortDesc {
		sortOrder = -1
	}

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: sortBy, Value: sortOrder}})

	cursor, err := r.collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetStats con signature corregida - CORREGIDO
func (r *logRepository) GetStats(ctx context.Context, filter repository.LogSearchFilter) (*models.LogStats, error) {
	matchStage := bson.M{}

	// Aplicar filtros
	if filter.WorkflowID != nil {
		matchStage["workflow_id"] = *filter.WorkflowID
	}

	if filter.UserID != nil {
		matchStage["user_id"] = *filter.UserID
	}

	if filter.Status != nil {
		matchStage["status"] = *filter.Status
	}

	// Filtro de fecha
	if filter.StartDate != nil || filter.EndDate != nil {
		dateFilter := bson.M{}
		if filter.StartDate != nil {
			dateFilter["$gte"] = *filter.StartDate
		}
		if filter.EndDate != nil {
			dateFilter["$lte"] = *filter.EndDate
		}
		matchStage["created_at"] = dateFilter
	}

	pipeline := mongo.Pipeline{
		{{"$match", matchStage}},
		{{"$group", bson.M{
			"_id":               nil,
			"total_executions":  bson.M{"$sum": 1},
			"successful_runs":   bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", "completed"}}, 1, 0}}},
			"failed_runs":       bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$status", "failed"}}, 1, 0}}},
			"avg_execution_time": bson.M{"$avg": "$duration"},
			"total_execution_time": bson.M{"$sum": "$duration"},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &models.LogStats{
			TotalExecutions:      0,
			SuccessfulRuns:       0,
			FailedRuns:           0,
			AverageExecutionTime: 0,
		}, nil
	}

	result := results[0]
	stats := &models.LogStats{
		TotalExecutions:      getInt64(result, "total_executions"),
		SuccessfulRuns:       getInt64(result, "successful_runs"),
		FailedRuns:           getInt64(result, "failed_runs"),
		AverageExecutionTime: getFloat64(result, "avg_execution_time"),
		TotalExecutionTime:   getInt64(result, "total_execution_time"),
	}

	// Calcular tasa de éxito
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions) * 100
	}

	return stats, nil
}

// GetStatistics versión legacy - IMPLEMENTADO
func (r *logRepository) GetStatistics(ctx context.Context, filter map[string]interface{}) (map[string]interface{}, error) {
	matchStage := bson.M{}
	for k, v := range filter {
		matchStage[k] = v
	}

	pipeline := mongo.Pipeline{
		{{"$match", matchStage}},
		{{"$group", bson.M{
			"_id":            "$status",
			"count":          bson.M{"$sum": 1},
			"total_duration": bson.M{"$sum": "$duration"},
			"avg_duration":   bson.M{"$avg": "$duration"},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	statusStats := make(map[string]interface{})
	totalLogs := int64(0)

	for _, result := range results {
		status := result["_id"].(string)
		count := getInt32(result, "count")
		totalLogs += int64(count)

		statusStats[status] = map[string]interface{}{
			"count":          count,
			"avg_duration":   getFloat64(result, "avg_duration"),
			"total_duration": getInt64(result, "total_duration"),
		}
	}

	stats["by_status"] = statusStats
	stats["total_logs"] = totalLogs

	return stats, nil
}

// GetRecentLogs obtiene logs recientes - IMPLEMENTADO
func (r *logRepository) GetRecentLogs(ctx context.Context, hours int, limit int) ([]*models.WorkflowLog, error) {
	startTime := time.Now().Add(-time.Duration(hours) * time.Hour)
	
	filter := bson.M{
		"created_at": bson.M{"$gte": startTime},
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

// GetLogsByDateRange obtiene logs por rango de fechas - IMPLEMENTADO
func (r *logRepository) GetLogsByDateRange(ctx context.Context, startDate, endDate time.Time, page, pageSize int) ([]*models.WorkflowLog, int64, error) {
	filter := bson.M{
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}

	// Contar total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Paginación
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize

	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// DeleteOldLogs elimina logs antiguos - IMPLEMENTADO
func (r *logRepository) DeleteOldLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := r.collection.DeleteMany(
		ctx,
		bson.M{"created_at": bson.M{"$lt": olderThan}},
	)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// DeleteLogsByWorkflow elimina logs de un workflow específico - IMPLEMENTADO
func (r *logRepository) DeleteLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (int64, error) {
	result, err := r.collection.DeleteMany(
		ctx,
		bson.M{"workflow_id": workflowID},
	)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// Helper functions para conversion de tipos
func getInt64(m bson.M, key string) int64 {
	if val, exists := m[key]; exists && val != nil {
		switch v := val.(type) {
		case int64:
			return v
		case int32:
			return int64(v)
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getInt32(m bson.M, key string) int32 {
	if val, exists := m[key]; exists && val != nil {
		switch v := val.(type) {
		case int32:
			return v
		case int64:
			return int32(v)
		case int:
			return int32(v)
		case float64:
			return int32(v)
		}
	}
	return 0
}

func getFloat64(m bson.M, key string) float64 {
	if val, exists := m[key]; exists && val != nil {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int64:
			return float64(v)
		case int32:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return 0.0
}) * pageSize

	// Configurar ordenamiento
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := -1 // desc por defecto
	if opts.SortDesc {
		sortOrder = -1
	}

	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: sortBy, Value: sortOrder}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetByUserID con signature unificada - CORREGIDO
func (r *logRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]*models.WorkflowLog, int64, error) {
	filter := bson.M{"user_id": userID}

	// Contar total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Configurar paginación
	page := opts.Page
	pageSize := opts.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1