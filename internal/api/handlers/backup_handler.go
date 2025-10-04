// internal/api/handlers/backup_handler.go
package handlers

import (
	"strings"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type BackupHandler struct {
	backupService services.BackupService
	logger        *zap.Logger
}

func NewBackupHandler(backupService services.BackupService, logger *zap.Logger) *BackupHandler {
	return &BackupHandler{
		backupService: backupService,
		logger:        logger,
	}
}

func (h *BackupHandler) CreateBackup(c *fiber.Ctx) error {
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup creation attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can create backups")
	}

	backupType := strings.ToLower(c.Query("type", "full"))
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
		return utils.BadRequestResponse(c, "Invalid backup type. Valid types: "+strings.Join(validTypes, ", "), nil)
	}

	h.logger.Info("Creating backup",
		zap.String("type", backupType),
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)),
		zap.String("ip_address", c.IP()))

	backupInfo, err := h.backupService.CreateBackup(c.Context(), services.BackupType(backupType))
	if err != nil {
		h.logger.Error("Failed to create backup",
			zap.String("backup_type", backupType),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))

		if strings.Contains(err.Error(), "not enabled") {
			return utils.BadRequestResponse(c, "Backup system is not enabled", nil)
		}
		if strings.Contains(err.Error(), "disk space") {
			return utils.BadRequestResponse(c, "Insufficient disk space for backup", nil)
		}
		if strings.Contains(err.Error(), "permission") {
			return utils.ForbiddenResponse(c, "Insufficient permissions for backup operation")
		}

		return utils.InternalServerErrorResponse(c, "Failed to create backup", err)
	}

	h.logger.Info("Backup created successfully",
		zap.String("backup_id", backupInfo.ID),
		zap.String("backup_type", backupType),
		zap.Duration("duration", backupInfo.Duration),
		zap.Int64("size_bytes", backupInfo.Size),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Backup created successfully", backupInfo)
}

func (h *BackupHandler) ListBackups(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	backupType := c.Query("type", "")
	status := c.Query("status", "")

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

	allBackups, err := h.backupService.ListBackups(c.Context())
	if err != nil {
		h.logger.Error("Failed to list backups",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to list backups", err)
	}

	var filteredBackups []models.BackupInfo
	for _, backup := range allBackups {
		if backupType != "" && backup.Type != backupType {
			continue
		}
		if status != "" && backup.Status != status {
			continue
		}
		filteredBackups = append(filteredBackups, backup)
	}

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

	return utils.SuccessResponse(c, fiber.StatusOK, "Backups retrieved successfully", response)
}

func (h *BackupHandler) GetBackupStatus(c *fiber.Ctx) error {
	h.logger.Info("Getting backup status", zap.String("user_id", getUserID(c)))

	status, err := h.backupService.GetBackupStatus(c.Context())
	if err != nil {
		h.logger.Error("Failed to get backup status",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get backup status", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Backup status retrieved successfully", status)
}

func (h *BackupHandler) GetBackupInfo(c *fiber.Ctx) error {
	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required", nil)
	}

	h.logger.Info("Getting backup info",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	backups, err := h.backupService.ListBackups(c.Context())
	if err != nil {
		h.logger.Error("Failed to get backup info",
			zap.String("backup_id", backupID),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get backup information", err)
	}

	for _, backup := range backups {
		if backup.ID == backupID {
			return utils.SuccessResponse(c, fiber.StatusOK, "Backup information retrieved successfully", backup)
		}
	}

	h.logger.Warn("Backup not found",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.NotFoundResponse(c, "Backup not found")
}

func (h *BackupHandler) DeleteBackup(c *fiber.Ctx) error {
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup deletion attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can delete backups")
	}

	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required", nil)
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

		return utils.InternalServerErrorResponse(c, "Failed to delete backup", err)
	}

	h.logger.Info("Backup deleted successfully",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Backup deleted successfully", nil)
}

func (h *BackupHandler) RestoreBackup(c *fiber.Ctx) error {
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		h.logger.Warn("Unauthorized backup restoration attempt",
			zap.String("user_id", getUserID(c)),
			zap.String("user_role", userRole))
		return utils.ForbiddenResponse(c, "Only administrators can restore backups")
	}

	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required", nil)
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
			return utils.BadRequestResponse(c, "Cannot restore backup with current status", nil)
		}

		return utils.InternalServerErrorResponse(c, "Failed to restore backup", err)
	}

	h.logger.Info("Backup restoration initiated successfully",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Backup restoration initiated successfully", map[string]interface{}{
		"backup_id": backupID,
		"message":   "Restoration process started. Please monitor application logs for progress.",
		"warning":   "Service may be temporarily unavailable during restoration.",
	})
}

func (h *BackupHandler) ValidateBackup(c *fiber.Ctx) error {
	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required", nil)
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

		return utils.BadRequestResponse(c, "Backup validation failed: "+err.Error(), err)
	}

	h.logger.Info("Backup validation successful",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Backup validation successful", map[string]interface{}{
		"backup_id":    backupID,
		"status":       "valid",
		"validated_at": time.Now(),
	})
}

func (h *BackupHandler) StartAutomatedBackups(c *fiber.Ctx) error {
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
			return utils.BadRequestResponse(c, "Automated backups are already running", nil)
		}
		if strings.Contains(err.Error(), "not enabled") {
			return utils.BadRequestResponse(c, "Backup system is not enabled", nil)
		}

		return utils.InternalServerErrorResponse(c, "Failed to start automated backups", err)
	}

	h.logger.Info("Automated backups started successfully",
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Automated backups started successfully", nil)
}

func (h *BackupHandler) StopAutomatedBackups(c *fiber.Ctx) error {
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
			return utils.BadRequestResponse(c, "Automated backups are not currently running", nil)
		}

		return utils.InternalServerErrorResponse(c, "Failed to stop automated backups", err)
	}

	h.logger.Info("Automated backups stopped successfully",
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Automated backups stopped successfully", nil)
}

func (h *BackupHandler) CleanupOldBackups(c *fiber.Ctx) error {
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
		return utils.InternalServerErrorResponse(c, "Failed to cleanup old backups", err)
	}

	h.logger.Info("Old backups cleaned up successfully",
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Old backups cleaned up successfully", nil)
}

func (h *BackupHandler) DownloadBackup(c *fiber.Ctx) error {
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "admin" {
		return utils.ForbiddenResponse(c, "Only administrators can download backups")
	}

	backupID := c.Params("id")
	if backupID == "" {
		return utils.BadRequestResponse(c, "Backup ID is required", nil)
	}

	h.logger.Info("Backup download requested (not implemented)",
		zap.String("backup_id", backupID),
		zap.String("user_id", getUserID(c)))

	return utils.BadRequestResponse(c, "Backup download feature is not yet implemented", nil)
}
