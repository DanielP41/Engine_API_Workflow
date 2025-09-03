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
	collection := db.Collection("logs")

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
	log.ID = primitive.NewObjectID()
	log.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, log)
	return err
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

func (r *logRepository) Update(ctx context.Context, id primitive.ObjectID, update map[string]interface{}) error {
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

// FindWithPagination busca logs con paginación
func (r *logRepository) FindWithPagination(ctx context.Context, filter map[string]interface{}, page, limit int) ([]models.WorkflowLog, int64, error) {
	// Calcular skip
	skip := (page - 1) * limit

	// Contar total de documentos
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Configurar opciones de búsqueda
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	// Ejecutar búsqueda
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	// Decodificar resultados
	var logs []models.WorkflowLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *logRepository) GetByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	filter := bson.M{"workflow_id": workflowID}
	return r.FindWithPagination(ctx, filter, opts.Page, opts.PageSize)
}

func (r *logRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	filter := bson.M{"user_id": userID}
	return r.FindWithPagination(ctx, filter, opts.Page, opts.PageSize)
}

func (r *logRepository) GetStats(ctx context.Context, filter repository.LogSearchFilter) (*models.LogStats, error) {
	// Construir filtro de búsqueda
	mongoFilter := bson.M{}

	if filter.WorkflowID != nil {
		mongoFilter["workflow_id"] = *filter.WorkflowID
	}

	if filter.UserID != nil {
		mongoFilter["user_id"] = *filter.UserID
	}

	if filter.Status != nil {
		mongoFilter["status"] = *filter.Status
	}

	if filter.StartDate != nil || filter.EndDate != nil {
		dateFilter := bson.M{}
		if filter.StartDate != nil {
			dateFilter["$gte"] = *filter.StartDate
		}
		if filter.EndDate != nil {
			dateFilter["$lte"] = *filter.EndDate
		}
		mongoFilter["created_at"] = dateFilter
	}

	// Ejecutar agregación para estadísticas
	pipeline := []bson.M{
		{"$match": mongoFilter},
		{"$group": bson.M{
			"_id":                    nil,
			"total_executions":       bson.M{"$sum": 1},
			"successful_runs":        bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []string{"$status", "completed"}}, 1, 0}}},
			"failed_runs":            bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []string{"$status", "failed"}}, 1, 0}}},
			"average_execution_time": bson.M{"$avg": "$duration"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil {
		return nil, err
	}

	stats := &models.LogStats{
		TotalExecutions:      0,
		SuccessfulRuns:       0,
		FailedRuns:           0,
		AverageExecutionTime: 0,
	}

	if len(result) > 0 {
		data := result[0]
		if total, ok := data["total_executions"].(int32); ok {
			stats.TotalExecutions = int64(total)
		}
		if successful, ok := data["successful_runs"].(int32); ok {
			stats.SuccessfulRuns = int64(successful)
		}
		if failed, ok := data["failed_runs"].(int32); ok {
			stats.FailedRuns = int64(failed)
		}
		if avgTime, ok := data["average_execution_time"].(float64); ok {
			stats.AverageExecutionTime = avgTime
		}
	}

	return stats, nil
}

func (r *logRepository) Search(ctx context.Context, filter repository.LogSearchFilter, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	// Convertir LogSearchFilter a filtro de MongoDB
	mongoFilter := bson.M{}

	if filter.WorkflowID != nil {
		mongoFilter["workflow_id"] = *filter.WorkflowID
	}

	if filter.UserID != nil {
		mongoFilter["user_id"] = *filter.UserID
	}

	if filter.ExecutionID != nil {
		mongoFilter["execution_id"] = *filter.ExecutionID
	}

	if filter.Status != nil {
		mongoFilter["status"] = *filter.Status
	}

	if filter.MessageContains != nil && *filter.MessageContains != "" {
		mongoFilter["message"] = bson.M{"$regex": *filter.MessageContains, "$options": "i"}
	}

	if filter.StartDate != nil || filter.EndDate != nil {
		dateFilter := bson.M{}
		if filter.StartDate != nil {
			dateFilter["$gte"] = *filter.StartDate
		}
		if filter.EndDate != nil {
			dateFilter["$lte"] = *filter.EndDate
		}
		mongoFilter["created_at"] = dateFilter
	}

	return r.FindWithPagination(ctx, mongoFilter, opts.Page, opts.PageSize)
}

// Implementar métodos restantes de la interfaz
func (r *logRepository) Query(ctx context.Context, req *models.LogQueryRequest) (*models.LogListResponse, error) {
	// Implementación básica
	filter := bson.M{}
	if req.WorkflowID != nil {
		filter["workflow_id"] = *req.WorkflowID
	}

	logs, total, err := r.FindWithPagination(ctx, filter, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	// Convertir a punteros
	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: int((total + int64(req.PageSize) - 1) / int64(req.PageSize)),
	}, nil
}

