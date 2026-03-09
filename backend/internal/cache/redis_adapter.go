package cache

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

// goRedisAdapter wraps a go-redis UniversalClient to satisfy the cache.RedisClient interface.
type goRedisAdapter struct {
	client goredis.UniversalClient
}

func (a *goRedisAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := a.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", fmt.Errorf("cache: key not found: %s", key)
	}
	return val, err
}

func (a *goRedisAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return a.client.Set(ctx, key, value, ttl).Err()
}

func (a *goRedisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.client.Del(ctx, keys...).Err()
}

func (a *goRedisAdapter) Exists(ctx context.Context, keys ...string) (int64, error) {
	return a.client.Exists(ctx, keys...).Result()
}

func (a *goRedisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.client.Expire(ctx, key, ttl).Err()
}

func (a *goRedisAdapter) Keys(ctx context.Context, pattern string) ([]string, error) {
	return a.client.Keys(ctx, pattern).Result()
}

func (a *goRedisAdapter) Pipeline() RedisPipeline {
	// Pipeline is defined in the interface but not used internally.
	// Return a no-op implementation to satisfy the interface.
	return &noopPipeline{}
}

func (a *goRedisAdapter) Close() error {
	return a.client.Close()
}

// noopPipeline satisfies RedisPipeline but performs no batching.
// The cache package defines the interface but never calls Pipeline in practice.
type noopPipeline struct{}

func (p *noopPipeline) Get(_ context.Context, _ string) *StringCmd {
	return &StringCmd{err: fmt.Errorf("pipeline not supported")}
}
func (p *noopPipeline) Set(_ context.Context, _ string, _ interface{}, _ time.Duration) *StatusCmd {
	return &StatusCmd{err: fmt.Errorf("pipeline not supported")}
}
func (p *noopPipeline) Exec(_ context.Context) ([]Cmder, error) {
	return nil, fmt.Errorf("pipeline not supported")
}

// NewRedisCacheFromURL creates a RedisCache backed by a real Redis connection.
// Falls back to in-memory cache if the URL is empty or the connection fails.
func NewRedisCacheFromURL(redisURL string, config *CacheConfig) *RedisCache {
	if config == nil {
		config = DefaultCacheConfig()
	}
	if redisURL == "" {
		return newMemoryCache(config, false, "REDIS_URL not set")
	}

	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		// Invalid URL — fall back to in-memory
		return newMemoryCache(config, true, fmt.Sprintf("invalid REDIS_URL: %v", err))
	}

	client := goredis.NewClient(opts)

	// Verify connectivity; fall back to in-memory on failure
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return newMemoryCache(config, true, fmt.Sprintf("redis ping failed: %v", err))
	}

	return NewRedisCacheWithClient(&goRedisAdapter{client: client}, config)
}
