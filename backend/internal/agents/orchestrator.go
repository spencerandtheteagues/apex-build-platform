// Package agents - Enhanced Multi-AI Orchestrator
// This orchestrator coordinates Claude Opus 4.5, GPT-5, and Gemini 3 working in parallel
// with specialized sub-agents to build applications quickly and with high quality.
package agents

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AIModelRole defines the primary role of each AI model
type AIModelRole string

const (
	ModelRoleStrategist AIModelRole = "strategist" // Claude Opus 4.5 - Planning, Architecture, Review
	ModelRoleCoder      AIModelRole = "coder"      // GPT-5 - Code Generation, Implementation
	ModelRoleValidator  AIModelRole = "validator"  // Gemini 3 - Testing, Optimization, Speed
)

// SubAgentType defines specialized sub-agent types
type SubAgentType string

const (
	// Claude Opus 4.5 Sub-Agents (Strategic)
	SubAgentArchitect     SubAgentType = "architect"      // System design and architecture
	SubAgentPlanner       SubAgentType = "planner"        // Task breakdown and planning
	SubAgentReviewer      SubAgentType = "reviewer"       // Code review and quality
	SubAgentDocumentor    SubAgentType = "documentor"     // Documentation generation

	// GPT-5 Sub-Agents (Creative Coding)
	SubAgentFrontendDev   SubAgentType = "frontend_dev"   // React/Vue/UI components
	SubAgentBackendDev    SubAgentType = "backend_dev"    // API and server logic
	SubAgentAPIDev        SubAgentType = "api_dev"        // REST/GraphQL endpoints
	SubAgentUIDev         SubAgentType = "ui_dev"         // Styling and UX
	SubAgentDatabaseDev   SubAgentType = "database_dev"   // Schema and queries

	// Gemini 3 Sub-Agents (Speed & Validation)
	SubAgentTester        SubAgentType = "tester"         // Unit and integration tests
	SubAgentOptimizer     SubAgentType = "optimizer"      // Performance optimization
	SubAgentDebugger      SubAgentType = "debugger"       // Bug detection and fixes
	SubAgentCompleter     SubAgentType = "completer"      // Code completion and polish
)

