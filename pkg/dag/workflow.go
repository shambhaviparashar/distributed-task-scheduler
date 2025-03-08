package dag

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// WorkflowDefinition represents a reusable workflow template
type WorkflowDefinition struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Version     string                   `json:"version"`
	Tasks       map[string]TaskTemplate  `json:"tasks"`
	Edges       []EdgeTemplate           `json:"edges"`
	Parameters  map[string]ParameterSpec `json:"parameters"`
	CreatedAt   int64                    `json:"created_at"`
	UpdatedAt   int64                    `json:"updated_at"`
	Tags        []string                 `json:"tags"`
	Metadata    map[string]interface{}   `json:"metadata"`
}

// TaskTemplate defines a template for a task within a workflow
type TaskTemplate struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Command        string                 `json:"command"`
	ParameterRefs  map[string]string      `json:"parameter_refs"`  // Map of task param name to workflow param name
	ResourceReqs   ResourceRequirements   `json:"resource_requirements"`
	RetryPolicy    RetryPolicy            `json:"retry_policy"`
	Timeout        int64                  `json:"timeout_seconds"`
	Metadata       map[string]interface{} `json:"metadata"`
	Condition      string                 `json:"condition"`        // Expression string for conditional execution
	CheckpointConf CheckpointConfig       `json:"checkpoint_config"`
}

