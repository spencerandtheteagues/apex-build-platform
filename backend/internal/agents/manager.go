// Package agents - Agent Manager
// This component spawns, tracks, and manages AI agents during builds.
package agents

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AgentManager handles the lifecycle and coordination of AI agents
type AgentManager struct {
	agents      map[string]*Agent
	builds      map[string]*Build
	taskQueue   chan *Task
	resultQueue chan *TaskResult
	subscribers map[string][]chan *WSMessage
	aiRouter    AIRouter
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// AIRouter interface for communicating with AI providers
type AIRouter interface {
	Generate(ctx context.Context, provider AIProvider, prompt string, opts GenerateOptions) (string, error)
}

// GenerateOptions for AI generation requests
type GenerateOptions struct {
	MaxTokens   int
	Temperature float64
	SystemPrompt string
	Context     []Message
}

// Message for AI context
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TaskResult holds the result of a completed task
type TaskResult struct {
	TaskID  string
	AgentID string
	Success bool
	Output  *TaskOutput
	Error   error
}

// NewAgentManager creates a new agent manager instance
func NewAgentManager(aiRouter AIRouter) *AgentManager {
	ctx, cancel := context.WithCancel(context.Background())

	am := &AgentManager{
		agents:      make(map[string]*Agent),
		builds:      make(map[string]*Build),
		taskQueue:   make(chan *Task, 100),
		resultQueue: make(chan *TaskResult, 100),
		subscribers: make(map[string][]chan *WSMessage),
		aiRouter:    aiRouter,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start background workers
	go am.taskDispatcher()
	go am.resultProcessor()

	log.Println("Agent Manager initialized")
	return am
}

// CreateBuild starts a new build session
func (am *AgentManager) CreateBuild(userID uint, req *BuildRequest) (*Build, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	buildID := uuid.New().String()
	now := time.Now()

	mode := req.Mode
	if mode == "" {
		mode = ModeFull
	}

	build := &Build{
		ID:          buildID,
		UserID:      userID,
		Status:      BuildPending,
		Mode:        mode,
		Description: req.Description,
		Agents:      make(map[string]*Agent),
		Tasks:       make([]*Task, 0),
		Checkpoints: make([]*Checkpoint, 0),
		Progress:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	am.builds[buildID] = build

	log.Printf("Created build %s for user %d: %s", buildID, userID, truncate(req.Description, 50))
	return build, nil
}

// StartBuild begins the build process
func (am *AgentManager) StartBuild(buildID string) error {
	log.Printf("StartBuild called for build %s", buildID)

	am.mu.Lock()
	build, exists := am.builds[buildID]
	am.mu.Unlock()

	if !exists {
		log.Printf("Build %s not found in manager", buildID)
		return fmt.Errorf("build %s not found", buildID)
	}

	log.Printf("Build %s found, updating status to planning", buildID)

	// Update status
	build.mu.Lock()
	build.Status = BuildPlanning
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	log.Printf("Build %s status updated, broadcasting", buildID)

	// Broadcast build started
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildStarted,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":      string(BuildPlanning),
			"description": build.Description,
			"mode":        string(build.Mode),
		},
	})

	// Spawn the lead agent first
	leadAgent, err := am.spawnAgent(buildID, RoleLead, ProviderClaude)
	if err != nil {
		am.broadcast(buildID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   buildID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":   "Failed to spawn lead agent",
				"details": err.Error(),
			},
		})
		return fmt.Errorf("failed to spawn lead agent: %w", err)
	}

	// Update lead agent status
	leadAgent.mu.Lock()
	leadAgent.Status = StatusWorking
	leadAgent.mu.Unlock()

	// Create planning task with proper initialization
	planTask := &Task{
		ID:          uuid.New().String(),
		Type:        TaskPlan,
		Description: fmt.Sprintf("Create comprehensive build plan for: %s", build.Description),
		Priority:    100,
		Status:      TaskPending,
		MaxRetries:  5,
		Input: map[string]any{
			"description": build.Description,
			"mode":        string(build.Mode),
		},
		CreatedAt: time.Now(),
	}

	// Assign to lead agent
	planTask.AssignedTo = leadAgent.ID
	planTask.Status = TaskInProgress
	now := time.Now()
	planTask.StartedAt = &now

	// Set current task on agent
	leadAgent.mu.Lock()
	leadAgent.CurrentTask = planTask
	leadAgent.mu.Unlock()

	build.mu.Lock()
	build.Tasks = append(build.Tasks, planTask)
	build.mu.Unlock()

	// Broadcast agent working
	am.broadcast(buildID, &WSMessage{
		Type:      WSAgentWorking,
		BuildID:   buildID,
		AgentID:   leadAgent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"task_id":     planTask.ID,
			"task_type":   string(planTask.Type),
			"description": planTask.Description,
			"agent_role":  string(leadAgent.Role),
			"provider":    string(leadAgent.Provider),
		},
	})

	// Queue the planning task
	am.taskQueue <- planTask

	// Start build timeout goroutine - force complete after timeout
	go am.buildTimeoutHandler(buildID, build.Mode)

	log.Printf("Build %s started with lead agent %s, planning task %s queued", buildID, leadAgent.ID, planTask.ID)
	return nil
}

