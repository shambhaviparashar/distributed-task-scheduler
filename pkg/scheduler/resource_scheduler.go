package scheduler

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ResourceType represents a type of resource (CPU, memory, etc.)
type ResourceType string

const (
	// ResourceCPU represents CPU resources
	ResourceCPU ResourceType = "cpu"
	// ResourceMemory represents memory resources
	ResourceMemory ResourceType = "memory"
	// ResourceDisk represents disk space
	ResourceDisk ResourceType = "disk"
	// ResourceGPU represents GPU resources
	ResourceGPU ResourceType = "gpu"
)

// ResourceQuantity represents a quantity of a resource
type ResourceQuantity struct {
	Type     ResourceType
	Value    float64
	Unit     string
	Preemptible bool
}

// Resources represents a collection of resources
type Resources struct {
	CPU    float64
	Memory float64
	Disk   float64
	GPU    int
}

// ResourceRequirements defines the resources required by a task
type ResourceRequirements struct {
	Requests Resources
	Limits   Resources
	Preemptible bool
}

// WorkerNode represents a worker node in the system
type WorkerNode struct {
	ID             string
	Name           string
	Address        string
	Status         string
	Tags           []string
	TotalResources Resources
	UsedResources  Resources
	RunningTasks   map[string]struct{}
	LastHeartbeat  time.Time
	mu             sync.RWMutex
}

// Task represents a task to be scheduled
type Task struct {
	ID               string
	Name             string
	Status           string
	Priority         int
	ResourceRequirements ResourceRequirements
	Dependencies     []string
	CreateTime       time.Time
	StartTime        time.Time
	EndTime          time.Time
	AssignedWorker   string
	Result           string
	Error            string
	RetryCount       int
	MaxRetries       int
}

// Clone creates a deep copy of the task
func (t *Task) Clone() *Task {
	return &Task{
		ID:               t.ID,
		Name:             t.Name,
		Status:           t.Status,
		Priority:         t.Priority,
		ResourceRequirements: t.ResourceRequirements,
		Dependencies:     append([]string{}, t.Dependencies...),
		CreateTime:       t.CreateTime,
		StartTime:        t.StartTime,
		EndTime:          t.EndTime,
		AssignedWorker:   t.AssignedWorker,
		Result:           t.Result,
		Error:            t.Error,
		RetryCount:       t.RetryCount,
		MaxRetries:       t.MaxRetries,
	}
}

// TaskQueue is a priority queue of tasks
type TaskQueue []*Task

// Len returns the length of the queue
func (tq TaskQueue) Len() int { return len(tq) }

// Less compares tasks by priority
func (tq TaskQueue) Less(i, j int) bool {
	// Higher priority value means higher priority
	if tq[i].Priority != tq[j].Priority {
		return tq[i].Priority > tq[j].Priority
	}
	// If priority is the same, schedule older tasks first
	return tq[i].CreateTime.Before(tq[j].CreateTime)
}

// Swap swaps two tasks in the queue
func (tq TaskQueue) Swap(i, j int) { tq[i], tq[j] = tq[j], tq[i] }

// Push adds a task to the queue
func (tq *TaskQueue) Push(x interface{}) {
	*tq = append(*tq, x.(*Task))
}

// Pop removes and returns the highest priority task
func (tq *TaskQueue) Pop() interface{} {
	old := *tq
	n := len(old)
	task := old[n-1]
	*tq = old[0 : n-1]
	return task
}

// ResourceScheduler is a scheduler that considers resource constraints
type ResourceScheduler struct {
	workers         map[string]*WorkerNode
	pendingTasks    TaskQueue
	runningTasks    map[string]*Task
	taskDependencies map[string]map[string]struct{} // Map of task ID to set of task IDs that depend on it
	lock            sync.RWMutex
}

// NewResourceScheduler creates a new resource scheduler
func NewResourceScheduler() *ResourceScheduler {
	scheduler := &ResourceScheduler{
		workers:         make(map[string]*WorkerNode),
		pendingTasks:    make(TaskQueue, 0),
		runningTasks:    make(map[string]*Task),
		taskDependencies: make(map[string]map[string]struct{}),
	}
	heap.Init(&scheduler.pendingTasks)
	return scheduler
}

