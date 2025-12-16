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

// ================================
// IMPLEMENTACIÓN DEL REPOSITORIO
// ================================

// MongoNotificationRepository implementación MongoDB del repositorio de notificaciones
type MongoNotificationRepository struct {
	collection *mongo.Collection
	logger     *zap.Logger
	dbName     string
}

// NewMongoNotificationRepository crea una nueva instancia del repositorio
func NewMongoNotificationRepository(db *mongo.Database, logger *zap.Logger) NotificationRepository {
	collection := db.Collection("email_notifications")

	repo := &MongoNotificationRepository{
		collection: collection,
		logger:     logger,
		dbName:     db.Name(),
	}

	// Crear índices en background
	go repo.createIndexes()

	return repo
}

// ================================
// CRUD BÁSICO
// ================================

// Create crea una nueva notificación
func (r *MongoNotificationRepository) Create(ctx context.Context, notification *models.EmailNotification) error {
	if notification.ID.IsZero() {
		notification.ID = primitive.NewObjectID()
	}

	notification.CreatedAt = time.Now()
	notification.UpdatedAt = time.Now()

	// Validar la notificación antes de crear
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.collection.InsertOne(ctx, notification)
	if err != nil {
		r.logger.Error("Failed to create notification",
			zap.String("id", notification.ID.Hex()),
			zap.String("type", string(notification.Type)),
			zap.Error(err))
		return fmt.Errorf("failed to create notification: %w", err)
	}

	r.logger.Debug("Notification created successfully",
		zap.String("id", notification.ID.Hex()),
		zap.String("type", string(notification.Type)),
		zap.String("status", string(notification.Status)),
		zap.Int("recipients", len(notification.To)))

	return nil
}

// GetByID obtiene una notificación por ID
func (r *MongoNotificationRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailNotification, error) {
	var notification models.EmailNotification

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&notification)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotificationNotFound
		}
		r.logger.Error("Failed to get notification by ID",
			zap.String("id", id.Hex()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	return &notification, nil
}

// Update actualiza una notificación existente
func (r *MongoNotificationRepository) Update(ctx context.Context, notification *models.EmailNotification) error {
	notification.UpdatedAt = time.Now()

	// Validar la notificación antes de actualizar
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

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
		return ErrNotificationNotFound
	}

	r.logger.Debug("Notification updated successfully",
		zap.String("id", notification.ID.Hex()),
		zap.String("status", string(notification.Status)))

	return nil
}

// Delete elimina una notificación
func (r *MongoNotificationRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		r.logger.Error("Failed to delete notification",
			zap.String("id", id.Hex()),
			zap.Error(err))
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrNotificationNotFound
	}

	r.logger.Debug("Notification deleted successfully",
		zap.String("id", id.Hex()))

	return nil
}

// ================================
// LISTADO Y BÚSQUEDA
// ================================

// List obtiene una lista de notificaciones con filtros y paginación
func (r *MongoNotificationRepository) List(ctx context.Context, filters map[string]interface{}, opts *PaginationOptions) ([]*models.EmailNotification, int64, error) {
	// Construir filtros MongoDB
	mongoFilter := r.buildMongoFilter(filters)

	// Configurar opciones de paginación
	findOpts := options.Find()
	if opts != nil {
		if opts.PageSize > 0 {
			findOpts.SetLimit(int64(opts.PageSize))
		}
		if opts.Page > 1 {
			skip := (opts.Page - 1) * opts.PageSize
			findOpts.SetSkip(int64(skip))
		}

		// Configurar ordenamiento
		if opts.SortBy != "" {
			sortOrder := 1
			if opts.SortDesc {
				sortOrder = -1
			}
			findOpts.SetSort(bson.D{{Key: opts.SortBy, Value: sortOrder}})
		} else {
			// Ordenamiento por defecto: más recientes primero
			findOpts.SetSort(bson.D{{Key: "created_at", Value: -1}})
		}
	} else {
		// Configuración por defecto
		findOpts.SetLimit(50).SetSort(bson.D{{Key: "created_at", Value: -1}})
	}

	// Ejecutar consulta
	cursor, err := r.collection.Find(ctx, mongoFilter, findOpts)
	if err != nil {
		r.logger.Error("Failed to list notifications",
			zap.Any("filters", filters),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list notifications: %w", err)
	}
	defer cursor.Close(ctx)

	// Decodificar resultados
	var notifications []*models.EmailNotification
	for cursor.Next(ctx) {
		var notification models.EmailNotification
		if err := cursor.Decode(&notification); err != nil {
			r.logger.Error("Failed to decode notification", zap.Error(err))
			continue
		}
		notifications = append(notifications, &notification)
	}

	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("cursor error: %w", err)
	}

	// Contar total de documentos
	total, err := r.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		r.logger.Error("Failed to count notifications", zap.Error(err))
		return notifications, 0, nil // Devolver resultados sin total
	}

	r.logger.Debug("Notifications listed successfully",
		zap.Int("count", len(notifications)),
		zap.Int64("total", total))

	return notifications, total, nil
}

