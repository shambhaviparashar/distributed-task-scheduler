package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// Errors
var (
	ErrNodeNotFound        = errors.New("node not found")
	ErrLeaderNotElected    = errors.New("leader not elected")
	ErrNodeAlreadyExists   = errors.New("node already exists")
	ErrInvalidNodeState    = errors.New("invalid node state")
	ErrLeaderTransferFailed = errors.New("leader transfer failed")
	ErrReplicationFailed   = errors.New("replication failed")
)

// NodeStatus represents the status of a node
type NodeStatus string

// Node statuses
const (
	NodeStatusActive   NodeStatus = "active"
	NodeStatusInactive NodeStatus = "inactive"
	NodeStatusStarting NodeStatus = "starting"
	NodeStatusLeaving  NodeStatus = "leaving"
	NodeStatusFailed   NodeStatus = "failed"
)

// NodeRole represents the role of a node
type NodeRole string

// Node roles
const (
	NodeRoleLeader  NodeRole = "leader"
	NodeRoleFollower NodeRole = "follower"
	NodeRoleObserver NodeRole = "observer"
)

// NodeType represents the type of a node
type NodeType string

// Node types
const (
	NodeTypeScheduler NodeType = "scheduler"
	NodeTypeWorker    NodeType = "worker"
	NodeTypeAPI       NodeType = "api"
	NodeTypeAll       NodeType = "all"
)

