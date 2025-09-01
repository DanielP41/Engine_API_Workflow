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
	if workflow.Active == nil {
		active := true
		workflow.Active = &active
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

// List obtiene workflows con paginación
func (r *workflowRepository) List(ctx context.Context, page, pageSize int) (*models.WorkflowListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize
	filter := bson.M{"is_active": bson.M{"$ne": false}}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByUser obtiene workflows de un usuario específico
func (r *workflowRepository) ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.WorkflowListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize
	filter := bson.M{
		"user_id":   userID,
		"is_active": bson.M{"$ne": false},
	}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListByStatus obtiene workflows por estatus
func (r *workflowRepository) ListByStatus(ctx context.Context, status models.WorkflowStatus, page, pageSize int) (*models.WorkflowListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize
	filter := bson.M{
		"status":    status,
		"is_active": bson.M{"$ne": false},
	}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// Search busca workflows con filtros
func (r *workflowRepository) Search(ctx context.Context, query string, page, pageSize int) (*models.WorkflowListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize
	filter := bson.M{"is_active": bson.M{"$ne": false}}

	// Agregar búsqueda de texto si se proporciona query
	if query != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
		}
	}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// SearchByUser busca workflows de un usuario específico
func (r *workflowRepository) SearchByUser(ctx context.Context, userID primitive.ObjectID, query string, page, pageSize int) (*models.WorkflowListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize
	filter := bson.M{
		"user_id":   userID,
		"is_active": bson.M{"$ne": false},
	}

	// Agregar búsqueda de texto si se proporciona query
	if query != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
		}
	}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count workflows: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search workflows: %w", err)
	}
	defer cursor.Close(ctx)

	var workflows []*models.Workflow
	if err = cursor.All(ctx, &workflows); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateStatus actualiza el status de un workflow
func (r *workflowRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.WorkflowStatus) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{
		"status":     status,
		"updated_at": time.Now(),
	}}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update workflow status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("workflow not found")
	}

	return nil
}

// UpdateRunStats actualiza las estadísticas de ejecución
func (r *workflowRepository) UpdateRunStats(ctx context.Context, id primitive.ObjectID, success bool) error {
	workflow, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if workflow.Stats == nil {
		workflow.Stats = &models.WorkflowStats{}
	}

	workflow.Stats.TotalExecutions++
	if success {
		workflow.Stats.SuccessfulExecutions++
	} else {
		workflow.Stats.FailedExecutions++
	}

	now := time.Now()
	workflow.Stats.LastExecutionAt = &now

	return r.Update(ctx, id, map[string]interface{}{
		"stats": workflow.Stats,
	})
}

// GetActiveWorkflows obtiene todos los workflows activos
func (r *workflowRepository) GetActiveWorkflows(ctx context.Context) ([]*models.Workflow, error) {
	filter := bson.M{
		"status": bson.M{
			"$in": []string{"active"},
		},
		"is_active": bson.M{"$ne": false},
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

// GetWorkflowsByTriggerType obtiene workflows por tipo de trigger
func (r *workflowRepository) GetWorkflowsByTriggerType(ctx context.Context, triggerType models.TriggerType) ([]*models.Workflow, error) {
	filter := bson.M{
		"triggers.type": triggerType,
		"is_active":     bson.M{"$ne": false},
		"status":        "active",
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

// CreateVersion crea una nueva versión de un workflow
func (r *workflowRepository) CreateVersion(ctx context.Context, workflow *models.Workflow) error {
	return r.Create(ctx, workflow)
}

// GetVersions obtiene todas las versiones de un workflow
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

// ListByTags obtiene workflows que contengan los tags especificados
func (r *workflowRepository) ListByTags(ctx context.Context, tags []string, page, pageSize int) (*models.WorkflowListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	skip := (page - 1) * pageSize
	filter := bson.M{
		"tags":      bson.M{"$in": tags},
		"is_active": bson.M{"$ne": false},
	}

	// Contar total
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count workflows by tags: %w", err)
	}

	// Obtener documentos
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize)).
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

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &models.WorkflowListResponse{
		Workflows:  workflows,
		Total:      totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAllTags obtiene todos los tags únicos
func (r *workflowRepository) GetAllTags(ctx context.Context) ([]string, error) {
	pipeline := []bson.M{
		{"$unwind": "$tags"},
		{"$group": bson.M{"_id": "$tags"}},
		{"$sort": bson.M{"_id": 1}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tags: %w", err)
	}
	defer cursor.Close(ctx)

	var tags []string
	for cursor.Next(ctx) {
		var result struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		tags = append(tags, result.ID)
	}

	return tags, nil
}

// Count retorna el total de workflows activos
func (r *workflowRepository) Count(ctx context.Context) (int64, error) {
	filter := bson.M{"is_active": bson.M{"$ne": false}}
	return r.collection.CountDocuments(ctx, filter)
}

// CountByUser retorna el total de workflows por usuario
func (r *workflowRepository) CountByUser(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"user_id":   userID,
		"is_active": bson.M{"$ne": false},
	}
	return r.collection.CountDocuments(ctx, filter)
}

// CountByStatus retorna el total de workflows por status
func (r *workflowRepository) CountByStatus(ctx context.Context, status models.WorkflowStatus) (int64, error) {
	filter := bson.M{
		"status":    status,
		"is_active": bson.M{"$ne": false},
	}
	return r.collection.CountDocuments(ctx, filter)
}

// NameExistsForUser verifica si existe un workflow con el mismo nombre para un usuario
func (r *workflowRepository) NameExistsForUser(ctx context.Context, name string, userID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"name":      name,
		"user_id":   userID,
		"is_active": bson.M{"$ne": false},
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check name existence: %w", err)
	}

	return count > 0, nil
}

// NameExistsForUserExcludeID verifica si existe un workflow con el mismo nombre excluyendo un ID específico
func (r *workflowRepository) NameExistsForUserExcludeID(ctx context.Context, name string, userID primitive.ObjectID, excludeID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"name":      name,
		"user_id":   userID,
		"_id":       bson.M{"$ne": excludeID},
		"is_active": bson.M{"$ne": false},
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check name existence: %w", err)
	}

	return count > 0, nil
}
