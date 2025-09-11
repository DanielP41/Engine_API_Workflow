// internal/repository/mongodb/metrics_repository.go
package mongodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
)

type metricsRepository struct {
	logCollection      *mongo.Collection
	workflowCollection *mongo.Collection
	userCollection     *mongo.Collection
	db                 *mongo.Database
}

// NewMetricsRepository crea una nueva instancia del repositorio de métricas
func NewMetricsRepository(db *mongo.Database) repository.MetricsRepository {
	return &metricsRepository{
		logCollection:      db.Collection("logs"),
		workflowCollection: db.Collection("workflows"),
		userCollection:     db.Collection("users"),
		db:                 db,
	}
}

// GetExecutionStats obtiene estadísticas de ejecuciones
func (r *metricsRepository) GetExecutionStats(ctx context.Context, filter repository.LogSearchFilter) (*models.LogStats, error) {
	pipeline := []bson.M{}

	// Agregar filtros al pipeline
	matchStage := bson.M{}

	if filter.WorkflowID != nil {
		matchStage["workflow_id"] = *filter.WorkflowID
	}

	if filter.UserID != nil {
		matchStage["user_id"] = *filter.UserID
	}

	if filter.StartDate != nil || filter.EndDate != nil {
		dateFilter := bson.M{}
		if filter.StartDate != nil {
			dateFilter["$gte"] = *filter.StartDate
		}
		if filter.EndDate != nil {
			dateFilter["$lte"] = *filter.EndDate
		}
		if len(dateFilter) > 0 {
			matchStage["created_at"] = dateFilter
		}
	}

	if len(matchStage) > 0 {
		pipeline = append(pipeline, bson.M{"$match": matchStage})
	}

	// Agregar etapa de agrupación para estadísticas
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id":              nil,
			"total_executions": bson.M{"$sum": 1},
			"successful_runs": bson.M{
				"$sum": bson.M{
					"$cond": bson.M{
						"if":   bson.M{"$eq": []interface{}{"$status", "completed"}},
						"then": 1,
						"else": 0,
					},
				},
			},
			"failed_runs": bson.M{
				"$sum": bson.M{
					"$cond": bson.M{
						"if":   bson.M{"$eq": []interface{}{"$status", "failed"}},
						"then": 1,
						"else": 0,
					},
				},
			},
			"avg_execution_time": bson.M{
				"$avg": "$duration",
			},
		},
	})

	cursor, err := r.logCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate execution stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode execution stats: %w", err)
	}

	if len(results) == 0 {
		return &models.LogStats{
			TotalExecutions:      0,
			SuccessfulRuns:       0,
			FailedRuns:           0,
			AverageExecutionTime: 0,
		}, nil
	}

	result := results[0]
	return &models.LogStats{
		TotalExecutions:      getInt64FromBSON(result, "total_executions"),
		SuccessfulRuns:       getInt64FromBSON(result, "successful_runs"),
		FailedRuns:           getInt64FromBSON(result, "failed_runs"),
		AverageExecutionTime: getFloat64FromBSON(result, "avg_execution_time"),
	}, nil
}

// GetTimeSeriesData obtiene datos de series temporales para gráficos
func (r *metricsRepository) GetTimeSeriesData(ctx context.Context, metric string, timeRange time.Duration, intervals int) ([]models.TimeSeriesPoint, error) {
	now := time.Now()
	startTime := now.Add(-timeRange)
	intervalDuration := timeRange / time.Duration(intervals)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{
					"$gte": startTime,
					"$lte": now,
				},
			},
		},
	}

	// Agregar etapa de agrupación por intervalos de tiempo
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"$toDate": bson.M{
					"$subtract": []interface{}{
						"$created_at",
						bson.M{
							"$mod": []interface{}{
								bson.M{"$toLong": "$created_at"},
								int64(intervalDuration.Milliseconds()),
							},
						},
					},
				},
			},
			"count": bson.M{"$sum": 1},
			"success_count": bson.M{
				"$sum": bson.M{
					"$cond": bson.M{
						"if":   bson.M{"$eq": []interface{}{"$status", "completed"}},
						"then": 1,
						"else": 0,
					},
				},
			},
			"avg_duration": bson.M{"$avg": "$duration"},
		},
	})

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": 1}})

	cursor, err := r.logCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate time series data: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode time series data: %w", err)
	}

	points := make([]models.TimeSeriesPoint, 0, len(results))
	for _, result := range results {
		var value float64
		switch metric {
		case "executions":
			value = getFloat64FromBSON(result, "count")
		case "success_rate":
			total := getFloat64FromBSON(result, "count")
			success := getFloat64FromBSON(result, "success_count")
			if total > 0 {
				value = (success / total) * 100
			}
		case "avg_execution_time":
			value = getFloat64FromBSON(result, "avg_duration")
		}

		if timestamp, ok := result["_id"].(primitive.DateTime); ok {
			points = append(points, models.TimeSeriesPoint{
				Timestamp: timestamp.Time(),
				Value:     value,
			})
		}
	}

	return points, nil
}

