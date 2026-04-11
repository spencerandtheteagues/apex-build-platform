package agents

import "apex-build/internal/ai"

const (
	routingWaterfallStageCheap     = "cheap"
	routingWaterfallStageMedium    = "medium"
	routingWaterfallStageExpensive = "expensive"
)

type RoutingWaterfallDecision struct {
	Stage        string
	PowerMode    PowerMode
	Model        string
	Reason       string
	Triage       TaskTriageResult
	UsedFallback bool
}

func planRoutingWaterfall(build *Build, task *Task, provider ai.AIProvider) RoutingWaterfallDecision {
	triage := triageTaskForWaterfall(task)

	stage := routingWaterfallStageMedium
	reason := "bounded_generation"

	if triage.RiskLevel == RiskHigh || triage.RiskLevel == RiskCritical {
		if triage.CrossSurface || task != nil && (task.Type == TaskArchitecture || task.Type == TaskDeploy || task.Type == TaskGenerateSchema) {
			stage = routingWaterfallStageExpensive
			reason = "high_risk_cross_surface"
		}
	}
	if task != nil && task.RetryCount >= 2 && (task.Type == TaskFix || task.Type == TaskGenerateFile || task.Type == TaskGenerateAPI || task.Type == TaskGenerateUI) {
		stage = routingWaterfallStageExpensive
		reason = "repeated_repair_failure"
	}

	if stage != routingWaterfallStageExpensive {
		switch triage.TaskShape {
		case TaskShapeAdversarialCritique, TaskShapeVerification, TaskShapeIntentNormalization, TaskShapeContract:
			stage = routingWaterfallStageCheap
			reason = "cheap_triage_or_critique"
		}
		if task != nil && (task.Type == TaskReview || task.Type == TaskTest) {
			stage = routingWaterfallStageCheap
			reason = "verification_first"
		}
	}

	targetMode := PowerBalanced
	switch stage {
	case routingWaterfallStageCheap:
		targetMode = PowerFast
	case routingWaterfallStageExpensive:
		targetMode = PowerMax
	default:
		targetMode = PowerBalanced
	}

	maxMode := PowerBalanced
	if build != nil && build.PowerMode != "" {
		maxMode = build.PowerMode
	}
	if modeRank(targetMode) > modeRank(maxMode) {
		targetMode = maxMode
		reason += "_capped_by_build_power"
	}

	model := selectModelForPowerMode(provider, targetMode)
	return RoutingWaterfallDecision{
		Stage:        stage,
		PowerMode:    targetMode,
		Model:        model,
		Reason:       reason,
		Triage:       triage,
		UsedFallback: model == "",
	}
}

func modeRank(mode PowerMode) int {
	switch mode {
	case PowerFast:
		return 0
	case PowerBalanced:
		return 1
	case PowerMax:
		return 2
	default:
		return 0
	}
}