// RegisterWorker registers a worker with the scheduler
func (rs *ResourceScheduler) RegisterWorker(worker *WorkerNode) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	if worker.ID == "" {
		return errors.New("worker ID cannot be empty")
	}

	if _, exists := rs.workers[worker.ID]; exists {
		return fmt.Errorf("worker with ID %s already exists", worker.ID)
	}

	worker.RunningTasks = make(map[string]struct{})
	worker.LastHeartbeat = time.Now()
	rs.workers[worker.ID] = worker
	return nil
}

// UpdateWorker updates a worker's status and resources
func (rs *ResourceScheduler) UpdateWorker(workerID string, status string, usedResources Resources) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	worker, exists := rs.workers[workerID]
	if !exists {
		return fmt.Errorf("worker with ID %s does not exist", workerID)
	}

	worker.mu.Lock()
	defer worker.mu.Unlock()

	worker.Status = status
	worker.UsedResources = usedResources
	worker.LastHeartbeat = time.Now()
	return nil
}

// RemoveWorker removes a worker from the scheduler
func (rs *ResourceScheduler) RemoveWorker(workerID string) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	worker, exists := rs.workers[workerID]
	if !exists {
		return fmt.Errorf("worker with ID %s does not exist", workerID)
	}

	worker.mu.RLock()
	defer worker.mu.RUnlock()

	// Check if the worker has running tasks
	if len(worker.RunningTasks) > 0 {
		return fmt.Errorf("cannot remove worker %s with running tasks", workerID)
	}

	delete(rs.workers, workerID)
	return nil
}

// SubmitTask submits a task to be scheduled
func (rs *ResourceScheduler) SubmitTask(task *Task) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	if task.ID == "" {
		return errors.New("task ID cannot be empty")
	}

	// Check if task with the same ID already exists
	for _, t := range rs.pendingTasks {
		if t.ID == task.ID {
			return fmt.Errorf("task with ID %s already exists in pending queue", task.ID)
		}
	}
	if _, exists := rs.runningTasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s is already running", task.ID)
	}

	// Set default values
	if task.Priority == 0 {
		task.Priority = 5 // Default priority
	}
	if task.CreateTime.IsZero() {
		task.CreateTime = time.Now()
	}
	task.Status = "pending"

	// Record dependencies
	for _, depID := range task.Dependencies {
		if rs.taskDependencies[depID] == nil {
			rs.taskDependencies[depID] = make(map[string]struct{})
		}
		rs.taskDependencies[depID][task.ID] = struct{}{}
	}

	// Add to priority queue
	heap.Push(&rs.pendingTasks, task)
	return nil
}

