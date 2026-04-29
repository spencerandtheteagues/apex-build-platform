package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"apex-build/internal/agents/autonomous"
	"apex-build/internal/ai"
)

const (
	taskStartAckOpenTag          = "<task_start_ack>"
	taskStartAckCloseTag         = "</task_start_ack>"
	taskCompletionReportOpenTag  = "<task_completion_report>"
	taskCompletionReportCloseTag = "</task_completion_report>"
)

func extractBalancedJSONObjectAt(raw string, start int) (string, int) {
	if start < 0 || start >= len(raw) || raw[start] != '{' {
		return "", -1
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], i + 1
			}
		}
	}

	return "", -1
}

func extractTaggedJSONObject(raw, openTag, closeTag string) (string, string, bool) {
	idx := strings.Index(raw, openTag)
	if idx == -1 {
		return raw, "", false
	}

	start := idx + len(openTag)
	for start < len(raw) {
		switch raw[start] {
		case ' ', '\t', '\r', '\n':
			start++
		default:
			goto parseObject
		}
	}

parseObject:
	obj, end := extractBalancedJSONObjectAt(raw, start)
	if obj == "" || end < 0 {
		return raw, "", false
	}

	removeEnd := end
	rest := raw[end:]
	trimmedRest := strings.TrimLeft(rest, " \t\r\n")
	if closeTag != "" && strings.HasPrefix(trimmedRest, closeTag) {
		removeEnd = len(raw) - len(trimmedRest) + len(closeTag)
	}

	cleaned := raw[:idx] + raw[removeEnd:]
	return cleaned, strings.TrimSpace(obj), true
}

func (am *AgentManager) executeStructuredPlanningTask(ctx context.Context, task *Task, build *Build, agent *Agent) (*TaskOutput, error) {
	if build == nil {
		return nil, fmt.Errorf("build not found for structured planning task")
	}
	provider := agent.Provider
	if provider == "" {
		provider = ai.ProviderClaude
	}

	plannerAdapter := &plannerRouterAdapter{
		router:          am.aiRouter,
		provider:        provider,
		providers:       am.planningProviderOrder(build, task, provider),
		manager:         am,
		buildID:         build.ID,
		agentID:         agent.ID,
		taskID:          task.ID,
		userID:          build.UserID,
		powerMode:       build.PowerMode,
		usePlatformKeys: am.buildUsesPlatformKeys(build),
	}
	planner := autonomous.NewPlanner(plannerAdapter)

	bundle, err := planner.CreatePlanningBundle(ctx, planningDescriptionForBuild(build))
	if err != nil {
		return nil, err
	}

	plan := createBuildPlanFromPlanningBundle(build.ID, build.Description, build.TechStack, bundle)
	if plan == nil {
		return nil, fmt.Errorf("planner produced no build plan")
	}
	plan = applyBuildAssurancePolicyToPlan(build, plan)

	return &TaskOutput{
		Messages: []string{summarizeBuildPlan(plan)},
		Metrics: map[string]any{
			"spec_hash":         plan.SpecHash,
			"scaffold_id":       plan.ScaffoldID,
			"work_orders":       len(plan.WorkOrders),
			"planned_files":     len(plan.Files),
			"provider":          string(firstNonEmptyProvider(plannerAdapter.lastProvider, provider)),
			"selected_provider": string(firstNonEmptyProvider(plannerAdapter.lastProvider, provider)),
			"model":             firstNonEmptyString(plannerAdapter.lastModel, agent.Model),
		},
		Plan: plan,
	}, nil
}

func (am *AgentManager) planningProviderOrder(build *Build, task *Task, primary ai.AIProvider) []ai.AIProvider {
	providers := []ai.AIProvider{primary}
	if am == nil || build == nil {
		return compactPlanningProviders(primary, providers)
	}
	tried := map[ai.AIProvider]bool{}
	if primary != "" {
		tried[primary] = true
	}
	for _, provider := range am.rankedFallbackProvidersForTask(build, task, RoleLead, tried) {
		providers = append(providers, provider)
	}
	return compactPlanningProviders(primary, providers)
}

