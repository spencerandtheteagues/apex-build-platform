package startup

import (
	"testing"
	"time"
)

func TestSnapshotReadyAndDegradedStatus(t *testing.T) {
	registry := NewRegistry()
	registry.Register("database", TierCritical, "Waiting for database", nil)
	registry.Register("payments", TierOptional, "Waiting for payments", nil)

	starting := registry.Snapshot()
	if starting.Status != "starting" {
		t.Fatalf("expected starting status, got %q", starting.Status)
	}
	if starting.Ready {
		t.Fatal("expected registry to be not ready during startup")
	}

	registry.MarkReady("database", TierCritical, "Database connected", nil)
	registry.MarkDegraded("payments", TierOptional, "Stripe not configured", map[string]any{"enabled": false})
	registry.SetPhase(PhaseReady)

	summary := registry.Snapshot()
	if summary.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", summary.Status)
	}
	if !summary.Ready {
		t.Fatal("expected registry to be ready when critical services are ready")
	}
	if len(summary.DegradedFeatures) != 1 || summary.DegradedFeatures[0] != "payments" {
		t.Fatalf("unexpected degraded features: %#v", summary.DegradedFeatures)
	}
}

func TestSnapshotUnhealthyWhenCriticalServiceDegraded(t *testing.T) {
	registry := NewRegistry()
	registry.MarkDegraded("database", TierCritical, "Database unreachable", nil)
	registry.SetPhase(PhaseReady)

	summary := registry.Snapshot()
	if summary.Status != "unhealthy" {
		t.Fatalf("expected unhealthy status, got %q", summary.Status)
	}
	if summary.Ready {
		t.Fatal("expected registry to be not ready when a critical service is degraded")
	}
}

func TestApplyRuntimeServiceRecomputesDegradedFeatures(t *testing.T) {
	now := time.Now().UTC()
	summary := Summary{
		Phase:     PhaseReady,
		Status:    "healthy",
		Ready:     true,
		StartedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		Services: []Service{
			{Name: "primary_database", Tier: TierCritical, State: StateReady, Summary: "Database connected", UpdatedAt: now},
			{Name: "redis_cache", Tier: TierOptional, State: StateReady, Summary: "Redis cache connected", UpdatedAt: now},
		},
	}

	next := ApplyRuntimeService(summary, Service{
		Name:      "redis_cache",
		Tier:      TierOptional,
		State:     StateDegraded,
		Summary:   "Using in-memory cache fallback",
		UpdatedAt: now.Add(time.Second),
	})

	if next.Status != "degraded" {
		t.Fatalf("expected degraded status after runtime overlay, got %q", next.Status)
	}
	if !next.Ready {
		t.Fatal("expected readiness to stay true when only optional service is degraded")
	}
	if len(next.DegradedFeatures) != 1 || next.DegradedFeatures[0] != "redis_cache" {
		t.Fatalf("unexpected degraded features after overlay: %#v", next.DegradedFeatures)
	}
}
