package repository

import (
	"context"
	"errors"
	"time"

	"Engine_API_Workflow/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRepository defines the contract for user data operations
type UserRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, user *models.User) (*models.User, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, id primitive.ObjectID, update *models.UpdateUserRequest) error
	Delete(ctx context.Context, id primitive.ObjectID) error

	// Compatibility methods for auth.go
	GetByIDString(ctx context.Context, id string) (*models.User, error)
	UpdateLastLoginString(ctx context.Context, id string) error

	// List and search operations
	List(ctx context.Context, page, pageSize int) (*models.UserListResponse, error)
	Search(ctx context.Context, query string, page, pageSize int) (*models.UserListResponse, error)
	ListByRole(ctx context.Context, role models.Role, page, pageSize int) (*models.UserListResponse, error)

	// Authentication and session management
	UpdateLastLogin(ctx context.Context, id primitive.ObjectID) error
	UpdatePassword(ctx context.Context, id primitive.ObjectID, hashedPassword string) error

	// Status management
	SetActiveStatus(ctx context.Context, id primitive.ObjectID, isActive bool) error

	// Statistics
	Count(ctx context.Context) (int64, error)
	CountByRole(ctx context.Context, role models.Role) (int64, error)

	// Validation
	EmailExists(ctx context.Context, email string) (bool, error)
	EmailExistsExcludeID(ctx context.Context, email string, excludeID primitive.ObjectID) (bool, error)
}

// WorkflowRepository defines the contract for workflow data operations
type WorkflowRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, workflow *models.Workflow) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Workflow, error)
	Update(ctx context.Context, id primitive.ObjectID, update map[string]interface{}) error
	Delete(ctx context.Context, id primitive.ObjectID) error

	// List and search operations
	List(ctx context.Context, page, pageSize int) (*models.WorkflowListResponse, error)
	ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.WorkflowListResponse, error)
	ListByStatus(ctx context.Context, status models.WorkflowStatus, page, pageSize int) (*models.WorkflowListResponse, error)
	Search(ctx context.Context, query string, page, pageSize int) (*models.WorkflowListResponse, error)
	SearchByUser(ctx context.Context, userID primitive.ObjectID, query string, page, pageSize int) (*models.WorkflowListResponse, error)

	// Status and execution management
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.WorkflowStatus) error
	UpdateRunStats(ctx context.Context, id primitive.ObjectID, success bool) error
	GetActiveWorkflows(ctx context.Context) ([]*models.Workflow, error)
	GetWorkflowsByTriggerType(ctx context.Context, triggerType models.TriggerType) ([]*models.Workflow, error)

	// Version management
	CreateVersion(ctx context.Context, workflow *models.Workflow) error
	GetVersions(ctx context.Context, workflowID primitive.ObjectID) ([]*models.Workflow, error)

	// Filtering and tags
	ListByTags(ctx context.Context, tags []string, page, pageSize int) (*models.WorkflowListResponse, error)
	GetAllTags(ctx context.Context) ([]string, error)

	// Statistics
	Count(ctx context.Context) (int64, error)
	CountByUser(ctx context.Context, userID primitive.ObjectID) (int64, error)
	CountByStatus(ctx context.Context, status models.WorkflowStatus) (int64, error)

	// Validation
	NameExistsForUser(ctx context.Context, name string, userID primitive.ObjectID) (bool, error)
	NameExistsForUserExcludeID(ctx context.Context, name string, userID primitive.ObjectID, excludeID primitive.ObjectID) (bool, error)
}

// LogRepository defines the contract for log data operations
type LogRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, log *models.WorkflowLog) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.WorkflowLog, error)
	GetByExecutionID(ctx context.Context, executionID string) (*models.WorkflowLog, error)
	Update(ctx context.Context, id primitive.ObjectID, update map[string]interface{}) error
	Delete(ctx context.Context, id primitive.ObjectID) error

	// Methods used by log_service.go
	GetByWorkflowID(ctx context.Context, workflowID primitive.ObjectID, opts PaginationOptions) ([]models.WorkflowLog, int64, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, opts PaginationOptions) ([]models.WorkflowLog, int64, error)
	GetStats(ctx context.Context, filter LogSearchFilter) (*models.LogStats, error)
	Search(ctx context.Context, filter LogSearchFilter, opts PaginationOptions) ([]models.WorkflowLog, int64, error)

	// Query operations
	Query(ctx context.Context, req *models.LogQueryRequest) (*models.LogListResponse, error)
	List(ctx context.Context, page, pageSize int) (*models.LogListResponse, error)
	ListByWorkflow(ctx context.Context, workflowID primitive.ObjectID, page, pageSize int) (*models.LogListResponse, error)
	ListByUser(ctx context.Context, userID primitive.ObjectID, page, pageSize int) (*models.LogListResponse, error)
	ListByStatus(ctx context.Context, status models.ExecutionStatus, page, pageSize int) (*models.LogListResponse, error)

	// Time-based queries
	GetRecentLogs(ctx context.Context, hours int, limit int) ([]*models.WorkflowLog, error)
	GetLogsByDateRange(ctx context.Context, startDate, endDate string, page, pageSize int) (*models.LogListResponse, error)

	// Statistics and analytics
	GetStatistics(ctx context.Context) (*models.LogStatistics, error)
	GetStatisticsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (*models.LogStatistics, error)
	GetStatisticsByUser(ctx context.Context, userID primitive.ObjectID) (*models.LogStatistics, error)
	GetStatisticsByDateRange(ctx context.Context, startDate, endDate string) (*models.LogStatistics, error)

	// Execution tracking
	GetRunningExecutions(ctx context.Context) ([]*models.WorkflowLog, error)
	GetFailedExecutions(ctx context.Context, limit int) ([]*models.WorkflowLog, error)
	GetLastExecution(ctx context.Context, workflowID primitive.ObjectID) (*models.WorkflowLog, error)
	GetExecutionHistory(ctx context.Context, workflowID primitive.ObjectID, limit int) ([]*models.WorkflowLog, error)

	// Cleanup operations
	DeleteOldLogs(ctx context.Context, daysOld int) (int64, error)
	DeleteLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID) (int64, error)

	// Aggregations
	GetExecutionCountByStatus(ctx context.Context) (map[models.ExecutionStatus]int64, error)
	GetExecutionCountByTriggerType(ctx context.Context) (map[models.TriggerType]int64, error)
	GetAverageExecutionDuration(ctx context.Context) (float64, error)
	GetExecutionTrends(ctx context.Context, days int) (map[string]int64, error)

	// Search operations
	SearchLogs(ctx context.Context, searchTerm string, page, pageSize int) (*models.LogListResponse, error)
	SearchLogsByWorkflow(ctx context.Context, workflowID primitive.ObjectID, searchTerm string, page, pageSize int) (*models.LogListResponse, error)
}

