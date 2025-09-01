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
	if log.ID.IsZero() {
		log.ID = primitive.NewObjectID()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	log.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("failed to create log: %w", err)
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
		return nil, fmt.Errorf("failed to get log by ID: %w", err)
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
		return nil, fmt.Errorf("failed to get log by execution ID: %w", err)
	}

	return &log, nil
}

func (r *logRepository) Update(ctx context.Context, id primitive.ObjectID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	updates["updated_at"] = time.Now()

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": updates},
	)
	if err != nil {
		return fmt.Errorf("failed to update log: %w", err)
	}

	if result.MatchedCount == 0 {
		return repository.ErrLogNotFound
	}

	return nil
}

func (r *logRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete log: %w", err)
	}

	if result.DeletedCount == 0 {
		return repository.ErrLogNotFound
	}

	return nil
}

// GetByWorkflowID obtiene logs por workflow ID con paginación
func (r *logRepository) GetByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	filter := bson.M{"workflow_id": workflowID}

	// Count total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// Build find options
	findOpts := options.Find()
	if opts.Page > 0 && opts.PageSize > 0 {
		skip := (opts.Page - 1) * opts.PageSize
		findOpts.SetSkip(int64(skip)).SetLimit(int64(opts.PageSize))
	}

	// Sorting
	sortField := "created_at"
	sortOrder := -1
	if opts.SortBy != "" {
		sortField = opts.SortBy
	}
	if !opts.SortDesc {
		sortOrder = 1
	}
	findOpts.SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, fmt.Errorf("failed to decode logs: %w", err)
	}

	return logs, total, nil
}

// GetByUserID obtiene logs por user ID con paginación
func (r *logRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	filter := bson.M{"user_id": userID}

	// Count total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// Build find options
	findOpts := options.Find()
	if opts.Page > 0 && opts.PageSize > 0 {
		skip := (opts.Page - 1) * opts.PageSize
		findOpts.SetSkip(int64(skip)).SetLimit(int64(opts.PageSize))
	}

	// Sorting
	sortField := "created_at"
	sortOrder := -1
	if opts.SortBy != "" {
		sortField = opts.SortBy
	}
	if !opts.SortDesc {
		sortOrder = 1
	}
	findOpts.SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, fmt.Errorf("failed to decode logs: %w", err)
	}

	return logs, total, nil
}

