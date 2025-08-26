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

type workflowRepository struct {
	collection *mongo.Collection
	db         *mongo.Database
}

// NewWorkflowRepository crea una nueva instancia del repositorio de workflows
func NewWorkflowRepository(db *mongo.Database) repository.WorkflowRepository {
	collection := db.Collection("workflows")

	repo := &workflowRepository{
		collection: collection,
		db:         db,
	}

	// Inicializar índices
	repo.createIndexes()

	return repo
}

// createIndexes crea los índices necesarios para optimizar las consultas
func (r *workflowRepository) createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_user_created"),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_status_created"),
		},
		{
			Keys: bson.D{
				{Key: "tags", Value: 1},
			},
			Options: options.Index().SetName("idx_tags"),
		},
		{
			Keys: bson.D{
				{Key: "name", Value: "text"},
				{Key: "description", Value: "text"},
			},
			Options: options.Index().SetName("idx_text_search"),
		},
		{
			Keys: bson.D{
				{Key: "is_active", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("idx_active_status"),
		},
		{
			Keys: bson.D{
				{Key: "version", Value: -1},
				{Key: "parent_workflow_id", Value: 1},
			},
			Options: options.Index().SetName("idx_version_parent"),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		fmt.Printf("Error creating indexes for workflows collection: %v\n", err)
	}
}

// Create crea un nuevo workflow
func (r *workflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	if workflow == nil {
		return fmt.Errorf("workflow cannot be nil")
	}

	// Validaciones básicas
	if workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if workflow.UserID.IsZero() {
		return fmt.Errorf("user_id is required")
	}
	if len(workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	// Establecer valores por defecto
	now := time.Now()
	workflow.CreatedAt = now
	workflow.UpdatedAt = now
	workflow.ID = primitive.NewObjectID()

	if workflow.Status == "" {
		workflow.Status = "draft"
	}
	if workflow.Version == 0 {
		workflow.Version = 1
	}
	if workflow.IsActive == nil {
		active := true
		workflow.IsActive = &active
	}

	// Inicializar estadísticas si no existen
	if workflow.Stats == nil {
		workflow.Stats = &models.WorkflowStats{
			TotalExecutions:      0,
			SuccessfulExecutions: 0,
			FailedExecutions:     0,
			AverageExecutionTime: 0,
			LastExecutionAt:      nil,
		}
	}

	_, err := r.collection.InsertOne(ctx, workflow)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	return nil
}

// GetByID obtiene un workflow por su ID
func (r *workflowRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Workflow, error) {
	if id.IsZero() {
		return nil, fmt.Errorf("invalid workflow ID")
	}

	var workflow models.Workflow
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&workflow)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return &workflow, nil
}

// GetByUserID obtiene workflows de un usuario con paginación
func (r *workflowRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Workflow, error) {
	if userID.IsZero() {
		return nil, fmt.Errorf("invalid user ID")
	}

	// Validar límites
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if skip < 0 {
		skip = 0
	}

	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflows by user: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, nil
}

// GetByStatus obtiene workflows por estatus
func (r *workflowRepository) GetByStatus(ctx context.Context, status string, limit, skip int) ([]*models.Workflow, error) {
	if status == "" {
		return nil, fmt.Errorf("status cannot be empty")
	}

	// Validar límites
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if skip < 0 {
		skip = 0
	}

	filter := bson.M{"status": status}
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflows by status: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, nil
}

// GetByTags obtiene workflows que contengan cualquiera de los tags especificados
func (r *workflowRepository) GetByTags(ctx context.Context, tags []string, limit, skip int) ([]*models.Workflow, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}

	// Validar límites
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if skip < 0 {
		skip = 0
	}

	filter := bson.M{
		"tags": bson.M{
			"$in": tags,
		},
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflows by tags: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, nil
}

// Search busca workflows con múltiples filtros
func (r *workflowRepository) Search(ctx context.Context, filters repository.WorkflowSearchFilters) ([]*models.Workflow, int64, error) {
	filter := bson.M{}

	// Filtros básicos
	if !filters.UserID.IsZero() {
		filter["user_id"] = filters.UserID
	}

	if filters.Status != "" {
		filter["status"] = filters.Status
	}

	if filters.IsActive != nil {
		filter["is_active"] = *filters.IsActive
	}

	// Filtro por tags
	if len(filters.Tags) > 0 {
		filter["tags"] = bson.M{"$in": filters.Tags}
	}

	// Búsqueda por texto
	if filters.Query != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": filters.Query, "$options": "i"}},
			{"description": bson.M{"$regex": filters.Query, "$options": "i"}},
		}
	}

	// Filtro de fecha
	if !filters.CreatedAfter.IsZero() || !filters.CreatedBefore.IsZero() {
		dateFilter := bson.M{}
		if !filters.CreatedAfter.IsZero() {
			dateFilter["$gte"] = filters.CreatedAfter
		}
		if !filters.CreatedBefore.IsZero() {
			dateFilter["$lte"] = filters.CreatedBefore
		}
		if len(dateFilter) > 0 {
			filter["created_at"] = dateFilter
		}
	}

	// Validar límites
	limit := filters.Limit
	skip := filters.Skip
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if skip < 0 {
		skip = 0
	}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, 0, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, totalCount, nil
}