// EdgeTemplate defines a template for an edge within a workflow
type EdgeTemplate struct {
	From     string                 `json:"from"`
	To       string                 `json:"to"`
	Type     string                 `json:"type"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ParameterSpec defines a parameter specification
type ParameterSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        string      `json:"type"`        // "string", "int", "float", "bool", "array", "object"
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Constraints []string    `json:"constraints"` // Validation constraints
}

// ResourceRequirements defines the resource requirements for a task
type ResourceRequirements struct {
	CPU      string `json:"cpu"`      // CPU units (e.g., "0.5", "1")
	Memory   string `json:"memory"`   // Memory units (e.g., "512Mi", "1Gi")
	DiskSize string `json:"disk_size"` // Disk size (e.g., "1Gi")
	GPU      int    `json:"gpu"`      // Number of GPUs
}

// RetryPolicy defines the retry behavior for a task
type RetryPolicy struct {
	MaxRetries  int   `json:"max_retries"`
	InitialWait int64 `json:"initial_wait_seconds"`
	MaxWait     int64 `json:"max_wait_seconds"`
	Backoff     float64 `json:"backoff_multiplier"`
}

// CheckpointConfig defines checkpoint configuration for a task
type CheckpointConfig struct {
	Enabled       bool  `json:"enabled"`
	IntervalSecs  int64 `json:"interval_seconds"`
	MaxCheckpoints int  `json:"max_checkpoints"`
}

// WorkflowInstance represents an instance of a workflow being executed
type WorkflowInstance struct {
	ID                string                 `json:"id"`
	WorkflowDefID     string                 `json:"workflow_def_id"`
	WorkflowDefVersion string                `json:"workflow_def_version"`
	Name              string                 `json:"name"`
	Status            string                 `json:"status"` // pending, running, completed, failed, cancelled
	Parameters        map[string]interface{} `json:"parameters"`
	Tasks             map[string]string      `json:"tasks"`  // Map of workflow task ID to executor task ID
	Graph             *Graph                 `json:"-"`      // Internal DAG representation
	CreatedAt         int64                  `json:"created_at"`
	StartedAt         int64                  `json:"started_at"`
	CompletedAt       int64                  `json:"completed_at"`
	Metadata          map[string]interface{} `json:"metadata"`
	mu                sync.RWMutex           `json:"-"`
}

// NewWorkflowDefinition creates a new workflow definition
func NewWorkflowDefinition(id, name, version string) *WorkflowDefinition {
	return &WorkflowDefinition{
		ID:         id,
		Name:       name,
		Version:    version,
		Tasks:      make(map[string]TaskTemplate),
		Parameters: make(map[string]ParameterSpec),
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}
}

// AddTask adds a task to the workflow definition
func (wd *WorkflowDefinition) AddTask(id string, task TaskTemplate) error {
	if _, exists := wd.Tasks[id]; exists {
		return fmt.Errorf("task with ID %s already exists in workflow", id)
	}
	wd.Tasks[id] = task
	wd.UpdatedAt = time.Now().Unix()
	return nil
}

// AddEdge adds an edge to the workflow definition
func (wd *WorkflowDefinition) AddEdge(edge EdgeTemplate) error {
	if _, exists := wd.Tasks[edge.From]; !exists {
		return fmt.Errorf("source task %s does not exist", edge.From)
	}
	if _, exists := wd.Tasks[edge.To]; !exists {
		return fmt.Errorf("target task %s does not exist", edge.To)
	}
	
	// Check for duplicate edges
	for _, e := range wd.Edges {
		if e.From == edge.From && e.To == edge.To {
			return fmt.Errorf("edge from %s to %s already exists", edge.From, edge.To)
		}
	}
	
	wd.Edges = append(wd.Edges, edge)
	wd.UpdatedAt = time.Now().Unix()
	return nil
}

// AddParameter adds a parameter to the workflow definition
func (wd *WorkflowDefinition) AddParameter(param ParameterSpec) error {
	if _, exists := wd.Parameters[param.Name]; exists {
		return fmt.Errorf("parameter %s already exists", param.Name)
	}
	wd.Parameters[param.Name] = param
	wd.UpdatedAt = time.Now().Unix()
	return nil
}

// Validate validates the workflow definition
func (wd *WorkflowDefinition) Validate() error {
	// Check for empty ID or name
	if wd.ID == "" {
		return errors.New("workflow ID cannot be empty")
	}
	if wd.Name == "" {
		return errors.New("workflow name cannot be empty")
	}
	
	// Check for tasks
	if len(wd.Tasks) == 0 {
		return errors.New("workflow must have at least one task")
	}
	
	// Build a temporary graph to check for cycles
	graph := NewGraph()
	
	// Add nodes
	for id, task := range wd.Tasks {
		node := &Node{
			ID:   id,
			Name: task.Name,
		}
		if err := graph.AddNode(node); err != nil {
			return fmt.Errorf("failed to add node: %v", err)
		}
	}
	
	// Add edges
	for _, edge := range wd.Edges {
		e := &Edge{
			From: edge.From,
			To:   edge.To,
			Type: edge.Type,
		}
		if err := graph.AddEdge(e); err != nil {
			return fmt.Errorf("failed to add edge: %v", err)
		}
	}
	
	// Try to do a topological sort to check for cycles
	_, err := graph.TopologicalSort()
	if err != nil {
		return fmt.Errorf("workflow contains a cycle: %v", err)
	}
	
	// Validate parameter references
	for taskID, task := range wd.Tasks {
		for _, paramRef := range task.ParameterRefs {
			if _, exists := wd.Parameters[paramRef]; !exists {
				return fmt.Errorf("task %s references undefined parameter %s", taskID, paramRef)
			}
		}
		
		// Validate condition (simple validation)
		if task.Condition != "" {
			// In a real implementation, this would validate the condition syntax
			// For now, we just check that it's not empty when specified
			if len(task.Condition) == 0 {
				return fmt.Errorf("task %s has an empty condition", taskID)
			}
		}
	}
	
	return nil
}

// ToJSON serializes the workflow definition to JSON
func (wd *WorkflowDefinition) ToJSON() ([]byte, error) {
	return json.Marshal(wd)
}

// FromJSON deserializes a workflow definition from JSON
func WorkflowDefinitionFromJSON(data []byte) (*WorkflowDefinition, error) {
	var wd WorkflowDefinition
	err := json.Unmarshal(data, &wd)
	if err != nil {
		return nil, err
	}
	return &wd, nil
}

// InstantiateWorkflow creates a workflow instance from a workflow definition
func (wd *WorkflowDefinition) InstantiateWorkflow(id string, params map[string]interface{}) (*WorkflowInstance, error) {
	// Validate parameters
	for name, paramSpec := range wd.Parameters {
		if paramSpec.Required {
			if _, exists := params[name]; !exists {
				if paramSpec.Default == nil {
					return nil, fmt.Errorf("required parameter %s is missing", name)
				}
				// Use default value
				params[name] = paramSpec.Default
			}
		} else if _, exists := params[name]; !exists && paramSpec.Default != nil {
			// Use default value for optional parameter
			params[name] = paramSpec.Default
		}
	}
	
	// Create workflow instance
	instance := &WorkflowInstance{
		ID:                id,
		WorkflowDefID:     wd.ID,
		WorkflowDefVersion: wd.Version,
		Name:              wd.Name,
		Status:            "pending",
		Parameters:        params,
		Tasks:             make(map[string]string),
		Graph:             NewGraph(),
		CreatedAt:         time.Now().Unix(),
		Metadata:          make(map[string]interface{}),
	}
	
	// Add nodes to the graph
	for id, task := range wd.Tasks {
		// Create a node for each task
		node := &Node{
			ID:          id,
			Name:        task.Name,
			Description: task.Description,
			Status:      "pending",
			Metadata:    make(map[string]interface{}),
		}
		
		// Add resolved parameters to metadata
		resolvedParams := make(map[string]interface{})
		for paramName, paramRef := range task.ParameterRefs {
			if value, exists := params[paramRef]; exists {
				resolvedParams[paramName] = value
			}
		}
		
		// Add task properties to metadata
		node.Metadata["command"] = task.Command
		node.Metadata["timeout"] = task.Timeout
		node.Metadata["retry_policy"] = task.RetryPolicy
		node.Metadata["resource_requirements"] = task.ResourceReqs
		node.Metadata["parameters"] = resolvedParams
		node.Metadata["condition"] = task.Condition
		node.Metadata["checkpoint_config"] = task.CheckpointConf
		
		if err := instance.Graph.AddNode(node); err != nil {
			return nil, fmt.Errorf("failed to add node: %v", err)
		}
	}
	
	// Add edges to the graph
	for _, edge := range wd.Edges {
		e := &Edge{
			From:     edge.From,
			To:       edge.To,
			Type:     edge.Type,
			Metadata: edge.Metadata,
		}
		
		if err := instance.Graph.AddEdge(e); err != nil {
			return nil, fmt.Errorf("failed to add edge: %v", err)
		}
	}
	
	return instance, nil
}

// GetTasksToExecute returns tasks ready for execution
func (wi *WorkflowInstance) GetTasksToExecute() []*Node {
	wi.mu.RLock()
	defer wi.mu.RUnlock()
	
	if wi.Status != "running" {
		return nil
	}
	
	return wi.Graph.GetReadyNodes()
}

// UpdateTaskStatus updates the status of a task in the workflow
func (wi *WorkflowInstance) UpdateTaskStatus(taskID, status string, result interface{}) error {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	
	node, exists := wi.Graph.Nodes[taskID]
	if !exists {
		return fmt.Errorf("task %s does not exist in workflow", taskID)
	}
	
	// Update node status
	node.Status = status
	node.Result = result
	
	// Check if workflow is complete
	isComplete := true
	isFailed := false
	
	for _, node := range wi.Graph.Nodes {
		if node.Status != "completed" && node.Status != "skipped" && node.Status != "failed" {
			isComplete = false
			break
		}
		if node.Status == "failed" {
			isFailed = true
		}
	}
	
	// Update workflow status if all tasks are complete
	if isComplete {
		if isFailed {
			wi.Status = "failed"
		} else {
			wi.Status = "completed"
		}
		wi.CompletedAt = time.Now().Unix()
	}
	
	return nil
}

// StartWorkflow starts the workflow execution
func (wi *WorkflowInstance) StartWorkflow() error {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	
	if wi.Status != "pending" {
		return fmt.Errorf("workflow is already in %s state", wi.Status)
	}
	
	wi.Status = "running"
	wi.StartedAt = time.Now().Unix()
	return nil
}

// CancelWorkflow cancels the workflow execution
func (wi *WorkflowInstance) CancelWorkflow() error {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	
	if wi.Status == "completed" || wi.Status == "failed" {
		return fmt.Errorf("workflow is already in terminal state: %s", wi.Status)
	}
	
	wi.Status = "cancelled"
	wi.CompletedAt = time.Now().Unix()
	return nil
}

// ToJSON serializes the workflow instance to JSON
func (wi *WorkflowInstance) ToJSON() ([]byte, error) {
	wi.mu.RLock()
	defer wi.mu.RUnlock()
	
	// Create a copy without the mutex and graph
	instanceCopy := struct {
		ID                string                 `json:"id"`
		WorkflowDefID     string                 `json:"workflow_def_id"`
		WorkflowDefVersion string                `json:"workflow_def_version"`
		Name              string                 `json:"name"`
		Status            string                 `json:"status"`
		Parameters        map[string]interface{} `json:"parameters"`
		Tasks             map[string]string      `json:"tasks"`
		CreatedAt         int64                  `json:"created_at"`
		StartedAt         int64                  `json:"started_at"`
		CompletedAt       int64                  `json:"completed_at"`
		Metadata          map[string]interface{} `json:"metadata"`
		// Include task statuses
		TaskStatuses      map[string]string      `json:"task_statuses"`
	}{
		ID:                wi.ID,
		WorkflowDefID:     wi.WorkflowDefID,
		WorkflowDefVersion: wi.WorkflowDefVersion,
		Name:              wi.Name,
		Status:            wi.Status,
		Parameters:        wi.Parameters,
		Tasks:             wi.Tasks,
		CreatedAt:         wi.CreatedAt,
		StartedAt:         wi.StartedAt,
		CompletedAt:       wi.CompletedAt,
		Metadata:          wi.Metadata,
		TaskStatuses:      make(map[string]string),
	}
	
	// Add task statuses
	for id, node := range wi.Graph.Nodes {
		instanceCopy.TaskStatuses[id] = node.Status
	}
	
	return json.Marshal(instanceCopy)
}

// GetWorkflowDOT returns a DOT representation of the workflow
func (wi *WorkflowInstance) GetWorkflowDOT() (string, error) {
	wi.mu.RLock()
	defer wi.mu.RUnlock()
	
	opts := DefaultRenderOptions()
	opts.Format = FormatDOT
	opts.Title = wi.Name
	opts.IncludeMetadata = true
	
	return wi.Graph.RenderToString(opts)
}

// GetWorkflowJSON returns a JSON visualization of the workflow
func (wi *WorkflowInstance) GetWorkflowJSON() (string, error) {
	wi.mu.RLock()
	defer wi.mu.RUnlock()
	
	opts := DefaultRenderOptions()
	opts.Format = FormatJSON
	opts.Title = wi.Name
	opts.IncludeMetadata = true
	
	return wi.Graph.RenderToString(opts)
}

// CloneWorkflowDefinition creates a new version of a workflow definition
func CloneWorkflowDefinition(wd *WorkflowDefinition, newVersion string) *WorkflowDefinition {
	clone := &WorkflowDefinition{
		ID:          wd.ID,
		Name:        wd.Name,
		Description: wd.Description,
		Version:     newVersion,
		Tasks:       make(map[string]TaskTemplate),
		Parameters:  make(map[string]ParameterSpec),
		Edges:       make([]EdgeTemplate, len(wd.Edges)),
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		Tags:        make([]string, len(wd.Tags)),
		Metadata:    make(map[string]interface{}),
	}
	
	// Copy tasks
	for id, task := range wd.Tasks {
		paramRefs := make(map[string]string)
		for k, v := range task.ParameterRefs {
			paramRefs[k] = v
		}
		
		metadata := make(map[string]interface{})
		for k, v := range task.Metadata {
			metadata[k] = v
		}
		
		clone.Tasks[id] = TaskTemplate{
			Name:          task.Name,
			Description:   task.Description,
			Command:       task.Command,
			ParameterRefs: paramRefs,
			ResourceReqs:  task.ResourceReqs,
			RetryPolicy:   task.RetryPolicy,
			Timeout:       task.Timeout,
			Metadata:      metadata,
			Condition:     task.Condition,
			CheckpointConf: task.CheckpointConf,
		}
	}
	
	// Copy edges
	for i, edge := range wd.Edges {
		metadata := make(map[string]interface{})
		for k, v := range edge.Metadata {
			metadata[k] = v
		}
		
		clone.Edges[i] = EdgeTemplate{
			From:     edge.From,
			To:       edge.To,
			Type:     edge.Type,
			Metadata: metadata,
		}
	}
	
	// Copy parameters
	for name, param := range wd.Parameters {
		clone.Parameters[name] = param
	}
	
	// Copy tags
	copy(clone.Tags, wd.Tags)
	
	// Copy metadata
	for k, v := range wd.Metadata {
		clone.Metadata[k] = v
	}
	
	return clone
} 