// GetStats obtiene estadísticas de logs
func (r *logRepository) GetStats(ctx context.Context, filter repository.LogSearchFilter) (*models.LogStats, error) {
	// Convert LogSearchFilter to MongoDB filter
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

	// Count totals
	totalExecutions, _ := r.collection.CountDocuments(ctx, mongoFilter)

	successFilter := bson.M{}
	for k, v := range mongoFilter {
		successFilter[k] = v
	}
	successFilter["status"] = "completed"
	successfulRuns, _ := r.collection.CountDocuments(ctx, successFilter)

	failedFilter := bson.M{}
	for k, v := range mongoFilter {
		failedFilter[k] = v
	}
	failedFilter["status"] = "failed"
	failedRuns, _ := r.collection.CountDocuments(ctx, failedFilter)

	// Calculate average execution time
	pipeline := []bson.M{
		{"$match": mongoFilter},
		{"$match": bson.M{"duration": bson.M{"$ne": nil}}},
		{"$group": bson.M{
			"_id":            nil,
			"avg_duration":   bson.M{"$avg": "$duration"},
			"total_duration": bson.M{"$sum": "$duration"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate stats: %w", err)
	}
	defer cursor.Close(ctx)

	var avgDuration float64
	var totalDuration int64
	if cursor.Next(ctx) {
		var result struct {
			AvgDuration   float64 `bson:"avg_duration"`
			TotalDuration int64   `bson:"total_duration"`
		}
		if err := cursor.Decode(&result); err == nil {
			avgDuration = result.AvgDuration
			totalDuration = result.TotalDuration
		}
	}

	// Calculate success rate
	var successRate float64
	if totalExecutions > 0 {
		successRate = float64(successfulRuns) / float64(totalExecutions) * 100
	}

	return &models.LogStats{
		TotalExecutions:      totalExecutions,
		SuccessfulRuns:       successfulRuns,
		FailedRuns:           failedRuns,
		AverageExecutionTime: avgDuration,
		TotalExecutionTime:   totalDuration,
		SuccessRate:          successRate,
		MostUsedTriggers:     []models.TriggerStats{},
		ErrorDistribution:    []models.ErrorStats{},
	}, nil
}

// Search busca logs con filtros
func (r *logRepository) Search(ctx context.Context, filter repository.LogSearchFilter, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	// Convert LogSearchFilter to MongoDB filter
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
	if filter.Level != nil {
		mongoFilter["level"] = *filter.Level
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
	if filter.MessageContains != nil && *filter.MessageContains != "" {
		mongoFilter["$or"] = []bson.M{
			{"workflow_name": bson.M{"$regex": *filter.MessageContains, "$options": "i"}},
			{"error_message": bson.M{"$regex": *filter.MessageContains, "$options": "i"}},
		}
	}

	// Count total
	total, err := r.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// Build find options
	findOpts := options.Find()
	if opts.Page > 0 && opts.PageSize > 0 {
		skip := (opts.Page - 1) * opts.PageSize
		findOpts.SetSkip(int64(skip)).SetLimit(int64(opts.PageSize))
	}

	// Sorting
	sortField := "created_at"
	sortOrder := -1
	if opts.SortBy != "" {
		sortField = opts.SortBy
	}
	if !opts.SortDesc {
		sortOrder = 1
	}
	findOpts.SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	// Execute query
	cursor, err := r.collection.Find(ctx, mongoFilter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, fmt.Errorf("failed to decode logs: %w", err)
	}

	return logs, total, nil
}

// Métodos adicionales requeridos por la interfaz LogRepository

func (r *logRepository) Query(ctx context.Context, req *models.LogQueryRequest) (*models.LogListResponse, error) {
	filter := bson.M{}

	// Build filter based on query request
	if req.WorkflowID != nil {
		filter["workflow_id"] = *req.WorkflowID
	}
	if req.UserID != nil {
		filter["user_id"] = *req.UserID
	}
	if req.Status != nil {
		filter["status"] = *req.Status
	}
	if req.TriggerType != nil {
		filter["trigger_type"] = *req.TriggerType
	}
	if req.StartDate != nil {
		if filter["created_at"] == nil {
			filter["created_at"] = bson.M{}
		}
		filter["created_at"].(bson.M)["$gte"] = *req.StartDate
	}
	if req.EndDate != nil {
		if filter["created_at"] == nil {
			filter["created_at"] = bson.M{}
		}
		filter["created_at"].(bson.M)["$lte"] = *req.EndDate
	}

	// Count total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count logs: %w", err)
	}

	// Build find options
	opts := options.Find()

	// Pagination
	if req.Page > 0 && req.PageSize > 0 {
		skip := (req.Page - 1) * req.PageSize
		opts.SetSkip(int64(skip)).SetLimit(int64(req.PageSize))
	}

	// Sorting
	sortField := "created_at"
	sortOrder := -1
	if req.SortBy != "" {
		sortField = req.SortBy
	}
	if req.SortOrder == "asc" {
		sortOrder = 1
	}
	opts.SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode logs: %w", err)
	}

	// Calculate pagination
	totalPages := int((total + int64(req.PageSize) - 1) / int64(req.PageSize))

	return &models.LogListResponse{
		Logs:       logs,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *logRepository) List(ctx context.Context, page, pageSize int) (*models.LogListResponse, error) {
	req := &models.LogQueryRequest{
		Page:     page,
		PageSize: pageSize,
	}
	return r.Query(ctx, req)
}

func (r *logRepository) ListByWorkflow(ctx context.Context, workflowID primitive.ObjectID, page, pageSize int) (*models.LogListResponse, error) {
	req := &models.LogQueryRequest{
		WorkflowID: &workflowID,
		Page:       page,
		PageSize:   pageSize,
	}
	return r.Query(ctx, req)
}

func (r *logRepository) ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.LogListResponse, error) {
	req := &models.LogQueryRequest{
		UserID:   &userID,
		Page:     page,
		PageSize: pageSize,
	}
	return r.Query(ctx, req)
}

func (r *logRepository) ListByStatus(ctx context.Context, status models.ExecutionStatus, page, pageSize int) (*models.LogListResponse, error) {
	statusPtr := &status
	req := &models.LogQueryRequest{
		Status:   statusPtr,
		Page:     page,
		PageSize: pageSize,
	}
	return r.Query(ctx, req)
}

func (r *logRepository) GetRecentLogs(ctx context.Context, hours int, limit int) ([]*models.WorkflowLog, error) {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	filter := bson.M{
		"created_at": bson.M{"$gte": cutoff},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode logs: %w", err)
	}

	return logs, nil
}

func (r *logRepository) GetLogsByDateRange(ctx context.Context, startDate, endDate string, page, pageSize int) (*models.LogListResponse, error) {
	var start, end *time.Time

	if startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			start = &t
		}
	}

	if endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			end = &t
		}
	}

	req := &models.LogQueryRequest{
		StartDate: start,
		EndDate:   end,
		Page:      page,
		PageSize:  pageSize,
	}

	return r.Query(ctx, req)
}

