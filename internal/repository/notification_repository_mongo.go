package repository

import (
	"context"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// MongoNotificationRepository implementación MongoDB del repositorio de notificaciones
type MongoNotificationRepository struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

// NewMongoNotificationRepository crea una nueva instancia del repositorio
func NewMongoNotificationRepository(db *mongo.Database, logger *zap.Logger) *MongoNotificationRepository {
	collection := db.Collection("email_notifications")

	repo := &MongoNotificationRepository{
		collection: collection,
		logger:     logger,
	}

	// Crear índices
	repo.createIndexes()

	return repo
}

// createIndexes crea los índices necesarios para optimizar las consultas
func (r *MongoNotificationRepository) createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "priority", Value: 1},
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
				{Key: "workflow_id", Value: 1},
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
				{Key: "scheduled_at", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "next_retry_at", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "to", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "message_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "template_name", Value: 1},
			},
		},
		// Índice compuesto para obtener notificaciones para reintentar
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "attempts", Value: 1},
				{Key: "max_attempts", Value: 1},
				{Key: "next_retry_at", Value: 1},
			},
		},
		// Índice para limpieza de datos antiguos
		{
			Keys: bson.D{
				{Key: "created_at", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(90 * 24 * 60 * 60), // 90 días TTL
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		r.logger.Error("Failed to create indexes", zap.Error(err))
	} else {
		r.logger.Info("Notification repository indexes created successfully")
	}
}

// Create crea una nueva notificación
func (r *MongoNotificationRepository) Create(ctx context.Context, notification *models.EmailNotification) error {
	if notification.ID.IsZero() {
		notification.ID = primitive.NewObjectID()
	}

	notification.CreatedAt = time.Now()
	notification.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, notification)
	if err != nil {
		r.logger.Error("Failed to create notification",
			zap.String("id", notification.ID.Hex()),
			zap.Error(err))
		return fmt.Errorf("failed to create notification: %w", err)
	}

	r.logger.Debug("Notification created",
		zap.String("id", notification.ID.Hex()),
		zap.String("type", string(notification.Type)))

	return nil
}

// GetByID obtiene una notificación por ID
func (r *MongoNotificationRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailNotification, error) {
	var notification models.EmailNotification

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&notification)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, &RepositoryError{
				Code:    "NOTIFICATION_NOT_FOUND",
				Message: "notification not found",
			}
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	return &notification, nil
}

// Update actualiza una notificación existente
func (r *MongoNotificationRepository) Update(ctx context.Context, notification *models.EmailNotification) error {
	notification.UpdatedAt = time.Now()

	filter := bson.M{"_id": notification.ID}
	update := bson.M{"$set": notification}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.logger.Error("Failed to update notification",
			zap.String("id", notification.ID.Hex()),
			zap.Error(err))
		return fmt.Errorf("failed to update notification: %w", err)
	}

	if result.MatchedCount == 0 {
		return &RepositoryError{
			Code:    "NOTIFICATION_NOT_FOUND",
			Message: "notification not found",
		}
	}

	r.logger.Debug("Notification updated",
		zap.String("id", notification.ID.Hex()),
		zap.String("status", string(notification.Status)))

	return nil
}

// Delete elimina una notificación
func (r *MongoNotificationRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	if result.DeletedCount == 0 {
		return &RepositoryError{
			Code:    "NOTIFICATION_NOT_FOUND",
			Message: "notification not found",
		}
	}

	r.logger.Debug("Notification deleted", zap.String("id", id.Hex()))
	return nil
}

