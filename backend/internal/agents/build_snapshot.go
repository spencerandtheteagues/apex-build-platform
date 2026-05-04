package agents

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"apex-build/internal/ai"
)

type buildAgentTaskSnapshot struct {
	ID          string   `json:"id"`
	Type        TaskType `json:"type"`
	Description string   `json:"description"`
}

type buildAgentSnapshot struct {
	ID          string                  `json:"id"`
	Role        AgentRole               `json:"role"`
	Provider    ai.AIProvider           `json:"provider"`
	Model       string                  `json:"model,omitempty"`
	Status      AgentStatus             `json:"status"`
	BuildID     string                  `json:"build_id"`
	CurrentTask *buildAgentTaskSnapshot `json:"current_task,omitempty"`
	Progress    int                     `json:"progress"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	Error       string                  `json:"error,omitempty"`
}

type buildTaskSnapshot struct {
	ID            string         `json:"id"`
	Type          TaskType       `json:"type"`
	Description   string         `json:"description"`
	Priority      int            `json:"priority"`
	Dependencies  []string       `json:"dependencies,omitempty"`
	AssignedTo    string         `json:"assigned_to,omitempty"`
	Status        TaskStatus     `json:"status"`
	Input         map[string]any `json:"input,omitempty"`
	Output        *TaskOutput    `json:"output,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	Error         string         `json:"error,omitempty"`
	RetryCount    int            `json:"retry_count,omitempty"`
	MaxRetries    int            `json:"max_retries,omitempty"`
	ErrorHistory  []ErrorAttempt `json:"error_history,omitempty"`
	RetryStrategy RetryStrategy  `json:"retry_strategy,omitempty"`
}

