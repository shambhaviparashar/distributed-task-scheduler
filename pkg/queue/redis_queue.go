package queue

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

const defaultQueueName = "task_queue"

// ErrQueueEmpty is returned when the queue has no tasks
var ErrQueueEmpty = errors.New("queue is empty")

// RedisQueue implements TaskQueue using Redis
type RedisQueue struct {
	client    *redis.Client
	queueName string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewRedisQueue creates a new Redis-backed task queue
func NewRedisQueue(config *RedisConfig, queueName string) *RedisQueue {
	if queueName == "" {
		queueName = defaultQueueName
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	return &RedisQueue{
		client:    client,
		queueName: queueName,
	}
}

// Push adds a task to the queue
func (q *RedisQueue) Push(ctx context.Context, taskID string) error {
	return q.client.RPush(ctx, q.queueName, taskID).Err()
}

// Pop retrieves a task from the queue with blocking until timeout
func (q *RedisQueue) Pop(ctx context.Context, timeout time.Duration) (string, error) {
	// Use BLPOP for blocking pop with timeout
	result, err := q.client.BLPop(ctx, timeout, q.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrQueueEmpty
		}
		return "", err
	}

	// BLPOP returns [queueName, element], so we want the second element
	if len(result) < 2 {
		return "", ErrQueueEmpty
	}

	return result[1], nil
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
} 