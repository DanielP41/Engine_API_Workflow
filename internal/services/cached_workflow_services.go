// internal/services/cached_workflow_service.go
package services

import (
	"context"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/cache"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// cachedWorkflowService implementa WorkflowService con sistema de caché
type cachedWorkflowService struct {
	baseService  WorkflowService
	workflowRepo repository.WorkflowRepository
	userRepo     repository.UserRepository
	cacheManager *cache.CacheManager
	logger       *zap.Logger

	// TTL configurations
	workflowTTL time.Duration
	searchTTL   time.Duration
	configTTL   time.Duration
}

// NewCachedWorkflowService crea un nuevo workflow service con caché
func NewCachedWorkflowService(
	baseService WorkflowService,
	workflowRepo repository.WorkflowRepository,
	userRepo repository.UserRepository,
	cacheManager *cache.CacheManager,
	logger *zap.Logger,
) WorkflowService {

	service := &cachedWorkflowService{
		baseService:  baseService,
		workflowRepo: workflowRepo,
		userRepo:     userRepo,
		cacheManager: cacheManager,
		logger:       logger,
		workflowTTL:  cache.TTLLong,     // 30 minutos para workflows
		searchTTL:    cache.TTLMedium,   // 5 minutos para búsquedas
		configTTL:    cache.TTLVeryLong, // 2 horas para configuraciones
	}

	// Configurar invalidación automática
	service.setupInvalidationHandlers()

	return service
}

// Create crea un workflow y actualiza caché
func (s *cachedWorkflowService) Create(ctx context.Context, req *models.CreateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	// Crear workflow usando servicio base
	response, err := s.baseService.Create(ctx, req, userID)
	if err != nil {
		return nil, err
	}

	// Invalidar cachés relacionados
	go s.invalidateWorkflowCaches(ctx, "", userID, "workflow_created")

	// Precargar el workflow recién creado en caché
	go func() {
		ctx := context.Background()
		key := cache.WorkflowKeys.BuildWithID(response.ID, "config")
		if err := s.cacheManager.Set(ctx, key, response, s.workflowTTL); err != nil {
			s.logger.Warn("Failed to cache new workflow", zap.String("workflow_id", response.ID), zap.Error(err))
		}
	}()

	s.logger.Info("Workflow created and cached", zap.String("workflow_id", response.ID))
	return response, nil
}

// GetByID obtiene workflow por ID con caché
func (s *cachedWorkflowService) GetByID(ctx context.Context, workflowID primitive.ObjectID) (*models.Workflow, error) {
	key := cache.WorkflowKeys.BuildWithID(workflowID.Hex(), "config")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.workflowTTL, func() (interface{}, error) {
		s.logger.Debug("Computing workflow config",
			zap.String("cache_key", key),
			zap.String("workflow_id", workflowID.Hex()))
		return s.baseService.GetByID(ctx, workflowID)
	})

	if err != nil {
		s.logger.Error("Failed to get workflow", zap.String("workflow_id", workflowID.Hex()), zap.Error(err))
		return nil, err
	}

	return result.(*models.Workflow), nil
}

// Search realiza búsqueda de workflows con caché
func (s *cachedWorkflowService) Search(ctx context.Context, filters repository.WorkflowSearchFilters, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	// Generar clave de caché basada en filtros y opciones
	key := s.buildSearchCacheKey(filters, opts)

	result, err := s.cacheManager.GetOrSet(ctx, key, s.searchTTL, func() (interface{}, error) {
		s.logger.Debug("Computing workflow search",
			zap.String("cache_key", key))

		workflows, total, err := s.baseService.Search(ctx, filters, opts)
		if err != nil {
			return nil, err
		}

		// Envolver resultado para caché
		return &searchResult{
			Workflows: workflows,
			Total:     total,
		}, nil
	})

	if err != nil {
		s.logger.Error("Failed to search workflows", zap.Error(err))
		return nil, 0, err
	}

	searchRes := result.(*searchResult)
	return searchRes.Workflows, searchRes.Total, nil
}