// List obtiene una lista de notificaciones con filtros y paginación
func (r *MongoNotificationRepository) List(ctx context.Context, filters map[string]interface{}, opts *PaginationOptions) ([]*models.EmailNotification, int64, error) {
	// Construir filtros MongoDB
	mongoFilter := r.buildMongoFilter(filters)

	// Configurar opciones de paginación
	findOptions := options.Find()
	if opts != nil {
		if opts.Limit > 0 {
			findOptions.SetLimit(int64(opts.Limit))
		}
		if opts.Offset > 0 {
			findOptions.SetSkip(int64(opts.Offset))
		}
		if opts.SortBy != "" {
			sortOrder := 1
			if opts.SortOrder == "desc" {
				sortOrder = -1
			}
			findOptions.SetSort(bson.D{{Key: opts.SortBy, Value: sortOrder}})
		} else {
			// Por defecto, ordenar por fecha de creación descendente
			findOptions.SetSort(bson.D{{Key: "created_at", Value: -1}})
		}
	}

	// Obtener el total de documentos
	total, err := r.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Ejecutar la consulta
	cursor, err := r.collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find notifications: %w", err)
	}
	defer cursor.Close(ctx)

	// Decodificar resultados
	var notifications []*models.EmailNotification
	if err := cursor.All(ctx, &notifications); err != nil {
		return nil, 0, fmt.Errorf("failed to decode notifications: %w", err)
	}

	return notifications, total, nil
}

// GetPending obtiene notificaciones pendientes
func (r *MongoNotificationRepository) GetPending(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	filter := bson.M{
		"status": models.NotificationStatusPending,
	}

	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "created_at", Value: 1}}) // Prioridad alta primero, luego FIFO

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending notifications: %w", err)
	}
	defer cursor.Close(ctx)

	var notifications []*models.EmailNotification
	if err := cursor.All(ctx, &notifications); err != nil {
		return nil, fmt.Errorf("failed to decode pending notifications: %w", err)
	}

	return notifications, nil
}

// GetScheduled obtiene notificaciones programadas que deben enviarse
func (r *MongoNotificationRepository) GetScheduled(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	filter := bson.M{
		"status": models.NotificationStatusScheduled,
		"scheduled_at": bson.M{
			"$lte": time.Now(),
		},
	}

	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "scheduled_at", Value: 1}}) // Más antigua primero

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled notifications: %w", err)
	}
	defer cursor.Close(ctx)

	var notifications []*models.EmailNotification
	if err := cursor.All(ctx, &notifications); err != nil {
		return nil, fmt.Errorf("failed to decode scheduled notifications: %w", err)
	}

	return notifications, nil
}

// GetFailedForRetry obtiene notificaciones fallidas que pueden reintentarse
func (r *MongoNotificationRepository) GetFailedForRetry(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	filter := bson.M{
		"status": models.NotificationStatusFailed,
		"$expr": bson.M{
			"$lt": []interface{}{"$attempts", "$max_attempts"},
		},
		"$or": []bson.M{
			{"next_retry_at": bson.M{"$lte": time.Now()}},
			{"next_retry_at": bson.M{"$exists": false}},
		},
	}

	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "next_retry_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed notifications for retry: %w", err)
	}
	defer cursor.Close(ctx)

	var notifications []*models.EmailNotification
	if err := cursor.All(ctx, &notifications); err != nil {
		return nil, fmt.Errorf("failed to decode failed notifications: %w", err)
	}

	return notifications, nil
}

