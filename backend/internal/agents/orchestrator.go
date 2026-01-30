// Package agents - Build Orchestrator
// High-level orchestration of the build process across multiple AI agents.
// This provides the coordination layer that manages the entire build lifecycle.
package agents

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BuildOrchestrator coordinates the overall build process
type BuildOrchestrator struct {
	manager     *AgentManager
	hub         *WSHub
	activeBuild map[string]*OrchestrationState
	mu          sync.RWMutex
}

// OrchestrationState tracks the orchestration state for a build
type OrchestrationState struct {
	BuildID       string
	Phase         BuildPhase
	StartTime     time.Time
	PhaseStarted  time.Time
	TotalAgents   int
	ActiveAgents  int
	CompletedTasks int
	TotalTasks    int
	Errors        []string
	ctx           context.Context
	cancel        context.CancelFunc
}

// BuildPhase represents the current phase of the build
type BuildPhase string

const (
	PhaseInitializing   BuildPhase = "initializing"
	PhasePlanning       BuildPhase = "planning"
	PhaseArchitecting   BuildPhase = "architecting"
	PhaseGenerating     BuildPhase = "generating"
	PhaseTesting        BuildPhase = "testing"
	PhaseReviewing      BuildPhase = "reviewing"
	PhaseCompleting     BuildPhase = "completing"
	PhaseComplete       BuildPhase = "complete"
	PhaseFailed         BuildPhase = "failed"
)

// NewBuildOrchestrator creates a new build orchestrator
func NewBuildOrchestrator(manager *AgentManager, hub *WSHub) *BuildOrchestrator {
	return &BuildOrchestrator{
		manager:     manager,
		hub:         hub,
		activeBuild: make(map[string]*OrchestrationState),
	}
}

// StartOrchestration begins orchestrating a build
func (o *BuildOrchestrator) StartOrchestration(buildID string) error {
	log.Printf("Orchestrator: Starting orchestration for build %s", buildID)

	// Get the build
	build, err := o.manager.GetBuild(buildID)
	if err != nil {
		return fmt.Errorf("build not found: %w", err)
	}

	// Create orchestration context with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

	// Initialize orchestration state
	state := &OrchestrationState{
		BuildID:      buildID,
		Phase:        PhaseInitializing,
		StartTime:    time.Now(),
		PhaseStarted: time.Now(),
		ctx:          ctx,
		cancel:       cancel,
	}

	o.mu.Lock()
	o.activeBuild[buildID] = state
	o.mu.Unlock()

	// Broadcast orchestration started
	o.broadcastPhase(buildID, state.Phase, "Build orchestration started")

	// Run the orchestration pipeline
	go o.runPipeline(build, state)

	return nil
}

// runPipeline executes the build pipeline
func (o *BuildOrchestrator) runPipeline(build *Build, state *OrchestrationState) {
	defer func() {
		state.cancel()
		o.mu.Lock()
		delete(o.activeBuild, state.BuildID)
		o.mu.Unlock()
	}()

	// Phase 1: Planning
	if err := o.executePlanningPhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhasePlanning, err)
		return
	}

	// Phase 2: Architecture
	if err := o.executeArchitecturePhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseArchitecting, err)
		return
	}

	// Phase 3: Code Generation
	if err := o.executeGenerationPhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseGenerating, err)
		return
	}

	// Phase 4: Testing
	if err := o.executeTestingPhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseTesting, err)
		return
	}

	// Phase 5: Review
	if err := o.executeReviewPhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseReviewing, err)
		return
	}

	// Complete
	o.completeOrchestration(build, state)
}

// executePlanningPhase runs the planning phase
func (o *BuildOrchestrator) executePlanningPhase(build *Build, state *OrchestrationState) error {
	state.Phase = PhasePlanning
	state.PhaseStarted = time.Now()
	o.broadcastPhase(build.ID, state.Phase, "Creating build plan...")

	// Wait for planning task to complete
	return o.waitForTasksOfType(build, state, TaskPlan, 5*time.Minute)
}

// executeArchitecturePhase runs the architecture phase
func (o *BuildOrchestrator) executeArchitecturePhase(build *Build, state *OrchestrationState) error {
	state.Phase = PhaseArchitecting
	state.PhaseStarted = time.Now()
	o.broadcastPhase(build.ID, state.Phase, "Designing system architecture...")

	return o.waitForTasksOfType(build, state, TaskArchitecture, 5*time.Minute)
}

