package worker

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/yourusername/taskscheduler/internal/model"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/queue"
)

// ProcessTaskFunc is a function type for processing a task
type ProcessTaskFunc func(ctx context.Context, taskID string) error

// Worker polls the task queue and executes tasks
type Worker struct {
	repo           repository.TaskRepository
	queue          queue.TaskQueue
	maxRetries     int
	timeout        time.Duration
	concurrent     int
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	processTaskFunc ProcessTaskFunc  // Function to process a task
}

// NewWorker creates a new worker
func NewWorker(repo repository.TaskRepository, queue queue.TaskQueue, concurrent int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	
	worker := &Worker{
		repo:       repo,
		queue:      queue,
		maxRetries: 3,
		timeout:    30 * time.Second, // Default timeout for queue pop operations
		concurrent: concurrent,
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// Default process task function
	worker.processTaskFunc = worker.processTask
	
	return worker
}

// Start begins the worker processing loop
func (w *Worker) Start() {
	log.Printf("Starting worker with %d concurrent processors\n", w.concurrent)
	
	for i := 0; i < w.concurrent; i++ {
		w.wg.Add(1)
		go w.processLoop(i)
	}
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	log.Println("Stopping worker...")
	w.cancel()
	w.wg.Wait()
	log.Println("Worker stopped")
}

// processLoop continuously processes tasks from the queue
func (w *Worker) processLoop(workerID int) {
	defer w.wg.Done()
	
	log.Printf("Worker %d started\n", workerID)
	
	for {
		select {
		case <-w.ctx.Done():
			log.Printf("Worker %d stopping due to context cancellation\n", workerID)
			return
		default:
			// Try to get a task from the queue
			taskID, err := w.queue.Pop(w.ctx, w.timeout)
			if err != nil {
				if err == queue.ErrQueueEmpty {
					// Queue is empty, wait a bit and try again
					time.Sleep(1 * time.Second)
					continue
				}
				log.Printf("Worker %d error popping from queue: %v\n", workerID, err)
				continue
			}
			
			// Process the task
			if err := w.processTaskFunc(w.ctx, taskID); err != nil {
				log.Printf("Worker %d failed to process task %s: %v\n", workerID, taskID, err)
			}
		}
	}
}

// processTask handles the execution of a single task
func (w *Worker) processTask(ctx context.Context, taskID string) error {
	// Get task details from the repository
	task, err := w.repo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to retrieve task: %w", err)
	}
	
	log.Printf("Processing task %s (%s)\n", task.ID, task.Name)
	
	// Update status to running
	if err := w.repo.UpdateStatus(ctx, task.ID, model.TaskStatusRunning); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	
	// Execute the task with retries
	success := false
	var lastErr error
	
	for attempt := 0; attempt <= task.Retries; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying task %s, attempt %d/%d\n", task.ID, attempt, task.Retries)
			time.Sleep(time.Duration(attempt) * time.Second) // Simple backoff
		}
		
		// Execute the command
		err := w.executeCommand(ctx, task)
		if err == nil {
			success = true
			break
		}
		
		lastErr = err
		log.Printf("Task %s execution failed: %v\n", task.ID, err)
	}
	
	// Update task status based on execution result
	var status string
	if success {
		status = model.TaskStatusCompleted
		log.Printf("Task %s completed successfully\n", task.ID)
	} else {
		status = model.TaskStatusFailed
		log.Printf("Task %s failed after %d retries: %v\n", task.ID, task.Retries, lastErr)
	}
	
	if err := w.repo.UpdateStatus(ctx, task.ID, status); err != nil {
		return fmt.Errorf("failed to update task status after execution: %w", err)
	}
	
	return lastErr
}

// executeCommand runs the task command in a controlled environment
func (w *Worker) executeCommand(ctx context.Context, task *model.Task) error {
	// Create a context with the task timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(task.Timeout)*time.Second)
	defer cancel()
	
	// Create the command
	cmd := exec.CommandContext(execCtx, "sh", "-c", task.Command)
	
	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command execution failed: %w, output: %s", err, string(output))
	}
	
	log.Printf("Task %s command output: %s\n", task.ID, string(output))
	return nil
} 