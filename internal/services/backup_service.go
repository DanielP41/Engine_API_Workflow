// internal/services/backup_service.go
package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"

	"go.uber.org/zap"
)

// BackupService define la interfaz para el servicio de backup
type BackupService interface {
	// Backup operations
	CreateBackup(ctx context.Context, backupType BackupType) (*models.BackupInfo, error)
	RestoreBackup(ctx context.Context, backupID string) error
	ListBackups(ctx context.Context) ([]models.BackupInfo, error)
	DeleteBackup(ctx context.Context, backupID string) error

	// Automated operations
	StartAutomatedBackups(ctx context.Context) error
	StopAutomatedBackups() error
	CleanupOldBackups(ctx context.Context) error

	// Health and status
	GetBackupStatus(ctx context.Context) (*models.BackupStatus, error)
	ValidateBackup(ctx context.Context, backupID string) error
}

// BackupType representa los tipos de backup disponibles
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
	BackupTypeMongoDB     BackupType = "mongodb"
	BackupTypeRedis       BackupType = "redis"
	BackupTypeConfig      BackupType = "config"
)

// BackupStorage representa los tipos de almacenamiento
type BackupStorage string

const (
	BackupStorageLocal BackupStorage = "local"
	BackupStorageS3    BackupStorage = "s3"
	BackupStorageGCS   BackupStorage = "gcs"
	BackupStorageAzure BackupStorage = "azure"
)

// backupService implementa BackupService
type backupService struct {
	config       *config.Config
	logger       *zap.Logger
	userRepo     repository.UserRepository
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	queueRepo    repository.QueueRepository

	// Control de automatización
	stopChan  chan struct{}
	isRunning bool
}

// NewBackupService crea una nueva instancia del servicio de backup
func NewBackupService(
	cfg *config.Config,
	logger *zap.Logger,
	userRepo repository.UserRepository,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	queueRepo repository.QueueRepository,
) BackupService {
	return &backupService{
		config:       cfg,
		logger:       logger,
		userRepo:     userRepo,
		workflowRepo: workflowRepo,
		logRepo:      logRepo,
		queueRepo:    queueRepo,
		stopChan:     make(chan struct{}),
		isRunning:    false,
	}
}

// CreateBackup crea un nuevo backup
func (s *backupService) CreateBackup(ctx context.Context, backupType BackupType) (*models.BackupInfo, error) {
	if !s.isBackupEnabled() {
		return nil, fmt.Errorf("backup is not enabled in configuration")
	}

	backupID := s.generateBackupID(backupType)
	backupPath := s.getBackupPath(backupID)

	s.logger.Info("Starting backup creation",
		zap.String("backup_id", backupID),
		zap.String("type", string(backupType)),
		zap.String("path", backupPath))

	// Crear directorio de backup
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	backupInfo := &models.BackupInfo{
		ID:        backupID,
		Type:      string(backupType),
		Status:    "in_progress",
		StartTime: time.Now(),
		Path:      backupPath,
		Size:      0,
	}

	var err error

	// Realizar backup según el tipo
	switch backupType {
	case BackupTypeFull:
		err = s.createFullBackup(ctx, backupPath)
	case BackupTypeIncremental:
		err = s.createIncrementalBackup(ctx, backupPath)
	case BackupTypeMongoDB:
		err = s.createMongoDBBackup(ctx, backupPath)
	case BackupTypeRedis:
		err = s.createRedisBackup(ctx, backupPath)
	case BackupTypeConfig:
		err = s.createConfigBackup(ctx, backupPath)
	default:
		err = fmt.Errorf("unsupported backup type: %s", backupType)
	}

	// Actualizar información del backup
	backupInfo.EndTime = time.Now()
	backupInfo.Duration = backupInfo.EndTime.Sub(backupInfo.StartTime)

	if err != nil {
		backupInfo.Status = "failed"
		backupInfo.Error = err.Error()
		s.logger.Error("Backup creation failed",
			zap.String("backup_id", backupID),
			zap.Error(err))
		return backupInfo, err
	}

	// Calcular tamaño del backup
	if size, sizeErr := s.calculateBackupSize(backupPath); sizeErr == nil {
		backupInfo.Size = size
	}

	backupInfo.Status = "completed"

	// Comprimir backup si está habilitado
	if s.shouldCompressBackup() {
		if compressErr := s.compressBackup(backupPath); compressErr != nil {
			s.logger.Warn("Failed to compress backup", zap.Error(compressErr))
		}
	}

	// Subir a almacenamiento remoto si está configurado
	if s.shouldUploadBackup() {
		if uploadErr := s.uploadBackup(ctx, backupInfo); uploadErr != nil {
			s.logger.Warn("Failed to upload backup", zap.Error(uploadErr))
		}
	}

	s.logger.Info("Backup creation completed successfully",
		zap.String("backup_id", backupID),
		zap.Duration("duration", backupInfo.Duration),
		zap.Int64("size_bytes", backupInfo.Size))

	return backupInfo, nil
}

