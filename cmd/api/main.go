package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/yourusername/taskscheduler/internal/api"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/pkg/config"
	"github.com/yourusername/taskscheduler/pkg/database"
	"github.com/yourusername/taskscheduler/pkg/metrics"
)

func main() {
	// Load configuration
	cfg := config.LoadFromEnv()

	// Connect to database
	dbConfig := &database.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		Database: cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	}

	db, err := database.NewConnection(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close(db)

	// Create repository
	repo := repository.NewPostgresTaskRepository(db)

	// Initialize metrics
	metricsInstance := metrics.GetInstance()
	
	// Register custom metrics if needed
	metricsInstance.RegisterCustomCounter(
		"api_requests_custom_total",
		"Total count of API requests by endpoint",
		[]string{"endpoint", "method"},
	)

	// Create and start API server
	addr := fmt.Sprintf(":%d", cfg.APIPort)
	server := api.NewServer(repo, addr)

	// Start metrics server on a different port if needed
	if cfg.MetricsPort > 0 && cfg.MetricsPort != cfg.APIPort {
		metricsAddr := fmt.Sprintf(":%d", cfg.MetricsPort)
		metricsServer := &http.Server{
			Addr:    metricsAddr,
			Handler: metricsInstance.Handler(),
		}
		
		go func() {
			log.Printf("Metrics server starting on %s", metricsAddr)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Metrics server failed: %v", err)
			}
		}()
	}

	// Graceful shutdown handling
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("API server started on port %d", cfg.APIPort)

	// Wait for termination signal
	<-done
	log.Println("Server is shutting down...")

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
} 