type buildCheckpointSnapshot struct {
	ID          string    `json:"id"`
	BuildID     string    `json:"build_id"`
	Number      int       `json:"number"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Progress    int       `json:"progress"`
	Restorable  bool      `json:"restorable"`
	CreatedAt   time.Time `json:"created_at"`
}

func copyBuildSnapshotStateLocked(build *Build) BuildSnapshotState {
	if build == nil {
		return BuildSnapshotState{}
	}
	return cloneBuildSnapshotState(build.SnapshotState)
}

func snapshotStateForPersistenceLocked(build *Build) BuildSnapshotState {
	state := copyBuildSnapshotStateLocked(build)
	if build == nil {
		return state
	}
	state.RestoreContext = &BuildRestoreContext{
		SubscriptionPlan:            build.SubscriptionPlan,
		ProviderMode:                build.ProviderMode,
		RequirePreviewReady:         build.RequirePreviewReady,
		RequestsUsed:                build.RequestsUsed,
		ReadinessRecoveryAttempts:   build.ReadinessRecoveryAttempts,
		PreviewVerificationAttempts: build.PreviewVerificationAttempts,
		CompileValidationPassed:     build.CompileValidationPassed,
		CompileValidationAttempts:   build.CompileValidationAttempts,
		CompileValidationRepairs:    build.CompileValidationRepairs,
		CompileValidationStartedAt:  cloneTimePtr(build.CompileValidationStartedAt),
		MaxAgents:                   build.MaxAgents,
		MaxRetries:                  build.MaxRetries,
		MaxRequests:                 build.MaxRequests,
		MaxTokensPerRequest:         build.MaxTokensPerRequest,
		PhasedPipelineComplete:      build.PhasedPipelineComplete,
		DiffMode:                    build.DiffMode,
		RoleAssignments:             cloneStringMap(build.RoleAssignments),
		ProviderModelOverrides:      normalizeProviderModelOverridesMap(build.ProviderModelOverrides),
		TechStack:                   cloneTechStack(build.TechStack),
		Plan:                        cloneBuildPlan(build.Plan),
	}
	return state
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func copyBuildOrchestrationStateLocked(build *Build) *BuildOrchestrationState {
	if build == nil {
		return nil
	}
	return cloneBuildOrchestrationState(build.SnapshotState.Orchestration)
}

func orderedBuildAgents(agents map[string]*Agent) []*Agent {
	if len(agents) == 0 {
		return []*Agent{}
	}

	ordered := make([]*Agent, 0, len(agents))
	for _, agent := range agents {
		if agent != nil {
			ordered = append(ordered, agent)
		}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		if left.Role != right.Role {
			return left.Role < right.Role
		}
		return left.ID < right.ID
	})

	return ordered
}

func buildAgentCurrentTaskSnapshotLocked(build *Build, agent *Agent) *buildAgentTaskSnapshot {
	if agent == nil {
		return nil
	}
	if agent.CurrentTask != nil {
		return &buildAgentTaskSnapshot{
			ID:          agent.CurrentTask.ID,
			Type:        agent.CurrentTask.Type,
			Description: agent.CurrentTask.Description,
		}
	}
	if build == nil {
		return nil
	}
	for _, task := range build.Tasks {
		if task == nil || task.AssignedTo != agent.ID || task.Status != TaskInProgress {
			continue
		}
		return &buildAgentTaskSnapshot{
			ID:          task.ID,
			Type:        task.Type,
			Description: task.Description,
		}
	}
	return nil
}

func copyBuildAgentSnapshotsLocked(build *Build) []buildAgentSnapshot {
	if build == nil || len(build.Agents) == 0 {
		return []buildAgentSnapshot{}
	}

	ordered := orderedBuildAgents(build.Agents)
	snapshots := make([]buildAgentSnapshot, 0, len(ordered))
	for _, agent := range ordered {
		snapshots = append(snapshots, buildAgentSnapshot{
			ID:          agent.ID,
			Role:        agent.Role,
			Provider:    agent.Provider,
			Model:       agent.Model,
			Status:      agent.Status,
			BuildID:     agent.BuildID,
			CurrentTask: buildAgentCurrentTaskSnapshotLocked(build, agent),
			Progress:    agent.Progress,
			CreatedAt:   agent.CreatedAt,
			UpdatedAt:   agent.UpdatedAt,
			Error:       agent.Error,
		})
	}
	return snapshots
}

func copyBuildTaskSnapshotsLocked(build *Build) []buildTaskSnapshot {
	if build == nil || len(build.Tasks) == 0 {
		return []buildTaskSnapshot{}
	}

	snapshots := make([]buildTaskSnapshot, 0, len(build.Tasks))
	for _, task := range build.Tasks {
		if task == nil || strings.HasPrefix(task.ID, "snapshot-files-") {
			continue
		}
		dependencies := append([]string(nil), task.Dependencies...)
		snapshots = append(snapshots, buildTaskSnapshot{
			ID:            task.ID,
			Type:          task.Type,
			Description:   task.Description,
			Priority:      task.Priority,
			Dependencies:  dependencies,
			AssignedTo:    task.AssignedTo,
			Status:        task.Status,
			Input:         cloneTaskInputForSnapshot(task),
			Output:        cloneTaskOutput(task.Output),
			CreatedAt:     task.CreatedAt,
			StartedAt:     task.StartedAt,
			CompletedAt:   task.CompletedAt,
			Error:         task.Error,
			RetryCount:    task.RetryCount,
			MaxRetries:    task.MaxRetries,
			ErrorHistory:  cloneErrorAttempts(task.ErrorHistory),
			RetryStrategy: task.RetryStrategy,
		})
	}
	return snapshots
}

func copyBuildCheckpointSnapshotsLocked(build *Build) []buildCheckpointSnapshot {
	if build == nil || len(build.Checkpoints) == 0 {
		return []buildCheckpointSnapshot{}
	}

	snapshots := make([]buildCheckpointSnapshot, 0, len(build.Checkpoints))
	for _, checkpoint := range build.Checkpoints {
		if checkpoint == nil {
			continue
		}
		snapshots = append(snapshots, buildCheckpointSnapshot{
			ID:          checkpoint.ID,
			BuildID:     checkpoint.BuildID,
			Number:      checkpoint.Number,
			Name:        checkpoint.Name,
			Description: checkpoint.Description,
			Progress:    checkpoint.Progress,
			Restorable:  false,
			CreatedAt:   checkpoint.CreatedAt,
		})
	}
	return snapshots
}

func parseBuildAgents(raw string) map[string]*Agent {
	agents := make(map[string]*Agent)
	if strings.TrimSpace(raw) == "" {
		return agents
	}

	var snapshots []buildAgentSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshots); err != nil {
		return agents
	}

	for _, snapshot := range snapshots {
		id := strings.TrimSpace(snapshot.ID)
		if id == "" {
			continue
		}
		agent := &Agent{
			ID:        id,
			Role:      snapshot.Role,
			Provider:  snapshot.Provider,
			Model:     snapshot.Model,
			Status:    snapshot.Status,
			BuildID:   snapshot.BuildID,
			Progress:  snapshot.Progress,
			CreatedAt: snapshot.CreatedAt,
			UpdatedAt: snapshot.UpdatedAt,
			Error:     snapshot.Error,
		}
		if snapshot.CurrentTask != nil {
			agent.CurrentTask = &Task{
				ID:          snapshot.CurrentTask.ID,
				Type:        snapshot.CurrentTask.Type,
				Description: snapshot.CurrentTask.Description,
			}
		}
		agents[id] = agent
	}

	return agents
}

func parseBuildTasks(raw string) []*Task {
	if strings.TrimSpace(raw) == "" {
		return []*Task{}
	}

	var snapshots []buildTaskSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshots); err != nil {
		return []*Task{}
	}

	tasks := make([]*Task, 0, len(snapshots))
	for _, snapshot := range snapshots {
		id := strings.TrimSpace(snapshot.ID)
		if id == "" {
			continue
		}
		dependencies := append([]string(nil), snapshot.Dependencies...)
		tasks = append(tasks, &Task{
			ID:            id,
			Type:          snapshot.Type,
			Description:   snapshot.Description,
			Priority:      snapshot.Priority,
			Dependencies:  dependencies,
			AssignedTo:    snapshot.AssignedTo,
			Status:        snapshot.Status,
			Input:         cloneTaskInput(snapshot.Input),
			Output:        cloneTaskOutput(snapshot.Output),
			CreatedAt:     snapshot.CreatedAt,
			StartedAt:     snapshot.StartedAt,
			CompletedAt:   snapshot.CompletedAt,
			Error:         snapshot.Error,
			RetryCount:    snapshot.RetryCount,
			MaxRetries:    snapshot.MaxRetries,
			ErrorHistory:  cloneErrorAttempts(snapshot.ErrorHistory),
			RetryStrategy: snapshot.RetryStrategy,
		})
	}

	return tasks
}

func parseBuildCheckpoints(raw string) []*Checkpoint {
	if strings.TrimSpace(raw) == "" {
		return []*Checkpoint{}
	}

	var snapshots []buildCheckpointSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshots); err != nil {
		return []*Checkpoint{}
	}

	checkpoints := make([]*Checkpoint, 0, len(snapshots))
	for _, snapshot := range snapshots {
		id := strings.TrimSpace(snapshot.ID)
		if id == "" {
			continue
		}
		checkpoints = append(checkpoints, &Checkpoint{
			ID:          id,
			BuildID:     snapshot.BuildID,
			Number:      snapshot.Number,
			Name:        snapshot.Name,
			Description: snapshot.Description,
			Progress:    snapshot.Progress,
			Restorable:  snapshot.Restorable,
			CreatedAt:   snapshot.CreatedAt,
		})
	}

	return checkpoints
}

func parseBuildSnapshotState(raw string) BuildSnapshotState {
	if strings.TrimSpace(raw) == "" {
		return BuildSnapshotState{}
	}

	var state BuildSnapshotState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return BuildSnapshotState{}
	}
	return cloneBuildSnapshotState(state)
}

func cloneBuildSnapshotState(state BuildSnapshotState) BuildSnapshotState {
	encoded, err := json.Marshal(state)
	if err != nil {
		clone := state
		if clone.AvailableProviders != nil {
			clone.AvailableProviders = append([]string(nil), clone.AvailableProviders...)
		}
		clone.Orchestration = cloneBuildOrchestrationState(clone.Orchestration)
		return clone
	}

	var decoded BuildSnapshotState
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		clone := state
		if clone.AvailableProviders != nil {
			clone.AvailableProviders = append([]string(nil), clone.AvailableProviders...)
		}
		clone.Orchestration = cloneBuildOrchestrationState(clone.Orchestration)
		return clone
	}
	return decoded
}

func cloneBuildOrchestrationState(state *BuildOrchestrationState) *BuildOrchestrationState {
	if state == nil {
		return nil
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		clone := *state
		return &clone
	}

	var decoded BuildOrchestrationState
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		clone := *state
		return &clone
	}
	return &decoded
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneTechStack(stack *TechStack) *TechStack {
	if stack == nil {
		return nil
	}
	cloned := *stack
	if stack.Extras != nil {
		cloned.Extras = append([]string(nil), stack.Extras...)
	}
	return &cloned
}

func cloneBuildPlan(plan *BuildPlan) *BuildPlan {
	if plan == nil {
		return nil
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		cloned := *plan
		return &cloned
	}
	var decoded BuildPlan
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		cloned := *plan
		return &cloned
	}
	return &decoded
}

func cloneTaskInput(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		cloned := make(map[string]any, len(input))
		for key, value := range input {
			cloned[key] = value
		}
		return cloned
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		cloned := make(map[string]any, len(input))
		for key, value := range input {
			cloned[key] = value
		}
		return cloned
	}
	return decoded
}

func cloneTaskInputForSnapshot(task *Task) map[string]any {
	if task == nil {
		return nil
	}
	task.mu.RLock()
	defer task.mu.RUnlock()
	return cloneTaskInput(task.Input)
}

func cloneTaskOutput(output *TaskOutput) *TaskOutput {
	if output == nil {
		return nil
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		cloned := *output
		return &cloned
	}
	var decoded TaskOutput
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		cloned := *output
		return &cloned
	}
	return &decoded
}

func cloneErrorAttempts(history []ErrorAttempt) []ErrorAttempt {
	if len(history) == 0 {
		return nil
	}
	cloned := make([]ErrorAttempt, len(history))
	copy(cloned, history)
	return cloned
}
