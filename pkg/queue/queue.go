package queue

import (
	"context"
	"time"
)

// TaskQueue defines the interface for task queue operations
type TaskQueue interface {
	// Push adds a task to the queue
	Push(ctx context.Context, taskID string) error

	// Pop retrieves a task from the queue with blocking until timeout
	Pop(ctx context.Context, timeout time.Duration) (string, error)

	// Close closes the queue connection
	Close() error
} 