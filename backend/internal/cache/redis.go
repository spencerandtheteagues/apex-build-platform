// Package cache provides Redis-based caching for APEX.BUILD
// Implements caching for project listings, user sessions, and file listings
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RedisCache provides a Redis-compatible caching layer
// Falls back to in-memory cache when Redis is unavailable
type RedisCache struct {
	// In-memory fallback cache
	memCache    map[string]*cacheEntry
	memMu       sync.RWMutex

	// Redis connection (nil if not available)
	redisClient RedisClient

	// Configuration
	defaultTTL time.Duration
	maxMemSize int

	// Stats
	hits   int64
	misses int64
	statsMu sync.RWMutex
}

// RedisClient interface for Redis operations
// Can be implemented with go-redis or similar library
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Keys(ctx context.Context, pattern string) ([]string, error)
	Pipeline() RedisPipeline
	Close() error
}

// RedisPipeline interface for batched operations
type RedisPipeline interface {
	Get(ctx context.Context, key string) *StringCmd
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *StatusCmd
	Exec(ctx context.Context) ([]Cmder, error)
}

// Cmd interfaces for pipeline results
type Cmder interface{}
type StringCmd struct {
	val string
	err error
}
type StatusCmd struct {
	err error
}

func (c *StringCmd) Val() string { return c.val }
func (c *StringCmd) Err() error  { return c.err }
func (c *StatusCmd) Err() error  { return c.err }

// cacheEntry represents a cached item with expiration
type cacheEntry struct {
	Value     []byte
	ExpiresAt time.Time
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	// Redis connection URL (redis://host:port/db)
	RedisURL string

	// Default TTL for cached items
	DefaultTTL time.Duration

	// Maximum in-memory cache size (number of items)
	MaxMemoryItems int

	// Project cache TTL (30s as specified)
	ProjectCacheTTL time.Duration

	// Session cache TTL
	SessionCacheTTL time.Duration

	// File listing cache TTL
	FileCacheTTL time.Duration
}

// DefaultCacheConfig returns the default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL:      30 * time.Second,
		MaxMemoryItems:  10000,
		ProjectCacheTTL: 30 * time.Second,
		SessionCacheTTL: 15 * time.Minute,
		FileCacheTTL:    60 * time.Second,
	}
}

// NewRedisCache creates a new cache instance
func NewRedisCache(config *CacheConfig) *RedisCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &RedisCache{
		memCache:   make(map[string]*cacheEntry),
		defaultTTL: config.DefaultTTL,
		maxMemSize: config.MaxMemoryItems,
	}

	// Start cleanup goroutine for expired entries
	go cache.cleanupLoop()

	return cache
}

// NewRedisCacheWithClient creates a cache with an existing Redis client
func NewRedisCacheWithClient(client RedisClient, config *CacheConfig) *RedisCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &RedisCache{
		memCache:    make(map[string]*cacheEntry),
		redisClient: client,
		defaultTTL:  config.DefaultTTL,
		maxMemSize:  config.MaxMemoryItems,
	}

	go cache.cleanupLoop()

	return cache
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	// Try Redis first if available
	if c.redisClient != nil {
		val, err := c.redisClient.Get(ctx, key)
		if err == nil {
			c.recordHit()
			return []byte(val), nil
		}
	}

	// Fall back to memory cache
	c.memMu.RLock()
	entry, exists := c.memCache[key]
	c.memMu.RUnlock()

	if !exists {
		c.recordMiss()
		return nil, ErrCacheMiss
	}

	if time.Now().After(entry.ExpiresAt) {
		c.memMu.Lock()
		delete(c.memCache, key)
		c.memMu.Unlock()
		c.recordMiss()
		return nil, ErrCacheMiss
	}

	c.recordHit()
	return entry.Value, nil
}

// Set stores a value in cache with TTL
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	// Store in Redis if available
	if c.redisClient != nil {
		if err := c.redisClient.Set(ctx, key, string(value), ttl); err == nil {
			return nil
		}
		// Fall through to memory cache on Redis error
	}

	// Store in memory cache
	c.memMu.Lock()
	defer c.memMu.Unlock()

	// Evict if at capacity
	if len(c.memCache) >= c.maxMemSize {
		c.evictOldest()
	}

	c.memCache[key] = &cacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a key from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if c.redisClient != nil {
		c.redisClient.Del(ctx, key)
	}

	c.memMu.Lock()
	delete(c.memCache, key)
	c.memMu.Unlock()

	return nil
}