// GetWorkflowDistribution obtiene distribución de workflows por ejecuciones
func (r *metricsRepository) GetWorkflowDistribution(ctx context.Context, timeRange time.Duration) ([]models.WorkflowCount, error) {
	startTime := time.Now().Add(-timeRange)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startTime},
			},
		},
		{
			"$group": bson.M{
				"_id":   "$workflow_id",
				"count": bson.M{"$sum": 1},
				"success_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$eq": []interface{}{"$status", "completed"}},
							"then": 1,
							"else": 0,
						},
					},
				},
				"workflow_name": bson.M{"$first": "$workflow_name"},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
		{
			"$limit": 10,
		},
	}

	cursor, err := r.logCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate workflow distribution: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode workflow distribution: %w", err)
	}

	// Calcular total para porcentajes
	var totalExecutions int64
	for _, result := range results {
		totalExecutions += getInt64FromBSON(result, "count")
	}

	distribution := make([]models.WorkflowCount, 0, len(results))
	for _, result := range results {
		count := getInt64FromBSON(result, "count")
		successCount := getInt64FromBSON(result, "success_count")

		var successRate float64
		if count > 0 {
			successRate = (float64(successCount) / float64(count)) * 100
		}

		var percentage float64
		if totalExecutions > 0 {
			percentage = (float64(count) / float64(totalExecutions)) * 100
		}

		workflowID := ""
		if id, ok := result["_id"].(primitive.ObjectID); ok {
			workflowID = id.Hex()
		}

		workflowName := getStringFromBSON(result, "workflow_name")

		distribution = append(distribution, models.WorkflowCount{
			WorkflowID:   workflowID,
			WorkflowName: workflowName,
			Count:        int(count),
			Percentage:   percentage,
			SuccessRate:  successRate,
		})
	}

	return distribution, nil
}

// GetTriggerDistribution obtiene distribución de triggers
func (r *metricsRepository) GetTriggerDistribution(ctx context.Context, timeRange time.Duration) ([]models.TriggerCount, error) {
	startTime := time.Now().Add(-timeRange)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startTime},
			},
		},
		{
			"$group": bson.M{
				"_id":   "$trigger_type",
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
	}

	cursor, err := r.logCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate trigger distribution: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode trigger distribution: %w", err)
	}

	// Calcular total para porcentajes
	var total int64
	for _, result := range results {
		total += getInt64FromBSON(result, "count")
	}

	distribution := make([]models.TriggerCount, 0, len(results))
	for _, result := range results {
		count := getInt64FromBSON(result, "count")

		var percentage float64
		if total > 0 {
			percentage = (float64(count) / float64(total)) * 100
		}

		triggerType := getStringFromBSON(result, "_id")

		distribution = append(distribution, models.TriggerCount{
			TriggerType: triggerType,
			Count:       int(count),
			Percentage:  percentage,
		})
	}

	return distribution, nil
}

// GetErrorDistribution obtiene distribución de errores
func (r *metricsRepository) GetErrorDistribution(ctx context.Context, timeRange time.Duration) ([]models.ErrorCount, error) {
	startTime := time.Now().Add(-timeRange)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at":    bson.M{"$gte": startTime},
				"status":        "failed",
				"error_message": bson.M{"$ne": ""},
			},
		},
		{
			"$group": bson.M{
				"_id":           "$error_message",
				"count":         bson.M{"$sum": 1},
				"last_occurred": bson.M{"$max": "$created_at"},
				"workflow_ids":  bson.M{"$addToSet": "$workflow_id"},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
		{
			"$limit": 10,
		},
	}

	cursor, err := r.logCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate error distribution: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode error distribution: %w", err)
	}

	// Calcular total para porcentajes
	var total int64
	for _, result := range results {
		total += getInt64FromBSON(result, "count")
	}

	distribution := make([]models.ErrorCount, 0, len(results))
	for _, result := range results {
		count := getInt64FromBSON(result, "count")

		var percentage float64
		if total > 0 {
			percentage = (float64(count) / float64(total)) * 100
		}

		errorMessage := getStringFromBSON(result, "_id")

		var lastOccurred time.Time
		if occurred, ok := result["last_occurred"].(primitive.DateTime); ok {
			lastOccurred = occurred.Time()
		}

		// Determinar tipo y severidad del error
		errorType := categorizeError(errorMessage)
		severity := determineSeverity(errorMessage, int(count))

		distribution = append(distribution, models.ErrorCount{
			ErrorType:    errorType,
			ErrorMessage: errorMessage,
			Count:        int(count),
			Percentage:   percentage,
			LastOccurred: lastOccurred,
			Severity:     severity,
		})
	}

	return distribution, nil
}

