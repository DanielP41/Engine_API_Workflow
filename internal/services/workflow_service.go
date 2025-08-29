package services

import (
	"context"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowService interface {
	Create(ctx context.Context, req *models.CreateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error)
	GetByID(ctx context.Context, workflowID primitive.ObjectID) (*models.Workflow, error)
	Search(ctx context.Context, filters repository.WorkflowSearchFilters, opts repository.PaginationOptions) ([]models.Workflow, int64, error)
	Update(ctx context.Context, workflowID primitive.ObjectID, req *models.UpdateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error)
	Delete(ctx context.Context, workflowID primitive.ObjectID) error
	ToggleStatus(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID) (*models.WorkflowResponse, error)
	Clone(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID, name, description string) (*models.WorkflowResponse, error)
	GetUserWorkflows(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.Workflow, int64, error)
	GetActiveWorkflows(ctx context.Context, opts repository.PaginationOptions) ([]models.Workflow, int64, error)
	ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error
}

type workflowService struct {
	workflowRepo repository.WorkflowRepository
	userRepo     repository.UserRepository
}

func NewWorkflowService(workflowRepo repository.WorkflowRepository, userRepo repository.UserRepository) WorkflowService {
	return &workflowService{
		workflowRepo: workflowRepo,
		userRepo:     userRepo,
	}
}

func (s *workflowService) Create(ctx context.Context, req *models.CreateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	// Verificar que el usuario existe
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Crear el workflow
	workflow := &models.Workflow{
		ID:          primitive.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		UserID:      userID,
		Username:    user.GetFullName(),
		Status:      models.WorkflowStatusDraft,
		Steps:       req.Steps,
		Triggers:    req.Triggers,
		Tags:        req.Tags,
		Category:    req.Category,
		Priority:    req.Priority,
		CreatedBy:   userID,
	}

	// Establecer valores por defecto
	workflow.SetDefaults()

	// Aplicar configuraciones opcionales
	if req.TimeoutMinutes > 0 {
		workflow.TimeoutMinutes = req.TimeoutMinutes
	}
	if req.RetryAttempts >= 0 {
		workflow.RetryAttempts = req.RetryAttempts
	}
	if req.RetryDelayMs > 0 {
		workflow.RetryDelayMs = req.RetryDelayMs
	}
	if req.MaxConcurrent > 0 {
		workflow.MaxConcurrent = req.MaxConcurrent
	}
	if req.Environment != "" {
		workflow.Environment = req.Environment
	}
	if req.Configuration != nil {
		workflow.Configuration = req.Configuration
	}

	// Validar el workflow
	if err := s.ValidateWorkflow(ctx, workflow); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	// Guardar en la base de datos
	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Convertir a response
	response := s.toWorkflowResponse(workflow)
	return response, nil
}

func (s *workflowService) GetByID(ctx context.Context, workflowID primitive.ObjectID) (*models.Workflow, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	return workflow, nil
}

func (s *workflowService) Search(ctx context.Context, filters repository.WorkflowSearchFilters, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	workflows, total, err := s.workflowRepo.Search(ctx, filters, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search workflows: %w", err)
	}
	return workflows, total, nil
}

func (s *workflowService) Update(ctx context.Context, workflowID primitive.ObjectID, req *models.UpdateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	// Obtener el workflow existente
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	if workflow == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	// Preparar el update bson
	update := bson.M{}

	// Actualizar campos si están presentes
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}
	if req.Steps != nil {
		update["steps"] = req.Steps
	}
	if req.Triggers != nil {
		update["triggers"] = req.Triggers
	}
	if req.Tags != nil {
		update["tags"] = req.Tags
	}
	if req.Category != nil {
		update["category"] = *req.Category
	}
	if req.Priority != nil {
		update["priority"] = *req.Priority
	}
	if req.TimeoutMinutes != nil {
		update["timeout_minutes"] = *req.TimeoutMinutes
	}
	if req.RetryAttempts != nil {
		update["retry_attempts"] = *req.RetryAttempts
	}
	if req.RetryDelayMs != nil {
		update["retry_delay_ms"] = *req.RetryDelayMs
	}
	if req.MaxConcurrent != nil {
		update["max_concurrent"] = *req.MaxConcurrent
	}
	if req.Environment != nil {
		update["environment"] = *req.Environment
	}
	if req.Configuration != nil {
		update["configuration"] = req.Configuration
	}

	// Agregar metadatos de actualización
	update["updated_at"] = time.Now()
	update["last_edited_by"] = userID

	// Incrementar versión si hay cambios significativos
	if req.Steps != nil || req.Triggers != nil {
		workflow.IncrementVersion()
		update["version"] = workflow.Version
	}

	// Actualizar en la base de datos
	if err := s.workflowRepo.Update(ctx, workflowID, update); err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	// Obtener el workflow actualizado
	updatedWorkflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated workflow: %w", err)
	}

	// Convertir a response
	response := s.toWorkflowResponse(updatedWorkflow)
	return response, nil
}