// Update actualiza un workflow
func (r *workflowRepository) Update(ctx context.Context, id primitive.ObjectID, updates bson.M) error {
	if id.IsZero() {
		return fmt.Errorf("invalid workflow ID")
	}

	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	// Añadir timestamp de actualización
	updates["updated_at"] = time.Now()

	filter := bson.M{"_id": id}
	update := bson.M{"$set": updates}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update workflow: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("workflow not found")
	}

	return nil
}

// Delete elimina un workflow (soft delete)
func (r *workflowRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if id.IsZero() {
		return fmt.Errorf("invalid workflow ID")
	}

	// Soft delete - marcar como inactivo
	updates := bson.M{
		"is_active":  false,
		"status":     "deleted",
		"updated_at": time.Now(),
		"deleted_at": time.Now(),
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("workflow not found")
	}

	return nil
}

// CreateVersion crea una nueva versión de un workflow existente
func (r *workflowRepository) CreateVersion(ctx context.Context, originalID primitive.ObjectID, workflow *models.Workflow) error {
	if originalID.IsZero() {
		return fmt.Errorf("original workflow ID is required")
	}

	// Obtener workflow original para determinar la siguiente versión
	original, err := r.GetByID(ctx, originalID)
	if err != nil {
		return fmt.Errorf("failed to get original workflow: %w", err)
	}

	// Configurar nueva versión
	workflow.ParentWorkflowID = &originalID
	workflow.Version = original.Version + 1
	workflow.ID = primitive.NewObjectID()

	now := time.Now()
	workflow.CreatedAt = now
	workflow.UpdatedAt = now

	// Crear la nueva versión
	if err := r.Create(ctx, workflow); err != nil {
		return fmt.Errorf("failed to create workflow version: %w", err)
	}

	return nil
}

// GetVersions obtiene todas las versiones de un workflow
func (r *workflowRepository) GetVersions(ctx context.Context, parentID primitive.ObjectID) ([]*models.Workflow, error) {
	if parentID.IsZero() {
		return nil, fmt.Errorf("parent workflow ID is required")
	}

	filter := bson.M{
		"$or": []bson.M{
			{"_id": parentID},
			{"parent_workflow_id": parentID},
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "version", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow versions: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflow versions: %w", err)
	}

	return workflows, nil
}

// UpdateStats actualiza las estadísticas de un workflow
func (r *workflowRepository) UpdateStats(ctx context.Context, id primitive.ObjectID, stats *models.WorkflowStats) error {
	if id.IsZero() {
		return fmt.Errorf("invalid workflow ID")
	}
	if stats == nil {
		return fmt.Errorf("stats cannot be nil")
	}

	updates := bson.M{
		"stats.total_executions":       stats.TotalExecutions,
		"stats.successful_executions":  stats.SuccessfulExecutions,
		"stats.failed_executions":      stats.FailedExecutions,
		"stats.average_execution_time": stats.AverageExecutionTime,
		"stats.last_execution_at":      stats.LastExecutionAt,
		"updated_at":                   time.Now(),
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("failed to update workflow stats: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("workflow not found")
	}

	return nil
}

// GetActiveWorkflows obtiene todos los workflows activos
func (r *workflowRepository) GetActiveWorkflows(ctx context.Context) ([]*models.Workflow, error) {
	filter := bson.M{
		"is_active": true,
		"status": bson.M{
			"$in": []string{"active", "published"},
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get active workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode active workflows: %w", err)
	}

	return workflows, nil
}

// GetPopularTags obtiene los tags más utilizados
func (r *workflowRepository) GetPopularTags(ctx context.Context, limit int) ([]repository.TagCount, error) {
	if limit <= 0 {
		limit = 20
	}

	pipeline := []bson.M{
		{"$unwind": "$tags"},
		{
			"$group": bson.M{
				"_id":   "$tags",
				"count": bson.M{"$sum": 1},
			},
		},
		{"$sort": bson.M{"count": -1}},
		{"$limit": limit},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular tags: %w", err)
	}
	defer cursor.Close(ctx)

	var tags []repository.TagCount
	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}

		if err := cursor.Decode(&result); err != nil {
			continue
		}

		tags = append(tags, repository.TagCount{
			Tag:   result.ID,
			Count: result.Count,
		})
	}

	return tags, nil
}
