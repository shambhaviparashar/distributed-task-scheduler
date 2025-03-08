package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/yourusername/taskscheduler/internal/model"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/coordination"
	"github.com/yourusername/taskscheduler/pkg/metrics"
	"github.com/yourusername/taskscheduler/pkg/queue"
)

// FaultTolerantWorker extends the basic worker with fault tolerance
type FaultTolerantWorker struct {
	*Worker                            // Embed the basic worker
	instanceID          string         // Unique identifier for this worker
	redisClient         *redis.Client
	workerRegistry      *coordination.WorkerRegistry
	metrics             *metrics.Metrics
	taskHeartbeats      map[string]*time.Ticker
	taskHeartbeatsMutex sync.Mutex
	maxTasksPerWorker   int // Maximum number of tasks this worker can process concurrently
	shutdownGracePeriod time.Duration
}

// FaultTolerantWorkerConfig holds configuration for the fault tolerant worker
type FaultTolerantWorkerConfig struct {
	Repo              repository.TaskRepository
	Queue             queue.TaskQueue
	RedisClient       *redis.Client
	InstanceID        string
	Capacity          int
	ShutdownGracePeriod time.Duration
}

// NewFaultTolerantWorker creates a new fault-tolerant worker
func NewFaultTolerantWorker(config FaultTolerantWorkerConfig) *FaultTolerantWorker {
	// Create the basic worker
	// Use config.Capacity instead of hard-coded concurrent value
	concurrency := config.Capacity
	if concurrency <= 0 {
		concurrency = 5 // Default to 5 concurrent tasks
	}
	
	basicWorker := NewWorker(config.Repo, config.Queue, concurrency)
	
	// Get metrics singleton
	metrics := metrics.GetInstance()
	
	// Create worker registry config
	registryConfig := coordination.WorkerRegistryConfig{
		Client:     config.RedisClient,
		InstanceID: config.InstanceID,
		Capacity:   concurrency,
		Tags:       []string{"worker"},
	}
	
	// Create worker registry
	workerRegistry := coordination.NewWorkerRegistry(registryConfig)
	
	gracePeriod := config.ShutdownGracePeriod
	if gracePeriod == 0 {
		gracePeriod = 30 * time.Second // Default grace period of 30 seconds
	}
	
	return &FaultTolerantWorker{
		Worker:              basicWorker,
		instanceID:          config.InstanceID,
		redisClient:         config.RedisClient,
		workerRegistry:      workerRegistry,
		metrics:             metrics,
		taskHeartbeats:      make(map[string]*time.Ticker),
		maxTasksPerWorker:   concurrency,
		shutdownGracePeriod: gracePeriod,
	}
}

// Start begins the fault-tolerant worker processing
func (w *FaultTolerantWorker) Start() {
	log.Printf("Starting fault-tolerant worker with instance ID: %s", w.instanceID)
	
	// Start the worker registry
	err := w.workerRegistry.Start()
	if err != nil {
		log.Printf("Failed to start worker registry: %v", err)
		return
	}
	
	// Override the Worker's processTask method to add fault tolerance
	w.processTaskFunc = w.processTaskWithFaultTolerance
	
	// Start the basic worker
	w.Worker.Start()
	
	// Start utilization reporting
	go w.reportUtilization()
	
	log.Printf("Fault-tolerant worker started successfully with capacity: %d", w.maxTasksPerWorker)
}

// Stop gracefully terminates the fault-tolerant worker
func (w *FaultTolerantWorker) Stop() {
	log.Println("Stopping fault-tolerant worker...")
	
	// Set shutting down flag to prevent accepting new tasks
	// Wait for grace period to allow tasks to complete
	ctx, cancel := context.WithTimeout(context.Background(), w.shutdownGracePeriod)
	defer cancel()
	
	// Wait until all tasks are completed or timeout
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown grace period expired, forcing worker stop")
			goto forceStop
		case <-ticker.C:
			// Get current task count
			runningTaskCount := w.getRunningTaskCount()
			if runningTaskCount == 0 {
				log.Println("All tasks completed, proceeding with worker shutdown")
				goto forceStop
			}
			log.Printf("Waiting for %d tasks to complete before shutdown...", runningTaskCount)
		}
	}
	
forceStop:
	// Stop all task heartbeats
	w.stopAllHeartbeats()
	
	// Update worker status in registry
	w.workerRegistry.Stop()
	
	// Stop the basic worker
	w.Worker.Stop()
	
	log.Println("Fault-tolerant worker stopped")
}

