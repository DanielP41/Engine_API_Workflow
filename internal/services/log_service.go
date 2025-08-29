package services

import (
	"context"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LogService interface {
	CreateWorkflowLog(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID, triggerType models.TriggerType, triggerData map[string]interface{}) (*models.WorkflowLog, error)
	UpdateLogStatus(ctx context.Context, logID primitive.ObjectID, status models.WorkflowStatus, errorMessage string) error
	AddStepExecution(ctx context.Context, logID primitive.ObjectID, stepExecution models.StepExecution) error
	CompleteWorkflowLog(ctx context.Context, logID primitive.ObjectID, success bool, errorMessage string) error
	GetWorkflowLogs(ctx context.Context, workflowID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error)
	GetUserLogs(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error)
	GetWorkflowStats(ctx context.Context, workflowID primitive.ObjectID, days int) (*models.LogStats, error)
	SearchLogs(ctx context.Context, filter repository.LogSearchFilter, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error)
	GetLogByID(ctx context.Context, logID primitive.ObjectID) (*models.WorkflowLog, error)
	GetSystemStats(ctx context.Context) (*models.LogStatistics, error)
}

type logService struct {
	logRepo      repository.LogRepository
	workflowRepo repository.WorkflowRepository
	userRepo     repository.UserRepository
}

func NewLogService(logRepo repository.LogRepository, workflowRepo repository.WorkflowRepository, userRepo repository.UserRepository) LogService {
	return &logService{
		logRepo:      logRepo,
		workflowRepo: workflowRepo,
		userRepo:     userRepo,
	}
}

func (s *logService) CreateWorkflowLog(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID, triggerType models.TriggerType, triggerData map[string]interface{}) (*models.WorkflowLog, error) {
	// Obtener información del workflow
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	if workflow == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	// Crear el log
	now := time.Now()
	log := &models.WorkflowLog{
		ID:           primitive.NewObjectID(),
		WorkflowID:   workflowID,
		WorkflowName: workflow.Name,
		UserID:       userID,
		Status:       models.WorkflowStatusActive, // Estado inicial: en ejecución
		TriggerType:  triggerType,
		TriggerData:  triggerData,
		StartedAt:    now,
		Steps:        []models.StepExecution{},
		Context:      make(map[string]interface{}),
		Metadata: models.LogMetadata{
			Source:      string(triggerType),
			Version:     fmt.Sprintf("v%d", workflow.Version),
			Environment: workflow.Environment,
			Tags:        workflow.Tags,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Guardar en la base de datos
	if err := s.logRepo.Create(ctx, log); err != nil {
		return nil, fmt.Errorf("failed to create workflow log: %w", err)
	}

	return log, nil
}

func (s *logService) UpdateLogStatus(ctx context.Context, logID primitive.ObjectID, status models.WorkflowStatus, errorMessage string) error {
	update := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if errorMessage != "" {
		update["error_message"] = errorMessage
	}

	return s.logRepo.Update(ctx, logID, update)
}

func (s *logService) AddStepExecution(ctx context.Context, logID primitive.ObjectID, stepExecution models.StepExecution) error {
	// Obtener el log actual
	log, err := s.logRepo.GetByID(ctx, logID)
	if err != nil {
		return fmt.Errorf("failed to get log: %w", err)
	}
	if log == nil {
		return fmt.Errorf("log not found")
	}

	// Agregar la ejecución del paso
	log.Steps = append(log.Steps, stepExecution)

	// Actualizar en la base de datos
	update := map[string]interface{}{
		"steps":      log.Steps,
		"updated_at": time.Now(),
	}

	return s.logRepo.Update(ctx, logID, update)
}

func (s *logService) CompleteWorkflowLog(ctx context.Context, logID primitive.ObjectID, success bool, errorMessage string) error {
	now := time.Now()

	// Obtener el log para calcular la duración
	log, err := s.logRepo.GetByID(ctx, logID)
	if err != nil {
		return fmt.Errorf("failed to get log: %w", err)
	}
	if log == nil {
		return fmt.Errorf("log not found")
	}

	// Calcular duración
	duration := now.Sub(log.StartedAt).Milliseconds()

	// Determinar estado final
	var finalStatus models.WorkflowStatus
	if success {
		finalStatus = models.WorkflowStatus("completed")
	} else {
		finalStatus = models.WorkflowStatus("failed")
	}

	// Preparar actualización
	update := map[string]interface{}{
		"status":       finalStatus,
		"completed_at": now,
		"duration":     duration,
		"updated_at":   now,
	}

	if errorMessage != "" {
		update["error_message"] = errorMessage
	}

	// Actualizar el log
	if err := s.logRepo.Update(ctx, logID, update); err != nil {
		return fmt.Errorf("failed to complete workflow log: %w", err)
	}

	// Actualizar estadísticas del workflow
	if err := s.updateWorkflowStats(ctx, log.WorkflowID, success, duration); err != nil {
		// Log error pero no fallar la operación principal
		fmt.Printf("Warning: failed to update workflow stats: %v\n", err)
	}

	return nil
}

func (s *logService) GetWorkflowLogs(ctx context.Context, workflowID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	return s.logRepo.GetByWorkflowID(ctx, workflowID, opts)
}

func (s *logService) GetUserLogs(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	return s.logRepo.GetByUserID(ctx, userID, opts)
}

func (s *logService) GetWorkflowStats(ctx context.Context, workflowID primitive.ObjectID, days int) (*models.LogStats, error) {
	// Crear filtro por workflow y rango de fechas
	startDate := time.Now().AddDate(0, 0, -days)
	filter := repository.LogSearchFilter{
		WorkflowID: &workflowID,
		StartDate:  &startDate,
	}

	return s.logRepo.GetStats(ctx, filter)
}

func (s *logService) SearchLogs(ctx context.Context, filter repository.LogSearchFilter, opts repository.PaginationOptions) ([]models.WorkflowLog, int64, error) {
	return s.logRepo.Search(ctx, filter, opts)
}

func (s *logService) GetLogByID(ctx context.Context, logID primitive.ObjectID) (*models.WorkflowLog, error) {
	return s.logRepo.GetByID(ctx, logID)
}

func (s *logService) GetSystemStats(ctx context.Context) (*models.LogStatistics, error) {
	// Obtener estadísticas generales del sistema
	now := time.Now()

	// Estadísticas de diferentes períodos
	periods := map[string]time.Time{
		"today":      now.AddDate(0, 0, -1),
		"this_week":  now.AddDate(0, 0, -7),
		"this_month": now.AddDate(0, 0, -30),
	}

	stats := &models.LogStatistics{
		LastUpdated: now,
		DataPeriod:  "all_time",
	}

	// Obtener estadísticas básicas (todos los logs)
	filter := repository.LogSearchFilter{}
	basicStats, err := s.logRepo.GetStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get basic stats: %w", err)
	}

	// Mapear estadísticas básicas
	if basicStats != nil {
		stats.TotalExecutions = basicStats.TotalExecutions
		stats.SuccessfulRuns = basicStats.SuccessfulRuns
		stats.FailedRuns = basicStats.FailedRuns
		stats.AverageExecutionTime = basicStats.AverageExecutionTime

		// Calcular tasas
		if stats.TotalExecutions > 0 {
			stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions) * 100
			stats.FailureRate = float64(stats.FailedRuns) / float64(stats.TotalExecutions) * 100
		}

		// Mapear distribuciones
		for _, trigger := range basicStats.MostUsedTriggers {
			percentage := float64(trigger.Count) / float64(stats.TotalExecutions) * 100
			stats.TriggerDistribution = append(stats.TriggerDistribution, models.TriggerDistribution{
				TriggerType: trigger.Type,
				Count:       trigger.Count,
				Percentage:  percentage,
			})
		}

		for _, error := range basicStats.ErrorDistribution {
			percentage := float64(error.Count) / float64(stats.TotalExecutions) * 100
			stats.ErrorDistribution = append(stats.ErrorDistribution, models.ErrorDistribution{
				ErrorType:    "runtime_error",
				ErrorMessage: error.Error,
				Count:        error.Count,
				Percentage:   percentage,
				LastOccurred: now, // Simplificado - en producción deberías obtener la fecha real
			})
		}
	}

	// Obtener estadísticas por período
	for periodName, startDate := range periods {
		periodFilter := repository.LogSearchFilter{
			StartDate: &startDate,
		}

		periodStats, err := s.logRepo.GetStats(ctx, periodFilter)
		if err != nil {
			continue // Continuar con otros períodos si uno falla
		}

		if periodStats != nil {
			switch periodName {
			case "today":
				stats.ExecutionsToday = periodStats.TotalExecutions
			case "this_week":
				stats.ExecutionsThisWeek = periodStats.TotalExecutions
			case "this_month":
				stats.ExecutionsThisMonth = periodStats.TotalExecutions
			}
		}
	}

	// Generar estadísticas por hora (simplificado)
	stats.HourlyDistribution = s.generateHourlyStats()

	return stats, nil
}

// updateWorkflowStats actualiza las estadísticas del workflow después de una ejecución
func (s *logService) updateWorkflowStats(ctx context.Context, workflowID primitive.ObjectID, success bool, duration int64) error {
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow for stats update: %w", err)
	}
	if workflow == nil {
		return fmt.Errorf("workflow not found for stats update")
	}

	// Actualizar contadores
	workflow.Stats.TotalExecutions++
	if success {
		workflow.Stats.SuccessfulRuns++
		now := time.Now()
		workflow.Stats.LastSuccess = &now
	} else {
		workflow.Stats.FailedRuns++
		now := time.Now()
		workflow.Stats.LastFailure = &now
	}

	// Actualizar tiempo promedio de ejecución
	if workflow.Stats.TotalExecutions == 1 {
		workflow.Stats.AvgExecutionTimeMs = float64(duration)
	} else {
		// Fórmula para promedio incremental
		workflow.Stats.AvgExecutionTimeMs = (workflow.Stats.AvgExecutionTimeMs*float64(workflow.Stats.TotalExecutions-1) + float64(duration)) / float64(workflow.Stats.TotalExecutions)
	}

	// Actualizar última ejecución
	now := time.Now()
	workflow.Stats.LastExecutedAt = &now

	// Guardar en la base de datos
	update := map[string]interface{}{
		"stats":      workflow.Stats,
		"updated_at": now,
	}

	return s.workflowRepo.Update(ctx, workflowID, update)
}

// generateHourlyStats genera estadísticas por hora (simplificado para demo)
func (s *logService) generateHourlyStats() []models.HourlyStats {
	hourlyStats := make([]models.HourlyStats, 24)

	for i := 0; i < 24; i++ {
		hourlyStats[i] = models.HourlyStats{
			Hour:           i,
			ExecutionCount: 0, // En producción, calcular desde la base de datos
			SuccessCount:   0,
			FailureCount:   0,
			AverageTime:    0.0,
		}
	}

	return hourlyStats
}