// GetStats obtiene estadísticas de notificaciones
func (r *MongoNotificationRepository) GetStats(ctx context.Context, timeRange time.Duration) (*models.NotificationStats, error) {
	since := time.Now().Add(-timeRange)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": since},
			},
		},
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": 1},
				"by_status": bson.M{
					"$push": bson.M{
						"status": "$status",
						"count":  1,
					},
				},
				"by_type": bson.M{
					"$push": bson.M{
						"type":  "$type",
						"count": 1,
					},
				},
				"by_priority": bson.M{
					"$push": bson.M{
						"priority": "$priority",
						"count":    1,
					},
				},
				"total_attempts": bson.M{"$sum": "$attempts"},
				"sent_count": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							bson.M{"$eq": []interface{}{"$status", models.NotificationStatusSent}},
							1,
							0,
						},
					},
				},
				"last_processed": bson.M{"$max": "$updated_at"},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	if len(results) == 0 {
		return &models.NotificationStats{
			ByStatus:   make(map[models.NotificationStatus]int64),
			ByType:     make(map[models.NotificationType]int64),
			ByPriority: make(map[models.NotificationPriority]int64),
			Period:     timeRange,
		}, nil
	}

	result := results[0]

	stats := &models.NotificationStats{
		TotalNotifications: getInt64FromBson(result, "total"),
		ByStatus:           make(map[models.NotificationStatus]int64),
		ByType:             make(map[models.NotificationType]int64),
		ByPriority:         make(map[models.NotificationPriority]int64),
		Period:             timeRange,
	}

	// Calcular tasa de éxito
	sentCount := getInt64FromBson(result, "sent_count")
	if stats.TotalNotifications > 0 {
		stats.SuccessRate = float64(sentCount) / float64(stats.TotalNotifications)
	}

	// Calcular promedio de reintentos
	totalAttempts := getInt64FromBson(result, "total_attempts")
	if stats.TotalNotifications > 0 {
		stats.AverageRetries = float64(totalAttempts) / float64(stats.TotalNotifications)
	}

	// Último procesado
	if lastProcessed, ok := result["last_processed"].(primitive.DateTime); ok {
		t := lastProcessed.Time()
		stats.LastProcessed = &t
	}

	// Procesar conteos por categoría (esto requeriría una agregación más compleja)
	// Por simplicidad, hacemos consultas adicionales
	if err := r.fillStatsByCounts(ctx, since, stats); err != nil {
		r.logger.Warn("Failed to fill detailed stats", zap.Error(err))
	}

	return stats, nil
}

// CleanupOld elimina notificaciones antiguas
func (r *MongoNotificationRepository) CleanupOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	filter := bson.M{
		"created_at": bson.M{"$lt": cutoff},
		"status": bson.M{"$in": []models.NotificationStatus{
			models.NotificationStatusSent,
			models.NotificationStatusFailed,
			models.NotificationStatusCancelled,
		}},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old notifications: %w", err)
	}

	r.logger.Info("Cleaned up old notifications",
		zap.Int64("count", result.DeletedCount),
		zap.Duration("older_than", olderThan))

	return result.DeletedCount, nil
}