// Update actualiza workflow e invalida caché
func (s *cachedWorkflowService) Update(ctx context.Context, workflowID primitive.ObjectID, req *models.UpdateWorkflowRequest, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	// Actualizar usando servicio base
	response, err := s.baseService.Update(ctx, workflowID, req, userID)
	if err != nil {
		return nil, err
	}

	// Invalidar cachés específicos del workflow
	go s.invalidateWorkflowCaches(ctx, workflowID.Hex(), userID, "workflow_updated")

	// Actualizar caché con nueva versión
	go func() {
		ctx := context.Background()
		key := cache.WorkflowKeys.BuildWithID(workflowID.Hex(), "config")
		if err := s.cacheManager.Set(ctx, key, response, s.workflowTTL); err != nil {
			s.logger.Warn("Failed to update workflow cache", zap.String("workflow_id", workflowID.Hex()), zap.Error(err))
		}
	}()

	s.logger.Info("Workflow updated and cache refreshed", zap.String("workflow_id", workflowID.Hex()))
	return response, nil
}

// Delete elimina workflow e invalida caché
func (s *cachedWorkflowService) Delete(ctx context.Context, workflowID primitive.ObjectID) error {
	// Eliminar usando servicio base
	err := s.baseService.Delete(ctx, workflowID)
	if err != nil {
		return err
	}

	// Invalidar todos los cachés relacionados con este workflow
	go s.invalidateWorkflowCaches(ctx, workflowID.Hex(), primitive.NilObjectID, "workflow_deleted")

	s.logger.Info("Workflow deleted and cache invalidated", zap.String("workflow_id", workflowID.Hex()))
	return nil
}

// ToggleStatus cambia estado del workflow e invalida caché
func (s *cachedWorkflowService) ToggleStatus(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID) (*models.WorkflowResponse, error) {
	// Cambiar estado usando servicio base
	response, err := s.baseService.ToggleStatus(ctx, workflowID, userID)
	if err != nil {
		return nil, err
	}

	// Invalidar cachés relacionados
	go s.invalidateWorkflowCaches(ctx, workflowID.Hex(), userID, "workflow_status_changed")

	s.logger.Info("Workflow status toggled",
		zap.String("workflow_id", workflowID.Hex()),
		zap.String("new_status", string(response.Status)))
	return response, nil
}

// Clone clona workflow y actualiza caché
func (s *cachedWorkflowService) Clone(ctx context.Context, workflowID primitive.ObjectID, userID primitive.ObjectID, name, description string) (*models.WorkflowResponse, error) {
	// Clonar usando servicio base
	response, err := s.baseService.Clone(ctx, workflowID, userID, name, description)
	if err != nil {
		return nil, err
	}

	// Invalidar cachés de usuario para mostrar nuevo workflow
	go s.invalidateWorkflowCaches(ctx, "", userID, "workflow_cloned")

	// Precargar workflow clonado en caché
	go func() {
		ctx := context.Background()
		key := cache.WorkflowKeys.BuildWithID(response.ID, "config")
		if err := s.cacheManager.Set(ctx, key, response, s.workflowTTL); err != nil {
			s.logger.Warn("Failed to cache cloned workflow", zap.String("workflow_id", response.ID), zap.Error(err))
		}
	}()

	s.logger.Info("Workflow cloned and cached",
		zap.String("original_id", workflowID.Hex()),
		zap.String("cloned_id", response.ID))
	return response, nil
}

// GetUserWorkflows obtiene workflows de usuario con caché
func (s *cachedWorkflowService) GetUserWorkflows(ctx context.Context, userID primitive.ObjectID, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	key := cache.UserKeys.BuildWithID(userID.Hex(), "workflows", s.buildPaginationKey(opts))

	result, err := s.cacheManager.GetOrSet(ctx, key, s.searchTTL, func() (interface{}, error) {
		s.logger.Debug("Computing user workflows",
			zap.String("cache_key", key),
			zap.String("user_id", userID.Hex()))

		workflows, total, err := s.baseService.GetUserWorkflows(ctx, userID, opts)
		if err != nil {
			return nil, err
		}

		return &searchResult{
			Workflows: workflows,
			Total:     total,
		}, nil
	})

	if err != nil {
		s.logger.Error("Failed to get user workflows", zap.String("user_id", userID.Hex()), zap.Error(err))
		return nil, 0, err
	}

	searchRes := result.(*searchResult)
	return searchRes.Workflows, searchRes.Total, nil
}

