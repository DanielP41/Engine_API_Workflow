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

// MongoTemplateRepository implementación MongoDB del repositorio de templates
type MongoTemplateRepository struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

// NewMongoTemplateRepository crea una nueva instancia del repositorio de templates
func NewMongoTemplateRepository(db *mongo.Database, logger *zap.Logger) *MongoTemplateRepository {
	collection := db.Collection("email_templates")

	repo := &MongoTemplateRepository{
		collection: collection,
		logger:     logger,
	}

	// Crear índices
	repo.createIndexes()

	return repo
}

// createIndexes crea los índices necesarios para optimizar las consultas
func (r *MongoTemplateRepository) createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "version", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "is_active", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "language", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "is_active", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "tags", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_by", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		// Índice único para nombre + versión
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		r.logger.Error("Failed to create template indexes", zap.Error(err))
	} else {
		r.logger.Info("Template repository indexes created successfully")
	}
}

// Create crea un nuevo template
func (r *MongoTemplateRepository) Create(ctx context.Context, template *models.EmailTemplate) error {
	if template.ID.IsZero() {
		template.ID = primitive.NewObjectID()
	}

	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	// Verificar si ya existe un template con el mismo nombre y versión
	existing, err := r.GetByName(ctx, template.Name)
	if err == nil && existing != nil {
		// Si existe, incrementar la versión
		template.Version = existing.Version + 1
	} else if template.Version == 0 {
		template.Version = 1
	}

	_, err = r.collection.InsertOne(ctx, template)
	if err != nil {
		// Si hay conflicto de índice único, incrementar versión y reintentar
		if mongo.IsDuplicateKeyError(err) {
			template.Version++
			_, err = r.collection.InsertOne(ctx, template)
		}

		if err != nil {
			r.logger.Error("Failed to create template",
				zap.String("name", template.Name),
				zap.Error(err))
			return fmt.Errorf("failed to create template: %w", err)
		}
	}

	r.logger.Debug("Template created",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name),
		zap.Int("version", template.Version))

	return nil
}

// GetByID obtiene un template por ID
func (r *MongoTemplateRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailTemplate, error) {
	var template models.EmailTemplate

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&template)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, &RepositoryError{
				Code:    "TEMPLATE_NOT_FOUND",
				Message: "email template not found",
			}
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return &template, nil
}

// GetByName obtiene la versión más reciente de un template por nombre
func (r *MongoTemplateRepository) GetByName(ctx context.Context, name string) (*models.EmailTemplate, error) {
	var template models.EmailTemplate

	filter := bson.M{"name": name}
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	err := r.collection.FindOne(ctx, filter, opts).Decode(&template)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, &RepositoryError{
				Code:    "TEMPLATE_NOT_FOUND",
				Message: "email template not found",
			}
		}
		return nil, fmt.Errorf("failed to get template by name: %w", err)
	}

	return &template, nil
}

// Update actualiza un template existente
func (r *MongoTemplateRepository) Update(ctx context.Context, template *models.EmailTemplate) error {
	template.UpdatedAt = time.Now()

	filter := bson.M{"_id": template.ID}
	update := bson.M{"$set": template}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.logger.Error("Failed to update template",
			zap.String("id", template.ID.Hex()),
			zap.Error(err))
		return fmt.Errorf("failed to update template: %w", err)
	}

	if result.MatchedCount == 0 {
		return &RepositoryError{
			Code:    "TEMPLATE_NOT_FOUND",
			Message: "email template not found",
		}
	}

	r.logger.Debug("Template updated",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name))

	return nil
}

// Delete elimina un template
func (r *MongoTemplateRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	if result.DeletedCount == 0 {
		return &RepositoryError{
			Code:    "TEMPLATE_NOT_FOUND",
			Message: "email template not found",
		}
	}

	r.logger.Debug("Template deleted", zap.String("id", id.Hex()))
	return nil
}

// List obtiene una lista de templates con filtros
func (r *MongoTemplateRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.EmailTemplate, error) {
	// Construir filtros MongoDB
	mongoFilter := r.buildMongoFilter(filters)

	// Configurar opciones de consulta
	findOptions := options.Find().SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to find templates: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, fmt.Errorf("failed to decode templates: %w", err)
	}

	return templates, nil
}

