package repository

import (
	"context"

	"Engine_API_Workflow/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRepository defines the contract for user data operations
type UserRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, id primitive.ObjectID, update *models.UpdateUserRequest) error
	Delete(ctx context.Context, id primitive.ObjectID) error

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
	Update(ctx context.Context, id primitive.ObjectID, update *models.UpdateWorkflowRequest) error
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
	Update(ctx context.Context, id primitive.ObjectID, log *models.WorkflowLog) error
	Delete(ctx context.Context, id primitive.ObjectID) error

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

// Repository aggregates all repository interfaces
type Repository struct {
	User     UserRepository
	Workflow WorkflowRepository
	Log      LogRepository
	Queue    QueueRepository
}

// RepositoryManager defines the interface for managing all repositories
type RepositoryManager interface {
	GetUserRepository() UserRepository
	GetWorkflowRepository() WorkflowRepository
	GetLogRepository() LogRepository
	GetQueueRepository() QueueRepository
	Close() error
	Ping(ctx context.Context) error
}

// TransactionManager defines the interface for database transactions
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// CacheRepository defines the contract for caching operations
type CacheRepository interface {
	// Basic cache operations
	Set(ctx context.Context, key string, value interface{}, expiration int64) error
	Get(ctx context.Context, key string) (interface{}, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Batch operations
	SetMultiple(ctx context.Context, data map[string]interface{}, expiration int64) error
	GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)
	DeleteMultiple(ctx context.Context, keys []string) error

	// Pattern operations
	DeleteByPattern(ctx context.Context, pattern string) error
	GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)

	// Expiration management
	SetExpiration(ctx context.Context, key string, expiration int64) error
	GetTTL(ctx context.Context, key string) (int64, error)

	// Statistics
	GetStats(ctx context.Context) (map[string]interface{}, error)

	// Health check
	Ping(ctx context.Context) error
}
