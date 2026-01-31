// Package bundler - Bundle caching implementation
package bundler

import (
	"container/list"
	"log"
	"sync"
	"time"
)

// BundleCache provides caching for bundled content with TTL and LRU eviction
type BundleCache struct {
	// items stores cached bundle results
	items map[string]*cacheItem
	// order maintains LRU order
	order *list.List
	// mu protects the cache
	mu sync.RWMutex
	// maxSize is the maximum number of items in the cache
	maxSize int
	// ttl is the time-to-live for cache entries
	ttl time.Duration
	// stats tracks cache statistics
	stats CacheStats
	// stopCleanup signals the cleanup goroutine to stop
	stopCleanup chan struct{}
}

// cacheItem represents a cached bundle result
type cacheItem struct {
	// key is the cache key
	key string
	// result is the cached bundle result
	result *BundleResult
	// createdAt is when the item was cached
	createdAt time.Time
	// lastAccess is when the item was last accessed
	lastAccess time.Time
	// accessCount is the number of times this item was accessed
	accessCount int
	// element is the list element for LRU tracking
	element *list.Element
	// size is the approximate size in bytes
	size int64
}

// CacheStats contains cache statistics
type CacheStats struct {
	// Hits is the number of cache hits
	Hits int64 `json:"hits"`
	// Misses is the number of cache misses
	Misses int64 `json:"misses"`
	// Evictions is the number of items evicted
	Evictions int64 `json:"evictions"`
	// Expirations is the number of items expired
	Expirations int64 `json:"expirations"`
	// CurrentSize is the current number of items
	CurrentSize int `json:"current_size"`
	// TotalBytesStored is the approximate total bytes stored
	TotalBytesStored int64 `json:"total_bytes_stored"`
}

// CacheConfig contains configuration for the bundle cache
type CacheConfig struct {
	// MaxSize is the maximum number of items (default: 100)
	MaxSize int
	// TTL is the time-to-live for entries (default: 10 minutes)
	TTL time.Duration
	// CleanupInterval is how often to run cleanup (default: 1 minute)
	CleanupInterval time.Duration
}

// DefaultCacheConfig returns the default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize:         100,
		TTL:             10 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
}

// NewBundleCache creates a new bundle cache with the given configuration
func NewBundleCache(config CacheConfig) *BundleCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 100
	}
	if config.TTL <= 0 {
		config.TTL = 10 * time.Minute
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 1 * time.Minute
	}

	cache := &BundleCache{
		items:       make(map[string]*cacheItem),
		order:       list.New(),
		maxSize:     config.MaxSize,
		ttl:         config.TTL,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup
	go cache.cleanupLoop(config.CleanupInterval)

	return cache
}

// Get retrieves a cached bundle result by key
func (c *BundleCache) Get(key string) *BundleResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		return nil
	}

	// Check if expired
	if time.Since(item.createdAt) > c.ttl {
		c.removeItem(item)
		c.stats.Misses++
		c.stats.Expirations++
		return nil
	}

	// Update access time and move to front (most recently used)
	item.lastAccess = time.Now()
	item.accessCount++
	c.order.MoveToFront(item.element)

	c.stats.Hits++
	return item.result
}

// Set stores a bundle result in the cache
func (c *BundleCache) Set(key string, result *BundleResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate approximate size
	size := int64(len(result.OutputJS) + len(result.OutputCSS) + len(result.SourceMap))

	// Check if key already exists
	if existing, exists := c.items[key]; exists {
		// Update existing item
		c.stats.TotalBytesStored -= existing.size
		existing.result = result
		existing.createdAt = time.Now()
		existing.lastAccess = time.Now()
		existing.size = size
		c.order.MoveToFront(existing.element)
		c.stats.TotalBytesStored += size
		return
	}

	// Evict if at capacity
	for len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	// Create new item
	item := &cacheItem{
		key:        key,
		result:     result,
		createdAt:  time.Now(),
		lastAccess: time.Now(),
		size:       size,
	}
	item.element = c.order.PushFront(item)
	c.items[key] = item

	c.stats.CurrentSize = len(c.items)
	c.stats.TotalBytesStored += size

	keyPrefix := key
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}
	log.Printf("[cache] Stored bundle %s (size: %d bytes, total items: %d)", keyPrefix, size, len(c.items))
}

