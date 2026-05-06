package agents

import (
	"time"

	"apex-build/internal/ai"
	"apex-build/internal/architecture"
)

func (am *AgentManager) recordArchitectureReferences(
	build *Build,
	agent *Agent,
	task *Task,
	provider ai.AIProvider,
	model string,
	texts ...string,
) {
	if build == nil || task == nil || agent == nil {
		return
	}
	event := architecture.CollectReferenceEvent(architecture.ReferenceInput{
		BuildID:   build.ID,
		TaskID:    task.ID,
		TaskType:  string(task.Type),
		AgentRole: string(agent.Role),
		Provider:  string(provider),
		Model:     model,
		Timestamp: time.Now().UTC(),
		Texts:     texts,
	})
	if len(event.Hits) == 0 {
		return
	}

	build.mu.Lock()
	build.SnapshotState.ArchitectureReferences = architecture.MergeReferenceTelemetry(build.SnapshotState.ArchitectureReferences, event)
	build.mu.Unlock()
}

func (am *AgentManager) ArchitectureReferenceTelemetrySnapshot() *architecture.ReferenceTelemetry {
	if am == nil {
		return nil
	}
	am.mu.RLock()
	builds := make([]*Build, 0, len(am.builds))
	for _, build := range am.builds {
		if build != nil {
			builds = append(builds, build)
		}
	}
	am.mu.RUnlock()

	var out *architecture.ReferenceTelemetry
	for _, build := range builds {
		build.mu.RLock()
		refs := architecture.CloneReferenceTelemetry(build.SnapshotState.ArchitectureReferences)
		build.mu.RUnlock()
		out = architecture.MergeTelemetry(out, refs)
	}
	return out
}