func (s *workflowService) Delete(ctx context.Context, workflowID primitive.ObjectID) error {
	// Verificar que el workflow existe
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}
	if workflow == nil {
		return fmt.Errorf("workflow not found")
	}

	// Soft delete
	if err := s.workflowRepo.Delete(ctx, workflowID); err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	return nil
}

func (s *workflowService) ToggleStatus(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	// Obtener el workflow actual
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	if workflow == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	// Determinar nuevo estado
	var newStatus models.WorkflowStatus
	if workflow.Status == models.WorkflowStatusActive {
		newStatus = models.WorkflowStatusInactive
	} else {
		newStatus = models.WorkflowStatusActive

		// Validar antes de activar
		if err := s.ValidateWorkflow(ctx, workflow); err != nil {
			return nil, fmt.Errorf("cannot activate invalid workflow: %w", err)
		}
	}

	// Actualizar el estado
	update := bson.M{
		"status":         newStatus,
		"updated_at":     time.Now(),
		"last_edited_by": userID,
	}

	if err := s.workflowRepo.Update(ctx, workflowID, update); err != nil {
		return nil, fmt.Errorf("failed to update workflow status: %w", err)
	}

	// Obtener el workflow actualizado
	updatedWorkflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated workflow: %w", err)
	}

	response := s.toWorkflowResponse(updatedWorkflow)
	return response, nil
}

func (s *workflowService) Clone(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID, name, description string) (*models.WorkflowResponse, error) {
	// Obtener el workflow original
	originalWorkflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get original workflow: %w", err)
	}
	if originalWorkflow == nil {
		return nil, fmt.Errorf("original workflow not found")
	}

	// Obtener información del usuario
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Crear el workflow clonado
	clonedWorkflow := &models.Workflow{
		ID:             primitive.NewObjectID(),
		Name:           name,
		Description:    description,
		UserID:         userID,
		Username:       user.GetFullName(),
		Status:         models.WorkflowStatusDraft, // Siempre empezar como draft
		Steps:          originalWorkflow.Steps,
		Triggers:       originalWorkflow.Triggers,
		Tags:           originalWorkflow.Tags,
		Category:       originalWorkflow.Category,
		Priority:       originalWorkflow.Priority,
		TimeoutMinutes: originalWorkflow.TimeoutMinutes,
		RetryAttempts:  originalWorkflow.RetryAttempts,
		RetryDelayMs:   originalWorkflow.RetryDelayMs,
		MaxConcurrent:  originalWorkflow.MaxConcurrent,
		Environment:    originalWorkflow.Environment,
		Configuration:  originalWorkflow.Configuration,
		CreatedBy:      userID,
	}

	// Si no se proporciona nombre, usar el original con sufijo
	if name == "" {
		clonedWorkflow.Name = originalWorkflow.Name + " (Copy)"
	}

	// Si no se proporciona descripción, usar la original
	if description == "" {
		clonedWorkflow.Description = originalWorkflow.Description
	}

	// Establecer valores por defecto
	clonedWorkflow.SetDefaults()

	// Limpiar estadísticas (empezar con stats en cero)
	clonedWorkflow.Stats = models.WorkflowStats{}

	// Guardar en la base de datos
	if err := s.workflowRepo.Create(ctx, clonedWorkflow); err != nil {
		return nil, fmt.Errorf("failed to create cloned workflow: %w", err)
	}

	response := s.toWorkflowResponse(clonedWorkflow)
	return response, nil
}