// ================================
// PROCESAMIENTO DE NOTIFICACIONES
// ================================

// GetPending obtiene notificaciones pendientes para procesar
func (r *MongoNotificationRepository) GetPending(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	filter := bson.M{
		"status": models.NotificationStatusPending,
		"$or": []bson.M{
			{"scheduled_at": bson.M{"$exists": false}},
			{"scheduled_at": nil},
			{"scheduled_at": bson.M{"$lte": time.Now()}},
		},
	}

	return r.findNotifications(ctx, filter, limit, "created_at")
}

// GetScheduled obtiene notificaciones programadas que deben procesarse
func (r *MongoNotificationRepository) GetScheduled(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	filter := bson.M{
		"status":       models.NotificationStatusScheduled,
		"scheduled_at": bson.M{"$lte": time.Now()},
	}

	return r.findNotifications(ctx, filter, limit, "scheduled_at")
}

// GetFailedForRetry obtiene notificaciones fallidas que pueden reintentarse
func (r *MongoNotificationRepository) GetFailedForRetry(ctx context.Context, limit int) ([]*models.EmailNotification, error) {
	now := time.Now()
	filter := bson.M{
		"status": models.NotificationStatusFailed,
		"$expr":  bson.M{"$lt": []interface{}{"$attempts", "$max_attempts"}},
		"$or": []bson.M{
			{"next_retry_at": bson.M{"$exists": false}},
			{"next_retry_at": nil},
			{"next_retry_at": bson.M{"$lte": now}},
		},
	}

	return r.findNotifications(ctx, filter, limit, "next_retry_at")
}

// ================================
// ESTADÍSTICAS Y MÉTRICAS
// ================================

