package agents

import (
	"fmt"
	"strings"
)

type compiledWorkOrder struct {
	TaskShape        TaskShape
	RiskLevel        TaskRiskLevel
	OwnedFiles       []string
	ReadableFiles    []string
	ForbiddenFiles   []string
	MaxContextBudget int
	Scope            string
}

func compileTaskWorkOrder(task *Task) compiledWorkOrder {
	compiled := compiledWorkOrder{
		TaskShape: TaskShapeRepair,
		RiskLevel: RiskMedium,
		Scope:     "unknown",
	}
	if task == nil {
		return compiled
	}

	if artifact := taskArtifactWorkOrderFromInput(task); artifact != nil {
		if artifact.TaskShape != "" {
			compiled.TaskShape = artifact.TaskShape
		}
		if artifact.RiskLevel != "" {
			compiled.RiskLevel = artifact.RiskLevel
		}
		compiled.OwnedFiles = append([]string(nil), artifact.OwnedFiles...)
		compiled.ReadableFiles = append([]string(nil), artifact.ReadableFiles...)
		compiled.ForbiddenFiles = append([]string(nil), artifact.ForbiddenFiles...)
		compiled.MaxContextBudget = artifact.MaxContextBudget
		compiled.Scope = inferScopeFromPaths(append(append([]string{}, artifact.OwnedFiles...), artifact.ReadableFiles...))
		return compiled
	}

	switch task.Type {
	case TaskGenerateUI:
		compiled.TaskShape = TaskShapeFrontendPatch
	case TaskGenerateAPI:
		compiled.TaskShape = TaskShapeBackendPatch
	case TaskGenerateSchema:
		compiled.TaskShape = TaskShapeSchema
	case TaskArchitecture, TaskPlan:
		compiled.TaskShape = TaskShapeContract
	case TaskReview, TaskTest:
		compiled.TaskShape = TaskShapeVerification
	case TaskDeploy:
		compiled.TaskShape = TaskShapeIntegration
	case TaskFix:
		compiled.TaskShape = TaskShapeRepair
	default:
		compiled.TaskShape = TaskShapeRepair
	}

	if task.Type == TaskArchitecture || task.Type == TaskDeploy {
		compiled.RiskLevel = RiskHigh
	}
	if task.RetryCount >= 2 && compiled.RiskLevel != RiskCritical {
		compiled.RiskLevel = RiskHigh
	}
	compiled.Scope = inferScopeFromTaskInput(task.Input)
	return compiled
}

func inferScopeFromTaskInput(input map[string]any) string {
	if len(input) == 0 {
		return "unknown"
	}
	paths := make([]string, 0, 16)
	for _, key := range []string{"owned_files", "readable_files", "required_files", "affected_files"} {
		raw, ok := input[key]
		if !ok {
			continue
		}
		switch typed := raw.(type) {
		case []string:
			paths = append(paths, typed...)
		case []any:
			for _, value := range typed {
				paths = append(paths, strings.TrimSpace(fmt.Sprintf("%v", value)))
			}
		}
	}
	return inferScopeFromPaths(paths)
}

func inferScopeFromPaths(paths []string) string {
	hasFrontend := false
	hasBackend := false
	for _, path := range paths {
		normalized := strings.ToLower(strings.TrimSpace(path))
		if normalized == "" {
			continue
		}
		switch {
		case strings.HasPrefix(normalized, "src/"), strings.HasPrefix(normalized, "frontend/"):
			hasFrontend = true
		case strings.HasPrefix(normalized, "backend/"), strings.HasPrefix(normalized, "api/"), strings.HasPrefix(normalized, "server/"), strings.HasPrefix(normalized, "internal/"):
			hasBackend = true
		}
	}
	switch {
	case hasFrontend && hasBackend:
		return "cross_surface"
	case hasFrontend || hasBackend:
		return "local"
	default:
		return "unknown"
	}
}