// buildTimeoutHandler forces build completion after a timeout to ensure users get results
func (am *AgentManager) buildTimeoutHandler(buildID string, mode BuildMode) {
	// Fast build: 2 minutes, Full build: 5 minutes
	timeout := 2 * time.Minute
	if mode == ModeFull {
		timeout = 5 * time.Minute
	}

	time.Sleep(timeout)

	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return
	}

	build.mu.RLock()
	status := build.Status
	build.mu.RUnlock()

	// If build is still in progress, force complete it
	if status == BuildPlanning || status == BuildInProgress {
		log.Printf("Build %s timed out after %v, forcing completion", buildID, timeout)
		am.forceCompleteBuild(buildID)
	}
}

// forceCompleteBuild marks a build as complete even if some tasks are still pending
func (am *AgentManager) forceCompleteBuild(buildID string) {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return
	}

	build.mu.Lock()
	now := time.Now()
	build.CompletedAt = &now
	build.UpdatedAt = now
	build.Status = BuildCompleted
	build.Progress = 100

	// Cancel any pending tasks
	for _, task := range build.Tasks {
		if task.Status == TaskPending || task.Status == TaskInProgress {
			task.Status = TaskCancelled
		}
	}
	build.mu.Unlock()

	// Create final checkpoint with all generated files
	am.createCheckpoint(build, "Build Complete (Timeout)", "Build completed with available results")

	// Broadcast completion
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildCompleted,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"status":       string(BuildCompleted),
			"progress":     100,
			"timed_out":    true,
			"files_count":  len(am.collectGeneratedFiles(build)),
		},
	})

	log.Printf("Build %s force completed with %d files", buildID, len(am.collectGeneratedFiles(build)))
}

// spawnAgent creates a new AI agent with a specific role
func (am *AgentManager) spawnAgent(buildID string, role AgentRole, provider AIProvider) (*Agent, error) {
	am.mu.Lock()

	build, exists := am.builds[buildID]
	if !exists {
		am.mu.Unlock()
		return nil, fmt.Errorf("build %s not found", buildID)
	}

	agentID := uuid.New().String()
	now := time.Now()

	agent := &Agent{
		ID:        agentID,
		Role:      role,
		Provider:  provider,
		Status:    StatusIdle,
		BuildID:   buildID,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
		Output:    make([]string, 0),
	}

	am.agents[agentID] = agent
	build.mu.Lock()
	build.Agents[agentID] = agent
	build.mu.Unlock()

	// Release lock BEFORE broadcasting to avoid deadlock
	am.mu.Unlock()

	// Broadcast agent spawned (outside of lock)
	am.broadcast(buildID, &WSMessage{
		Type:      WSAgentSpawned,
		BuildID:   buildID,
		AgentID:   agentID,
		Timestamp: now,
		Data: map[string]any{
			"role":     string(role),
			"provider": string(provider),
		},
	})

	log.Printf("Spawned %s agent %s (provider: %s) for build %s", role, agentID, provider, buildID)
	return agent, nil
}

// SpawnAgentTeam creates the full team of agents for a build
func (am *AgentManager) SpawnAgentTeam(buildID string) error {
	// Spawn agents with appropriate providers based on their roles
	agentConfigs := []struct {
		Role     AgentRole
		Provider AIProvider
	}{
		{RolePlanner, ProviderClaude},    // Claude excels at planning
		{RoleArchitect, ProviderClaude},  // Claude for architecture
		{RoleFrontend, ProviderGPT},      // GPT for code generation
		{RoleBackend, ProviderGPT},       // GPT for code generation
		{RoleDatabase, ProviderClaude},   // Claude for schemas
		{RoleTesting, ProviderGemini},    // Gemini for testing
		{RoleReviewer, ProviderClaude},   // Claude for code review
	}

	for _, config := range agentConfigs {
		_, err := am.spawnAgent(buildID, config.Role, config.Provider)
		if err != nil {
			return fmt.Errorf("failed to spawn %s agent: %w", config.Role, err)
		}
	}

	return nil
}

// AssignTask assigns a task to a specific agent
func (am *AgentManager) AssignTask(agentID string, task *Task) error {
	am.mu.RLock()
	agent, exists := am.agents[agentID]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.mu.Lock()
	agent.CurrentTask = task
	agent.Status = StatusWorking
	agent.UpdatedAt = time.Now()
	agent.mu.Unlock()

	task.AssignedTo = agentID
	task.Status = TaskInProgress
	now := time.Now()
	task.StartedAt = &now

	// Broadcast agent working
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      WSAgentWorking,
		BuildID:   agent.BuildID,
		AgentID:   agentID,
		Timestamp: now,
		Data: map[string]any{
			"task_id":     task.ID,
			"task_type":   string(task.Type),
			"description": task.Description,
		},
	})

	am.taskQueue <- task
	return nil
}

// taskDispatcher processes tasks from the queue
func (am *AgentManager) taskDispatcher() {
	for {
		select {
		case <-am.ctx.Done():
			return
		case task := <-am.taskQueue:
			go am.executeTask(task)
		}
	}
}

