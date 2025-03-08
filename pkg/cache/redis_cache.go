package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// CacheKey represents a key in the cache
type CacheKey string

// CacheTags represents a set of tags associated with a cached item
type CacheTags []string

// Item represents an item stored in the cache
type Item struct {
	Key        CacheKey
	Value      []byte
	Expiration time.Duration
	Tags       CacheTags
}

// Cache defines the interface for caching operations
type Cache interface {
	Get(ctx context.Context, key CacheKey) ([]byte, error)
	Set(ctx context.Context, item *Item) error
	Delete(ctx context.Context, key CacheKey) error
	DeleteByTag(ctx context.Context, tag string) error
	Exists(ctx context.Context, key CacheKey) (bool, error)
	Flush(ctx context.Context) error
}

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client      *redis.Client
	keyPrefix   string
	defaultTTL  time.Duration
	tagIndexKey string
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(client *redis.Client, keyPrefix string, defaultTTL time.Duration) *RedisCache {
	return &RedisCache{
		client:      client,
		keyPrefix:   keyPrefix,
		defaultTTL:  defaultTTL,
		tagIndexKey: fmt.Sprintf("%s:tag_index", keyPrefix),
	}
}

// prefixKey adds the key prefix to a cache key
func (rc *RedisCache) prefixKey(key CacheKey) string {
	return fmt.Sprintf("%s:%s", rc.keyPrefix, key)
}

// tagKey generates a key for a tag
func (rc *RedisCache) tagKey(tag string) string {
	return fmt.Sprintf("%s:tag:%s", rc.keyPrefix, tag)
}

// Get retrieves an item from the cache
func (rc *RedisCache) Get(ctx context.Context, key CacheKey) ([]byte, error) {
	val, err := rc.client.Get(ctx, rc.prefixKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Key doesn't exist
		}
		return nil, err
	}
	return val, nil
}