// executeGenerationPhase runs the code generation phase
func (o *BuildOrchestrator) executeGenerationPhase(build *Build, state *OrchestrationState) error {
	state.Phase = PhaseGenerating
	state.PhaseStarted = time.Now()
	o.broadcastPhase(build.ID, state.Phase, "Generating code across all components...")

	// Wait for all generation tasks
	generationTypes := []TaskType{TaskGenerateFile, TaskGenerateAPI, TaskGenerateUI, TaskGenerateSchema}
	for _, taskType := range generationTypes {
		if err := o.waitForTasksOfType(build, state, taskType, 10*time.Minute); err != nil {
			// Log but continue - some task types may not exist
			log.Printf("Orchestrator: No tasks of type %s or error: %v", taskType, err)
		}
	}

	return nil
}

// executeTestingPhase runs the testing phase
func (o *BuildOrchestrator) executeTestingPhase(build *Build, state *OrchestrationState) error {
	state.Phase = PhaseTesting
	state.PhaseStarted = time.Now()
	o.broadcastPhase(build.ID, state.Phase, "Running tests...")

	return o.waitForTasksOfType(build, state, TaskTest, 5*time.Minute)
}

// executeReviewPhase runs the code review phase
func (o *BuildOrchestrator) executeReviewPhase(build *Build, state *OrchestrationState) error {
	state.Phase = PhaseReviewing
	state.PhaseStarted = time.Now()
	o.broadcastPhase(build.ID, state.Phase, "Reviewing generated code...")

	return o.waitForTasksOfType(build, state, TaskReview, 5*time.Minute)
}

// waitForTasksOfType waits for all tasks of a specific type to complete
func (o *BuildOrchestrator) waitForTasksOfType(build *Build, state *OrchestrationState, taskType TaskType, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(state.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			build.mu.RLock()
			allComplete := true
			anyFailed := false
			foundTasks := false

			for _, task := range build.Tasks {
				if task.Type == taskType {
					foundTasks = true
					if task.Status == TaskFailed {
						anyFailed = true
					}
					if task.Status != TaskCompleted && task.Status != TaskFailed && task.Status != TaskCancelled {
						allComplete = false
					}
				}
			}
			build.mu.RUnlock()

			if !foundTasks {
				// No tasks of this type, skip
				return nil
			}

			if anyFailed {
				return fmt.Errorf("task of type %s failed", taskType)
			}

			if allComplete {
				return nil
			}
		}
	}
}

// completeOrchestration marks the build as complete
func (o *BuildOrchestrator) completeOrchestration(build *Build, state *OrchestrationState) {
	state.Phase = PhaseComplete
	o.broadcastPhase(build.ID, state.Phase, "Build completed successfully!")

	build.mu.Lock()
	build.Status = BuildCompleted
	build.Progress = 100
	now := time.Now()
	build.CompletedAt = &now
	build.UpdatedAt = now
	build.mu.Unlock()

	// Broadcast completion
	o.hub.Broadcast(build.ID, &WSMessage{
		Type:      WSBuildCompleted,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":      string(BuildCompleted),
			"progress":    100,
			"duration_ms": time.Since(state.StartTime).Milliseconds(),
			"message":     "Build completed successfully!",
		},
	})

	log.Printf("Orchestrator: Build %s completed in %v", build.ID, time.Since(state.StartTime))
}

// handlePhaseError handles errors during a phase
func (o *BuildOrchestrator) handlePhaseError(build *Build, state *OrchestrationState, phase BuildPhase, err error) {
	state.Phase = PhaseFailed
	state.Errors = append(state.Errors, fmt.Sprintf("Phase %s failed: %v", phase, err))

	log.Printf("Orchestrator: Build %s failed in phase %s: %v", build.ID, phase, err)

	build.mu.Lock()
	build.Status = BuildFailed
	build.Error = fmt.Sprintf("Build failed in %s phase: %v", phase, err)
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	// Broadcast failure
	o.hub.Broadcast(build.ID, &WSMessage{
		Type:      WSBuildError,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":  string(BuildFailed),
			"phase":   string(phase),
			"error":   err.Error(),
			"message": fmt.Sprintf("Build failed during %s phase", phase),
		},
	})
}

// broadcastPhase broadcasts a phase change
func (o *BuildOrchestrator) broadcastPhase(buildID string, phase BuildPhase, message string) {
	o.hub.Broadcast(buildID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":   string(phase),
			"message": message,
		},
	})
}

