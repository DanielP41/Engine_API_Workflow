// internal/api/handlers/backup_handler.go
package handlers

import (
	"strings"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// BackupHandler maneja las operaciones relacionadas con backups
type BackupHandler struct {
	backupService services.BackupService
	logger        *zap.Logger
}

// NewBackupHandler crea un nuevo handler de backup
func NewBackupHandler(backupService services.BackupService, logger *zap.Logger) *BackupHandler {
	return &BackupHandler{
		backupService: backupService,
		logger:        logger,
	}
}

// CreateBackup crea un nuevo backup
// @Summary Crear backup
// @Description Crea un nuevo backup del sistema
// @Tags backup
// @Accept json
// @Produce json
// @Param type query string false "Tipo de backup" Enums(full,incremental,mongodb,redis,config) default(full)
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.BackupInfo}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/create [post]
func (h *BackupHandler) CreateBackup(c *fiber.Ctx) error {
	// Validar que el usuario tenga permisos de admin
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup creation attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can create backups")
	}

	// Parsear tipo de backup
	backupType := strings.ToLower(c.Query("type", "full"))

	// Validar tipo de backup
	validTypes := []string{"full", "incremental", "mongodb", "redis", "config"}
	isValid := false
	for _, t := range validTypes {
		if backupType == t {
			isValid = true
			break
		}
	}

	if !isValid {
		h.logger.Warn("Invalid backup type requested",
			zap.String("backup_type", backupType),
			zap.String("user_id", getUserID(c)))
		return utils.BadRequestResponse(c, "Invalid backup type. Valid types: "+strings.Join(validTypes, ", "))
	}

	// Log del inicio de backup
	h.logger.Info("Creating backup",
		zap.String("type", backupType),
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)),
		zap.String("ip_address", c.IP()))

	// Crear backup
	backupInfo, err := h.backupService.CreateBackup(c.Context(), services.BackupType(backupType))
	if err != nil {
		h.logger.Error("Failed to create backup",
			zap.String("backup_type", backupType),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		// Determinar tipo de error para respuesta apropiada
		if strings.Contains(err.Error(), "not enabled") {
			return utils.BadRequestResponse(c, "Backup system is not enabled")
		}
		if strings.Contains(err.Error(), "disk space") {
			return utils.BadRequestResponse(c, "Insufficient disk space for backup")
		}
		if strings.Contains(err.Error(), "permission") {
			return utils.ForbiddenResponse(c, "Insufficient permissions for backup operation")
		}

		return utils.InternalServerErrorResponse(c, "Failed to create backup")
	}

	// Log de éxito
	h.logger.Info("Backup created successfully",
		zap.String("backup_id", backupInfo.ID),
		zap.String("backup_type", backupType),
		zap.Duration("duration", backupInfo.Duration),
		zap.Int64("size_bytes", backupInfo.Size),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Backup created successfully", backupInfo)
}

// ListBackups lista todos los backups disponibles
// @Summary Listar backups
// @Description Obtiene una lista de todos los backups disponibles
// @Tags backup
// @Accept json
// @Produce json
// @Param page query int false "Número de página" default(1)
// @Param limit query int false "Límite por página" default(20)
// @Param type query string false "Filtrar por tipo de backup"
// @Param status query string false "Filtrar por estado"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=object{backups=[]models.BackupInfo,total=int,page=int,limit=int}}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/list [get]
func (h *BackupHandler) ListBackups(c *fiber.Ctx) error {
	// Parsear parámetros de paginación
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	backupType := c.Query("type", "")
	status := c.Query("status", "")

	// Validar parámetros
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	h.logger.Info("Listing backups",
		zap.Int("page", page),
		zap.Int("limit", limit),
		zap.String("type_filter", backupType),
		zap.String("status_filter", status),
		zap.String("user_id", getUserID(c)))

	// Obtener lista de backups
	allBackups, err := h.backupService.ListBackups(c.Context())
	if err != nil {
		h.logger.Error("Failed to list backups",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to list backups")
	}

	// Aplicar filtros
	var filteredBackups []models.BackupInfo
	for _, backup := range allBackups {
		// Filtrar por tipo
		if backupType != "" && backup.Type != backupType {
			continue
		}
		// Filtrar por estado
		if status != "" && backup.Status != status {
			continue
		}
		filteredBackups = append(filteredBackups, backup)
	}

	// Aplicar paginación
	total := len(filteredBackups)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		filteredBackups = []models.BackupInfo{}
	} else {
		if end > total {
			end = total
		}
		filteredBackups = filteredBackups[start:end]
	}

	// Preparar respuesta
	response := map[string]interface{}{
		"backups": filteredBackups,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"pages":   (total + limit - 1) / limit,
	}

	h.logger.Info("Backups listed successfully",
		zap.Int("total_found", total),
		zap.Int("page_size", len(filteredBackups)),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Backups retrieved successfully", response)
}

// GetBackupStatus obtiene el estado del sistema de backup
// @Summary Estado del sistema de backup
// @Description Obtiene el estado actual del sistema de backup
// @Tags backup
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.BackupStatus}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/status [get]
func (h *BackupHandler) GetBackupStatus(c *fiber.Ctx) error {
	h.logger.Info("Getting backup status", zap.String("user_id", getUserID(c)))

	status, err := h.backupService.GetBackupStatus(c.Context())
	if err != nil {
		h.logger.Error("Failed to get backup status",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get backup status")
	}

	return utils.SuccessResponse(c, "Backup status retrieved successfully", status)
}

// GetBackupInfo obtiene información detallada de un backup específico
// @Summary Información de backup específico
// @Description Obtiene información detallada de un backup por su ID
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "ID del backup"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.BackupInfo}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/{id} [get]
func (h *BackupHandler) GetBackupInfo(c *fiber.Ctx) error {
	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required")
	}

	h.logger.Info("Getting backup info",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	// Buscar el backup en la lista
	backups, err := h.backupService.ListBackups(c.Context())
	if err != nil {
		h.logger.Error("Failed to get backup info",
			zap.String("backup_id", backupID),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get backup information")
	}

	// Buscar backup específico
	for _, backup := range backups {
		if backup.ID == backupID {
			return utils.SuccessResponse(c, "Backup information retrieved successfully", backup)
		}
	}

	h.logger.Warn("Backup not found",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.NotFoundResponse(c, "Backup not found")
}

// DeleteBackup elimina un backup específico
// @Summary Eliminar backup
// @Description Elimina un backup específico del sistema
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "ID del backup"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/{id} [delete]
func (h *BackupHandler) DeleteBackup(c *fiber.Ctx) error {
	// Solo admins pueden eliminar backups
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup deletion attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can delete backups")
	}

	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required")
	}

	h.logger.Info("Deleting backup",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)),
		zap.String("ip_address", c.IP()))

	if err := h.backupService.DeleteBackup(c.Context(), backupID); err != nil {
		h.logger.Error("Failed to delete backup",
			zap.String("backup_id", backupID),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			return utils.NotFoundResponse(c, "Backup not found")
		}

		return utils.InternalServerErrorResponse(c, "Failed to delete backup")
	}

	h.logger.Info("Backup deleted successfully",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Backup deleted successfully", nil)
}

