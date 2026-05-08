package ai

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

type providerRateLimitStore interface {
	Allow(ctx context.Context, provider AIProvider, limit int, window time.Duration) (bool, error)
	Close() error
}

type redisProviderRateLimitStore struct {
	client goredis.UniversalClient
}

func newProviderRateLimitStoreFromEnv() providerRateLimitStore {
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return nil
	}

	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		log.Printf("WARNING: provider shared rate limiter disabled - invalid REDIS_URL: %v", err)
		return nil
	}

	client := goredis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: provider shared rate limiter disabled - redis ping failed: %v", err)
		_ = client.Close()
		return nil
	}

	return &redisProviderRateLimitStore{client: client}
}

func (s *redisProviderRateLimitStore) Allow(ctx context.Context, provider AIProvider, limit int, window time.Duration) (bool, error) {
	if s == nil || s.client == nil || limit <= 0 {
		return true, nil
	}
	if window <= 0 {
		window = time.Minute
	}

	now := time.Now().UTC()
	windowStart := now.Truncate(window)
	key := providerRateLimitKey(provider, windowStart)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return true, err
	}
	if count == 1 {
		ttl := time.Until(windowStart.Add(window)) + time.Second
		if ttl < time.Second {
			ttl = time.Second
		}
		if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
			return true, err
		}
	}
	return count <= int64(limit), nil
}

func (s *redisProviderRateLimitStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func providerRateLimitKey(provider AIProvider, windowStart time.Time) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(provider))))
	return fmt.Sprintf("ai:provider_rate_limit:%d:%x", windowStart.UTC().Unix(), sum[:8])
}