// ListByType obtiene templates por tipo de notificación
func (r *MongoTemplateRepository) ListByType(ctx context.Context, notificationType models.NotificationType) ([]*models.EmailTemplate, error) {
	filter := bson.M{"type": notificationType}
	findOptions := options.Find().SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to find templates by type: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, fmt.Errorf("failed to decode templates: %w", err)
	}

	return templates, nil
}

// CreateVersion crea una nueva versión de un template existente
func (r *MongoTemplateRepository) CreateVersion(ctx context.Context, template *models.EmailTemplate) error {
	// Obtener la versión más reciente
	latest, err := r.GetByName(ctx, template.Name)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	// Crear nueva versión
	template.ID = primitive.NewObjectID()
	template.Version = latest.Version + 1
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	_, err = r.collection.InsertOne(ctx, template)
	if err != nil {
		r.logger.Error("Failed to create template version",
			zap.String("name", template.Name),
			zap.Int("version", template.Version),
			zap.Error(err))
		return fmt.Errorf("failed to create template version: %w", err)
	}

	r.logger.Info("Template version created",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name),
		zap.Int("version", template.Version))

	return nil
}

// GetVersions obtiene todas las versiones de un template
func (r *MongoTemplateRepository) GetVersions(ctx context.Context, templateName string) ([]*models.EmailTemplate, error) {
	filter := bson.M{"name": templateName}
	findOptions := options.Find().SetSort(bson.D{{Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get template versions: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, fmt.Errorf("failed to decode template versions: %w", err)
	}

	return templates, nil
}

// GetLatestVersion obtiene la versión más reciente de un template
func (r *MongoTemplateRepository) GetLatestVersion(ctx context.Context, templateName string) (*models.EmailTemplate, error) {
	return r.GetByName(ctx, templateName)
}

// SetActive activa o desactiva un template
func (r *MongoTemplateRepository) SetActive(ctx context.Context, id primitive.ObjectID, active bool) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"is_active":  active,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to set template active status: %w", err)
	}

	if result.MatchedCount == 0 {
		return &RepositoryError{
			Code:    "TEMPLATE_NOT_FOUND",
			Message: "email template not found",
		}
	}

	r.logger.Debug("Template active status updated",
		zap.String("id", id.Hex()),
		zap.Bool("active", active))

	return nil
}

// GetActive obtiene todos los templates activos
func (r *MongoTemplateRepository) GetActive(ctx context.Context) ([]*models.EmailTemplate, error) {
	filter := bson.M{"is_active": true}
	findOptions := options.Find().SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get active templates: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, fmt.Errorf("failed to decode active templates: %w", err)
	}

	return templates, nil
}

// buildMongoFilter construye un filtro MongoDB desde el mapa de filtros
func (r *MongoTemplateRepository) buildMongoFilter(filters map[string]interface{}) bson.M {
	mongoFilter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			mongoFilter["name"] = value
		case "type":
			if types, ok := value.([]models.NotificationType); ok {
				mongoFilter["type"] = bson.M{"$in": types}
			} else {
				mongoFilter["type"] = value
			}
		case "language":
			mongoFilter["language"] = value
		case "is_active":
			mongoFilter["is_active"] = value
		case "created_by":
			mongoFilter["created_by"] = value
		case "tags":
			if tags, ok := value.([]string); ok {
				mongoFilter["tags"] = bson.M{"$in": tags}
			} else {
				mongoFilter["tags"] = value
			}
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
		case "description_like":
			mongoFilter["description"] = bson.M{"$regex": value, "$options": "i"}
		case "latest_only":
			if value.(bool) {
				// Esto requiere una agregación más compleja, por simplicidad lo omitimos aquí
				// En una implementación completa, se usaría un pipeline de agregación
			}
		default:
			// Filtro genérico
			mongoFilter[key] = value
		}
	}

	return mongoFilter
}

// Métodos adicionales para funcionalidades avanzadas

