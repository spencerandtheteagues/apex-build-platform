package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubSQLPinger struct {
	remainingFailures int
}

func (s *stubSQLPinger) PingContext(context.Context) error {
	if s.remainingFailures > 0 {
		s.remainingFailures--
		return errors.New("temporary database outage")
	}
	return nil
}

func TestWaitForDatabasePingRetriesUntilSuccess(t *testing.T) {
	t.Setenv("DB_PING_TIMEOUT", "100ms")

	pinger := &stubSQLPinger{remainingFailures: 2}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := waitForDatabasePing(ctx, pinger, 10*time.Millisecond); err != nil {
		t.Fatalf("expected retry loop to recover, got %v", err)
	}
	if pinger.remainingFailures != 0 {
		t.Fatalf("expected all planned failures to be consumed, got %d remaining", pinger.remainingFailures)
	}
}

func TestWaitForDatabasePingReturnsDeadlineExceededAfterRetries(t *testing.T) {
	t.Setenv("DB_PING_TIMEOUT", "20ms")

	pinger := &stubSQLPinger{remainingFailures: 100}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := waitForDatabasePing(ctx, pinger, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected retry loop to fail when deadline expires")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected deadline-related error, got %v", err)
	}
}
