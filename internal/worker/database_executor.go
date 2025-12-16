package worker

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
)

// DatabaseActionExecutor ejecuta acciones relacionadas con base de datos
type DatabaseActionExecutor struct {
	client       *mongo.Client
	databaseName string
	logger       *zap.Logger
}

// DatabaseOperation representa una operación de base de datos
type DatabaseOperation struct {
	Type       string                 `json:"type"`       // "insert", "update", "delete", "find", "aggregate"
	Collection string                 `json:"collection"` // Nombre de la colección
	Document   map[string]interface{} `json:"document,omitempty"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
	Update     map[string]interface{} `json:"update,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
	Pipeline   []interface{}          `json:"pipeline,omitempty"` // Para aggregation
}

// DatabaseResult resultado de una operación de base de datos
type DatabaseResult struct {
	Success       bool                   `json:"success"`
	Operation     string                 `json:"operation"`
	Collection    string                 `json:"collection"`
	AffectedRows  int64                  `json:"affected_rows,omitempty"`
	InsertedID    interface{}            `json:"inserted_id,omitempty"`
	Documents     []interface{}          `json:"documents,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	ExecutionTime time.Duration          `json:"execution_time"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewDatabaseActionExecutor crea un nuevo ejecutor de acciones de base de datos
func NewDatabaseActionExecutor(mongoClient *mongo.Client, databaseName string, logger *zap.Logger) *DatabaseActionExecutor {
	return &DatabaseActionExecutor{
		client:       mongoClient,
		databaseName: databaseName,
		logger:       logger.With(zap.String("component", "database_executor")),
	}
}

// Execute ejecuta una acción de base de datos basada en la configuración del paso
func (e *DatabaseActionExecutor) Execute(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	startTime := time.Now()

	e.logger.Info("Executing database action",
		zap.String("step_id", step.ID),
		zap.String("step_name", step.Name))

	// Extraer operación de la configuración del paso
	operation, err := e.parseOperation(step.Config)
	if err != nil {
		return &StepResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse operation: %v", err),
			Duration:     time.Since(startTime),
			Output:       make(map[string]interface{}),
		}, err
	}

	// Validar operación
	if err := e.validateOperation(operation); err != nil {
		return &StepResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Invalid operation: %v", err),
			Duration:     time.Since(startTime),
			Output: map[string]interface{}{
				"operation":  operation.Type,
				"collection": operation.Collection,
			},
		}, err
	}

	// Obtener base de datos
	database := e.client.Database(e.databaseName)
	collection := database.Collection(operation.Collection)

	// Ejecutar operación según el tipo
	var dbResult *DatabaseResult

	switch operation.Type {
	case "insert":
		dbResult, err = e.executeInsert(ctx, collection, operation)
	case "update":
		dbResult, err = e.executeUpdate(ctx, collection, operation)
	case "delete":
		dbResult, err = e.executeDelete(ctx, collection, operation)
	case "find":
		dbResult, err = e.executeFind(ctx, collection, operation)
	case "aggregate":
		dbResult, err = e.executeAggregate(ctx, collection, operation)
	default:
		err = fmt.Errorf("unsupported operation type: %s", operation.Type)
		dbResult = &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}
	}

	// Convertir DatabaseResult a StepResult
	stepResult := &StepResult{
		Success:  dbResult != nil && dbResult.Success,
		Duration: time.Since(startTime),
		Output: map[string]interface{}{
			"operation":      operation.Type,
			"collection":     operation.Collection,
			"execution_time": time.Since(startTime).String(),
		},
	}

	if dbResult != nil {
		if dbResult.ErrorMessage != "" {
			stepResult.ErrorMessage = dbResult.ErrorMessage
		}
		if dbResult.AffectedRows > 0 {
			stepResult.Output["affected_rows"] = dbResult.AffectedRows
		}
		if dbResult.InsertedID != nil {
			stepResult.Output["inserted_id"] = dbResult.InsertedID
		}
		if dbResult.Documents != nil {
			stepResult.Output["documents"] = dbResult.Documents
		}
		if dbResult.Metadata != nil {
			for k, v := range dbResult.Metadata {
				stepResult.Output[k] = v
			}
		}
	}

	if err != nil {
		stepResult.Success = false
		stepResult.ErrorMessage = err.Error()
	}

	e.logger.Info("Database action completed",
		zap.String("operation", operation.Type),
		zap.String("collection", operation.Collection),
		zap.Bool("success", stepResult.Success),
		zap.Duration("duration", stepResult.Duration))

	return stepResult, err
}

