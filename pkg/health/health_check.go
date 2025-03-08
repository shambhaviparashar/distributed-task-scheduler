package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	// StatusUp indicates that the component is healthy
	StatusUp Status = "up"
	
	// StatusDown indicates that the component is unhealthy
	StatusDown Status = "down"
	
	// StatusDegraded indicates that the component is operating with reduced functionality
	StatusDegraded Status = "degraded"
)

// CheckFunc is a function that performs a health check
type CheckFunc func(ctx context.Context) (Status, error)

// Component represents a system component that can be health-checked
type Component struct {
	Name        string
	Description string
	CheckFunc   CheckFunc
}

// Result represents the result of a health check
type Result struct {
	Status      Status    `json:"status"`
	Component   string    `json:"component"`
	Description string    `json:"description"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Checker is responsible for executing health checks and reporting results
type Checker struct {
	components []Component
	results    map[string]Result
	mu         sync.RWMutex
	checkFreq  time.Duration
}

// NewChecker creates a new health checker
func NewChecker(checkFrequency time.Duration) *Checker {
	return &Checker{
		components: []Component{},
		results:    make(map[string]Result),
		checkFreq:  checkFrequency,
	}
}

// AddCheck registers a new component for health checking
func (c *Checker) AddCheck(component Component) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.components = append(c.components, component)
	c.results[component.Name] = Result{
		Status:      StatusDown,
		Component:   component.Name,
		Description: component.Description,
		Timestamp:   time.Now(),
	}
}

// StartChecking begins periodic health checks
func (c *Checker) StartChecking(ctx context.Context) {
	ticker := time.NewTicker(c.checkFreq)
	defer ticker.Stop()
	
	// Run initial checks
	c.runChecks(ctx)
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Health checker stopped due to context cancellation")
			return
		case <-ticker.C:
			c.runChecks(ctx)
		}
	}
}

// runChecks executes all health checks
func (c *Checker) runChecks(ctx context.Context) {
	c.mu.RLock()
	checks := make([]Component, len(c.components))
	copy(checks, c.components)
	c.mu.RUnlock()
	
	for _, comp := range checks {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		status, err := comp.CheckFunc(checkCtx)
		cancel()
		
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		
		c.mu.Lock()
		c.results[comp.Name] = Result{
			Status:      status,
			Component:   comp.Name,
			Description: comp.Description,
			Error:       errStr,
			Timestamp:   time.Now(),
		}
		c.mu.Unlock()
	}
}

// GetResults returns the current health check results
func (c *Checker) GetResults() map[string]Result {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Create a copy to avoid races
	results := make(map[string]Result, len(c.results))
	for k, v := range c.results {
		results[k] = v
	}
	
	return results
}

// GetStatus returns the overall system status
func (c *Checker) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.results) == 0 {
		return StatusDown
	}
	
	hasDown := false
	for _, result := range c.results {
		if result.Status == StatusDown {
			hasDown = true
			break
		}
	}
	
	if hasDown {
		return StatusDegraded
	}
	
	return StatusUp
}

// HTTPHandler returns an HTTP handler for health check results
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := c.GetResults()
		status := c.GetStatus()
		
		response := struct {
			Status  Status            `json:"status"`
			Details map[string]Result `json:"details"`
		}{
			Status:  status,
			Details: results,
		}
		
		w.Header().Set("Content-Type", "application/json")
		
		// Set appropriate status code based on health
		if status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if status == StatusDegraded {
			w.WriteHeader(http.StatusOK) // Still 200 but with degraded status
		} else {
			w.WriteHeader(http.StatusOK)
		}
		
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding health check response: %v", err)
		}
	}
}

// LivenessHandler returns an HTTP handler for liveness probe
func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For liveness, we only check if the service is running at all
		// Kubernetes will restart the pod if this fails
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "alive")
	}
}

// ReadinessHandler returns an HTTP handler for readiness probe
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For readiness, we check if the service can accept requests
		status := c.GetStatus()
		if status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "not ready")
			return
		}
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ready")
	}
} 