package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"apex-build/internal/ai"
	"apex-build/internal/spend"

	"github.com/google/uuid"
)

func taskRoutingMode(task *Task) ProviderRoutingMode {
	if task == nil || task.Input == nil {
		return RoutingModeSingleProvider
	}
	if artifact := taskArtifactWorkOrderFromInput(task); artifact != nil && artifact.RoutingMode != "" {
		return artifact.RoutingMode
	}
	if raw, ok := task.Input["routing_mode"]; ok {
		if mode := ProviderRoutingMode(strings.TrimSpace(fmt.Sprintf("%v", raw))); mode != "" {
			return mode
		}
	}
	return RoutingModeSingleProvider
}

func taskRiskLevel(task *Task) TaskRiskLevel {
	if task == nil {
		return RiskMedium
	}
	if artifact := taskArtifactWorkOrderFromInput(task); artifact != nil && artifact.RiskLevel != "" {
		return artifact.RiskLevel
	}
	if task.Input != nil {
		if raw, ok := task.Input["risk_level"]; ok {
			if risk := TaskRiskLevel(strings.TrimSpace(fmt.Sprintf("%v", raw))); risk != "" {
				return risk
			}
		}
	}
	return RiskMedium
}

func effectiveTaskRoutingMode(build *Build, task *Task) ProviderRoutingMode {
	mode := taskRoutingMode(task)
	if build == nil {
		return mode
	}
	if build.PowerMode != PowerFast && build.Mode != ModeFast {
		return mode
	}

	risk := taskRiskLevel(task)
	switch mode {
	case RoutingModeDualCandidate, RoutingModeSingleWithVerifier:
		if risk != RiskHigh && risk != RiskCritical {
			return RoutingModeSingleProvider
		}
	}
	return mode
}

func shouldRunProviderAssistedContractCritique(build *Build, contract *BuildContract) bool {
	if build == nil || contract == nil {
		return false
	}
	if build.PowerMode != PowerFast && build.Mode != ModeFast {
		return true
	}
	if contract.AuthContract != nil && contract.AuthContract.Required {
		return true
	}
	if contract.APIContract != nil && len(contract.APIContract.Endpoints) > 0 && len(contract.DBSchemaContract) > 0 {
		return true
	}
	return false
}

type taskGenerationCandidate struct {
	Provider           ai.AIProvider
	Model              string
	Output             *TaskOutput
	RawContent         string
	DeterministicScore int
	VerifyPassed       bool
	VerifyErrors       []string
	Triage             TaskTriageResult
	WaterfallStage     string
	WaterfallReason    string
	WaterfallPowerMode PowerMode
}

func summarizeTaskOutputForJudge(output *TaskOutput, limit int) string {
	if output == nil {
		return "(no output)"
	}
	var b strings.Builder
	if output.StructuredPatchBundle != nil && len(output.StructuredPatchBundle.Operations) > 0 {
		b.WriteString("structured_patch_bundle:\n")
		if payload, err := json.MarshalIndent(output.StructuredPatchBundle, "", "  "); err == nil {
			b.Write(payload)
			b.WriteString("\n")
		}
	}
	if len(output.Files) > 0 {
		b.WriteString("files:\n")
		for _, file := range output.Files {
			b.WriteString(fmt.Sprintf("// File: %s\n```%s\n%s\n```\n", file.Path, file.Language, file.Content))
			if limit > 0 && b.Len() >= limit {
				break
			}
		}
	}
	if len(output.DeletedFiles) > 0 {
		b.WriteString("deleted_files:\n- ")
		b.WriteString(strings.Join(output.DeletedFiles, "\n- "))
		b.WriteString("\n")
	}
	if len(output.Messages) > 0 {
		b.WriteString("messages:\n- ")
		b.WriteString(strings.Join(output.Messages, "\n- "))
		b.WriteString("\n")
	}
	summary := strings.TrimSpace(b.String())
	if limit > 0 && len(summary) > limit {
		summary = strings.TrimSpace(summary[:limit]) + "\n... (truncated)"
	}
	if summary == "" {
		return "(no output)"
	}
	return summary
}