// QueueRepository defines the contract for queue operations (Redis)
type QueueRepository interface {
	// Basic queue operations
	Push(ctx context.Context, queueName string, data interface{}) error
	Pop(ctx context.Context, queueName string, timeout int) (interface{}, error)
	PopBlocking(ctx context.Context, queueName string, timeout int) (interface{}, error)

	// Queue management
	Length(ctx context.Context, queueName string) (int64, error)
	Peek(ctx context.Context, queueName string) (interface{}, error)
	Clear(ctx context.Context, queueName string) error

	// Priority queues
	PushPriority(ctx context.Context, queueName string, data interface{}, priority int64) error
	PopPriority(ctx context.Context, queueName string, timeout int) (interface{}, error)

	// Delayed jobs
	PushDelayed(ctx context.Context, queueName string, data interface{}, delay int64) error
	ProcessDelayedJobs(ctx context.Context, queueName string) error

	// Job tracking
	SetJobStatus(ctx context.Context, jobID string, status string) error
	GetJobStatus(ctx context.Context, jobID string) (string, error)

	// Statistics
	GetQueueStats(ctx context.Context, queueName string) (map[string]interface{}, error)
	GetAllQueueNames(ctx context.Context) ([]string, error)

	// Health check
	Ping(ctx context.Context) error
}

// PaginationOptions for consistent pagination
type PaginationOptions struct {
	Page     int    `json:"page" validate:"min=1"`
	PageSize int    `json:"page_size" validate:"min=1,max=100"`
	SortBy   string `json:"sort_by,omitempty"`
	SortDesc bool   `json:"sort_desc,omitempty"`
}

// LogSearchFilter for log searches
type LogSearchFilter struct {
	WorkflowID      *primitive.ObjectID `json:"workflow_id,omitempty"`
	UserID          *primitive.ObjectID `json:"user_id,omitempty"`
	ExecutionID     *string             `json:"execution_id,omitempty"`
	Status          *string             `json:"status,omitempty"`
	Level           *string             `json:"level,omitempty"`
	StartDate       *time.Time          `json:"start_date,omitempty"`
	EndDate         *time.Time          `json:"end_date,omitempty"`
	MessageContains *string             `json:"message_contains,omitempty"`
	SortBy          *string             `json:"sort_by,omitempty"`
	SortOrder       *string             `json:"sort_order,omitempty"`
	Page            *int                `json:"page,omitempty"`
	Limit           *int                `json:"limit,omitempty"`
}

// WorkflowSearchFilters for workflow searches - CORREGIDO: Usar punteros para permitir verificación de nil
type WorkflowSearchFilters struct {
	UserID        *primitive.ObjectID    `json:"user_id,omitempty"`
	Status        *models.WorkflowStatus `json:"status,omitempty"`
	IsActive      *bool                  `json:"is_active,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Query         *string                `json:"query,omitempty"`
	Search        *string                `json:"search,omitempty"`      // AGREGADO: Alias para compatibilidad
	Environment   *string                `json:"environment,omitempty"` // AGREGADO: Campo faltante
	CreatedAfter  *time.Time             `json:"created_after,omitempty"`
	CreatedBefore *time.Time             `json:"created_before,omitempty"`
	Limit         int                    `json:"limit,omitempty"`
	Skip          int                    `json:"skip,omitempty"`
}

// TagCount for popular tags
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// Common repository errors
var (
	// User specific errors
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidUserData   = errors.New("invalid user data")

	// Workflow specific errors
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrWorkflowAlreadyExists = errors.New("workflow already exists")
	ErrInvalidWorkflowData   = errors.New("invalid workflow data")

	// Queue specific errors
	ErrQueueEmpty   = errors.New("queue is empty")
	ErrTaskNotFound = errors.New("task not found")
	ErrTaskTimeout  = errors.New("task processing timeout")

	// Log specific errors
	ErrLogNotFound   = errors.New("log not found")
	ErrInvalidFilter = errors.New("invalid search filter")
)
