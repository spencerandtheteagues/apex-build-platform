package guarantee

import (
	"context"
	"errors"
	"testing"
	"time"

	"apex-build/internal/agents/core"
)

// --- helpers ---

func buildFSM(t *testing.T) *core.AgentFSM {
	t.Helper()
	fsm := core.NewAgentFSM(core.AgentFSMConfig{BuildID: t.Name(), TotalSteps: 10})
	for _, ev := range []core.AgentEvent{core.EventStart, core.EventInitialized, core.EventPlanReady} {
		if err := fsm.Transition(ev); err != nil {
			t.Fatalf("FSM setup transition(%s): %v", ev, err)
		}
	}
	return fsm
}

func buildValidator(t *testing.T, smokeCmd string, runner core.SmokeTestRunner) *core.BuildValidator {
	t.Helper()
	cfg := core.DefaultValidatorConfig()
	cfg.RunSmokeTest = smokeCmd != ""
	cfg.SmokeTestCommand = smokeCmd
	v, err := core.NewBuildValidator(cfg, runner)
	if err != nil {
		t.Fatalf("NewBuildValidator: %v", err)
	}
	return v
}

func cleanArtifacts() []core.BuildArtifact {
	return []core.BuildArtifact{
		{Path: "main.go", Content: "package main\n\nfunc main() {}\n", Language: "go"},
	}
}

func instantConfig() EngineConfig {
	return EngineConfig{
		MaxRetries:         3,
		RetryBackoff:       0, // no sleep in tests
		CheckpointEveryN:   1,
		FailFastOnHardFail: true,
	}
}

// --- tests ---

func TestExecuteWithGuarantee_CleanStep_Succeeds(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	engine := NewGuaranteeEngine(fsm, validator, instantConfig())

	result, err := engine.ExecuteWithGuarantee(context.Background(), "gen", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		return cleanArtifacts(), nil
	})

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", result.Attempts)
	}
}

func TestExecuteWithGuarantee_StepReturnsError_RetriesToMax(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	cfg := instantConfig()
	cfg.MaxRetries = 2
	engine := NewGuaranteeEngine(fsm, validator, cfg)

	attempts := 0
	result, err := engine.ExecuteWithGuarantee(context.Background(), "failing", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		attempts++
		return nil, errors.New("simulated build error")
	})

	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
	if result.Success {
		t.Fatal("expected Success=false")
	}
	if attempts != cfg.MaxRetries+1 {
		t.Fatalf("expected %d attempts, got %d", cfg.MaxRetries+1, attempts)
	}
}

func TestExecuteWithGuarantee_RetryContext_IsPassedOnRetry(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	cfg := instantConfig()
	cfg.MaxRetries = 1
	engine := NewGuaranteeEngine(fsm, validator, cfg)

	var contexts []*RetryContext
	_, _ = engine.ExecuteWithGuarantee(context.Background(), "ctx-test", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		contexts = append(contexts, rc)
		return nil, errors.New("fail")
	})

	if len(contexts) < 2 {
		t.Fatalf("expected ≥2 retry contexts, got %d", len(contexts))
	}
	if contexts[0].IsRetry {
		t.Error("first attempt should not be a retry")
	}
	if !contexts[1].IsRetry {
		t.Error("second attempt should be marked as retry")
	}
}

func TestExecuteWithGuarantee_SoftFail_RetriesThenRollsBack(t *testing.T) {
	fsm := buildFSM(t)

	// Validator with smoke test that always fails (soft fail)
	smokeRunner := &alwaysFailSmokeRunner{}
	cfg := core.DefaultValidatorConfig()
	cfg.RunSmokeTest = true
	cfg.SmokeTestCommand = "test"
	validator, _ := core.NewBuildValidator(cfg, smokeRunner)

	engineCfg := instantConfig()
	engineCfg.MaxRetries = 1
	engine := NewGuaranteeEngine(fsm, validator, engineCfg)

	result, err := engine.ExecuteWithGuarantee(context.Background(), "smoke-fail", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		return cleanArtifacts(), nil
	})

	if err == nil {
		t.Fatal("expected error after smoke test failures")
	}
	if result.Success {
		t.Fatal("expected Success=false")
	}
}