// executeTask runs a task using the appropriate AI agent
func (am *AgentManager) executeTask(task *Task) {
	log.Printf("executeTask called for task %s (type: %s, assignedTo: %s)", task.ID, task.Type, task.AssignedTo)

	am.mu.RLock()
	agent, exists := am.agents[task.AssignedTo]
	am.mu.RUnlock()

	if !exists {
		log.Printf("Agent %s not found for task %s", task.AssignedTo, task.ID)
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("agent %s not found", task.AssignedTo),
		}
		return
	}
	log.Printf("Found agent %s (role: %s, provider: %s)", agent.ID, agent.Role, agent.Provider)

	// Get the build for context
	am.mu.RLock()
	build, buildExists := am.builds[agent.BuildID]
	am.mu.RUnlock()

	if !buildExists {
		log.Printf("Build %s not found for agent %s", agent.BuildID, agent.ID)
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   fmt.Errorf("build %s not found", agent.BuildID),
		}
		return
	}
	log.Printf("Found build %s for task execution", build.ID)

	// Broadcast that the agent is thinking
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      "agent:thinking",
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role": agent.Role,
			"provider":   agent.Provider,
			"task_id":    task.ID,
			"task_type":  string(task.Type),
			"content":    fmt.Sprintf("%s agent is analyzing the task...", agent.Role),
		},
	})

	// Build the prompt based on task type
	prompt := am.buildTaskPrompt(task, build, agent)
	systemPrompt := am.getSystemPrompt(agent.Role)
	log.Printf("Built prompt for task (prompt_length: %d, system_prompt_length: %d)", len(prompt), len(systemPrompt))

	// Execute using AI router
	ctx, cancel := context.WithTimeout(am.ctx, 5*time.Minute)
	defer cancel()

	// Broadcast that the agent is generating
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      "agent:generating",
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role": agent.Role,
			"provider":   agent.Provider,
			"task_id":    task.ID,
			"task_type":  string(task.Type),
			"content":    fmt.Sprintf("%s agent is generating code with %s...", agent.Role, agent.Provider),
		},
	})

	log.Printf("Calling AI router for task %s with provider %s", task.ID, agent.Provider)
	response, err := am.aiRouter.Generate(ctx, agent.Provider, prompt, GenerateOptions{
		MaxTokens:    8000,
		Temperature:  0.7,
		SystemPrompt: systemPrompt,
	})

	if err != nil {
		log.Printf("AI generation failed for task %s: %v", task.ID, err)

		// Broadcast the error
		am.broadcast(agent.BuildID, &WSMessage{
			Type:      "agent:generation_failed",
			BuildID:   agent.BuildID,
			AgentID:   agent.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"agent_role":   agent.Role,
				"provider":     agent.Provider,
				"task_id":      task.ID,
				"error":        err.Error(),
				"retry_count":  task.RetryCount,
				"max_retries":  task.MaxRetries,
			},
		})

		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   err,
		}
		return
	}

	log.Printf("AI generation succeeded for task %s (response_length: %d)", task.ID, len(response))

	// Parse the response into task output
	output := am.parseTaskOutput(task.Type, response)

	// Broadcast code generated with file count
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      WSCodeGenerated,
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role":  agent.Role,
			"provider":    agent.Provider,
			"task_id":     task.ID,
			"task_type":   string(task.Type),
			"files_count": len(output.Files),
			"files":       output.Files,
		},
	})

	am.resultQueue <- &TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output:  output,
	}
	log.Printf("Task %s completed successfully with %d files generated", task.ID, len(output.Files))
}

// resultProcessor handles completed task results
func (am *AgentManager) resultProcessor() {
	for {
		select {
		case <-am.ctx.Done():
			return
		case result := <-am.resultQueue:
			am.processResult(result)
		}
	}
}

// processResult handles a task result with retry logic
func (am *AgentManager) processResult(result *TaskResult) {
	am.mu.RLock()
	agent, agentExists := am.agents[result.AgentID]
	am.mu.RUnlock()

	if !agentExists {
		log.Printf("Warning: result for unknown agent %s", result.AgentID)
		return
	}

	agent.mu.Lock()
	task := agent.CurrentTask

	if result.Success {
		agent.Status = StatusCompleted
		if task != nil {
			task.Status = TaskCompleted
			now := time.Now()
			task.CompletedAt = &now
			task.Output = result.Output
		}
		agent.UpdatedAt = time.Now()
		agent.mu.Unlock()

		// Broadcast success
		am.broadcast(agent.BuildID, &WSMessage{
			Type:      WSAgentCompleted,
			BuildID:   agent.BuildID,
			AgentID:   agent.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"task_id": result.TaskID,
				"success": true,
				"output":  result.Output,
			},
		})

		// Handle task completion - may trigger next tasks
		if task != nil {
			am.handleTaskCompletion(agent.BuildID, task, result.Output)
		}
	} else {
		// FAILURE HANDLING - Learn from error and retry
		if task == nil {
			agent.Status = StatusError
			agent.Error = result.Error.Error()
			agent.mu.Unlock()
			return
		}

		// Set default max retries if not set
		if task.MaxRetries == 0 {
			task.MaxRetries = 5 // Default: try up to 5 times
		}

		// Track the error for learning
		errorAttempt := ErrorAttempt{
			AttemptNumber: task.RetryCount + 1,
			Error:         result.Error.Error(),
			Timestamp:     time.Now(),
			Context:       fmt.Sprintf("Attempt %d of %d", task.RetryCount+1, task.MaxRetries),
		}
		task.ErrorHistory = append(task.ErrorHistory, errorAttempt)
		task.RetryCount++

		// Check if we should retry
		if task.RetryCount < task.MaxRetries {
			// Analyze error and prepare for retry
			log.Printf("Task %s failed (attempt %d/%d): %v. Retrying...",
				task.ID, task.RetryCount, task.MaxRetries, result.Error)

			// Set status back to pending for retry
			task.Status = TaskPending
			task.Error = "" // Clear error for retry
			task.RetryStrategy = RetryWithFix // Use learning-based retry

			agent.Status = StatusWorking
			agent.Error = ""
			agent.UpdatedAt = time.Now()
			agent.mu.Unlock()

			// Broadcast retry attempt
			am.broadcast(agent.BuildID, &WSMessage{
				Type:      "agent:retrying",
				BuildID:   agent.BuildID,
				AgentID:   agent.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"task_id":       result.TaskID,
					"attempt":       task.RetryCount,
					"max_retries":   task.MaxRetries,
					"error":         result.Error.Error(),
					"error_history": task.ErrorHistory,
					"message":       fmt.Sprintf("Learning from error, retrying (%d/%d)...", task.RetryCount, task.MaxRetries),
				},
			})

			// Broadcast thinking about the error
			am.broadcast(agent.BuildID, &WSMessage{
				Type:      "agent:thinking",
				BuildID:   agent.BuildID,
				AgentID:   agent.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"agent_role": agent.Role,
					"provider":   agent.Provider,
					"content":    fmt.Sprintf("Analyzing error: %s. Adjusting approach for retry attempt %d...", result.Error.Error(), task.RetryCount+1),
				},
			})

			// Re-queue the task with error context for learning
			task.Input["previous_errors"] = task.ErrorHistory
			task.Input["retry_guidance"] = "Previous attempt failed. Analyze the error and try a different approach."

			// Put task back in queue
			am.taskQueue <- task
		} else {
			// Max retries exceeded - mark as failed
			log.Printf("Task %s failed after %d attempts. Giving up.", task.ID, task.RetryCount)

			agent.Status = StatusError
			agent.Error = fmt.Sprintf("Failed after %d attempts: %s", task.RetryCount, result.Error.Error())
			task.Status = TaskFailed
			task.Error = agent.Error
			agent.UpdatedAt = time.Now()
			agent.mu.Unlock()

			// Broadcast final failure
			am.broadcast(agent.BuildID, &WSMessage{
				Type:      WSAgentError,
				BuildID:   agent.BuildID,
				AgentID:   agent.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"task_id":       result.TaskID,
					"success":       false,
					"error":         agent.Error,
					"error_history": task.ErrorHistory,
					"attempts":      task.RetryCount,
					"max_retries":   task.MaxRetries,
					"message":       "Task failed after multiple retry attempts. Consider breaking down the task or providing more guidance.",
				},
			})
		}
	}
}

