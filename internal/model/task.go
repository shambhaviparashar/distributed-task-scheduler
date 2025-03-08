package model

import (
	"math/rand"
	"time"
)

// Task represents a scheduled unit of work
type Task struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Command      string                 `json:"command"`
	Schedule     string                 `json:"schedule"` // Cron expression
	Retries      int                    `json:"retries"`
	Timeout      int                    `json:"timeout"` // In seconds
	Dependencies []string               `json:"dependencies"` // IDs of tasks that must complete first
	Status       string                 `json:"status"`
	Result       string                 `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	ExitCode     int                    `json:"exit_code"`
	RetryCount   int                    `json:"retry_count"`
	Priority     int                    `json:"priority"`
	WorkerID     string                 `json:"worker_id,omitempty"` // ID of worker executing this task
	LastHeartbeat time.Time             `json:"last_heartbeat,omitempty"` // Last heartbeat from worker
	Checkpoint   map[string]interface{} `json:"checkpoint,omitempty"` // Checkpoint data for resumable tasks
	Version      int                    `json:"version"` // For optimistic concurrency control
	QueuedAt     time.Time              `json:"queued_at,omitempty"`
	StartedAt    time.Time              `json:"started_at,omitempty"`
	FinishedAt   time.Time              `json:"finished_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// TaskStatus constants
const (
	TaskStatusPending   = "pending"
	TaskStatusScheduled = "scheduled"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
	TaskStatusTimedOut  = "timed_out"
	TaskStatusRetrying  = "retrying"
	TaskStatusPaused    = "paused"
	TaskStatusBlocked   = "blocked" // Blocked by dependencies
	TaskStatusDeadLetter = "dead_letter" // Too many retries
)

// TaskPriority constants
const (
	PriorityHigh   = 10
	PriorityNormal = 5
	PriorityLow    = 1
)

// NewTask creates a new task with defaults
func NewTask(name, command, schedule string) *Task {
	now := time.Now()
	return &Task{
		ID:        "", // Will be assigned by the repository
		Name:      name,
		Command:   command,
		Schedule:  schedule,
		Retries:   3, // Default to 3 retries
		Timeout:   300, // Default timeout of 5 minutes
		Status:    TaskStatusPending,
		Priority:  PriorityNormal,
		Version:   1,
		RetryCount: 0,
		ExitCode:  -1, // -1 indicates not executed yet
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsFailed returns true if the task has permanently failed
func (t *Task) IsFailed() bool {
	return t.Status == TaskStatusFailed || t.Status == TaskStatusDeadLetter
}

// IsActive returns true if the task is currently being processed
func (t *Task) IsActive() bool {
	return t.Status == TaskStatusRunning || t.Status == TaskStatusRetrying
}

// IsComplete returns true if the task has completed successfully
func (t *Task) IsComplete() bool {
	return t.Status == TaskStatusCompleted
}

// IsTerminal returns true if the task is in a terminal state (no more processing)
func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted || 
	       t.Status == TaskStatusFailed || 
	       t.Status == TaskStatusCancelled || 
	       t.Status == TaskStatusDeadLetter
}

// ShouldRetry determines if a task should be retried based on its current state
func (t *Task) ShouldRetry() bool {
	if t.IsTerminal() {
		return false
	}
	
	return t.RetryCount < t.Retries
}

// CalculateBackoff returns the backoff duration for the next retry attempt
// Implements exponential backoff with jitter
func (t *Task) CalculateBackoff() time.Duration {
	// Base backoff duration (starts at 1 second)
	base := time.Second
	
	// Calculate exponential backoff: 2^retry_count seconds
	// Cap at 5 minutes to avoid extremely long waits
	maxBackoff := 5 * time.Minute
	
	// 2^retry_count (capped)
	multiplier := 1
	for i := 0; i < t.RetryCount && multiplier < 300; i++ {
		multiplier *= 2
	}
	
	backoff := base * time.Duration(multiplier)
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	
	// Add jitter (±20%)
	jitter := time.Duration(float64(backoff) * 0.2 * (0.5 - rand.Float64()))
	backoff += jitter
	
	return backoff
}

// SetCheckpoint stores checkpoint data for resumable tasks
func (t *Task) SetCheckpoint(data map[string]interface{}) {
	if t.Checkpoint == nil {
		t.Checkpoint = make(map[string]interface{})
	}
	
	// Copy data to checkpoint
	for k, v := range data {
		t.Checkpoint[k] = v
	}
	
	t.UpdatedAt = time.Now()
} 