func TestExecuteWithGuarantee_HardFail_ImmediatelyRollsBack(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	engine := NewGuaranteeEngine(fsm, validator, instantConfig())

	calls := 0
	result, err := engine.ExecuteWithGuarantee(context.Background(), "hard-fail", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		calls++
		// 7+ placeholder hits → hard fail
		var content string
		for i := 0; i < 8; i++ {
			content += "x = \"TODO: fix\"\n"
		}
		return []core.BuildArtifact{{Path: "bad.py", Content: content}}, nil
	})

	if err == nil {
		t.Fatal("expected error on hard fail")
	}
	if result.Success {
		t.Fatal("expected Success=false on hard fail")
	}
	// Hard fail should not retry (FailFastOnHardFail=true) — should only run once
	if calls != 1 {
		t.Fatalf("FailFastOnHardFail should prevent retries, got %d calls", calls)
	}
}

func TestExecuteWithGuarantee_CreatesCheckpoint(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	engine := NewGuaranteeEngine(fsm, validator, instantConfig())

	_, _ = engine.ExecuteWithGuarantee(context.Background(), "cp-step", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		return cleanArtifacts(), nil
	})

	cps := fsm.ListCheckpoints()
	if len(cps) == 0 {
		t.Fatal("expected at least one checkpoint after execution")
	}
}

func TestExecuteWithGuarantee_MultipleSteps_AccumulateCheckpoints(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	engine := NewGuaranteeEngine(fsm, validator, instantConfig())

	for i := 0; i < 3; i++ {
		_, err := engine.ExecuteWithGuarantee(context.Background(), "step", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
			return cleanArtifacts(), nil
		})
		if err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}

	cps := fsm.ListCheckpoints()
	if len(cps) < 3 {
		t.Fatalf("expected ≥3 checkpoints for 3 steps, got %d", len(cps))
	}
}

func TestDefaultEngineConfig_ReasonableDefaults(t *testing.T) {
	cfg := DefaultEngineConfig()
	if cfg.MaxRetries <= 0 {
		t.Errorf("MaxRetries should be positive, got %d", cfg.MaxRetries)
	}
	if cfg.RetryBackoff <= 0 {
		t.Errorf("RetryBackoff should be positive, got %v", cfg.RetryBackoff)
	}
	if cfg.CheckpointEveryN <= 0 {
		t.Errorf("CheckpointEveryN should be positive, got %d", cfg.CheckpointEveryN)
	}
}

func TestBuildCorrectionHints_PlaceholderCheck(t *testing.T) {
	result := &core.ValidationResult{
		Checks: []core.ValidationCheck{
			{Name: "placeholder_scan", Passed: false},
		},
		Placeholders: []core.PlaceholderHit{
			{FilePath: "f.go", Match: "TODO"},
		},
	}

	hints := buildCorrectionHints(result)
	if len(hints) == 0 {
		t.Fatal("expected correction hints for failed placeholder check")
	}
}

func TestBuildCorrectionHints_AllCheckTypes(t *testing.T) {
	result := &core.ValidationResult{
		Checks: []core.ValidationCheck{
			{Name: "placeholder_scan", Passed: false},
			{Name: "no_empty_files", Passed: false},
			{Name: "syntax_sanity", Passed: false},
			{Name: "smoke_test", Passed: false},
			{Name: "import_check", Passed: false},
		},
	}

	hints := buildCorrectionHints(result)
	if len(hints) < 4 {
		t.Fatalf("expected at least 4 correction hints, got %d", len(hints))
	}
}

func TestGuaranteeEvents_RecordedProperly(t *testing.T) {
	fsm := buildFSM(t)
	validator := buildValidator(t, "", nil)
	engine := NewGuaranteeEngine(fsm, validator, instantConfig())

	_, _ = engine.ExecuteWithGuarantee(context.Background(), "log-step", func(ctx context.Context, rc *RetryContext) ([]core.BuildArtifact, error) {
		return cleanArtifacts(), nil
	})

	events := engine.Events()
	if len(events) == 0 {
		t.Fatal("expected guarantee events to be recorded")
	}
	// Should have at least "execute" and "pass" events
	hasExecute := false
	hasPass := false
	for _, ev := range events {
		if ev.Type == "execute" {
			hasExecute = true
		}
		if ev.Type == "pass" {
			hasPass = true
		}
	}
	if !hasExecute {
		t.Error("expected 'execute' event in log")
	}
	if !hasPass {
		t.Error("expected 'pass' event in log")
	}
}

// --- stub smoke runner that always fails ---

type alwaysFailSmokeRunner struct{}

func (s *alwaysFailSmokeRunner) RunSmokeTest(_ context.Context, _ string, _ time.Duration) (string, int, error) {
	return "test failed", 1, nil
}