func (r *logRepository) List(ctx context.Context, page, pageSize int) (*models.LogListResponse, error) {
	logs, total, err := r.FindWithPagination(ctx, bson.M{}, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func (r *logRepository) ListByWorkflow(ctx context.Context, workflowID primitive.ObjectID, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{"workflow_id": workflowID}
	logs, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func (r *logRepository) ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{"user_id": userID}
	logs, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func (r *logRepository) ListByStatus(ctx context.Context, status models.ExecutionStatus, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{"status": string(status)}
	logs, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

// Métodos adicionales requeridos por la interfaz
func (r *logRepository) GetRecentLogs(ctx context.Context, hours int, limit int) ([]*models.WorkflowLog, error) {
	filter := bson.M{
		"created_at": bson.M{
			"$gte": time.Now().Add(-time.Duration(hours) * time.Hour),
		},
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
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetLogsByDateRange(ctx context.Context, startDate, endDate string, page, pageSize int) (*models.LogListResponse, error) {
	// Parsear fechas
	start, err := time.Parse(time.RFC3339, startDate)
	if err != nil {
		return nil, err
	}

	end, err := time.Parse(time.RFC3339, endDate)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"created_at": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	logs, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func (r *logRepository) GetStatistics(ctx context.Context) (*models.LogStatistics, error) {
	// Implementación básica de estadísticas
	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	return &models.LogStatistics{
		TotalExecutions: total,
		DataPeriod:      "all_time",
		LastUpdated:     time.Now(),
	}, nil
}

func (r *logRepository) GetStatisticsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (*models.LogStatistics, error) {
	filter := bson.M{"workflow_id": workflowID}
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &models.LogStatistics{
		TotalExecutions: total,
		DataPeriod:      "workflow_specific",
		LastUpdated:     time.Now(),
	}, nil
}

func (r *logRepository) GetStatisticsByUser(ctx context.Context, userID primitive.ObjectID) (*models.LogStatistics, error) {
	filter := bson.M{"user_id": userID}
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &models.LogStatistics{
		TotalExecutions: total,
		DataPeriod:      "user_specific",
		LastUpdated:     time.Now(),
	}, nil
}

func (r *logRepository) GetStatisticsByDateRange(ctx context.Context, startDate, endDate string) (*models.LogStatistics, error) {
	start, err := time.Parse(time.RFC3339, startDate)
	if err != nil {
		return nil, err
	}

	end, err := time.Parse(time.RFC3339, endDate)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"created_at": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &models.LogStatistics{
		TotalExecutions: total,
		DataPeriod:      "date_range",
		LastUpdated:     time.Now(),
	}, nil
}

func (r *logRepository) GetRunningExecutions(ctx context.Context) ([]*models.WorkflowLog, error) {
	filter := bson.M{"status": "running"}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetFailedExecutions(ctx context.Context, limit int) ([]*models.WorkflowLog, error) {
	filter := bson.M{"status": "failed"}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetLastExecution(ctx context.Context, workflowID primitive.ObjectID) (*models.WorkflowLog, error) {
	filter := bson.M{"workflow_id": workflowID}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})

	var log models.WorkflowLog
	err := r.collection.FindOne(ctx, filter, opts).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, repository.ErrLogNotFound
		}
		return nil, err
	}

	return &log, nil
}

func (r *logRepository) GetExecutionHistory(ctx context.Context, workflowID primitive.ObjectID, limit int) ([]*models.WorkflowLog, error) {
	filter := bson.M{"workflow_id": workflowID}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) DeleteOldLogs(ctx context.Context, cutoffDate time.Time) (int64, error) {
	filter := bson.M{"created_at": bson.M{"$lt": cutoffDate}}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

func (r *logRepository) DeleteLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (int64, error) {
	filter := bson.M{"workflow_id": workflowID}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

func (r *logRepository) GetExecutionCountByStatus(ctx context.Context) (map[models.ExecutionStatus]int64, error) {
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[models.ExecutionStatus]int64)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		result[models.ExecutionStatus(doc.ID)] = doc.Count
	}

	return result, nil
}

func (r *logRepository) GetExecutionCountByTriggerType(ctx context.Context) (map[models.TriggerType]int64, error) {
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$trigger_type",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[models.TriggerType]int64)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		result[models.TriggerType(doc.ID)] = doc.Count
	}

	return result, nil
}

func (r *logRepository) GetAverageExecutionDuration(ctx context.Context) (float64, error) {
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":     nil,
			"avgTime": bson.M{"$avg": "$duration"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var result struct {
			AvgTime float64 `bson:"avgTime"`
		}
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
		return result.AvgTime, nil
	}

	return 0, nil
}

func (r *logRepository) GetExecutionTrends(ctx context.Context, days int) (map[string]int64, error) {
	// Implementación básica
	startDate := time.Now().AddDate(0, 0, -days)
	filter := bson.M{"created_at": bson.M{"$gte": startDate}}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return map[string]int64{"total": count}, nil
}

func (r *logRepository) SearchLogs(ctx context.Context, searchTerm string, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"workflow_name": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"error_message": bson.M{"$regex": searchTerm, "$options": "i"}},
		},
	}

	logs, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func (r *logRepository) SearchLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID, searchTerm string, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{
		"workflow_id": workflowID,
		"$or": []bson.M{
			{"workflow_name": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"error_message": bson.M{"$regex": searchTerm, "$options": "i"}},
		},
	}

	logs, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	logPtrs := make([]*models.WorkflowLog, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}

	return &models.LogListResponse{
		Logs:       logPtrs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}