// handleTaskCompletion processes a completed task and triggers follow-up work
func (am *AgentManager) handleTaskCompletion(buildID string, task *Task, output *TaskOutput) {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return
	}

	switch task.Type {
	case TaskPlan:
		// Planning completed - parse plan and spawn agents
		am.handlePlanCompletion(build, output)
	case TaskGenerateFile:
		// File generated - broadcast and update progress
		am.handleFileGeneration(build, output)
	case TaskTest:
		// Tests completed - check results
		am.handleTestCompletion(build, output)
	case TaskReview:
		// Review completed - apply fixes if needed
		am.handleReviewCompletion(build, output)
	}

	// Update build progress
	am.updateBuildProgress(build)

	// Check if build is complete
	am.checkBuildCompletion(build)
}

// handlePlanCompletion processes the build plan and spawns the agent team
func (am *AgentManager) handlePlanCompletion(build *Build, output *TaskOutput) {
	log.Printf("handlePlanCompletion called for build %s", build.ID)

	// Broadcast planning phase completion
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":    "planning_complete",
			"message":  "Build plan created successfully",
			"progress": 10,
		},
	})

	// Store plan messages as the build plan
	if output != nil && len(output.Messages) > 0 {
		build.mu.Lock()
		build.Plan = &BuildPlan{
			ID:        uuid.New().String(),
			BuildID:   build.ID,
			CreatedAt: time.Now(),
		}
		build.mu.Unlock()
	}

	build.mu.Lock()
	build.Status = BuildInProgress
	build.Progress = 15
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	// Spawn the full agent team
	if err := am.SpawnAgentTeam(build.ID); err != nil {
		log.Printf("Error spawning agent team: %v", err)
		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":   "Failed to spawn agent team",
				"details": err.Error(),
			},
		})
		return
	}

	// Broadcast agent team spawned
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":    "agents_spawned",
			"message":  "Agent team assembled and ready",
			"progress": 20,
		},
	})

	// Create checkpoint for planning phase
	am.createCheckpoint(build, "Planning Complete", "Build plan created and agent team spawned")

	// Queue initial tasks for each agent based on the plan
	am.queuePlanTasks(build)

	log.Printf("Build %s transitioned to in_progress with full agent team", build.ID)
}

// handleFileGeneration processes a generated file
func (am *AgentManager) handleFileGeneration(build *Build, output *TaskOutput) {
	if output == nil || len(output.Files) == 0 {
		return
	}

	for _, file := range output.Files {
		am.broadcast(build.ID, &WSMessage{
			Type:      WSFileCreated,
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"path":     file.Path,
				"language": file.Language,
				"size":     file.Size,
			},
		})
	}
}

// handleTestCompletion processes test results
func (am *AgentManager) handleTestCompletion(build *Build, output *TaskOutput) {
	// Check if tests passed
	// If not, create fix tasks

	build.mu.Lock()
	build.Status = BuildReviewing
	build.UpdatedAt = time.Now()
	build.mu.Unlock()
}

// handleReviewCompletion processes code review results
func (am *AgentManager) handleReviewCompletion(build *Build, output *TaskOutput) {
	// Apply any suggested fixes
	// Create fix tasks if needed
}

// updateBuildProgress calculates and updates overall build progress
func (am *AgentManager) updateBuildProgress(build *Build) {
	build.mu.Lock()
	defer build.mu.Unlock()

	if len(build.Tasks) == 0 {
		return
	}

	completed := 0
	for _, task := range build.Tasks {
		if task.Status == TaskCompleted {
			completed++
		}
	}

	progress := (completed * 100) / len(build.Tasks)
	build.Progress = progress
	build.UpdatedAt = time.Now()

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"progress":        progress,
			"tasks_completed": completed,
			"tasks_total":     len(build.Tasks),
		},
	})
}

