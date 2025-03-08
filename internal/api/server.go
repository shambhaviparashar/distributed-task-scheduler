package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/health"
	"github.com/yourusername/taskscheduler/pkg/metrics"
)

// Server represents the API server
type Server struct {
	router     chi.Router
	server     *http.Server
	repo       repository.TaskRepository
	healthChecker *health.Checker
	metrics    *metrics.Metrics
}

// NewServer creates a new API server
func NewServer(repo repository.TaskRepository, addr string) *Server {
	r := chi.NewRouter()
	
	// Get metrics singleton
	metricsInstance := metrics.GetInstance()
	
	// Create a health checker
	healthChecker := health.NewChecker(30 * time.Second)
	
	server := &Server{
		router: r,
		server: &http.Server{
			Addr:    addr,
			Handler: r,
		},
		repo: repo,
		healthChecker: healthChecker,
		metrics: metricsInstance,
	}
	
	// Setup middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(metricsInstance.APIMiddleware)
	
	// Setup CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	
	// Add health checks
	server.registerHealthChecks()
	
	// Register handlers
	server.registerHandlers()
	
	return server
}

// registerHandlers registers all the API handlers
func (s *Server) registerHandlers() {
	// Health check
	s.router.Get("/health", s.healthChecker.HTTPHandler())
	s.router.Get("/health/live", s.healthChecker.LivenessHandler())
	s.router.Get("/health/ready", s.healthChecker.ReadinessHandler())
	
	// Metrics endpoint
	s.router.Handle("/metrics", s.metrics.Handler())
	
	// System info endpoint
	s.router.Get("/api/system/info", s.getSystemInfo)
	
	// Register task handlers
	taskHandler := NewTaskHandler(s.repo)
	taskHandler.RegisterRoutes(s.router)
}

// registerHealthChecks sets up health checks for the API server
func (s *Server) registerHealthChecks() {
	// Add a database health check
	s.healthChecker.AddCheck(health.Component{
		Name:        "database",
		Description: "Checks if the database is reachable",
		CheckFunc: func(ctx context.Context) (health.Status, error) {
			// A simple query to check database connectivity
			_, err := s.repo.GetTasksByStatus(ctx, "running", 1)
			if err != nil {
				return health.StatusDown, err
			}
			return health.StatusUp, nil
		},
	})
	
	// Add a self-check (API server)
	s.healthChecker.AddCheck(health.Component{
		Name:        "api",
		Description: "Checks if the API server is functioning",
		CheckFunc: func(ctx context.Context) (health.Status, error) {
			return health.StatusUp, nil // Self-test always passes if we're running
		},
	})
	
	// Start periodic health checks
	go s.healthChecker.StartChecking(context.Background())
}

// getSystemInfo returns information about the system
func (s *Server) getSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"version":   "1.0.0", // Would come from version package in real app
		"hostname":  getHostname(),
		"timestamp": time.Now(),
		"status":    "ok",
		"health":    s.healthChecker.GetStatus(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// getHostname returns the hostname or a fallback if not available
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown-host"
	}
	return hostname
}

// Start starts the API server
func (s *Server) Start() error {
	log.Printf("API server starting on %s\n", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the API server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("API server shutting down...")
	return s.server.Shutdown(ctx)
} 