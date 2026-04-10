package agents

import (
	"math"
	"os"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func withAdaptiveScorecardRouting(t *testing.T) {
	t.Helper()
	t.Setenv("APEX_ADAPTIVE_SCORECARD_ROUTING", "true")
}

func makeScorecardBuild(t *testing.T) *Build {
	t.Helper()
	build := &Build{
		ID:          "sc-test-" + t.Name(),
		ProviderMode: "platform",
	}
	ensureBuildOrchestrationStateLocked(build)
	return build
}

func TestObserveScorecardOutcomeUpdatesMatchingEntry(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{
		CompilePassed:     true,
		FirstPassVerified: true,
		LatencySeconds:    5.0,
	})

	build.mu.RLock()
	defer build.mu.RUnlock()
	var sc *ProviderScorecard
	for i := range build.SnapshotState.Orchestration.ProviderScorecards {
		s := &build.SnapshotState.Orchestration.ProviderScorecards[i]
		if s.Provider == ai.ProviderGPT4 && s.TaskShape == TaskShapeFrontendPatch {
			sc = s
			break
		}
	}
	if sc == nil {
		t.Fatal("expected GPT4+FrontendPatch scorecard to exist after observation")
	}
	if sc.SampleCount != 1 {
		t.Errorf("SampleCount = %d, want 1", sc.SampleCount)
	}
	if sc.FirstPassSampleCount != 1 || sc.FirstPassSuccessCount != 1 {
		t.Errorf("FirstPass counts = %d/%d, want 1/1", sc.FirstPassSuccessCount, sc.FirstPassSampleCount)
	}
	if sc.CompilePassRate != 1.0 {
		t.Errorf("CompilePassRate = %f, want 1.0", sc.CompilePassRate)
	}
	if math.Abs(sc.AverageLatencySeconds-5.0) > 0.001 {
		t.Errorf("AverageLatencySeconds = %f, want 5.0", sc.AverageLatencySeconds)
	}
}

func TestObserveScorecardOutcomeCreatesNewEntryForUnknownShape(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	// Claude+FrontendPatch is not in the default scorecards
	observeScorecardOutcome(build, ai.ProviderClaude, TaskShapeFrontendPatch, ScorecardOutcome{
		CompilePassed:     true,
		FirstPassVerified: false,
	})

	build.mu.RLock()
	defer build.mu.RUnlock()
	found := false
	for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
		if sc.Provider == ai.ProviderClaude && sc.TaskShape == TaskShapeFrontendPatch {
			found = true
			if sc.SampleCount != 1 {
				t.Errorf("new entry SampleCount = %d, want 1", sc.SampleCount)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected new Claude+FrontendPatch scorecard to be created")
	}
}

func TestObserveScorecardOutcomePreservesOtherEntries(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	before := len(build.SnapshotState.Orchestration.ProviderScorecards)

	// Observe GPT4+FrontendPatch (already in defaults)
	observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{CompilePassed: true})

	build.mu.RLock()
	after := len(build.SnapshotState.Orchestration.ProviderScorecards)
	build.mu.RUnlock()

	// No new entries should have been created since GPT4+FrontendPatch already exists
	if after != before {
		t.Errorf("scorecard count changed from %d to %d; expected no new entry for existing provider+shape", before, after)
	}
}

func TestObserveScorecardOutcomeRollingRateConvergence(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	// Observe 6 samples: 4 passing, 2 failing
	for i := 0; i < 4; i++ {
		observeScorecardOutcome(build, ai.ProviderGemini, TaskShapeVerification, ScorecardOutcome{CompilePassed: true})
	}
	for i := 0; i < 2; i++ {
		observeScorecardOutcome(build, ai.ProviderGemini, TaskShapeVerification, ScorecardOutcome{CompilePassed: false})
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
		if sc.Provider == ai.ProviderGemini && sc.TaskShape == TaskShapeVerification {
			if sc.SampleCount != 6 {
				t.Errorf("SampleCount = %d, want 6", sc.SampleCount)
			}
			wantRate := 4.0 / 6.0
			if math.Abs(sc.CompilePassRate-wantRate) > 0.01 {
				t.Errorf("CompilePassRate = %f, want ~%f (4/6)", sc.CompilePassRate, wantRate)
			}
			return
		}
	}
	t.Fatal("Gemini+Verification scorecard not found")
}