// checkBuildCompletion determines if the build is finished
func (am *AgentManager) checkBuildCompletion(build *Build) {
	build.mu.RLock()
	allComplete := true
	anyFailed := false
	for _, task := range build.Tasks {
		if task.Status != TaskCompleted && task.Status != TaskFailed && task.Status != TaskCancelled {
			allComplete = false
		}
		if task.Status == TaskFailed {
			anyFailed = true
		}
	}
	build.mu.RUnlock()

	if !allComplete {
		return
	}

	build.mu.Lock()
	now := time.Now()
	build.CompletedAt = &now
	build.UpdatedAt = now

	if anyFailed {
		build.Status = BuildFailed
	} else {
		build.Status = BuildCompleted
		build.Progress = 100
	}
	build.mu.Unlock()

	// Create final checkpoint
	am.createCheckpoint(build, "Build Complete", "All tasks completed successfully")

	// Broadcast completion
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildCompleted,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"status":   string(build.Status),
			"progress": build.Progress,
		},
	})

	log.Printf("Build %s completed with status: %s", build.ID, build.Status)
}

// createCheckpoint saves a checkpoint of the current build state
func (am *AgentManager) createCheckpoint(build *Build, name, description string) *Checkpoint {
	build.mu.Lock()
	defer build.mu.Unlock()

	checkpoint := &Checkpoint{
		ID:          uuid.New().String(),
		BuildID:     build.ID,
		Number:      len(build.Checkpoints) + 1,
		Name:        name,
		Description: description,
		Files:       am.collectGeneratedFiles(build),
		Progress:    build.Progress,
		CreatedAt:   time.Now(),
	}

	build.Checkpoints = append(build.Checkpoints, checkpoint)

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildCheckpoint,
		BuildID:   build.ID,
		Timestamp: checkpoint.CreatedAt,
		Data: map[string]any{
			"checkpoint_id": checkpoint.ID,
			"number":        checkpoint.Number,
			"name":          name,
			"description":   description,
		},
	})

	return checkpoint
}

// collectGeneratedFiles gathers all generated files from completed tasks
func (am *AgentManager) collectGeneratedFiles(build *Build) []GeneratedFile {
	files := make([]GeneratedFile, 0)
	for _, task := range build.Tasks {
		if task.Output != nil {
			files = append(files, task.Output.Files...)
		}
	}
	return files
}

// queuePlanTasks creates and queues tasks based on the build plan
func (am *AgentManager) queuePlanTasks(build *Build) {
	log.Printf("queuePlanTasks called for build %s", build.ID)

	build.mu.RLock()
	agents := make(map[string]*Agent)
	for k, v := range build.Agents {
		agents[k] = v
	}
	description := build.Description
	build.mu.RUnlock()

	// Sort agents by priority to ensure proper ordering
	type agentPriority struct {
		agent    *Agent
		priority int
	}
	sortedAgents := make([]agentPriority, 0)
	for _, agent := range agents {
		if agent.Role == RoleLead {
			continue // Lead already has a task
		}
		sortedAgents = append(sortedAgents, agentPriority{
			agent:    agent,
			priority: am.getPriorityForRole(agent.Role),
		})
	}

	// Sort by priority (higher first)
	for i := 0; i < len(sortedAgents)-1; i++ {
		for j := i + 1; j < len(sortedAgents); j++ {
			if sortedAgents[j].priority > sortedAgents[i].priority {
				sortedAgents[i], sortedAgents[j] = sortedAgents[j], sortedAgents[i]
			}
		}
	}

	// Create and assign tasks to agents
	for _, ap := range sortedAgents {
		agent := ap.agent

		task := &Task{
			ID:          uuid.New().String(),
			Type:        am.getTaskTypeForRole(agent.Role),
			Description: am.getTaskDescriptionForRole(agent.Role, description),
			Priority:    ap.priority,
			Status:      TaskPending,
			MaxRetries:  5,
			Input: map[string]any{
				"app_description": description,
				"agent_role":      string(agent.Role),
			},
			CreatedAt: time.Now(),
		}

		build.mu.Lock()
		build.Tasks = append(build.Tasks, task)
		build.mu.Unlock()

		// Broadcast task created
		am.broadcast(build.ID, &WSMessage{
			Type:      "task:created",
			BuildID:   build.ID,
			AgentID:   agent.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"task_id":     task.ID,
				"task_type":   string(task.Type),
				"description": task.Description,
				"priority":    task.Priority,
				"agent_role":  string(agent.Role),
			},
		})

		if err := am.AssignTask(agent.ID, task); err != nil {
			log.Printf("Failed to assign task to agent %s: %v", agent.ID, err)
		} else {
			log.Printf("Assigned task %s (%s) to agent %s (%s)", task.ID, task.Type, agent.ID, agent.Role)
		}
	}

	log.Printf("Queued %d tasks for build %s", len(sortedAgents), build.ID)
}

// GetBuild retrieves a build by ID
func (am *AgentManager) GetBuild(buildID string) (*Build, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	build, exists := am.builds[buildID]
	if !exists {
		return nil, fmt.Errorf("build %s not found", buildID)
	}
	return build, nil
}

// GetAgent retrieves an agent by ID
func (am *AgentManager) GetAgent(agentID string) (*Agent, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agent, exists := am.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	return agent, nil
}

