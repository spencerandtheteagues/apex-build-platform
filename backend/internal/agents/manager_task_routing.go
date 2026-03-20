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

type taskGenerationCandidate struct {
	Provider           ai.AIProvider
	Model              string
	Output             *TaskOutput
	RawContent         string
	DeterministicScore int
	VerifyPassed       bool
	VerifyErrors       []string
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

	prompt := fmt.Sprintf(`Review this AI-generated task result for correctness and build safety.

Task type: %s
Task description: %s
Routing mode: %s
Provider that generated the candidate: %s
Deterministic verification passed: %t
Deterministic verification errors:
%s

Candidate output:
%s

Return JSON only:
{
  "summary": "one short sentence",
  "warnings": ["optional warning"],
  "blockers": ["only concrete correctness/build blockers"],
  "confidence": 0.0
}`, task.Type, task.Description, taskRoutingMode(task), candidate.Provider, candidate.VerifyPassed, strings.Join(candidate.VerifyErrors, "\n"), summarizeTaskOutputForJudge(candidate.Output, 12000))

	ctx, cancel := context.WithTimeout(am.ctx, 45*time.Second)
	defer cancel()
	resp, err := am.aiRouter.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		MaxTokens:       300,
		Temperature:     0.1,
		SystemPrompt:    "You are a strict build verifier. Return concise JSON only.",
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
	report.Warnings = append([]string(nil), critique.Warnings...)
	report.Blockers = append([]string(nil), critique.Blockers...)
	report.Errors = append([]string(nil), critique.Blockers...)
	report.ConfidenceScore = critique.Confidence
	if len(report.Blockers) > 0 {
		report.Status = VerificationBlocked
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
}`, task.Type, task.Description, taskRoutingMode(task), string(payload))

	ctx, cancel := context.WithTimeout(am.ctx, 45*time.Second)
	defer cancel()
	resp, err := am.aiRouter.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		MaxTokens:       180,
		Temperature:     0.1,
		SystemPrompt:    "You are a strict build judge. Return concise JSON only.",
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
	passed, errs := am.verifyGeneratedCode(buildID, candidate.Output)
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
	model := agent.Model
	if provider != agent.Provider || strings.TrimSpace(model) == "" {
		model = selectModelForPowerMode(provider, build.PowerMode)
	}

	if am.budgetEnforcer != nil {
		preAuth, preAuthErr := am.budgetEnforcer.PreAuthorize(build.UserID, agent.BuildID, estimatedRequestCostUSD)
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

	response, err := am.aiRouter.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		MaxTokens:       maxTokens,
		Temperature:     temperature,
		SystemPrompt:    systemPrompt,
		PowerMode:       build.PowerMode,
		UsePlatformKeys: am.buildUsesPlatformKeys(build),
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.Content == "" {
		return nil, fmt.Errorf("AI generation returned empty response")
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
			Provider:     string(provider),
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
	attachAIResponseMetrics(output, provider, modelUsed, response)
	am.materializeStructuredPatchOutput(build, task, output)

	candidateAgent := *agent
	candidateAgent.Provider = provider
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
		Provider:   provider,
		Model:      modelUsed,
		Output:     output,
		RawContent: response.Content,
	}
	am.scoreTaskGenerationCandidate(agent.BuildID, candidate)
	return candidate, nil
}