// GetStats obtiene estadísticas de notificaciones para un rango de tiempo
func (r *MongoNotificationRepository) GetStats(ctx context.Context, timeRange time.Duration) (*models.NotificationStats, error) {
	since := time.Now().Add(-timeRange)

	// Pipeline de agregación para estadísticas
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": since},
			},
		},
		{
			"$group": bson.M{
				"_id":                 nil,
				"total_notifications": bson.M{"$sum": 1},
				"by_status": bson.M{
					"$push": bson.M{
						"status":   "$status",
						"attempts": "$attempts",
					},
				},
				"by_type": bson.M{
					"$push": "$type",
				},
				"by_priority": bson.M{
					"$push": "$priority",
				},
				"total_attempts": bson.M{"$sum": "$attempts"},
				"last_processed": bson.M{"$max": "$sent_at"},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer cursor.Close(ctx)

	var result struct {
		TotalNotifications int64                         `bson:"total_notifications"`
		ByStatus           []map[string]interface{}      `bson:"by_status"`
		ByType             []models.NotificationType     `bson:"by_type"`
		ByPriority         []models.NotificationPriority `bson:"by_priority"`
		TotalAttempts      int64                         `bson:"total_attempts"`
		LastProcessed      *time.Time                    `bson:"last_processed"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode stats: %w", err)
		}
	}

	// Construir estadísticas
	stats := &models.NotificationStats{
		TotalNotifications: result.TotalNotifications,
		ByStatus:           make(map[models.NotificationStatus]int64),
		ByType:             make(map[models.NotificationType]int64),
		ByPriority:         make(map[models.NotificationPriority]int64),
		LastProcessed:      result.LastProcessed,
		Period:             timeRange,
	}

	// Procesar estadísticas por estado
	var successCount int64
	for _, statusInfo := range result.ByStatus {
		status := statusInfo["status"].(string)
		notifStatus := models.NotificationStatus(status)
		stats.ByStatus[notifStatus]++

		if notifStatus == models.NotificationStatusSent {
			successCount++
		}
	}

	// Calcular tasa de éxito
	if stats.TotalNotifications > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalNotifications)
	}

	// Calcular promedio de reintentos
	if stats.TotalNotifications > 0 {
		stats.AverageRetries = float64(result.TotalAttempts) / float64(stats.TotalNotifications)
	}

	// Procesar estadísticas por tipo
	for _, notifType := range result.ByType {
		stats.ByType[notifType]++
	}

	// Procesar estadísticas por prioridad
	for _, priority := range result.ByPriority {
		stats.ByPriority[priority]++
	}

	r.logger.Debug("Stats generated successfully",
		zap.Int64("total", stats.TotalNotifications),
		zap.Float64("success_rate", stats.SuccessRate),
		zap.Duration("time_range", timeRange))

	return stats, nil
}

// ================================
// MANTENIMIENTO
// ================================

// CleanupOld elimina notificaciones antiguas
func (r *MongoNotificationRepository) CleanupOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	filter := bson.M{
		"created_at": bson.M{"$lt": cutoff},
		"status": bson.M{
			"$in": []models.NotificationStatus{
				models.NotificationStatusSent,
				models.NotificationStatusCancelled,
			},
		},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		r.logger.Error("Failed to cleanup old notifications",
			zap.Duration("older_than", olderThan),
			zap.Error(err))
		return 0, fmt.Errorf("failed to cleanup old notifications: %w", err)
	}

	r.logger.Info("Old notifications cleaned up",
		zap.Int64("deleted_count", result.DeletedCount),
		zap.Duration("older_than", olderThan))

	return result.DeletedCount, nil
}

// ================================
// MÉTODOS DE BÚSQUEDA AVANZADA
// ================================

// FindByMessageID busca una notificación por Message-ID
func (r *MongoNotificationRepository) FindByMessageID(ctx context.Context, messageID string) (*models.EmailNotification, error) {
	var notification models.EmailNotification

	filter := bson.M{"message_id": messageID}
	err := r.collection.FindOne(ctx, filter).Decode(&notification)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("failed to find notification by message ID: %w", err)
	}

	return &notification, nil
}

// FindByUserID obtiene notificaciones de un usuario específico
func (r *MongoNotificationRepository) FindByUserID(ctx context.Context, userID primitive.ObjectID, opts *PaginationOptions) ([]*models.EmailNotification, int64, error) {
	filters := map[string]interface{}{
		"user_id": userID,
	}
	return r.List(ctx, filters, opts)
}

// FindByWorkflowID obtiene notificaciones de un workflow específico
func (r *MongoNotificationRepository) FindByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, opts *PaginationOptions) ([]*models.EmailNotification, int64, error) {
	filters := map[string]interface{}{
		"workflow_id": workflowID,
	}
	return r.List(ctx, filters, opts)
}

// FindByExecutionID obtiene notificaciones de una ejecución específica
func (r *MongoNotificationRepository) FindByExecutionID(ctx context.Context, executionID string, opts *PaginationOptions) ([]*models.EmailNotification, int64, error) {
	filters := map[string]interface{}{
		"execution_id": executionID,
	}
	return r.List(ctx, filters, opts)
}

// ================================
// MÉTODOS DE ACTUALIZACIÓN ATÓMICA
// ================================

// UpdateStatus actualiza el estado de una notificación de forma atómica
func (r *MongoNotificationRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.NotificationStatus) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if status == models.NotificationStatusSent {
		update["$set"].(bson.M)["sent_at"] = time.Now()
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

// IncrementAttempts incrementa el contador de intentos de forma atómica
func (r *MongoNotificationRepository) IncrementAttempts(ctx context.Context, id primitive.ObjectID, nextRetryAt *time.Time) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$inc": bson.M{"attempts": 1},
		"$set": bson.M{
			"last_attempt_at": time.Now(),
			"updated_at":      time.Now(),
		},
	}

	if nextRetryAt != nil {
		update["$set"].(bson.M)["next_retry_at"] = *nextRetryAt
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to increment attempts: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

// AddError añade un error a una notificación de forma atómica
func (r *MongoNotificationRepository) AddError(ctx context.Context, id primitive.ObjectID, notifError models.NotificationError) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$push": bson.M{"errors": notifError},
		"$set":  bson.M{"updated_at": time.Now()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to add error: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

// ================================
// MÉTODOS INTERNOS Y UTILIDADES
// ================================

// createIndexes crea los índices necesarios para optimizar consultas
func (r *MongoNotificationRepository) createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		// Índice compuesto para listado por estado y fecha
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("status_created_at"),
		},
		// Índice para notificaciones programadas
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "scheduled_at", Value: 1},
			},
			Options: options.Index().SetName("status_scheduled_at"),
		},
		// Índice para reintentos
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "attempts", Value: 1},
				{Key: "max_attempts", Value: 1},
				{Key: "next_retry_at", Value: 1},
			},
			Options: options.Index().SetName("retry_index"),
		},
		// Índices para filtros comunes
		{
			Keys:    bson.D{{Key: "type", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("type_created_at"),
		},
		{
			Keys:    bson.D{{Key: "priority", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("priority_created_at"),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("user_id_created_at"),
		},
		{
			Keys:    bson.D{{Key: "workflow_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("workflow_id_created_at"),
		},
		// Índices únicos
		{
			Keys:    bson.D{{Key: "message_id", Value: 1}},
			Options: options.Index().SetName("message_id_unique").SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "execution_id", Value: 1}},
			Options: options.Index().SetName("execution_id"),
		},
		// Índice para destinatarios
		{
			Keys:    bson.D{{Key: "to", Value: 1}},
			Options: options.Index().SetName("recipients"),
		},
		// Índice para templates
		{
			Keys:    bson.D{{Key: "template_name", Value: 1}},
			Options: options.Index().SetName("template_name"),
		},
		// Índice compuesto para template_name + created_at (optimiza agregaciones por fecha)
		{
			Keys:    bson.D{{Key: "template_name", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("template_name_created_at"),
		},
		// Índice compuesto para template_name + status (optimiza conteos por estado)
		{
			Keys:    bson.D{{Key: "template_name", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetName("template_name_status"),
		},
		// TTL index para limpieza automática (opcional)
		{
			Keys: bson.D{{Key: "created_at", Value: 1}},
			Options: options.Index().
				SetName("ttl_cleanup").
				SetExpireAfterSeconds(90 * 24 * 60 * 60), // 90 días
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		r.logger.Error("Failed to create notification indexes", zap.Error(err))
	} else {
		r.logger.Info("Notification repository indexes created successfully",
			zap.String("collection", r.collection.Name()),
			zap.String("database", r.dbName))
	}
}

// findNotifications helper para buscar notificaciones con filtro y límite
func (r *MongoNotificationRepository) findNotifications(ctx context.Context, filter bson.M, limit int, sortField string) ([]*models.EmailNotification, error) {
	findOpts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: sortField, Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find notifications: %w", err)
	}
	defer cursor.Close(ctx)

	var notifications []*models.EmailNotification
	for cursor.Next(ctx) {
		var notification models.EmailNotification
		if err := cursor.Decode(&notification); err != nil {
			r.logger.Error("Failed to decode notification", zap.Error(err))
			continue
		}
		notifications = append(notifications, &notification)
	}

	return notifications, cursor.Err()
}

// buildMongoFilter construye un filtro MongoDB a partir de filtros genéricos
func (r *MongoNotificationRepository) buildMongoFilter(filters map[string]interface{}) bson.M {
	mongoFilter := bson.M{}

	for key, value := range filters {
		switch key {
		case "status":
			if statuses, ok := value.([]models.NotificationStatus); ok {
				mongoFilter["status"] = bson.M{"$in": statuses}
			} else if status, ok := value.(models.NotificationStatus); ok {
				mongoFilter["status"] = status
			} else if statusStr, ok := value.(string); ok {
				mongoFilter["status"] = statusStr
			}

		case "type":
			if types, ok := value.([]models.NotificationType); ok {
				mongoFilter["type"] = bson.M{"$in": types}
			} else if notifType, ok := value.(models.NotificationType); ok {
				mongoFilter["type"] = notifType
			} else if typeStr, ok := value.(string); ok {
				mongoFilter["type"] = typeStr
			}

		case "priority":
			if priorities, ok := value.([]models.NotificationPriority); ok {
				mongoFilter["priority"] = bson.M{"$in": priorities}
			} else if priority, ok := value.(models.NotificationPriority); ok {
				mongoFilter["priority"] = priority
			} else if priorityStr, ok := value.(string); ok {
				mongoFilter["priority"] = priorityStr
			}

		case "user_id":
			if userID, ok := value.(primitive.ObjectID); ok {
				mongoFilter["user_id"] = userID
			} else if userIDStr, ok := value.(string); ok {
				if objID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
					mongoFilter["user_id"] = objID
				}
			}

		case "workflow_id":
			if workflowID, ok := value.(primitive.ObjectID); ok {
				mongoFilter["workflow_id"] = workflowID
			} else if workflowIDStr, ok := value.(string); ok {
				if objID, err := primitive.ObjectIDFromHex(workflowIDStr); err == nil {
					mongoFilter["workflow_id"] = objID
				}
			}

		case "execution_id":
			mongoFilter["execution_id"] = value

		case "created_after":
			if date, ok := value.(time.Time); ok {
				if existingCreated, exists := mongoFilter["created_at"]; exists {
					if createdFilter, ok := existingCreated.(bson.M); ok {
						createdFilter["$gte"] = date
					}
				} else {
					mongoFilter["created_at"] = bson.M{"$gte": date}
				}
			}

		case "created_before":
			if date, ok := value.(time.Time); ok {
				if existingCreated, exists := mongoFilter["created_at"]; exists {
					if createdFilter, ok := existingCreated.(bson.M); ok {
						createdFilter["$lte"] = date
					}
				} else {
					mongoFilter["created_at"] = bson.M{"$lte": date}
				}
			}

		case "sent_after":
			if date, ok := value.(time.Time); ok {
				if existingSent, exists := mongoFilter["sent_at"]; exists {
					if sentFilter, ok := existingSent.(bson.M); ok {
						sentFilter["$gte"] = date
					}
				} else {
					mongoFilter["sent_at"] = bson.M{"$gte": date}
				}
			}

		case "sent_before":
			if date, ok := value.(time.Time); ok {
				if existingSent, exists := mongoFilter["sent_at"]; exists {
					if sentFilter, ok := existingSent.(bson.M); ok {
						sentFilter["$lte"] = date
					}
				} else {
					mongoFilter["sent_at"] = bson.M{"$lte": date}
				}
			}

		case "to_email":
			if email, ok := value.(string); ok {
				mongoFilter["to"] = bson.M{"$in": []string{email}}
			}

		case "subject_like":
			if subject, ok := value.(string); ok {
				mongoFilter["subject"] = bson.M{"$regex": subject, "$options": "i"}
			}

		case "template_name":
			mongoFilter["template_name"] = value

		case "message_id":
			mongoFilter["message_id"] = value

		case "has_errors":
			if hasErrors, ok := value.(bool); ok {
				if hasErrors {
					mongoFilter["errors"] = bson.M{"$exists": true, "$ne": []interface{}{}}
				} else {
					mongoFilter["$or"] = []bson.M{
						{"errors": bson.M{"$exists": false}},
						{"errors": []interface{}{}},
					}
				}
			}

		case "is_scheduled":
			if isScheduled, ok := value.(bool); ok {
				if isScheduled {
					mongoFilter["scheduled_at"] = bson.M{"$exists": true, "$ne": nil}
				} else {
					mongoFilter["$or"] = []bson.M{
						{"scheduled_at": bson.M{"$exists": false}},
						{"scheduled_at": nil},
					}
				}
			}

		case "can_retry":
			if canRetry, ok := value.(bool); ok {
				if canRetry {
					mongoFilter["status"] = models.NotificationStatusFailed
					mongoFilter["$expr"] = bson.M{"$lt": []interface{}{"$attempts", "$max_attempts"}}
				}
			}

		default:
			// Para campos no reconocidos, usar tal como están
			mongoFilter[key] = value
		}
	}

	return mongoFilter
}

// ================================
// MÉTODOS DE VALIDACIÓN Y UTILIDAD
// ================================

// ValidateConnection verifica que la conexión a la base de datos esté activa
func (r *MongoNotificationRepository) ValidateConnection(ctx context.Context) error {
	return r.collection.Database().Client().Ping(ctx, nil)
}

// GetCollectionStats obtiene estadísticas de la colección
func (r *MongoNotificationRepository) GetCollectionStats(ctx context.Context) (*CollectionStats, error) {
	pipeline := []bson.M{
		{"$collStats": bson.M{"storageStats": bson.M{}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection stats: %w", err)
	}
	defer cursor.Close(ctx)

	var result struct {
		StorageStats struct {
			Size       int64 `bson:"size"`
			Count      int64 `bson:"count"`
			IndexCount int   `bson:"nindexes"`
		} `bson:"storageStats"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode stats: %w", err)
		}
	}

	stats := &CollectionStats{
		DocumentCount:  result.StorageStats.Count,
		StorageSize:    result.StorageStats.Size,
		IndexCount:     result.StorageStats.IndexCount,
		CollectionName: r.collection.Name(),
	}

	return stats, nil
}

// CollectionStats estadísticas de la colección
type CollectionStats struct {
	DocumentCount  int64  `json:"document_count"`
	StorageSize    int64  `json:"storage_size"`
	IndexCount     int    `json:"index_count"`
	CollectionName string `json:"collection_name"`
}