// SendMessage sends a user message to the build's lead agent
func (am *AgentManager) SendMessage(buildID string, message string) error {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("build %s not found", buildID)
	}

	// Find the lead agent
	var leadAgent *Agent
	build.mu.RLock()
	for _, agent := range build.Agents {
		if agent.Role == RoleLead {
			leadAgent = agent
			break
		}
	}
	build.mu.RUnlock()

	if leadAgent == nil {
		return fmt.Errorf("no lead agent found for build %s", buildID)
	}

	// Broadcast user message
	am.broadcast(buildID, &WSMessage{
		Type:      WSUserMessage,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"content": message,
		},
	})

	// Process message with lead agent
	go am.processUserMessage(leadAgent, message)

	return nil
}

// processUserMessage handles a user message with the lead agent
func (am *AgentManager) processUserMessage(agent *Agent, message string) {
	ctx, cancel := context.WithTimeout(am.ctx, 2*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf("User message: %s\n\nRespond helpfully and briefly.", message)

	response, err := am.aiRouter.Generate(ctx, agent.Provider, prompt, GenerateOptions{
		MaxTokens:    1000,
		Temperature:  0.7,
		SystemPrompt: am.getSystemPrompt(RoleLead),
	})

	if err != nil {
		log.Printf("Failed to process user message: %v", err)
		return
	}

	// Broadcast lead response
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      WSLeadResponse,
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"content": response,
		},
	})
}

// Subscribe adds a channel to receive build updates
func (am *AgentManager) Subscribe(buildID string, ch chan *WSMessage) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.subscribers[buildID] == nil {
		am.subscribers[buildID] = make([]chan *WSMessage, 0)
	}
	am.subscribers[buildID] = append(am.subscribers[buildID], ch)
}

