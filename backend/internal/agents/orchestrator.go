// Package agents - Build Orchestrator
// High-level orchestration of the build process across multiple AI agents.
// This provides the coordination layer that manages the entire build lifecycle.
package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"apex-build/internal/agents/autonomous"
	"apex-build/internal/agents/core"
	"apex-build/internal/ai"

	"github.com/google/uuid"
)

// orchestratorAIAdapter bridges the agents.AIRouter to autonomous.AIProvider interface
type orchestratorAIAdapter struct {
	router AIRouter
	userID uint
}

func (a *orchestratorAIAdapter) Generate(ctx context.Context, prompt string, opts autonomous.AIOptions) (string, error) {
	resp, err := a.router.Generate(ctx, ai.ProviderClaude, prompt, GenerateOptions{
		UserID:          a.userID,
		MaxTokens:       opts.MaxTokens,
		Temperature:     opts.Temperature,
		SystemPrompt:    opts.SystemPrompt,
		PowerMode:       PowerFast, // Use fast model for verification to save cost
		UsePlatformKeys: true,
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty response from AI")
	}
	return resp.Content, nil
}

func (a *orchestratorAIAdapter) Analyze(ctx context.Context, content string, instruction string, opts autonomous.AIOptions) (string, error) {
	prompt := fmt.Sprintf("Content to analyze:\n%s\n\nInstruction: %s", content, instruction)
	return a.Generate(ctx, prompt, opts)
}

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

	// FSM + Guarantee Engine context (nil-safe — orchestrator checks before use)
	FSMContext      *BuildFSMContext
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

	// Initialize FSM + Guarantee Engine for this build
	// Estimate total steps from task count (will be refined during planning)
	build.mu.RLock()
	totalSteps := len(build.Tasks)
	build.mu.RUnlock()
	if totalSteps < 5 {
		totalSteps = 5 // minimum — planning, architecture, generation, testing, review
	}

	broadcastFn := MakeBroadcastFunc(o.hub)
	fsmCtx, err := NewBuildFSMContext(buildID, totalSteps, broadcastFn)
	if err != nil {
		log.Printf("Orchestrator: Warning — FSM initialization failed for build %s: %v (continuing without FSM)", buildID, err)
	} else {
		state.FSMContext = fsmCtx
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
		// Clean up FSM bridge
		if state.FSMContext != nil {
			state.FSMContext.Stop()
		}
		state.cancel()
		o.mu.Lock()
		delete(o.activeBuild, state.BuildID)
		o.mu.Unlock()
	}()

	// Initialize enhanced tracking
	state.DependencyGraph = make(map[string][]string)
	state.TaskStatus = make(map[string]TaskStatus)
	state.VerifyGates = o.initializeVerifyGates()

	// Start FSM lifecycle
	o.fsmTransition(state, core.EventStart)       // idle → initializing
	o.fsmTransition(state, core.EventInitialized)  // initializing → planning

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

	// FSM: planning → executing (plan is ready, begin execution phases)
	o.fsmTransition(state, core.EventPlanReady)

	// Phase 2: Architecture
	if err := o.executeArchitecturePhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseArchitecting, err)
		return
	}

	// FSM: mark architecture as a completed step
	o.fsmTransition(state, core.EventStepComplete)

	// Phase 3: Code Generation (with parallel execution)
	if err := o.executeGenerationPhaseParallel(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseGenerating, err)
		return
	}

	// FSM: mark generation as a completed step
	o.fsmTransition(state, core.EventStepComplete)

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

	// FSM: mark testing as a completed step
	o.fsmTransition(state, core.EventStepComplete)

	// Phase 5: Review
	if err := o.executeReviewPhase(build, state); err != nil {
		o.handlePhaseError(build, state, PhaseReviewing, err)
		return
	}

	// FSM: all execution steps complete → transition to validating
	o.fsmTransition(state, core.EventAllStepsComplete)

	// Final Verify Gate
	if !o.runVerifyGate(build, state, "final") {
		log.Printf("Orchestrator: Final verification has warnings but proceeding")
	}

	// FSM: validation passed → completed
	o.fsmTransition(state, core.EventValidationPass)

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

// runVerifyGate executes a verification checkpoint with real validation logic
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

	// Real verification logic per gate type
	switch gateID {
	case "post-planning":
		gate.Passed, gate.Score = o.verifyPostPlanning(build)
	case "post-generation":
		gate.Passed, gate.Score = o.verifyPostGeneration(build, state)
	case "final":
		gate.Passed, gate.Score = o.verifyFinal(build)
	default:
		gate.Passed = true
		gate.Score = 80
	}

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

	log.Printf("Orchestrator: Gate '%s' result: passed=%v score=%d duration=%dms", gate.Name, gate.Passed, gate.Score, gate.Duration)

	if !gate.Passed && gate.Required {
		return false
	}

	return true
}

