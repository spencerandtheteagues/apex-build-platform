package agents

type TaskTriageResult struct {
	TaskShape    TaskShape
	RiskLevel    TaskRiskLevel
	Scope        string
	LocalRepair  bool
	CrossSurface bool
}

func triageTaskForWaterfall(task *Task) TaskTriageResult {
	compiled := compileTaskWorkOrder(task)
	result := TaskTriageResult{
		TaskShape: compiled.TaskShape,
		RiskLevel: compiled.RiskLevel,
		Scope:     compiled.Scope,
	}

	if result.TaskShape == "" {
		result.TaskShape = TaskShapeRepair
	}
	if result.RiskLevel == "" {
		result.RiskLevel = RiskMedium
	}
	result.CrossSurface = result.Scope == "cross_surface"
	result.LocalRepair = result.Scope == "local" && (result.TaskShape == TaskShapeRepair || result.TaskShape == TaskShapeFrontendPatch || result.TaskShape == TaskShapeBackendPatch)
	return result
}
