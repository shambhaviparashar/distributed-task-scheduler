package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/yourusername/taskscheduler/internal/model"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/coordination"
	"github.com/yourusername/taskscheduler/pkg/metrics"
	"github.com/yourusername/taskscheduler/pkg/queue"
)

// TaskRecoveryService handles recovery of tasks from failed workers
type TaskRecoveryService struct {
	repo              repository.TaskRepository
	queue             queue.TaskQueue
	workerRegistry    *coordination.WorkerRegistry
	metrics           *metrics.Metrics
	staleTaskInterval time.Duration
	checkInterval     time.Duration
	isRunning         bool
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewTaskRecoveryService creates a new task recovery service
func NewTaskRecoveryService(
	repo repository.TaskRepository,
	queue queue.TaskQueue,
	workerRegistry *coordination.WorkerRegistry,
	staleTaskInterval, checkInterval time.Duration,
) *TaskRecoveryService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TaskRecoveryService{
		repo:              repo,
		queue:             queue,
		workerRegistry:    workerRegistry,
		metrics:           metrics.GetInstance(),
		staleTaskInterval: staleTaskInterval,
		checkInterval:     checkInterval,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start begins the task recovery process
func (s *TaskRecoveryService) Start() {
	if s.isRunning {
		return
	}
	s.isRunning = true
	
	// Register worker failure handler
	s.workerRegistry.OnWorkerFailure(s.handleWorkerFailure)
	
	// Start a goroutine to periodically check for stale tasks
	go s.checkStaleTasks()
	
	log.Println("Task recovery service started")
}

// Stop terminates the task recovery process
func (s *TaskRecoveryService) Stop() {
	if !s.isRunning {
		return
	}
	
	s.cancel()
	s.isRunning = false
	log.Println("Task recovery service stopped")
}

// handleWorkerFailure is called when a worker is detected as failed
func (s *TaskRecoveryService) handleWorkerFailure(workerID string) {
	log.Printf("Handling worker failure for worker %s", workerID)
	
	// Get all tasks assigned to the failed worker
	tasks, err := s.repo.GetTasksByWorker(s.ctx, workerID)
	if err != nil {
		log.Printf("Error retrieving tasks for failed worker %s: %v", workerID, err)
		return
	}
	
	log.Printf("Found %d tasks assigned to failed worker %s", len(tasks), workerID)
	
	// Process each task
	for _, task := range tasks {
		s.recoverTask(task, "worker_failure")
	}
}

// checkStaleTasks periodically checks for tasks that have not been updated recently
func (s *TaskRecoveryService) checkStaleTasks() {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processStaleTasks()
		}
	}
}

// processStaleTasks finds and processes stale tasks
func (s *TaskRecoveryService) processStaleTasks() {
	tasks, err := s.repo.GetStaleTasks(s.ctx, s.staleTaskInterval)
	if err != nil {
		log.Printf("Error retrieving stale tasks: %v", err)
		return
	}
	
	if len(tasks) > 0 {
		log.Printf("Found %d stale tasks", len(tasks))
		for _, task := range tasks {
			s.recoverTask(task, "stale_task")
		}
	}
}

// recoverTask attempts to recover a failed task
func (s *TaskRecoveryService) recoverTask(task *model.Task, reason string) {
	log.Printf("Recovering task %s (%s) due to %s", task.ID, task.Name, reason)
	
	// Check if task should be retried
	if task.ShouldRetry() {
		// Increment retry count
		retryCount, err := s.repo.IncrementRetryCount(s.ctx, task.ID)
		if err != nil {
			log.Printf("Error incrementing retry count for task %s: %v", task.ID, err)
			return
		}
		
		// Update task status
		err = s.repo.UpdateStatus(s.ctx, task.ID, model.TaskStatusRetrying)
		if err != nil {
			log.Printf("Error updating task status for %s: %v", task.ID, err)
			return
		}
		
		// Calculate backoff duration
		task.RetryCount = retryCount
		backoff := task.CalculateBackoff()
		
		log.Printf("Scheduling task %s for retry %d/%d after %v", task.ID, retryCount, task.Retries, backoff)
		
		// Wait for backoff period and then requeue
		go func(taskID string, delay time.Duration) {
			time.Sleep(delay)
			
			// Release task from worker
			if task.WorkerID != "" {
				err = s.repo.ReleaseTask(s.ctx, taskID, task.WorkerID)
				if err != nil {
					log.Printf("Error releasing task %s from worker: %v", taskID, err)
				}
			}
			
			// Update task status to pending
			err = s.repo.UpdateStatus(s.ctx, taskID, model.TaskStatusPending)
			if err != nil {
				log.Printf("Error updating task status for %s: %v", taskID, err)
				return
			}
			
			// Push back to queue with priority
			err = s.queue.Push(s.ctx, taskID)
			if err != nil {
				log.Printf("Error requeueing task %s: %v", taskID, err)
				return
			}
			
			log.Printf("Task %s successfully requeued after failure", taskID)
			s.metrics.TasksRetried.Inc()
		}(task.ID, backoff)
	} else {
		// Move to dead letter queue if retries are exhausted
		log.Printf("Task %s has exhausted retries (%d/%d), moving to dead letter queue", 
			task.ID, task.RetryCount, task.Retries)
		
		// Update task status to dead letter
		err := s.repo.UpdateStatus(s.ctx, task.ID, model.TaskStatusDeadLetter)
		if err != nil {
			log.Printf("Error updating task status for %s: %v", task.ID, err)
			return
		}
		
		// Release task from worker
		if task.WorkerID != "" {
			err = s.repo.ReleaseTask(s.ctx, task.ID, task.WorkerID)
			if err != nil {
				log.Printf("Error releasing task %s from worker: %v", task.ID, err)
			}
		}
		
		s.metrics.TasksInDeadLetter.Inc()
	}
}

// RecoverStuckTasks attempts to recover tasks stuck in an inconsistent state
func (s *TaskRecoveryService) RecoverStuckTasks() error {
	// This is a maintenance operation, typically called on system startup
	
	// Get tasks stuck in running state
	runningTasks, err := s.repo.GetTasksByStatus(s.ctx, model.TaskStatusRunning, 1000)
	if err != nil {
		return err
	}
	
	log.Printf("Found %d tasks stuck in running state during system startup", len(runningTasks))
	
	// Attempt to recover each task
	for _, task := range runningTasks {
		// Check if the worker is still active
		activeWorkers := s.workerRegistry.GetActiveWorkers()
		workerActive := false
		
		for _, worker := range activeWorkers {
			if worker.ID == task.WorkerID {
				workerActive = true
				break
			}
		}
		
		if !workerActive {
			// Worker is not active, recover the task
			s.recoverTask(task, "system_startup")
		} else {
			// Worker is active, let the stale task check handle it if needed
			log.Printf("Task %s assigned to worker %s which is still active, skipping recovery",
				task.ID, task.WorkerID)
		}
	}
	
	return nil
} 