// ScheduleTask attempts to schedule the highest priority task
func (rs *ResourceScheduler) ScheduleTask() (*Task, string, error) {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	if rs.pendingTasks.Len() == 0 {
		return nil, "", nil // No tasks to schedule
	}

	// Find the highest priority ready task
	var readyTaskIndex int = -1
	var readyTask *Task

	// Check each task in priority order
	for i := 0; i < rs.pendingTasks.Len(); i++ {
		task := rs.pendingTasks[i]
		
		// Check dependencies
		allDependenciesSatisfied := true
		for _, depID := range task.Dependencies {
			if depTask, exists := rs.runningTasks[depID]; exists && depTask.Status != "completed" {
				allDependenciesSatisfied = false
				break
			}
		}
		
		if allDependenciesSatisfied {
			readyTaskIndex = i
			readyTask = task
			break
		}
	}

	if readyTaskIndex == -1 {
		return nil, "", nil // No ready tasks
	}

	// Try to find a suitable worker
	bestWorker, preemptTasks := rs.findSuitableWorker(readyTask)
	if bestWorker == "" {
		return nil, "", nil // No suitable worker found
	}

	// If we need to preempt tasks
	if len(preemptTasks) > 0 {
		// Preempt lower priority tasks
		for _, taskID := range preemptTasks {
			task := rs.runningTasks[taskID]
			
			// Update task status
			task.Status = "preempted"
			
			// Push task back to pending queue
			heap.Push(&rs.pendingTasks, task)
			
			// Remove from running tasks
			delete(rs.runningTasks, taskID)
			
			// Update worker's resources
			worker := rs.workers[task.AssignedWorker]
			worker.mu.Lock()
			delete(worker.RunningTasks, taskID)
			worker.UsedResources.CPU -= task.ResourceRequirements.Requests.CPU
			worker.UsedResources.Memory -= task.ResourceRequirements.Requests.Memory
			worker.UsedResources.Disk -= task.ResourceRequirements.Requests.Disk
			worker.UsedResources.GPU -= task.ResourceRequirements.Requests.GPU
			worker.mu.Unlock()
		}
	}

	// Remove the task from the pending queue
	heap.Remove(&rs.pendingTasks, readyTaskIndex)

	// Update task status
	readyTask.Status = "scheduled"
	readyTask.AssignedWorker = bestWorker
	readyTask.StartTime = time.Now()

	// Add to running tasks
	rs.runningTasks[readyTask.ID] = readyTask

	// Update worker's resources
	worker := rs.workers[bestWorker]
	worker.mu.Lock()
	worker.RunningTasks[readyTask.ID] = struct{}{}
	worker.UsedResources.CPU += readyTask.ResourceRequirements.Requests.CPU
	worker.UsedResources.Memory += readyTask.ResourceRequirements.Requests.Memory
	worker.UsedResources.Disk += readyTask.ResourceRequirements.Requests.Disk
	worker.UsedResources.GPU += readyTask.ResourceRequirements.Requests.GPU
	worker.mu.Unlock()

	return readyTask, bestWorker, nil
}

// findSuitableWorker finds the best worker for a task
func (rs *ResourceScheduler) findSuitableWorker(task *Task) (string, []string) {
	var bestWorker string
	var preemptTasks []string
	minResourcesAvailable := false

	// First check: can we find a worker with available resources without preemption?
	for id, worker := range rs.workers {
		if worker.Status != "active" {
			continue
		}

		worker.mu.RLock()
		
		// Check if worker has enough resources
		if worker.TotalResources.CPU-worker.UsedResources.CPU >= task.ResourceRequirements.Requests.CPU &&
			worker.TotalResources.Memory-worker.UsedResources.Memory >= task.ResourceRequirements.Requests.Memory &&
			worker.TotalResources.Disk-worker.UsedResources.Disk >= task.ResourceRequirements.Requests.Disk &&
			worker.TotalResources.GPU-worker.UsedResources.GPU >= task.ResourceRequirements.Requests.GPU {
			
			bestWorker = id
			minResourcesAvailable = true
			worker.mu.RUnlock()
			return bestWorker, nil
		}
		
		worker.mu.RUnlock()
	}

	// If no worker has enough resources, check if preemption is allowed
	if !minResourcesAvailable && task.ResourceRequirements.Preemptible {
		// For each worker, check if preempting lower priority tasks would free enough resources
		for id, worker := range rs.workers {
			if worker.Status != "active" {
				continue
			}

			worker.mu.RLock()
			
			// Calculate how much resources we need to free
			neededCPU := task.ResourceRequirements.Requests.CPU - (worker.TotalResources.CPU - worker.UsedResources.CPU)
			neededMemory := task.ResourceRequirements.Requests.Memory - (worker.TotalResources.Memory - worker.UsedResources.Memory)
			neededDisk := task.ResourceRequirements.Requests.Disk - (worker.TotalResources.Disk - worker.UsedResources.Disk)
			neededGPU := task.ResourceRequirements.Requests.GPU - (worker.TotalResources.GPU - worker.UsedResources.GPU)
			
			// Collect all running tasks on this worker
			var tasksOnWorker []*Task
			for taskID := range worker.RunningTasks {
				if runningTask, exists := rs.runningTasks[taskID]; exists {
					tasksOnWorker = append(tasksOnWorker, runningTask)
				}
			}
			
			worker.mu.RUnlock()
			
			// Sort tasks by priority (lower priority first)
			sort.Slice(tasksOnWorker, func(i, j int) bool {
				return tasksOnWorker[i].Priority < tasksOnWorker[j].Priority
			})
			
			// Identify tasks to preempt
			var tasksToPreempt []string
			freeCPU, freeMemory, freeDisk, freeGPU := 0.0, 0.0, 0.0, 0
			
			for _, runningTask := range tasksOnWorker {
				// Only preempt lower priority tasks that are preemptible
				if runningTask.Priority < task.Priority && runningTask.ResourceRequirements.Preemptible {
					tasksToPreempt = append(tasksToPreempt, runningTask.ID)
					
					freeCPU += runningTask.ResourceRequirements.Requests.CPU
					freeMemory += runningTask.ResourceRequirements.Requests.Memory
					freeDisk += runningTask.ResourceRequirements.Requests.Disk
					freeGPU += runningTask.ResourceRequirements.Requests.GPU
					
					// Check if we've freed enough resources
					if freeCPU >= neededCPU && freeMemory >= neededMemory && 
					   freeDisk >= neededDisk && freeGPU >= neededGPU {
						bestWorker = id
						return bestWorker, tasksToPreempt
					}
				}
			}
		}
	}

	return "", nil
}

