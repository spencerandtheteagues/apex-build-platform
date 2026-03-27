package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRedisClient struct {
	pingErr error
}

func (s *stubRedisClient) Get(context.Context, string) (string, error) {
	return "", ErrRedisKeyNotFound
}
func (s *stubRedisClient) Set(context.Context, string, interface{}, time.Duration) error {
	return nil
}
func (s *stubRedisClient) Del(context.Context, ...string) error                { return nil }
func (s *stubRedisClient) Exists(context.Context, ...string) (int64, error)    { return 0, nil }
func (s *stubRedisClient) Expire(context.Context, string, time.Duration) error { return nil }
func (s *stubRedisClient) Keys(context.Context, string) ([]string, error)      { return nil, nil }
func (s *stubRedisClient) Pipeline() RedisPipeline                             { return &noopPipeline{} }
func (s *stubRedisClient) Close() error                                        { return nil }
func (s *stubRedisClient) Ping(context.Context) error                          { return s.pingErr }

func TestRedisCacheStatusFallsBackWhenRedisPingFails(t *testing.T) {
	cache := NewRedisCacheWithClient(&stubRedisClient{
		pingErr: errors.New("maintenance window"),
	}, DefaultCacheConfig())

	status := cache.Status()
	if status.Backend != "memory" {
		t.Fatalf("expected memory backend during redis outage, got %q", status.Backend)
	}
	if status.RedisConnected {
		t.Fatal("expected redis to report disconnected during ping failure")
	}
	if status.FallbackReason == "" {
		t.Fatal("expected fallback reason to be populated")
	}
}

func TestRedisCacheStatusReturnsToRedisAfterRecovery(t *testing.T) {
	client := &stubRedisClient{
		pingErr: errors.New("maintenance window"),
	}
	cache := NewRedisCacheWithClient(client, DefaultCacheConfig())

	_ = cache.Status()
	client.pingErr = nil

	status := cache.Status()
	if status.Backend != "redis" {
		t.Fatalf("expected redis backend after recovery, got %q", status.Backend)
	}
	if !status.RedisConnected {
		t.Fatal("expected redis to report connected after recovery")
	}
	if status.FallbackReason != "" {
		t.Fatalf("expected fallback reason to clear after recovery, got %q", status.FallbackReason)
	}
}