// NodeInfo represents information about a node
type NodeInfo struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Status         NodeStatus `json:"status"`
	Role           NodeRole   `json:"role"`
	Type           NodeType   `json:"type"`
	Address        string     `json:"address"`
	Port           int        `json:"port"`
	StartTime      time.Time  `json:"start_time"`
	LastHeartbeat  time.Time  `json:"last_heartbeat"`
	Version        string     `json:"version"`
	Load           NodeLoad   `json:"load"`
	Tags           []string   `json:"tags,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Region         string     `json:"region,omitempty"`
	Zone           string     `json:"zone,omitempty"`
	LeadershipTerm int64      `json:"leadership_term,omitempty"`
}

// NodeLoad represents load information about a node
type NodeLoad struct {
	CPUUsage       float64 `json:"cpu_usage"`       // CPU usage (0-100%)
	MemoryUsage    float64 `json:"memory_usage"`    // Memory usage (0-100%)
	TaskCount      int     `json:"task_count"`      // Number of tasks being processed
	QueueDepth     int     `json:"queue_depth"`     // Depth of task queue
	NetworkInMbps  float64 `json:"network_in_mbps"` // Network inbound traffic in Mbps
	NetworkOutMbps float64 `json:"network_out_mbps"`// Network outbound traffic in Mbps
	DiskUsage      float64 `json:"disk_usage"`      // Disk usage (0-100%)
	LastUpdated    time.Time `json:"last_updated"`  // Time of last load update
}

// ClusterInfo represents information about a cluster
type ClusterInfo struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	LeaderID        string    `json:"leader_id"`
	NodeCount       int       `json:"node_count"`
	SchedulerCount  int       `json:"scheduler_count"`
	WorkerCount     int       `json:"worker_count"`
	APICount        int       `json:"api_count"`
	UpdateTime      time.Time `json:"update_time"`
	Status          string    `json:"status"`
	Version         string    `json:"version"`
	CurrentTerm     int64     `json:"current_term"`
}

// ReplicationEvent represents an event to be replicated
type ReplicationEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	NodeID    string                 `json:"node_id"`
	Sequence  int64                  `json:"sequence"`
	Term      int64                  `json:"term"`
}

// Store defines the interface for a distributed store
type Store interface {
	// Node management
	RegisterNode(node *NodeInfo) error
	UpdateNode(node *NodeInfo) error
	GetNode(nodeID string) (*NodeInfo, error)
	ListNodes() ([]*NodeInfo, error)
	DeregisterNode(nodeID string) error
	
	// Leadership
	AcquireLock(lockID string, nodeID string, ttl time.Duration) (bool, error)
	ReleaseLock(lockID string, nodeID string) error
	GetLock(lockID string) (string, error)
	
	// Events
	PublishEvent(event *ReplicationEvent) error
	SubscribeEvents(eventType string) (<-chan *ReplicationEvent, error)
	GetEvents(startSequence int64, limit int) ([]*ReplicationEvent, error)
	
	// Cluster information
	GetClusterInfo() (*ClusterInfo, error)
	UpdateClusterInfo(info *ClusterInfo) error
}

// ClusterManager manages high availability and replication
type ClusterManager struct {
	store           Store
	localNode       *NodeInfo
	clusterInfo     *ClusterInfo
	isLeader        bool
	leadershipLock  string
	heartbeatInterval time.Duration
	electionTimeout   time.Duration
	eventCh         chan *ReplicationEvent
	stopCh          chan struct{}
	mu              sync.RWMutex
	listeners       map[string][]EventListener
	nodeFailureHandlers []NodeFailureHandler
	ctx             context.Context
	cancel          context.CancelFunc
}

// EventListener is a callback for events
type EventListener func(event *ReplicationEvent) error

// NodeFailureHandler is a callback for node failures
type NodeFailureHandler func(nodeID string, info *NodeInfo) error

// ClusterManagerConfig contains configuration for the cluster manager
type ClusterManagerConfig struct {
	NodeID            string
	NodeName          string
	NodeType          NodeType
	Address           string
	Port              int
	Region            string
	Zone              string
	Tags              []string
	Metadata          map[string]string
	HeartbeatInterval time.Duration
	ElectionTimeout   time.Duration
	LeadershipLock    string
	ClusterID         string
	ClusterName       string
	Version           string
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(store Store, config ClusterManagerConfig) (*ClusterManager, error) {
	if config.NodeID == "" {
		return nil, errors.New("node ID is required")
	}

	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 5 * time.Second
	}

	if config.ElectionTimeout == 0 {
		config.ElectionTimeout = 15 * time.Second
	}

	if config.LeadershipLock == "" {
		config.LeadershipLock = "leadership"
	}

	ctx, cancel := context.WithCancel(context.Background())

	localNode := &NodeInfo{
		ID:            config.NodeID,
		Name:          config.NodeName,
		Status:        NodeStatusStarting,
		Role:          NodeRoleFollower,
		Type:          config.NodeType,
		Address:       config.Address,
		Port:          config.Port,
		StartTime:     time.Now(),
		LastHeartbeat: time.Now(),
		Version:       config.Version,
		Tags:          config.Tags,
		Metadata:      config.Metadata,
		Region:        config.Region,
		Zone:          config.Zone,
		Load: NodeLoad{
			LastUpdated: time.Now(),
		},
	}

	clusterInfo := &ClusterInfo{
		ID:          config.ClusterID,
		Name:        config.ClusterName,
		UpdateTime:  time.Now(),
		Status:      "initializing",
		Version:     config.Version,
		CurrentTerm: 0,
	}

	cm := &ClusterManager{
		store:             store,
		localNode:         localNode,
		clusterInfo:       clusterInfo,
		isLeader:          false,
		leadershipLock:    config.LeadershipLock,
		heartbeatInterval: config.HeartbeatInterval,
		electionTimeout:   config.ElectionTimeout,
		eventCh:           make(chan *ReplicationEvent, 100),
		stopCh:            make(chan struct{}),
		listeners:         make(map[string][]EventListener),
		ctx:               ctx,
		cancel:            cancel,
	}

	return cm, nil
}

// Start starts the cluster manager
func (cm *ClusterManager) Start() error {
	// Register the node
	cm.localNode.Status = NodeStatusActive
	if err := cm.store.RegisterNode(cm.localNode); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Initialize or get cluster info
	existingInfo, err := cm.store.GetClusterInfo()
	if err == nil && existingInfo != nil {
		cm.clusterInfo = existingInfo
	} else {
		// Initialize cluster info
		if err := cm.store.UpdateClusterInfo(cm.clusterInfo); err != nil {
			return fmt.Errorf("failed to initialize cluster info: %w", err)
		}
	}

	// Subscribe to events
	eventCh, err := cm.store.SubscribeEvents("")
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Start background processes
	go cm.heartbeatLoop()
	go cm.leaderElectionLoop()
	go cm.eventLoop(eventCh)
	go cm.nodeMonitorLoop()

	return nil
}

// Stop stops the cluster manager
func (cm *ClusterManager) Stop() error {
	cm.cancel()
	close(cm.stopCh)

	// If we're the leader, release the leadership lock
	if cm.isLeader {
		if err := cm.store.ReleaseLock(cm.leadershipLock, cm.localNode.ID); err != nil {
			log.Printf("Error releasing leadership lock: %v", err)
		}
	}

	// Update node status to leaving
	cm.localNode.Status = NodeStatusLeaving
	if err := cm.store.UpdateNode(cm.localNode); err != nil {
		log.Printf("Error updating node status: %v", err)
	}

	// Deregister the node
	if err := cm.store.DeregisterNode(cm.localNode.ID); err != nil {
		log.Printf("Error deregistering node: %v", err)
	}

	return nil
}

// heartbeatLoop periodically sends heartbeats
func (cm *ClusterManager) heartbeatLoop() {
	ticker := time.NewTicker(cm.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.sendHeartbeat()
		case <-cm.stopCh:
			return
		}
	}
}

// sendHeartbeat sends a heartbeat
func (cm *ClusterManager) sendHeartbeat() {
	cm.mu.RLock()
	node := *cm.localNode
	cm.mu.RUnlock()

	node.LastHeartbeat = time.Now()
	
	// Update load information
	node.Load.LastUpdated = time.Now()
	// In a real implementation, we'd collect actual load metrics here

	if err := cm.store.UpdateNode(&node); err != nil {
		log.Printf("Error sending heartbeat: %v", err)
	}

	// If we're the leader, publish a leadership heartbeat event
	if cm.isLeader {
		event := &ReplicationEvent{
			ID:        fmt.Sprintf("heartbeat-%s-%d", cm.localNode.ID, time.Now().UnixNano()),
			Type:      "leader_heartbeat",
			Data:      map[string]interface{}{"leader_id": cm.localNode.ID, "term": cm.localNode.LeadershipTerm},
			Timestamp: time.Now(),
			NodeID:    cm.localNode.ID,
			Sequence:  time.Now().UnixNano(),
			Term:      cm.localNode.LeadershipTerm,
		}

		if err := cm.store.PublishEvent(event); err != nil {
			log.Printf("Error publishing leadership heartbeat: %v", err)
		}
	}
}

// leaderElectionLoop periodically tries to become the leader
func (cm *ClusterManager) leaderElectionLoop() {
	ticker := time.NewTicker(cm.electionTimeout / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.tryBecomeLeader()
		case <-cm.stopCh:
			return
		}
	}
}

// tryBecomeLeader attempts to become the leader
func (cm *ClusterManager) tryBecomeLeader() {
	// If we're already the leader, just renew the lock
	if cm.isLeader {
		acquired, err := cm.store.AcquireLock(cm.leadershipLock, cm.localNode.ID, cm.electionTimeout*2)
		if err != nil || !acquired {
			// We lost leadership
			cm.mu.Lock()
			cm.isLeader = false
			cm.localNode.Role = NodeRoleFollower
			cm.mu.Unlock()

			log.Printf("Lost leadership: %v", err)
		}
		return
	}

	// Check if there's already a leader
	leaderID, err := cm.store.GetLock(cm.leadershipLock)
	if err == nil && leaderID != "" && leaderID != cm.localNode.ID {
		// There's already a leader that's not us
		return
	}

	// Try to acquire the leadership lock
	acquired, err := cm.store.AcquireLock(cm.leadershipLock, cm.localNode.ID, cm.electionTimeout*2)
	if err != nil || !acquired {
		// Failed to acquire leadership
		return
	}

	// We acquired leadership
	cm.mu.Lock()
	cm.isLeader = true
	cm.localNode.Role = NodeRoleLeader
	cm.localNode.LeadershipTerm++
	term := cm.localNode.LeadershipTerm
	cm.mu.Unlock()

	// Update cluster info
	info, err := cm.store.GetClusterInfo()
	if err == nil {
		info.LeaderID = cm.localNode.ID
		info.CurrentTerm = term
		info.UpdateTime = time.Now()
		if err := cm.store.UpdateClusterInfo(info); err != nil {
			log.Printf("Error updating cluster info after leadership acquisition: %v", err)
		}
	}

	// Publish leadership event
	event := &ReplicationEvent{
		ID:        fmt.Sprintf("leader_elected-%s-%d", cm.localNode.ID, time.Now().UnixNano()),
		Type:      "leader_elected",
		Data:      map[string]interface{}{"leader_id": cm.localNode.ID, "term": term},
		Timestamp: time.Now(),
		NodeID:    cm.localNode.ID,
		Sequence:  time.Now().UnixNano(),
		Term:      term,
	}

	if err := cm.store.PublishEvent(event); err != nil {
		log.Printf("Error publishing leadership event: %v", err)
	}

	// Update our node info
	if err := cm.store.UpdateNode(cm.localNode); err != nil {
		log.Printf("Error updating node after leadership acquisition: %v", err)
	}

	log.Printf("Node %s became leader with term %d", cm.localNode.ID, term)
}

// eventLoop processes events from the store
func (cm *ClusterManager) eventLoop(eventCh <-chan *ReplicationEvent) {
	for {
		select {
		case event := <-eventCh:
			if event == nil {
				continue
			}
			cm.handleEvent(event)
		case <-cm.stopCh:
			return
		}
	}
}

// handleEvent processes a replication event
func (cm *ClusterManager) handleEvent(event *ReplicationEvent) {
	// Handle specific event types
	switch event.Type {
	case "leader_elected":
		cm.handleLeaderElection(event)
	case "node_failed":
		cm.handleNodeFailure(event)
	case "config_updated":
		cm.handleConfigUpdate(event)
	case "leader_heartbeat":
		// Just used to ensure leader is still active
	}

	// Notify listeners
	if listeners, ok := cm.listeners[event.Type]; ok {
		for _, listener := range listeners {
			if err := listener(event); err != nil {
				log.Printf("Error in event listener for %s: %v", event.Type, err)
			}
		}
	}

	// Notify global listeners
	if listeners, ok := cm.listeners["*"]; ok {
		for _, listener := range listeners {
			if err := listener(event); err != nil {
				log.Printf("Error in global event listener: %v", err)
			}
		}
	}
}

// handleLeaderElection processes a leader election event
func (cm *ClusterManager) handleLeaderElection(event *ReplicationEvent) {
	leaderID, ok := event.Data["leader_id"].(string)
	if !ok {
		log.Printf("Invalid leader_id in leader_elected event")
		return
	}

	term, ok := event.Data["term"].(float64)
	if !ok {
		log.Printf("Invalid term in leader_elected event")
		return
	}

	// If we think we're the leader but someone else was elected with a higher term,
	// step down
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isLeader && leaderID != cm.localNode.ID && int64(term) > cm.localNode.LeadershipTerm {
		cm.isLeader = false
		cm.localNode.Role = NodeRoleFollower
		if err := cm.store.UpdateNode(cm.localNode); err != nil {
			log.Printf("Error updating node after stepping down: %v", err)
		}
		log.Printf("Stepped down as leader due to election of %s with term %d", leaderID, int64(term))
	}
}

// handleNodeFailure processes a node failure event
func (cm *ClusterManager) handleNodeFailure(event *ReplicationEvent) {
	nodeID, ok := event.Data["node_id"].(string)
	if !ok {
		log.Printf("Invalid node_id in node_failed event")
		return
	}

	// Get the node info
	var nodeInfo *NodeInfo
	if nodeInfoData, ok := event.Data["node_info"].(map[string]interface{}); ok {
		// Convert to NodeInfo
		infoBytes, err := json.Marshal(nodeInfoData)
		if err == nil {
			var info NodeInfo
			if err := json.Unmarshal(infoBytes, &info); err == nil {
				nodeInfo = &info
			}
		}
	}

	// Notify node failure handlers
	for _, handler := range cm.nodeFailureHandlers {
		if err := handler(nodeID, nodeInfo); err != nil {
			log.Printf("Error in node failure handler: %v", err)
		}
	}
}

// handleConfigUpdate processes a configuration update event
func (cm *ClusterManager) handleConfigUpdate(event *ReplicationEvent) {
	// Implementation depends on what configurations can be updated
	log.Printf("Received config update event: %v", event)
}

// nodeMonitorLoop periodically checks the health of nodes
func (cm *ClusterManager) nodeMonitorLoop() {
	ticker := time.NewTicker(cm.heartbeatInterval * 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkNodeHealth()
		case <-cm.stopCh:
			return
		}
	}
}

// checkNodeHealth checks the health of all nodes
func (cm *ClusterManager) checkNodeHealth() {
	// Only the leader checks node health
	if !cm.isLeader {
		return
	}

	nodes, err := cm.store.ListNodes()
	if err != nil {
		log.Printf("Error listing nodes: %v", err)
		return
	}

	now := time.Now()
	failureTimeout := cm.heartbeatInterval * 3

	for _, node := range nodes {
		// Skip ourselves
		if node.ID == cm.localNode.ID {
			continue
		}

		// Check if the node has missed heartbeats
		if node.Status == NodeStatusActive && now.Sub(node.LastHeartbeat) > failureTimeout {
			// Mark the node as failed
			node.Status = NodeStatusFailed
			if err := cm.store.UpdateNode(node); err != nil {
				log.Printf("Error updating failed node %s: %v", node.ID, err)
				continue
			}

			// Publish a node failure event
			nodeInfoBytes, _ := json.Marshal(node)
			var nodeInfoMap map[string]interface{}
			_ = json.Unmarshal(nodeInfoBytes, &nodeInfoMap)

			event := &ReplicationEvent{
				ID:        fmt.Sprintf("node_failed-%s-%d", node.ID, time.Now().UnixNano()),
				Type:      "node_failed",
				Data:      map[string]interface{}{
					"node_id":   node.ID,
					"node_info": nodeInfoMap,
					"reason":    "heartbeat_timeout",
				},
				Timestamp: time.Now(),
				NodeID:    cm.localNode.ID,
				Sequence:  time.Now().UnixNano(),
				Term:      cm.localNode.LeadershipTerm,
			}

			if err := cm.store.PublishEvent(event); err != nil {
				log.Printf("Error publishing node failure event: %v", err)
			}

			log.Printf("Node %s marked as failed due to missed heartbeats", node.ID)
		}
	}

	// Update cluster info
	clusterInfo, err := cm.store.GetClusterInfo()
	if err == nil {
		var schedulerCount, workerCount, apiCount int
		for _, node := range nodes {
			if node.Status == NodeStatusActive {
				switch node.Type {
				case NodeTypeScheduler:
					schedulerCount++
				case NodeTypeWorker:
					workerCount++
				case NodeTypeAPI:
					apiCount++
				case NodeTypeAll:
					schedulerCount++
					workerCount++
					apiCount++
				}
			}
		}

		clusterInfo.NodeCount = len(nodes)
		clusterInfo.SchedulerCount = schedulerCount
		clusterInfo.WorkerCount = workerCount
		clusterInfo.APICount = apiCount
		clusterInfo.UpdateTime = time.Now()
		clusterInfo.Status = "active"

		if err := cm.store.UpdateClusterInfo(clusterInfo); err != nil {
			log.Printf("Error updating cluster info: %v", err)
		}
	}
}

// IsLeader returns whether this node is the leader
func (cm *ClusterManager) IsLeader() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.isLeader
}

// GetLeader returns the current leader's node ID
func (cm *ClusterManager) GetLeader() (string, error) {
	leaderID, err := cm.store.GetLock(cm.leadershipLock)
	if err != nil {
		return "", err
	}
	if leaderID == "" {
		return "", ErrLeaderNotElected
	}
	return leaderID, nil
}

// GetClusterInfo returns the current cluster information
func (cm *ClusterManager) GetClusterInfo() (*ClusterInfo, error) {
	return cm.store.GetClusterInfo()
}

// ListNodes returns a list of all nodes
func (cm *ClusterManager) ListNodes() ([]*NodeInfo, error) {
	return cm.store.ListNodes()
}

// GetNode returns information about a specific node
func (cm *ClusterManager) GetNode(nodeID string) (*NodeInfo, error) {
	return cm.store.GetNode(nodeID)
}

// UpdateNodeLoad updates the load information for this node
func (cm *ClusterManager) UpdateNodeLoad(load NodeLoad) error {
	cm.mu.Lock()
	cm.localNode.Load = load
	cm.localNode.Load.LastUpdated = time.Now()
	node := *cm.localNode
	cm.mu.Unlock()

	return cm.store.UpdateNode(&node)
}

// AddEventListener adds a listener for events
func (cm *ClusterManager) AddEventListener(eventType string, listener EventListener) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, ok := cm.listeners[eventType]; !ok {
		cm.listeners[eventType] = make([]EventListener, 0)
	}
	cm.listeners[eventType] = append(cm.listeners[eventType], listener)
}

// AddNodeFailureHandler adds a handler for node failures
func (cm *ClusterManager) AddNodeFailureHandler(handler NodeFailureHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.nodeFailureHandlers = append(cm.nodeFailureHandlers, handler)
}

// PublishEvent publishes an event
func (cm *ClusterManager) PublishEvent(eventType string, data map[string]interface{}) error {
	cm.mu.RLock()
	nodeID := cm.localNode.ID
	term := cm.localNode.LeadershipTerm
	cm.mu.RUnlock()

	event := &ReplicationEvent{
		ID:        fmt.Sprintf("%s-%s-%d", eventType, nodeID, time.Now().UnixNano()),
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Sequence:  time.Now().UnixNano(),
		Term:      term,
	}

	return cm.store.PublishEvent(event)
}

// TransferLeadership attempts to transfer leadership to another node
func (cm *ClusterManager) TransferLeadership(targetNodeID string) error {
	if !cm.IsLeader() {
		return errors.New("only the leader can transfer leadership")
	}

	// Check if the target node exists and is active
	targetNode, err := cm.store.GetNode(targetNodeID)
	if err != nil {
		return err
	}
	if targetNode.Status != NodeStatusActive {
		return ErrInvalidNodeState
	}

	// Publish a leadership transfer event
	event := &ReplicationEvent{
		ID:        fmt.Sprintf("leadership_transfer-%s-%d", cm.localNode.ID, time.Now().UnixNano()),
		Type:      "leadership_transfer",
		Data:      map[string]interface{}{
			"from_node": cm.localNode.ID,
			"to_node":   targetNodeID,
			"term":      cm.localNode.LeadershipTerm,
		},
		Timestamp: time.Now(),
		NodeID:    cm.localNode.ID,
		Sequence:  time.Now().UnixNano(),
		Term:      cm.localNode.LeadershipTerm,
	}

	if err := cm.store.PublishEvent(event); err != nil {
		return err
	}

	// Release the leadership lock
	if err := cm.store.ReleaseLock(cm.leadershipLock, cm.localNode.ID); err != nil {
		return err
	}

	// Update our role
	cm.mu.Lock()
	cm.isLeader = false
	cm.localNode.Role = NodeRoleFollower
	if err := cm.store.UpdateNode(cm.localNode); err != nil {
		cm.mu.Unlock()
		return err
	}
	cm.mu.Unlock()

	log.Printf("Transferred leadership from %s to %s", cm.localNode.ID, targetNodeID)
	return nil
}

// BackupClusterState creates a backup of the cluster state
func (cm *ClusterManager) BackupClusterState() (map[string]interface{}, error) {
	// In a real implementation, this would create a comprehensive backup
	// of all cluster state, including configuration, nodes, etc.
	clusterInfo, err := cm.store.GetClusterInfo()
	if err != nil {
		return nil, err
	}

	nodes, err := cm.store.ListNodes()
	if err != nil {
		return nil, err
	}

	events, err := cm.store.GetEvents(0, 100)
	if err != nil {
		return nil, err
	}

	backup := map[string]interface{}{
		"cluster_info": clusterInfo,
		"nodes":        nodes,
		"events":       events,
		"timestamp":    time.Now(),
		"version":      cm.localNode.Version,
	}

	return backup, nil
}

// RestoreClusterState restores a backup of the cluster state
func (cm *ClusterManager) RestoreClusterState(backup map[string]interface{}) error {
	// In a real implementation, this would restore a comprehensive backup
	// of all cluster state, including configuration, nodes, etc.
	// This is a simplified example

	// Restore cluster info
	if clusterInfoData, ok := backup["cluster_info"].(map[string]interface{}); ok {
		infoBytes, err := json.Marshal(clusterInfoData)
		if err != nil {
			return err
		}
		var clusterInfo ClusterInfo
		if err := json.Unmarshal(infoBytes, &clusterInfo); err != nil {
			return err
		}
		if err := cm.store.UpdateClusterInfo(&clusterInfo); err != nil {
			return err
		}
	}

	// Publish a restore event
	event := &ReplicationEvent{
		ID:        fmt.Sprintf("state_restored-%s-%d", cm.localNode.ID, time.Now().UnixNano()),
		Type:      "state_restored",
		Data:      map[string]interface{}{
			"node_id":   cm.localNode.ID,
			"timestamp": time.Now(),
		},
		Timestamp: time.Now(),
		NodeID:    cm.localNode.ID,
		Sequence:  time.Now().UnixNano(),
		Term:      cm.localNode.LeadershipTerm,
	}

	return cm.store.PublishEvent(event)
} 