// GetTemplateByNameAndVersion obtiene una versión específica de un template
func (r *MongoTemplateRepository) GetTemplateByNameAndVersion(ctx context.Context, name string, version int) (*models.EmailTemplate, error) {
	var template models.EmailTemplate

	filter := bson.M{
		"name":    name,
		"version": version,
	}

	err := r.collection.FindOne(ctx, filter).Decode(&template)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, &RepositoryError{
				Code:    "TEMPLATE_NOT_FOUND",
				Message: "email template not found",
			}
		}
		return nil, fmt.Errorf("failed to get template by name and version: %w", err)
	}

	return &template, nil
}

// ValidateTemplateUniqueness verifica que un template no exista con el mismo nombre y versión
func (r *MongoTemplateRepository) ValidateTemplateUniqueness(ctx context.Context, name string, version int, excludeID *primitive.ObjectID) error {
	filter := bson.M{
		"name":    name,
		"version": version,
	}

	if excludeID != nil {
		filter["_id"] = bson.M{"$ne": *excludeID}
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to validate template uniqueness: %w", err)
	}

	if count > 0 {
		return &RepositoryError{
			Code:    "TEMPLATE_ALREADY_EXISTS",
			Message: "email template already exists",
		}
	}

	return nil
}

// GetTemplateStats obtiene estadísticas de uso de templates
func (r *MongoTemplateRepository) GetTemplateStats(ctx context.Context) (map[string]int64, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   "$type",
				"count": bson.M{"$sum": 1},
				"active_count": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							"$is_active",
							1,
							0,
						},
					},
				},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get template stats: %w", err)
	}
	defer cursor.Close(ctx)

	stats := make(map[string]int64)
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode template stats: %w", err)
	}

	for _, result := range results {
		templateType, ok := result["_id"].(string)
		if !ok {
			continue
		}

		count := getInt64FromBSON(result, "count")
		activeCount := getInt64FromBSON(result, "active_count")

		stats[templateType+"_total"] = count
		stats[templateType+"_active"] = activeCount
	}

	return stats, nil
}

// BulkActivate activa/desactiva múltiples templates
func (r *MongoTemplateRepository) BulkActivate(ctx context.Context, ids []primitive.ObjectID, active bool) error {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{
		"$set": bson.M{
			"is_active":  active,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to bulk activate templates: %w", err)
	}

	r.logger.Info("Bulk template activation",
		zap.Int64("updated", result.ModifiedCount),
		zap.Bool("active", active))

	return nil
}

// SearchTemplates busca templates por texto en nombre, descripción o contenido
func (r *MongoTemplateRepository) SearchTemplates(ctx context.Context, searchTerm string, limit int) ([]*models.EmailTemplate, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"description": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"subject": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"body_text": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"body_html": bson.M{"$regex": searchTerm, "$options": "i"}},
			{"tags": bson.M{"$in": []string{searchTerm}}},
		},
	}

	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to search templates: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return templates, nil
}

// CloneTemplate clona un template con un nuevo nombre
func (r *MongoTemplateRepository) CloneTemplate(ctx context.Context, sourceID primitive.ObjectID, newName string, createdBy string) (*models.EmailTemplate, error) {
	// Obtener template original
	source, err := r.GetByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source template: %w", err)
	}

	// Crear clon
	clone := &models.EmailTemplate{
		ID:          primitive.NewObjectID(),
		Name:        newName,
		Type:        source.Type,
		Language:    source.Language,
		Version:     1,
		Subject:     source.Subject,
		BodyText:    source.BodyText,
		BodyHTML:    source.BodyHTML,
		Description: fmt.Sprintf("Cloned from %s", source.Name),
		Tags:        append([]string{"cloned"}, source.Tags...),
		Variables:   source.Variables,
		IsActive:    false, // Los clones inician inactivos
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   createdBy,
	}

	if err := r.Create(ctx, clone); err != nil {
		return nil, fmt.Errorf("failed to create cloned template: %w", err)
	}

	r.logger.Info("Template cloned",
		zap.String("source_id", sourceID.Hex()),
		zap.String("clone_id", clone.ID.Hex()),
		zap.String("new_name", newName))

	return clone, nil
}
