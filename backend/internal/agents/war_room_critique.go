package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"apex-build/internal/ai"
)

type buildSpecCritiqueCategory string

const (
	buildSpecCritiqueSecurity    buildSpecCritiqueCategory = "security"
	buildSpecCritiquePerformance buildSpecCritiqueCategory = "performance"
)

type buildSpecCritiqueIssue struct {
	Code           string
	Category       buildSpecCritiqueCategory
	Surface        ContractSurface
	Summary        string
	Recommendation string
}

func critiqueDraftBuildSpec(spec *ValidatedBuildSpec, contract *BuildContract) []buildSpecCritiqueIssue {
	if spec == nil {
		return nil
	}

	issues := make([]buildSpecCritiqueIssue, 0, 6)
	add := func(code string, category buildSpecCritiqueCategory, surface ContractSurface, summary, recommendation string) {
		issues = append(issues, buildSpecCritiqueIssue{
			Code:           code,
			Category:       category,
			Surface:        surface,
			Summary:        summary,
			Recommendation: recommendation,
		})
	}

	deliveryMode := strings.TrimSpace(strings.ToLower(spec.DeliveryMode))
	appType := strings.TrimSpace(strings.ToLower(spec.AppType))

	if (deliveryMode == "full_stack_preview" || deliveryMode == "api_runtime" || appType == "fullstack") && len(spec.APIPaths) == 0 {
		add(
			"war_room_api_plan_gap",
			buildSpecCritiqueSecurity,
			SurfaceIntegration,
			"Validated spec has backend/runtime expectations but no API path plan.",
			"Freeze the expected API routes before generation so frontend/backend integration remains truthful.",
		)
	}

	if (appType == "fullstack" || appType == "web") && len(spec.RoutePlan) == 0 {
		add(
			"war_room_route_plan_gap",
			buildSpecCritiquePerformance,
			SurfaceFrontend,
			"Validated spec is missing a concrete route plan for preview-visible surfaces.",
			"Add at least one concrete route/component target so generation does not improvise navigation.",
		)
	}

	if len(spec.PrimaryUserFlows) == 0 {
		add(
			"war_room_user_flow_gap",
			buildSpecCritiquePerformance,
			SurfaceGlobal,
			"Validated spec is missing explicit primary user flows.",
			"Define at least one critical flow to anchor implementation and verification tasks.",
		)
	}

	if contract != nil && contract.AuthContract != nil && contract.AuthContract.Required && !buildSpecHasAdvisoryCode(spec.SecurityAdvisories, "auth_session_hardening") {
		add(
			"war_room_auth_boundary_gap",
			buildSpecCritiqueSecurity,
			SurfaceBackend,
			"Auth contract is required but the validated spec does not call out auth session hardening.",
			"Require callback/session/token strategy guardrails in the validated spec before coding tasks start.",
		)
	}

	if containsString(spec.AcceptanceSurfaces, string(SurfaceBackend)) && len(spec.APIPaths) == 0 {
		add(
			"war_room_backend_acceptance_without_api",
			buildSpecCritiqueSecurity,
			SurfaceBackend,
			"Backend acceptance is required but no backend API surface is listed in the validated spec.",
			"Add backend API paths or explicitly scope backend acceptance to non-HTTP runtime checks.",
		)
	}

	return dedupeBuildSpecCritiqueIssues(issues)
}

func applyWarRoomCritiqueAdvisories(spec *ValidatedBuildSpec, issues []buildSpecCritiqueIssue) {
	if spec == nil || len(issues) == 0 {
		return
	}

	for _, issue := range issues {
		advisory := BuildSpecAdvisory{
			Code:           issue.Code,
			Severity:       BuildSpecSeverityWarning,
			Surface:        issue.Surface,
			Summary:        issue.Summary,
			Recommendation: issue.Recommendation,
		}

		switch issue.Category {
		case buildSpecCritiquePerformance:
			spec.PerformanceAdvisories = append(spec.PerformanceAdvisories, advisory)
		default:
			spec.SecurityAdvisories = append(spec.SecurityAdvisories, advisory)
		}
	}
}

func dedupeBuildSpecCritiqueIssues(values []buildSpecCritiqueIssue) []buildSpecCritiqueIssue {
	if len(values) == 0 {
		return nil
	}
	out := make([]buildSpecCritiqueIssue, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, issue := range values {
		key := strings.TrimSpace(issue.Code) + "|" + strings.TrimSpace(string(issue.Surface))
		if key == "|" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, issue)
	}
	return out
}