func planningDescriptionForBuild(build *Build) string {
	if build == nil {
		return ""
	}
	description := strings.TrimSpace(build.Description)
	if description == "" {
		return ""
	}
	if !buildRequiresStaticFrontendFallback(build) {
		return description
	}

	return fmt.Sprintf(`%s

APEX BUILD DELIVERY TARGET:
- This account is on the free/static tier.
- Plan and build the strongest truthful frontend-only app preview that matches the prompt.
- Do not require backend, database, auth, billing, jobs, or realtime implementation to succeed in this pass.
- Preserve the product shape by freezing deferred backend/data/runtime contracts in the architecture and UI states so a later paid pass can wire them in behind the same interface.`, description)
}

func extractTaskCheckins(response string) (string, *TaskStartAck, *TaskCompletionReport) {
	clean := response
	var startAck *TaskStartAck
	var completion *TaskCompletionReport

	if next, payload, ok := extractTaggedJSONObject(clean, taskStartAckOpenTag, taskStartAckCloseTag); ok {
		var parsed TaskStartAck
		if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
			startAck = &parsed
		}
		clean = next
	}

	if next, payload, ok := extractTaggedJSONObject(clean, taskCompletionReportOpenTag, taskCompletionReportCloseTag); ok {
		var parsed TaskCompletionReport
		if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
			completion = &parsed
		}
		clean = next
	}

	return strings.TrimSpace(clean), startAck, completion
}

func taskRequiresCoordinationCheckins(task *Task) bool {
	if task == nil || task.Input == nil {
		return false
	}
	required, _ := task.Input["require_checkins"].(bool)
	return required
}

func taskArtifactWorkOrderFromInput(task *Task) *WorkOrder {
	if task == nil || task.Input == nil {
		return nil
	}
	switch raw := task.Input["work_order_artifact"].(type) {
	case *WorkOrder:
		if raw == nil {
			return nil
		}
		return cloneWorkOrderArtifact(raw)
	case WorkOrder:
		return cloneWorkOrderArtifact(&raw)
	default:
		if raw == nil {
			return nil
		}
		payload, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var order WorkOrder
		if err := json.Unmarshal(payload, &order); err != nil {
			return nil
		}
		return cloneWorkOrderArtifact(&order)
	}
}

func taskWorkOrderFromInput(task *Task) *BuildWorkOrder {
	if task == nil || task.Input == nil {
		return nil
	}
	switch raw := task.Input["work_order"].(type) {
	case *BuildWorkOrder:
		if raw == nil {
			return nil
		}
		order := *raw
		return &order
	case BuildWorkOrder:
		order := raw
		return &order
	default:
		if raw != nil {
			payload, err := json.Marshal(raw)
			if err == nil {
				var order BuildWorkOrder
				if err := json.Unmarshal(payload, &order); err == nil {
					return &order
				}
			}
		}
		if artifact := taskArtifactWorkOrderFromInput(task); artifact != nil {
			return legacyBuildWorkOrderFromArtifact(artifact)
		}
		return nil
	}
}

func (am *AgentManager) validateTaskCoordinationOutput(task *Task, output *TaskOutput) []string {
	if !taskRequiresCoordinationCheckins(task) || output == nil {
		return nil
	}

	errs := make([]string, 0, 4)

	// Missing check-in XML blocks are logged as warnings but NOT treated as
	// retry-worthy errors. Most models (especially smaller ones like deepseek)
	// rarely emit the <task_start_ack>/<task_completion_report> blocks.
	// Burning retries on this was the #1 cause of build failures.
	if output.StartAck == nil {
		log.Printf("Info: task %s missing <task_start_ack> (non-fatal)", task.ID)
	} else if strings.TrimSpace(output.StartAck.Summary) == "" {
		log.Printf("Info: task %s has empty task_start_ack summary (non-fatal)", task.ID)
	}
	if output.Completion == nil {
		log.Printf("Info: task %s missing <task_completion_report> (non-fatal)", task.ID)
	} else if strings.TrimSpace(output.Completion.Summary) == "" {
		log.Printf("Info: task %s has empty task_completion_report summary (non-fatal)", task.ID)
	}

	// File ownership violations ARE hard errors — these protect build coherence.
	workOrder := taskWorkOrderFromInput(task)
	if workOrder == nil {
		return errs
	}

	for _, file := range output.Files {
		// Parser-assigned placeholder paths (generated_1.ts, generated_2.go, etc.) are
		// created when the LLM omits the "// File: path" marker in its response. These are
		// not intentional file paths — skip ownership validation so they don't cause
		// spurious coordination failures. They'll be discarded or renamed downstream.
		if isGeneratedArtifactPath(file.Path) {
			log.Printf("Info: skipping ownership check for parser placeholder %q", file.Path)
			continue
		}
		if !pathAllowedByWorkOrder(file.Path, workOrder) {
			errs = append(errs, fmt.Sprintf("file %s is outside work order ownership", file.Path))
		}
	}
	return errs
}

