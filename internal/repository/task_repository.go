package repository

import (
	"context"
	"errors"
	"time"

	"github.com/yourusername/taskscheduler/internal/model"
)

// Common repository errors
var (
	// ErrTaskNotFound indicates that a task was not found in the repository
	ErrTaskNotFound = errors.New("task not found")
	
	// ErrConcurrencyConflict indicates an optimistic concurrency control failure
	ErrConcurrencyConflict = errors.New("task was modified by another operation")
	
	// ErrTransactionFailed indicates that a database transaction failed
	ErrTransactionFailed = errors.New("transaction failed")
)

// TaskRepository defines the interface for task persistence operations
type TaskRepository interface {
	// Create adds a new task to the repository
	Create(ctx context.Context, task *model.Task) error

	// GetByID retrieves a task by its ID
	GetByID(ctx context.Context, id string) (*model.Task, error)

	// List retrieves all tasks, with optional filtering
	List(ctx context.Context, limit, offset int) ([]*model.Task, error)

	// Update modifies an existing task with optimistic concurrency control
	Update(ctx context.Context, task *model.Task) error

	// Delete removes a task from the repository
	Delete(ctx context.Context, id string) error

	// GetDueTasksWithoutDependencies retrieves tasks that are scheduled to run
	// and have no unresolved dependencies
	GetDueTasksWithoutDependencies(ctx context.Context) ([]*model.Task, error)

	// UpdateStatus changes the status of a task
	UpdateStatus(ctx context.Context, id, status string) error
	
	// UpdateTaskWithWorker updates a task with worker assignment and status
	UpdateTaskWithWorker(ctx context.Context, taskID, workerID, status string) error
	
	// ReleaseTask releases a task from a worker
	ReleaseTask(ctx context.Context, taskID, workerID string) error
	
	// GetTasksByStatus retrieves tasks with a specific status
	GetTasksByStatus(ctx context.Context, status string, limit int) ([]*model.Task, error)
	
	// GetTasksByWorker retrieves tasks assigned to a specific worker
	GetTasksByWorker(ctx context.Context, workerID string) ([]*model.Task, error)
	
	// GetStaleTasks retrieves tasks that have not received a heartbeat for the specified duration
	GetStaleTasks(ctx context.Context, cutoff time.Duration) ([]*model.Task, error)
	
	// BatchUpdateStatus updates the status of multiple tasks in a single transaction
	BatchUpdateStatus(ctx context.Context, taskIDs []string, status string) error
	
	// IncrementRetryCount atomically increments a task's retry count
	IncrementRetryCount(ctx context.Context, taskID string) (int, error)
	
	// SaveCheckpoint saves a task's checkpoint data
	SaveCheckpoint(ctx context.Context, taskID string, checkpoint map[string]interface{}) error
	
	// GetDeadLetterTasks retrieves tasks that are in the dead letter state
	GetDeadLetterTasks(ctx context.Context, limit int) ([]*model.Task, error)
} 