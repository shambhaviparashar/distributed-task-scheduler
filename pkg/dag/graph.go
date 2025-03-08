package dag

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Node represents a task node in the graph
type Node struct {
	ID          string                 // Unique identifier for the node
	Name        string                 // Human-readable name
	Description string                 // Optional description
	Metadata    map[string]interface{} // Additional metadata
	Status      string                 // Current status (pending, running, completed, failed, etc.)
	Result      interface{}            // Task execution result
}

// Edge represents a directed edge between two nodes
type Edge struct {
	From     string // Source node ID
	To       string // Target node ID
	Type     string // Edge type (e.g., "dependency", "conditional")
	Metadata map[string]interface{}
}

// Graph represents a directed acyclic graph of tasks
type Graph struct {
	Nodes     map[string]*Node   // Map of node ID to node
	Edges     map[string][]*Edge // Map of source node ID to outgoing edges
	InEdges   map[string][]*Edge // Map of target node ID to incoming edges
	mu        sync.RWMutex       // Mutex for thread safety
	Version   string             // Graph version
	CreatedAt int64              // Creation timestamp
	UpdatedAt int64              // Last updated timestamp
}

// NewGraph creates a new empty graph
func NewGraph() *Graph {
	return &Graph{
		Nodes:   make(map[string]*Node),
		Edges:   make(map[string][]*Edge),
		InEdges: make(map[string][]*Edge),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node.ID == "" {
		return errors.New("node ID cannot be empty")
	}

	if _, exists := g.Nodes[node.ID]; exists {
		return fmt.Errorf("node with ID %s already exists", node.ID)
	}

	g.Nodes[node.ID] = node
	return nil
}

// AddEdge adds a directed edge between two nodes
func (g *Graph) AddEdge(edge *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Verify that both nodes exist
	if _, exists := g.Nodes[edge.From]; !exists {
		return fmt.Errorf("source node %s does not exist", edge.From)
	}
	if _, exists := g.Nodes[edge.To]; !exists {
		return fmt.Errorf("target node %s does not exist", edge.To)
	}

	// Check for cycles
	if g.wouldCreateCycle(edge.From, edge.To) {
		return fmt.Errorf("adding edge from %s to %s would create a cycle", edge.From, edge.To)
	}

	// Add the edge to outgoing edges
	g.Edges[edge.From] = append(g.Edges[edge.From], edge)
	
	// Add to incoming edges
	g.InEdges[edge.To] = append(g.InEdges[edge.To], edge)

	return nil
}

// wouldCreateCycle checks if adding an edge would create a cycle
func (g *Graph) wouldCreateCycle(from, to string) bool {
	visited := make(map[string]bool)
	var dfs func(string) bool

	dfs = func(current string) bool {
		if current == from {
			return true // Found a cycle
		}

		visited[current] = true
		for _, edge := range g.Edges[current] {
			if !visited[edge.To] {
				if dfs(edge.To) {
					return true
				}
			}
		}
		return false
	}

	return dfs(to)
}

// GetChildren returns all direct successor nodes of a given node
func (g *Graph) GetChildren(nodeID string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.Nodes[nodeID]; !exists {
		return nil, fmt.Errorf("node %s does not exist", nodeID)
	}

	var children []*Node
	for _, edge := range g.Edges[nodeID] {
		children = append(children, g.Nodes[edge.To])
	}
	return children, nil
}

// GetParents returns all direct predecessor nodes of a given node
func (g *Graph) GetParents(nodeID string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.Nodes[nodeID]; !exists {
		return nil, fmt.Errorf("node %s does not exist", nodeID)
	}

	var parents []*Node
	for _, edge := range g.InEdges[nodeID] {
		parents = append(parents, g.Nodes[edge.From])
	}
	return parents, nil
}

// RemoveNode removes a node and all its edges from the graph
func (g *Graph) RemoveNode(nodeID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.Nodes[nodeID]; !exists {
		return fmt.Errorf("node %s does not exist", nodeID)
	}

	// Remove all edges that involve this node
	delete(g.Edges, nodeID)
	delete(g.InEdges, nodeID)

	// Remove any edges in other nodes that reference this node
	for id, edges := range g.Edges {
		var newEdges []*Edge
		for _, edge := range edges {
			if edge.To != nodeID {
				newEdges = append(newEdges, edge)
			}
		}
		g.Edges[id] = newEdges
	}

	for id, edges := range g.InEdges {
		var newEdges []*Edge
		for _, edge := range edges {
			if edge.From != nodeID {
				newEdges = append(newEdges, edge)
			}
		}
		g.InEdges[id] = newEdges
	}

	// Remove the node
	delete(g.Nodes, nodeID)
	return nil
}

// TopologicalSort returns a topological ordering of nodes
func (g *Graph) TopologicalSort() ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for id := range g.Nodes {
		inDegree[id] = 0
	}

	for _, edges := range g.Edges {
		for _, edge := range edges {
			inDegree[edge.To]++
		}
	}

	// Enqueue nodes with in-degree 0
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var result []*Node
	for len(queue) > 0 {
		// Dequeue a node
		id := queue[0]
		queue = queue[1:]
		
		// Add to result
		result = append(result, g.Nodes[id])

		// Reduce in-degree of neighbors
		for _, edge := range g.Edges[id] {
			inDegree[edge.To]--
			if inDegree[edge.To] == 0 {
				queue = append(queue, edge.To)
			}
		}
	}

	// Check if all nodes were visited
	if len(result) != len(g.Nodes) {
		return nil, errors.New("graph contains a cycle")
	}

	return result, nil
}