// GetActiveWorkflows obtiene workflows activos con caché
func (s *cachedWorkflowService) GetActiveWorkflows(ctx context.Context, opts repository.PaginationOptions) ([]models.Workflow, int64, error) {
	key := cache.WorkflowKeys.Build("active", s.buildPaginationKey(opts))

	result, err := s.cacheManager.GetOrSet(ctx, key, s.searchTTL, func() (interface{}, error) {
		s.logger.Debug("Computing active workflows", zap.String("cache_key", key))

		workflows, total, err := s.baseService.GetActiveWorkflows(ctx, opts)
		if err != nil {
			return nil, err
		}

		return &searchResult{
			Workflows: workflows,
			Total:     total,
		}, nil
	})

	if err != nil {
		s.logger.Error("Failed to get active workflows", zap.Error(err))
		return nil, 0, err
	}

	searchRes := result.(*searchResult)
	return searchRes.Workflows, searchRes.Total, nil
}

// ValidateWorkflow valida workflow (sin caché por ser operación de validación)
func (s *cachedWorkflowService) ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	// Las validaciones no se cachean para garantizar consistencia
	return s.baseService.ValidateWorkflow(ctx, workflow)
}

// Métodos auxiliares y cache management

// searchResult estructura para cachear resultados de búsqueda
type searchResult struct {
	Workflows []models.Workflow `json:"workflows"`
	Total     int64             `json:"total"`
}

// buildSearchCacheKey construye clave de caché para búsquedas
func (s *cachedWorkflowService) buildSearchCacheKey(filters repository.WorkflowSearchFilters, opts repository.PaginationOptions) string {
	keyBuilder := cache.WorkflowKeys

	parts := []string{"search"}

	// Agregar filtros
	if filters.UserID != nil {
		parts = append(parts, fmt.Sprintf("user_%s", filters.UserID.Hex()))
	}
	if filters.Status != nil {
		parts = append(parts, fmt.Sprintf("status_%s", string(*filters.Status)))
	}
	if filters.Query != nil && *filters.Query != "" {
		// Crear hash de la query para evitar claves muy largas
		parts = append(parts, fmt.Sprintf("q_%x", hashString(*filters.Query)))
	}
	if filters.Tags != nil && len(filters.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags_%x", hashSlice(filters.Tags)))
	}
	if filters.Category != nil && *filters.Category != "" {
		parts = append(parts, fmt.Sprintf("cat_%s", *filters.Category))
	}

	// Agregar paginación
	parts = append(parts, s.buildPaginationKey(opts))

	return keyBuilder.Build(parts...)
}

// buildPaginationKey construye clave para paginación
func (s *cachedWorkflowService) buildPaginationKey(opts repository.PaginationOptions) string {
	return fmt.Sprintf("page_%d_size_%d_sort_%s", opts.Page, opts.PageSize, opts.SortBy)
}

// invalidateWorkflowCaches invalida cachés relacionados con workflows
func (s *cachedWorkflowService) invalidateWorkflowCaches(ctx context.Context, workflowID string, userID primitive.ObjectID, reason string) {
	patterns := []string{
		cache.WorkflowKeys.Build("search", "*"), // Búsquedas
		cache.WorkflowKeys.Build("active", "*"), // Workflows activos
		cache.DashboardKeys.Build("*"),          // Dashboard data
		cache.MetricsKeys.Build("*"),            // Métricas
	}

	// Si es un workflow específico, invalidar sus cachés
	if workflowID != "" {
		patterns = append(patterns, cache.WorkflowKeys.BuildWithID(workflowID, "*"))
	}

	// Si es de un usuario específico, invalidar sus cachés
	if userID != primitive.NilObjectID {
		patterns = append(patterns, cache.UserKeys.BuildWithID(userID.Hex(), "workflows", "*"))
	}

	// Ejecutar invalidaciones
	for _, pattern := range patterns {
		if err := s.cacheManager.InvalidatePattern(ctx, pattern, reason); err != nil {
			s.logger.Warn("Failed to invalidate cache pattern",
				zap.String("pattern", pattern),
				zap.String("reason", reason),
				zap.Error(err))
		}
	}

	s.logger.Debug("Workflow caches invalidated",
		zap.String("workflow_id", workflowID),
		zap.String("user_id", userID.Hex()),
		zap.String("reason", reason),
		zap.Int("patterns", len(patterns)))
}

