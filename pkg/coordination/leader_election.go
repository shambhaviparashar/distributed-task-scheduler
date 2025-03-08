package coordination

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	// Default lock duration
	defaultLockDuration = 10 * time.Second
	
	// Default retry interval
	defaultRetryInterval = 2 * time.Second
	
	// Heartbeat interval should be less than lock duration
	defaultHeartbeatInterval = 3 * time.Second
)

// ErrNotLeader indicates that the current instance is not the leader
var ErrNotLeader = errors.New("not the leader")

// LeaderElection handles leader election using Redis
type LeaderElection struct {
	client         *redis.Client
	lockKey        string
	lockValue      string
	lockDuration   time.Duration
	retryInterval  time.Duration
	heartbeatCtx   context.Context
	heartbeatStop  context.CancelFunc
	isLeader       bool
	leaderChangeCh chan bool
	instanceID     string
}

// LeaderElectionConfig holds configuration for the leader election
type LeaderElectionConfig struct {
	RedisClient    *redis.Client
	LockKey        string
	LockDuration   time.Duration
	RetryInterval  time.Duration
}

// NewLeaderElection creates a new leader election instance
func NewLeaderElection(config LeaderElectionConfig) *LeaderElection {
	// Generate a unique instance ID (hostname + uuid)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	instanceID := fmt.Sprintf("%s-%s", hostname, uuid.New().String())

	// Use defaults if not provided
	if config.LockDuration == 0 {
		config.LockDuration = defaultLockDuration
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = defaultRetryInterval
	}
	if config.LockKey == "" {
		config.LockKey = "scheduler:leader"
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LeaderElection{
		client:         config.RedisClient,
		lockKey:        config.LockKey,
		lockValue:      instanceID,
		lockDuration:   config.LockDuration,
		retryInterval:  config.RetryInterval,
		heartbeatCtx:   ctx,
		heartbeatStop:  cancel,
		isLeader:       false,
		leaderChangeCh: make(chan bool, 1),
		instanceID:     instanceID,
	}
}

// Start begins the leader election process
func (le *LeaderElection) Start(ctx context.Context) {
	log.Printf("Starting leader election process with instance ID: %s", le.instanceID)
	
	// Try to acquire leader lock initially
	le.tryAcquireLock(ctx)
	
	// Start a goroutine to continuously try to acquire the lock
	go le.electionLoop(ctx)
}

// electionLoop continuously tries to acquire the leader lock
func (le *LeaderElection) electionLoop(ctx context.Context) {
	ticker := time.NewTicker(le.retryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Leader election loop stopped due to context cancellation")
			le.resignLeadership()
			return
		case <-ticker.C:
			le.tryAcquireLock(ctx)
		}
	}
}

// tryAcquireLock attempts to acquire the leader lock
func (le *LeaderElection) tryAcquireLock(ctx context.Context) {
	// Skip if already leader
	if le.isLeader {
		return
	}
	
	// Try to acquire lock with NX (only if key does not exist) and expiration
	success, err := le.client.SetNX(ctx, le.lockKey, le.lockValue, le.lockDuration).Result()
	if err != nil {
		log.Printf("Error trying to acquire leader lock: %v", err)
		return
	}
	
	if success {
		log.Printf("Instance %s acquired leadership", le.instanceID)
		le.isLeader = true
		le.leaderChangeCh <- true
		
		// Start heartbeat to maintain leadership
		go le.startHeartbeat(le.heartbeatCtx)
	}
}

// startHeartbeat periodically refreshes the lock to maintain leadership
func (le *LeaderElection) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(defaultHeartbeatInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Leader heartbeat stopped due to context cancellation")
			return
		case <-ticker.C:
			if !le.isLeader {
				return
			}
			
			// Extend the lock duration
			exists, err := le.client.Exists(ctx, le.lockKey).Result()
			if err != nil || exists == 0 {
				log.Printf("Lost leadership, lock no longer exists: %v", err)
				le.resignLeadership()
				return
			}
			
			// Check if we're still the leader by checking the lock value
			val, err := le.client.Get(ctx, le.lockKey).Result()
			if err != nil || val != le.lockValue {
				log.Printf("Lost leadership, another instance took over: %v", err)
				le.resignLeadership()
				return
			}
			
			// Extend lock
			_, err = le.client.Expire(ctx, le.lockKey, le.lockDuration).Result()
			if err != nil {
				log.Printf("Failed to refresh leader lock: %v", err)
				le.resignLeadership()
				return
			}
			
			log.Printf("Leadership heartbeat refreshed for instance %s", le.instanceID)
		}
	}
}

// resignLeadership gives up leadership
func (le *LeaderElection) resignLeadership() {
	if !le.isLeader {
		return
	}
	
	// Only delete the key if we're the leader
	ctx := context.Background()
	val, err := le.client.Get(ctx, le.lockKey).Result()
	if err == nil && val == le.lockValue {
		_, err = le.client.Del(ctx, le.lockKey).Result()
		if err != nil {
			log.Printf("Error deleting leadership key during resignation: %v", err)
		}
	}
	
	le.isLeader = false
	le.leaderChangeCh <- false
	log.Printf("Instance %s resigned leadership", le.instanceID)
}

// IsLeader returns true if the current instance is the leader
func (le *LeaderElection) IsLeader() bool {
	return le.isLeader
}

// GetLeaderChangeChan returns a channel that signals leadership changes
func (le *LeaderElection) GetLeaderChangeChan() <-chan bool {
	return le.leaderChangeCh
}

// Stop terminates the leader election process
func (le *LeaderElection) Stop() {
	le.heartbeatStop()
	le.resignLeadership()
}

// GetInstanceID returns this instance's unique identifier
func (le *LeaderElection) GetInstanceID() string {
	return le.instanceID
} 