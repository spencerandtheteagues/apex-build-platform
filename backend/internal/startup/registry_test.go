package startup

import "testing"

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
