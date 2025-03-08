package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/coordination"
	"github.com/yourusername/taskscheduler/pkg/metrics"
	"github.com/yourusername/taskscheduler/pkg/queue"
)

// FaultTolerantScheduler extends the basic scheduler with fault tolerance
type FaultTolerantScheduler struct {
	*Scheduler              // Embed the basic scheduler
	leaderElection    *coordination.LeaderElection
	recoveryService   *TaskRecoveryService
	workerRegistry    *coordination.WorkerRegistry
	metrics           *metrics.Metrics
	healthCheck       func(ctx context.Context) error
	instanceID        string
	isLeader          bool
	leaderCtx         context.Context
	leaderCancel      context.CancelFunc
}

// FaultTolerantSchedulerConfig holds configuration for the fault tolerant scheduler
type FaultTolerantSchedulerConfig struct {
	Repo                repository.TaskRepository
	Queue               queue.TaskQueue
	RedisClient         *redis.Client
	InstanceID          string
	LockKey             string
	LockDuration        time.Duration
	StaleTaskInterval   time.Duration
	WorkerRegistry      *coordination.WorkerRegistry
}

// NewFaultTolerantScheduler creates a new fault-tolerant scheduler
func NewFaultTolerantScheduler(config FaultTolerantSchedulerConfig) *FaultTolerantScheduler {
	// Create the basic scheduler
	basicScheduler := NewScheduler(config.Repo, config.Queue)
	
	// Get metrics singleton
	metrics := metrics.GetInstance()
	
	// Create leader election
	leaderCfg := coordination.LeaderElectionConfig{
		RedisClient:  config.RedisClient,
		LockKey:      config.LockKey,
		LockDuration: config.LockDuration,
	}
	
	leaderElection := coordination.NewLeaderElection(leaderCfg)
	
	// Create a dummy context that will be replaced when leadership is acquired
	dummyCtx, dummyCancel := context.WithCancel(context.Background())
	
	scheduler := &FaultTolerantScheduler{
		Scheduler:       basicScheduler,
		leaderElection:  leaderElection,
		workerRegistry:  config.WorkerRegistry,
		metrics:         metrics,
		instanceID:      config.InstanceID,
		isLeader:        false,
		leaderCtx:       dummyCtx,
		leaderCancel:    dummyCancel,
	}
	
	// Create recovery service
	recoveryService := NewTaskRecoveryService(
		config.Repo,
		config.Queue,
		config.WorkerRegistry,
		config.StaleTaskInterval,
		30*time.Second, // Check interval
	)
	scheduler.recoveryService = recoveryService
	
	// Create health check function
	scheduler.healthCheck = func(ctx context.Context) error {
		// A simple health check that verifies we can connect to Redis and the database
		// For Redis, we use the leader election's Redis client
		_, err := config.RedisClient.Ping(ctx).Result()
		if err != nil {
			return err
		}
		
		// For database, we use a simple query
		_, err = config.Repo.GetTasksByStatus(ctx, "running", 1)
		return err
	}
	
	return scheduler
}

// Start begins the fault-tolerant scheduler operation
func (s *FaultTolerantScheduler) Start(ctx context.Context) error {
	log.Printf("Starting fault-tolerant scheduler with instance ID: %s", s.instanceID)
	
	// Start the leader election process
	s.leaderElection.Start(ctx)
	
	// Start task recovery service
	s.recoveryService.Start()
	
	// Set up a goroutine to monitor leadership changes
	go s.monitorLeadership(ctx)
	
	// Update metrics
	s.metrics.RegisterCustomGauge("scheduler_is_leader", "Whether this scheduler instance is the leader (1=true, 0=false)", []string{"instance_id"})
	s.metrics.SetCustomGauge("scheduler_is_leader", 0, s.instanceID)
	
	log.Println("Fault-tolerant scheduler started successfully")
	return nil
}

// Stop terminates the fault-tolerant scheduler
func (s *FaultTolerantScheduler) Stop() {
	log.Println("Stopping fault-tolerant scheduler")
	
	// Stop the leader election process
	s.leaderElection.Stop()
	
	// If we're the leader, stop the scheduler
	if s.isLeader {
		s.leaderCancel()
		s.Scheduler.Stop()
	}
	
	// Stop the recovery service
	s.recoveryService.Stop()
	
	log.Println("Fault-tolerant scheduler stopped")
}

// monitorLeadership watches for leadership changes and reacts accordingly
func (s *FaultTolerantScheduler) monitorLeadership(ctx context.Context) {
	leaderChangeCh := s.leaderElection.GetLeaderChangeChan()
	
	for {
		select {
		case <-ctx.Done():
			return
		case isLeader := <-leaderChangeCh:
			if isLeader {
				s.handleLeadershipAcquisition()
			} else {
				s.handleLeadershipLoss()
			}
		}
	}
}

// handleLeadershipAcquisition handles acquiring leadership
func (s *FaultTolerantScheduler) handleLeadershipAcquisition() {
	log.Printf("Instance %s acquired leadership, activating scheduler", s.instanceID)
	
	s.isLeader = true
	
	// Create a new context for this leadership period
	s.leaderCtx, s.leaderCancel = context.WithCancel(context.Background())
	
	// Start the scheduler with the new context
	if err := s.Scheduler.Start(s.leaderCtx); err != nil {
		log.Printf("Error starting scheduler: %v", err)
		return
	}
	
	// Recover any stuck tasks
	if err := s.recoveryService.RecoverStuckTasks(); err != nil {
		log.Printf("Error recovering stuck tasks: %v", err)
	}
	
	// Update metrics
	s.metrics.SetCustomGauge("scheduler_is_leader", 1, s.instanceID)
	
	log.Printf("Scheduler is now active on instance %s", s.instanceID)
}

// handleLeadershipLoss handles losing leadership
func (s *FaultTolerantScheduler) handleLeadershipLoss() {
	if !s.isLeader {
		return // We weren't the leader anyway
	}
	
	log.Printf("Instance %s lost leadership, deactivating scheduler", s.instanceID)
	
	// Cancel the leader context to stop all leadership-related activities
	s.leaderCancel()
	
	// Stop the scheduler
	s.Scheduler.Stop()
	
	s.isLeader = false
	
	// Update metrics
	s.metrics.SetCustomGauge("scheduler_is_leader", 0, s.instanceID)
	
	log.Printf("Scheduler is now inactive on instance %s", s.instanceID)
}

// IsLeader returns whether this instance is currently the leader
func (s *FaultTolerantScheduler) IsLeader() bool {
	return s.isLeader
}

// GetInstanceID returns this instance's ID
func (s *FaultTolerantScheduler) GetInstanceID() string {
	return s.instanceID
}

// CheckHealth performs a health check and returns error if unhealthy
func (s *FaultTolerantScheduler) CheckHealth(ctx context.Context) error {
	return s.healthCheck(ctx)
} 