// createFullBackup crea un backup completo del sistema
func (s *backupService) createFullBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Creating full backup", zap.String("path", backupPath))

	// 1. Backup MongoDB
	if err := s.createMongoDBBackup(ctx, filepath.Join(backupPath, "mongodb")); err != nil {
		return fmt.Errorf("failed to backup MongoDB: %w", err)
	}

	// 2. Backup Redis
	if err := s.createRedisBackup(ctx, filepath.Join(backupPath, "redis")); err != nil {
		return fmt.Errorf("failed to backup Redis: %w", err)
	}

	// 3. Backup configuración
	if err := s.createConfigBackup(ctx, filepath.Join(backupPath, "config")); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}

	// 4. Backup logs (si están en archivos)
	if err := s.createLogsBackup(ctx, filepath.Join(backupPath, "logs")); err != nil {
		s.logger.Warn("Failed to backup logs", zap.Error(err))
	}

	return nil
}

// createMongoDBBackup crea un backup de MongoDB
func (s *backupService) createMongoDBBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Creating MongoDB backup", zap.String("path", backupPath))

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create MongoDB backup directory: %w", err)
	}

	// Usar mongodump para crear el backup
	cmd := exec.CommandContext(ctx, "mongodump",
		"--uri", s.config.MongoURI,
		"--out", backupPath,
		"--gzip", // Comprimir automáticamente
	)

	// Configurar logging de salida
	cmd.Stdout = &logWriter{logger: s.logger, level: zap.InfoLevel}
	cmd.Stderr = &logWriter{logger: s.logger, level: zap.ErrorLevel}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongodump failed: %w", err)
	}

	s.logger.Info("MongoDB backup completed successfully")
	return nil
}

// createRedisBackup crea un backup de Redis
func (s *backupService) createRedisBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Creating Redis backup", zap.String("path", backupPath))

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create Redis backup directory: %w", err)
	}

	// Opción 1: Usar BGSAVE (recomendado)
	if err := s.queueRepo.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Crear dump de Redis usando redis-cli
	dumpFile := filepath.Join(backupPath, "dump.rdb")

	cmd := exec.CommandContext(ctx, "redis-cli",
		"-h", s.config.RedisHost,
		"-p", s.config.RedisPort,
		"--rdb", dumpFile,
	)

	if s.config.RedisPassword != "" {
		cmd.Args = append(cmd.Args, "-a", s.config.RedisPassword)
	}

	cmd.Stdout = &logWriter{logger: s.logger, level: zap.InfoLevel}
	cmd.Stderr = &logWriter{logger: s.logger, level: zap.ErrorLevel}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("redis backup failed: %w", err)
	}

	s.logger.Info("Redis backup completed successfully")
	return nil
}

// createConfigBackup crea un backup de la configuración
func (s *backupService) createConfigBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Creating configuration backup", zap.String("path", backupPath))

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create config backup directory: %w", err)
	}

	// Archivos de configuración a incluir en el backup
	configFiles := []string{
		".env",
		".env.example",
		"docker-compose.yml",
		"docker-compose.prod.yml",
		"Dockerfile",
		"go.mod",
		"go.sum",
		"Makefile",
	}

	for _, file := range configFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue // Archivo no existe, skip
		}

		destPath := filepath.Join(backupPath, file)
		if err := s.copyFile(file, destPath); err != nil {
			s.logger.Warn("Failed to backup config file",
				zap.String("file", file), zap.Error(err))
			continue
		}
	}

	// Backup de directorios de configuración
	configDirs := []string{
		"config",
		"scripts",
		"web/templates",
	}

	for _, dir := range configDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		destDir := filepath.Join(backupPath, dir)
		if err := s.copyDirectory(dir, destDir); err != nil {
			s.logger.Warn("Failed to backup config directory",
				zap.String("dir", dir), zap.Error(err))
		}
	}

	s.logger.Info("Configuration backup completed successfully")
	return nil
}