func (s *workflowService) GetUserWorkflows(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	filters := repository.WorkflowSearchFilters{
		UserID: &userID,
	}

	return s.workflowRepo.Search(ctx, filters, opts)
}

func (s *workflowService) GetActiveWorkflows(ctx context.Context, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	status := models.WorkflowStatusActive
	filters := repository.WorkflowSearchFilters{
		Status: &status,
	}

	return s.workflowRepo.Search(ctx, filters, opts)
}

func (s *workflowService) ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	// Validaciones básicas
	if workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	if len(workflow.Triggers) == 0 {
		return fmt.Errorf("workflow must have at least one trigger")
	}

	// Validar pasos
	stepIDs := make(map[string]bool)
	for i, step := range workflow.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: ID is required", i)
		}

		if stepIDs[step.ID] {
			return fmt.Errorf("step %d: duplicate step ID '%s'", i, step.ID)
		}
		stepIDs[step.ID] = true

		if step.Name == "" {
			return fmt.Errorf("step %d: name is required", i)
		}

		if !models.ActionType(step.Type).IsValid() {
			return fmt.Errorf("step %d: invalid type '%s'", i, step.Type)
		}
	}

	// Validar triggers
	for i, trigger := range workflow.Triggers {
		if !models.TriggerType(trigger.Type).IsValid() {
			return fmt.Errorf("trigger %d: invalid type '%s'", i, trigger.Type)
		}
	}

	// Validar configuraciones
	if workflow.Priority < 1 || workflow.Priority > 5 {
		return fmt.Errorf("priority must be between 1 and 5")
	}

	if workflow.TimeoutMinutes < 1 || workflow.TimeoutMinutes > 1440 {
		return fmt.Errorf("timeout must be between 1 and 1440 minutes")
	}

	if workflow.RetryAttempts < 0 || workflow.RetryAttempts > 10 {
		return fmt.Errorf("retry attempts must be between 0 and 10")
	}

	if workflow.MaxConcurrent < 1 || workflow.MaxConcurrent > 100 {
		return fmt.Errorf("max concurrent must be between 1 and 100")
	}

	return nil
}

// toWorkflowResponse convierte un modelo Workflow a WorkflowResponse
func (s *workflowService) toWorkflowResponse(workflow *models.Workflow) *models.WorkflowResponse {
	if workflow == nil {
		return nil
	}

	// Convertir steps a ExecutedWorkflowStep
	executedSteps := make([]models.ExecutedWorkflowStep, len(workflow.Steps))
	for i, step := range workflow.Steps {
		executedSteps[i] = models.ExecutedWorkflowStep{
			ID:     step.ID,
			Name:   step.Name,
			Type:   models.ActionType(step.Type),
			Config: step.Config,
			Order:  i,
			Status: models.WorkflowStatusDraft, // Estado inicial
		}
	}

	return &models.WorkflowResponse{
		ID:             workflow.ID,
		Name:           workflow.Name,
		Description:    workflow.Description,
		UserID:         workflow.UserID,
		Username:       workflow.Username,
		Status:         workflow.Status,
		Steps:          executedSteps,
		Triggers:       workflow.Triggers,
		Tags:           workflow.Tags,
		Category:       workflow.Category,
		Priority:       workflow.Priority,
		Version:        workflow.Version,
		VersionName:    workflow.VersionName,
		TimeoutMinutes: workflow.TimeoutMinutes,
		RetryAttempts:  workflow.RetryAttempts,
		RetryDelayMs:   workflow.RetryDelayMs,
		MaxConcurrent:  workflow.MaxConcurrent,
		Environment:    workflow.Environment,
		Stats:          workflow.Stats,
		CreatedAt:      workflow.CreatedAt,
		UpdatedAt:      workflow.UpdatedAt,
	}
}