func (am *AgentManager) providerAssistedTaskVerification(build *Build, task *Task, candidate *taskGenerationCandidate) *VerificationReport {
	if am == nil || am.aiRouter == nil || build == nil || task == nil || candidate == nil || candidate.Output == nil {
		return nil
	}

	if skip, reason := skipRecursiveProviderCritiqueForTask(task, candidate.Output); skip {
		return skippedTaskProviderVerificationReport(build, task, reason)
	}

	availableProviders := am.getCurrentlyAvailableProvidersForBuild(build)
	if len(availableProviders) == 0 {
		return nil
	}
	scorecards := am.providerScorecardsForBuild(build, availableProviders)
	provider := preferredProviderForTaskShape(TaskShapeAdversarialCritique, scorecards)
	if provider == "" || provider == candidate.Provider {
		for _, available := range availableProviders {
			if available != candidate.Provider {
				provider = available
				break
			}
		}
		if provider == "" {
			provider = availableProviders[0]
		}
	}

	report := &VerificationReport{
		ID:            uuid.New().String(),
		BuildID:       build.ID,
		Phase:         "task_provider_verification",
		Surface:       SurfaceGlobal,
		Status:        VerificationPassed,
		Deterministic: false,
		Provider:      provider,
		ChecksRun:     []string{"provider_adversarial_review"},
		GeneratedAt:   time.Now().UTC(),
	}
	if artifact := taskArtifactWorkOrderFromInput(task); artifact != nil {
		report.WorkOrderID = artifact.ID
		if artifact.ContractSlice.Surface != "" {
			report.Surface = artifact.ContractSlice.Surface
		}
		report.TruthTags = append([]TruthTag(nil), artifact.ContractSlice.TruthTags...)
	}

	if deterministicTaskGatesEnabledForBuild(build) {
		deterministic := am.evaluateDeterministicVerification(build, task, candidate)
		report.Deterministic = deterministic.Ran
		report.DeterministicStatus = deterministic.DeterministicStatus
		report.ProviderCritiqueStatus = deterministic.ProviderCritiqueStatus
		report.ChecksRun = dedupeStrings(append(report.ChecksRun, deterministic.Checks...))
		report.Warnings = dedupeStrings(append(report.Warnings, deterministic.Warnings...))

		if deterministic.DeterministicStatus == verificationReasonDeterministicFailed {
			report.Status = VerificationBlocked
			report.Provider = ""
			report.Errors = dedupeStrings(append(report.Errors, deterministic.Errors...))
			report.Blockers = dedupeStrings(append(report.Blockers, deterministic.Errors...))
			report.ChecksRun = dedupeStrings(append(report.ChecksRun, verificationReasonDeterministicFailed, verificationReasonProviderCritiqueSkip))
			report.ProviderCritiqueStatus = verificationReasonProviderCritiqueSkip
			report.ConfidenceScore = 0.99
			am.broadcast(build.ID, &WSMessage{
				Type:      WSGlassDeterministicGateFailed,
				BuildID:   build.ID,
				AgentID:   task.AssignedTo,
				Timestamp: time.Now(),
				Data: map[string]any{
					"agent_role": "verifier",
					"provider":   "deterministic_gate",
					"task_id":    task.ID,
					"task_type":  string(task.Type),
					"errors":     deterministic.Errors,
					"content":    fmt.Sprintf("Deterministic gate failed for %s.", task.Type),
				},
			})
			return report
		}

		report.ChecksRun = dedupeStrings(append(report.ChecksRun, verificationReasonDeterministicPassed, verificationReasonProviderCritiqueNeed))
		report.ProviderCritiqueStatus = verificationReasonProviderCritiqueNeed
		if deterministic.Ran {
			am.broadcast(build.ID, &WSMessage{
				Type:      WSGlassDeterministicGatePassed,
				BuildID:   build.ID,
				AgentID:   task.AssignedTo,
				Timestamp: time.Now(),
				Data: map[string]any{
					"agent_role": "verifier",
					"provider":   "deterministic_gate",
					"task_id":    task.ID,
					"task_type":  string(task.Type),
					"checks":     deterministic.Checks,
					"content":    fmt.Sprintf("Deterministic gate passed for %s.", task.Type),
				},
			})
		}
	}

	prompt := fmt.Sprintf(`Review this AI-generated task result for concrete correctness issues only.

Task type: %s
Task description: %s
Routing mode: %s
Provider that generated the candidate: %s
Deterministic verification passed: %t
Deterministic verification errors:
%s

Candidate output:
%s

IMPORTANT RULES — do NOT flag these as blockers or warnings:
- Mock classes, stub implementations, or test doubles in test files (*.test.*, *.spec.*, _test.*) — mocks are correct test practice
- Use of interfaces or dependency injection
- TODO comments that are inside test files
- Placeholder values in example/seed files
- Standard test patterns like MockJobManager, FakeRepository, StubService

Only flag as a blocker if there is a concrete compilation error, a missing required import, or a structurally broken file that would prevent the build from running.

Return JSON only:
{
  "summary": "one short sentence",
  "warnings": ["only if genuinely suspicious"],
  "blockers": ["only concrete build-breaking issues, never test patterns"],
  "confidence": 0.0
}`, task.Type, task.Description, effectiveTaskRoutingMode(build, task), candidate.Provider, candidate.VerifyPassed, strings.Join(candidate.VerifyErrors, "\n"), summarizeTaskOutputForJudge(candidate.Output, 12000))

	ctx, cancel := context.WithTimeout(am.ctx, 45*time.Second)
	defer cancel()
	resp, err := am.aiRouter.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		BuildID:         build.ID,
		MaxTokens:       300,
		Temperature:     0.1,
		SystemPrompt:    "You are a strict build verifier. Return concise JSON only.",
		RoleHint:        string(RoleReviewer),
		PowerMode:       PowerFast,
		UsePlatformKeys: am.buildUsesPlatformKeys(build),
	})
	if err != nil || resp == nil {
		return nil
	}
	payload := extractJSONObjectBlock(resp.Content)
	if payload == "" {
		return nil
	}
	var critique contractCritiquePayload
	if err := json.Unmarshal([]byte(payload), &critique); err != nil {
		return nil
	}

	critique.Warnings = dedupeStrings(critique.Warnings)
	critique.Blockers = dedupeStrings(critique.Blockers)
	report.ConfidenceScore = critique.Confidence
	if report.ConfidenceScore <= 0 {
		report.ConfidenceScore = 0.72
	}

	// Single-with-verifier routing is an explicit reliability mode. If the second
	// provider returns concrete blockers with reasonable confidence, it should be
	// able to veto the candidate instead of being reduced to an advisory note.
	if len(critique.Blockers) > 0 && report.ConfidenceScore >= 0.75 {
		report.Status = VerificationBlocked
		report.Blockers = critique.Blockers
		report.Errors = append([]string(nil), critique.Blockers...)
		report.Warnings = critique.Warnings
		return report
	}

	// Low-confidence critique remains advisory so the verifier does not create a
	// new source of false-positive build failures.
	warnings := append([]string(nil), report.Warnings...)
	warnings = append(warnings, critique.Warnings...)
	warnings = append(warnings, critique.Blockers...)
	report.Warnings = dedupeStrings(warnings)
	if report.ProviderCritiqueStatus == "" {
		report.ProviderCritiqueStatus = verificationReasonProviderCritiqueNeed
	}
	if report.DeterministicStatus == "" {
		report.DeterministicStatus = verificationReasonDeterministicPassed
	}
	return report
}

