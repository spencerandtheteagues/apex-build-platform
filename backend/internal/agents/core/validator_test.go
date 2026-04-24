package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- stub SmokeTestRunner ---

type stubSmokeRunner struct {
	output   string
	exitCode int
	err      error
}

func (s *stubSmokeRunner) RunSmokeTest(_ context.Context, _ string, _ time.Duration) (string, int, error) {
	return s.output, s.exitCode, s.err
}

// --- helpers ---

func cleanArtifacts() []BuildArtifact {
	return []BuildArtifact{
		{Path: "main.go", Content: "package main\n\nfunc main() {\n}\n", Language: "go", IsNew: true},
		{Path: "helper.go", Content: "package main\n\nfunc helper() string { return \"ok\" }\n", Language: "go"},
	}
}

func newValidator(t *testing.T, cfg ValidatorConfig, runner SmokeTestRunner) *BuildValidator {
	t.Helper()
	v, err := NewBuildValidator(cfg, runner)
	if err != nil {
		t.Fatalf("NewBuildValidator: %v", err)
	}
	return v
}

// --- tests ---

func TestValidate_CleanArtifacts_Pass(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	result := v.Validate(context.Background(), cleanArtifacts())
	if result.Verdict != VerdictPass {
		t.Fatalf("verdict = %s, want pass (score=%.2f, summary=%q)", result.Verdict, result.Score, result.ErrorSummary)
	}
}

func TestValidate_NoArtifacts_FilesExistCheckFails(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	result := v.Validate(context.Background(), nil)

	var found bool
	for _, ch := range result.Checks {
		if ch.Name == "files_exist" && !ch.Passed {
			found = true
		}
	}
	if !found {
		t.Fatal("expected files_exist check to fail when no artifacts are produced")
	}
}

func TestValidate_EmptyFile_FailsCheck(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	artifacts := []BuildArtifact{
		{Path: "empty.go", Content: "", Language: "go"},
	}
	result := v.Validate(context.Background(), artifacts)

	var found bool
	for _, ch := range result.Checks {
		if ch.Name == "no_empty_files" && !ch.Passed {
			found = true
		}
	}
	if !found {
		t.Fatal("expected no_empty_files check to fail for empty file")
	}
}

func TestValidate_UnbalancedBracket_FailsSyntaxCheck(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	artifacts := []BuildArtifact{
		{Path: "bad.go", Content: "func broken() {\n  if true {\n", Language: "go"},
	}
	result := v.Validate(context.Background(), artifacts)

	var found bool
	for _, ch := range result.Checks {
		if ch.Name == "syntax_sanity" && !ch.Passed {
			found = true
		}
	}
	if !found {
		t.Fatal("expected syntax_sanity check to fail for unbalanced brackets")
	}
}

func TestValidate_PlaceholderInCode_ReducesScore(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	artifacts := []BuildArtifact{
		{Path: "todo.go", Content: "func doSomething() {\n  // TODO: implement this\n  x := 1\n  _ = x\n}\n"},
	}
	result := v.Validate(context.Background(), artifacts)

	// A TODO in non-strict mode inside a comment is skipped; no placeholder hits expected
	if len(result.Placeholders) > 0 {
		t.Fatalf("expected 0 placeholder hits for doc comment TODO, got %d", len(result.Placeholders))
	}
}

func TestValidate_PlaceholderOutsideComment_Hit(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	// "TODO" in a string literal counts as a placeholder
	artifacts := []BuildArtifact{
		{Path: "bad.go", Content: `func f() string { return "TODO: replace this" }` + "\n"},
	}
	result := v.Validate(context.Background(), artifacts)
	if len(result.Placeholders) == 0 {
		t.Fatal("expected placeholder hits for inline TODO string, got 0")
	}
}

func TestValidate_SmokeTestPass_VerdictPass(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = true
	cfg.SmokeTestCommand = "echo ok"
	v := newValidator(t, cfg, &stubSmokeRunner{output: "ok", exitCode: 0})

	result := v.Validate(context.Background(), cleanArtifacts())
	if !result.SmokeTestPass {
		t.Fatal("expected SmokeTestPass=true for exit 0")
	}
	if result.Verdict != VerdictPass {
		t.Fatalf("verdict = %s, want pass", result.Verdict)
	}
}

func TestValidate_SmokeTestFail_SoftFail(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = true
	cfg.SmokeTestCommand = "false"
	v := newValidator(t, cfg, &stubSmokeRunner{output: "error", exitCode: 1})

	result := v.Validate(context.Background(), cleanArtifacts())
	if result.SmokeTestPass {
		t.Fatal("expected SmokeTestPass=false for exit 1")
	}
	if result.Verdict != VerdictSoftFail {
		t.Fatalf("verdict = %s, want soft_fail for smoke test failure", result.Verdict)
	}
}

func TestValidate_SmokeTestError_SoftFail(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = true
	cfg.SmokeTestCommand = "something"
	v := newValidator(t, cfg, &stubSmokeRunner{err: errors.New("exec: not found")})

	result := v.Validate(context.Background(), cleanArtifacts())
	if result.SmokeTestPass {
		t.Fatal("expected SmokeTestPass=false on runner error")
	}
}

func TestNewBuildValidator_InvalidPattern_ReturnsError(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.PlaceholderPatterns = []string{"[invalid("}
	_, err := NewBuildValidator(cfg, nil)
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestValidate_StrictMode_CommentTODO_IsHit(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	cfg.StrictMode = true
	v := newValidator(t, cfg, nil)

	artifacts := []BuildArtifact{
		{Path: "strict.go", Content: "// TODO: implement\nfunc f() {}\n"},
	}
	result := v.Validate(context.Background(), artifacts)
	if len(result.Placeholders) == 0 {
		t.Fatal("strict mode should flag comment TODOs as placeholders")
	}
}

func TestValidate_ManyPlaceholders_HardFail(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	// >5 placeholder hits should cause HardFail
	content := ""
	for i := 0; i < 6; i++ {
		content += "var x = \"TODO: fix this\"\n"
	}
	artifacts := []BuildArtifact{{Path: "many.go", Content: content}}
	result := v.Validate(context.Background(), artifacts)
	if result.Verdict != VerdictHardFail {
		t.Fatalf("verdict = %s, want hard_fail for >5 placeholders", result.Verdict)
	}
}

func TestValidate_ScoreAndDurationSet(t *testing.T) {
	cfg := DefaultValidatorConfig()
	cfg.RunSmokeTest = false
	v := newValidator(t, cfg, nil)

	result := v.Validate(context.Background(), cleanArtifacts())
	if result.Score <= 0 {
		t.Fatal("expected positive score")
	}
	if result.Duration <= 0 {
		t.Fatal("expected positive duration")
	}
	if result.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}