// Unsubscribe removes a channel from build updates
func (am *AgentManager) Unsubscribe(buildID string, ch chan *WSMessage) {
	am.mu.Lock()
	defer am.mu.Unlock()

	subs := am.subscribers[buildID]
	for i, sub := range subs {
		if sub == ch {
			am.subscribers[buildID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

// broadcast sends a message to all subscribers of a build
func (am *AgentManager) broadcast(buildID string, msg *WSMessage) {
	am.mu.RLock()
	subs := am.subscribers[buildID]
	am.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
			// Channel full, skip
		}
	}
}

// RollbackToCheckpoint restores the build to a previous checkpoint
func (am *AgentManager) RollbackToCheckpoint(buildID, checkpointID string) error {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("build %s not found", buildID)
	}

	build.mu.Lock()
	defer build.mu.Unlock()

	// Find the checkpoint
	var targetCheckpoint *Checkpoint
	var checkpointIndex int
	for i, cp := range build.Checkpoints {
		if cp.ID == checkpointID {
			targetCheckpoint = cp
			checkpointIndex = i
			break
		}
	}

	if targetCheckpoint == nil {
		return fmt.Errorf("checkpoint %s not found", checkpointID)
	}

	// Remove checkpoints after the target
	build.Checkpoints = build.Checkpoints[:checkpointIndex+1]
	build.Progress = targetCheckpoint.Progress
	build.Status = BuildInProgress
	build.UpdatedAt = time.Now()

	log.Printf("Rolled back build %s to checkpoint %s", buildID, checkpointID)
	return nil
}

// Shutdown gracefully stops the agent manager
func (am *AgentManager) Shutdown() {
	am.cancel()
	close(am.taskQueue)
	close(am.resultQueue)
	log.Println("Agent Manager shut down")
}

// Helper functions

func (am *AgentManager) buildTaskPrompt(task *Task, build *Build, agent *Agent) string {
	// Check if there are previous errors to learn from
	errorContext := ""
	if prevErrors, ok := task.Input["previous_errors"]; ok {
		errorContext = fmt.Sprintf(`
PREVIOUS ATTEMPTS FAILED - LEARN FROM THESE ERRORS:
%v

Analyze what went wrong and use a different approach this time.
`, prevErrors)
	}

	return fmt.Sprintf(`Task: %s

Description: %s

App being built: %s
%s
OUTPUT FORMAT - CRITICAL:
For EVERY file you create, use this EXACT format:

// File: path/to/filename.ext
`+"```"+`language
[complete file content here]
`+"```"+`

Example:
// File: src/components/App.tsx
`+"```"+`typescript
import React from 'react';
export const App: React.FC = () => {
  return <div>Hello World</div>;
};
`+"```"+`

// File: src/api/server.ts
`+"```"+`typescript
import express from 'express';
const app = express();
// complete implementation...
`+"```"+`

MANDATORY REQUIREMENTS:
1. Output COMPLETE, PRODUCTION-READY code only
2. Mark EVERY file with "// File: path/filename" comment before its code block
3. NEVER use placeholder data, mock responses, TODO comments, or demo content
4. If external API keys or credentials are needed:
   - Use environment variable patterns (e.g., process.env.API_KEY)
   - Add ONE clear comment indicating what the user must provide
   - Build ALL other functionality completely without waiting
5. Include all imports, error handling, and edge cases
6. Every function must be fully implemented and working

FORBIDDEN OUTPUTS:
- "// TODO: implement this"
- Mock or fake data
- Placeholder functions
- Demo or example code
- Incomplete implementations

Build the REAL, COMPLETE implementation now.`,
		task.Type, task.Description, build.Description, errorContext)
}

func (am *AgentManager) getSystemPrompt(role AgentRole) string {
	baseRules := `
ABSOLUTE RULES FOR ALL AGENTS:
1. NEVER output demo code, mock data, placeholder content, or TODO comments
2. ALWAYS produce complete, production-ready, fully functional code
3. If external resources are needed (API keys, credentials), either:
   a) Ask the user to provide them, OR
   b) Use environment variable patterns and build everything else completely
4. Build as much functionality as possible without blocking on user input
5. Real implementations only - no stubs, no examples, no "this would be" code`

	prompts := map[AgentRole]string{
		RoleLead: `You are the Lead Agent coordinating the APEX.BUILD platform.
You oversee all other agents and communicate with users.
Be helpful, concise, and focused on delivering excellent production-ready results.
When users need to provide information (API keys, credentials), clearly ask for it.
When you can proceed without user input, do so and build maximum functionality.` + baseRules,

		RolePlanner: `You are the Planning Agent. Analyze user requirements and create detailed, actionable build plans.
Break down the app into specific components, data models, and APIs.
Identify what external resources the user needs to provide (API keys, etc.).
Output structured plans with real file paths and implementations.` + baseRules,

		RoleArchitect: `You are the Architect Agent. Design production-grade system architecture.
Make concrete technology decisions with specific libraries and versions.
Consider scalability, maintainability, and security from day one.
Provide actual architecture code, not just diagrams or descriptions.` + baseRules,

		RoleFrontend: `You are the Frontend Agent. Build beautiful, responsive, production-ready user interfaces.
Use modern React patterns with TypeScript, proper component structure, and clean styling.
Every component must be complete with all props, state, and handlers implemented.
No placeholder UI - every button must work, every form must submit.` + baseRules,

		RoleBackend: `You are the Backend Agent. Create robust, secure APIs and business logic.
Implement comprehensive error handling, input validation, and security measures.
Every endpoint must be complete with authentication, authorization, and data validation.
No placeholder routes - every endpoint must be fully functional.` + baseRules,

		RoleDatabase: `You are the Database Agent. Design efficient, normalized schemas with proper constraints.
Create complete migration files with indexes, foreign keys, and seed data.
Every query must be optimized and handle edge cases.
No placeholder schemas - include all fields, relationships, and constraints.` + baseRules,

		RoleTesting: `You are the Testing Agent. Write comprehensive, executable tests.
Cover unit tests, integration tests, edge cases, and error scenarios.
Every test must actually run and verify real functionality.
No placeholder tests - include real assertions and complete coverage.` + baseRules,

		RoleReviewer: `You are the Reviewer Agent. Review code for production-readiness.
Identify security vulnerabilities, performance issues, and incomplete implementations.
Provide specific, actionable fixes with complete code - not just suggestions.
Flag any placeholder code, mock data, or TODOs for immediate replacement.` + baseRules,
	}
	return prompts[role]
}

func (am *AgentManager) getTaskTypeForRole(role AgentRole) TaskType {
	switch role {
	case RolePlanner:
		return TaskPlan
	case RoleArchitect:
		return TaskArchitecture
	case RoleFrontend:
		return TaskGenerateUI
	case RoleBackend:
		return TaskGenerateAPI
	case RoleDatabase:
		return TaskGenerateSchema
	case RoleTesting:
		return TaskTest
	case RoleReviewer:
		return TaskReview
	default:
		return TaskGenerateFile
	}
}

func (am *AgentManager) getTaskDescriptionForRole(role AgentRole, appDescription string) string {
	switch role {
	case RoleArchitect:
		return fmt.Sprintf("Design the architecture for: %s", appDescription)
	case RoleFrontend:
		return fmt.Sprintf("Build the frontend UI for: %s", appDescription)
	case RoleBackend:
		return fmt.Sprintf("Create the backend API for: %s", appDescription)
	case RoleDatabase:
		return fmt.Sprintf("Design the database schema for: %s", appDescription)
	case RoleTesting:
		return fmt.Sprintf("Write tests for: %s", appDescription)
	case RoleReviewer:
		return fmt.Sprintf("Review code quality for: %s", appDescription)
	default:
		return appDescription
	}
}

func (am *AgentManager) getPriorityForRole(role AgentRole) int {
	switch role {
	case RoleArchitect:
		return 90
	case RoleDatabase:
		return 80
	case RoleBackend:
		return 70
	case RoleFrontend:
		return 60
	case RoleTesting:
		return 50
	case RoleReviewer:
		return 40
	default:
		return 50
	}
}

func (am *AgentManager) parseTaskOutput(taskType TaskType, response string) *TaskOutput {
	output := &TaskOutput{
		Messages: []string{},
		Files:    make([]GeneratedFile, 0),
	}

	// Parse the AI response to extract code blocks and files
	// Look for patterns like ```language\n...code...\n``` or file markers
	lines := strings.Split(response, "\n")
	var currentFile *GeneratedFile
	var codeBuffer strings.Builder
	inCodeBlock := false
	currentLanguage := ""

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for file path markers like "// File: path/to/file.ts" or "# File: path/to/file.py"
		if strings.HasPrefix(trimmedLine, "// File:") || strings.HasPrefix(trimmedLine, "# File:") ||
			strings.HasPrefix(trimmedLine, "/* File:") || strings.HasPrefix(trimmedLine, "<!-- File:") {
			// Save previous file if any
			if currentFile != nil && codeBuffer.Len() > 0 {
				currentFile.Content = strings.TrimSpace(codeBuffer.String())
				currentFile.Size = int64(len(currentFile.Content))
				output.Files = append(output.Files, *currentFile)
			}

			// Extract file path
			filePath := ""
			if strings.HasPrefix(trimmedLine, "// File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "// File:"))
			} else if strings.HasPrefix(trimmedLine, "# File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "# File:"))
			} else if strings.HasPrefix(trimmedLine, "/* File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "/* File:"))
				filePath = strings.TrimSuffix(filePath, "*/")
			} else if strings.HasPrefix(trimmedLine, "<!-- File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "<!-- File:"))
				filePath = strings.TrimSuffix(filePath, "-->")
			}

			if filePath != "" {
				currentFile = &GeneratedFile{
					Path:     strings.TrimSpace(filePath),
					Language: am.detectLanguage(filePath),
					IsNew:    true,
				}
				codeBuffer.Reset()
			}
			continue
		}

		// Check for code block start
		if strings.HasPrefix(trimmedLine, "```") {
			if !inCodeBlock {
				// Starting a code block
				inCodeBlock = true
				currentLanguage = strings.TrimPrefix(trimmedLine, "```")
				currentLanguage = strings.TrimSpace(currentLanguage)

				// If we don't have a current file, create one from the code block
				if currentFile == nil && currentLanguage != "" {
					ext := am.languageToExtension(currentLanguage)
					currentFile = &GeneratedFile{
						Path:     fmt.Sprintf("generated_%d.%s", len(output.Files)+1, ext),
						Language: currentLanguage,
						IsNew:    true,
					}
				}
				continue
			} else {
				// Ending a code block
				inCodeBlock = false
				if currentFile != nil && codeBuffer.Len() > 0 {
					currentFile.Content = strings.TrimSpace(codeBuffer.String())
					currentFile.Size = int64(len(currentFile.Content))
					output.Files = append(output.Files, *currentFile)
					currentFile = nil
					codeBuffer.Reset()
				}
				continue
			}
		}

		// Add line to buffer if in code block or if we have a current file
		if inCodeBlock || currentFile != nil {
			if codeBuffer.Len() > 0 {
				codeBuffer.WriteString("\n")
			}
			codeBuffer.WriteString(line)
		} else if i < 5 || i > len(lines)-5 {
			// Add non-code content as messages (first and last few lines typically explanations)
			if trimmedLine != "" {
				output.Messages = append(output.Messages, trimmedLine)
			}
		}
	}

	// Handle any remaining file content
	if currentFile != nil && codeBuffer.Len() > 0 {
		currentFile.Content = strings.TrimSpace(codeBuffer.String())
		currentFile.Size = int64(len(currentFile.Content))
		output.Files = append(output.Files, *currentFile)
	}

	// If no files were extracted but we have response content, treat the whole response as a single file
	if len(output.Files) == 0 && len(response) > 100 {
		// Try to detect if it's code and create a file
		if am.looksLikeCode(response) {
			lang := am.detectLanguageFromContent(response)
			output.Files = append(output.Files, GeneratedFile{
				Path:     fmt.Sprintf("generated_1.%s", am.languageToExtension(lang)),
				Content:  response,
				Language: lang,
				Size:     int64(len(response)),
				IsNew:    true,
			})
		} else {
			output.Messages = []string{response}
		}
	}

	return output
}

