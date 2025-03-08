package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// PriorityLevels defines the different priority levels
const (
	PriorityHigh   = 10
	PriorityNormal = 5
	PriorityLow    = 1
	
	DefaultPriorityQueuePrefix = "task_queue:"
)

// TaskItem represents an item in the priority queue
type TaskItem struct {
	ID       string                 `json:"id"`
	Priority int                    `json:"priority"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Added    time.Time              `json:"added"`
}

// PriorityQueue implements a priority queue using Redis sorted sets
type PriorityQueue struct {
	client    *redis.Client
	queueName string
	prefix    string
}

// PriorityQueueConfig holds configuration for the priority queue
type PriorityQueueConfig struct {
	Client    *redis.Client
	QueueName string
	Prefix    string
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(config PriorityQueueConfig) *PriorityQueue {
	prefix := config.Prefix
	if prefix == "" {
		prefix = DefaultPriorityQueuePrefix
	}
	
	return &PriorityQueue{
		client:    config.Client,
		queueName: config.QueueName,
		prefix:    prefix,
	}
}

// getQueueKey returns the Redis key for a specific priority queue
func (q *PriorityQueue) getQueueKey() string {
	return q.prefix + q.queueName
}

// getTaskDataKey returns the Redis key for storing task data
func (q *PriorityQueue) getTaskDataKey(taskID string) string {
	return fmt.Sprintf("%stask:%s", q.prefix, taskID)
}

// Push adds a task to the priority queue
func (q *PriorityQueue) Push(ctx context.Context, taskID string, priority int, data map[string]interface{}) error {
	// Store the task in the priority queue using a sorted set
	// Higher score = higher priority
	score := float64(priority)
	
	// Create a pipe for multiple operations
	pipe := q.client.Pipeline()
	
	// Add task to the sorted set
	pipe.ZAdd(ctx, q.getQueueKey(), &redis.Z{
		Score:  score,
		Member: taskID,
	})
	
	// Store task data in a separate key if provided
	if data != nil {
		taskItem := &TaskItem{
			ID:       taskID,
			Priority: priority,
			Data:     data,
			Added:    time.Now(),
		}
		
		taskJSON, err := json.Marshal(taskItem)
		if err != nil {
			return fmt.Errorf("failed to marshal task data: %w", err)
		}
		
		pipe.Set(ctx, q.getTaskDataKey(taskID), taskJSON, 24*time.Hour) // 24-hour expiration for task data
	}
	
	// Execute the pipe commands
	_, err := pipe.Exec(ctx)
	return err
}

// Pop retrieves and removes the highest priority task from the queue
func (q *PriorityQueue) Pop(ctx context.Context, timeout time.Duration) (*TaskItem, error) {
	// Use BZPOPMAX to pop the highest-scored item with blocking until timeout
	// Unfortunately, Redis doesn't provide BZPOPMAX, so we need to simulate it
	
	// Try to pop immediately first
	result, err := q.tryPop(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}
	
	// If we got a task, return it
	if result != nil {
		return result, nil
	}
	
	// If there's no task, set up a ticker to periodically check
	// This isn't as efficient as a true blocking operation but it simulates it
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	timeoutCh := time.After(timeout)
	
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
			
		case <-timeoutCh:
			return nil, ErrQueueEmpty
			
		case <-ticker.C:
			result, err := q.tryPop(ctx)
			if err != nil && err != redis.Nil {
				return nil, err
			}
			
			if result != nil {
				return result, nil
			}
		}
	}
}

// tryPop attempts to pop the highest priority task
func (q *PriorityQueue) tryPop(ctx context.Context) (*TaskItem, error) {
	// Use a transaction to ensure atomicity
	txf := func(tx *redis.Tx) error {
		// Get the highest priority item
		zRangeResult, err := tx.ZRevRangeWithScores(ctx, q.getQueueKey(), 0, 0).Result()
		if err != nil {
			return err
		}
		
		// If there are no items, return
		if len(zRangeResult) == 0 {
			return redis.Nil
		}
		
		// Remove the item from the sorted set
		member := zRangeResult[0].Member
		_, err = tx.ZRem(ctx, q.getQueueKey(), member).Result()
		return err
	}
	
	// Run the transaction with optimistic locking
	taskID := ""
	priority := 0
	
	err := q.client.Watch(ctx, txf, q.getQueueKey())
	if err == redis.Nil {
		return nil, ErrQueueEmpty
	}
	if err != nil {
		return nil, err
	}
	
	zRangeResult, err := q.client.ZRevRangeWithScores(ctx, q.getQueueKey(), 0, 0).Result()
	if err != nil {
		return nil, err
	}
	
	if len(zRangeResult) > 0 {
		taskID = zRangeResult[0].Member.(string)
		priority = int(zRangeResult[0].Score)
	}
	
	// Get task data if available
	var taskItem *TaskItem
	
	dataKey := q.getTaskDataKey(taskID)
	taskData, err := q.client.Get(ctx, dataKey).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	
	if err == redis.Nil || taskData == "" {
		// No task data found, create a minimal task item
		taskItem = &TaskItem{
			ID:       taskID,
			Priority: priority,
			Added:    time.Now(), // We don't know when it was actually added
		}
	} else {
		// Parse task data
		if err := json.Unmarshal([]byte(taskData), &taskItem); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task data: %w", err)
		}
		
		// Delete task data after retrieving it
		q.client.Del(ctx, dataKey)
	}
	
	return taskItem, nil
}

// Peek returns the highest priority task without removing it
func (q *PriorityQueue) Peek(ctx context.Context) (*TaskItem, error) {
	// Get the highest priority item
	zRangeResult, err := q.client.ZRevRangeWithScores(ctx, q.getQueueKey(), 0, 0).Result()
	if err != nil {
		return nil, err
	}
	
	// If there are no items, return empty
	if len(zRangeResult) == 0 {
		return nil, ErrQueueEmpty
	}
	
	taskID := zRangeResult[0].Member.(string)
	priority := int(zRangeResult[0].Score)
	
	// Get task data if available
	var taskItem *TaskItem
	
	dataKey := q.getTaskDataKey(taskID)
	taskData, err := q.client.Get(ctx, dataKey).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	
	if err == redis.Nil || taskData == "" {
		// No task data found, create a minimal task item
		taskItem = &TaskItem{
			ID:       taskID,
			Priority: priority,
			Added:    time.Now(), // We don't know when it was actually added
		}
	} else {
		// Parse task data
		if err := json.Unmarshal([]byte(taskData), &taskItem); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task data: %w", err)
		}
	}
	
	return taskItem, nil
}

// Size returns the number of items in the queue
func (q *PriorityQueue) Size(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, q.getQueueKey()).Result()
}

// Close closes the priority queue
func (q *PriorityQueue) Close() error {
	// Nothing to close in this implementation
	return nil
}

// BatchPush adds multiple tasks to the priority queue
func (q *PriorityQueue) BatchPush(ctx context.Context, tasks []*TaskItem) error {
	if len(tasks) == 0 {
		return nil
	}
	
	// Create a pipe for multiple operations
	pipe := q.client.Pipeline()
	
	// Add all tasks to the queue
	zAddMembers := make([]*redis.Z, 0, len(tasks))
	for _, task := range tasks {
		zAddMembers = append(zAddMembers, &redis.Z{
			Score:  float64(task.Priority),
			Member: task.ID,
		})
		
		// Store task data in a separate key
		if task.Data != nil {
			taskJSON, err := json.Marshal(task)
			if err != nil {
				return fmt.Errorf("failed to marshal task data: %w", err)
			}
			
			pipe.Set(ctx, q.getTaskDataKey(task.ID), taskJSON, 24*time.Hour)
		}
	}
	
	// Add all tasks to the sorted set
	pipe.ZAdd(ctx, q.getQueueKey(), zAddMembers...)
	
	// Execute the pipe commands
	_, err := pipe.Exec(ctx)
	return err
}

// Remove removes a task from the queue
func (q *PriorityQueue) Remove(ctx context.Context, taskID string) error {
	// Remove task from the sorted set and its data
	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, q.getQueueKey(), taskID)
	pipe.Del(ctx, q.getTaskDataKey(taskID))
	
	_, err := pipe.Exec(ctx)
	return err
}

// GetQueueStats returns statistics about the queue
func (q *PriorityQueue) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	pipe := q.client.Pipeline()
	
	// Total queue size
	sizeCmd := pipe.ZCard(ctx, q.getQueueKey())
	
	// Get number of tasks by priority range
	highPriorityCmd := pipe.ZCount(ctx, q.getQueueKey(), strconv.Itoa(PriorityHigh), "+inf")
	normalPriorityCmd := pipe.ZCount(ctx, q.getQueueKey(), strconv.Itoa(PriorityNormal), strconv.Itoa(PriorityHigh-1))
	lowPriorityCmd := pipe.ZCount(ctx, q.getQueueKey(), "-inf", strconv.Itoa(PriorityNormal-1))
	
	// Execute all commands
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	
	// Collect results
	stats := map[string]interface{}{
		"total_size":    sizeCmd.Val(),
		"high_priority": highPriorityCmd.Val(),
		"normal_priority": normalPriorityCmd.Val(),
		"low_priority":  lowPriorityCmd.Val(),
		"queue_name":    q.queueName,
		"timestamp":     time.Now(),
	}
	
	return stats, nil
} 