// GetReadyNodes returns all nodes that are ready to execute
// (all dependencies satisfied)
func (g *Graph) GetReadyNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ready := make([]*Node, 0)
	for id, node := range g.Nodes {
		if node.Status != "pending" {
			continue
		}

		allDepsComplete := true
		for _, edge := range g.InEdges[id] {
			parent := g.Nodes[edge.From]
			if parent.Status != "completed" {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			ready = append(ready, node)
		}
	}

	// Sort by priority (if available in metadata)
	sort.Slice(ready, func(i, j int) bool {
		iPriority := 0
		jPriority := 0

		if ready[i].Metadata != nil {
			if p, ok := ready[i].Metadata["priority"].(int); ok {
				iPriority = p
			}
		}

		if ready[j].Metadata != nil {
			if p, ok := ready[j].Metadata["priority"].(int); ok {
				jPriority = p
			}
		}

		return iPriority > jPriority // Higher priority first
	})

	return ready
}

// ConditionalExecution handles nodes that should execute conditionally
// based on parent results
func (g *Graph) ConditionalExecution(nodeID string) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.Nodes[nodeID]
	if !exists {
		return false, fmt.Errorf("node %s does not exist", nodeID)
	}

	// Check if this node has conditional execution logic
	condition, hasCondition := node.Metadata["condition"]
	if !hasCondition {
		return true, nil // No condition, should execute
	}

	// For each parent, check the condition
	shouldExecute := true
	for _, edge := range g.InEdges[nodeID] {
		if edge.Type == "conditional" {
			parent := g.Nodes[edge.From]
			
			// Simple condition handling - check if parent succeeded
			if condFunc, ok := condition.(func(interface{}) bool); ok {
				if !condFunc(parent.Result) {
					shouldExecute = false
					break
				}
			} else {
				// Default logic: Only execute if all parents completed successfully
				if parent.Status != "completed" {
					shouldExecute = false
					break
				}
			}
		}
	}

	return shouldExecute, nil
}

// Clone creates a deep copy of the graph
func (g *Graph) Clone() *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	newGraph := NewGraph()
	newGraph.Version = g.Version
	newGraph.CreatedAt = g.CreatedAt
	newGraph.UpdatedAt = g.UpdatedAt

	// Clone nodes
	for id, node := range g.Nodes {
		newNode := &Node{
			ID:          node.ID,
			Name:        node.Name,
			Description: node.Description,
			Status:      node.Status,
			Result:      node.Result,
		}
		
		if node.Metadata != nil {
			newNode.Metadata = make(map[string]interface{})
			for k, v := range node.Metadata {
				newNode.Metadata[k] = v
			}
		}
		
		newGraph.Nodes[id] = newNode
	}

	// Clone edges
	for from, edges := range g.Edges {
		for _, edge := range edges {
			newEdge := &Edge{
				From: edge.From,
				To:   edge.To,
				Type: edge.Type,
			}
			
			if edge.Metadata != nil {
				newEdge.Metadata = make(map[string]interface{})
				for k, v := range edge.Metadata {
					newEdge.Metadata[k] = v
				}
			}
			
			newGraph.Edges[from] = append(newGraph.Edges[from], newEdge)
		}
	}

	// Clone in-edges
	for to, edges := range g.InEdges {
		for _, edge := range edges {
			newEdge := &Edge{
				From: edge.From,
				To:   edge.To,
				Type: edge.Type,
			}
			
			if edge.Metadata != nil {
				newEdge.Metadata = make(map[string]interface{})
				for k, v := range edge.Metadata {
					newEdge.Metadata[k] = v
				}
			}
			
			newGraph.InEdges[to] = append(newGraph.InEdges[to], newEdge)
		}
	}

	return newGraph
}

// GetSubgraph returns a subgraph containing only the specified nodes and their connections
func (g *Graph) GetSubgraph(nodeIDs []string) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	newGraph := NewGraph()
	nodeSet := make(map[string]bool)
	
	for _, id := range nodeIDs {
		nodeSet[id] = true
	}

	// Add nodes
	for id := range nodeSet {
		if node, exists := g.Nodes[id]; exists {
			newNode := &Node{
				ID:          node.ID,
				Name:        node.Name,
				Description: node.Description,
				Status:      node.Status,
				Result:      node.Result,
			}
			
			if node.Metadata != nil {
				newNode.Metadata = make(map[string]interface{})
				for k, v := range node.Metadata {
					newNode.Metadata[k] = v
				}
			}
			
			newGraph.Nodes[id] = newNode
		}
	}

	// Add edges between nodes in the set
	for from, edges := range g.Edges {
		if nodeSet[from] {
			for _, edge := range edges {
				if nodeSet[edge.To] {
					newEdge := &Edge{
						From: edge.From,
						To:   edge.To,
						Type: edge.Type,
					}
					
					if edge.Metadata != nil {
						newEdge.Metadata = make(map[string]interface{})
						for k, v := range edge.Metadata {
							newEdge.Metadata[k] = v
						}
					}
					
					newGraph.Edges[from] = append(newGraph.Edges[from], newEdge)
					newGraph.InEdges[edge.To] = append(newGraph.InEdges[edge.To], newEdge)
				}
			}
		}
	}

	return newGraph
} 