func skipRecursiveProviderCritiqueForTask(task *Task, output *TaskOutput) (bool, string) {
	if task == nil || output == nil {
		return false, ""
	}
	switch task.Type {
	case TaskReview:
		if len(output.Files) > 0 || len(output.DeletedFiles) > 0 {
			return false, ""
		}
		return true, "review_task_recursive_critique_skipped"
	default:
		return false, ""
	}
}

func skippedTaskProviderVerificationReport(build *Build, task *Task, reason string) *VerificationReport {
	report := &VerificationReport{
		ID:                     uuid.New().String(),
		BuildID:                build.ID,
		Phase:                  "task_provider_verification",
		Surface:                SurfaceGlobal,
		Status:                 VerificationPassed,
		Deterministic:          false,
		DeterministicStatus:    verificationReasonDeterministicPassed,
		ProviderCritiqueStatus: verificationReasonProviderCritiqueSkip,
		ChecksRun:              dedupeStrings([]string{"provider_adversarial_review_skipped", verificationReasonProviderCritiqueSkip, reason}),
		ConfidenceScore:        1,
		GeneratedAt:            time.Now().UTC(),
	}
	if artifact := taskArtifactWorkOrderFromInput(task); artifact != nil {
		report.WorkOrderID = artifact.ID
		if artifact.ContractSlice.Surface != "" {
			report.Surface = artifact.ContractSlice.Surface
		}
		report.TruthTags = append([]TruthTag(nil), artifact.ContractSlice.TruthTags...)
	}
	return report
}