func (r *logRepository) GetStatistics(ctx context.Context) (*models.LogStatistics, error) {
	return r.getStatisticsWithFilter(ctx, bson.M{})
}

func (r *logRepository) GetStatisticsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (*models.LogStatistics, error) {
	filter := bson.M{"workflow_id": workflowID}
	return r.getStatisticsWithFilter(ctx, filter)
}

func (r *logRepository) GetStatisticsByUser(ctx context.Context, userID primitive.ObjectID) (*models.LogStatistics, error) {
	filter := bson.M{"user_id": userID}
	return r.getStatisticsWithFilter(ctx, filter)
}

func (r *logRepository) GetStatisticsByDateRange(ctx context.Context, startDate, endDate string) (*models.LogStatistics, error) {
	filter := bson.M{}

	if startDate != "" || endDate != "" {
		dateFilter := bson.M{}
		if startDate != "" {
			if t, err := time.Parse(time.RFC3339, startDate); err == nil {
				dateFilter["$gte"] = t
			}
		}
		if endDate != "" {
			if t, err := time.Parse(time.RFC3339, endDate); err == nil {
				dateFilter["$lte"] = t
			}
		}
		if len(dateFilter) > 0 {
			filter["created_at"] = dateFilter
		}
	}

	return r.getStatisticsWithFilter(ctx, filter)
}

func (r *logRepository) getStatisticsWithFilter(ctx context.Context, filter bson.M) (*models.LogStatistics, error) {
	// Count total executions
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count total executions: %w", err)
	}

	// Count by status
	successFilter := bson.M{}
	for k, v := range filter {
		successFilter[k] = v
	}
	successFilter["status"] = "completed"
	successful, _ := r.collection.CountDocuments(ctx, successFilter)

	failedFilter := bson.M{}
	for k, v := range filter {
		failedFilter[k] = v
	}
	failedFilter["status"] = "failed"
	failed, _ := r.collection.CountDocuments(ctx, failedFilter)

	// Calculate rates
	var successRate, failureRate float64
	if total > 0 {
		successRate = float64(successful) / float64(total) * 100
		failureRate = float64(failed) / float64(total) * 100
	}

	// Get today's executions
	today := time.Now().Truncate(24 * time.Hour)
	todayFilter := bson.M{}
	for k, v := range filter {
		todayFilter[k] = v
	}
	todayFilter["created_at"] = bson.M{"$gte": today}
	executionsToday, _ := r.collection.CountDocuments(ctx, todayFilter)

	// Get this week's executions
	thisWeek := today.AddDate(0, 0, -7)
	weekFilter := bson.M{}
	for k, v := range filter {
		weekFilter[k] = v
	}
	weekFilter["created_at"] = bson.M{"$gte": thisWeek}
	executionsThisWeek, _ := r.collection.CountDocuments(ctx, weekFilter)

	// Get this month's executions
	thisMonth := today.AddDate(0, 0, -30)
	monthFilter := bson.M{}
	for k, v := range filter {
		monthFilter[k] = v
	}
	monthFilter["created_at"] = bson.M{"$gte": thisMonth}
	executionsThisMonth, _ := r.collection.CountDocuments(ctx, monthFilter)

	return &models.LogStatistics{
		TotalExecutions:      total,
		SuccessfulRuns:       successful,
		FailedRuns:           failed,
		ExecutionsToday:      executionsToday,
		ExecutionsThisWeek:   executionsThisWeek,
		ExecutionsThisMonth:  executionsThisMonth,
		AverageExecutionTime: 0, // TODO: Calculate from duration field
		SuccessRate:          successRate,
		FailureRate:          failureRate,
		TriggerDistribution:  []models.TriggerDistribution{},
		ErrorDistribution:    []models.ErrorDistribution{},
		HourlyDistribution:   []models.HourlyStats{},
		LastUpdated:          time.Now(),
		DataPeriod:           "filtered",
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
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetFailedExecutions(ctx context.Context, limit int) ([]*models.WorkflowLog, error) {
	filter := bson.M{"status": "failed"}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

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

func (r *logRepository) GetLastExecution(ctx context.Context, workflowID primitive.ObjectID) (*models.WorkflowLog, error) {
	filter := bson.M{"workflow_id": workflowID}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})

	var log models.WorkflowLog
	err := r.collection.FindOne(ctx, filter, opts).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &log, nil
}