// CompleteTask marks a task as completed
func (rs *ResourceScheduler) CompleteTask(taskID string, result string) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	task, exists := rs.runningTasks[taskID]
	if !exists {
		return fmt.Errorf("task with ID %s is not running", taskID)
	}

	// Update task status
	task.Status = "completed"
	task.Result = result
	task.EndTime = time.Now()

	// Update worker resources
	workerID := task.AssignedWorker
	worker, exists := rs.workers[workerID]
	if !exists {
		return fmt.Errorf("worker with ID %s does not exist", workerID)
	}

	worker.mu.Lock()
	delete(worker.RunningTasks, taskID)
	worker.UsedResources.CPU -= task.ResourceRequirements.Requests.CPU
	worker.UsedResources.Memory -= task.ResourceRequirements.Requests.Memory
	worker.UsedResources.Disk -= task.ResourceRequirements.Requests.Disk
	worker.UsedResources.GPU -= task.ResourceRequirements.Requests.GPU
	worker.mu.Unlock()

	// Remove from running tasks
	delete(rs.runningTasks, taskID)

	// Check if any dependent tasks can now be scheduled
	if dependentTasks, hasDependents := rs.taskDependencies[taskID]; hasDependents {
		// Remove this dependency from all dependent tasks
		for depID := range dependentTasks {
			for i, t := range rs.pendingTasks {
				if t.ID == depID {
					// Remove the dependency
					newDeps := make([]string, 0, len(t.Dependencies)-1)
					for _, dep := range t.Dependencies {
						if dep != taskID {
							newDeps = append(newDeps, dep)
						}
					}
					t.Dependencies = newDeps
					rs.pendingTasks[i] = t
					break
				}
			}
		}
		delete(rs.taskDependencies, taskID)
	}

	return nil
}

// FailTask marks a task as failed
func (rs *ResourceScheduler) FailTask(taskID string, errorMsg string) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	task, exists := rs.runningTasks[taskID]
	if !exists {
		return fmt.Errorf("task with ID %s is not running", taskID)
	}

	// Update task status
	task.Status = "failed"
	task.Error = errorMsg
	task.EndTime = time.Now()
	task.RetryCount++

	// Check if task should be retried
	if task.RetryCount <= task.MaxRetries {
		// Reset task for retry
		retryTask := task.Clone()
		retryTask.Status = "pending"
		retryTask.StartTime = time.Time{}
		retryTask.EndTime = time.Time{}
		retryTask.AssignedWorker = ""
		retryTask.Result = ""
		retryTask.Error = ""

		// Add back to pending queue
		heap.Push(&rs.pendingTasks, retryTask)
	}

	// Update worker resources
	workerID := task.AssignedWorker
	worker, exists := rs.workers[workerID]
	if !exists {
		return fmt.Errorf("worker with ID %s does not exist", workerID)
	}

	worker.mu.Lock()
	delete(worker.RunningTasks, taskID)
	worker.UsedResources.CPU -= task.ResourceRequirements.Requests.CPU
	worker.UsedResources.Memory -= task.ResourceRequirements.Requests.Memory
	worker.UsedResources.Disk -= task.ResourceRequirements.Requests.Disk
	worker.UsedResources.GPU -= task.ResourceRequirements.Requests.GPU
	worker.mu.Unlock()

	// Remove from running tasks
	delete(rs.runningTasks, taskID)

	return nil
}