// parseOperation extrae la operación de la configuración del paso
func (e *DatabaseActionExecutor) parseOperation(config map[string]interface{}) (*DatabaseOperation, error) {
	operation := &DatabaseOperation{}

	// Tipo de operación (requerido)
	if opType, ok := config["operation"]; ok {
		if typeStr, ok := opType.(string); ok {
			operation.Type = typeStr
		} else {
			return nil, fmt.Errorf("operation type must be a string")
		}
	} else {
		return nil, fmt.Errorf("operation type is required")
	}

	// Colección (requerida)
	if coll, ok := config["collection"]; ok {
		if collStr, ok := coll.(string); ok {
			operation.Collection = collStr
		} else {
			return nil, fmt.Errorf("collection must be a string")
		}
	} else {
		return nil, fmt.Errorf("collection is required")
	}

	// Documento (para insert)
	if doc, ok := config["document"]; ok {
		if docMap, ok := doc.(map[string]interface{}); ok {
			operation.Document = docMap
		}
	}

	// Filtro (para update, delete, find)
	if filter, ok := config["filter"]; ok {
		if filterMap, ok := filter.(map[string]interface{}); ok {
			operation.Filter = filterMap
		}
	}

	// Update (para update)
	if update, ok := config["update"]; ok {
		if updateMap, ok := update.(map[string]interface{}); ok {
			operation.Update = updateMap
		}
	}

	// Pipeline (para aggregate)
	if pipeline, ok := config["pipeline"]; ok {
		if pipelineArray, ok := pipeline.([]interface{}); ok {
			operation.Pipeline = pipelineArray
		}
	}

	// Opciones (opcional)
	if options, ok := config["options"]; ok {
		if optionsMap, ok := options.(map[string]interface{}); ok {
			operation.Options = optionsMap
		}
	}

	return operation, nil
}

// validateOperation valida que la operación tenga los campos requeridos
func (e *DatabaseActionExecutor) validateOperation(op *DatabaseOperation) error {
	if op.Type == "" {
		return fmt.Errorf("operation type cannot be empty")
	}

	if op.Collection == "" {
		return fmt.Errorf("collection cannot be empty")
	}

	switch op.Type {
	case "insert":
		if op.Document == nil || len(op.Document) == 0 {
			return fmt.Errorf("document is required for insert operation")
		}
	case "update":
		if op.Filter == nil || len(op.Filter) == 0 {
			return fmt.Errorf("filter is required for update operation")
		}
		if op.Update == nil || len(op.Update) == 0 {
			return fmt.Errorf("update is required for update operation")
		}
	case "delete":
		if op.Filter == nil || len(op.Filter) == 0 {
			return fmt.Errorf("filter is required for delete operation")
		}
	case "aggregate":
		if op.Pipeline == nil || len(op.Pipeline) == 0 {
			return fmt.Errorf("pipeline is required for aggregate operation")
		}
	case "find":
		// find puede tener filtro vacío (obtener todos los documentos)
	default:
		return fmt.Errorf("unsupported operation type: %s", op.Type)
	}

	return nil
}

// executeInsert ejecuta una operación de inserción
func (e *DatabaseActionExecutor) executeInsert(ctx context.Context, collection *mongo.Collection, op *DatabaseOperation) (*DatabaseResult, error) {
	// Agregar timestamp automático si no existe
	if _, exists := op.Document["created_at"]; !exists {
		op.Document["created_at"] = time.Now()
	}

	result, err := collection.InsertOne(ctx, op.Document)
	if err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	return &DatabaseResult{
		Success:      true,
		AffectedRows: 1,
		InsertedID:   result.InsertedID,
		Metadata: map[string]interface{}{
			"inserted_document_id": result.InsertedID,
		},
	}, nil
}

