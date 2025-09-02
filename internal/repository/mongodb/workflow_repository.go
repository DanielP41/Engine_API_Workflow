package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options" //para interactuar con mongoDB: proporciona estructuras y métodos para personalizar operaciones como consultas, inserciones, actualizaciones, índices, y más.-

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
	workflow.SetDefaults()

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
			return nil, repository.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return &workflow, nil
}

// Update actualiza un workflow
func (r *workflowRepository) Update(ctx context.Context, id primitive.ObjectID, updates map[string]interface{}) error {
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
		return repository.ErrWorkflowNotFound
	}

	return nil
}

// Delete elimina un workflow (soft delete)
func (r *workflowRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if id.IsZero() {
		return fmt.Errorf("invalid workflow ID")
	}

	// Soft delete - marcar como inactivo
	updates := map[string]interface{}{
		"is_active":  false,
		"status":     models.WorkflowStatusArchived,
		"updated_at": time.Now(),
		"deleted_at": time.Now(),
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	if result.MatchedCount == 0 {
		return repository.ErrWorkflowNotFound
	}

	return nil
}

// FindWithPagination busca workflows con paginación - IMPLEMENTADO
func (r *workflowRepository) FindWithPagination(ctx context.Context, filter map[string]interface{}, page, limit int) ([]*models.Workflow, int64, error) {
	// Validar parámetros
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	skip := (page - 1) * limit

	// Convertir filter a bson.M
	bsonFilter := bson.M{}
	for k, v := range filter {
		bsonFilter[k] = v
	}

	// Contar total de documentos
	total, err := r.collection.CountDocuments(ctx, bsonFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bsonFilter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, 0, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, total, nil
}

// SearchAdvanced busca workflows con filtros avanzados - IMPLEMENTADO
func (r *workflowRepository) SearchAdvanced(ctx context.Context, filters repository.WorkflowSearchFilters) ([]*models.Workflow, int64, error) {
	filter := bson.M{}

	// Aplicar filtros
	if filters.UserID != nil {
		filter["user_id"] = *filters.UserID
	}

	if filters.Status != nil {
		filter["status"] = *filters.Status
	}

	if filters.IsActive != nil {
		filter["is_active"] = *filters.IsActive
	}

	if len(filters.Tags) > 0 {
		filter["tags"] = bson.M{"$in": filters.Tags}
	}

	if filters.Search != nil && *filters.Search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": *filters.Search, "$options": "i"}},
			{"description": bson.M{"$regex": *filters.Search, "$options": "i"}},
		}
	}

	if filters.Environment != nil {
		filter["environment"] = *filters.Environment
	}

	// Filtros de fecha
	if filters.CreatedAfter != nil || filters.CreatedBefore != nil {
		dateFilter := bson.M{}
		if filters.CreatedAfter != nil {
			dateFilter["$gte"] = *filters.CreatedAfter
		}
		if filters.CreatedBefore != nil {
			dateFilter["$lte"] = *filters.CreatedBefore
		}
		filter["created_at"] = dateFilter
	}

	// Contar total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos con ordenamiento por fecha
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, 0, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, total, nil
}

// List retrieves a paginated list of workflows - IMPLEMENTADO
func (r *workflowRepository) List(ctx context.Context, page, pageSize int) (*models.WorkflowListResponse, error) {
	workflows, total, err := r.FindWithPagination(ctx, map[string]interface{}{}, page, pageSize)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByUser retrieves workflows for a specific user - IMPLEMENTADO
func (r *workflowRepository) ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.WorkflowListResponse, error) {
	filter := map[string]interface{}{
		"user_id": userID,
	}

	workflows, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByStatus retrieves workflows by status - IMPLEMENTADO
func (r *workflowRepository) ListByStatus(ctx context.Context, status models.WorkflowStatus, page, pageSize int) (*models.WorkflowListResponse, error) {
	filter := map[string]interface{}{
		"status": status,
	}

	workflows, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// Search searches workflows - IMPLEMENTADO
func (r *workflowRepository) Search(ctx context.Context, query string, page, pageSize int) (*models.WorkflowListResponse, error) {
	filter := map[string]interface{}{}

	if query != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
		}
	}

	workflows, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// SearchByUser searches workflows for a specific user - IMPLEMENTADO
func (r *workflowRepository) SearchByUser(ctx context.Context, userID primitive.ObjectID, query string, page, pageSize int) (*models.WorkflowListResponse, error) {
	filter := map[string]interface{}{
		"user_id": userID,
	}

	if query != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
		}
	}

	workflows, total, err := r.FindWithPagination(ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateStatus updates workflow status - IMPLEMENTADO
func (r *workflowRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.WorkflowStatus) error {
	return r.Update(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// GetActiveWorkflows obtiene todos los workflows activos - IMPLEMENTADO
func (r *workflowRepository) GetActiveWorkflows(ctx context.Context) ([]*models.Workflow, error) {
	filter := bson.M{
		"is_active": true,
		"status":    models.WorkflowStatusActive,
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

// GetWorkflowsByTriggerType obtiene workflows por tipo de trigger - IMPLEMENTADO
func (r *workflowRepository) GetWorkflowsByTriggerType(ctx context.Context, triggerType models.TriggerType) ([]*models.Workflow, error) {
	filter := bson.M{
		"triggers.type": triggerType,
		"is_active":     true,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows by trigger type: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return workflows, nil
}

// CreateVersion crea una nueva versión de un workflow - IMPLEMENTADO
func (r *workflowRepository) CreateVersion(ctx context.Context, workflow *models.Workflow) error {
	// Implementación básica - crear nueva versión
	workflow.SetDefaults()
	return r.Create(ctx, workflow)
}

// GetVersions obtiene todas las versiones de un workflow - IMPLEMENTADO
func (r *workflowRepository) GetVersions(ctx context.Context, workflowID primitive.ObjectID) ([]*models.Workflow, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"_id": workflowID},
			{"parent_workflow_id": workflowID},
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

// Count returns total number of workflows - IMPLEMENTADO
func (r *workflowRepository) Count(ctx context.Context) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{})
}

// CountByUser returns workflow count for user - IMPLEMENTADO
func (r *workflowRepository) CountByUser(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{"user_id": userID})
}

// CountByStatus returns workflow count by status - IMPLEMENTADO
func (r *workflowRepository) CountByStatus(ctx context.Context, status models.WorkflowStatus) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{"status": status})
}

// NameExistsForUser checks if name exists for user - IMPLEMENTADO
func (r *workflowRepository) NameExistsForUser(ctx context.Context, name string, userID primitive.ObjectID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"name":    name,
		"user_id": userID,
	})
	return count > 0, err
}

// NameExistsForUserExcludeID checks if name exists for user excluding specific ID - IMPLEMENTADO
func (r *workflowRepository) NameExistsForUserExcludeID(ctx context.Context, name string, userID primitive.ObjectID, excludeID primitive.ObjectID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"name":    name,
		"user_id": userID,
		"_id":     bson.M{"$ne": excludeID},
	})
	return count > 0, err
}
