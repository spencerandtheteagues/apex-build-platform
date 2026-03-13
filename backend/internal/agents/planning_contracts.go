package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"apex-build/internal/agents/autonomous"
	"apex-build/internal/ai"
)

var (
	taskStartAckPattern         = regexp.MustCompile(`(?s)<task_start_ack>\s*(\{.*?\})\s*</task_start_ack>`)
	taskCompletionReportPattern = regexp.MustCompile(`(?s)<task_completion_report>\s*(\{.*?\})\s*</task_completion_report>`)
)

func (am *AgentManager) executeStructuredPlanningTask(ctx context.Context, task *Task, build *Build, agent *Agent) (*TaskOutput, error) {
	if build == nil {
		return nil, fmt.Errorf("build not found for structured planning task")
	}
	provider := agent.Provider
	if provider == "" {
		provider = ai.ProviderClaude
	}

	planner := autonomous.NewPlanner(&plannerRouterAdapter{
		router:          am.aiRouter,
		provider:        provider,
		userID:          build.UserID,
		powerMode:       build.PowerMode,
		usePlatformKeys: am.buildUsesPlatformKeys(build),
	})

	bundle, err := planner.CreatePlanningBundle(ctx, build.Description)
	if err != nil {
		return nil, err
	}

	plan := createBuildPlanFromPlanningBundle(build.ID, build.Description, build.TechStack, bundle)
	if plan == nil {
		return nil, fmt.Errorf("planner produced no build plan")
	}

	return &TaskOutput{
		Messages: []string{summarizeBuildPlan(plan)},
		Metrics: map[string]any{
			"spec_hash":     plan.SpecHash,
			"scaffold_id":   plan.ScaffoldID,
			"work_orders":   len(plan.WorkOrders),
			"planned_files": len(plan.Files),
		},
		Plan: plan,
	}, nil
}

func extractTaskCheckins(response string) (string, *TaskStartAck, *TaskCompletionReport) {
	clean := response
	var startAck *TaskStartAck
	var completion *TaskCompletionReport

	if match := taskStartAckPattern.FindStringSubmatch(clean); len(match) == 2 {
		var parsed TaskStartAck
		if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &parsed); err == nil {
			startAck = &parsed
		}
		clean = strings.Replace(clean, match[0], "", 1)
	}

	if match := taskCompletionReportPattern.FindStringSubmatch(clean); len(match) == 2 {
		var parsed TaskCompletionReport
		if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &parsed); err == nil {
			completion = &parsed
		}
		clean = strings.Replace(clean, match[0], "", 1)
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

	for _, forbidden := range workOrder.ForbiddenFiles {
		if pathMatchesOwnedPattern(cleanPath, forbidden) {
			return false
		}
	}
	if len(workOrder.OwnedFiles) == 0 {
		return true
	}
	for _, owned := range workOrder.OwnedFiles {
		if pathMatchesOwnedPattern(cleanPath, owned) {
			return true
		}
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
	// Handle **/*<suffix> (e.g., **/*_test.go) — suffix after **/*
	if strings.HasPrefix(pattern, "**/*") {
		suffix := strings.TrimPrefix(pattern, "**/*")
		if suffix != "" {
			return strings.HasSuffix(path, suffix)
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