// verifyPostPlanning validates that the planning output contains required structure
func (o *BuildOrchestrator) verifyPostPlanning(build *Build) (bool, int) {
	score := 0

	build.mu.RLock()
	defer build.mu.RUnlock()

	// Check that we have a plan
	if build.Plan != nil {
		score += 30 // Plan exists
		if len(build.Plan.Files) > 0 {
			score += 20 // Files planned
		}
		if len(build.Plan.Features) > 0 {
			score += 15 // Features identified
		}
		if build.Plan.TechStack.Frontend != "" || build.Plan.TechStack.Backend != "" {
			score += 15 // Tech stack decided
		}
		if len(build.Plan.DataModels) > 0 {
			score += 10 // Data models defined
		}
		if len(build.Plan.APIEndpoints) > 0 {
			score += 10 // API endpoints defined
		}
	} else {
		// No plan struct — check if planning task produced any output
		for _, task := range build.Tasks {
			if task.Type == TaskPlan && task.Status == TaskCompleted {
				score += 40 // Planning task completed
				if task.Output != nil && (len(task.Output.Messages) > 0 || len(task.Output.Files) > 0) {
					score += 30 // Has output content
				}
				break
			}
		}
	}

	return score >= 40, score
}

// verifyPostGeneration runs the real BuildVerifier pipeline on generated code
func (o *BuildOrchestrator) verifyPostGeneration(build *Build, state *OrchestrationState) (bool, int) {
	// Collect all generated files
	allFiles := o.manager.collectGeneratedFiles(build)

	if len(allFiles) == 0 {
		log.Printf("Orchestrator: No files generated — verification fails")
		return false, 0
	}

	// Quick syntax check using manager's built-in verification (no filesystem needed)
	syntaxScore := 0
	syntaxErrors := 0
	placeholderErrors := 0

	for _, file := range allFiles {
		if file.Content == "" {
			continue
		}
		// Bracket balance check
		if strings.Count(file.Content, "{") != strings.Count(file.Content, "}") {
			syntaxErrors++
		}
		if strings.Count(file.Content, "(") != strings.Count(file.Content, ")") {
			syntaxErrors++
		}
		// Placeholder detection
		placeholders := []string{"// TODO:", "// FIXME:", "throw new Error('Not implemented')",
			"raise NotImplementedError", "panic(\"not implemented\")", "/* placeholder */"}
		for _, p := range placeholders {
			if strings.Contains(file.Content, p) {
				placeholderErrors++
			}
		}
	}

	// Calculate syntax score (0-40)
	if syntaxErrors == 0 {
		syntaxScore = 40
	} else if syntaxErrors <= 2 {
		syntaxScore = 25
	} else {
		syntaxScore = 10
	}

	// Placeholder penalty
	if placeholderErrors > 0 {
		syntaxScore -= placeholderErrors * 5
		if syntaxScore < 0 {
			syntaxScore = 0
		}
	}

	// File coverage score (0-30): did we generate enough files?
	fileScore := 0
	if len(allFiles) >= 5 {
		fileScore = 30
	} else if len(allFiles) >= 3 {
		fileScore = 20
	} else if len(allFiles) >= 1 {
		fileScore = 10
	}

	// Content quality score (0-30): files have substantive content
	contentScore := 0
	substantiveFiles := 0
	for _, file := range allFiles {
		if len(file.Content) > 100 { // More than 100 chars = substantive
			substantiveFiles++
		}
	}
	if substantiveFiles == len(allFiles) {
		contentScore = 30
	} else if substantiveFiles > len(allFiles)/2 {
		contentScore = 20
	} else if substantiveFiles > 0 {
		contentScore = 10
	}

	totalScore := syntaxScore + fileScore + contentScore

	// Try filesystem-based verification with BuildVerifier (best-effort, with timeout)
	verifierScore, verifierRan := o.runBuildVerifier(build, allFiles)
	if verifierRan {
		// Blend scores: 40% syntax/structure, 60% real build verification
		totalScore = (totalScore * 40 / 100) + (verifierScore * 60 / 100)
	}

	log.Printf("Orchestrator: Post-generation verification: syntax=%d files=%d content=%d verifier=%d(ran=%v) total=%d",
		syntaxScore, fileScore, contentScore, verifierScore, verifierRan, totalScore)

	return totalScore >= 50, totalScore
}

