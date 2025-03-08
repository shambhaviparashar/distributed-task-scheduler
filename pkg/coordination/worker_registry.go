package coordination

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// Default worker heartbeat interval
	defaultWorkerHeartbeatInterval = 5 * time.Second
	
	// Default worker heartbeat timeout
	defaultWorkerHeartbeatTimeout = 15 * time.Second
	
	// Worker registry key prefix
	workerRegistryPrefix = "worker:registry:"
	
	// Worker heartbeat channel
	workerHeartbeatChannel = "worker:heartbeat"
)

// WorkerInfo holds information about a worker instance
type WorkerInfo struct {
	ID           string    `json:"id"`
	Host         string    `json:"host"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Status       string    `json:"status"`
	Tags         []string  `json:"tags"`
	Capacity     int       `json:"capacity"`
	Utilization  int       `json:"utilization"`
}

// WorkerRegistry manages worker registration and heartbeats
type WorkerRegistry struct {
	client           *redis.Client
	instanceID       string
	workerInfo       *WorkerInfo
	heartbeatTicker  *time.Ticker
	checkTicker      *time.Ticker
	activeWorkers    map[string]*WorkerInfo
	failureHandlers  []func(string)
	joinHandlers     []func(string)
	mu               sync.RWMutex
	heartbeatTimeout time.Duration
	isRunning        bool
	pubsub           *redis.PubSub
	ctx              context.Context
	cancel           context.CancelFunc
}

// WorkerRegistryConfig holds configuration for the worker registry
type WorkerRegistryConfig struct {
	Client         *redis.Client
	InstanceID     string
	Capacity       int
	Tags           []string
	HeartbeatTimeout time.Duration
}

// NewWorkerRegistry creates a new worker registry
func NewWorkerRegistry(config WorkerRegistryConfig) *WorkerRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Use default timeout if not specified
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = defaultWorkerHeartbeatTimeout
	}
	
	wr := &WorkerRegistry{
		client:           config.Client,
		instanceID:       config.InstanceID,
		activeWorkers:    make(map[string]*WorkerInfo),
		failureHandlers:  []func(string){},
		joinHandlers:     []func(string){},
		heartbeatTimeout: config.HeartbeatTimeout,
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// Create worker info
	wr.workerInfo = &WorkerInfo{
		ID:           config.InstanceID,
		Host:         getHostname(),
		LastHeartbeat: time.Now(),
		Status:       "active",
		Tags:         config.Tags,
		Capacity:     config.Capacity,
		Utilization:  0, // Start with 0 utilization
	}
	
	return wr
}

// Start begins the worker registry operation
func (wr *WorkerRegistry) Start() error {
	// Avoid starting twice
	wr.mu.Lock()
	if wr.isRunning {
		wr.mu.Unlock()
		return nil
	}
	wr.isRunning = true
	wr.mu.Unlock()
	
	// Register this worker
	if err := wr.registerWorker(); err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}
	
	// Start sending heartbeats
	wr.heartbeatTicker = time.NewTicker(defaultWorkerHeartbeatInterval)
	go wr.sendHeartbeats()
	
	// Start checking for worker timeouts
	wr.checkTicker = time.NewTicker(defaultWorkerHeartbeatInterval)
	go wr.checkWorkerTimeouts()
	
	// Subscribe to worker heartbeats
	wr.pubsub = wr.client.Subscribe(wr.ctx, workerHeartbeatChannel)
	go wr.receiveHeartbeats()
	
	log.Printf("Worker registry started for worker %s", wr.instanceID)
	return nil
}

// Stop terminates the worker registry operation
func (wr *WorkerRegistry) Stop() error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	if !wr.isRunning {
		return nil
	}
	
	// Cancel context to stop all goroutines
	wr.cancel()
	
	// Stop tickers
	if wr.heartbeatTicker != nil {
		wr.heartbeatTicker.Stop()
	}
	if wr.checkTicker != nil {
		wr.checkTicker.Stop()
	}
	
	// Unsubscribe from pubsub
	if wr.pubsub != nil {
		wr.pubsub.Close()
	}
	
	// Update status to inactive
	wr.workerInfo.Status = "inactive"
	if err := wr.updateWorkerInfo(); err != nil {
		log.Printf("Error updating worker status to inactive: %v", err)
	}
	
	wr.isRunning = false
	log.Printf("Worker registry stopped for worker %s", wr.instanceID)
	return nil
}

// registerWorker registers this worker in the registry
func (wr *WorkerRegistry) registerWorker() error {
	key := fmt.Sprintf("%s%s", workerRegistryPrefix, wr.instanceID)
	
	// Serialize worker info
	data, err := json.Marshal(wr.workerInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal worker info: %w", err)
	}
	
	// Store in Redis with expiration
	err = wr.client.Set(wr.ctx, key, data, wr.heartbeatTimeout*3).Err()
	if err != nil {
		return fmt.Errorf("failed to store worker info: %w", err)
	}
	
	// Announce worker joining
	announcement := map[string]string{
		"type":     "join",
		"worker_id": wr.instanceID,
	}
	
	announcementData, _ := json.Marshal(announcement)
	err = wr.client.Publish(wr.ctx, workerHeartbeatChannel, announcementData).Err()
	if err != nil {
		log.Printf("Failed to announce worker joining: %v", err)
	}
	
	return nil
}

// updateWorkerInfo updates the worker information in the registry
func (wr *WorkerRegistry) updateWorkerInfo() error {
	key := fmt.Sprintf("%s%s", workerRegistryPrefix, wr.instanceID)
	
	// Update last heartbeat time
	wr.workerInfo.LastHeartbeat = time.Now()
	
	// Serialize worker info
	data, err := json.Marshal(wr.workerInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal worker info: %w", err)
	}
	
	// Store in Redis with expiration
	err = wr.client.Set(wr.ctx, key, data, wr.heartbeatTimeout*3).Err()
	if err != nil {
		return fmt.Errorf("failed to update worker info: %w", err)
	}
	
	return nil
}

// sendHeartbeats periodically sends heartbeats to the registry
func (wr *WorkerRegistry) sendHeartbeats() {
	for {
		select {
		case <-wr.ctx.Done():
			return
		case <-wr.heartbeatTicker.C:
			// Send heartbeat
			heartbeat := map[string]interface{}{
				"type":     "heartbeat",
				"worker_id": wr.instanceID,
				"timestamp": time.Now().UnixNano(),
				"utilization": wr.workerInfo.Utilization,
			}
			
			data, _ := json.Marshal(heartbeat)
			err := wr.client.Publish(wr.ctx, workerHeartbeatChannel, data).Err()
			if err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
			
			// Update worker info in registry
			wr.mu.Lock()
			if err := wr.updateWorkerInfo(); err != nil {
				log.Printf("Failed to update worker info: %v", err)
			}
			wr.mu.Unlock()
		}
	}
}

// receiveHeartbeats listens for heartbeats from other workers
func (wr *WorkerRegistry) receiveHeartbeats() {
	ch := wr.pubsub.Channel()
	for {
		select {
		case <-wr.ctx.Done():
			return
		case msg := <-ch:
			// Parse heartbeat
			var heartbeat map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &heartbeat); err != nil {
				log.Printf("Failed to parse heartbeat: %v", err)
				continue
			}
			
			heartbeatType, _ := heartbeat["type"].(string)
			workerID, _ := heartbeat["worker_id"].(string)
			
			// Skip our own heartbeats
			if workerID == wr.instanceID {
				continue
			}
			
			// Process based on heartbeat type
			switch heartbeatType {
			case "heartbeat":
				// Update last heartbeat time
				wr.mu.Lock()
				if worker, exists := wr.activeWorkers[workerID]; exists {
					worker.LastHeartbeat = time.Now()
					if utilization, ok := heartbeat["utilization"].(float64); ok {
						worker.Utilization = int(utilization)
					}
				} else {
					// New worker discovered, fetch its info
					go wr.fetchWorkerInfo(workerID)
				}
				wr.mu.Unlock()
				
			case "join":
				// New worker joined, fetch its info
				go wr.fetchWorkerInfo(workerID)
				
			case "leave":
				// Worker left gracefully, remove it
				wr.mu.Lock()
				delete(wr.activeWorkers, workerID)
				wr.mu.Unlock()
			}
		}
	}
}

// fetchWorkerInfo retrieves worker information from the registry
func (wr *WorkerRegistry) fetchWorkerInfo(workerID string) {
	key := fmt.Sprintf("%s%s", workerRegistryPrefix, workerID)
	
	// Get worker data from Redis
	data, err := wr.client.Get(wr.ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Failed to fetch worker info for %s: %v", workerID, err)
		}
		return
	}
	
	// Parse worker info
	var workerInfo WorkerInfo
	if err := json.Unmarshal(data, &workerInfo); err != nil {
		log.Printf("Failed to parse worker info for %s: %v", workerID, err)
		return
	}
	
	// Update active workers map
	wr.mu.Lock()
	
	// Check if this is a new worker
	isNew := false
	if _, exists := wr.activeWorkers[workerID]; !exists {
		isNew = true
	}
	
	// Store worker info
	wr.activeWorkers[workerID] = &workerInfo
	wr.mu.Unlock()
	
	// Call join handlers for new workers
	if isNew {
		for _, handler := range wr.joinHandlers {
			go handler(workerID)
		}
	}
}

// checkWorkerTimeouts periodically checks for workers that have timed out
func (wr *WorkerRegistry) checkWorkerTimeouts() {
	for {
		select {
		case <-wr.ctx.Done():
			return
		case <-wr.checkTicker.C:
			now := time.Now()
			var failedWorkers []string
			
			// Find timed out workers
			wr.mu.Lock()
			for id, worker := range wr.activeWorkers {
				if now.Sub(worker.LastHeartbeat) > wr.heartbeatTimeout {
					failedWorkers = append(failedWorkers, id)
					delete(wr.activeWorkers, id)
				}
			}
			wr.mu.Unlock()
			
			// Call failure handlers for timed out workers
			for _, id := range failedWorkers {
				log.Printf("Worker %s failed (heartbeat timeout)", id)
				for _, handler := range wr.failureHandlers {
					go handler(id)
				}
			}
		}
	}
}

// OnWorkerFailure registers a handler to be called when a worker fails
func (wr *WorkerRegistry) OnWorkerFailure(handler func(string)) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	wr.failureHandlers = append(wr.failureHandlers, handler)
}

// OnWorkerJoin registers a handler to be called when a new worker joins
func (wr *WorkerRegistry) OnWorkerJoin(handler func(string)) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	wr.joinHandlers = append(wr.joinHandlers, handler)
}

// GetActiveWorkers returns a list of currently active workers
func (wr *WorkerRegistry) GetActiveWorkers() []*WorkerInfo {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	
	workers := make([]*WorkerInfo, 0, len(wr.activeWorkers))
	for _, worker := range wr.activeWorkers {
		workerCopy := *worker // Make a copy to avoid race conditions
		workers = append(workers, &workerCopy)
	}
	
	return workers
}

// UpdateUtilization updates the worker's utilization level
func (wr *WorkerRegistry) UpdateUtilization(utilization int) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	wr.workerInfo.Utilization = utilization
}

// getHostname returns the hostname or a fallback if not available
func getHostname() string {
	hostname, err := getLocalHostname()
	if err != nil {
		return "unknown-host"
	}
	return hostname
}

// Wrapper function to make testing easier
var getLocalHostname = func() (string, error) {
	return os.Hostname()
} 