// Set stores an item in the cache
func (rc *RedisCache) Set(ctx context.Context, item *Item) error {
	if item == nil {
		return errors.New("cache item cannot be nil")
	}

	// Use default TTL if not specified
	expiration := item.Expiration
	if expiration == 0 {
		expiration = rc.defaultTTL
	}

	prefixedKey := rc.prefixKey(item.Key)

	// Use pipeline for atomicity
	pipe := rc.client.Pipeline()

	// Set the value with expiration
	pipe.Set(ctx, prefixedKey, item.Value, expiration)

	// Update tag indexes if tags are provided
	if len(item.Tags) > 0 {
		// Store key in each tag's set
		for _, tag := range item.Tags {
			tagKey := rc.tagKey(tag)
			pipe.SAdd(ctx, tagKey, prefixedKey)
			pipe.Expire(ctx, tagKey, expiration)
		}

		// Store tags for this key
		tagsJSON, err := json.Marshal(item.Tags)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		pipe.Set(ctx, fmt.Sprintf("%s:tags", prefixedKey), tagsJSON, expiration)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	return err
}

// Delete removes an item from the cache
func (rc *RedisCache) Delete(ctx context.Context, key CacheKey) error {
	prefixedKey := rc.prefixKey(key)

	// Get tags for this key if any
	tagsJSON, err := rc.client.Get(ctx, fmt.Sprintf("%s:tags", prefixedKey)).Bytes()
	if err != nil && err != redis.Nil {
		return err
	}

	pipe := rc.client.Pipeline()

	// Remove key from tag sets
	if err != redis.Nil && len(tagsJSON) > 0 {
		var tags []string
		if err := json.Unmarshal(tagsJSON, &tags); err == nil {
			for _, tag := range tags {
				pipe.SRem(ctx, rc.tagKey(tag), prefixedKey)
			}
		}
		pipe.Del(ctx, fmt.Sprintf("%s:tags", prefixedKey))
	}

	// Delete the key
	pipe.Del(ctx, prefixedKey)

	_, err = pipe.Exec(ctx)
	return err
}

// DeleteByTag removes all items with the specified tag
func (rc *RedisCache) DeleteByTag(ctx context.Context, tag string) error {
	tagKey := rc.tagKey(tag)

	// Get all keys with this tag
	keys, err := rc.client.SMembers(ctx, tagKey).Result()
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	// Use pipeline for efficiency
	pipe := rc.client.Pipeline()

	// Delete all keys
	for _, key := range keys {
		pipe.Del(ctx, key)
		pipe.Del(ctx, fmt.Sprintf("%s:tags", key))
	}

	// Delete the tag set
	pipe.Del(ctx, tagKey)

	_, err = pipe.Exec(ctx)
	return err
}

// Exists checks if a key exists in the cache
func (rc *RedisCache) Exists(ctx context.Context, key CacheKey) (bool, error) {
	exists, err := rc.client.Exists(ctx, rc.prefixKey(key)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// Flush removes all items from the cache matching the prefix
func (rc *RedisCache) Flush(ctx context.Context) error {
	pattern := fmt.Sprintf("%s:*", rc.keyPrefix)
	
	var cursor uint64
	var keysToDelete []string
	
	// Scan for keys with the prefix
	for {
		var keys []string
		var err error
		
		keys, cursor, err = rc.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		
		keysToDelete = append(keysToDelete, keys...)
		
		if cursor == 0 {
			break
		}
	}
	
	// Delete keys in batches
	const batchSize = 100
	for i := 0; i < len(keysToDelete); i += batchSize {
		end := i + batchSize
		if end > len(keysToDelete) {
			end = len(keysToDelete)
		}
		
		batch := keysToDelete[i:end]
		if len(batch) > 0 {
			if err := rc.client.Del(ctx, batch...).Err(); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// LuaScript enables efficient Redis operations using Lua scripts
type LuaScript struct {
	script *redis.Script
	cache  *RedisCache
}

// NewLuaScript creates a new Lua script
func (rc *RedisCache) NewLuaScript(script string) *LuaScript {
	return &LuaScript{
		script: redis.NewScript(script),
		cache:  rc,
	}
}

// Run executes the Lua script
func (ls *LuaScript) Run(ctx context.Context, keys []string, args ...interface{}) (interface{}, error) {
	return ls.script.Run(ctx, ls.cache.client, keys, args...).Result()
}

// PipelinedCache provides Redis pipelining for bulk operations
type PipelinedCache struct {
	cache    *RedisCache
	pipeline redis.Pipeliner
	ctx      context.Context
}

// StartPipeline starts a new pipeline
func (rc *RedisCache) StartPipeline(ctx context.Context) *PipelinedCache {
	return &PipelinedCache{
		cache:    rc,
		pipeline: rc.client.Pipeline(),
		ctx:      ctx,
	}
}

// Get queues a get operation in the pipeline
func (pc *PipelinedCache) Get(key CacheKey) *redis.StringCmd {
	return pc.pipeline.Get(pc.ctx, pc.cache.prefixKey(key))
}

// Set queues a set operation in the pipeline
func (pc *PipelinedCache) Set(item *Item) {
	expiration := item.Expiration
	if expiration == 0 {
		expiration = pc.cache.defaultTTL
	}
	
	prefixedKey := pc.cache.prefixKey(item.Key)
	
	// Set the value with expiration
	pc.pipeline.Set(pc.ctx, prefixedKey, item.Value, expiration)
	
	// Update tag indexes if tags are provided
	if len(item.Tags) > 0 {
		// Store key in each tag's set
		for _, tag := range item.Tags {
			tagKey := pc.cache.tagKey(tag)
			pc.pipeline.SAdd(pc.ctx, tagKey, prefixedKey)
			pc.pipeline.Expire(pc.ctx, tagKey, expiration)
		}
		
		// Store tags for this key
		tagsJSON, _ := json.Marshal(item.Tags)
		pc.pipeline.Set(pc.ctx, fmt.Sprintf("%s:tags", prefixedKey), tagsJSON, expiration)
	}
}

// Delete queues a delete operation in the pipeline
func (pc *PipelinedCache) Delete(key CacheKey) {
	prefixedKey := pc.cache.prefixKey(key)
	pc.pipeline.Del(pc.ctx, prefixedKey)
	pc.pipeline.Del(pc.ctx, fmt.Sprintf("%s:tags", prefixedKey))
}

// Exec executes the pipeline
func (pc *PipelinedCache) Exec() ([]redis.Cmder, error) {
	return pc.pipeline.Exec(pc.ctx)
}

// Discard discards the pipeline
func (pc *PipelinedCache) Discard() {
	pc.pipeline.Discard()
}

// RedisCacheConfig contains configuration options for Redis cache
type RedisCacheConfig struct {
	Addr         string
	Password     string
	DB           int
	KeyPrefix    string
	DefaultTTL   time.Duration
	PoolSize     int
	MinIdleConns int
}

// NewRedisCacheFromConfig creates a new Redis cache from configuration
func NewRedisCacheFromConfig(cfg *RedisCacheConfig) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
	
	return NewRedisCache(client, cfg.KeyPrefix, cfg.DefaultTTL)
} 