// GetHourlyStats obtiene estadísticas por hora
func (r *metricsRepository) GetHourlyStats(ctx context.Context, timeRange time.Duration) ([]models.HourlyStats, error) {
	startTime := time.Now().Add(-timeRange)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startTime},
			},
		},
		{
			"$group": bson.M{
				"_id":             bson.M{"$hour": "$created_at"},
				"execution_count": bson.M{"$sum": 1},
				"success_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$eq": []interface{}{"$status", "completed"}},
							"then": 1,
							"else": 0,
						},
					},
				},
				"failure_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$eq": []interface{}{"$status", "failed"}},
							"then": 1,
							"else": 0,
						},
					},
				},
				"avg_time": bson.M{"$avg": "$duration"},
				"max_time": bson.M{"$max": "$duration"},
			},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := r.logCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate hourly stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode hourly stats: %w", err)
	}

	stats := make([]models.HourlyStats, 0, len(results))
	for _, result := range results {
		hour := getIntFromBSON(result, "_id")
		executionCount := getIntFromBSON(result, "execution_count")
		successCount := getIntFromBSON(result, "success_count")
		failureCount := getIntFromBSON(result, "failure_count")
		avgTime := getFloat64FromBSON(result, "avg_time")
		maxTime := getFloat64FromBSON(result, "max_time")

		stats = append(stats, models.HourlyStats{
			Hour:           hour,
			ExecutionCount: executionCount,
			SuccessCount:   successCount,
			FailureCount:   failureCount,
			AverageTime:    avgTime,
			PeakTime:       maxTime,
		})
	}

	return stats, nil
}

// GetSystemMetrics obtiene métricas del sistema
func (r *metricsRepository) GetSystemMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Obtener estadísticas de la base de datos
	var dbStats bson.M
	if err := r.db.RunCommand(ctx, bson.D{{"dbStats", 1}}).Decode(&dbStats); err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}

	// Obtener estadísticas del servidor
	var serverStatus bson.M
	if err := r.db.RunCommand(ctx, bson.D{{"serverStatus", 1}}).Decode(&serverStatus); err != nil {
		return nil, fmt.Errorf("failed to get server status: %w", err)
	}

	metrics["database"] = map[string]interface{}{
		"collections":  dbStats["collections"],
		"data_size":    dbStats["dataSize"],
		"storage_size": dbStats["storageSize"],
		"index_size":   dbStats["indexSize"],
		"objects":      dbStats["objects"],
	}

	if connections, ok := serverStatus["connections"]; ok {
		metrics["connections"] = connections
	}

	if mem, ok := serverStatus["mem"]; ok {
		metrics["memory"] = mem
	}

	return metrics, nil
}

// CheckDatabaseHealth verifica la salud de la base de datos
func (r *metricsRepository) CheckDatabaseHealth(ctx context.Context) error {
	// Ping la base de datos
	if err := r.db.Client().Ping(ctx, nil); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Verificar que las colecciones principales existan
	collections := []string{"users", "workflows", "logs", "queue_tasks"}

	for _, collName := range collections {
		coll := r.db.Collection(collName)

		// Intentar contar documentos para verificar acceso
		_, err := coll.CountDocuments(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("failed to access collection %s: %w", collName, err)
		}
	}

	return nil
}

// Helper functions para extraer valores de BSON
func getInt64FromBSON(doc bson.M, key string) int64 {
	if val, ok := doc[key]; ok {
		switch v := val.(type) {
		case int32:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getFloat64FromBSON(doc bson.M, key string) float64 {
	if val, ok := doc[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return 0
}

func getStringFromBSON(doc bson.M, key string) string {
	if val, ok := doc[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromBSON(doc bson.M, key string) int {
	return int(getInt64FromBSON(doc, key))
}

// categorizeError categoriza un error por tipo
func categorizeError(errorMessage string) string {
	errorMessage = strings.ToLower(errorMessage)

	if strings.Contains(errorMessage, "timeout") || strings.Contains(errorMessage, "time") {
		return "timeout"
	}
	if strings.Contains(errorMessage, "network") || strings.Contains(errorMessage, "connection") {
		return "network"
	}
	if strings.Contains(errorMessage, "validation") || strings.Contains(errorMessage, "invalid") {
		return "validation"
	}
	if strings.Contains(errorMessage, "permission") || strings.Contains(errorMessage, "auth") {
		return "authorization"
	}
	if strings.Contains(errorMessage, "database") || strings.Contains(errorMessage, "db") {
		return "database"
	}

	return "runtime"
}

// determineSeverity determina la severidad de un error
func determineSeverity(errorMessage string, count int) string {
	if count > 50 {
		return "critical"
	}
	if count > 10 {
		return "high"
	}
	if count > 5 {
		return "medium"
	}
	return "low"
}
