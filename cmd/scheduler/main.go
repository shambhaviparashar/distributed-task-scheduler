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
	"github.com/yourusername/taskscheduler/internal/scheduler"
	"github.com/yourusername/taskscheduler/pkg/config"
	"github.com/yourusername/taskscheduler/pkg/coordination"
	"github.com/yourusername/taskscheduler/pkg/database"
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
	redisConfig := &redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	redisClient := redis.NewClient(redisConfig)
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

	// Generate a unique instance ID for this scheduler
	hostname, _ := os.Hostname()
	instanceID := hostname + "-" + uuid.New().String()

	// Create worker registry
	workerRegistry := coordination.NewWorkerRegistry(coordination.WorkerRegistryConfig{
		Client:     redisClient,
		InstanceID: instanceID,
		Tags:       []string{"scheduler"},
	})
	
	// Start the worker registry
	if err := workerRegistry.Start(); err != nil {
		log.Fatalf("Failed to start worker registry: %v", err)
	}
	defer workerRegistry.Stop()

	// Create fault-tolerant scheduler
	ftScheduler := scheduler.NewFaultTolerantScheduler(scheduler.FaultTolerantSchedulerConfig{
		Repo:              repo,
		Queue:             taskQueue,
		RedisClient:       redisClient,
		InstanceID:        instanceID,
		LockKey:           "scheduler:leader",
		LockDuration:      10 * time.Second,
		StaleTaskInterval: 60 * time.Second,
		WorkerRegistry:    workerRegistry,
	})

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler
	if err := ftScheduler.Start(ctx); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	log.Printf("Scheduler started successfully with instance ID: %s", instanceID)

	// Setup graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sig
	log.Println("Shutting down scheduler...")

	// Stop the scheduler
	ftScheduler.Stop()

	log.Println("Scheduler shut down properly")
} 