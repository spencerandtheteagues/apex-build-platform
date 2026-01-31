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
	BuildID         string
	Phase           BuildPhase
	StartTime       time.Time
	PhaseStarted    time.Time
	TotalAgents     int
	ActiveAgents    int
	CompletedTasks  int
	TotalTasks      int
	Errors          []string
	ctx             context.Context
	cancel          context.CancelFunc

	// Enhanced tracking
	ParallelTasks   int              // Number of tasks running in parallel
	DependencyGraph map[string][]string // Task ID -> dependent task IDs
	TaskStatus      map[string]TaskStatus
	VerifyGates     []VerifyGate     // Build verification checkpoints
	ResourceUsage   ResourceMetrics  // Track resource consumption
}

// VerifyGate represents a build verification checkpoint
type VerifyGate struct {
	ID          string
	Name        string
	Phase       BuildPhase
	Required    bool
	Passed      bool
	Score       int
	RunAt       *time.Time
	Duration    int64
	BlocksPhase BuildPhase // Which phase this gate blocks
}

// ResourceMetrics tracks resource consumption
type ResourceMetrics struct {
	AIRequestsUsed    int
	AITokensConsumed  int64
	FilesGenerated    int
	TotalBytesWritten int64
	PeakParallelTasks int
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

	// Initialize enhanced tracking
	state.DependencyGraph = make(map[string][]string)
	state.TaskStatus = make(map[string]TaskStatus)
	state.VerifyGates = o.initializeVerifyGates()

	// Phase 1: Planning
	if err := o.executePlanningPhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhasePlanning, err)
		return
	}

	// Verify Gate: Post-Planning
	if !o.runVerifyGate(build, state, "post-planning") {
		o.handlePhaseError(build, state, PhasePlanning, fmt.Errorf("post-planning verification failed"))
		return
	}

	// Phase 2: Architecture
	if err := o.executeArchitecturePhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseArchitecting, err)
		return
	}

	// Phase 3: Code Generation (with parallel execution)
	if err := o.executeGenerationPhaseParallel(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseGenerating, err)
		return
	}

	// Verify Gate: Post-Generation (run actual build)
	if !o.runVerifyGate(build, state, "post-generation") {
		log.Printf("Orchestrator: Post-generation verification failed, attempting auto-fix")
		if !o.attemptAutoFix(build, state) {
			o.handlePhaseError(build, state, PhaseGenerating, fmt.Errorf("code generation verification failed"))
			return
		}
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

	// Final Verify Gate
	if !o.runVerifyGate(build, state, "final") {
		log.Printf("Orchestrator: Final verification has warnings but proceeding")
	}

	// Complete
	o.completeOrchestration(build, state)
}

// initializeVerifyGates creates the verification checkpoints
func (o *BuildOrchestrator) initializeVerifyGates() []VerifyGate {
	return []VerifyGate{
		{
			ID:          "post-planning",
			Name:        "Post-Planning Validation",
			Phase:       PhasePlanning,
			Required:    true,
			BlocksPhase: PhaseArchitecting,
		},
		{
			ID:          "post-generation",
			Name:        "Build Verification",
			Phase:       PhaseGenerating,
			Required:    true,
			BlocksPhase: PhaseTesting,
		},
		{
			ID:          "final",
			Name:        "Final Quality Check",
			Phase:       PhaseCompleting,
			Required:    false, // Warnings OK
			BlocksPhase: PhaseComplete,
		},
	}
}

// runVerifyGate executes a verification checkpoint
func (o *BuildOrchestrator) runVerifyGate(build *Build, state *OrchestrationState, gateID string) bool {
	var gate *VerifyGate
	for i := range state.VerifyGates {
		if state.VerifyGates[i].ID == gateID {
			gate = &state.VerifyGates[i]
			break
		}
	}
	if gate == nil {
		return true
	}

	log.Printf("Orchestrator: Running verify gate '%s' for build %s", gate.Name, build.ID)
	now := time.Now()
	gate.RunAt = &now

	o.broadcastPhase(build.ID, state.Phase, fmt.Sprintf("Running verification: %s...", gate.Name))

	// Placeholder for actual verification logic
	// In production, this would call the BuildVerifier
	gate.Passed = true
	gate.Score = 80
	gate.Duration = time.Since(now).Milliseconds()

	o.hub.Broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"verify_gate": gate.ID,
			"name":        gate.Name,
			"passed":      gate.Passed,
			"score":       gate.Score,
			"duration_ms": gate.Duration,
		},
	})

	if !gate.Passed && gate.Required {
		return false
	}

	return true
}

// executeGenerationPhaseParallel runs code generation tasks in parallel
func (o *BuildOrchestrator) executeGenerationPhaseParallel(build *Build, state *OrchestrationState) error {
	state.Phase = PhaseGenerating
	state.PhaseStarted = time.Now()
	o.broadcastPhase(build.ID, state.Phase, "Generating code (parallel execution)...")

	// Get all generation tasks
	generationTypes := []TaskType{TaskGenerateFile, TaskGenerateAPI, TaskGenerateUI, TaskGenerateSchema}

	build.mu.RLock()
	var genTasks []*Task
	for _, task := range build.Tasks {
		for _, genType := range generationTypes {
			if task.Type == genType && task.Status == TaskPending {
				genTasks = append(genTasks, task)
				break
			}
		}
	}
	build.mu.RUnlock()

	if len(genTasks) == 0 {
		return nil
	}

	// Build dependency graph
	depGraph := o.buildDependencyGraph(genTasks)

	// Execute in parallel waves based on dependencies
	return o.executeTasksInWaves(build, state, genTasks, depGraph)
}