// buildMongoFilter construye un filtro MongoDB desde el mapa de filtros
func (r *MongoNotificationRepository) buildMongoFilter(filters map[string]interface{}) bson.M {
	mongoFilter := bson.M{}

	for key, value := range filters {
		switch key {
		case "status":
			if statuses, ok := value.([]models.NotificationStatus); ok {
				mongoFilter["status"] = bson.M{"$in": statuses}
			} else {
				mongoFilter["status"] = value
			}
		case "type":
			if types, ok := value.([]models.NotificationType); ok {
				mongoFilter["type"] = bson.M{"$in": types}
			} else {
				mongoFilter["type"] = value
			}
		case "priority":
			if priorities, ok := value.([]models.NotificationPriority); ok {
				mongoFilter["priority"] = bson.M{"$in": priorities}
			} else {
				mongoFilter["priority"] = value
			}
		case "user_id":
			mongoFilter["user_id"] = value
		case "workflow_id":
			mongoFilter["workflow_id"] = value
		case "execution_id":
			mongoFilter["execution_id"] = value
		case "created_after":
			if mongoFilter["created_at"] == nil {
				mongoFilter["created_at"] = bson.M{}
			}
			mongoFilter["created_at"].(bson.M)["$gte"] = value
		case "created_before":
			if mongoFilter["created_at"] == nil {
				mongoFilter["created_at"] = bson.M{}
			}
			mongoFilter["created_at"].(bson.M)["$lte"] = value
		case "sent_after":
			if mongoFilter["sent_at"] == nil {
				mongoFilter["sent_at"] = bson.M{}
			}
			mongoFilter["sent_at"].(bson.M)["$gte"] = value
		case "sent_before":
			if mongoFilter["sent_at"] == nil {
				mongoFilter["sent_at"] = bson.M{}
			}
			mongoFilter["sent_at"].(bson.M)["$lte"] = value
		case "to_email":
			mongoFilter["to"] = bson.M{"$in": []string{value.(string)}}
		case "subject_like":
			mongoFilter["subject"] = bson.M{"$regex": value, "$options": "i"}
		case "has_errors":
			if value.(bool) {
				mongoFilter["errors"] = bson.M{"$exists": true, "$ne": []interface{}{}}
			} else {
				mongoFilter["$or"] = []bson.M{
					{"errors": bson.M{"$exists": false}},
					{"errors": []interface{}{}},
				}
			}
		case "is_scheduled":
			if value.(bool) {
				mongoFilter["scheduled_at"] = bson.M{"$exists": true}
			} else {
				mongoFilter["scheduled_at"] = bson.M{"$exists": false}
			}
		case "can_retry":
			if value.(bool) {
				mongoFilter["$expr"] = bson.M{
					"$and": []bson.M{
						{"$eq": []interface{}{"$status", models.NotificationStatusFailed}},
						{"$lt": []interface{}{"$attempts", "$max_attempts"}},
					},
				}
			}
		case "message_id":
			mongoFilter["message_id"] = value
		case "template_name":
			mongoFilter["template_name"] = value
		default:
			// Filtro genérico
			mongoFilter[key] = value
		}
	}

	return mongoFilter
}

// fillStatsByCounts llena las estadísticas detalladas por categoría
func (r *MongoNotificationRepository) fillStatsByCounts(ctx context.Context, since time.Time, stats *models.NotificationStats) error {
	// Por status
	statusPipeline := []bson.M{
		{"$match": bson.M{"created_at": bson.M{"$gte": since}}},
		{"$group": bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, statusPipeline)
	if err == nil {
		var statusResults []bson.M
		if err := cursor.All(ctx, &statusResults); err == nil {
			for _, result := range statusResults {
				if status, ok := result["_id"].(string); ok {
					count := getInt64FromBson(result, "count")
					stats.ByStatus[models.NotificationStatus(status)] = count
				}
			}
		}
		cursor.Close(ctx)
	}

	// Por type
	typePipeline := []bson.M{
		{"$match": bson.M{"created_at": bson.M{"$gte": since}}},
		{"$group": bson.M{
			"_id":   "$type",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err = r.collection.Aggregate(ctx, typePipeline)
	if err == nil {
		var typeResults []bson.M
		if err := cursor.All(ctx, &typeResults); err == nil {
			for _, result := range typeResults {
				if notifType, ok := result["_id"].(string); ok {
					count := getInt64FromBson(result, "count")
					stats.ByType[models.NotificationType(notifType)] = count
				}
			}
		}
		cursor.Close(ctx)
	}

	// Por priority
	priorityPipeline := []bson.M{
		{"$match": bson.M{"created_at": bson.M{"$gte": since}}},
		{"$group": bson.M{
			"_id":   "$priority",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err = r.collection.Aggregate(ctx, priorityPipeline)
	if err == nil {
		var priorityResults []bson.M
		if err := cursor.All(ctx, &priorityResults); err == nil {
			for _, result := range priorityResults {
				if priority, ok := result["_id"].(string); ok {
					count := getInt64FromBson(result, "count")
					stats.ByPriority[models.NotificationPriority(priority)] = count
				}
			}
		}
		cursor.Close(ctx)
	}

	return nil
}

// Helper function para extraer int64 de BSON
func getInt64FromBson(doc bson.M, key string) int64 {
	if value, ok := doc[key]; ok {
		switch v := value.(type) {
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
