// Package db provides Redis client setup for APEX.BUILD
// Supports connection pooling, Sentinel/Cluster, and health checks
package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	// Standard connection
	URL      string // redis://host:port/db or rediss://host:port/db for TLS
	Host     string
	Port     int
	Password string
	DB       int

	// Connection pool settings
	PoolSize        int
	MinIdleConns    int
	MaxConnAge      time.Duration
	PoolTimeout     time.Duration
	IdleTimeout     time.Duration
	IdleCheckFreq   time.Duration

	// Timeouts
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Sentinel configuration (for high availability)
	SentinelAddrs    []string
	SentinelMaster   string
	SentinelPassword string

	// Cluster configuration
	ClusterAddrs []string
}

// DefaultRedisConfig returns sensible defaults for Redis configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:           "localhost",
		Port:           6379,
		DB:             0,
		PoolSize:       100,
		MinIdleConns:   10,
		MaxConnAge:     0, // No max age
		PoolTimeout:    4 * time.Second,
		IdleTimeout:    5 * time.Minute,
		IdleCheckFreq:  1 * time.Minute,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
	}
}

// RedisConfigFromEnv creates Redis config from environment variables
func RedisConfigFromEnv() *RedisConfig {
	config := DefaultRedisConfig()

	// Primary Redis URL (takes precedence)
	if url := os.Getenv("REDIS_URL"); url != "" {
		config.URL = url
	}

	// Individual settings
	if host := os.Getenv("REDIS_HOST"); host != "" {
		config.Host = host
	}
	if port := os.Getenv("REDIS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Password = password
	}
	if db := os.Getenv("REDIS_DB"); db != "" {
		if d, err := strconv.Atoi(db); err == nil {
			config.DB = d
		}
	}

	// Pool settings
	if poolSize := os.Getenv("REDIS_POOL_SIZE"); poolSize != "" {
		if ps, err := strconv.Atoi(poolSize); err == nil {
			config.PoolSize = ps
		}
	}
	if minIdle := os.Getenv("REDIS_MIN_IDLE_CONNS"); minIdle != "" {
		if mi, err := strconv.Atoi(minIdle); err == nil {
			config.MinIdleConns = mi
		}
	}

	// Sentinel configuration
	if sentinelAddrs := os.Getenv("REDIS_SENTINEL_ADDRS"); sentinelAddrs != "" {
		config.SentinelAddrs = strings.Split(sentinelAddrs, ",")
	}
	if sentinelMaster := os.Getenv("REDIS_SENTINEL_MASTER"); sentinelMaster != "" {
		config.SentinelMaster = sentinelMaster
	}
	if sentinelPassword := os.Getenv("REDIS_SENTINEL_PASSWORD"); sentinelPassword != "" {
		config.SentinelPassword = sentinelPassword
	}

	// Cluster configuration
	if clusterAddrs := os.Getenv("REDIS_CLUSTER_ADDRS"); clusterAddrs != "" {
		config.ClusterAddrs = strings.Split(clusterAddrs, ",")
	}

	return config
}

// RedisClient wraps the go-redis client with health checks and convenience methods
type RedisClient struct {
	client      redis.UniversalClient
	isCluster   bool
	isSentinel  bool
	config      *RedisConfig
	healthCheck chan struct{}
}