func TestObserveScorecardOutcomeActivatesSufficientSampleGate(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	build.mu.RLock()
	before := hasSufficientLiveScorecards(build.SnapshotState.Orchestration.ProviderScorecards)
	build.mu.RUnlock()
	if before {
		t.Fatal("expected routing to be inactive before any observations")
	}

	for i := 0; i < minLiveScorecardSamples; i++ {
		observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{CompilePassed: true, FirstPassVerified: true})
	}

	build.mu.RLock()
	after := hasSufficientLiveScorecards(build.SnapshotState.Orchestration.ProviderScorecards)
	build.mu.RUnlock()
	if !after {
		t.Errorf("expected routing to activate after %d observations", minLiveScorecardSamples)
	}
}

func TestObserveScorecardOutcomeNoopWhenFlagDisabled(t *testing.T) {
	os.Unsetenv("APEX_ADAPTIVE_SCORECARD_ROUTING")
	build := makeScorecardBuild(t)

	countBefore := func() int {
		build.mu.RLock()
		defer build.mu.RUnlock()
		total := 0
		for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
			total += sc.SampleCount
		}
		return total
	}

	before := countBefore()
	observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{CompilePassed: true})
	after := countBefore()

	if after != before {
		t.Errorf("expected no-op when flag disabled; sample count changed from %d to %d", before, after)
	}
}

func TestObserveScorecardRepairOutcomeUpdatesRepairRate(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	observeScorecardRepairOutcome(build, ai.ProviderGrok, TaskShapeRepair, true)
	observeScorecardRepairOutcome(build, ai.ProviderGrok, TaskShapeRepair, false)
	observeScorecardRepairOutcome(build, ai.ProviderGrok, TaskShapeRepair, true)

	build.mu.RLock()
	defer build.mu.RUnlock()
	for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
		if sc.Provider == ai.ProviderGrok && sc.TaskShape == TaskShapeRepair {
			if sc.RepairAttemptCount != 3 {
				t.Errorf("RepairAttemptCount = %d, want 3", sc.RepairAttemptCount)
			}
			if sc.RepairSuccessCount != 2 {
				t.Errorf("RepairSuccessCount = %d, want 2", sc.RepairSuccessCount)
			}
			wantRate := 2.0 / 3.0
			if math.Abs(sc.RepairSuccessRate-wantRate) > 0.01 {
				t.Errorf("RepairSuccessRate = %f, want ~%f", sc.RepairSuccessRate, wantRate)
			}
			return
		}
	}
	t.Fatal("Grok+Repair scorecard not found")
}

func TestObserveScorecardOutcomeTruncationUpdatesRate(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	// 1 truncated, 3 clean
	observeScorecardOutcome(build, ai.ProviderClaude, TaskShapeContract, ScorecardOutcome{TruncationOccurred: true})
	for i := 0; i < 3; i++ {
		observeScorecardOutcome(build, ai.ProviderClaude, TaskShapeContract, ScorecardOutcome{TruncationOccurred: false})
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
		if sc.Provider == ai.ProviderClaude && sc.TaskShape == TaskShapeContract {
			wantRate := 1.0 / 4.0
			if math.Abs(sc.TruncationRate-wantRate) > 0.01 {
				t.Errorf("TruncationRate = %f, want ~%f", sc.TruncationRate, wantRate)
			}
			if sc.TruncationEventCount != 1 {
				t.Errorf("TruncationEventCount = %d, want 1", sc.TruncationEventCount)
			}
			return
		}
	}
	t.Fatal("Claude+Contract scorecard not found")
}

func TestTaskGenerationLatency(t *testing.T) {
	now := time.Now()
	started := now.Add(-7 * time.Second)
	task := &Task{StartedAt: &started}

	latency := taskGenerationLatency(task, now)
	if math.Abs(latency-7.0) > 0.1 {
		t.Errorf("latency = %f, want ~7.0", latency)
	}
}