// createLogsBackup crea backup de logs si están en archivos
func (s *backupService) createLogsBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Creating logs backup", zap.String("path", backupPath))

	logDirs := []string{"logs", "var/log"}

	for _, logDir := range logDirs {
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			continue
		}

		destDir := filepath.Join(backupPath, logDir)
		if err := s.copyDirectory(logDir, destDir); err != nil {
			return fmt.Errorf("failed to backup logs from %s: %w", logDir, err)
		}
	}

	return nil
}

// StartAutomatedBackups inicia los backups automatizados
func (s *backupService) StartAutomatedBackups(ctx context.Context) error {
	if !s.isBackupEnabled() {
		return fmt.Errorf("backup is not enabled in configuration")
	}

	if s.isRunning {
		return fmt.Errorf("automated backups are already running")
	}

	s.isRunning = true
	s.stopChan = make(chan struct{})

	interval := s.getBackupInterval()
	ticker := time.NewTicker(interval)

	s.logger.Info("Starting automated backups",
		zap.Duration("interval", interval))

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if _, err := s.CreateBackup(ctx, BackupTypeFull); err != nil {
					s.logger.Error("Automated backup failed", zap.Error(err))
				}

				// Limpiar backups antiguos después de crear uno nuevo
				if err := s.CleanupOldBackups(ctx); err != nil {
					s.logger.Error("Failed to cleanup old backups", zap.Error(err))
				}

			case <-s.stopChan:
				s.logger.Info("Stopping automated backups")
				s.isRunning = false
				return
			}
		}
	}()

	return nil
}

// StopAutomatedBackups detiene los backups automatizados
func (s *backupService) StopAutomatedBackups() error {
	if !s.isRunning {
		return fmt.Errorf("automated backups are not running")
	}

	close(s.stopChan)
	s.isRunning = false
	s.logger.Info("Automated backups stopped")

	return nil
}

// CleanupOldBackups elimina backups antiguos según la política de retención
func (s *backupService) CleanupOldBackups(ctx context.Context) error {
	backups, err := s.ListBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backups for cleanup: %w", err)
	}

	retentionDays := s.getRetentionDays()
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	deletedCount := 0
	for _, backup := range backups {
		if backup.StartTime.Before(cutoffDate) {
			if err := s.DeleteBackup(ctx, backup.ID); err != nil {
				s.logger.Error("Failed to delete old backup",
					zap.String("backup_id", backup.ID), zap.Error(err))
				continue
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		s.logger.Info("Cleaned up old backups",
			zap.Int("deleted_count", deletedCount),
			zap.Int("retention_days", retentionDays))
	}

	return nil
}

// RestoreBackup restaura un backup específico
func (s *backupService) RestoreBackup(ctx context.Context, backupID string) error {
	s.logger.Info("Starting backup restoration", zap.String("backup_id", backupID))

	// Verificar que el backup existe
	backups, err := s.ListBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	var targetBackup *models.BackupInfo
	for _, backup := range backups {
		if backup.ID == backupID {
			targetBackup = &backup
			break
		}
	}

	if targetBackup == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	if targetBackup.Status != "completed" {
		return fmt.Errorf("cannot restore backup with status: %s", targetBackup.Status)
	}

	backupPath := targetBackup.Path

	// Restaurar según el tipo de backup
	switch targetBackup.Type {
	case "full":
		return s.restoreFullBackup(ctx, backupPath)
	case "mongodb":
		return s.restoreMongoDBBackup(ctx, backupPath)
	case "redis":
		return s.restoreRedisBackup(ctx, backupPath)
	case "config":
		return s.restoreConfigBackup(ctx, backupPath)
	default:
		return fmt.Errorf("unsupported backup type for restoration: %s", targetBackup.Type)
	}
}

// ListBackups lista todos los backups disponibles
func (s *backupService) ListBackups(ctx context.Context) ([]models.BackupInfo, error) {
	backupDir := s.config.BackupStoragePath

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.BackupInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []models.BackupInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupPath := filepath.Join(backupDir, entry.Name())

		// Leer información del backup
		info, err := s.getBackupInfo(backupPath, entry.Name())
		if err != nil {
			s.logger.Warn("Failed to read backup info",
				zap.String("path", backupPath), zap.Error(err))
			continue
		}

		backups = append(backups, *info)
	}

	// Ordenar por fecha (más reciente primero)
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].StartTime.Before(backups[j].StartTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	return backups, nil
}

