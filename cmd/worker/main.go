package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/yourusername/taskscheduler/internal/repository"
	"github.com/yourusername/taskscheduler/internal/worker"
	"github.com/yourusername/taskscheduler/pkg/config"
	"github.com/yourusername/taskscheduler/pkg/database"
	"github.com/yourusername/taskscheduler/pkg/metrics"
	"github.com/yourusername/taskscheduler/pkg/queue"
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

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	// Verify Redis connection
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create task queue
	taskQueue := queue.NewRedisQueue(&queue.RedisConfig{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}, "task_queue")
	defer taskQueue.Close()

	// Create repository
	repo := repository.NewPostgresTaskRepository(db)

	// Initialize metrics
	metrics.GetInstance()

	// Generate a unique instance ID for this worker
	hostname, _ := os.Hostname()
	instanceID := hostname + "-" + uuid.New().String()

	// Create and start worker
	ftWorker := worker.NewFaultTolerantWorker(worker.FaultTolerantWorkerConfig{
		Repo:                repo,
		Queue:               taskQueue,
		RedisClient:         redisClient,
		InstanceID:          instanceID,
		Capacity:            cfg.WorkerCount,
		ShutdownGracePeriod: 30 * time.Second,
	})
	ftWorker.Start()

	log.Printf("Worker started with instance ID: %s and capacity: %d", instanceID, cfg.WorkerCount)

	// Setup graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sig
	log.Println("Shutting down worker...")

	// Stop the worker - this will wait for in-progress tasks to complete
	ftWorker.Stop()

	log.Println("Worker shut down properly")
} 