func TestTaskGenerationLatencyNilTask(t *testing.T) {
	if got := taskGenerationLatency(nil, time.Now()); got != 0 {
		t.Errorf("expected 0 for nil task, got %f", got)
	}
}

func TestObserveScorecardOutcomeFirstPassTracking(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	// attempt=0 + verify passed = first pass success
	observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeBackendPatch, ScorecardOutcome{
		CompilePassed:     true,
		FirstPassVerified: true,
	})
	// attempt=1 (retry) — not first pass
	observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeBackendPatch, ScorecardOutcome{
		CompilePassed:     true,
		FirstPassVerified: false,
	})

	build.mu.RLock()
	defer build.mu.RUnlock()
	for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
		if sc.Provider == ai.ProviderGPT4 && sc.TaskShape == TaskShapeBackendPatch {
			if sc.FirstPassSampleCount != 2 {
				t.Errorf("FirstPassSampleCount = %d, want 2", sc.FirstPassSampleCount)
			}
			if sc.FirstPassSuccessCount != 1 {
				t.Errorf("FirstPassSuccessCount = %d, want 1", sc.FirstPassSuccessCount)
			}
			wantRate := 0.5
			if math.Abs(sc.FirstPassVerificationRate-wantRate) > 0.01 {
				t.Errorf("FirstPassVerificationRate = %f, want 0.5", sc.FirstPassVerificationRate)
			}
			return
		}
	}
	t.Fatal("GPT4+BackendPatch scorecard not found")
}

func TestObserveScorecardOutcomeSkipsNilBuild(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	// Should not panic on nil build
	observeScorecardOutcome(nil, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{CompilePassed: true})
}

func TestObserveScorecardRoutingSelectsBetterProvider(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := makeScorecardBuild(t)

	// Seed GPT4+Frontend with poor performance (3 samples, 0% first pass)
	for i := 0; i < 3; i++ {
		observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{
			CompilePassed:     false,
			FirstPassVerified: false,
		})
	}
	// Seed Gemini+Frontend with excellent performance (3 samples, 100% first pass)
	for i := 0; i < 3; i++ {
		observeScorecardOutcome(build, ai.ProviderGemini, TaskShapeFrontendPatch, ScorecardOutcome{
			CompilePassed:     true,
			FirstPassVerified: true,
		})
	}

	build.mu.RLock()
	scorecards := append([]ProviderScorecard(nil), build.SnapshotState.Orchestration.ProviderScorecards...)
	build.mu.RUnlock()

	if !hasSufficientLiveScorecards(scorecards) {
		t.Fatal("expected sufficient live scorecards after 6 observations")
	}

	available := []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGemini}
	winner := selectProviderByScorecard(build, RoleFrontend, TaskShapeFrontendPatch, available, scorecards)
	if winner != ai.ProviderGemini {
		t.Errorf("expected Gemini to be selected (better observed performance), got %q", winner)
	}
}

func TestObserveScorecardOutcomeConcurrentSafe(t *testing.T) {
	withAdaptiveScorecardRouting(t)
	build := &Build{
		ID:           "sc-concurrent",
		ProviderMode: "platform",
	}
	ensureBuildOrchestrationStateLocked(build)

	// Run observations from multiple goroutines concurrently
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			observeScorecardOutcome(build, ai.ProviderGPT4, TaskShapeFrontendPatch, ScorecardOutcome{
				CompilePassed:     true,
				FirstPassVerified: true,
				LatencySeconds:    3.0,
			})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	for _, sc := range build.SnapshotState.Orchestration.ProviderScorecards {
		if sc.Provider == ai.ProviderGPT4 && sc.TaskShape == TaskShapeFrontendPatch {
			if sc.SampleCount != 10 {
				t.Errorf("expected 10 samples after concurrent observations, got %d", sc.SampleCount)
			}
			return
		}
	}
	t.Fatal("GPT4+FrontendPatch scorecard not found after concurrent writes")
}