// setupInvalidationHandlers configura manejadores de invalidación automática
func (s *cachedWorkflowService) setupInvalidationHandlers() {
	// Configurar warmup para workflows críticos
	s.setupWorkflowWarmup()

	s.logger.Info("Workflow cache invalidation handlers configured")
}

// setupWorkflowWarmup configura precalentamiento para workflows críticos
func (s *cachedWorkflowService) setupWorkflowWarmup() {
	// Warmup para workflows activos más utilizados
	warmupTask := cache.WarmupTask{
		Name: "active_workflows",
		Key:  cache.WorkflowKeys.Build("active", "page_1_size_20_sort_updated_at"),
		TTL:  s.searchTTL,
		Fetcher: func(ctx context.Context) (interface{}, error) {
			workflows, total, err := s.baseService.GetActiveWorkflows(ctx, repository.PaginationOptions{
				Page:     1,
				PageSize: 20,
				SortBy:   "updated_at",
				SortDesc: true,
			})
			if err != nil {
				return nil, err
			}
			return &searchResult{Workflows: workflows, Total: total}, nil
		},
		Schedule: 2 * time.Minute,
		Priority: 2,
	}

	s.cacheManager.AddWarmupTask(warmupTask)
	s.logger.Info("Workflow warmup task configured")
}

// GetWorkflowForExecution obtiene workflow optimizado para ejecución
func (s *cachedWorkflowService) GetWorkflowForExecution(ctx context.Context, workflowID primitive.ObjectID) (*models.Workflow, error) {
	// Caché especial para ejecución con TTL más largo y prioridad alta
	key := cache.WorkflowKeys.BuildWithID(workflowID.Hex(), "execution")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.configTTL, func() (interface{}, error) {
		s.logger.Debug("Computing workflow for execution",
			zap.String("cache_key", key),
			zap.String("workflow_id", workflowID.Hex()))

		workflow, err := s.baseService.GetByID(ctx, workflowID)
		if err != nil {
			return nil, err
		}

		// Validar que el workflow está activo y es ejecutable
		if workflow.Status != models.WorkflowStatusActive {
			return nil, fmt.Errorf("workflow is not active: %s", workflow.Status)
		}

		return workflow, nil
	})

	if err != nil {
		s.logger.Error("Failed to get workflow for execution",
			zap.String("workflow_id", workflowID.Hex()),
			zap.Error(err))
		return nil, err
	}

	return result.(*models.Workflow), nil
}

// InvalidateWorkflow invalida cachés específicos de un workflow
func (s *cachedWorkflowService) InvalidateWorkflow(ctx context.Context, workflowID primitive.ObjectID, reason string) error {
	s.invalidateWorkflowCaches(ctx, workflowID.Hex(), primitive.NilObjectID, reason)
	return nil
}

// GetCacheStats obtiene estadísticas del caché de workflows
func (s *cachedWorkflowService) GetCacheStats(ctx context.Context) (*cache.CacheStats, error) {
	return s.cacheManager.GetStats(ctx)
}

// PreloadWorkflow precarga un workflow en caché
func (s *cachedWorkflowService) PreloadWorkflow(ctx context.Context, workflowID primitive.ObjectID) error {
	// Precargar tanto para configuración como para ejecución
	keys := []string{
		cache.WorkflowKeys.BuildWithID(workflowID.Hex(), "config"),
		cache.WorkflowKeys.BuildWithID(workflowID.Hex(), "execution"),
	}

	workflow, err := s.baseService.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if err := s.cacheManager.Set(ctx, key, workflow, s.workflowTTL); err != nil {
			s.logger.Warn("Failed to preload workflow cache",
				zap.String("key", key),
				zap.String("workflow_id", workflowID.Hex()),
				zap.Error(err))
		}
	}

	s.logger.Info("Workflow preloaded in cache", zap.String("workflow_id", workflowID.Hex()))
	return nil
}

// Utility functions

// hashString crea un hash simple de una string para claves de caché
func hashString(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h *= 16777619
		h ^= uint32(s[i])
	}
	return h
}

// hashSlice crea un hash simple de un slice de strings
func hashSlice(slice []string) uint32 {
	h := uint32(2166136261)
	for _, s := range slice {
		for i := 0; i < len(s); i++ {
			h *= 16777619
			h ^= uint32(s[i])
		}
		h *= 16777619
		h ^= uint32(',') // separador
	}
	return h
}