func (am *AgentManager) judgeTaskCandidates(build *Build, task *Task, candidates []*taskGenerationCandidate) (int, ai.AIProvider, string) {
	if am == nil || am.aiRouter == nil || build == nil || task == nil || len(candidates) < 2 {
		return -1, "", ""
	}
	availableProviders := am.getCurrentlyAvailableProvidersForBuild(build)
	if len(availableProviders) == 0 {
		return -1, "", ""
	}
	scorecards := am.providerScorecardsForBuild(build, availableProviders)
	provider := preferredProviderForTaskShape(TaskShapeAdversarialCritique, scorecards)
	if provider == "" {
		provider = availableProviders[0]
	}

	type candidateSummary struct {
		Index              int      `json:"index"`
		Provider           string   `json:"provider"`
		Model              string   `json:"model"`
		DeterministicScore int      `json:"deterministic_score"`
		VerifyPassed       bool     `json:"verify_passed"`
		VerifyErrors       []string `json:"verify_errors,omitempty"`
		Output             string   `json:"output"`
	}
	summaries := make([]candidateSummary, 0, len(candidates))
	for i, candidate := range candidates {
		summaries = append(summaries, candidateSummary{
			Index:              i,
			Provider:           string(candidate.Provider),
			Model:              candidate.Model,
			DeterministicScore: candidate.DeterministicScore,
			VerifyPassed:       candidate.VerifyPassed,
			VerifyErrors:       append([]string(nil), candidate.VerifyErrors...),
			Output:             summarizeTaskOutputForJudge(candidate.Output, 8000),
		})
	}
	payload, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return -1, "", ""
	}

	prompt := fmt.Sprintf(`Choose the better build candidate for this task.

Task type: %s
Task description: %s
Routing mode: %s

Candidates:
%s

Favor candidates that are more correct, more complete, and more likely to compile.
Penalize truncation, placeholders, invalid structure, and unnecessary scope.

Return JSON only:
{
  "winner_index": 0,
  "rationale": "one short sentence"
}`, task.Type, task.Description, effectiveTaskRoutingMode(build, task), string(payload))

	ctx, cancel := context.WithTimeout(am.ctx, 45*time.Second)
	defer cancel()
	resp, err := am.aiRouter.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		BuildID:         build.ID,
		MaxTokens:       180,
		Temperature:     0.1,
		SystemPrompt:    "You are a strict build judge. Return concise JSON only.",
		RoleHint:        string(RoleReviewer),
		PowerMode:       PowerFast,
		UsePlatformKeys: am.buildUsesPlatformKeys(build),
	})
	if err != nil || resp == nil {
		return -1, "", ""
	}
	raw := extractJSONObjectBlock(resp.Content)
	if raw == "" {
		return -1, "", ""
	}
	var verdict struct {
		WinnerIndex int    `json:"winner_index"`
		Rationale   string `json:"rationale"`
	}
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return -1, "", ""
	}
	if verdict.WinnerIndex < 0 || verdict.WinnerIndex >= len(candidates) {
		return -1, "", ""
	}
	return verdict.WinnerIndex, provider, strings.TrimSpace(verdict.Rationale)
}