func summarizeTaskOutputForCoordination(output *TaskOutput, fallback string) string {
	if output == nil {
		return fallback
	}
	if output.Completion != nil && strings.TrimSpace(output.Completion.Summary) != "" {
		return strings.TrimSpace(output.Completion.Summary)
	}
	if output.StartAck != nil && strings.TrimSpace(output.StartAck.Summary) != "" {
		return strings.TrimSpace(output.StartAck.Summary)
	}
	if len(output.Messages) > 0 {
		return strings.TrimSpace(output.Messages[0])
	}
	return fallback
}

func pathAllowedByWorkOrder(path string, workOrder *BuildWorkOrder) bool {
	if workOrder == nil {
		return true
	}
	cleanPath := normalizeOwnedPath(path)
	if cleanPath == "" {
		return false
	}

	for _, owned := range workOrder.OwnedFiles {
		if pathMatchesOwnedPattern(cleanPath, owned) {
			return true
		}
	}
	for _, forbidden := range workOrder.ForbiddenFiles {
		if pathMatchesOwnedPattern(cleanPath, forbidden) {
			return false
		}
	}
	if len(workOrder.OwnedFiles) == 0 {
		return true
	}
	return false
}

func normalizeOwnedPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return path
}

func pathMatchesOwnedPattern(path string, pattern string) bool {
	pattern = normalizeOwnedPath(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "**" {
		return true
	}
	// Handle **/*.ext (e.g., **/*.test.ts) — suffix after **/*
	if strings.HasPrefix(pattern, "**/*.") {
		return strings.HasSuffix(path, strings.TrimPrefix(pattern, "**/*"))
	}
	// Handle **/*<suffix> (e.g., **/*_test.go) — use filepath.Match against basename
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if !strings.Contains(suffix, "*") {
			return strings.HasSuffix(filepath.Base(path), suffix) || path == suffix
		}
		// suffix still contains a wildcard (e.g. *_test.go) — match against basename
		if matched, err := filepath.Match(suffix, filepath.Base(path)); err == nil && matched {
			return true
		}
	}
	// Handle dir/** (e.g., src/**)
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	// Handle dir/**/*.ext (e.g., src/**/*.ts)
	if idx := strings.Index(pattern, "/**/"); idx >= 0 {
		prefix := pattern[:idx]
		suffix := pattern[idx+4:] // after "/**/"
		if !strings.HasPrefix(path, prefix+"/") {
			return false
		}
		rest := strings.TrimPrefix(path, prefix+"/")
		// suffix may itself be a glob like *.ts
		if strings.HasPrefix(suffix, "*.") {
			ext := strings.TrimPrefix(suffix, "*")
			return strings.HasSuffix(rest, ext)
		}
		if strings.HasPrefix(suffix, "*") {
			sfx := strings.TrimPrefix(suffix, "*")
			return strings.HasSuffix(rest, sfx)
		}
		// plain filename suffix
		return strings.HasSuffix(rest, suffix) || rest == suffix
	}
	// Handle *.ext at top level (e.g., *.ts)
	if strings.HasPrefix(pattern, "*.") {
		ext := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(path, ext) && !strings.Contains(path, "/")
	}
	return path == pattern
}