func (r *logRepository) GetExecutionHistory(ctx context.Context, workflowID primitive.ObjectID, limit int) ([]*models.WorkflowLog, error) {
	filter := bson.M{"workflow_id": workflowID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

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

func (r *logRepository) DeleteOldLogs(ctx context.Context, daysOld int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -daysOld)
	result, err := r.collection.DeleteMany(
		ctx,
		bson.M{"created_at": bson.M{"$lt": cutoff}},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %w", err)
	}

	return result.DeletedCount, nil
}

func (r *logRepository) DeleteLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (int64, error) {
	result, err := r.collection.DeleteMany(ctx, bson.M{"workflow_id": workflowID})
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
		{"$match": bson.M{"duration": bson.M{"$ne": nil}}},
		{"$group": bson.M{
			"_id":          nil,
			"avg_duration": bson.M{"$avg": "$duration"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var result struct {
			AvgDuration float64 `bson:"avg_duration"`
		}
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
		return result.AvgDuration, nil
	}

	return 0, nil
}

func (r *logRepository) GetExecutionTrends(ctx context.Context, days int) (map[string]int64, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	pipeline := []bson.M{
		{"$match": bson.M{
			"created_at": bson.M{"$gte": startDate},
		}},
		{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id": 1}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[string]int64)
	for cursor.Next(ctx) {
		var doc struct {
			ID struct {
				Year  int `bson:"year"`
				Month int `bson:"month"`
				Day   int `bson:"day"`
			} `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		dateKey := fmt.Sprintf("%04d-%02d-%02d", doc.ID.Year, doc.ID.Month, doc.ID.Day)
		result[dateKey] = doc.Count
	}

	return result, nil
}

func (r *logRepository) SearchLogs(ctx context.Context, searchTerm string, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"workflow_name": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"error_message": bson.M{"$regex": searchTerm, "$options": "i"}},
		},
	}

	return r.listWithFilter(ctx, filter, page, pageSize)
}

func (r *logRepository) SearchLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID, searchTerm string, page, pageSize int) (*models.LogListResponse, error) {
	filter := bson.M{
		"workflow_id": workflowID,
		"$or": []bson.M{
			{"workflow_name": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"error_message": bson.M{"$regex": searchTerm, "$options": "i"}},
		},
	}

	return r.listWithFilter(ctx, filter, page, pageSize)
}

func (r *logRepository) listWithFilter(ctx context.Context, filter bson.M, page, pageSize int) (*models.LogListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize

	// Count total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count logs: %w", err)
	}

	// Find documents
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.WorkflowLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode logs: %w", err)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.LogListResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