// RestoreBackup restaura un backup específico
// @Summary Restaurar backup
// @Description Restaura el sistema desde un backup específico
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "ID del backup"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/{id}/restore [post]
func (h *BackupHandler) RestoreBackup(c *fiber.Ctx) error {
	// Solo admins pueden restaurar backups
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup restoration attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can restore backups")
	}

	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required")
	}

	h.logger.Warn("CRITICAL: Backup restoration initiated",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)),
		zap.String("ip_address", c.IP()),
		zap.String("user_agent", c.Get("User-Agent")))

	if err := h.backupService.RestoreBackup(c.Context(), backupID); err != nil {
		h.logger.Error("Failed to restore backup",
			zap.String("backup_id", backupID),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			return utils.NotFoundResponse(c, "Backup not found")
		}
		if strings.Contains(err.Error(), "status") {
			return utils.BadRequestResponse(c, "Cannot restore backup with current status")
		}

		return utils.InternalServerErrorResponse(c, "Failed to restore backup")
	}

	h.logger.Info("Backup restoration initiated successfully",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Backup restoration initiated successfully", map[string]interface{}{
		"backup_id": backupID,
		"message":   "Restoration process started. Please monitor application logs for progress.",
		"warning":   "Service may be temporarily unavailable during restoration.",
	})
}

// ValidateBackup valida la integridad de un backup
// @Summary Validar backup
// @Description Valida la integridad de un backup específico
// @Tags backup
// @Accept json
// @Produce json
// @Param id path string true "ID del backup"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/{id}/validate [post]
func (h *BackupHandler) ValidateBackup(c *fiber.Ctx) error {
	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required")
	}

	h.logger.Info("Validating backup",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	if err := h.backupService.ValidateBackup(c.Context(), backupID); err != nil {
		h.logger.Error("Backup validation failed",
			zap.String("backup_id", backupID),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			return utils.NotFoundResponse(c, "Backup not found")
		}

		return utils.BadRequestResponse(c, "Backup validation failed: "+err.Error())
	}

	h.logger.Info("Backup validation successful",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Backup validation successful", map[string]interface{}{
		"backup_id":    backupID,
		"status":       "valid",
		"validated_at": time.Now(),
	})
}