func buildSpecHasAdvisoryCode(values []BuildSpecAdvisory, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

// ── LLM Debate ───────────────────────────────────────────────────────────────

// llmWarRoomIssue is the JSON shape each provider returns for a single finding.
type llmWarRoomIssue struct {
	Code           string `json:"code"`
	Category       string `json:"category"` // "security" | "performance"
	Surface        string `json:"surface"`  // ContractSurface value
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
}

const warRoomLLMDebateTimeout = 8 * time.Second

func effectiveWarRoomCritiquePowerMode(build *Build) PowerMode {
	if build == nil {
		return PowerFast
	}
	switch build.PowerMode {
	case PowerMax:
		return PowerMax
	case PowerBalanced:
		return PowerBalanced
	case PowerFast:
		return PowerFast
	default:
		if build.Mode == ModeFast {
			return PowerFast
		}
		return PowerFast
	}
}

func shouldRunWarRoomLLMDebate(powerMode PowerMode) bool {
	switch powerMode {
	case PowerBalanced, PowerMax:
		return true
	default:
		return false
	}
}

func warRoomModelOverrideForProvider(provider ai.AIProvider, powerMode PowerMode, usesPlatformKeys bool) string {
	switch powerMode {
	case PowerMax:
		return normalizeExecutionModelForProvider(provider, "", PowerMax, usesPlatformKeys)
	case PowerBalanced:
		return normalizeExecutionModelForProvider(provider, "", PowerBalanced, usesPlatformKeys)
	default:
		return normalizeExecutionModelForProvider(provider, "", PowerFast, usesPlatformKeys)
	}
}

func warRoomLLMDebateTimeoutForPowerMode(mode PowerMode) time.Duration {
	switch mode {
	case PowerMax:
		return 12 * time.Second
	case PowerBalanced:
		return 10 * time.Second
	default:
		return warRoomLLMDebateTimeout
	}
}

func warRoomLLMDebateMaxTokensForPowerMode(mode PowerMode) int {
	switch mode {
	case PowerMax:
		return 900
	case PowerBalanced:
		return 700
	default:
		return 512
	}
}

func contractCritiqueTimeoutForPowerMode(mode PowerMode) time.Duration {
	switch mode {
	case PowerMax:
		return 30 * time.Second
	case PowerBalanced:
		return 25 * time.Second
	default:
		return 20 * time.Second
	}
}

func contractCritiqueMaxTokensForPowerMode(mode PowerMode) int {
	switch mode {
	case PowerMax:
		return 1000
	case PowerBalanced:
		return 750
	default:
		return 600
	}
}

// runWarRoomLLMDebate sends the frozen ValidatedBuildSpec to two providers
// concurrently (Claude for security focus, GPT-5 for architecture focus) and
// merges their findings into the spec's advisory lists. It is best-effort: any
// provider error is silently skipped so the build is never blocked.
func (am *AgentManager) runWarRoomLLMDebate(buildID string, userID uint, usesPlatformKeys bool, powerMode PowerMode, spec *ValidatedBuildSpec, contract *BuildContract) []buildSpecCritiqueIssue {
	if am == nil || am.aiRouter == nil || spec == nil {
		return nil
	}
	if !shouldRunWarRoomLLMDebate(powerMode) {
		return nil
	}

	summary := buildSpecDebateSummary(spec, contract)

	type debateRound struct {
		provider ai.AIProvider
		focus    string
	}
	rounds := []debateRound{
		{ai.ProviderClaude, "security vulnerabilities, auth gaps, and data exposure risks"},
		{ai.ProviderGPT4, "architectural bottlenecks, scalability risks, and missing integration surfaces"},
	}

	ctx := am.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, warRoomLLMDebateTimeoutForPowerMode(powerMode))
	defer cancel()
	maxTokens := warRoomLLMDebateMaxTokensForPowerMode(powerMode)

	type roundResult struct {
		issues []buildSpecCritiqueIssue
	}
	results := make([]roundResult, len(rounds))
	var wg sync.WaitGroup

	for i, round := range rounds {
		wg.Add(1)
		go func(idx int, r debateRound) {
			defer wg.Done()
			issues := runSingleDebateRound(ctx, am.aiRouter, buildID, userID, usesPlatformKeys, powerMode, maxTokens, summary, r.provider, r.focus)
			results[idx] = roundResult{issues: issues}
		}(i, round)
	}
	wg.Wait()

	var all []buildSpecCritiqueIssue
	for _, r := range results {
		all = append(all, r.issues...)
	}
	return dedupeBuildSpecCritiqueIssues(all)
}

