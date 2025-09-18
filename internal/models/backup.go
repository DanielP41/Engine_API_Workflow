// internal/models/backup.go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BackupInfo representa la información de un backup
type BackupInfo struct {
	ID         string             `json:"id" bson:"_id"`
	Type       string             `json:"type" bson:"type"`     // full, incremental, mongodb, redis, config
	Status     string             `json:"status" bson:"status"` // in_progress, completed, failed
	StartTime  time.Time          `json:"start_time" bson:"start_time"`
	EndTime    time.Time          `json:"end_time,omitempty" bson:"end_time,omitempty"`
	Duration   time.Duration      `json:"duration" bson:"duration"`
	Size       int64              `json:"size_bytes" bson:"size_bytes"`
	Path       string             `json:"path" bson:"path"`
	RemotePath string             `json:"remote_path,omitempty" bson:"remote_path,omitempty"` // Para almacenamiento remoto
	Compressed bool               `json:"compressed" bson:"compressed"`
	Uploaded   bool               `json:"uploaded" bson:"uploaded"`
	Error      string             `json:"error,omitempty" bson:"error,omitempty"`
	Metadata   BackupMetadata     `json:"metadata" bson:"metadata"`
	CreatedBy  primitive.ObjectID `json:"created_by,omitempty" bson:"created_by,omitempty"` // Usuario que creó el backup
	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// BackupMetadata contiene metadata adicional del backup
type BackupMetadata struct {
	MongoDBVersion   string           `json:"mongodb_version" bson:"mongodb_version"`
	RedisVersion     string           `json:"redis_version" bson:"redis_version"`
	AppVersion       string           `json:"app_version" bson:"app_version"`
	Environment      string           `json:"environment" bson:"environment"`
	Collections      map[string]int64 `json:"collections" bson:"collections"` // collection -> document count
	RedisKeys        int64            `json:"redis_keys" bson:"redis_keys"`
	ConfigFiles      []string         `json:"config_files" bson:"config_files"`
	Checksum         string           `json:"checksum" bson:"checksum"`               // SHA256 checksum
	OriginalSize     int64            `json:"original_size" bson:"original_size"`     // Tamaño antes de compresión
	CompressedSize   int64            `json:"compressed_size" bson:"compressed_size"` // Tamaño después de compresión
	CompressionRatio float64          `json:"compression_ratio" bson:"compression_ratio"`
	BackupMethod     string           `json:"backup_method" bson:"backup_method"`   // mongodump, redis-cli, etc.
	SchemaVersion    string           `json:"schema_version" bson:"schema_version"` // Versión del esquema de backup
}

// BackupStatus representa el estado actual del sistema de backup
type BackupStatus struct {
	Enabled              bool             `json:"enabled"`
	AutomatedRunning     bool             `json:"automated_running"`
	LastBackup           time.Time        `json:"last_backup,omitempty"`
	NextScheduledBackup  time.Time        `json:"next_scheduled_backup,omitempty"`
	TotalBackups         int              `json:"total_backups"`
	TotalSizeMB          float64          `json:"total_size_mb"`
	AvailableDiskSpaceMB float64          `json:"available_disk_space_mb"`
	BackupInterval       string           `json:"backup_interval"`
	RetentionDays        int              `json:"retention_days"`
	StorageType          string           `json:"storage_type"`
	StoragePath          string           `json:"storage_path"`
	LastError            string           `json:"last_error,omitempty"`
	Health               string           `json:"health"` // healthy, warning, critical
	Statistics           BackupStatistics `json:"statistics"`
}

// BackupStatistics estadísticas del sistema de backup
type BackupStatistics struct {
	TotalBackupsCreated   int64     `json:"total_backups_created"`
	TotalBackupsRestored  int64     `json:"total_backups_restored"`
	TotalBackupsFailed    int64     `json:"total_backups_failed"`
	TotalBackupsDeleted   int64     `json:"total_backups_deleted"`
	AverageBackupSizeMB   float64   `json:"average_backup_size_mb"`
	AverageBackupDuration string    `json:"average_backup_duration"`
	SuccessRate           float64   `json:"success_rate"` // Porcentaje de éxito
	LastSuccessfulBackup  time.Time `json:"last_successful_backup,omitempty"`
	LastFailedBackup      time.Time `json:"last_failed_backup,omitempty"`
	OldestBackupDate      time.Time `json:"oldest_backup_date,omitempty"`
	NewestBackupDate      time.Time `json:"newest_backup_date,omitempty"`
}

// BackupRestoreInfo información sobre una restauración
type BackupRestoreInfo struct {
	ID         string             `json:"id" bson:"_id"`
	BackupID   string             `json:"backup_id" bson:"backup_id"`
	Status     string             `json:"status" bson:"status"` // in_progress, completed, failed
	StartTime  time.Time          `json:"start_time" bson:"start_time"`
	EndTime    time.Time          `json:"end_time,omitempty" bson:"end_time,omitempty"`
	Duration   time.Duration      `json:"duration" bson:"duration"`
	Error      string             `json:"error,omitempty" bson:"error,omitempty"`
	RestoredBy primitive.ObjectID `json:"restored_by" bson:"restored_by"`
	Options    RestoreOptions     `json:"options" bson:"options"`
	Progress   RestoreProgress    `json:"progress" bson:"progress"`
	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// RestoreOptions opciones para la restauración
type RestoreOptions struct {
	DropExisting   bool     `json:"drop_existing" bson:"drop_existing"`                 // Drop collections before restore
	RestoreIndexes bool     `json:"restore_indexes" bson:"restore_indexes"`             // Restaurar índices
	RestoreUsers   bool     `json:"restore_users" bson:"restore_users"`                 // Restaurar usuarios
	Collections    []string `json:"collections,omitempty" bson:"collections,omitempty"` // Específicas
	DryRun         bool     `json:"dry_run" bson:"dry_run"`                             // Solo simular
	StopOnError    bool     `json:"stop_on_error" bson:"stop_on_error"`                 // Parar en errores
}

// RestoreProgress progreso de la restauración
type RestoreProgress struct {
	TotalSteps             int     `json:"total_steps" bson:"total_steps"`
	CompletedSteps         int     `json:"completed_steps" bson:"completed_steps"`
	CurrentStep            string  `json:"current_step" bson:"current_step"`
	PercentComplete        float64 `json:"percent_complete" bson:"percent_complete"`
	EstimatedTimeRemaining string  `json:"estimated_time_remaining,omitempty" bson:"estimated_time_remaining,omitempty"`
}

// BackupSchedule representa un horario de backup automatizado
type BackupSchedule struct {
	ID             primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name           string             `json:"name" bson:"name"`
	Type           string             `json:"type" bson:"type"` // full, incremental, etc.
	CronExpression string             `json:"cron_expression" bson:"cron_expression"`
	Enabled        bool               `json:"enabled" bson:"enabled"`
	NextRun        time.Time          `json:"next_run" bson:"next_run"`
	LastRun        time.Time          `json:"last_run,omitempty" bson:"last_run,omitempty"`
	LastStatus     string             `json:"last_status" bson:"last_status"` // success, failed
	RunCount       int64              `json:"run_count" bson:"run_count"`
	FailureCount   int64              `json:"failure_count" bson:"failure_count"`
	RetentionDays  int                `json:"retention_days" bson:"retention_days"`
	Options        ScheduleOptions    `json:"options" bson:"options"`
	CreatedBy      primitive.ObjectID `json:"created_by" bson:"created_by"`
	CreatedAt      time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at" bson:"updated_at"`
}

// ScheduleOptions opciones para horarios programados
type ScheduleOptions struct {
	MaxRetries          int           `json:"max_retries" bson:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay" bson:"retry_delay"`
	NotifyOnSuccess     bool          `json:"notify_on_success" bson:"notify_on_success"`
	NotifyOnFailure     bool          `json:"notify_on_failure" bson:"notify_on_failure"`
	CleanupAfterDays    int           `json:"cleanup_after_days" bson:"cleanup_after_days"`
	CompressionLevel    int           `json:"compression_level" bson:"compression_level"`
	UploadToRemote      bool          `json:"upload_to_remote" bson:"upload_to_remote"`
	ValidateAfterBackup bool          `json:"validate_after_backup" bson:"validate_after_backup"`
	ExcludedCollections []string      `json:"excluded_collections,omitempty" bson:"excluded_collections,omitempty"`
	Tags                []string      `json:"tags,omitempty" bson:"tags,omitempty"`
}

// BackupValidationResult resultado de validación de backup
type BackupValidationResult struct {
	BackupID       string            `json:"backup_id"`
	IsValid        bool              `json:"is_valid"`
	ValidationTime time.Time         `json:"validation_time"`
	Checks         []ValidationCheck `json:"checks"`
	Summary        ValidationSummary `json:"summary"`
}

// ValidationCheck verificación individual
type ValidationCheck struct {
	Name      string                 `json:"name"`
	Status    string                 `json:"status"` // passed, failed, warning, skipped
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// ValidationSummary resumen de validación
type ValidationSummary struct {
	TotalChecks   int `json:"total_checks"`
	PassedChecks  int `json:"passed_checks"`
	FailedChecks  int `json:"failed_checks"`
	WarningChecks int `json:"warning_checks"`
	SkippedChecks int `json:"skipped_checks"`
}

// BackupConfiguration configuración de backup
type BackupConfiguration struct {
	ID                   primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Name                 string               `json:"name" bson:"name"`
	Description          string               `json:"description,omitempty" bson:"description,omitempty"`
	BackupTypes          []string             `json:"backup_types" bson:"backup_types"` // Tipos permitidos
	StorageLocations     []StorageLocation    `json:"storage_locations" bson:"storage_locations"`
	DefaultSchedule      string               `json:"default_schedule" bson:"default_schedule"`
	RetentionPolicy      RetentionPolicy      `json:"retention_policy" bson:"retention_policy"`
	SecuritySettings     SecuritySettings     `json:"security_settings" bson:"security_settings"`
	NotificationSettings NotificationSettings `json:"notification_settings" bson:"notification_settings"`
	IsActive             bool                 `json:"is_active" bson:"is_active"`
	CreatedBy            primitive.ObjectID   `json:"created_by" bson:"created_by"`
	CreatedAt            time.Time            `json:"created_at" bson:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at" bson:"updated_at"`
}

// StorageLocation ubicación de almacenamiento
type StorageLocation struct {
	Type      string            `json:"type" bson:"type"` // local, s3, gcs, azure
	Name      string            `json:"name" bson:"name"`
	Path      string            `json:"path" bson:"path"`
	Config    map[string]string `json:"config" bson:"config"` // Configuración específica
	IsDefault bool              `json:"is_default" bson:"is_default"`
	IsActive  bool              `json:"is_active" bson:"is_active"`
	Priority  int               `json:"priority" bson:"priority"` // Para failover
}

// RetentionPolicy política de retención
type RetentionPolicy struct {
	DailyRetentionDays     int  `json:"daily_retention_days" bson:"daily_retention_days"`
	WeeklyRetentionWeeks   int  `json:"weekly_retention_weeks" bson:"weekly_retention_weeks"`
	MonthlyRetentionMonths int  `json:"monthly_retention_months" bson:"monthly_retention_months"`
	YearlyRetentionYears   int  `json:"yearly_retention_years" bson:"yearly_retention_years"`
	MaxTotalBackups        int  `json:"max_total_backups" bson:"max_total_backups"`
	MaxTotalSizeGB         int  `json:"max_total_size_gb" bson:"max_total_size_gb"`
	AutoCleanupEnabled     bool `json:"auto_cleanup_enabled" bson:"auto_cleanup_enabled"`
}

// SecuritySettings configuración de seguridad
type SecuritySettings struct {
	EncryptionEnabled    bool   `json:"encryption_enabled" bson:"encryption_enabled"`
	EncryptionAlgorithm  string `json:"encryption_algorithm" bson:"encryption_algorithm"`
	KeyRotationDays      int    `json:"key_rotation_days" bson:"key_rotation_days"`
	AccessControlEnabled bool   `json:"access_control_enabled" bson:"access_control_enabled"`
	AuditLogsEnabled     bool   `json:"audit_logs_enabled" bson:"audit_logs_enabled"`
	RequireApproval      bool   `json:"require_approval" bson:"require_approval"` // Para restauraciones
}

// NotificationSettings configuración de notificaciones
type NotificationSettings struct {
	EmailNotifications   EmailNotificationConfig   `json:"email_notifications" bson:"email_notifications"`
	SlackNotifications   SlackNotificationConfig   `json:"slack_notifications" bson:"slack_notifications"`
	WebhookNotifications WebhookNotificationConfig `json:"webhook_notifications" bson:"webhook_notifications"`
}

// EmailNotificationConfig configuración de email
type EmailNotificationConfig struct {
	Enabled      bool     `json:"enabled" bson:"enabled"`
	Recipients   []string `json:"recipients" bson:"recipients"`
	OnSuccess    bool     `json:"on_success" bson:"on_success"`
	OnFailure    bool     `json:"on_failure" bson:"on_failure"`
	OnWarning    bool     `json:"on_warning" bson:"on_warning"`
	SMTPServer   string   `json:"smtp_server,omitempty" bson:"smtp_server,omitempty"`
	SMTPPort     int      `json:"smtp_port,omitempty" bson:"smtp_port,omitempty"`
	SMTPUsername string   `json:"smtp_username,omitempty" bson:"smtp_username,omitempty"`
	SMTPPassword string   `json:"smtp_password,omitempty" bson:"smtp_password,omitempty"`
}

// SlackNotificationConfig configuración de Slack
type SlackNotificationConfig struct {
	Enabled    bool   `json:"enabled" bson:"enabled"`
	WebhookURL string `json:"webhook_url,omitempty" bson:"webhook_url,omitempty"`
	Channel    string `json:"channel,omitempty" bson:"channel,omitempty"`
	OnSuccess  bool   `json:"on_success" bson:"on_success"`
	OnFailure  bool   `json:"on_failure" bson:"on_failure"`
	OnWarning  bool   `json:"on_warning" bson:"on_warning"`
	BotToken   string `json:"bot_token,omitempty" bson:"bot_token,omitempty"`
}

// WebhookNotificationConfig configuración de webhook
type WebhookNotificationConfig struct {
	Enabled   bool              `json:"enabled" bson:"enabled"`
	URL       string            `json:"url,omitempty" bson:"url,omitempty"`
	Method    string            `json:"method" bson:"method"` // POST, PUT
	Headers   map[string]string `json:"headers,omitempty" bson:"headers,omitempty"`
	OnSuccess bool              `json:"on_success" bson:"on_success"`
	OnFailure bool              `json:"on_failure" bson:"on_failure"`
	OnWarning bool              `json:"on_warning" bson:"on_warning"`
	Secret    string            `json:"secret,omitempty" bson:"secret,omitempty"`
	Timeout   time.Duration     `json:"timeout" bson:"timeout"`
}

// BackupEvent evento de backup para auditoría
type BackupEvent struct {
	ID          primitive.ObjectID     `json:"id" bson:"_id,omitempty"`
	BackupID    string                 `json:"backup_id" bson:"backup_id"`
	EventType   string                 `json:"event_type" bson:"event_type"` // created, started, completed, failed, deleted, restored
	Description string                 `json:"description" bson:"description"`
	UserID      primitive.ObjectID     `json:"user_id,omitempty" bson:"user_id,omitempty"`
	UserEmail   string                 `json:"user_email,omitempty" bson:"user_email,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty" bson:"user_agent,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
}

// BackupAlert alerta del sistema de backup
type BackupAlert struct {
	ID         primitive.ObjectID     `json:"id" bson:"_id,omitempty"`
	Type       string                 `json:"type" bson:"type"`         // error, warning, info
	Severity   string                 `json:"severity" bson:"severity"` // low, medium, high, critical
	Title      string                 `json:"title" bson:"title"`
	Message    string                 `json:"message" bson:"message"`
	BackupID   string                 `json:"backup_id,omitempty" bson:"backup_id,omitempty"`
	Component  string                 `json:"component" bson:"component"` // service, storage, schedule, etc.
	IsResolved bool                   `json:"is_resolved" bson:"is_resolved"`
	ResolvedAt time.Time              `json:"resolved_at,omitempty" bson:"resolved_at,omitempty"`
	ResolvedBy primitive.ObjectID     `json:"resolved_by,omitempty" bson:"resolved_by,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// PerformanceMetrics métricas de rendimiento
type PerformanceMetrics struct {
	BackupID            string        `json:"backup_id"`
	StartTime           time.Time     `json:"start_time"`
	EndTime             time.Time     `json:"end_time"`
	TotalDuration       time.Duration `json:"total_duration"`
	MongoDBDuration     time.Duration `json:"mongodb_duration"`
	RedisDuration       time.Duration `json:"redis_duration"`
	ConfigDuration      time.Duration `json:"config_duration"`
	CompressionDuration time.Duration `json:"compression_duration"`
	UploadDuration      time.Duration `json:"upload_duration"`
	ThroughputMBps      float64       `json:"throughput_mbps"`
	CompressionRatio    float64       `json:"compression_ratio"`
	CPUUsagePercent     float64       `json:"cpu_usage_percent"`
	MemoryUsageMB       float64       `json:"memory_usage_mb"`
	DiskIOReadMB        float64       `json:"disk_io_read_mb"`
	DiskIOWriteMB       float64       `json:"disk_io_write_mb"`
	NetworkUploadMB     float64       `json:"network_upload_mb"`
}

// Métodos helper para BackupInfo

// IsCompleted verifica si el backup está completado
func (b *BackupInfo) IsCompleted() bool {
	return b.Status == "completed"
}

// IsFailed verifica si el backup falló
func (b *BackupInfo) IsFailed() bool {
	return b.Status == "failed"
}

// IsInProgress verifica si el backup está en progreso
func (b *BackupInfo) IsInProgress() bool {
	return b.Status == "in_progress"
}

// GetSizeMB retorna el tamaño en MB
func (b *BackupInfo) GetSizeMB() float64 {
	return float64(b.Size) / (1024 * 1024)
}

// GetSizeGB retorna el tamaño en GB
func (b *BackupInfo) GetSizeGB() float64 {
	return float64(b.Size) / (1024 * 1024 * 1024)
}

// GetDurationString retorna la duración como string
func (b *BackupInfo) GetDurationString() string {
	if b.Duration == 0 {
		return "N/A"
	}
	return b.Duration.String()
}

// Métodos helper para BackupStatus

// IsHealthy verifica si el sistema está saludable
func (bs *BackupStatus) IsHealthy() bool {
	return bs.Health == "healthy"
}

// HasWarnings verifica si hay advertencias
func (bs *BackupStatus) HasWarnings() bool {
	return bs.Health == "warning"
}

// IsCritical verifica si el estado es crítico
func (bs *BackupStatus) IsCritical() bool {
	return bs.Health == "critical"
}

// GetDiskUsagePercent calcula el porcentaje de uso de disco
func (bs *BackupStatus) GetDiskUsagePercent() float64 {
	if bs.AvailableDiskSpaceMB <= 0 {
		return 0
	}

	total := bs.TotalSizeMB + bs.AvailableDiskSpaceMB
	if total <= 0 {
		return 0
	}

	return (bs.TotalSizeMB / total) * 100
}

// Constantes para tipos y estados
const (
	// Tipos de backup
	BackupTypeFull        = "full"
	BackupTypeIncremental = "incremental"
	BackupTypeMongoDB     = "mongodb"
	BackupTypeRedis       = "redis"
	BackupTypeConfig      = "config"

	// Estados de backup
	BackupStatusInProgress = "in_progress"
	BackupStatusCompleted  = "completed"
	BackupStatusFailed     = "failed"
	BackupStatusCancelled  = "cancelled"

	// Estados de salud
	HealthStatusHealthy  = "healthy"
	HealthStatusWarning  = "warning"
	HealthStatusCritical = "critical"

	// Tipos de almacenamiento
	StorageTypeLocal = "local"
	StorageTypeS3    = "s3"
	StorageTypeGCS   = "gcs"
	StorageTypeAzure = "azure"

	// Tipos de eventos
	EventTypeCreated   = "created"
	EventTypeStarted   = "started"
	EventTypeCompleted = "completed"
	EventTypeFailed    = "failed"
	EventTypeDeleted   = "deleted"
	EventTypeRestored  = "restored"
	EventTypeValidated = "validated"

	// Tipos de alertas
	AlertTypeError   = "error"
	AlertTypeWarning = "warning"
	AlertTypeInfo    = "info"

	// Severidades de alertas
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)