// executeUpdate ejecuta una operación de actualización
func (e *DatabaseActionExecutor) executeUpdate(ctx context.Context, collection *mongo.Collection, op *DatabaseOperation) (*DatabaseResult, error) {
	// Agregar timestamp automático
	if update, ok := op.Update["$set"].(map[string]interface{}); ok {
		update["updated_at"] = time.Now()
	} else {
		// Si no hay $set, crearlo
		if op.Update["$set"] == nil {
			op.Update["$set"] = make(map[string]interface{})
		}
		if setMap, ok := op.Update["$set"].(map[string]interface{}); ok {
			setMap["updated_at"] = time.Now()
		}
	}

	result, err := collection.UpdateMany(ctx, op.Filter, op.Update)
	if err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	return &DatabaseResult{
		Success:      true,
		AffectedRows: result.ModifiedCount,
		Metadata: map[string]interface{}{
			"matched_count":  result.MatchedCount,
			"modified_count": result.ModifiedCount,
		},
	}, nil
}

// executeDelete ejecuta una operación de eliminación
func (e *DatabaseActionExecutor) executeDelete(ctx context.Context, collection *mongo.Collection, op *DatabaseOperation) (*DatabaseResult, error) {
	result, err := collection.DeleteMany(ctx, op.Filter)
	if err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	return &DatabaseResult{
		Success:      true,
		AffectedRows: result.DeletedCount,
		Metadata: map[string]interface{}{
			"deleted_count": result.DeletedCount,
		},
	}, nil
}

// executeFind ejecuta una operación de búsqueda
func (e *DatabaseActionExecutor) executeFind(ctx context.Context, collection *mongo.Collection, op *DatabaseOperation) (*DatabaseResult, error) {
	filter := op.Filter
	if filter == nil {
		filter = bson.M{}
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}
	defer cursor.Close(ctx)

	var documents []interface{}
	if err := cursor.All(ctx, &documents); err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	return &DatabaseResult{
		Success:   true,
		Documents: documents,
		Metadata: map[string]interface{}{
			"document_count": len(documents),
		},
	}, nil
}

// executeAggregate ejecuta una operación de agregación
func (e *DatabaseActionExecutor) executeAggregate(ctx context.Context, collection *mongo.Collection, op *DatabaseOperation) (*DatabaseResult, error) {
	cursor, err := collection.Aggregate(ctx, op.Pipeline)
	if err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}
	defer cursor.Close(ctx)

	var documents []interface{}
	if err := cursor.All(ctx, &documents); err != nil {
		return &DatabaseResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	return &DatabaseResult{
		Success:   true,
		Documents: documents,
		Metadata: map[string]interface{}{
			"document_count":  len(documents),
			"pipeline_stages": len(op.Pipeline),
		},
	}, nil
}

// TestConnection prueba la conexión a la base de datos
func (e *DatabaseActionExecutor) TestConnection(ctx context.Context) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return e.client.Ping(ctxTimeout, nil)
}

// GetStats obtiene estadísticas del ejecutor de base de datos
func (e *DatabaseActionExecutor) GetStats(ctx context.Context) (map[string]interface{}, error) {
	// Obtener estadísticas de la base de datos
	database := e.client.Database(e.databaseName)

	// Obtener información básica
	stats := map[string]interface{}{
		"database_name":        e.databaseName,
		"connection_status":    "connected",
		"supported_operations": []string{"insert", "update", "delete", "find", "aggregate"},
		"last_check":           time.Now(),
	}

	// Intentar obtener estadísticas de la base de datos
	var dbStats bson.M
	err := database.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&dbStats)
	if err == nil {
		stats["db_stats"] = dbStats
	}

	return stats, nil
}

// GetType retorna el tipo de ejecutor
func (e *DatabaseActionExecutor) GetType() string {
	return "database"
}

// IsEnabled retorna si el ejecutor está habilitado
func (e *DatabaseActionExecutor) IsEnabled() bool {
	return e.client != nil
}