// GetTaskStatus returns the status of a task
func (rs *ResourceScheduler) GetTaskStatus(taskID string) (string, error) {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	// Check running tasks
	if task, exists := rs.runningTasks[taskID]; exists {
		return task.Status, nil
	}

	// Check pending tasks
	for _, task := range rs.pendingTasks {
		if task.ID == taskID {
			return task.Status, nil
		}
	}

	return "", fmt.Errorf("task with ID %s not found", taskID)
}

// GetWorkerStatus returns the status of a worker
func (rs *ResourceScheduler) GetWorkerStatus(workerID string) (*WorkerNode, error) {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	worker, exists := rs.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker with ID %s not found", workerID)
	}

	worker.mu.RLock()
	defer worker.mu.RUnlock()

	return worker, nil
}

// GetAllWorkers returns all registered workers
func (rs *ResourceScheduler) GetAllWorkers() []*WorkerNode {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	workers := make([]*WorkerNode, 0, len(rs.workers))
	for _, worker := range rs.workers {
		workers = append(workers, worker)
	}

	return workers
}

// GetPendingTasks returns all pending tasks
func (rs *ResourceScheduler) GetPendingTasks() []*Task {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	tasks := make([]*Task, rs.pendingTasks.Len())
	for i, task := range rs.pendingTasks {
		tasks[i] = task
	}

	return tasks
}

// GetRunningTasks returns all running tasks
func (rs *ResourceScheduler) GetRunningTasks() []*Task {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	tasks := make([]*Task, 0, len(rs.runningTasks))
	for _, task := range rs.runningTasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// HandleWorkerFailure handles the failure of a worker
func (rs *ResourceScheduler) HandleWorkerFailure(workerID string) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	worker, exists := rs.workers[workerID]
	if !exists {
		return fmt.Errorf("worker with ID %s not found", workerID)
	}

	worker.mu.Lock()
	worker.Status = "offline"
	tasksToReschedule := make([]string, 0, len(worker.RunningTasks))
	for taskID := range worker.RunningTasks {
		tasksToReschedule = append(tasksToReschedule, taskID)
	}
	worker.mu.Unlock()

	// Reschedule all tasks running on the failed worker
	for _, taskID := range tasksToReschedule {
		task, exists := rs.runningTasks[taskID]
		if exists {
			// Create a copy for rescheduling
			rescheduleTask := task.Clone()
			rescheduleTask.Status = "pending"
			rescheduleTask.StartTime = time.Time{}
			rescheduleTask.EndTime = time.Time{}
			rescheduleTask.AssignedWorker = ""

			// Add back to pending queue
			heap.Push(&rs.pendingTasks, rescheduleTask)

			// Remove from running tasks
			delete(rs.runningTasks, taskID)
		}
	}

	return nil
}

// CheckStaleWorkers checks for workers that haven't sent a heartbeat in a while
func (rs *ResourceScheduler) CheckStaleWorkers(timeout time.Duration) []string {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	now := time.Now()
	staleWorkers := make([]string, 0)

	for id, worker := range rs.workers {
		worker.mu.RLock()
		if worker.Status == "active" && now.Sub(worker.LastHeartbeat) > timeout {
			staleWorkers = append(staleWorkers, id)
		}
		worker.mu.RUnlock()
	}

	return staleWorkers
} 