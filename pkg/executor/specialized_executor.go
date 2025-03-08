package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	Success     bool                   `json:"success"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Duration    time.Duration          `json:"duration"`
	Checkpoint  interface{}            `json:"checkpoint,omitempty"`
	WorkerID    string                 `json:"worker_id"`
	Retryable   bool                   `json:"retryable"`
	RetryReason string                 `json:"retry_reason,omitempty"`
}

// TaskExecutor defines the interface for task execution
type TaskExecutor interface {
	Execute(ctx context.Context, task *Task) (*TaskResult, error)
	CanExecute(task *Task) bool
	GetSpecialization() string
	GetCapabilities() []string
}

// Task represents a task to be executed
type Task struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Command      string                 `json:"command"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Timeout      time.Duration          `json:"timeout"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	Priority     int                    `json:"priority"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Checkpoint   interface{}            `json:"checkpoint,omitempty"`
	BatchKey     string                 `json:"batch_key,omitempty"`
	BatchSize    int                    `json:"batch_size,omitempty"`
	Resources    ResourceRequirements   `json:"resources,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
}

// ResourceRequirements specifies the resources needed by a task
type ResourceRequirements struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
	GPU           int    `json:"gpu,omitempty"`
}

// ExecutorManager manages multiple specialized executors
type ExecutorManager struct {
	executors     map[string]TaskExecutor
	defaultExecutor TaskExecutor
	batchers      map[string]*TaskBatcher
	mu            sync.RWMutex
}

// NewExecutorManager creates a new executor manager
func NewExecutorManager(defaultExecutor TaskExecutor) *ExecutorManager {
	return &ExecutorManager{
		executors:       make(map[string]TaskExecutor),
		defaultExecutor: defaultExecutor,
		batchers:        make(map[string]*TaskBatcher),
	}
}

// RegisterExecutor registers a specialized executor
func (em *ExecutorManager) RegisterExecutor(executor TaskExecutor) {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	specialization := executor.GetSpecialization()
	em.executors[specialization] = executor
}

// GetExecutor finds the most appropriate executor for a task
func (em *ExecutorManager) GetExecutor(task *Task) TaskExecutor {
	em.mu.RLock()
	defer em.mu.RUnlock()
	
	// First, check if we have a specialized executor for this task type
	if executor, ok := em.executors[task.Type]; ok && executor.CanExecute(task) {
		return executor
	}
	
	// If no specialized executor, try to find one that can execute the task
	for _, executor := range em.executors {
		if executor.CanExecute(task) {
			return executor
		}
	}
	
	// If no specialized executor found, use the default one
	return em.defaultExecutor
}

// ExecuteTask executes a task using the appropriate executor
func (em *ExecutorManager) ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error) {
	// Check if the task should be batched
	if task.BatchKey != "" && task.BatchSize > 1 {
		batcher, err := em.getOrCreateBatcher(task.BatchKey, task.BatchSize, task.Type)
		if err != nil {
			return nil, err
		}
		
		return batcher.AddTask(ctx, task)
	}
	
	// Find the appropriate executor
	executor := em.GetExecutor(task)
	
	// Execute the task
	return executor.Execute(ctx, task)
}

// getOrCreateBatcher gets or creates a task batcher
func (em *ExecutorManager) getOrCreateBatcher(batchKey string, batchSize int, taskType string) (*TaskBatcher, error) {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	// Check if batcher already exists
	if batcher, ok := em.batchers[batchKey]; ok {
		return batcher, nil
	}
	
	// Find an executor for this task type
	var executor TaskExecutor
	if specializedExecutor, ok := em.executors[taskType]; ok {
		executor = specializedExecutor
	} else {
		executor = em.defaultExecutor
	}
	
	// Create a new batcher
	batcher := NewTaskBatcher(batchKey, batchSize, executor)
	em.batchers[batchKey] = batcher
	
	return batcher, nil
}

// CleanupBatcher removes a batcher when it's done
func (em *ExecutorManager) CleanupBatcher(batchKey string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	delete(em.batchers, batchKey)
}

// BatchedResult represents the result of a batched task execution
type BatchedResult struct {
	TaskID    string      // Original task ID
	BatchedID string      // ID of the batch this task was part of
	Result    *TaskResult // Actual task result
}

// TaskBatcher batches tasks together for execution
type TaskBatcher struct {
	batchKey       string
	batchSize      int
	executor       TaskExecutor
	tasks          []*Task
	resultChannels map[string]chan *BatchedResult
	timer          *time.Timer
	mu             sync.Mutex
	currentBatchID string
}

// NewTaskBatcher creates a new task batcher
func NewTaskBatcher(batchKey string, batchSize int, executor TaskExecutor) *TaskBatcher {
	return &TaskBatcher{
		batchKey:       batchKey,
		batchSize:      batchSize,
		executor:       executor,
		tasks:          make([]*Task, 0, batchSize),
		resultChannels: make(map[string]chan *BatchedResult),
		currentBatchID: fmt.Sprintf("batch-%s-%d", batchKey, time.Now().UnixNano()),
	}
}

// AddTask adds a task to the batch and processes the batch if full
func (tb *TaskBatcher) AddTask(ctx context.Context, task *Task) (*TaskResult, error) {
	tb.mu.Lock()
	
	// Create a channel for this task's result
	resultChan := make(chan *BatchedResult, 1)
	tb.resultChannels[task.ID] = resultChan
	
	// Add task to batch
	tb.tasks = append(tb.tasks, task)
	
	// Check if batch is full
	readyToExecute := len(tb.tasks) >= tb.batchSize
	
	// If this is the first task, start a timer to execute the batch
	// even if it doesn't fill up
	if len(tb.tasks) == 1 {
		// Cancel any existing timer
		if tb.timer != nil {
			tb.timer.Stop()
		}
		
		// Start a new timer (5 seconds)
		tb.timer = time.AfterFunc(5*time.Second, func() {
			tb.processBatch(ctx)
		})
	}
	
	// Remember the batchID for this task
	batchID := tb.currentBatchID
	
	// Process batch if ready
	if readyToExecute {
		// Stop timer since we're executing now
		if tb.timer != nil {
			tb.timer.Stop()
			tb.timer = nil
		}
		
		// Process the batch
		go tb.processBatch(ctx)
	}
	
	tb.mu.Unlock()
	
	// Wait for result
	select {
	case result := <-resultChan:
		if result.Result == nil {
			return nil, errors.New("task execution failed")
		}
		return result.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processBatch processes all tasks in the current batch
func (tb *TaskBatcher) processBatch(ctx context.Context) {
	tb.mu.Lock()
	
	// Get tasks in the current batch
	tasks := tb.tasks
	batchID := tb.currentBatchID
	channels := make(map[string]chan *BatchedResult)
	
	// Copy channels to avoid race conditions
	for taskID, ch := range tb.resultChannels {
		channels[taskID] = ch
	}
	
	// Reset for next batch
	tb.tasks = make([]*Task, 0, tb.batchSize)
	tb.resultChannels = make(map[string]chan *BatchedResult)
	tb.currentBatchID = fmt.Sprintf("batch-%s-%d", tb.batchKey, time.Now().UnixNano())
	tb.timer = nil
	
	tb.mu.Unlock()
	
	// Skip if no tasks
	if len(tasks) == 0 {
		return
	}
	
	// Create a batch task
	batchTask := &Task{
		ID:         batchID,
		Type:       tasks[0].Type, // Use the type of the first task
		Parameters: map[string]interface{}{
			"batch":      true,
			"batch_key":  tb.batchKey,
			"batch_size": len(tasks),
			"tasks":      tasks,
		},
	}
	
	// Execute the batch task
	result, err := tb.executor.Execute(ctx, batchTask)
	
	if err != nil {
		// If batch execution fails, notify all tasks with the error
		for _, task := range tasks {
			if ch, ok := channels[task.ID]; ok {
				ch <- &BatchedResult{
					TaskID:    task.ID,
					BatchedID: batchID,
					Result: &TaskResult{
						TaskID:      task.ID,
						Success:     false,
						Error:       err.Error(),
						StartTime:   time.Now(),
						EndTime:     time.Now(),
						Duration:    0,
						WorkerID:    "batcher",
						Retryable:   true,
						RetryReason: "batch execution failed",
					},
				}
				close(ch)
			}
		}
		return
	}
	
	// Parse individual results
	var batchResults map[string]*TaskResult
	if result.Data != nil {
		if resultsData, ok := result.Data["results"]; ok {
			if resultsJSON, err := json.Marshal(resultsData); err == nil {
				if err := json.Unmarshal(resultsJSON, &batchResults); err != nil {
					batchResults = nil
				}
			}
		}
	}
	
	// Send results to individual tasks
	for _, task := range tasks {
		var taskResult *TaskResult
		
		// If we have a specific result for this task, use it
		if batchResults != nil {
			if specificResult, ok := batchResults[task.ID]; ok {
				taskResult = specificResult
			}
		}
		
		// If no specific result, create a generic one based on batch result
		if taskResult == nil {
			taskResult = &TaskResult{
				TaskID:    task.ID,
				Success:   result.Success,
				StartTime: result.StartTime,
				EndTime:   result.EndTime,
				Duration:  result.Duration,
				WorkerID:  result.WorkerID,
				Retryable: result.Retryable,
			}
			
			if !result.Success {
				taskResult.Error = fmt.Sprintf("batch execution failed: %s", result.Error)
			}
		}
		
		// Send the result to the waiting task
		if ch, ok := channels[task.ID]; ok {
			ch <- &BatchedResult{
				TaskID:    task.ID,
				BatchedID: batchID,
				Result:    taskResult,
			}
			close(ch)
		}
	}
}

// BaseExecutor provides common functionality for executors
type BaseExecutor struct {
	specialization string
	capabilities   []string
	executeFunc    func(ctx context.Context, task *Task) (*TaskResult, error)
	canExecuteFunc func(task *Task) bool
}

// NewBaseExecutor creates a new base executor
func NewBaseExecutor(specialization string, capabilities []string, 
                     executeFunc func(ctx context.Context, task *Task) (*TaskResult, error),
                     canExecuteFunc func(task *Task) bool) *BaseExecutor {
	return &BaseExecutor{
		specialization: specialization,
		capabilities:   capabilities,
		executeFunc:    executeFunc,
		canExecuteFunc: canExecuteFunc,
	}
}

// Execute executes a task
func (be *BaseExecutor) Execute(ctx context.Context, task *Task) (*TaskResult, error) {
	if be.executeFunc == nil {
		return nil, errors.New("execute function not implemented")
	}
	return be.executeFunc(ctx, task)
}

// CanExecute checks if the executor can execute a task
func (be *BaseExecutor) CanExecute(task *Task) bool {
	if be.canExecuteFunc != nil {
		return be.canExecuteFunc(task)
	}
	return task.Type == be.specialization
}

// GetSpecialization returns the specialization of the executor
func (be *BaseExecutor) GetSpecialization() string {
	return be.specialization
}

// GetCapabilities returns the capabilities of the executor
func (be *BaseExecutor) GetCapabilities() []string {
	return be.capabilities
}

// DefaultExecutor is a simple executor that executes all tasks
type DefaultExecutor struct {
	*BaseExecutor
}

// NewDefaultExecutor creates a new default executor
func NewDefaultExecutor() *DefaultExecutor {
	return &DefaultExecutor{
		BaseExecutor: &BaseExecutor{
			specialization: "default",
			capabilities:   []string{"basic"},
			executeFunc: func(ctx context.Context, task *Task) (*TaskResult, error) {
				// Simple implementation that just returns success
				startTime := time.Now()
				time.Sleep(100 * time.Millisecond) // Simulate work
				endTime := time.Now()
				
				return &TaskResult{
					TaskID:    task.ID,
					Success:   true,
					Data:      map[string]interface{}{"status": "completed"},
					StartTime: startTime,
					EndTime:   endTime,
					Duration:  endTime.Sub(startTime),
					WorkerID:  "default-executor",
				}, nil
			},
			canExecuteFunc: func(task *Task) bool {
				return true // Default executor can execute all tasks
			},
		},
	}
}

// WithTimeout adds a timeout to task execution
func WithTimeout(executor TaskExecutor) TaskExecutor {
	return &BaseExecutor{
		specialization: executor.GetSpecialization(),
		capabilities:   executor.GetCapabilities(),
		executeFunc: func(ctx context.Context, task *Task) (*TaskResult, error) {
			timeoutDuration := task.Timeout
			if timeoutDuration == 0 {
				timeoutDuration = 30 * time.Second // Default timeout
			}
			
			ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
			defer cancel()
			
			resultChan := make(chan *TaskResult, 1)
			errChan := make(chan error, 1)
			
			go func() {
				result, err := executor.Execute(ctx, task)
				if err != nil {
					errChan <- err
					return
				}
				resultChan <- result
			}()
			
			select {
			case result := <-resultChan:
				return result, nil
			case err := <-errChan:
				return nil, err
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return &TaskResult{
						TaskID:      task.ID,
						Success:     false,
						Error:       "task execution timed out",
						StartTime:   time.Now(),
						EndTime:     time.Now(),
						Duration:    timeoutDuration,
						WorkerID:    "timeout-wrapper",
						Retryable:   true,
						RetryReason: "timeout",
					}, nil
				}
				return nil, ctx.Err()
			}
		},
		canExecuteFunc: executor.CanExecute,
	}
}

// WithRetry adds retry logic to task execution
func WithRetry(executor TaskExecutor) TaskExecutor {
	return &BaseExecutor{
		specialization: executor.GetSpecialization(),
		capabilities:   executor.GetCapabilities(),
		executeFunc: func(ctx context.Context, task *Task) (*TaskResult, error) {
			var lastErr error
			var result *TaskResult
			
			for attempt := 0; attempt <= task.MaxRetries; attempt++ {
				// Only apply retry delay after the first attempt
				if attempt > 0 {
					// Calculate exponential backoff delay
					delay := time.Duration(1<<uint(attempt-1)) * time.Second
					if delay > 60*time.Second {
						delay = 60 * time.Second // Cap at 1 minute
					}
					
					// Wait for the backoff delay
					select {
					case <-time.After(delay):
						// Continue with retry
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				
				// Execute the task
				result, lastErr = executor.Execute(ctx, task)
				
				// If success or non-retryable error, return immediately
				if lastErr == nil && result.Success {
					return result, nil
				}
				
				if result != nil && !result.Retryable {
					return result, nil
				}
			}
			
			// If we get here, we've exhausted all retries
			if result != nil {
				return result, nil
			}
			
			return nil, lastErr
		},
		canExecuteFunc: executor.CanExecute,
	}
}

// WithLogging adds logging to task execution
func WithLogging(executor TaskExecutor, logFn func(format string, args ...interface{})) TaskExecutor {
	return &BaseExecutor{
		specialization: executor.GetSpecialization(),
		capabilities:   executor.GetCapabilities(),
		executeFunc: func(ctx context.Context, task *Task) (*TaskResult, error) {
			logFn("Starting execution of task %s (type: %s)", task.ID, task.Type)
			startTime := time.Now()
			
			result, err := executor.Execute(ctx, task)
			
			duration := time.Since(startTime)
			if err != nil {
				logFn("Task %s failed after %v: %v", task.ID, duration, err)
				return nil, err
			}
			
			if result.Success {
				logFn("Task %s completed successfully in %v", task.ID, duration)
			} else {
				logFn("Task %s failed in %v: %s", task.ID, duration, result.Error)
			}
			
			return result, nil
		},
		canExecuteFunc: executor.CanExecute,
	}
}

// WithMetrics adds metrics collection to task execution
func WithMetrics(executor TaskExecutor, recordMetric func(name string, value float64, tags map[string]string)) TaskExecutor {
	return &BaseExecutor{
		specialization: executor.GetSpecialization(),
		capabilities:   executor.GetCapabilities(),
		executeFunc: func(ctx context.Context, task *Task) (*TaskResult, error) {
			startTime := time.Now()
			
			result, err := executor.Execute(ctx, task)
			
			duration := time.Since(startTime)
			
			// Record execution time
			recordMetric("task.execution_time", duration.Seconds(), map[string]string{
				"task_type": task.Type,
				"success":   fmt.Sprintf("%t", err == nil && (result == nil || result.Success)),
				"executor":  executor.GetSpecialization(),
			})
			
			// Record task result
			if err == nil && result != nil {
				status := "success"
				if !result.Success {
					status = "failure"
				}
				
				recordMetric("task.result", 1, map[string]string{
					"task_type": task.Type,
					"status":    status,
					"executor":  executor.GetSpecialization(),
				})
			}
			
			return result, err
		},
		canExecuteFunc: executor.CanExecute,
	}
} 