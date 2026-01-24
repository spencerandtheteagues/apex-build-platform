// Package agents - Agent Manager
// This component spawns, tracks, and manages AI agents during builds.
package agents

import (
	"context"
	"fmt"
	"log"
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
	am.mu.Lock()
	build, exists := am.builds[buildID]
	am.mu.Unlock()

	if !exists {
		return fmt.Errorf("build %s not found", buildID)
	}

	// Update status
	build.mu.Lock()
	build.Status = BuildPlanning
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

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
		return fmt.Errorf("failed to spawn lead agent: %w", err)
	}

	// Create planning task
	planTask := &Task{
		ID:          uuid.New().String(),
		Type:        TaskPlan,
		Description: fmt.Sprintf("Create comprehensive build plan for: %s", build.Description),
		Priority:    100,
		Status:      TaskPending,
		Input: map[string]any{
			"description": build.Description,
			"mode":        string(build.Mode),
		},
		CreatedAt: time.Now(),
	}

	// Assign to lead agent
	planTask.AssignedTo = leadAgent.ID
	build.mu.Lock()
	build.Tasks = append(build.Tasks, planTask)
	build.mu.Unlock()

	// Queue the planning task
	am.taskQueue <- planTask

	log.Printf("Build %s started with lead agent %s", buildID, leadAgent.ID)
	return nil
}

// spawnAgent creates a new AI agent with a specific role
func (am *AgentManager) spawnAgent(buildID string, role AgentRole, provider AIProvider) (*Agent, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	build, exists := am.builds[buildID]
	if !exists {
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

	// Broadcast agent spawned
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
	am.mu.RLock()
	agent, exists := am.agents[task.AssignedTo]
	am.mu.RUnlock()

	if !exists {
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("agent %s not found", task.AssignedTo),
		}
		return
	}

	// Get the build for context
	am.mu.RLock()
	build, buildExists := am.builds[agent.BuildID]
	am.mu.RUnlock()

	if !buildExists {
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   fmt.Errorf("build %s not found", agent.BuildID),
		}
		return
	}

	// Build the prompt based on task type
	prompt := am.buildTaskPrompt(task, build, agent)
	systemPrompt := am.getSystemPrompt(agent.Role)

	// Execute using AI router
	ctx, cancel := context.WithTimeout(am.ctx, 5*time.Minute)
	defer cancel()

	response, err := am.aiRouter.Generate(ctx, agent.Provider, prompt, GenerateOptions{
		MaxTokens:    8000,
		Temperature:  0.7,
		SystemPrompt: systemPrompt,
	})

	if err != nil {
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   err,
		}
		return
	}

	// Parse the response into task output
	output := am.parseTaskOutput(task.Type, response)

	am.resultQueue <- &TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output:  output,
	}
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

// processResult handles a task result
func (am *AgentManager) processResult(result *TaskResult) {
	am.mu.RLock()
	agent, agentExists := am.agents[result.AgentID]
	am.mu.RUnlock()

	if !agentExists {
		log.Printf("Warning: result for unknown agent %s", result.AgentID)
		return
	}

	agent.mu.Lock()
	if result.Success {
		agent.Status = StatusCompleted
		if agent.CurrentTask != nil {
			agent.CurrentTask.Status = TaskCompleted
			now := time.Now()
			agent.CurrentTask.CompletedAt = &now
			agent.CurrentTask.Output = result.Output
		}
	} else {
		agent.Status = StatusError
		agent.Error = result.Error.Error()
		if agent.CurrentTask != nil {
			agent.CurrentTask.Status = TaskFailed
			agent.CurrentTask.Error = result.Error.Error()
		}
	}
	agent.UpdatedAt = time.Now()
	task := agent.CurrentTask
	agent.mu.Unlock()

	// Broadcast completion or error
	msgType := WSAgentCompleted
	if !result.Success {
		msgType = WSAgentError
	}

	am.broadcast(agent.BuildID, &WSMessage{
		Type:      msgType,
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"task_id": result.TaskID,
			"success": result.Success,
			"output":  result.Output,
			"error":   getErrorString(result.Error),
		},
	})

	// Handle task completion - may trigger next tasks
	if result.Success && task != nil {
		am.handleTaskCompletion(agent.BuildID, task, result.Output)
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
	// Parse the plan from output
	// In reality, this would parse the AI response into a structured plan

	build.mu.Lock()
	build.Status = BuildInProgress
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	// Spawn the full agent team
	if err := am.SpawnAgentTeam(build.ID); err != nil {
		log.Printf("Error spawning agent team: %v", err)
		return
	}

	// Create checkpoint for planning phase
	am.createCheckpoint(build, "Planning Complete", "Build plan created and agent team spawned")

	// Queue initial tasks for each agent based on the plan
	am.queuePlanTasks(build)
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
	// This would parse the plan and create appropriate tasks
	// For now, create a basic set of tasks for the agents

	build.mu.Lock()
	agents := build.Agents
	build.mu.Unlock()

	for _, agent := range agents {
		if agent.Role == RoleLead {
			continue // Lead already has a task
		}

		task := &Task{
			ID:          uuid.New().String(),
			Type:        am.getTaskTypeForRole(agent.Role),
			Description: am.getTaskDescriptionForRole(agent.Role, build.Description),
			Priority:    am.getPriorityForRole(agent.Role),
			Status:      TaskPending,
			CreatedAt:   time.Now(),
		}

		build.mu.Lock()
		build.Tasks = append(build.Tasks, task)
		build.mu.Unlock()

		if err := am.AssignTask(agent.ID, task); err != nil {
			log.Printf("Failed to assign task to agent %s: %v", agent.ID, err)
		}
	}
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
	return fmt.Sprintf(`Task: %s

Description: %s

App being built: %s

Provide a complete, production-ready solution. No placeholders or TODOs.`,
		task.Type, task.Description, build.Description)
}

func (am *AgentManager) getSystemPrompt(role AgentRole) string {
	prompts := map[AgentRole]string{
		RoleLead: `You are the Lead Agent coordinating the APEX.BUILD platform.
You oversee all other agents and communicate with users.
Be helpful, concise, and focused on delivering excellent results.`,

		RolePlanner: `You are the Planning Agent. Analyze user requirements and create detailed build plans.
Break down the app into components, data models, and APIs.
Output structured plans that other agents can execute.`,

		RoleArchitect: `You are the Architect Agent. Design system architecture and make technology decisions.
Consider scalability, maintainability, and best practices.
Provide clear architectural diagrams and decisions.`,

		RoleFrontend: `You are the Frontend Agent. Build beautiful, responsive user interfaces.
Use modern React patterns, proper component structure, and clean styling.
Output complete, working code with no placeholders.`,

		RoleBackend: `You are the Backend Agent. Create robust APIs and business logic.
Implement proper error handling, validation, and security.
Output complete, working code with no placeholders.`,

		RoleDatabase: `You are the Database Agent. Design efficient schemas and write optimal queries.
Consider relationships, indexes, and data integrity.
Output complete migration files and query implementations.`,

		RoleTesting: `You are the Testing Agent. Write comprehensive tests for all code.
Cover unit tests, integration tests, and edge cases.
Output complete test files with proper assertions.`,

		RoleReviewer: `You are the Reviewer Agent. Review code for quality, security, and best practices.
Identify issues and suggest improvements.
Be thorough but constructive in your feedback.`,
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
	// Parse the AI response into structured output
	// This would extract files, messages, etc. from the response
	return &TaskOutput{
		Messages: []string{response},
		Files:    make([]GeneratedFile, 0), // Would parse actual files
	}
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