func runSingleDebateRound(ctx context.Context, router AIRouter, buildID string, userID uint, usesPlatformKeys bool, powerMode PowerMode, maxTokens int, summary string, provider ai.AIProvider, focus string) []buildSpecCritiqueIssue {
	if maxTokens <= 0 {
		maxTokens = warRoomLLMDebateMaxTokensForPowerMode(powerMode)
	}
	prompt := fmt.Sprintf(`You are a senior software architect in a pre-generation War Room review.
Analyze this build specification and identify concrete issues in the area of: %s.

Build spec:
%s

Return a JSON array (no prose, no markdown fences) of issues. Each issue:
{
  "code": "war_room_llm_<slug>",
  "category": "security" or "performance",
  "surface": "frontend" | "backend" | "integration" | "global",
  "summary": "one sentence",
  "recommendation": "one actionable sentence"
}

Return [] if no issues found. Maximum 4 issues. Only flag real problems.`, focus, summary)

	resp, err := router.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:          userID,
		BuildID:         buildID,
		MaxTokens:       maxTokens,
		Temperature:     0.15,
		SystemPrompt:    "You are a strict build spec reviewer. Return a JSON array only.",
		RoleHint:        string(RoleReviewer),
		ModelOverride:   warRoomModelOverrideForProvider(provider, powerMode, usesPlatformKeys),
		PowerMode:       powerMode,
		UsePlatformKeys: usesPlatformKeys,
	})
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return nil
	}

	raw := extractJSONArrayBlock(resp.Content)
	if raw == "" {
		return nil
	}
	var parsed []llmWarRoomIssue
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.Printf("[war_room_llm] build=%s provider=%s parse error: %v", buildID, provider, err)
		return nil
	}

	issues := make([]buildSpecCritiqueIssue, 0, len(parsed))
	for _, p := range parsed {
		code := strings.TrimSpace(p.Code)
		summary := strings.TrimSpace(p.Summary)
		rec := strings.TrimSpace(p.Recommendation)
		if code == "" || summary == "" {
			continue
		}
		if !strings.HasPrefix(code, "war_room_llm_") {
			code = "war_room_llm_" + strings.ReplaceAll(strings.ToLower(code), " ", "_")
		}
		cat := buildSpecCritiqueSecurity
		if strings.Contains(strings.ToLower(p.Category), "perf") {
			cat = buildSpecCritiquePerformance
		}
		surface := ContractSurface(strings.TrimSpace(p.Surface))
		if surface == "" {
			surface = SurfaceGlobal
		}
		issues = append(issues, buildSpecCritiqueIssue{
			Code:           code,
			Category:       cat,
			Surface:        surface,
			Summary:        summary,
			Recommendation: rec,
		})
	}
	return issues
}

// buildSpecDebateSummary produces a compact text summary of the spec for LLM consumption.
func buildSpecDebateSummary(spec *ValidatedBuildSpec, contract *BuildContract) string {
	var b strings.Builder
	fmt.Fprintf(&b, "AppType: %s\n", spec.AppType)
	fmt.Fprintf(&b, "DeliveryMode: %s\n", spec.DeliveryMode)
	if len(spec.PrimaryUserFlows) > 0 {
		fmt.Fprintf(&b, "UserFlows: %s\n", strings.Join(spec.PrimaryUserFlows, " | "))
	}
	if len(spec.APIPaths) > 0 {
		fmt.Fprintf(&b, "APIPaths: %s\n", strings.Join(spec.APIPaths, ", "))
	}
	if len(spec.RoutePlan) > 0 {
		fmt.Fprintf(&b, "RoutePlan: %s\n", strings.Join(spec.RoutePlan, ", "))
	}
	if len(spec.AcceptanceSurfaces) > 0 {
		fmt.Fprintf(&b, "AcceptanceSurfaces: %s\n", strings.Join(spec.AcceptanceSurfaces, ", "))
	}
	if contract != nil {
		if contract.AuthContract != nil && contract.AuthContract.Required {
			strategy := firstNonEmptyStr(contract.AuthContract.TokenStrategy, contract.AuthContract.SessionStrategy, contract.AuthContract.CallbackStrategy)
			fmt.Fprintf(&b, "Auth: required (strategy=%s)\n", strategy)
		}
		if len(contract.DBSchemaContract) > 0 {
			names := make([]string, 0, len(contract.DBSchemaContract))
			for _, m := range contract.DBSchemaContract {
				names = append(names, m.Name)
			}
			fmt.Fprintf(&b, "DataModels: %s\n", strings.Join(names, ", "))
		}
	}
	return b.String()
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// extractJSONArrayBlock extracts the first [...] block from raw LLM output.
func extractJSONArrayBlock(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}