// DeleteBackup elimina un backup específico
func (s *backupService) DeleteBackup(ctx context.Context, backupID string) error {
	backupPath := s.getBackupPath(backupID)

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	s.logger.Info("Backup deleted successfully", zap.String("backup_id", backupID))
	return nil
}

// GetBackupStatus obtiene el estado del sistema de backup
func (s *backupService) GetBackupStatus(ctx context.Context) (*models.BackupStatus, error) {
	status := &models.BackupStatus{
		Enabled:          s.isBackupEnabled(),
		AutomatedRunning: s.isRunning,
		BackupInterval:   s.getBackupInterval().String(),
		RetentionDays:    s.getRetentionDays(),
		StorageType:      s.config.BackupStorageType,
		StoragePath:      s.config.BackupStoragePath,
		Health:           "healthy",
	}

	// Obtener lista de backups
	backups, err := s.ListBackups(ctx)
	if err != nil {
		status.Health = "warning"
		status.LastError = err.Error()
	} else {
		status.TotalBackups = len(backups)

		var totalSize int64
		for _, backup := range backups {
			totalSize += backup.Size

			if backup.StartTime.After(status.LastBackup) {
				status.LastBackup = backup.StartTime
			}
		}

		status.TotalSizeMB = float64(totalSize) / (1024 * 1024)
	}

	// Calcular próximo backup programado
	if s.isRunning {
		status.NextScheduledBackup = status.LastBackup.Add(s.getBackupInterval())
	}

	// Verificar espacio en disco disponible
	if diskSpace, err := s.getAvailableDiskSpace(); err == nil {
		status.AvailableDiskSpaceMB = diskSpace

		// Marcar como crítico si queda menos de 1GB
		if diskSpace < 1024 {
			status.Health = "critical"
		} else if diskSpace < 5*1024 { // Menos de 5GB
			status.Health = "warning"
		}
	}

	return status, nil
}

// ValidateBackup valida la integridad de un backup
func (s *backupService) ValidateBackup(ctx context.Context, backupID string) error {
	backupPath := s.getBackupPath(backupID)

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	// Validar estructura del backup
	requiredPaths := []string{
		filepath.Join(backupPath, "mongodb"),
		filepath.Join(backupPath, "redis"),
		filepath.Join(backupPath, "config"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing backup component: %s", filepath.Base(path))
		}
	}

	// Validar integridad de MongoDB backup
	mongoPath := filepath.Join(backupPath, "mongodb")
	if err := s.validateMongoDBBackup(mongoPath); err != nil {
		return fmt.Errorf("MongoDB backup validation failed: %w", err)
	}

	// Validar integridad de Redis backup
	redisPath := filepath.Join(backupPath, "redis", "dump.rdb")
	if _, err := os.Stat(redisPath); os.IsNotExist(err) {
		return fmt.Errorf("Redis backup file not found")
	}

	s.logger.Info("Backup validation completed successfully",
		zap.String("backup_id", backupID))

	return nil
}

// Helper methods

func (s *backupService) isBackupEnabled() bool {
	return s.config.BackupEnabled
}

func (s *backupService) getBackupInterval() time.Duration {
	return s.config.BackupInterval
}

func (s *backupService) getRetentionDays() int {
	return s.config.BackupRetentionDays
}

func (s *backupService) generateBackupID(backupType BackupType) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", backupType, timestamp)
}

func (s *backupService) getBackupPath(backupID string) string {
	return filepath.Join(s.config.BackupStoragePath, backupID)
}

func (s *backupService) createIncrementalBackup(ctx context.Context, backupPath string) error {
	// TODO: Implementar backup incremental
	// Por ahora, usar backup completo como fallback
	s.logger.Warn("Incremental backup not implemented, falling back to full backup")
	return s.createFullBackup(ctx, backupPath)
}

// Métodos de restauración

func (s *backupService) restoreFullBackup(ctx context.Context, backupPath string) error {
	// Restaurar MongoDB
	if err := s.restoreMongoDBBackup(ctx, filepath.Join(backupPath, "mongodb")); err != nil {
		return fmt.Errorf("failed to restore MongoDB: %w", err)
	}

	// Restaurar Redis
	if err := s.restoreRedisBackup(ctx, filepath.Join(backupPath, "redis")); err != nil {
		return fmt.Errorf("failed to restore Redis: %w", err)
	}

	return nil
}