// StartAutomatedBackups inicia los backups automatizados
// @Summary Iniciar backups automatizados
// @Description Inicia el sistema de backups automatizados
// @Tags backup
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/automated/start [post]
func (h *BackupHandler) StartAutomatedBackups(c *fiber.Ctx) error {
	// Solo admins pueden controlar backups automatizados
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized automated backup control attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can control automated backups")
	}

	h.logger.Info("Starting automated backups",
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)))

	if err := h.backupService.StartAutomatedBackups(c.Context()); err != nil {
		h.logger.Error("Failed to start automated backups",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		if strings.Contains(err.Error(), "already running") {
			return utils.BadRequestResponse(c, "Automated backups are already running")
		}
		if strings.Contains(err.Error(), "not enabled") {
			return utils.BadRequestResponse(c, "Backup system is not enabled")
		}

		return utils.InternalServerErrorResponse(c, "Failed to start automated backups")
	}

	h.logger.Info("Automated backups started successfully",
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Automated backups started successfully", nil)
}

// StopAutomatedBackups detiene los backups automatizados
// @Summary Detener backups automatizados
// @Description Detiene el sistema de backups automatizados
// @Tags backup
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/automated/stop [post]
func (h *BackupHandler) StopAutomatedBackups(c *fiber.Ctx) error {
	// Solo admins pueden controlar backups automatizados
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized automated backup control attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can control automated backups")
	}

	h.logger.Info("Stopping automated backups",
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)))

	if err := h.backupService.StopAutomatedBackups(); err != nil {
		h.logger.Error("Failed to stop automated backups",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		if strings.Contains(err.Error(), "not running") {
			return utils.BadRequestResponse(c, "Automated backups are not currently running")
		}

		return utils.InternalServerErrorResponse(c, "Failed to stop automated backups")
	}

	h.logger.Info("Automated backups stopped successfully",
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Automated backups stopped successfully", nil)
}

// CleanupOldBackups limpia backups antiguos manualmente
// @Summary Limpiar backups antiguos
// @Description Elimina backups antiguos según la política de retención
// @Tags backup
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/automated/cleanup [post]
func (h *BackupHandler) CleanupOldBackups(c *fiber.Ctx) error {
	// Solo admins pueden limpiar backups
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup cleanup attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can cleanup backups")
	}

	h.logger.Info("Cleaning up old backups",
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)))

	if err := h.backupService.CleanupOldBackups(c.Context()); err != nil {
		h.logger.Error("Failed to cleanup old backups",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to cleanup old backups")
	}

	h.logger.Info("Old backups cleaned up successfully",
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, "Old backups cleaned up successfully", nil)
}

// DownloadBackup permite descargar un backup (si está implementado)
// @Summary Descargar backup
// @Description Descarga un archivo de backup específico
// @Tags backup
// @Accept json
// @Produce application/octet-stream
// @Param id path string true "ID del backup"
// @Security BearerAuth
// @Success 200 {file} binary
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/backup/{id}/download [get]
func (h *BackupHandler) DownloadBackup(c *fiber.Ctx) error {
	// Solo admins pueden descargar backups
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		return utils.ForbiddenResponse(c, "Only administrators can download backups")
	}

	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required")
	}

	// Por ahora, esta funcionalidad no está implementada
	// En el futuro, aquí se implementaría la descarga del backup
	h.logger.Info("Backup download requested (not implemented)",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.BadRequestResponse(c, "Backup download feature is not yet implemented")
}

// Helper functions

// getUserID extrae el ID del usuario del contexto
func getUserID(c *fiber.Ctx) string {
	if userID := c.Locals("userID"); userID != nil {
		if id, ok := userID.(primitive.ObjectID); ok {
			return id.Hex()
		}
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return "unknown"
}

// getUserEmail extrae el email del usuario del contexto
func getUserEmail(c *fiber.Ctx) string {
	if userEmail := c.Locals("userEmail"); userEmail != nil {
		if email, ok := userEmail.(string); ok {
			return email
		}
	}
	return "unknown"
}

// getUserRole extrae el rol del usuario del contexto
func getUserRole(c *fiber.Ctx) string {
	if userRole := c.Locals("userRole"); userRole != nil {
		if role, ok := userRole.(string); ok {
			return role
		}
	}
	return "unknown"
}

// validateBackupID valida el formato del ID de backup
func validateBackupID(backupID string) bool {
	if backupID == "" {
		return false
	}

	// Un ID de backup típico tiene formato: tipo-timestamp
	// Ejemplo: full-20231215-143052
	parts := strings.Split(backupID, "-")
	if len(parts) < 3 {
		return false
	}

	// Validar que contiene un tipo válido
	validTypes := []string{"full", "incremental", "mongodb", "redis", "config"}
	backupType := parts[0]
	for _, validType := range validTypes {
		if backupType == validType {
			return true
		}
	}

	return false
}

// getClientInfo obtiene información del cliente para logging
func getClientInfo(c *fiber.Ctx) map[string]interface{} {
	return map[string]interface{}{
		"ip_address": c.IP(),
		"user_agent": c.Get("User-Agent"),
		"method":     c.Method(),
		"path":       c.Path(),
		"timestamp":  time.Now(),
	}
}
