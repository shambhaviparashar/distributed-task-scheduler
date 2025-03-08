package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yourusername/taskscheduler/internal/model"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/queue"
)

// Scheduler is responsible for scheduling tasks and pushing them to the queue
type Scheduler struct {
	repo      repository.TaskRepository
	queue     queue.TaskQueue
	cron      *cron.Cron
	pollDelay time.Duration
}

// NewScheduler creates a new scheduler
func NewScheduler(repo repository.TaskRepository, queue queue.TaskQueue) *Scheduler {
	return &Scheduler{
		repo:      repo,
		queue:     queue,
		cron:      cron.New(cron.WithSeconds()),
		pollDelay: 10 * time.Second, // Default poll delay
	}
}

// Start initializes and starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	// Start the cron scheduler
	s.cron.Start()

	// Start the polling loop in a separate goroutine
	go s.pollTasks(ctx)

	log.Println("Scheduler started successfully")
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done() // Wait for cron jobs to complete
	log.Println("Scheduler stopped")
}

// pollTasks periodically polls the database for due tasks
func (s *Scheduler) pollTasks(ctx context.Context) {
	ticker := time.NewTicker(s.pollDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Task polling stopped due to context cancellation")
			return
		case <-ticker.C:
			if err := s.processDueTasks(ctx); err != nil {
				log.Printf("Error processing due tasks: %v\n", err)
			}
		}
	}
}

// processDueTasks finds due tasks and queues them for execution
func (s *Scheduler) processDueTasks(ctx context.Context) error {
	// Get tasks that are due and have no unresolved dependencies
	tasks, err := s.repo.GetDueTasksWithoutDependencies(ctx)
	if err != nil {
		return err
	}

	log.Printf("Found %d tasks ready for execution\n", len(tasks))

	// Push each task to the queue
	for _, task := range tasks {
		if err := s.queue.Push(ctx, task.ID); err != nil {
			log.Printf("Failed to queue task %s: %v\n", task.ID, err)
			continue
		}

		// Update task status to scheduled
		if err := s.repo.UpdateStatus(ctx, task.ID, model.TaskStatusScheduled); err != nil {
			log.Printf("Failed to update task status: %v\n", err)
			continue
		}

		log.Printf("Task %s (%s) scheduled for execution\n", task.ID, task.Name)
	}

	return nil
}

// AddCronTask adds a task to the cron scheduler
// This is a more advanced feature to directly schedule tasks with cron expressions
func (s *Scheduler) AddCronTask(ctx context.Context, taskID string, cronExpr string) (cron.EntryID, error) {
	id, err := s.cron.AddFunc(cronExpr, func() {
		// This runs in the cron goroutine when the schedule hits
		task, err := s.repo.GetByID(ctx, taskID)
		if err != nil {
			log.Printf("Failed to get task %s: %v\n", taskID, err)
			return
		}

		// Check if the task can be executed (no pending dependencies)
		// In a real implementation, this would be more complex
		if len(task.Dependencies) > 0 {
			log.Printf("Task %s has dependencies, skipping\n", taskID)
			return
		}

		if err := s.queue.Push(ctx, taskID); err != nil {
			log.Printf("Failed to queue task %s: %v\n", taskID, err)
			return
		}

		if err := s.repo.UpdateStatus(ctx, taskID, model.TaskStatusScheduled); err != nil {
			log.Printf("Failed to update task status: %v\n", err)
			return
		}

		log.Printf("Cron task %s (%s) scheduled for execution\n", taskID, task.Name)
	})

	return id, err
} 