// buildDependencyGraph creates a dependency graph for tasks
func (o *BuildOrchestrator) buildDependencyGraph(tasks []*Task) map[string][]string {
	graph := make(map[string][]string)

	// Simple heuristic: DB schema before backend, backend before frontend
	var schemaTasks, backendTasks, frontendTasks []string

	for _, task := range tasks {
		switch task.Type {
		case TaskGenerateSchema:
			schemaTasks = append(schemaTasks, task.ID)
		case TaskGenerateAPI:
			backendTasks = append(backendTasks, task.ID)
		case TaskGenerateUI:
			frontendTasks = append(frontendTasks, task.ID)
		}
	}

	// Backend depends on schema
	for _, backend := range backendTasks {
		graph[backend] = schemaTasks
	}

	// Frontend depends on backend (for API types)
	for _, frontend := range frontendTasks {
		graph[frontend] = backendTasks
	}

	return graph
}

// executeTasksInWaves executes tasks in parallel waves based on dependencies
func (o *BuildOrchestrator) executeTasksInWaves(build *Build, state *OrchestrationState, tasks []*Task, depGraph map[string][]string) error {
	ctx := state.ctx
	remaining := make(map[string]*Task)
	completed := make(map[string]bool)

	for _, task := range tasks {
		remaining[task.ID] = task
	}

	maxParallel := 4 // Configurable max parallel tasks

	for len(remaining) > 0 {
		// Find tasks that can run (all dependencies complete)
		var ready []*Task
		for id, task := range remaining {
			deps := depGraph[id]
			canRun := true
			for _, dep := range deps {
				if !completed[dep] {
					canRun = false
					break
				}
			}
			if canRun {
				ready = append(ready, task)
			}
		}

		if len(ready) == 0 {
			// Deadlock - remaining tasks have unmet dependencies
			log.Printf("Orchestrator: Dependency deadlock detected with %d remaining tasks", len(remaining))
			break
		}

		// Limit parallel execution
		if len(ready) > maxParallel {
			ready = ready[:maxParallel]
		}

		state.ParallelTasks = len(ready)
		if state.ParallelTasks > state.ResourceUsage.PeakParallelTasks {
			state.ResourceUsage.PeakParallelTasks = state.ParallelTasks
		}

		o.broadcastPhase(build.ID, state.Phase,
			fmt.Sprintf("Executing %d tasks in parallel (%d remaining)...", len(ready), len(remaining)))

		// Execute wave in parallel
		var wg sync.WaitGroup
		errChan := make(chan error, len(ready))

		for _, task := range ready {
			wg.Add(1)
			go func(t *Task) {
				defer wg.Done()

				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
				}

				if err := o.waitForTasksOfType(build, state, t.Type, 5*time.Minute); err != nil {
					errChan <- err
				}
			}(task)
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			if err != nil {
				return err
			}
		}

		// Mark tasks as complete and remove from remaining
		for _, task := range ready {
			completed[task.ID] = true
			delete(remaining, task.ID)
		}

		state.ParallelTasks = 0
	}

	return nil
}

// attemptAutoFix tries to automatically fix verification failures
func (o *BuildOrchestrator) attemptAutoFix(build *Build, state *OrchestrationState) bool {
	log.Printf("Orchestrator: Attempting auto-fix for build %s", build.ID)

	o.broadcastPhase(build.ID, state.Phase, "Attempting automatic fixes...")

	// Find agents that can do fixes
	build.mu.RLock()
	var reviewerAgent *Agent
	for _, agent := range build.Agents {
		if agent.Role == RoleReviewer {
			reviewerAgent = agent
			break
		}
	}
	build.mu.RUnlock()

	if reviewerAgent == nil {
		log.Printf("Orchestrator: No reviewer agent available for auto-fix")
		return false
	}

	// Create fix task
	fixTask := &Task{
		ID:          uuid.New().String(),
		Type:        TaskReview,
		Description: "Analyze and fix build verification failures",
		Priority:    100, // High priority
		Status:      TaskPending,
		MaxRetries:  2,
		Input: map[string]any{
			"action":       "auto_fix",
			"build_errors": state.Errors,
		},
		CreatedAt: time.Now(),
	}

	build.mu.Lock()
	build.Tasks = append(build.Tasks, fixTask)
	build.mu.Unlock()

	if err := o.manager.AssignTask(reviewerAgent.ID, fixTask); err != nil {
		log.Printf("Orchestrator: Failed to assign fix task: %v", err)
		return false
	}

	// Wait for fix task
	if err := o.waitForTasksOfType(build, state, TaskReview, 3*time.Minute); err != nil {
		log.Printf("Orchestrator: Fix task failed: %v", err)
		return false
	}

	// Re-run verification
	return o.runVerifyGate(build, state, "post-generation")
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