// getRunningTaskCount returns the number of currently running tasks
func (w *FaultTolerantWorker) getRunningTaskCount() int {
	w.taskHeartbeatsMutex.Lock()
	defer w.taskHeartbeatsMutex.Unlock()
	return len(w.taskHeartbeats)
}

// processTaskWithFaultTolerance handles the execution of a single task with fault tolerance
func (w *FaultTolerantWorker) processTaskWithFaultTolerance(ctx context.Context, taskID string) error {
	// Get task details from the repository
	task, err := w.repo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to retrieve task: %w", err)
	}
	
	log.Printf("Processing task %s (%s) with fault tolerance", task.ID, task.Name)
	
	// Record metrics
	w.metrics.RecordWorkerTask(w.instanceID, "start")
	startTime := time.Now()
	
	// Assign the task to this worker
	err = w.repo.UpdateTaskWithWorker(ctx, task.ID, w.instanceID, model.TaskStatusRunning)
	if err != nil {
		w.metrics.RecordWorkerTask(w.instanceID, "failed_assign")
		return fmt.Errorf("failed to assign task to worker: %w", err)
	}
	
	// Start heartbeat for this task
	w.startTaskHeartbeat(ctx, task.ID)
	
	// Execute the task with retries
	success := false
	var lastErr error
	var result string
	var exitCode int
	
	// Mark time when task is actually queued
	task.QueuedAt = startTime
	task.StartedAt = time.Now()
	
	for attempt := 0; attempt <= task.Retries; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying task %s, attempt %d/%d", task.ID, attempt, task.Retries)
			
			// Update retry count
			w.repo.IncrementRetryCount(ctx, task.ID)
			
			// Calculate backoff
			task.RetryCount = attempt
			backoff := task.CalculateBackoff()
			log.Printf("Backing off for %v before retry", backoff)
			time.Sleep(backoff)
		}
		
		// Execute the command with checkpoint support
		var checkpoint map[string]interface{}
		exitCode, result, checkpoint, err = w.executeCommandWithCheckpoint(ctx, task)
		
		// If we have checkpoint data, save it
		if checkpoint != nil && len(checkpoint) > 0 {
			saveErr := w.repo.SaveCheckpoint(ctx, task.ID, checkpoint)
			if saveErr != nil {
				log.Printf("Error saving checkpoint for task %s: %v", task.ID, saveErr)
			}
		}
		
		if err == nil && exitCode == 0 {
			success = true
			break
		}
		
		lastErr = err
		log.Printf("Task %s execution failed with exit code %d: %v", task.ID, exitCode, err)
	}
	
	// Stop the heartbeat
	w.stopTaskHeartbeat(task.ID)
	
	// Update task status based on execution result
	task.FinishedAt = time.Now()
	task.ExitCode = exitCode
	task.Result = result
	
	var status string
	if success {
		status = model.TaskStatusCompleted
		log.Printf("Task %s completed successfully", task.ID)
		w.metrics.RecordWorkerTask(w.instanceID, "completed")
	} else {
		if task.RetryCount >= task.Retries {
			status = model.TaskStatusDeadLetter
			log.Printf("Task %s failed after %d retries, moved to dead letter queue", task.ID, task.RetryCount)
			w.metrics.RecordWorkerTask(w.instanceID, "dead_letter")
		} else {
			status = model.TaskStatusFailed
			log.Printf("Task %s failed with error: %v", task.ID, lastErr)
			w.metrics.RecordWorkerTask(w.instanceID, "failed")
		}
		
		if lastErr != nil {
			task.Error = lastErr.Error()
		} else {
			task.Error = fmt.Sprintf("Command exited with code %d", exitCode)
		}
	}
	
	// Update task in the repository
	err = w.repo.Update(ctx, task)
	if err != nil {
		log.Printf("Failed to update task after execution: %v", err)
	}
	
	// Record execution time
	duration := time.Since(startTime)
	w.metrics.RecordTaskExecution(task.Name, status, duration)
	
	// Calculate queue latency (time from queueing to starting)
	if !task.QueuedAt.IsZero() && !task.StartedAt.IsZero() {
		queueLatency := task.StartedAt.Sub(task.QueuedAt)
		w.metrics.RecordQueueLatency(queueLatency)
	}
	
	return lastErr
}

