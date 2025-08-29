// internal/repository/mongodb/log_repository.go
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

func (r *logRepository) Create(ctx context.Context, log *models.Log) error {
	log.ID = primitive.NewObjectID()
	log.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return err
	}

	return nil
}

func (r *logRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Log, error) {
	var log models.Log

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, repository.ErrLogNotFound
		}
		return nil, err
	}

	return &log, nil
}

func (r *logRepository) GetByExecutionID(ctx context.Context, executionID string) ([]*models.Log, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"execution_id": executionID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, limit int) ([]*models.Log, error) {
	options := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(
		ctx,
		bson.M{"workflow_id": workflowID},
		options,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit int) ([]*models.Log, error) {
	options := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(
		ctx,
		bson.M{"user_id": userID},
		options,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) Search(ctx context.Context, filter repository.LogSearchFilter) ([]*models.Log, error) {
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
		query["message"] = bson.M{"$regex": *filter.MessageContains, "$options": "i"}
	}

	// Build options
	findOptions := options.Find()

	// Sorting
	if filter.SortBy != nil {
		sortOrder := 1
		if filter.SortOrder != nil && *filter.SortOrder == "desc" {
			sortOrder = -1
		}
		findOptions.SetSort(bson.D{{Key: *filter.SortBy, Value: sortOrder}})
	} else {
		// Default sort by created_at desc
		findOptions.SetSort(bson.D{{Key: "created_at", Value: -1}})
	}

	// Pagination
	if filter.Page != nil && filter.Limit != nil {
		skip := (*filter.Page - 1) * *filter.Limit
		findOptions.SetSkip(int64(skip)).SetLimit(int64(*filter.Limit))
	} else if filter.Limit != nil {
		findOptions.SetLimit(int64(*filter.Limit))
	}

	cursor, err := r.collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) GetRecentByStatus(ctx context.Context, status string, limit int) ([]*models.Log, error) {
	options := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(
		ctx,
		bson.M{"status": status},
		options,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.Log
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *logRepository) Count(ctx context.Context, filter repository.LogSearchFilter) (int64, error) {
	query := bson.M{}

	// Build same query as Search method but for counting
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

	if filter.MessageContains != nil && *filter.MessageContains != "" {
		query["message"] = bson.M{"$regex": *filter.MessageContains, "$options": "i"}
	}

	return r.collection.CountDocuments(ctx, query)
}

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

func (r *logRepository) GetStats(ctx context.Context, workflowID *primitive.ObjectID, days int) (map[string]interface{}, error) {
	matchStage := bson.M{
		"created_at": bson.M{
			"$gte": time.Now().AddDate(0, 0, -days),
		},
	}

	if workflowID != nil {
		matchStage["workflow_id"] = *workflowID
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

	stats := make(map[string]interface{})
	var results []bson.M

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	statusStats := make(map[string]interface{})
	totalLogs := int64(0)

	for _, result := range results {
		status := result["_id"].(string)
		count := result["count"].(int32)
		totalLogs += int64(count)

		statusStats[status] = map[string]interface{}{
			"count":          count,
			"avg_duration":   result["avg_duration"],
			"total_duration": result["total_duration"],
		}
	}

	stats["by_status"] = statusStats
	stats["total_logs"] = totalLogs
	stats["period_days"] = days

	return stats, nil
}