// SubAgent represents a specialized sub-agent spawned by a main AI model
type SubAgent struct {
	ID           string        `json:"id"`
	Type         SubAgentType  `json:"type"`
	ParentModel  AIModelRole   `json:"parent_model"`
	Provider     AIProvider    `json:"provider"`
	Status       AgentStatus   `json:"status"`
	BuildID      string        `json:"build_id"`
	CurrentTask  *Task         `json:"current_task,omitempty"`
	Instructions string        `json:"instructions"`
	Progress     int           `json:"progress"`
	Output       []string      `json:"output,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	mu           sync.RWMutex
}

// BuildPhase represents the current phase of the build process
type BuildPhase string

const (
	PhaseInitializing BuildPhase = "initializing"
	PhasePlanning     BuildPhase = "planning"     // Claude leads
	PhaseArchitecture BuildPhase = "architecture" // Claude leads
	PhaseCoding       BuildPhase = "coding"       // GPT-5 leads, Gemini assists
	PhaseTesting      BuildPhase = "testing"      // Gemini leads
	PhaseReview       BuildPhase = "review"       // Claude leads
	PhaseOptimization BuildPhase = "optimization" // Gemini leads
	PhaseComplete     BuildPhase = "complete"
)

// Orchestrator coordinates multiple AI models and their sub-agents
type Orchestrator struct {
	buildID       string
	phase         BuildPhase
	subAgents     map[string]*SubAgent
	taskQueue     chan *OrchestratorTask
	resultQueue   chan *OrchestratorResult
	aiRouter      AIRouter
	subscribers   []chan *WSMessage
	generatedCode map[string]string // path -> content
	previewURL    string
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// OrchestratorTask represents a task for the orchestrator
type OrchestratorTask struct {
	ID          string       `json:"id"`
	SubAgentID  string       `json:"sub_agent_id"`
	Type        SubAgentType `json:"type"`
	Description string       `json:"description"`
	Input       map[string]any `json:"input"`
	Priority    int          `json:"priority"`
	Phase       BuildPhase   `json:"phase"`
	CreatedAt   time.Time    `json:"created_at"`
}

// OrchestratorResult holds the result of a sub-agent task
type OrchestratorResult struct {
	TaskID     string           `json:"task_id"`
	SubAgentID string           `json:"sub_agent_id"`
	Success    bool             `json:"success"`
	Output     *TaskOutput      `json:"output,omitempty"`
	Files      []GeneratedFile  `json:"files,omitempty"`
	Error      error            `json:"error,omitempty"`
}

// NewOrchestrator creates a new multi-AI orchestrator for a build
func NewOrchestrator(buildID string, aiRouter AIRouter) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	o := &Orchestrator{
		buildID:       buildID,
		phase:         PhaseInitializing,
		subAgents:     make(map[string]*SubAgent),
		taskQueue:     make(chan *OrchestratorTask, 50),
		resultQueue:   make(chan *OrchestratorResult, 50),
		aiRouter:      aiRouter,
		subscribers:   make([]chan *WSMessage, 0),
		generatedCode: make(map[string]string),
		ctx:           ctx,
		cancel:        cancel,
	}

	go o.taskProcessor()
	go o.resultProcessor()

	return o
}

// StartBuild initiates the multi-AI build process
func (o *Orchestrator) StartBuild(description string, mode BuildMode) error {
	o.mu.Lock()
	o.phase = PhasePlanning
	o.mu.Unlock()

	o.broadcast(&WSMessage{
		Type:      WSBuildStarted,
		BuildID:   o.buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":       string(PhasePlanning),
			"description": description,
			"message":     "Multi-AI orchestration initiated. Claude Opus 4.5, GPT-5, and Gemini 3 are now collaborating...",
		},
	})

	// Phase 1: Spawn Claude's strategic sub-agents for planning
	if err := o.spawnClaudeSubAgents(); err != nil {
		return fmt.Errorf("failed to spawn Claude sub-agents: %w", err)
	}

	// Create planning task for Claude's Planner
	plannerID := o.getSubAgentByType(SubAgentPlanner)
	if plannerID != "" {
		o.queueTask(&OrchestratorTask{
			ID:          uuid.New().String(),
			SubAgentID:  plannerID,
			Type:        SubAgentPlanner,
			Description: fmt.Sprintf("Create comprehensive build plan for: %s", description),
			Input: map[string]any{
				"description": description,
				"mode":        string(mode),
			},
			Priority:  100,
			Phase:     PhasePlanning,
			CreatedAt: time.Now(),
		})
	}

	// Create architecture task for Claude's Architect
	architectID := o.getSubAgentByType(SubAgentArchitect)
	if architectID != "" {
		o.queueTask(&OrchestratorTask{
			ID:          uuid.New().String(),
			SubAgentID:  architectID,
			Type:        SubAgentArchitect,
			Description: fmt.Sprintf("Design system architecture for: %s", description),
			Input: map[string]any{
				"description": description,
			},
			Priority:  95,
			Phase:     PhaseArchitecture,
			CreatedAt: time.Now(),
		})
	}

	return nil
}

// spawnClaudeSubAgents creates Claude Opus 4.5's specialized sub-agents
func (o *Orchestrator) spawnClaudeSubAgents() error {
	claudeSubAgents := []struct {
		Type         SubAgentType
		Instructions string
	}{
		{
			Type: SubAgentArchitect,
			Instructions: `You are the ARCHITECT sub-agent powered by Claude Opus 4.5.
Your role is to design robust, scalable system architectures.

RESPONSIBILITIES:
- Analyze requirements and identify system components
- Design database schemas with proper relationships
- Define API contracts and data flow
- Choose appropriate tech stack and patterns
- Create component diagrams and architecture docs

OUTPUT FORMAT:
Always output structured JSON with:
{
  "architecture": {
    "components": [...],
    "data_flow": [...],
    "tech_stack": {...},
    "database_schema": {...}
  }
}`,
		},
		{
			Type: SubAgentPlanner,
			Instructions: `You are the PLANNER sub-agent powered by Claude Opus 4.5.
Your role is to break down complex apps into actionable tasks.

RESPONSIBILITIES:
- Analyze user requirements thoroughly
- Break down the app into discrete, parallelizable tasks
- Prioritize tasks based on dependencies
- Estimate time and complexity for each task
- Coordinate with other sub-agents

OUTPUT FORMAT:
Always output structured JSON with:
{
  "plan": {
    "phases": [...],
    "tasks": [...],
    "dependencies": [...],
    "estimated_time": "..."
  }
}`,
		},
		{
			Type: SubAgentReviewer,
			Instructions: `You are the REVIEWER sub-agent powered by Claude Opus 4.5.
Your role is to ensure code quality and best practices.

RESPONSIBILITIES:
- Review all generated code for quality
- Check for security vulnerabilities
- Ensure consistent coding standards
- Identify potential bugs and issues
- Suggest improvements and optimizations

OUTPUT FORMAT:
Always output structured JSON with:
{
  "review": {
    "issues": [...],
    "suggestions": [...],
    "security_concerns": [...],
    "approved": true/false
  }
}`,
		},
		{
			Type: SubAgentDocumentor,
			Instructions: `You are the DOCUMENTOR sub-agent powered by Claude Opus 4.5.
Your role is to create comprehensive documentation.

RESPONSIBILITIES:
- Generate README files
- Create API documentation
- Write inline code comments
- Document setup and deployment
- Create user guides

OUTPUT FORMAT:
Generate complete markdown documentation files.`,
		},
	}

	for _, config := range claudeSubAgents {
		if err := o.spawnSubAgent(config.Type, ModelRoleStrategist, ProviderClaude, config.Instructions); err != nil {
			return err
		}
	}

	return nil
}

// spawnGPT5SubAgents creates GPT-5's specialized coding sub-agents
func (o *Orchestrator) spawnGPT5SubAgents() error {
	gptSubAgents := []struct {
		Type         SubAgentType
		Instructions string
	}{
		{
			Type: SubAgentFrontendDev,
			Instructions: `You are the FRONTEND DEVELOPER sub-agent powered by GPT-5.
Your role is to build beautiful, responsive user interfaces.

RESPONSIBILITIES:
- Create React/Vue components with TypeScript
- Implement responsive layouts with Tailwind/CSS
- Handle state management (Redux/Zustand/Context)
- Build forms with validation
- Implement routing and navigation

CODING STANDARDS:
- Use functional components with hooks
- Implement proper TypeScript types
- Follow accessibility best practices
- Write clean, maintainable code
- NO placeholders or TODOs - complete implementations only

OUTPUT FORMAT:
{
  "files": [
    {"path": "src/components/...", "content": "...", "language": "typescript"}
  ]
}`,
		},
		{
			Type: SubAgentBackendDev,
			Instructions: `You are the BACKEND DEVELOPER sub-agent powered by GPT-5.
Your role is to create robust server-side logic and APIs.

RESPONSIBILITIES:
- Build RESTful or GraphQL APIs
- Implement business logic
- Handle authentication and authorization
- Create middleware and utilities
- Implement error handling

CODING STANDARDS:
- Use proper error handling patterns
- Implement input validation
- Follow security best practices
- Write efficient, scalable code
- NO placeholders or TODOs - complete implementations only

OUTPUT FORMAT:
{
  "files": [
    {"path": "server/...", "content": "...", "language": "typescript"}
  ]
}`,
		},
		{
			Type: SubAgentAPIDev,
			Instructions: `You are the API DEVELOPER sub-agent powered by GPT-5.
Your role is to design and implement API endpoints.

RESPONSIBILITIES:
- Design RESTful API endpoints
- Implement request/response handling
- Create API documentation
- Handle authentication flows
- Implement rate limiting and caching

OUTPUT FORMAT:
{
  "files": [
    {"path": "api/...", "content": "...", "language": "typescript"}
  ],
  "api_spec": {...}
}`,
		},
		{
			Type: SubAgentUIDev,
			Instructions: `You are the UI/UX DEVELOPER sub-agent powered by GPT-5.
Your role is to create stunning visual designs and interactions.

RESPONSIBILITIES:
- Design beautiful UI components
- Implement animations and transitions
- Create consistent design systems
- Build responsive layouts
- Ensure great user experience

OUTPUT FORMAT:
{
  "files": [
    {"path": "src/styles/...", "content": "...", "language": "css"}
  ]
}`,
		},
		{
			Type: SubAgentDatabaseDev,
			Instructions: `You are the DATABASE DEVELOPER sub-agent powered by GPT-5.
Your role is to design and implement database solutions.

RESPONSIBILITIES:
- Create database schemas
- Write migrations
- Implement data models
- Optimize queries
- Handle data relationships

OUTPUT FORMAT:
{
  "files": [
    {"path": "database/...", "content": "...", "language": "sql"}
  ],
  "schema": {...}
}`,
		},
	}

	for _, config := range gptSubAgents {
		if err := o.spawnSubAgent(config.Type, ModelRoleCoder, ProviderGPT, config.Instructions); err != nil {
			return err
		}
	}

	return nil
}

// spawnGeminiSubAgents creates Gemini 3's validation sub-agents
func (o *Orchestrator) spawnGeminiSubAgents() error {
	geminiSubAgents := []struct {
		Type         SubAgentType
		Instructions string
	}{
		{
			Type: SubAgentTester,
			Instructions: `You are the TESTER sub-agent powered by Gemini 3.
Your role is to ensure code quality through comprehensive testing.

RESPONSIBILITIES:
- Write unit tests for all components
- Create integration tests
- Test edge cases and error handling
- Verify API endpoints
- Test UI components

TESTING STANDARDS:
- Use Jest/Vitest for JavaScript
- Achieve high code coverage
- Test both happy and error paths
- Mock external dependencies
- Write clear test descriptions

OUTPUT FORMAT:
{
  "files": [
    {"path": "tests/...", "content": "...", "language": "typescript"}
  ],
  "coverage": {...}
}`,
		},
		{
			Type: SubAgentOptimizer,
			Instructions: `You are the OPTIMIZER sub-agent powered by Gemini 3.
Your role is to improve performance and efficiency.

RESPONSIBILITIES:
- Identify performance bottlenecks
- Optimize database queries
- Improve frontend rendering
- Reduce bundle sizes
- Implement caching strategies

OUTPUT FORMAT:
{
  "optimizations": [...],
  "files": [...]
}`,
		},
		{
			Type: SubAgentDebugger,
			Instructions: `You are the DEBUGGER sub-agent powered by Gemini 3.
Your role is to identify and fix bugs quickly.

RESPONSIBILITIES:
- Analyze error messages
- Trace bug sources
- Implement fixes
- Verify fixes work
- Prevent regression

OUTPUT FORMAT:
{
  "bugs_found": [...],
  "fixes": [...],
  "files": [...]
}`,
		},
		{
			Type: SubAgentCompleter,
			Instructions: `You are the COMPLETER sub-agent powered by Gemini 3.
Your role is to polish and complete code quickly.

RESPONSIBILITIES:
- Fill in missing implementations
- Add error handling
- Complete partial code
- Add type definitions
- Polish rough edges

OUTPUT FORMAT:
{
  "completions": [...],
  "files": [...]
}`,
		},
	}

	for _, config := range geminiSubAgents {
		if err := o.spawnSubAgent(config.Type, ModelRoleValidator, ProviderGemini, config.Instructions); err != nil {
			return err
		}
	}

	return nil
}

// spawnSubAgent creates a new sub-agent
func (o *Orchestrator) spawnSubAgent(agentType SubAgentType, parentModel AIModelRole, provider AIProvider, instructions string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()

	subAgent := &SubAgent{
		ID:           id,
		Type:         agentType,
		ParentModel:  parentModel,
		Provider:     provider,
		Status:       StatusIdle,
		BuildID:      o.buildID,
		Instructions: instructions,
		Progress:     0,
		Output:       make([]string, 0),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	o.subAgents[id] = subAgent

	o.broadcast(&WSMessage{
		Type:      WSAgentSpawned,
		BuildID:   o.buildID,
		AgentID:   id,
		Timestamp: now,
		Data: map[string]any{
			"type":         string(agentType),
			"parent_model": string(parentModel),
			"provider":     string(provider),
			"message":      fmt.Sprintf("%s sub-agent spawned by %s", agentType, parentModel),
		},
	})

	log.Printf("Spawned %s sub-agent %s (parent: %s, provider: %s)", agentType, id, parentModel, provider)
	return nil
}

// getSubAgentByType finds a sub-agent by type
func (o *Orchestrator) getSubAgentByType(agentType SubAgentType) string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for id, agent := range o.subAgents {
		if agent.Type == agentType {
			return id
		}
	}
	return ""
}

// queueTask adds a task to the queue
func (o *Orchestrator) queueTask(task *OrchestratorTask) {
	o.taskQueue <- task
}

// taskProcessor handles tasks from the queue
func (o *Orchestrator) taskProcessor() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case task := <-o.taskQueue:
			go o.executeTask(task)
		}
	}
}

// executeTask runs a task with the appropriate sub-agent
func (o *Orchestrator) executeTask(task *OrchestratorTask) {
	o.mu.RLock()
	subAgent, exists := o.subAgents[task.SubAgentID]
	o.mu.RUnlock()

	if !exists {
		o.resultQueue <- &OrchestratorResult{
			TaskID:     task.ID,
			SubAgentID: task.SubAgentID,
			Success:    false,
			Error:      fmt.Errorf("sub-agent %s not found", task.SubAgentID),
		}
		return
	}

	// Update sub-agent status
	subAgent.mu.Lock()
	subAgent.Status = StatusWorking
	subAgent.CurrentTask = &Task{
		ID:          task.ID,
		Type:        TaskType(task.Type),
		Description: task.Description,
	}
	subAgent.UpdatedAt = time.Now()
	subAgent.mu.Unlock()

	// Broadcast working status
	o.broadcast(&WSMessage{
		Type:      WSAgentWorking,
		BuildID:   o.buildID,
		AgentID:   subAgent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"task_id":     task.ID,
			"type":        string(task.Type),
			"description": task.Description,
			"message":     fmt.Sprintf("%s is working on: %s", task.Type, truncate(task.Description, 50)),
		},
	})

	// Build prompt with instructions
	prompt := fmt.Sprintf(`%s

TASK: %s

INPUT:
%v

Provide a complete, production-ready solution. Output valid JSON as specified in your instructions.`,
		subAgent.Instructions, task.Description, task.Input)

	// Execute with AI router
	ctx, cancel := context.WithTimeout(o.ctx, 5*time.Minute)
	defer cancel()

	response, err := o.aiRouter.Generate(ctx, subAgent.Provider, prompt, GenerateOptions{
		MaxTokens:   8000,
		Temperature: 0.7,
	})

	if err != nil {
		o.resultQueue <- &OrchestratorResult{
			TaskID:     task.ID,
			SubAgentID: subAgent.ID,
			Success:    false,
			Error:      err,
		}
		return
	}

	// Parse response and extract files
	files := o.parseGeneratedFiles(response)

	o.resultQueue <- &OrchestratorResult{
		TaskID:     task.ID,
		SubAgentID: subAgent.ID,
		Success:    true,
		Output: &TaskOutput{
			Messages: []string{response},
			Files:    files,
		},
		Files: files,
	}
}

// resultProcessor handles completed task results
func (o *Orchestrator) resultProcessor() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case result := <-o.resultQueue:
			o.processResult(result)
		}
	}
}

// processResult handles a task result and triggers next phase if needed
func (o *Orchestrator) processResult(result *OrchestratorResult) {
	o.mu.RLock()
	subAgent, exists := o.subAgents[result.SubAgentID]
	o.mu.RUnlock()

	if !exists {
		return
	}

	subAgent.mu.Lock()
	if result.Success {
		subAgent.Status = StatusCompleted
		subAgent.Progress = 100
		if result.Output != nil {
			subAgent.Output = append(subAgent.Output, result.Output.Messages...)
		}
	} else {
		subAgent.Status = StatusError
	}
	subAgent.UpdatedAt = time.Now()
	subAgent.mu.Unlock()

	// Store generated files
	if result.Success && len(result.Files) > 0 {
		o.mu.Lock()
		for _, file := range result.Files {
			o.generatedCode[file.Path] = file.Content
		}
		o.mu.Unlock()

		// Broadcast file creation
		for _, file := range result.Files {
			o.broadcast(&WSMessage{
				Type:      WSFileCreated,
				BuildID:   o.buildID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"path":     file.Path,
					"language": file.Language,
					"size":     file.Size,
				},
			})
		}
	}

	// Broadcast completion
	msgType := WSAgentCompleted
	if !result.Success {
		msgType = WSAgentError
	}

	o.broadcast(&WSMessage{
		Type:      msgType,
		BuildID:   o.buildID,
		AgentID:   subAgent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"task_id": result.TaskID,
			"success": result.Success,
			"type":    string(subAgent.Type),
		},
	})

	// Check if we should advance to next phase
	o.checkPhaseCompletion()
}

// checkPhaseCompletion determines if current phase is complete and advances
func (o *Orchestrator) checkPhaseCompletion() {
	o.mu.Lock()
	currentPhase := o.phase
	o.mu.Unlock()

	switch currentPhase {
	case PhasePlanning:
		if o.isPhaseComplete(SubAgentPlanner) {
			o.advanceToPhase(PhaseArchitecture)
		}
	case PhaseArchitecture:
		if o.isPhaseComplete(SubAgentArchitect) {
			o.advanceToCodingPhase()
		}
	case PhaseCoding:
		if o.isCodingComplete() {
			o.advanceToPhase(PhaseTesting)
			o.startTestingPhase()
		}
	case PhaseTesting:
		if o.isPhaseComplete(SubAgentTester) {
			o.advanceToPhase(PhaseReview)
			o.startReviewPhase()
		}
	case PhaseReview:
		if o.isPhaseComplete(SubAgentReviewer) {
			o.advanceToPhase(PhaseOptimization)
			o.startOptimizationPhase()
		}
	case PhaseOptimization:
		if o.isPhaseComplete(SubAgentOptimizer) {
			o.completeBuild()
		}
	}
}

// isPhaseComplete checks if a specific sub-agent type has completed
func (o *Orchestrator) isPhaseComplete(agentType SubAgentType) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, agent := range o.subAgents {
		if agent.Type == agentType {
			return agent.Status == StatusCompleted
		}
	}
	return false
}

// isCodingComplete checks if all coding sub-agents are done
func (o *Orchestrator) isCodingComplete() bool {
	codingTypes := []SubAgentType{SubAgentFrontendDev, SubAgentBackendDev, SubAgentAPIDev}
	
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, agentType := range codingTypes {
		for _, agent := range o.subAgents {
			if agent.Type == agentType && agent.Status != StatusCompleted {
				return false
			}
		}
	}
	return true
}

// advanceToPhase moves to a new build phase
func (o *Orchestrator) advanceToPhase(phase BuildPhase) {
	o.mu.Lock()
	o.phase = phase
	o.mu.Unlock()

	o.broadcast(&WSMessage{
		Type:      WSBuildProgress,
		BuildID:   o.buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":   string(phase),
			"message": fmt.Sprintf("Advancing to %s phase", phase),
		},
	})
}

// advanceToCodingPhase spawns GPT-5 and Gemini sub-agents for parallel coding
func (o *Orchestrator) advanceToCodingPhase() {
	o.advanceToPhase(PhaseCoding)

	// Spawn GPT-5 coding sub-agents
	if err := o.spawnGPT5SubAgents(); err != nil {
		log.Printf("Error spawning GPT-5 sub-agents: %v", err)
	}

	// Spawn Gemini sub-agents for parallel assistance
	if err := o.spawnGeminiSubAgents(); err != nil {
		log.Printf("Error spawning Gemini sub-agents: %v", err)
	}

	// Queue coding tasks in parallel
	o.queueCodingTasks()
}

// queueCodingTasks creates parallel coding tasks for GPT-5 sub-agents
func (o *Orchestrator) queueCodingTasks() {
	// Get the architecture from Claude's architect
	// In a full implementation, this would parse the architecture output

	codingTasks := []struct {
		Type        SubAgentType
		Description string
	}{
		{SubAgentFrontendDev, "Build the frontend React components with TypeScript"},
		{SubAgentBackendDev, "Create the backend API with Express/Node.js"},
		{SubAgentAPIDev, "Implement REST API endpoints with proper validation"},
		{SubAgentDatabaseDev, "Design and implement database schema and models"},
		{SubAgentUIDev, "Create beautiful UI styles with Tailwind CSS"},
	}

	for _, task := range codingTasks {
		agentID := o.getSubAgentByType(task.Type)
		if agentID != "" {
			o.queueTask(&OrchestratorTask{
				ID:          uuid.New().String(),
				SubAgentID:  agentID,
				Type:        task.Type,
				Description: task.Description,
				Priority:    80,
				Phase:       PhaseCoding,
				CreatedAt:   time.Now(),
			})
		}
	}
}

// startTestingPhase initiates testing with Gemini
func (o *Orchestrator) startTestingPhase() {
	testerID := o.getSubAgentByType(SubAgentTester)
	if testerID != "" {
		o.queueTask(&OrchestratorTask{
			ID:          uuid.New().String(),
			SubAgentID:  testerID,
			Type:        SubAgentTester,
			Description: "Write comprehensive tests for all generated code",
			Priority:    70,
			Phase:       PhaseTesting,
			CreatedAt:   time.Now(),
		})
	}
}

// startReviewPhase initiates code review with Claude
func (o *Orchestrator) startReviewPhase() {
	reviewerID := o.getSubAgentByType(SubAgentReviewer)
	if reviewerID != "" {
		o.queueTask(&OrchestratorTask{
			ID:          uuid.New().String(),
			SubAgentID:  reviewerID,
			Type:        SubAgentReviewer,
			Description: "Review all generated code for quality and security",
			Priority:    60,
			Phase:       PhaseReview,
			CreatedAt:   time.Now(),
		})
	}
}

// startOptimizationPhase initiates optimization with Gemini
func (o *Orchestrator) startOptimizationPhase() {
	optimizerID := o.getSubAgentByType(SubAgentOptimizer)
	if optimizerID != "" {
		o.queueTask(&OrchestratorTask{
			ID:          uuid.New().String(),
			SubAgentID:  optimizerID,
			Type:        SubAgentOptimizer,
			Description: "Optimize code for performance and efficiency",
			Priority:    50,
			Phase:       PhaseOptimization,
			CreatedAt:   time.Now(),
		})
	}
}

// completeBuild finalizes the build and prepares preview
func (o *Orchestrator) completeBuild() {
	o.mu.Lock()
	o.phase = PhaseComplete
	o.mu.Unlock()

	// Generate preview URL
	previewURL := fmt.Sprintf("/preview/%s", o.buildID)
	o.mu.Lock()
	o.previewURL = previewURL
	o.mu.Unlock()

	o.broadcast(&WSMessage{
		Type:      WSBuildCompleted,
		BuildID:   o.buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":      "completed",
			"preview_url": previewURL,
			"files_count": len(o.generatedCode),
			"message":     "Build complete! Your app is ready for preview.",
		},
	})

	o.broadcast(&WSMessage{
		Type:      WSPreviewReady,
		BuildID:   o.buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"preview_url": previewURL,
			"message":     "App preview is now available!",
		},
	})

	log.Printf("Build %s completed with %d files", o.buildID, len(o.generatedCode))
}

// GetGeneratedCode returns all generated code files
func (o *Orchestrator) GetGeneratedCode() map[string]string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range o.generatedCode {
		result[k] = v
	}
	return result
}

// GetPreviewURL returns the preview URL for the built app
func (o *Orchestrator) GetPreviewURL() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.previewURL
}

// Subscribe adds a channel to receive updates
func (o *Orchestrator) Subscribe(ch chan *WSMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.subscribers = append(o.subscribers, ch)
}

// broadcast sends a message to all subscribers
func (o *Orchestrator) broadcast(msg *WSMessage) {
	o.mu.RLock()
	subs := o.subscribers
	o.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

// parseGeneratedFiles extracts file information from AI response
func (o *Orchestrator) parseGeneratedFiles(response string) []GeneratedFile {
	// This would parse JSON response to extract files
	// For now, return empty - full implementation would parse the structured output
	return []GeneratedFile{}
}

// Shutdown gracefully stops the orchestrator
func (o *Orchestrator) Shutdown() {
	o.cancel()
	close(o.taskQueue)
	close(o.resultQueue)
}
