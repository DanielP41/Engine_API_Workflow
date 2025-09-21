package repository

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"Engine_API_Workflow/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// ================================
// ESTRUCTURAS AUXILIARES
// ================================

// TemplatePreview vista previa renderizada de un template
type TemplatePreview struct {
	Subject  string   `json:"subject"`
	BodyHTML string   `json:"body_html"`
	BodyText string   `json:"body_text"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// TemplateUsageStats estadísticas de uso de un template
type TemplateUsageStats struct {
	TemplateName     string           `json:"template_name"`
	TotalUsage       int64            `json:"total_usage"`
	SuccessfulSends  int64            `json:"successful_sends"`
	FailedSends      int64            `json:"failed_sends"`
	LastUsed         *time.Time       `json:"last_used,omitempty"`
	UsageByDay       map[string]int64 `json:"usage_by_day"`
	PopularVariables []string         `json:"popular_variables"`
}

// TemplateDiff diferencias entre versiones de template
type TemplateDiff struct {
	TemplateName string              `json:"template_name"`
	Version1     int                 `json:"version1"`
	Version2     int                 `json:"version2"`
	Changes      []TemplateChange    `json:"changes"`
	Summary      TemplateDiffSummary `json:"summary"`
}

// TemplateChange representa un cambio específico
type TemplateChange struct {
	Field    string      `json:"field"`
	Type     string      `json:"type"` // added, removed, modified
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
}

// TemplateDiffSummary resumen de cambios
type TemplateDiffSummary struct {
	TotalChanges   int `json:"total_changes"`
	FieldsAdded    int `json:"fields_added"`
	FieldsRemoved  int `json:"fields_removed"`
	FieldsModified int `json:"fields_modified"`
}

// TemplateExport estructura para exportar templates
type TemplateExport struct {
	Template   *models.EmailTemplate  `json:"template"`
	Metadata   map[string]interface{} `json:"metadata"`
	Version    string                 `json:"version"`
	ExportedAt time.Time              `json:"exported_at"`
}

// TemplateImportOptions opciones para importar templates
type TemplateImportOptions struct {
	OverwriteExisting bool `json:"overwrite_existing"`
	CreateNewVersion  bool `json:"create_new_version"`
	ValidateOnly      bool `json:"validate_only"`
	ImportAsInactive  bool `json:"import_as_inactive"`
}

// TemplateSearchFilters filtros avanzados para búsqueda
type TemplateSearchFilters struct {
	Name          *string                   `json:"name,omitempty"`
	Type          []models.NotificationType `json:"type,omitempty"`
	Language      *string                   `json:"language,omitempty"`
	IsActive      *bool                     `json:"is_active,omitempty"`
	CreatedBy     *string                   `json:"created_by,omitempty"`
	Tags          []string                  `json:"tags,omitempty"`
	CreatedAfter  *time.Time                `json:"created_after,omitempty"`
	CreatedBefore *time.Time                `json:"created_before,omitempty"`
	HasVariables  []string                  `json:"has_variables,omitempty"`
	ContentSearch *string                   `json:"content_search,omitempty"`
}

// ================================
// IMPLEMENTACIÓN DEL REPOSITORIO
// ================================

// MongoTemplateRepository implementación MongoDB del repositorio de templates
type MongoTemplateRepository struct {
	collection *mongo.Collection
	logger     *zap.Logger
	dbName     string
}

// NewMongoTemplateRepository crea una nueva instancia del repositorio
func NewMongoTemplateRepository(db *mongo.Database, logger *zap.Logger) TemplateRepository {
	collection := db.Collection("email_templates")

	repo := &MongoTemplateRepository{
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

// Create crea un nuevo template
func (r *MongoTemplateRepository) Create(ctx context.Context, template *models.EmailTemplate) error {
	if template.ID.IsZero() {
		template.ID = primitive.NewObjectID()
	}

	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	// Validar template antes de crear
	if err := template.ValidateTemplate(); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Verificar si ya existe un template con el mismo nombre
	existing, err := r.GetByName(ctx, template.Name)
	if err == nil && existing != nil {
		// Si existe, incrementar la versión
		template.Version = existing.Version + 1
	} else if template.Version == 0 {
		template.Version = 1
	}

	// Establecer valores por defecto
	if template.Language == "" {
		template.Language = "en"
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
				zap.Int("version", template.Version),
				zap.Error(err))
			return fmt.Errorf("failed to create template: %w", err)
		}
	}

	r.logger.Info("Template created successfully",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name),
		zap.Int("version", template.Version),
		zap.String("type", string(template.Type)))

	return nil
}

// GetByID obtiene un template por ID
func (r *MongoTemplateRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailTemplate, error) {
	var template models.EmailTemplate

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&template)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrTemplateNotFound
		}
		r.logger.Error("Failed to get template by ID",
			zap.String("id", id.Hex()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return &template, nil
}

// GetByName obtiene la versión más reciente de un template por nombre
func (r *MongoTemplateRepository) GetByName(ctx context.Context, name string) (*models.EmailTemplate, error) {
	var template models.EmailTemplate

	filter := bson.M{
		"name":      name,
		"is_active": true,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	err := r.collection.FindOne(ctx, filter, opts).Decode(&template)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("failed to get template by name: %w", err)
	}

	return &template, nil
}

// Update actualiza un template existente
func (r *MongoTemplateRepository) Update(ctx context.Context, template *models.EmailTemplate) error {
	template.UpdatedAt = time.Now()

	// Validar template antes de actualizar
	if err := template.ValidateTemplate(); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

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
		return ErrTemplateNotFound
	}

	r.logger.Info("Template updated successfully",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name))

	return nil
}

// Delete elimina un template
func (r *MongoTemplateRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		r.logger.Error("Failed to delete template",
			zap.String("id", id.Hex()),
			zap.Error(err))
		return fmt.Errorf("failed to delete template: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrTemplateNotFound
	}

	r.logger.Info("Template deleted successfully",
		zap.String("id", id.Hex()))

	return nil
}

// ================================
// LISTADO Y BÚSQUEDA
// ================================

// List obtiene una lista de templates con filtros
func (r *MongoTemplateRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.EmailTemplate, error) {
	mongoFilter := r.buildMongoFilter(filters)

	// Configurar opciones de consulta
	findOptions := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		r.logger.Error("Failed to list templates",
			zap.Any("filters", filters),
			zap.Error(err))
		return nil, fmt.Errorf("failed to find templates: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	for cursor.Next(ctx) {
		var template models.EmailTemplate
		if err := cursor.Decode(&template); err != nil {
			r.logger.Error("Failed to decode template", zap.Error(err))
			continue
		}
		templates = append(templates, &template)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	r.logger.Debug("Templates listed successfully",
		zap.Int("count", len(templates)))

	return templates, nil
}

// ListByType obtiene templates por tipo de notificación
func (r *MongoTemplateRepository) ListByType(ctx context.Context, notificationType models.NotificationType) ([]*models.EmailTemplate, error) {
	filter := bson.M{
		"type":      notificationType,
		"is_active": true,
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to find templates by type: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	for cursor.Next(ctx) {
		var template models.EmailTemplate
		if err := cursor.Decode(&template); err != nil {
			r.logger.Error("Failed to decode template", zap.Error(err))
			continue
		}
		templates = append(templates, &template)
	}

	return templates, cursor.Err()
}

// ================================
// VERSIONADO
// ================================

// CreateVersion crea una nueva versión de un template existente
func (r *MongoTemplateRepository) CreateVersion(ctx context.Context, template *models.EmailTemplate) error {
	// Obtener la versión más reciente
	latest, err := r.GetLatestVersion(ctx, template.Name)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	// Crear nueva versión
	template.ID = primitive.NewObjectID()
	template.Version = latest.Version + 1
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	// Validar template
	if err := template.ValidateTemplate(); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	_, err = r.collection.InsertOne(ctx, template)
	if err != nil {
		r.logger.Error("Failed to create template version",
			zap.String("name", template.Name),
			zap.Int("version", template.Version),
			zap.Error(err))
		return fmt.Errorf("failed to create template version: %w", err)
	}

	r.logger.Info("Template version created successfully",
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
	for cursor.Next(ctx) {
		var template models.EmailTemplate
		if err := cursor.Decode(&template); err != nil {
			r.logger.Error("Failed to decode template version", zap.Error(err))
			continue
		}
		templates = append(templates, &template)
	}

	return templates, cursor.Err()
}

// GetLatestVersion obtiene la versión más reciente de un template
func (r *MongoTemplateRepository) GetLatestVersion(ctx context.Context, templateName string) (*models.EmailTemplate, error) {
	var template models.EmailTemplate

	filter := bson.M{"name": templateName}
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	err := r.collection.FindOne(ctx, filter, opts).Decode(&template)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("failed to get latest template version: %w", err)
	}

	return &template, nil
}

// ================================
// ESTADO Y ACTIVACIÓN
// ================================

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
		return ErrTemplateNotFound
	}

	r.logger.Info("Template active status updated",
		zap.String("id", id.Hex()),
		zap.Bool("active", active))

	return nil
}

// GetActive obtiene todos los templates activos
func (r *MongoTemplateRepository) GetActive(ctx context.Context) ([]*models.EmailTemplate, error) {
	filter := bson.M{"is_active": true}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get active templates: %w", err)
	}
	defer cursor.Close(ctx)

	var templates []*models.EmailTemplate
	for cursor.Next(ctx) {
		var template models.EmailTemplate
		if err := cursor.Decode(&template); err != nil {
			r.logger.Error("Failed to decode active template", zap.Error(err))
			continue
		}
		templates = append(templates, &template)
	}

	return templates, cursor.Err()
}

// ================================
// FUNCIONALIDADES AVANZADAS
// ================================

// RenderPreview genera una vista previa de un template con datos de prueba
func (r *MongoTemplateRepository) RenderPreview(ctx context.Context, templateName string, data map[string]interface{}) (*TemplatePreview, error) {
	template, err := r.GetByName(ctx, templateName)
	if err != nil {
		return nil, err
	}

	preview := &TemplatePreview{}

	// Renderizar subject
	if subject, err := r.renderTemplate(template.Subject, data); err != nil {
		preview.Errors = append(preview.Errors, fmt.Sprintf("Subject rendering error: %v", err))
	} else {
		preview.Subject = subject
	}

	// Renderizar body text
	if template.BodyText != "" {
		if bodyText, err := r.renderTemplate(template.BodyText, data); err != nil {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Text body rendering error: %v", err))
		} else {
			preview.BodyText = bodyText
		}
	}

	// Renderizar body HTML
	if template.BodyHTML != "" {
		if bodyHTML, err := r.renderHTMLTemplate(template.BodyHTML, data); err != nil {
			preview.Errors = append(preview.Errors, fmt.Sprintf("HTML body rendering error: %v", err))
		} else {
			preview.BodyHTML = bodyHTML
		}
	}

	// Verificar variables faltantes
	r.checkMissingVariables(template, data, preview)

	return preview, nil
}

// ValidateTemplate valida un template con datos de prueba
func (r *MongoTemplateRepository) ValidateTemplate(ctx context.Context, template *models.EmailTemplate, testData map[string]interface{}) error {
	// Validación sintáctica básica
	if err := template.ValidateTemplate(); err != nil {
		return err
	}

	// Validación de renderizado con datos de prueba
	if testData != nil {
		_, err := r.renderTemplate(template.Subject, testData)
		if err != nil {
			return fmt.Errorf("subject template validation failed: %w", err)
		}

		if template.BodyText != "" {
			_, err = r.renderTemplate(template.BodyText, testData)
			if err != nil {
				return fmt.Errorf("text body template validation failed: %w", err)
			}
		}

		if template.BodyHTML != "" {
			_, err = r.renderHTMLTemplate(template.BodyHTML, testData)
			if err != nil {
				return fmt.Errorf("HTML body template validation failed: %w", err)
			}
		}
	}

	return nil
}

// GetUsageStats obtiene estadísticas de uso de un template
func (r *MongoTemplateRepository) GetUsageStats(ctx context.Context, templateName string, days int) (*TemplateUsageStats, error) {
	// Esta función requiere acceso a la colección de notificaciones
	// Por simplicidad, devolvemos una estructura básica
	// En una implementación real, haríamos agregación cross-collection

	stats := &TemplateUsageStats{
		TemplateName: templateName,
		UsageByDay:   make(map[string]int64),
	}

	// TODO: Implementar consulta real a la colección de notificaciones
	r.logger.Info("Getting template usage stats",
		zap.String("template", templateName),
		zap.Int("days", days))

	return stats, nil
}

// CompareVersions compara dos versiones de un template
func (r *MongoTemplateRepository) CompareVersions(ctx context.Context, templateName string, version1, version2 int) (*TemplateDiff, error) {
	// Obtener ambas versiones
	filter1 := bson.M{"name": templateName, "version": version1}
	filter2 := bson.M{"name": templateName, "version": version2}

	var template1, template2 models.EmailTemplate

	err := r.collection.FindOne(ctx, filter1).Decode(&template1)
	if err != nil {
		return nil, fmt.Errorf("failed to get template version %d: %w", version1, err)
	}

	err = r.collection.FindOne(ctx, filter2).Decode(&template2)
	if err != nil {
		return nil, fmt.Errorf("failed to get template version %d: %w", version2, err)
	}

	// Comparar templates
	diff := &TemplateDiff{
		TemplateName: templateName,
		Version1:     version1,
		Version2:     version2,
		Changes:      []TemplateChange{},
	}

	// Comparar campos
	r.compareFields(diff, "subject", template1.Subject, template2.Subject)
	r.compareFields(diff, "body_text", template1.BodyText, template2.BodyText)
	r.compareFields(diff, "body_html", template1.BodyHTML, template2.BodyHTML)
	r.compareFields(diff, "description", template1.Description, template2.Description)
	r.compareFields(diff, "language", template1.Language, template2.Language)
	r.compareFields(diff, "is_active", template1.IsActive, template2.IsActive)

	// Calcular resumen
	diff.Summary.TotalChanges = len(diff.Changes)
	for _, change := range diff.Changes {
		switch change.Type {
		case "added":
			diff.Summary.FieldsAdded++
		case "removed":
			diff.Summary.FieldsRemoved++
		case "modified":
			diff.Summary.FieldsModified++
		}
	}

	return diff, nil
}

// ExportTemplate exporta un template con metadatos
func (r *MongoTemplateRepository) ExportTemplate(ctx context.Context, templateName string) (*TemplateExport, error) {
	template, err := r.GetByName(ctx, templateName)
	if err != nil {
		return nil, err
	}

	export := &TemplateExport{
		Template:   template,
		Version:    "1.0",
		ExportedAt: time.Now(),
		Metadata: map[string]interface{}{
			"exported_by":    "system",
			"total_versions": 1, // TODO: Contar versiones reales
		},
	}

	return export, nil
}

// ImportTemplate importa un template con opciones
func (r *MongoTemplateRepository) ImportTemplate(ctx context.Context, templateData *TemplateExport, options TemplateImportOptions) error {
	if options.ValidateOnly {
		return r.ValidateTemplate(ctx, templateData.Template, nil)
	}

	// Verificar si existe
	existing, err := r.GetByName(ctx, templateData.Template.Name)
	if err == nil && existing != nil {
		if !options.OverwriteExisting && !options.CreateNewVersion {
			return fmt.Errorf("template already exists: %s", templateData.Template.Name)
		}

		if options.CreateNewVersion {
			return r.CreateVersion(ctx, templateData.Template)
		}
	}

	// Configurar opciones de importación
	if options.ImportAsInactive {
		templateData.Template.IsActive = false
	}

	return r.Create(ctx, templateData.Template)
}

// ================================
// OPERACIONES EN LOTE
// ================================

// BulkSetActive activa/desactiva múltiples templates
func (r *MongoTemplateRepository) BulkSetActive(ctx context.Context, templateNames []string, active bool) error {
	filter := bson.M{"name": bson.M{"$in": templateNames}}
	update := bson.M{
		"$set": bson.M{
			"is_active":  active,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to bulk set active status: %w", err)
	}

	r.logger.Info("Bulk active status update completed",
		zap.Strings("templates", templateNames),
		zap.Bool("active", active),
		zap.Int64("modified_count", result.ModifiedCount))

	return nil
}

// BulkDelete elimina múltiples templates
func (r *MongoTemplateRepository) BulkDelete(ctx context.Context, ids []primitive.ObjectID) error {
	filter := bson.M{"_id": bson.M{"$in": ids}}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to bulk delete templates: %w", err)
	}

	r.logger.Info("Bulk delete completed",
		zap.Int("template_count", len(ids)),
		zap.Int64("deleted_count", result.DeletedCount))

	return nil
}

// ================================
// MÉTODOS INTERNOS Y UTILIDADES
// ================================

// createIndexes crea los índices necesarios para optimizar consultas
func (r *MongoTemplateRepository) createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		// Índice único compuesto para nombre y versión
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().SetName("name_version_unique").SetUnique(true),
		},
		// Índice para templates activos
		{
			Keys: bson.D{
				{Key: "is_active", Value: 1},
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetName("active_name"),
		},
		// Índice por tipo
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "is_active", Value: 1},
			},
			Options: options.Index().SetName("type_active"),
		},
		// Índice por idioma
		{
			Keys: bson.D{
				{Key: "language", Value: 1},
				{Key: "is_active", Value: 1},
			},
			Options: options.Index().SetName("language_active"),
		},
		// Índice por creador
		{
			Keys: bson.D{
				{Key: "created_by", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("created_by_date"),
		},
		// Índice por tags
		{
			Keys:    bson.D{{Key: "tags", Value: 1}},
			Options: options.Index().SetName("tags"),
		},
		// Índice de texto para búsqueda
		{
			Keys: bson.D{
				{Key: "name", Value: "text"},
				{Key: "description", Value: "text"},
				{Key: "subject", Value: "text"},
			},
			Options: options.Index().SetName("text_search"),
		},
		// Índice temporal para templates
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("created_at_desc"),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		r.logger.Error("Failed to create template indexes", zap.Error(err))
	} else {
		r.logger.Info("Template repository indexes created successfully",
			zap.String("collection", r.collection.Name()),
			zap.String("database", r.dbName))
	}
}

// buildMongoFilter construye un filtro MongoDB desde filtros genéricos
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
		case "content_search":
			if searchText, ok := value.(string); ok {
				mongoFilter["$text"] = bson.M{"$search": searchText}
			}
		default:
			mongoFilter[key] = value
		}
	}

	return mongoFilter
}

// renderTemplate renderiza un template de texto
func (r *MongoTemplateRepository) renderTemplate(templateText string, data map[string]interface{}) (string, error) {
	tmpl, err := textTemplate.New("template").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buffer.String(), nil
}

// renderHTMLTemplate renderiza un template HTML
func (r *MongoTemplateRepository) renderHTMLTemplate(templateHTML string, data map[string]interface{}) (string, error) {
	tmpl, err := htmlTemplate.New("template").Parse(templateHTML)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return buffer.String(), nil
}

// checkMissingVariables verifica variables faltantes en los datos
func (r *MongoTemplateRepository) checkMissingVariables(template *models.EmailTemplate, data map[string]interface{}, preview *TemplatePreview) {
	// Extraer variables de los templates
	subjectVars := r.extractVariables(template.Subject)
	textVars := r.extractVariables(template.BodyText)
	htmlVars := r.extractVariables(template.BodyHTML)

	// Combinar todas las variables
	allVars := make(map[string]bool)
	for _, v := range subjectVars {
		allVars[v] = true
	}
	for _, v := range textVars {
		allVars[v] = true
	}
	for _, v := range htmlVars {
		allVars[v] = true
	}

	// Verificar variables faltantes
	for variable := range allVars {
		if _, exists := data[variable]; !exists {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("Missing variable: %s", variable))
		}
	}
}

// extractVariables extrae variables de un template (implementación simple)
func (r *MongoTemplateRepository) extractVariables(templateText string) []string {
	var variables []string

	// Buscar patrones {{.Variable}}
	parts := strings.Split(templateText, "{{.")
	for i := 1; i < len(parts); i++ {
		end := strings.Index(parts[i], "}}")
		if end > 0 {
			variable := strings.TrimSpace(parts[i][:end])
			if variable != "" {
				variables = append(variables, variable)
			}
		}
	}

	return variables
}

// compareFields compara dos campos y añade cambios al diff
func (r *MongoTemplateRepository) compareFields(diff *TemplateDiff, fieldName string, oldValue, newValue interface{}) {
	if oldValue != newValue {
		changeType := "modified"
		if oldValue == nil || oldValue == "" {
			changeType = "added"
		} else if newValue == nil || newValue == "" {
			changeType = "removed"
		}

		diff.Changes = append(diff.Changes, TemplateChange{
			Field:    fieldName,
			Type:     changeType,
			OldValue: oldValue,
			NewValue: newValue,
		})
	}
}

// ================================
// MÉTODOS DE VALIDACIÓN Y UTILIDAD
// ================================

// ValidateConnection verifica que la conexión a la base de datos esté activa
func (r *MongoTemplateRepository) ValidateConnection(ctx context.Context) error {
	return r.collection.Database().Client().Ping(ctx, nil)
}

// GetCollectionInfo obtiene información sobre la colección
func (r *MongoTemplateRepository) GetCollectionInfo(ctx context.Context) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":             nil,
				"total_templates": bson.M{"$sum": 1},
				"active_templates": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							"$is_active",
							1,
							0,
						},
					},
				},
				"languages": bson.M{"$addToSet": "$language"},
				"types":     bson.M{"$addToSet": "$type"},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}
	defer cursor.Close(ctx)

	var result map[string]interface{}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode collection info: %w", err)
		}
	}

	return result, nil
}