func (s *backupService) restoreMongoDBBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Restoring MongoDB backup", zap.String("path", backupPath))

	cmd := exec.CommandContext(ctx, "mongorestore",
		"--uri", s.config.MongoURI,
		"--drop", // Drop collections before restoring
		"--gzip",
		backupPath,
	)

	cmd.Stdout = &logWriter{logger: s.logger, level: zap.InfoLevel}
	cmd.Stderr = &logWriter{logger: s.logger, level: zap.ErrorLevel}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongorestore failed: %w", err)
	}

	s.logger.Info("MongoDB restore completed successfully")
	return nil
}

func (s *backupService) restoreRedisBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Restoring Redis backup", zap.String("path", backupPath))

	rdbFile := filepath.Join(backupPath, "dump.rdb")
	if _, err := os.Stat(rdbFile); os.IsNotExist(err) {
		return fmt.Errorf("Redis backup file not found: %s", rdbFile)
	}

	// Para Redis, necesitamos parar el servicio, copiar el archivo RDB y reiniciar
	// En un entorno de producción, esto requeriría coordinar con el administrador del sistema
	s.logger.Warn("Redis restoration requires manual intervention in production")

	return nil
}

func (s *backupService) restoreConfigBackup(ctx context.Context, backupPath string) error {
	s.logger.Info("Restoring configuration backup", zap.String("path", backupPath))

	// Copiar archivos de configuración
	return s.copyDirectory(backupPath, ".")
}

// Utility methods

func (s *backupService) getBackupInfo(backupPath, backupID string) (*models.BackupInfo, error) {
	stat, err := os.Stat(backupPath)
	if err != nil {
		return nil, err
	}

	// Calcular tamaño del backup
	size, _ := s.calculateBackupSize(backupPath)

	// Determinar tipo de backup basado en el ID
	backupType := "full"
	if strings.Contains(backupID, "mongodb") {
		backupType = "mongodb"
	} else if strings.Contains(backupID, "redis") {
		backupType = "redis"
	} else if strings.Contains(backupID, "config") {
		backupType = "config"
	}

	return &models.BackupInfo{
		ID:        backupID,
		Type:      backupType,
		Status:    "completed", // Asumimos que si existe, está completado
		StartTime: stat.ModTime(),
		EndTime:   stat.ModTime(),
		Size:      size,
		Path:      backupPath,
		CreatedAt: stat.ModTime(),
	}, nil
}

func (s *backupService) calculateBackupSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

func (s *backupService) getAvailableDiskSpace() (float64, error) {
	// Implementación simplificada - en producción usar syscalls específicos del OS
	cmd := exec.Command("df", "-m", s.config.BackupStoragePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Parsear output del comando df (simplificado)
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected df output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, fmt.Errorf("unexpected df output format")
	}

	// Convertir a float64 (simplificado - en producción usar strconv)
	return 10240.0, nil // Placeholder - implementar parsing real
}

func (s *backupService) shouldCompressBackup() bool {
	return true // Por defecto comprimir
}

func (s *backupService) shouldUploadBackup() bool {
	return s.config.BackupStorageType != "local"
}

func (s *backupService) compressBackup(backupPath string) error {
	// Implementar compresión con gzip o tar.gz
	s.logger.Info("Compressing backup", zap.String("path", backupPath))
	// TODO: Implementar compresión real
	return nil
}

func (s *backupService) uploadBackup(ctx context.Context, backupInfo *models.BackupInfo) error {
	// Implementar upload a S3, GCS, etc.
	s.logger.Info("Uploading backup", zap.String("backup_id", backupInfo.ID))
	// TODO: Implementar upload real según el tipo de storage
	return nil
}

func (s *backupService) validateMongoDBBackup(mongoPath string) error {
	// Verificar que existen los archivos BSON esperados
	entries, err := os.ReadDir(mongoPath)
	if err != nil {
		return err
	}

	hasDatabase := false
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == s.config.MongoDatabase {
			hasDatabase = true
			break
		}
	}

	if !hasDatabase {
		return fmt.Errorf("database directory not found in backup: %s", s.config.MongoDatabase)
	}

	return nil
}

// logWriter implementa io.Writer para logging
type logWriter struct {
	logger *zap.Logger
	level  zap.AtomicLevel
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		lw.logger.Log(lw.level.Level(), msg)
	}
	return len(p), nil
}

// Utility functions for file operations
func (s *backupService) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Crear directorio destino si no existe
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (s *backupService) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return s.copyFile(path, destPath)
	})
}