// DeletePattern removes all keys matching a pattern
func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	if c.redisClient != nil {
		keys, err := c.redisClient.Keys(ctx, pattern)
		if err == nil && len(keys) > 0 {
			c.redisClient.Del(ctx, keys...)
		}
	}

	// For memory cache, we need to iterate
	c.memMu.Lock()
	defer c.memMu.Unlock()

	for key := range c.memCache {
		if matchPattern(pattern, key) {
			delete(c.memCache, key)
		}
	}

	return nil
}

// GetJSON retrieves and unmarshals a JSON value
func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// SetJSON marshals and stores a JSON value
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, ttl)
}

// GetOrSet retrieves from cache or calls the loader function
func (c *RedisCache) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	// Try cache first
	data, err := c.Get(ctx, key)
	if err == nil {
		return data, nil
	}

	// Load data
	data, err = loader()
	if err != nil {
		return nil, err
	}

	// Store in cache (ignore errors)
	c.Set(ctx, key, data, ttl)

	return data, nil
}

// GetOrSetJSON is GetOrSet for JSON values
func (c *RedisCache) GetOrSetJSON(ctx context.Context, key string, ttl time.Duration, dest interface{}, loader func() (interface{}, error)) error {
	// Try cache first
	if err := c.GetJSON(ctx, key, dest); err == nil {
		return nil
	}

	// Load data
	value, err := loader()
	if err != nil {
		return err
	}

	// Store in cache
	if err := c.SetJSON(ctx, key, value, ttl); err != nil {
		return err
	}

	// Marshal/unmarshal to copy to dest
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Stats returns cache statistics
func (c *RedisCache) Stats() CacheStats {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()

	c.memMu.RLock()
	memSize := len(c.memCache)
	c.memMu.RUnlock()

	total := c.hits + c.misses
	hitRatio := float64(0)
	if total > 0 {
		hitRatio = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Hits:       c.hits,
		Misses:     c.misses,
		HitRatio:   hitRatio,
		MemorySize: memSize,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	HitRatio   float64 `json:"hit_ratio"`
	MemorySize int     `json:"memory_size"`
}

// Close closes the cache and releases resources
func (c *RedisCache) Close() error {
	if c.redisClient != nil {
		return c.redisClient.Close()
	}
	return nil
}

// Internal methods

func (c *RedisCache) recordHit() {
	c.statsMu.Lock()
	c.hits++
	c.statsMu.Unlock()
}

func (c *RedisCache) recordMiss() {
	c.statsMu.Lock()
	c.misses++
	c.statsMu.Unlock()
}

func (c *RedisCache) evictOldest() {
	// Simple eviction: remove 10% of entries or first expired
	toEvict := c.maxMemSize / 10
	if toEvict < 1 {
		toEvict = 1
	}

	now := time.Now()
	evicted := 0

	for key, entry := range c.memCache {
		if evicted >= toEvict {
			break
		}
		if now.After(entry.ExpiresAt) {
			delete(c.memCache, key)
			evicted++
		}
	}

	// If we didn't evict enough expired entries, evict oldest
	if evicted < toEvict {
		for key := range c.memCache {
			if evicted >= toEvict {
				break
			}
			delete(c.memCache, key)
			evicted++
		}
	}
}

func (c *RedisCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *RedisCache) cleanup() {
	c.memMu.Lock()
	defer c.memMu.Unlock()

	now := time.Now()
	for key, entry := range c.memCache {
		if now.After(entry.ExpiresAt) {
			delete(c.memCache, key)
		}
	}
}

// matchPattern provides simple glob-style matching for cache keys
func matchPattern(pattern, key string) bool {
	// Simple implementation - supports * at end
	if len(pattern) == 0 {
		return len(key) == 0
	}

	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}

	return pattern == key
}

// Cache key builders

// ProjectCacheKey returns the cache key for a project listing
func ProjectCacheKey(userID uint, page, limit int) string {
	return fmt.Sprintf("projects:user:%d:page:%d:limit:%d", userID, page, limit)
}

// ProjectDetailCacheKey returns the cache key for a single project
func ProjectDetailCacheKey(projectID uint) string {
	return fmt.Sprintf("project:%d", projectID)
}

// UserSessionCacheKey returns the cache key for a user session
func UserSessionCacheKey(userID uint) string {
	return fmt.Sprintf("session:user:%d", userID)
}

// FileListCacheKey returns the cache key for file listings
func FileListCacheKey(projectID uint) string {
	return fmt.Sprintf("files:project:%d", projectID)
}

// UserProjectsPattern returns the pattern for all user's project cache entries
func UserProjectsPattern(userID uint) string {
	return fmt.Sprintf("projects:user:%d:*", userID)
}

// Errors
var ErrCacheMiss = fmt.Errorf("cache miss")
