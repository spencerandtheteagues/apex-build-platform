package agents

import (
	"os"
	"strings"
	"time"

	"apex-build/internal/ai"
)

// ScorecardOutcome captures the observed quality signals from a single AI generation
// attempt. These are used to incrementally update the in-build ProviderScorecard for
// the (provider, task_shape) pair so that hasSufficientLiveScorecards returns true
// after minLiveScorecardSamples tasks complete, activating scorecard-driven routing.
type ScorecardOutcome struct {
	CompilePassed      bool
	FirstPassVerified  bool // passed deterministic verify without requiring a retry
	TruncationOccurred bool
	LatencySeconds     float64
	CostUSD            float64
}

// observeScorecardOutcome records a single task generation outcome into the build's
// in-memory ProviderScorecards. It is gated by APEX_ADAPTIVE_SCORECARD_ROUTING.
//
// Thread-safety: acquires build.mu.Lock internally.
func observeScorecardOutcome(build *Build, provider ai.AIProvider, shape TaskShape, outcome ScorecardOutcome) {
	if !adaptiveScorecardRoutingEnabled() {
		return
	}
	if build == nil || provider == "" || shape == "" {
		return
	}

	build.mu.Lock()
	defer build.mu.Unlock()

	orchestration := ensureBuildOrchestrationStateLocked(build)
	if orchestration == nil {
		return
	}

	sc := findOrInitScorecard(orchestration, provider, shape)

	// Increment total sample count.
	sc.SampleCount++
	n := float64(sc.SampleCount)

	// Compile pass rate — rolling average.
	obs := boolToFloat(outcome.CompilePassed)
	sc.CompilePassRate = rollingAverage(sc.CompilePassRate, n, obs)

	// Truncation rate — rolling average.
	obs = boolToFloat(outcome.TruncationOccurred)
	if outcome.TruncationOccurred {
		sc.TruncationEventCount++
	}
	sc.TruncationRate = rollingAverage(sc.TruncationRate, n, obs)

	// First-pass verification rate — tracked via counters for precision.
	sc.FirstPassSampleCount++
	if outcome.FirstPassVerified {
		sc.FirstPassSuccessCount++
	}
	if sc.FirstPassSampleCount > 0 {
		sc.FirstPassVerificationRate = float64(sc.FirstPassSuccessCount) / float64(sc.FirstPassSampleCount)
	}

	// Latency — running average over observed samples.
	if outcome.LatencySeconds > 0 {
		sc.LatencySampleCount++
		lc := float64(sc.LatencySampleCount)
		sc.AverageLatencySeconds = rollingAverage(sc.AverageLatencySeconds, lc, outcome.LatencySeconds)
	}

	// Cost per success — only update when the generation succeeded (compile passed).
	if outcome.CostUSD > 0 && outcome.CompilePassed {
		sc.CostSampleCount++
		cc := float64(sc.CostSampleCount)
		sc.AverageCostPerSuccess = rollingAverage(sc.AverageCostPerSuccess, cc, outcome.CostUSD)
	}
}

// observeScorecardRepairOutcome records whether a repair task succeeded for a given
// provider/shape. This updates RepairSuccessRate without touching other fields.
func observeScorecardRepairOutcome(build *Build, provider ai.AIProvider, shape TaskShape, repairSucceeded bool) {
	if !adaptiveScorecardRoutingEnabled() {
		return
	}
	if build == nil || provider == "" || shape == "" {
		return
	}

	build.mu.Lock()
	defer build.mu.Unlock()

	orchestration := ensureBuildOrchestrationStateLocked(build)
	if orchestration == nil {
		return
	}

	sc := findOrInitScorecard(orchestration, provider, shape)

	sc.RepairAttemptCount++
	if repairSucceeded {
		sc.RepairSuccessCount++
	}
	if sc.RepairAttemptCount > 0 {
		sc.RepairSuccessRate = float64(sc.RepairSuccessCount) / float64(sc.RepairAttemptCount)
	}
}

// taskGenerationLatency computes the elapsed seconds since the task was started.
func taskGenerationLatency(task *Task, now time.Time) float64 {
	if task == nil || task.StartedAt == nil {
		return 0
	}
	elapsed := now.Sub(*task.StartedAt)
	if elapsed < 0 {
		return 0
	}
	return elapsed.Seconds()
}

// findOrInitScorecard returns a pointer to the matching ProviderScorecard or appends
// a new zero-value entry if none exists for (provider, shape). The returned pointer
// is always valid and points into orchestration.ProviderScorecards.
//
// Requires build.mu to be held by the caller.
func findOrInitScorecard(orchestration *BuildOrchestrationState, provider ai.AIProvider, shape TaskShape) *ProviderScorecard {
	for i := range orchestration.ProviderScorecards {
		if orchestration.ProviderScorecards[i].Provider == provider &&
			orchestration.ProviderScorecards[i].TaskShape == shape {
			return &orchestration.ProviderScorecards[i]
		}
	}
	orchestration.ProviderScorecards = append(orchestration.ProviderScorecards, ProviderScorecard{
		Provider:       provider,
		TaskShape:      shape,
		HostedEligible: true,
	})
	return &orchestration.ProviderScorecards[len(orchestration.ProviderScorecards)-1]
}

func adaptiveScorecardRoutingEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("APEX_ADAPTIVE_SCORECARD_ROUTING")))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func rollingAverage(current, n, newObs float64) float64 {
	if n <= 1 {
		return newObs
	}
	return (current*(n-1) + newObs) / n
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