// startTaskHeartbeat begins sending heartbeats for a running task
func (w *FaultTolerantWorker) startTaskHeartbeat(ctx context.Context, taskID string) {
	w.taskHeartbeatsMutex.Lock()
	defer w.taskHeartbeatsMutex.Unlock()
	
	// Create a ticker that will send heartbeats every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	w.taskHeartbeats[taskID] = ticker
	
	// Start a goroutine to send heartbeats
	go func(tid string, t *time.Ticker) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// Update the last heartbeat time for the task
				err := w.repo.UpdateTaskWithWorker(ctx, tid, w.instanceID, model.TaskStatusRunning)
				if err != nil {
					log.Printf("Failed to send heartbeat for task %s: %v", tid, err)
				}
			}
		}
	}(taskID, ticker)
}

// stopTaskHeartbeat stops sending heartbeats for a task
func (w *FaultTolerantWorker) stopTaskHeartbeat(taskID string) {
	w.taskHeartbeatsMutex.Lock()
	defer w.taskHeartbeatsMutex.Unlock()
	
	ticker, exists := w.taskHeartbeats[taskID]
	if exists {
		ticker.Stop()
		delete(w.taskHeartbeats, taskID)
	}
}

// stopAllHeartbeats stops all task heartbeats
func (w *FaultTolerantWorker) stopAllHeartbeats() {
	w.taskHeartbeatsMutex.Lock()
	defer w.taskHeartbeatsMutex.Unlock()
	
	for taskID, ticker := range w.taskHeartbeats {
		ticker.Stop()
		delete(w.taskHeartbeats, taskID)
	}
}

// reportUtilization periodically reports worker utilization
func (w *FaultTolerantWorker) reportUtilization() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.updateUtilizationMetrics()
		}
	}
}

// updateUtilizationMetrics updates worker utilization metrics
func (w *FaultTolerantWorker) updateUtilizationMetrics() {
	// Calculate utilization as the percentage of capacity in use
	taskCount := w.getRunningTaskCount()
	utilization := float64(taskCount) / float64(w.maxTasksPerWorker) * 100.0
	
	// Update worker registry
	w.workerRegistry.UpdateUtilization(int(utilization))
	
	// Update Prometheus metrics
	w.metrics.UpdateWorkerUtilization(w.instanceID, utilization)
}

// executeCommandWithCheckpoint runs the task command with checkpoint support
func (w *FaultTolerantWorker) executeCommandWithCheckpoint(
	ctx context.Context, 
	task *model.Task,
) (int, string, map[string]interface{}, error) {
	// Create a context with the task timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(task.Timeout)*time.Second)
	defer cancel()
	
	// Check if we have a checkpoint to resume from
	var cmdWrapper string
	if task.Checkpoint != nil && len(task.Checkpoint) > 0 {
		// Serialize checkpoint data to JSON
		checkpointData, err := json.Marshal(task.Checkpoint)
		if err != nil {
			log.Printf("Failed to serialize checkpoint data: %v", err)
			// Continue with original command
			cmdWrapper = task.Command
		} else {
			// Wrap the command with checkpoint data as an environment variable
			checkpointJSON := string(checkpointData)
			cmdWrapper = fmt.Sprintf("CHECKPOINT_DATA='%s' %s", checkpointJSON, task.Command)
			log.Printf("Resuming task %s with checkpoint data", task.ID)
		}
	} else {
		// No checkpoint, use original command
		cmdWrapper = task.Command
	}
	
	// Create the command
	cmd := createCommand(execCtx, cmdWrapper)
	
	// Capture output
	output, err := cmd.CombinedOutput()
	
	// Check for checkpoint data in the output
	var checkpoint map[string]interface{}
	exitCode := 0
	
	if err != nil {
		// Try to get the exit code
		exitCode = getExitCode(err)
	}
	
	// Check if the output contains checkpoint data
	// This is a simplified implementation; in a real system you would have a structured
	// way to extract checkpoint data from the command output
	checkpoint = extractCheckpoint(string(output))
	
	return exitCode, string(output), checkpoint, err
}

// extractCheckpoint attempts to extract checkpoint data from command output
// This is a simplified implementation for demonstration purposes
func extractCheckpoint(output string) map[string]interface{} {
	// In a real implementation, we would look for a specific format in the output
	// For example, a JSON block between special markers like "CHECKPOINT_START" and "CHECKPOINT_END"
	// For this example, we'll assume there's no checkpoint data
	return nil
} 