func (am *AgentManager) scoreTaskGenerationCandidate(buildID string, candidate *taskGenerationCandidate) {
	if am == nil || candidate == nil {
		return
	}
	passed, errs := am.verifyGeneratedCode(buildID, nil, candidate.Output)
	score := 100
	if !passed {
		score -= 40 + len(errs)*5
	}
	if candidate.Output != nil {
		score -= len(candidate.Output.TruncatedFiles) * 20
		if len(candidate.Output.Files) == 0 && len(candidate.Output.DeletedFiles) == 0 {
			score -= 25
		}
		score += minInt(len(candidate.Output.Files), 6) * 2
		score += minInt(len(candidate.Output.DeletedFiles), 3)
		if candidate.Output.StructuredPatchBundle != nil {
			score += 4
		}
		if candidate.Output.Completion != nil {
			score += 2
		}
	}
	candidate.VerifyPassed = passed
	candidate.VerifyErrors = append([]string(nil), errs...)
	candidate.DeterministicScore = score
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (am *AgentManager) generateTaskOutputWithProvider(
	ctx context.Context,
	build *Build,
	agent *Agent,
	task *Task,
	prompt string,
	systemPrompt string,
	provider ai.AIProvider,
	maxTokens int,
	temperature float64,
) (*taskGenerationCandidate, error) {
	if am == nil || build == nil || agent == nil || task == nil {
		return nil, fmt.Errorf("missing generation context")
	}
	triage := triageTaskForWaterfall(task)
	callPowerMode := build.PowerMode
	waterfallStage := "static_fallback"
	waterfallReason := "routing_waterfall_disabled"
	model := strings.TrimSpace(agent.Model)

	if routingWaterfallEnabledForBuild(build) {
		decision := planRoutingWaterfall(build, task, provider)
		triage = decision.Triage
		if decision.PowerMode != "" {
			callPowerMode = decision.PowerMode
		}
		if strings.TrimSpace(decision.Model) != "" {
			model = strings.TrimSpace(decision.Model)
		}
		waterfallStage = decision.Stage
		waterfallReason = decision.Reason
	}
	managedPlatformOllama := provider == ai.ProviderOllama && am.buildUsesPlatformKeys(build)
	if !managedPlatformOllama {
		if explicitModel := providerModelOverrideForBuild(build, provider); explicitModel != "" {
			model = explicitModel
			waterfallStage = "manual_override"
			waterfallReason = "provider_model_override"
		}
	}

	model = normalizeExecutionModelForProvider(provider, model, callPowerMode, am.buildUsesPlatformKeys(build))
	if managedPlatformOllama {
		waterfallStage = "managed_ollama"
		waterfallReason = "platform_forces_kimi_cloud"
	}

	if am.budgetEnforcer != nil {
		preAuth, preAuthErr := am.budgetEnforcer.PreAuthorize(build.UserID, agent.BuildID, estimatedRequestCostUSDForBuild(build))
		if preAuthErr == nil && !preAuth.Allowed {
			am.broadcast(agent.BuildID, &WSMessage{
				Type:      "budget:exceeded",
				BuildID:   agent.BuildID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"reason":      preAuth.Reason,
					"cap_type":    preAuth.CapType,
					"limit_usd":   preAuth.LimitUSD,
					"current_usd": preAuth.CurrentUSD,
				},
			})
			return nil, fmt.Errorf("budget cap exceeded: %s", preAuth.Reason)
		}
	}
	am.broadcast(build.ID, &WSMessage{
		Type:      WSGlassProviderRouteSelected,
		BuildID:   build.ID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role":         string(agent.Role),
			"provider":           string(provider),
			"selected_provider":  string(provider),
			"model":              model,
			"task_id":            task.ID,
			"task_type":          string(task.Type),
			"routing_stage":      waterfallStage,
			"routing_reason":     waterfallReason,
			"routing_power_mode": string(callPowerMode),
			"content":            fmt.Sprintf("%s routed %s to %s with %s (%s).", agent.Role, task.Type, provider, firstNonEmptyString(model, "default model"), waterfallReason),
		},
	})

	stopHeartbeat := am.startAgentActivityHeartbeat(ctx, build.ID, agent, task, "agent:generating", "generation", provider, model)
	defer stopHeartbeat()

	attemptCtx := ctx
	attemptCancel := func() {}
	if defaultTimeout := defaultGenerateTimeout(provider, callPowerMode); defaultTimeout > 0 {
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			remaining := time.Until(deadline)
			if remaining < defaultTimeout {
				defaultTimeout = remaining
			}
		}
		if defaultTimeout > 0 {
			attemptCtx, attemptCancel = context.WithTimeout(ctx, defaultTimeout)
		}
	}
	defer attemptCancel()

	// Lead, planner, and solver agents reuse the same large system prompt on every
	// call — enabling caching cuts their token costs by 50-80%.
	cacheSystemPrompt := agent.Role == RoleLead || agent.Role == RolePlanner || agent.Role == RoleSolver

	response, err := am.aiRouter.Generate(attemptCtx, provider, prompt, GenerateOptions{
		UserID:            build.UserID,
		BuildID:           build.ID,
		MaxTokens:         maxTokens,
		Temperature:       temperature,
		SystemPrompt:      systemPrompt,
		RoleHint:          string(agent.Role),
		ModelOverride:     model,
		PowerMode:         callPowerMode,
		UsePlatformKeys:   am.buildUsesPlatformKeys(build),
		CacheSystemPrompt: cacheSystemPrompt,
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.Content == "" {
		return nil, fmt.Errorf("AI generation returned empty response")
	}

	providerUsed := firstNonEmptyProvider(response.Provider, provider)
	actualProviderUsed := providerUsed
	if response != nil && response.Metadata != nil {
		if actual := parseAIProvider(fmt.Sprintf("%v", response.Metadata["actual_provider"])); actual != "" {
			actualProviderUsed = actual
		}
	}
	modelUsed := firstNonEmptyString(ai.GetModelUsed(response, nil), model)

	if am.spendTracker != nil && response.Usage != nil {
		projectID := build.ProjectID
		powerMode := string(build.PowerMode)
		usesPlatformKeys := am.buildUsesPlatformKeys(build)
		si := spend.RecordSpendInput{
			UserID:       build.UserID,
			ProjectID:    projectID,
			BuildID:      agent.BuildID,
			AgentID:      agent.ID,
			AgentRole:    string(agent.Role),
			Provider:     string(providerUsed),
			Model:        modelUsed,
			Capability:   string(task.Type),
			IsBYOK:       !usesPlatformKeys,
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
			PowerMode:    powerMode,
			Status:       "success",
		}
		if _, err := am.spendTracker.RecordSpend(si); err != nil {
			log.Printf("spend: failed to record agent spend for build %s agent %s: %v", agent.BuildID, agent.ID, err)
		}
	}

	output := am.parseTaskOutput(task.Type, response.Content)
	if output.Metrics == nil {
		output.Metrics = map[string]any{}
	}
	output.Metrics["triage_task_shape"] = string(triage.TaskShape)
	output.Metrics["triage_risk_level"] = string(triage.RiskLevel)
	output.Metrics["triage_scope"] = triage.Scope
	output.Metrics["routing_waterfall_stage"] = waterfallStage
	output.Metrics["routing_waterfall_reason"] = waterfallReason
	output.Metrics["routing_waterfall_power_mode"] = string(callPowerMode)
	output.Metrics["requested_provider"] = string(provider)
	output.Metrics["selected_provider"] = string(providerUsed)
	output.Metrics["actual_provider"] = string(actualProviderUsed)
	attachAIResponseMetrics(output, providerUsed, modelUsed, response)
	am.materializeStructuredPatchOutput(build, task, output)
	trackLikelyTruncatedSourceFiles(output)

	candidateAgent := *agent
	candidateAgent.Provider = providerUsed
	candidateAgent.Model = modelUsed
	if len(output.TruncatedFiles) > 0 {
		am.completeTruncatedFiles(ctx, task, build, &candidateAgent, output)
	}
	if am.isCodeGenerationTask(task.Type) {
		if rawHints := task.Input["repair_hints"]; rawHints != nil {
			am.applyChunkedRepairToLargeFiles(ctx, task, build, &candidateAgent, output)
		}
	}

	candidate := &taskGenerationCandidate{
		Provider:           providerUsed,
		Model:              modelUsed,
		Output:             output,
		RawContent:         response.Content,
		Triage:             triage,
		WaterfallStage:     waterfallStage,
		WaterfallReason:    waterfallReason,
		WaterfallPowerMode: callPowerMode,
	}
	am.scoreTaskGenerationCandidate(agent.BuildID, candidate)
	return candidate, nil
}
