package repository

import (
	"Engine_API_Workflow/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Métodos adicionales para WorkflowRepository
func (r *WorkflowRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Workflow, error) {
	// Alias para GetByID
	return r.GetByID(ctx, id)
}

// Métodos adicionales para LogRepository
func (r *LogRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.WorkflowLog, error) {
	// Alias para GetByID
	return r.GetByID(ctx, id)
}

// Update con log específico
func (r *LogRepository) Update(ctx context.Context, log *models.WorkflowLog) error {
	// Convertir a map para usar el método Update existente
	updates := map[string]interface{}{
		"status":     log.Status,
		"error":      log.ErrorMessage,
		"updated_at": log.UpdatedAt,
	}
	return r.Update(ctx, log.ID, updates)
}