// runBuildVerifier writes files to temp dir and runs the autonomous BuildVerifier pipeline
func (o *BuildOrchestrator) runBuildVerifier(build *Build, files []GeneratedFile) (int, bool) {
	// Create temp directory for verification
	tmpDir, err := os.MkdirTemp("", "apex-verify-*")
	if err != nil {
		log.Printf("Orchestrator: Failed to create temp dir for verification: %v", err)
		return 0, false
	}
	defer os.RemoveAll(tmpDir)

	// Write all generated files to temp dir
	for _, file := range files {
		if file.Path == "" || file.Content == "" {
			continue
		}
		fullPath := filepath.Join(tmpDir, file.Path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Orchestrator: Failed to create dir %s: %v", dir, err)
			continue
		}
		if err := os.WriteFile(fullPath, []byte(file.Content), 0644); err != nil {
			log.Printf("Orchestrator: Failed to write file %s: %v", fullPath, err)
			continue
		}
	}

	// Create AI adapter for the verifier
	aiAdapter := &orchestratorAIAdapter{
		router: o.manager.aiRouter,
		userID: build.UserID,
	}

	// Run verification with a 60-second timeout
	verifier := autonomous.NewBuildVerifier(aiAdapter, tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := verifier.Verify(ctx, 1) // 1 retry to keep it fast
	if err != nil {
		log.Printf("Orchestrator: BuildVerifier failed: %v", err)
		return 0, false
	}

	log.Printf("Orchestrator: BuildVerifier result: passed=%v score=%d summary=%s", result.Passed, result.Score, result.Summary)
	return result.Score, true
}

// verifyFinal runs a lightweight quality check on all generated files
func (o *BuildOrchestrator) verifyFinal(build *Build) (bool, int) {
	allFiles := o.manager.collectGeneratedFiles(build)
	if len(allFiles) == 0 {
		return true, 50 // No files is OK for final gate (might be planning-only)
	}

	score := 100
	issues := 0

	for _, file := range allFiles {
		// Check for empty content
		if file.Content == "" {
			issues++
			score -= 10
		}
		// Check for placeholder code
		if strings.Contains(file.Content, "// TODO:") || strings.Contains(file.Content, "// FIXME:") {
			issues++
			score -= 5
		}
		// Check for very small files (likely incomplete)
		if len(file.Content) > 0 && len(file.Content) < 50 {
			issues++
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}

	log.Printf("Orchestrator: Final verification: %d files, %d issues, score=%d", len(allFiles), issues, score)
	return score >= 50, score
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

	// Collect all generated files as safety net for the frontend
	allFiles := o.manager.collectGeneratedFiles(build)

	// Broadcast completion with full file manifest
	o.hub.Broadcast(build.ID, &WSMessage{
		Type:      WSBuildCompleted,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":      string(BuildCompleted),
			"progress":    100,
			"duration_ms": time.Since(state.StartTime).Milliseconds(),
			"message":     "Build completed successfully!",
			"files_count": len(allFiles),
			"files":       allFiles,
		},
	})

	log.Printf("Orchestrator: Build %s completed in %v (%d files)", build.ID, time.Since(state.StartTime), len(allFiles))

	// Persist to database
	o.manager.persistCompletedBuild(build, allFiles)
}

// fsmTransition safely transitions the FSM if it exists. Logs warnings on error.
func (o *BuildOrchestrator) fsmTransition(state *OrchestrationState, event core.AgentEvent) {
	if state.FSMContext == nil {
		return
	}
	if err := state.FSMContext.FSM.Transition(event); err != nil {
		log.Printf("Orchestrator: FSM transition warning (event=%s): %v", event, err)
	}
}

// fsmTransitionWithMeta safely transitions the FSM with metadata if it exists.
func (o *BuildOrchestrator) fsmTransitionWithMeta(state *OrchestrationState, event core.AgentEvent, meta string) {
	if state.FSMContext == nil {
		return
	}
	if err := state.FSMContext.FSM.TransitionWithMeta(event, meta); err != nil {
		log.Printf("Orchestrator: FSM transition warning (event=%s): %v", event, err)
	}
}

// handlePhaseError handles errors during a phase
func (o *BuildOrchestrator) handlePhaseError(build *Build, state *OrchestrationState, phase BuildPhase, err error) {
	state.Phase = PhaseFailed
	state.Errors = append(state.Errors, fmt.Sprintf("Phase %s failed: %v", phase, err))

	log.Printf("Orchestrator: Build %s failed in phase %s: %v", build.ID, phase, err)

	// Transition FSM to fatal error → will trigger rolling_back or failed
	o.fsmTransitionWithMeta(state, core.EventFatalError, err.Error())

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