// CancelOrchestration cancels an active orchestration
func (o *BuildOrchestrator) CancelOrchestration(buildID string) error {
	o.mu.Lock()
	state, exists := o.activeBuild[buildID]
	o.mu.Unlock()

	if !exists {
		return fmt.Errorf("no active orchestration for build %s", buildID)
	}

	state.cancel()
	return nil
}

// GetOrchestrationState returns the current orchestration state
func (o *BuildOrchestrator) GetOrchestrationState(buildID string) (*OrchestrationState, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	state, exists := o.activeBuild[buildID]
	if !exists {
		return nil, fmt.Errorf("no active orchestration for build %s", buildID)
	}

	return state, nil
}

// CreateFileGenerationTask creates a task to generate a specific file
func (o *BuildOrchestrator) CreateFileGenerationTask(buildID string, filePath string, description string, language string) (*Task, error) {
	build, err := o.manager.GetBuild(buildID)
	if err != nil {
		return nil, err
	}

	// Find an appropriate agent for this file type
	var agent *Agent
	build.mu.RLock()
	for _, a := range build.Agents {
		switch language {
		case "typescript", "javascript", "html", "css":
			if a.Role == RoleFrontend {
				agent = a
				break
			}
		case "go", "python", "java", "rust":
			if a.Role == RoleBackend {
				agent = a
				break
			}
		case "sql":
			if a.Role == RoleDatabase {
				agent = a
				break
			}
		}
	}
	build.mu.RUnlock()

	if agent == nil {
		return nil, fmt.Errorf("no suitable agent found for language %s", language)
	}

	task := &Task{
		ID:          uuid.New().String(),
		Type:        TaskGenerateFile,
		Description: fmt.Sprintf("Generate %s: %s", filePath, description),
		Priority:    50,
		Status:      TaskPending,
		MaxRetries:  5,
		Input: map[string]any{
			"file_path":   filePath,
			"description": description,
			"language":    language,
		},
		CreatedAt: time.Now(),
	}

	build.mu.Lock()
	build.Tasks = append(build.Tasks, task)
	build.mu.Unlock()

	if err := o.manager.AssignTask(agent.ID, task); err != nil {
		return nil, err
	}

	return task, nil
}

// GetBuildProgress returns detailed progress information
func (o *BuildOrchestrator) GetBuildProgress(buildID string) (map[string]any, error) {
	build, err := o.manager.GetBuild(buildID)
	if err != nil {
		return nil, err
	}

	o.mu.RLock()
	state := o.activeBuild[buildID]
	o.mu.RUnlock()

	build.mu.RLock()
	defer build.mu.RUnlock()

	// Count tasks by status
	taskCounts := map[TaskStatus]int{}
	for _, task := range build.Tasks {
		taskCounts[task.Status]++
	}

	// Count agents by status
	agentCounts := map[AgentStatus]int{}
	for _, agent := range build.Agents {
		agent.mu.RLock()
		agentCounts[agent.Status]++
		agent.mu.RUnlock()
	}

	// Collect generated files
	fileCount := 0
	totalSize := int64(0)
	for _, task := range build.Tasks {
		if task.Output != nil {
			for _, file := range task.Output.Files {
				fileCount++
				totalSize += file.Size
			}
		}
	}

	progress := map[string]any{
		"build_id":        buildID,
		"status":          string(build.Status),
		"progress":        build.Progress,
		"tasks_pending":   taskCounts[TaskPending],
		"tasks_running":   taskCounts[TaskInProgress],
		"tasks_completed": taskCounts[TaskCompleted],
		"tasks_failed":    taskCounts[TaskFailed],
		"tasks_total":     len(build.Tasks),
		"agents_idle":     agentCounts[StatusIdle],
		"agents_working":  agentCounts[StatusWorking],
		"agents_completed": agentCounts[StatusCompleted],
		"agents_error":    agentCounts[StatusError],
		"agents_total":    len(build.Agents),
		"files_generated": fileCount,
		"total_size":      totalSize,
		"created_at":      build.CreatedAt,
		"updated_at":      build.UpdatedAt,
	}

	if state != nil {
		progress["phase"] = string(state.Phase)
		progress["phase_started"] = state.PhaseStarted
		progress["duration_ms"] = time.Since(state.StartTime).Milliseconds()
	}

	return progress, nil
}