// Invalidate removes a specific key from the cache
func (c *BundleCache) Invalidate(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	c.removeItem(item)
	return true
}

// InvalidateByPrefix removes all keys with the given prefix
func (c *BundleCache) InvalidateByPrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for key, item := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.removeItem(item)
			count++
		}
	}
	return count
}

// InvalidateByProjectID removes all cache entries for a project
func (c *BundleCache) InvalidateByProjectID(projectID uint) int {
	// Project ID is encoded in the cache key prefix
	prefix := computeProjectPrefix(projectID)
	return c.InvalidateByPrefix(prefix)
}

// Clear removes all items from the cache
func (c *BundleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
	c.order = list.New()
	c.stats.CurrentSize = 0
	c.stats.TotalBytesStored = 0

	log.Printf("[cache] Cache cleared")
}

// Stats returns the current cache statistics
func (c *BundleCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.CurrentSize = len(c.items)
	return stats
}

// Close stops the cleanup goroutine and clears the cache
func (c *BundleCache) Close() {
	close(c.stopCleanup)
	c.Clear()
}

// removeItem removes an item from the cache (must hold lock)
func (c *BundleCache) removeItem(item *cacheItem) {
	delete(c.items, item.key)
	c.order.Remove(item.element)
	c.stats.TotalBytesStored -= item.size
	c.stats.CurrentSize = len(c.items)
}

// evictOldest removes the least recently used item (must hold lock)
func (c *BundleCache) evictOldest() {
	oldest := c.order.Back()
	if oldest == nil {
		return
	}

	item := oldest.Value.(*cacheItem)
	c.removeItem(item)
	c.stats.Evictions++

	keyPrefix := item.key
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}
	log.Printf("[cache] Evicted bundle %s (age: %v)", keyPrefix, time.Since(item.createdAt))
}

// cleanupLoop periodically removes expired items
func (c *BundleCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCleanup:
			return
		case <-ticker.C:
			c.cleanupExpired()
		}
	}
}

// cleanupExpired removes all expired items
func (c *BundleCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expired := make([]*cacheItem, 0)

	for _, item := range c.items {
		if now.Sub(item.createdAt) > c.ttl {
			expired = append(expired, item)
		}
	}

	for _, item := range expired {
		c.removeItem(item)
		c.stats.Expirations++
	}

	if len(expired) > 0 {
		log.Printf("[cache] Cleaned up %d expired items", len(expired))
	}
}

// computeProjectPrefix generates a cache key prefix for a project
func computeProjectPrefix(projectID uint) string {
	// This should match how ComputeCacheKey generates keys
	// The first part of the key includes the project ID
	key := ComputeCacheKey(projectID, BundleConfig{}, "")
	if len(key) > 16 {
		return key[:16]
	}
	return key
}

// MemoryCacheStore provides a file-system backed cache for persistence
// This can be used as a fallback when memory cache is cleared
type FileCacheStore struct {
	// basePath is the base directory for cached files
	basePath string
	// mu protects the store
	mu sync.RWMutex
	// maxAge is the maximum age of cached files
	maxAge time.Duration
}

// NewFileCacheStore creates a new file-based cache store
func NewFileCacheStore(basePath string, maxAge time.Duration) (*FileCacheStore, error) {
	// Disabled for now - memory cache is sufficient for preview
	return nil, nil
}

// WarmCache pre-populates the cache with commonly used bundles
func (c *BundleCache) WarmCache(bundler *ESBuildBundler, projects []uint) {
	// This could be used to pre-warm the cache on server startup
	// For now, bundles are cached on first access
	log.Printf("[cache] Cache warming not implemented yet for %d projects", len(projects))
}

// CacheKey generates a cache key for debugging purposes
func CacheKey(projectID uint, entryPoint string, fileHash string) string {
	config := BundleConfig{EntryPoint: entryPoint}
	return ComputeCacheKey(projectID, config, fileHash)
}