// NewRedisClient creates a new Redis client based on configuration
func NewRedisClient(config *RedisConfig) (*RedisClient, error) {
	if config == nil {
		config = RedisConfigFromEnv()
	}

	rc := &RedisClient{
		config:      config,
		healthCheck: make(chan struct{}),
	}

	var err error

	// Priority: Cluster > Sentinel > Standard
	if len(config.ClusterAddrs) > 0 {
		rc.client, err = rc.createClusterClient(config)
		rc.isCluster = true
	} else if len(config.SentinelAddrs) > 0 && config.SentinelMaster != "" {
		rc.client, err = rc.createSentinelClient(config)
		rc.isSentinel = true
	} else {
		rc.client, err = rc.createStandardClient(config)
	}

	if err != nil {
		return nil, err
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rc.client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Start health check goroutine
	go rc.runHealthCheck()

	log.Printf("Redis client connected successfully (cluster=%v, sentinel=%v)", rc.isCluster, rc.isSentinel)
	return rc, nil
}

// createStandardClient creates a standard Redis client
func (rc *RedisClient) createStandardClient(config *RedisConfig) (redis.UniversalClient, error) {
	opts := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxConnAge:   config.MaxConnAge,
		PoolTimeout:  config.PoolTimeout,
		IdleTimeout:  config.IdleTimeout,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	// Parse URL if provided (overrides individual settings)
	if config.URL != "" {
		parsedOpts, err := redis.ParseURL(config.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid Redis URL: %w", err)
		}
		// Merge pool settings
		parsedOpts.PoolSize = config.PoolSize
		parsedOpts.MinIdleConns = config.MinIdleConns
		parsedOpts.MaxConnAge = config.MaxConnAge
		parsedOpts.PoolTimeout = config.PoolTimeout
		parsedOpts.IdleTimeout = config.IdleTimeout
		parsedOpts.DialTimeout = config.DialTimeout
		parsedOpts.ReadTimeout = config.ReadTimeout
		parsedOpts.WriteTimeout = config.WriteTimeout
		opts = parsedOpts
	}

	return redis.NewClient(opts), nil
}

// createSentinelClient creates a Redis Sentinel client for high availability
func (rc *RedisClient) createSentinelClient(config *RedisConfig) (redis.UniversalClient, error) {
	return redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:       config.SentinelMaster,
		SentinelAddrs:    config.SentinelAddrs,
		SentinelPassword: config.SentinelPassword,
		Password:         config.Password,
		DB:               config.DB,
		PoolSize:         config.PoolSize,
		MinIdleConns:     config.MinIdleConns,
		MaxConnAge:       config.MaxConnAge,
		PoolTimeout:      config.PoolTimeout,
		IdleTimeout:      config.IdleTimeout,
		DialTimeout:      config.DialTimeout,
		ReadTimeout:      config.ReadTimeout,
		WriteTimeout:     config.WriteTimeout,
	}), nil
}

// createClusterClient creates a Redis Cluster client
func (rc *RedisClient) createClusterClient(config *RedisConfig) (redis.UniversalClient, error) {
	return redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        config.ClusterAddrs,
		Password:     config.Password,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxConnAge:   config.MaxConnAge,
		PoolTimeout:  config.PoolTimeout,
		IdleTimeout:  config.IdleTimeout,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}), nil
}

// runHealthCheck periodically checks Redis connection health
func (rc *RedisClient) runHealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := rc.client.Ping(ctx).Err(); err != nil {
				log.Printf("Redis health check failed: %v", err)
			}
			cancel()
		case <-rc.healthCheck:
			return
		}
	}
}

// Client returns the underlying Redis client
func (rc *RedisClient) Client() redis.UniversalClient {
	return rc.client
}

