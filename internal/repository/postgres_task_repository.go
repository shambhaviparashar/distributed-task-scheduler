package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/taskscheduler/internal/model"
)

// ErrTaskNotFound indicates that a task was not found in the repository
var ErrTaskNotFound = errors.New("task not found")

// PostgresTaskRepository implements TaskRepository using PostgreSQL
type PostgresTaskRepository struct {
	db *sql.DB
}

// NewPostgresTaskRepository creates a new PostgreSQL repository
func NewPostgresTaskRepository(db *sql.DB) *PostgresTaskRepository {
	return &PostgresTaskRepository{db: db}
}

// Create adds a new task to the repository
func (r *PostgresTaskRepository) Create(ctx context.Context, task *model.Task) error {
	query := `
		INSERT INTO tasks (
			id, name, command, schedule, retries, timeout, dependencies, status,
			retry_count, priority, exit_code, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		) RETURNING id
	`

	// Generate UUID if not provided
	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	// Initialize version
	task.Version = 1

	// Convert dependencies slice to array format for PostgreSQL
	deps := "{}"
	if len(task.Dependencies) > 0 {
		deps = fmt.Sprintf("{%s}", joinStringSlice(task.Dependencies))
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	_, err := r.db.ExecContext(
		ctx,
		query,
		task.ID,
		task.Name,
		task.Command,
		task.Schedule,
		task.Retries,
		task.Timeout,
		deps, // This will be handled as a text array in PostgreSQL
		task.Status,
		task.RetryCount,
		task.Priority,
		task.ExitCode,
		task.Version,
		task.CreatedAt,
		task.UpdatedAt,
	)

	return err
}

// GetByID retrieves a task by its ID
func (r *PostgresTaskRepository) GetByID(ctx context.Context, id string) (*model.Task, error) {
	query := `
		SELECT id, name, command, schedule, retries, timeout, dependencies, status,
		       result, error, exit_code, retry_count, priority, worker_id, 
		       last_heartbeat, checkpoint, version, queued_at, started_at, 
		       finished_at, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)

	task := &model.Task{}
	var depsStr, resultStr, errorStr, workerIDStr string
	var checkpointJSON []byte
	var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime

	err := row.Scan(
		&task.ID,
		&task.Name,
		&task.Command,
		&task.Schedule,
		&task.Retries,
		&task.Timeout,
		&depsStr,
		&task.Status,
		&resultStr,
		&errorStr,
		&task.ExitCode,
		&task.RetryCount,
		&task.Priority,
		&workerIDStr,
		&lastHeartbeat,
		&checkpointJSON,
		&task.Version,
		&queuedAt,
		&startedAt,
		&finishedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	// Set optional fields
	task.Result = resultStr
	task.Error = errorStr
	task.WorkerID = workerIDStr
	
	// Set time fields if they are valid
	if lastHeartbeat.Valid {
		task.LastHeartbeat = lastHeartbeat.Time
	}
	if queuedAt.Valid {
		task.QueuedAt = queuedAt.Time
	}
	if startedAt.Valid {
		task.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		task.FinishedAt = finishedAt.Time
	}

	// Parse dependencies from string format
	task.Dependencies = parseStringArray(depsStr)
	
	// Parse checkpoint data if available
	if len(checkpointJSON) > 0 {
		var checkpoint map[string]interface{}
		if err := json.Unmarshal(checkpointJSON, &checkpoint); err == nil {
			task.Checkpoint = checkpoint
		}
	}

	return task, nil
}

// List retrieves all tasks, with optional filtering
func (r *PostgresTaskRepository) List(ctx context.Context, limit, offset int) ([]*model.Task, error) {
	query := `
		SELECT id, name, command, schedule, retries, timeout, dependencies, status,
		       result, error, exit_code, retry_count, priority, worker_id, 
		       last_heartbeat, checkpoint, version, queued_at, started_at, 
		       finished_at, created_at, updated_at
		FROM tasks
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []*model.Task{}

	for rows.Next() {
		task := &model.Task{}
		var depsStr, resultStr, errorStr, workerIDStr string
		var checkpointJSON []byte
		var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Command,
			&task.Schedule,
			&task.Retries,
			&task.Timeout,
			&depsStr,
			&task.Status,
			&resultStr,
			&errorStr,
			&task.ExitCode,
			&task.RetryCount,
			&task.Priority,
			&workerIDStr,
			&lastHeartbeat,
			&checkpointJSON,
			&task.Version,
			&queuedAt,
			&startedAt,
			&finishedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Set optional fields
		task.Result = resultStr
		task.Error = errorStr
		task.WorkerID = workerIDStr
		
		// Set time fields if they are valid
		if lastHeartbeat.Valid {
			task.LastHeartbeat = lastHeartbeat.Time
		}
		if queuedAt.Valid {
			task.QueuedAt = queuedAt.Time
		}
		if startedAt.Valid {
			task.StartedAt = startedAt.Time
		}
		if finishedAt.Valid {
			task.FinishedAt = finishedAt.Time
		}

		// Parse dependencies
		task.Dependencies = parseStringArray(depsStr)
		
		// Parse checkpoint data if available
		if len(checkpointJSON) > 0 {
			var checkpoint map[string]interface{}
			if err := json.Unmarshal(checkpointJSON, &checkpoint); err == nil {
				task.Checkpoint = checkpoint
			}
		}
		
		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// Update modifies an existing task with optimistic concurrency control
func (r *PostgresTaskRepository) Update(ctx context.Context, task *model.Task) error {
	query := `
		UPDATE tasks
		SET
			name = $1,
			command = $2,
			schedule = $3,
			retries = $4,
			timeout = $5,
			dependencies = $6,
			status = $7,
			result = $8,
			error = $9,
			exit_code = $10,
			retry_count = $11,
			priority = $12,
			worker_id = $13,
			last_heartbeat = $14,
			checkpoint = $15,
			version = $16,
			queued_at = $17,
			started_at = $18,
			finished_at = $19,
			updated_at = $20
		WHERE id = $21 AND version = $22
	`

	// Convert dependencies slice to array format
	deps := "{}"
	if len(task.Dependencies) > 0 {
		deps = fmt.Sprintf("{%s}", joinStringSlice(task.Dependencies))
	}
	
	// Serialize checkpoint data
	var checkpointJSON []byte
	var err error
	if task.Checkpoint != nil {
		checkpointJSON, err = json.Marshal(task.Checkpoint)
		if err != nil {
			return fmt.Errorf("failed to serialize checkpoint data: %w", err)
		}
	}
	
	// Increment version for optimistic concurrency control
	oldVersion := task.Version
	task.Version++
	task.UpdatedAt = time.Now()
	
	// Convert time fields to SQL values
	var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime
	
	if !task.LastHeartbeat.IsZero() {
		lastHeartbeat.Time = task.LastHeartbeat
		lastHeartbeat.Valid = true
	}
	if !task.QueuedAt.IsZero() {
		queuedAt.Time = task.QueuedAt
		queuedAt.Valid = true
	}
	if !task.StartedAt.IsZero() {
		startedAt.Time = task.StartedAt
		startedAt.Valid = true
	}
	if !task.FinishedAt.IsZero() {
		finishedAt.Time = task.FinishedAt
		finishedAt.Valid = true
	}

	result, err := r.db.ExecContext(
		ctx,
		query,
		task.Name,
		task.Command,
		task.Schedule,
		task.Retries,
		task.Timeout,
		deps,
		task.Status,
		task.Result,
		task.Error,
		task.ExitCode,
		task.RetryCount,
		task.Priority,
		task.WorkerID,
		lastHeartbeat,
		checkpointJSON,
		task.Version,
		queuedAt,
		startedAt,
		finishedAt,
		task.UpdatedAt,
		task.ID,
		oldVersion,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		// Check if task exists
		exists, err := r.taskExists(ctx, task.ID)
		if err != nil {
			return err
		}
		
		if !exists {
			return ErrTaskNotFound
		}
		
		// Task exists but version doesn't match
		return ErrConcurrencyConflict
	}

	return nil
}

// taskExists checks if a task with the given ID exists
func (r *PostgresTaskRepository) taskExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	return exists, err
}

// Delete removes a task from the repository
func (r *PostgresTaskRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// GetDueTasksWithoutDependencies retrieves tasks that are scheduled to run and have no unresolved dependencies
func (r *PostgresTaskRepository) GetDueTasksWithoutDependencies(ctx context.Context) ([]*model.Task, error) {
	query := `
		SELECT id, name, command, schedule, retries, timeout, dependencies, status,
		       result, error, exit_code, retry_count, priority, worker_id, 
		       last_heartbeat, checkpoint, version, queued_at, started_at, 
		       finished_at, created_at, updated_at
		FROM tasks
		WHERE status = $1
		AND (schedule = '' OR schedule IS NOT NULL) -- Tasks with valid schedules
		-- In a real implementation, we would check if the task is due based on the cron expression
		-- and if all dependencies are resolved
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, model.TaskStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []*model.Task{}

	for rows.Next() {
		task := &model.Task{}
		var depsStr, resultStr, errorStr, workerIDStr string
		var checkpointJSON []byte
		var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Command,
			&task.Schedule,
			&task.Retries,
			&task.Timeout,
			&depsStr,
			&task.Status,
			&resultStr,
			&errorStr,
			&task.ExitCode,
			&task.RetryCount,
			&task.Priority,
			&workerIDStr,
			&lastHeartbeat,
			&checkpointJSON,
			&task.Version,
			&queuedAt,
			&startedAt,
			&finishedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Set optional fields
		task.Result = resultStr
		task.Error = errorStr
		task.WorkerID = workerIDStr
		
		// Set time fields if they are valid
		if lastHeartbeat.Valid {
			task.LastHeartbeat = lastHeartbeat.Time
		}
		if queuedAt.Valid {
			task.QueuedAt = queuedAt.Time
		}
		if startedAt.Valid {
			task.StartedAt = startedAt.Time
		}
		if finishedAt.Valid {
			task.FinishedAt = finishedAt.Time
		}

		// Parse dependencies
		task.Dependencies = parseStringArray(depsStr)
		
		// Parse checkpoint data if available
		if len(checkpointJSON) > 0 {
			var checkpoint map[string]interface{}
			if err := json.Unmarshal(checkpointJSON, &checkpoint); err == nil {
				task.Checkpoint = checkpoint
			}
		}
		
		// In a real implementation, we would check if dependencies are resolved
		// For now, just return tasks without dependencies
		if len(task.Dependencies) == 0 {
			tasks = append(tasks, task)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// UpdateStatus changes the status of a task
func (r *PostgresTaskRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE tasks
		SET status = $1, updated_at = $2, version = version + 1
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// UpdateTaskWithWorker updates a task with worker assignment and status
func (r *PostgresTaskRepository) UpdateTaskWithWorker(ctx context.Context, taskID, workerID, status string) error {
	query := `
		UPDATE tasks
		SET 
			status = $1, 
			worker_id = $2, 
			last_heartbeat = $3, 
			updated_at = $3,
			version = version + 1,
			started_at = CASE WHEN started_at IS NULL THEN $3 ELSE started_at END
		WHERE id = $4
		AND (worker_id IS NULL OR worker_id = $2)
	`
	
	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, status, workerID, now, taskID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		// Check if task exists
		exists, err := r.taskExists(ctx, taskID)
		if err != nil {
			return err
		}
		
		if !exists {
			return ErrTaskNotFound
		}
		
		// Task exists but is assigned to another worker
		return fmt.Errorf("task is assigned to another worker")
	}
	
	return nil
}

// ReleaseTask releases a task from a worker
func (r *PostgresTaskRepository) ReleaseTask(ctx context.Context, taskID, workerID string) error {
	query := `
		UPDATE tasks
		SET 
			worker_id = NULL, 
			updated_at = $1,
			version = version + 1
		WHERE id = $2
		AND worker_id = $3
	`
	
	result, err := r.db.ExecContext(ctx, query, time.Now(), taskID, workerID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		// Check if task exists
		exists, err := r.taskExists(ctx, taskID)
		if err != nil {
			return err
		}
		
		if !exists {
			return ErrTaskNotFound
		}
		
		// Task exists but is not assigned to the specified worker
		return fmt.Errorf("task is not assigned to the specified worker")
	}
	
	return nil
}

// GetTasksByStatus retrieves tasks with a specific status
func (r *PostgresTaskRepository) GetTasksByStatus(ctx context.Context, status string, limit int) ([]*model.Task, error) {
	query := `
		SELECT id, name, command, schedule, retries, timeout, dependencies, status,
		       result, error, exit_code, retry_count, priority, worker_id, 
		       last_heartbeat, checkpoint, version, queued_at, started_at, 
		       finished_at, created_at, updated_at
		FROM tasks
		WHERE status = $1
		ORDER BY priority DESC, created_at ASC
		LIMIT $2
	`
	
	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	// Reuse the scan logic from List method
	tasks := []*model.Task{}
	
	for rows.Next() {
		task := &model.Task{}
		var depsStr, resultStr, errorStr, workerIDStr string
		var checkpointJSON []byte
		var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Command,
			&task.Schedule,
			&task.Retries,
			&task.Timeout,
			&depsStr,
			&task.Status,
			&resultStr,
			&errorStr,
			&task.ExitCode,
			&task.RetryCount,
			&task.Priority,
			&workerIDStr,
			&lastHeartbeat,
			&checkpointJSON,
			&task.Version,
			&queuedAt,
			&startedAt,
			&finishedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Set optional fields
		task.Result = resultStr
		task.Error = errorStr
		task.WorkerID = workerIDStr
		
		// Set time fields if they are valid
		if lastHeartbeat.Valid {
			task.LastHeartbeat = lastHeartbeat.Time
		}
		if queuedAt.Valid {
			task.QueuedAt = queuedAt.Time
		}
		if startedAt.Valid {
			task.StartedAt = startedAt.Time
		}
		if finishedAt.Valid {
			task.FinishedAt = finishedAt.Time
		}

		// Parse dependencies
		task.Dependencies = parseStringArray(depsStr)
		
		// Parse checkpoint data if available
		if len(checkpointJSON) > 0 {
			var checkpoint map[string]interface{}
			if err := json.Unmarshal(checkpointJSON, &checkpoint); err == nil {
				task.Checkpoint = checkpoint
			}
		}
		
		tasks = append(tasks, task)
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return tasks, nil
}

// GetTasksByWorker retrieves tasks assigned to a specific worker
func (r *PostgresTaskRepository) GetTasksByWorker(ctx context.Context, workerID string) ([]*model.Task, error) {
	query := `
		SELECT id, name, command, schedule, retries, timeout, dependencies, status,
		       result, error, exit_code, retry_count, priority, worker_id, 
		       last_heartbeat, checkpoint, version, queued_at, started_at, 
		       finished_at, created_at, updated_at
		FROM tasks
		WHERE worker_id = $1
		ORDER BY priority DESC, created_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	// Reuse the scan logic from List method
	tasks := []*model.Task{}
	
	for rows.Next() {
		task := &model.Task{}
		var depsStr, resultStr, errorStr, workerIDStr string
		var checkpointJSON []byte
		var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Command,
			&task.Schedule,
			&task.Retries,
			&task.Timeout,
			&depsStr,
			&task.Status,
			&resultStr,
			&errorStr,
			&task.ExitCode,
			&task.RetryCount,
			&task.Priority,
			&workerIDStr,
			&lastHeartbeat,
			&checkpointJSON,
			&task.Version,
			&queuedAt,
			&startedAt,
			&finishedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Set optional fields
		task.Result = resultStr
		task.Error = errorStr
		task.WorkerID = workerIDStr
		
		// Set time fields if they are valid
		if lastHeartbeat.Valid {
			task.LastHeartbeat = lastHeartbeat.Time
		}
		if queuedAt.Valid {
			task.QueuedAt = queuedAt.Time
		}
		if startedAt.Valid {
			task.StartedAt = startedAt.Time
		}
		if finishedAt.Valid {
			task.FinishedAt = finishedAt.Time
		}

		// Parse dependencies
		task.Dependencies = parseStringArray(depsStr)
		
		// Parse checkpoint data if available
		if len(checkpointJSON) > 0 {
			var checkpoint map[string]interface{}
			if err := json.Unmarshal(checkpointJSON, &checkpoint); err == nil {
				task.Checkpoint = checkpoint
			}
		}
		
		tasks = append(tasks, task)
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return tasks, nil
}

// GetStaleTasks retrieves tasks that have not received a heartbeat for the specified duration
func (r *PostgresTaskRepository) GetStaleTasks(ctx context.Context, cutoff time.Duration) ([]*model.Task, error) {
	query := `
		SELECT id, name, command, schedule, retries, timeout, dependencies, status,
		       result, error, exit_code, retry_count, priority, worker_id, 
		       last_heartbeat, checkpoint, version, queued_at, started_at, 
		       finished_at, created_at, updated_at
		FROM tasks
		WHERE status = $1
		AND worker_id IS NOT NULL
		AND last_heartbeat < $2
	`
	
	cutoffTime := time.Now().Add(-cutoff)
	rows, err := r.db.QueryContext(ctx, query, model.TaskStatusRunning, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	// Reuse the scan logic from List method
	tasks := []*model.Task{}
	
	for rows.Next() {
		task := &model.Task{}
		var depsStr, resultStr, errorStr, workerIDStr string
		var checkpointJSON []byte
		var lastHeartbeat, queuedAt, startedAt, finishedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Command,
			&task.Schedule,
			&task.Retries,
			&task.Timeout,
			&depsStr,
			&task.Status,
			&resultStr,
			&errorStr,
			&task.ExitCode,
			&task.RetryCount,
			&task.Priority,
			&workerIDStr,
			&lastHeartbeat,
			&checkpointJSON,
			&task.Version,
			&queuedAt,
			&startedAt,
			&finishedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Set optional fields
		task.Result = resultStr
		task.Error = errorStr
		task.WorkerID = workerIDStr
		
		// Set time fields if they are valid
		if lastHeartbeat.Valid {
			task.LastHeartbeat = lastHeartbeat.Time
		}
		if queuedAt.Valid {
			task.QueuedAt = queuedAt.Time
		}
		if startedAt.Valid {
			task.StartedAt = startedAt.Time
		}
		if finishedAt.Valid {
			task.FinishedAt = finishedAt.Time
		}

		// Parse dependencies
		task.Dependencies = parseStringArray(depsStr)
		
		// Parse checkpoint data if available
		if len(checkpointJSON) > 0 {
			var checkpoint map[string]interface{}
			if err := json.Unmarshal(checkpointJSON, &checkpoint); err == nil {
				task.Checkpoint = checkpoint
			}
		}
		
		tasks = append(tasks, task)
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return tasks, nil
}

// BatchUpdateStatus updates the status of multiple tasks in a single transaction
func (r *PostgresTaskRepository) BatchUpdateStatus(ctx context.Context, taskIDs []string, status string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	
	// Defer rollback in case of error (no-op if committed successfully)
	defer tx.Rollback()
	
	// Prepare query
	query := `
		UPDATE tasks
		SET status = $1, updated_at = $2, version = version + 1
		WHERE id = ANY($3)
	`
	
	// Convert Go string slice to PostgreSQL array
	idArray := fmt.Sprintf("{%s}", joinStringSlice(taskIDs))
	
	// Execute update
	_, err = tx.ExecContext(ctx, query, status, time.Now(), idArray)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}
	
	return nil
}

// IncrementRetryCount atomically increments a task's retry count
func (r *PostgresTaskRepository) IncrementRetryCount(ctx context.Context, taskID string) (int, error) {
	query := `
		UPDATE tasks
		SET retry_count = retry_count + 1, updated_at = $1, version = version + 1
		WHERE id = $2
		RETURNING retry_count
	`
	
	var retryCount int
	err := r.db.QueryRowContext(ctx, query, time.Now(), taskID).Scan(&retryCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, ErrTaskNotFound
		}
		return 0, err
	}
	
	return retryCount, nil
}

// SaveCheckpoint saves a task's checkpoint data
func (r *PostgresTaskRepository) SaveCheckpoint(ctx context.Context, taskID string, checkpoint map[string]interface{}) error {
	query := `
		UPDATE tasks
		SET checkpoint = $1, updated_at = $2, version = version + 1
		WHERE id = $3
	`
	
	// Serialize checkpoint data
	checkpointJSON, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("failed to serialize checkpoint data: %w", err)
	}
	
	result, err := r.db.ExecContext(ctx, query, checkpointJSON, time.Now(), taskID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	
	return nil
}

// GetDeadLetterTasks retrieves tasks that are in the dead letter state
func (r *PostgresTaskRepository) GetDeadLetterTasks(ctx context.Context, limit int) ([]*model.Task, error) {
	return r.GetTasksByStatus(ctx, model.TaskStatusDeadLetter, limit)
}

// Helper functions

// joinStringSlice joins slice elements with quotes for SQL
func joinStringSlice(slice []string) string {
	result := ""
	for i, s := range slice {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("\"%s\"", s)
	}
	return result
}

// parseStringArray parses PostgreSQL array string into Go slice
// This is a simplified version; in production, consider using a library
func parseStringArray(input string) []string {
	// Placeholder implementation - in a real system use proper parsing
	// This is highly simplified and would not work with complex arrays
	if input == "{}" || input == "" {
		return []string{}
	}

	// Remove { and } and split by comma
	trimmed := input[1 : len(input)-1]
	// In a real implementation, we would properly handle quoted strings and escaping
	// TODO: Implement proper parsing of PostgreSQL arrays
	
	return []string{trimmed} // Placeholder
} 