// detectLanguage determines the programming language from a file path
func (am *AgentManager) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".sh":
		return "bash"
	default:
		return "text"
	}
}

// languageToExtension converts a language name to file extension
func (am *AgentManager) languageToExtension(lang string) string {
	lang = strings.ToLower(lang)
	switch lang {
	case "typescript", "tsx":
		return "ts"
	case "javascript", "jsx":
		return "js"
	case "python":
		return "py"
	case "go", "golang":
		return "go"
	case "rust":
		return "rs"
	case "java":
		return "java"
	case "html":
		return "html"
	case "css":
		return "css"
	case "sql":
		return "sql"
	case "json":
		return "json"
	case "yaml":
		return "yaml"
	case "markdown", "md":
		return "md"
	case "bash", "shell", "sh":
		return "sh"
	default:
		return "txt"
	}
}

// looksLikeCode checks if content appears to be code
func (am *AgentManager) looksLikeCode(content string) bool {
	codeIndicators := []string{
		"function ", "const ", "let ", "var ", "import ", "export ",
		"class ", "interface ", "type ", "def ", "async ", "await ",
		"return ", "if (", "for (", "while (", "package ", "func ",
		"public ", "private ", "protected ", "struct ", "enum ",
	}
	contentLower := strings.ToLower(content)
	for _, indicator := range codeIndicators {
		if strings.Contains(contentLower, indicator) {
			return true
		}
	}
	return false
}

// detectLanguageFromContent attempts to determine language from code content
func (am *AgentManager) detectLanguageFromContent(content string) string {
	contentLower := strings.ToLower(content)

	if strings.Contains(content, "import React") || strings.Contains(content, "from 'react'") ||
		strings.Contains(content, "interface ") && strings.Contains(content, ": ") {
		return "typescript"
	}
	if strings.Contains(content, "package main") || strings.Contains(content, "func ") && strings.Contains(content, "{}") {
		return "go"
	}
	if strings.Contains(content, "def ") && strings.Contains(content, ":") && !strings.Contains(content, "{") {
		return "python"
	}
	if strings.Contains(contentLower, "function ") || strings.Contains(contentLower, "const ") ||
		strings.Contains(contentLower, "let ") {
		return "javascript"
	}
	if strings.Contains(content, "fn ") && strings.Contains(content, "-> ") {
		return "rust"
	}
	if strings.Contains(content, "public class ") || strings.Contains(content, "private class ") {
		return "java"
	}

	return "text"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