// Ping tests the Redis connection
func (rc *RedisClient) Ping(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

// Health returns a detailed health status
func (rc *RedisClient) Health(ctx context.Context) map[string]interface{} {
	status := map[string]interface{}{
		"connected": false,
		"type":      "standard",
		"latency":   "unknown",
	}

	if rc.isCluster {
		status["type"] = "cluster"
	} else if rc.isSentinel {
		status["type"] = "sentinel"
	}

	start := time.Now()
	if err := rc.client.Ping(ctx).Err(); err != nil {
		status["error"] = err.Error()
		return status
	}
	latency := time.Since(start)

	status["connected"] = true
	status["latency"] = latency.String()

	// Get pool stats for standard client
	if !rc.isCluster {
		stats := rc.client.PoolStats()
		status["pool"] = map[string]interface{}{
			"hits":       stats.Hits,
			"misses":     stats.Misses,
			"timeouts":   stats.Timeouts,
			"total_conns": stats.TotalConns,
			"idle_conns":  stats.IdleConns,
			"stale_conns": stats.StaleConns,
		}
	}

	return status
}

// Close closes the Redis connection
func (rc *RedisClient) Close() error {
	close(rc.healthCheck)
	return rc.client.Close()
}

// Convenience methods for common operations

// Get retrieves a value from Redis
func (rc *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return rc.client.Get(ctx, key).Result()
}

// Set stores a value in Redis with optional TTL
func (rc *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

// Del deletes keys from Redis
func (rc *RedisClient) Del(ctx context.Context, keys ...string) error {
	return rc.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (rc *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return rc.client.Exists(ctx, keys...).Result()
}

// Expire sets TTL on a key
func (rc *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return rc.client.Expire(ctx, key, ttl).Err()
}

// Incr increments a counter
func (rc *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return rc.client.Incr(ctx, key).Result()
}

// IncrBy increments a counter by a specific amount
func (rc *RedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return rc.client.IncrBy(ctx, key, value).Result()
}

// TTL gets the remaining TTL of a key
func (rc *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return rc.client.TTL(ctx, key).Result()
}

// Eval executes a Lua script
func (rc *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	return rc.client.Eval(ctx, script, keys, args...)
}

// EvalSha executes a preloaded Lua script by SHA
func (rc *RedisClient) EvalSha(ctx context.Context, sha string, keys []string, args ...interface{}) *redis.Cmd {
	return rc.client.EvalSha(ctx, sha, keys, args...)
}

// ScriptLoad loads a Lua script into Redis
func (rc *RedisClient) ScriptLoad(ctx context.Context, script string) (string, error) {
	return rc.client.ScriptLoad(ctx, script).Result()
}

// Pipeline creates a pipeline for batched operations
func (rc *RedisClient) Pipeline() redis.Pipeliner {
	return rc.client.Pipeline()
}

// TxPipeline creates a transactional pipeline
func (rc *RedisClient) TxPipeline() redis.Pipeliner {
	return rc.client.TxPipeline()
}

// Keys returns all keys matching a pattern (use sparingly in production)
func (rc *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return rc.client.Keys(ctx, pattern).Result()
}

// Scan iterates over keys matching a pattern (preferred over Keys in production)
func (rc *RedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	return rc.client.Scan(ctx, cursor, match, count).Result()
}

// HSet sets hash fields
func (rc *RedisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	return rc.client.HSet(ctx, key, values...).Err()
}

// HGet gets a hash field
func (rc *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return rc.client.HGet(ctx, key, field).Result()
}

// HGetAll gets all hash fields
func (rc *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return rc.client.HGetAll(ctx, key).Result()
}

// ZAdd adds members to a sorted set
func (rc *RedisClient) ZAdd(ctx context.Context, key string, members ...*redis.Z) error {
	return rc.client.ZAdd(ctx, key, members...).Err()
}

// ZRangeByScore gets members from a sorted set by score range
func (rc *RedisClient) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	return rc.client.ZRangeByScore(ctx, key, opt).Result()
}

// ZRemRangeByScore removes members from a sorted set by score range
func (rc *RedisClient) ZRemRangeByScore(ctx context.Context, key string, min, max string) error {
	return rc.client.ZRemRangeByScore(ctx, key, min, max).Err()
}

// ZCard returns the number of members in a sorted set
func (rc *RedisClient) ZCard(ctx context.Context, key string) (int64, error) {
	return rc.client.ZCard(ctx, key).Result()
}

// Global Redis client instance
var globalRedisClient *RedisClient

// InitGlobalRedis initializes the global Redis client
func InitGlobalRedis(config *RedisConfig) error {
	client, err := NewRedisClient(config)
	if err != nil {
		return err
	}
	globalRedisClient = client
	return nil
}

// GetGlobalRedis returns the global Redis client (may be nil if not initialized)
func GetGlobalRedis() *RedisClient {
	return globalRedisClient
}

// CloseGlobalRedis closes the global Redis client
func CloseGlobalRedis() error {
	if globalRedisClient != nil {
		return globalRedisClient.Close()
	}
	return nil
}
