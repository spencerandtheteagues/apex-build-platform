// Package cache - Redis client adapter for go-redis/redis v9
// Implements the RedisClient interface using the go-redis library
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// GoRedisAdapter wraps a go-redis client to implement our RedisClient interface
type GoRedisAdapter struct {
	client *redis.Client
}

// NewGoRedisClient creates a new Redis client from a URL and returns an adapter
// URL format: redis://[:password@]host:port[/db]
// or: rediss://[:password@]host:port[/db] for TLS
func NewGoRedisClient(redisURL string) (*GoRedisAdapter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &GoRedisAdapter{client: client}, nil
}

// NewGoRedisClientWithOptions creates a Redis client with custom options
func NewGoRedisClientWithOptions(opts *redis.Options) (*GoRedisAdapter, error) {
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &GoRedisAdapter{client: client}, nil
}

// Get retrieves a value from Redis
func (a *GoRedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}

// Set stores a value in Redis with TTL
func (a *GoRedisAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return a.client.Set(ctx, key, value, ttl).Err()
}

// Del deletes one or more keys from Redis
func (a *GoRedisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist in Redis
func (a *GoRedisAdapter) Exists(ctx context.Context, keys ...string) (int64, error) {
	return a.client.Exists(ctx, keys...).Result()
}

// Expire sets a TTL on a key
func (a *GoRedisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.client.Expire(ctx, key, ttl).Err()
}

// Keys returns all keys matching a pattern
func (a *GoRedisAdapter) Keys(ctx context.Context, pattern string) ([]string, error) {
	return a.client.Keys(ctx, pattern).Result()
}

// Pipeline returns a new Redis pipeline
func (a *GoRedisAdapter) Pipeline() RedisPipeline {
	return &GoRedisPipeline{pipe: a.client.Pipeline()}
}

// Close closes the Redis connection
func (a *GoRedisAdapter) Close() error {
	return a.client.Close()
}

// Ping tests the Redis connection
func (a *GoRedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

// GoRedisPipeline wraps a go-redis pipeline
type GoRedisPipeline struct {
	pipe redis.Pipeliner
}

// Get adds a GET command to the pipeline
func (p *GoRedisPipeline) Get(ctx context.Context, key string) *StringCmd {
	cmd := p.pipe.Get(ctx, key)
	return &StringCmd{val: cmd.Val(), err: cmd.Err()}
}

// Set adds a SET command to the pipeline
func (p *GoRedisPipeline) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *StatusCmd {
	cmd := p.pipe.Set(ctx, key, value, ttl)
	return &StatusCmd{err: cmd.Err()}
}

// Exec executes all commands in the pipeline
func (p *GoRedisPipeline) Exec(ctx context.Context) ([]Cmder, error) {
	cmds, err := p.pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make([]Cmder, len(cmds))
	for i, cmd := range cmds {
		result[i] = cmd
	}
	return result, nil
}

// NewRedisCacheFromURL creates a RedisCache with a connection to the specified Redis URL
// Falls back to in-memory cache if connection fails
func NewRedisCacheFromURL(redisURL string, config *CacheConfig) (*RedisCache, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	adapter, err := NewGoRedisClient(redisURL)
	if err != nil {
		return nil, err
	}

	return NewRedisCacheWithClient(adapter, config), nil
}
