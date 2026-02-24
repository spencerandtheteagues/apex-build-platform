// Package agents provides the multi-agent orchestration system for APEX.BUILD
// This is the CORE FOUNDATION that enables multiple AI agents to collaborate
// on building complete applications from natural language descriptions.
package agents

import (
	"sync"
	"time"

	"apex-build/internal/ai"
)

// AgentRole defines the specialized role of an AI agent
type AgentRole string

const (
	RolePlanner   AgentRole = "planner"   // Analyzes requirements, creates build plans
	RoleArchitect AgentRole = "architect" // Designs system architecture
	RoleFrontend  AgentRole = "frontend"  // Builds UI components
	RoleBackend   AgentRole = "backend"   // Creates API and business logic
	RoleDatabase  AgentRole = "database"  // Designs schemas and queries
	RoleTesting   AgentRole = "testing"   // Writes and runs tests
	RoleDevOps    AgentRole = "devops"    // Handles deployment configuration
	RoleReviewer  AgentRole = "reviewer"  // Code review and quality assurance
	RoleSolver    AgentRole = "solver"    // Investigates and fixes failed tasks
	RoleLead      AgentRole = "lead"      // Coordinates all agents (main contact point)
)

// AgentStatus represents the current state of an agent
type AgentStatus string

const (
	StatusIdle       AgentStatus = "idle"       // Agent is available for work
	StatusWorking    AgentStatus = "working"    // Agent is actively working on a task
	StatusWaiting    AgentStatus = "waiting"    // Agent is waiting for dependencies
	StatusCompleted  AgentStatus = "completed"  // Agent finished its task successfully
	StatusError      AgentStatus = "error"      // Agent encountered an error
	StatusTerminated AgentStatus = "terminated" // Agent was stopped
)

// Note: AIProvider type is now imported from the ai package

// Agent represents a single AI agent working on part of a build
type Agent struct {
	ID          string        `json:"id"`
	Role        AgentRole     `json:"role"`
	Provider    ai.AIProvider `json:"provider"`
	Model       string        `json:"model,omitempty"`
	Status      AgentStatus   `json:"status"`
	BuildID     string        `json:"build_id"`
	CurrentTask *Task         `json:"current_task,omitempty"`
	Progress    int           `json:"progress"` // 0-100
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Output      []string      `json:"output,omitempty"` // Agent's messages/output
	Error       string        `json:"error,omitempty"`

	mu sync.RWMutex
}

// Task represents a unit of work assigned to an agent
type Task struct {
	ID           string         `json:"id"`
	Type         TaskType       `json:"type"`
	Description  string         `json:"description"`
	Priority     int            `json:"priority"`               // Higher = more important
	Dependencies []string       `json:"dependencies,omitempty"` // Task IDs that must complete first
	AssignedTo   string         `json:"assigned_to,omitempty"`  // Agent ID
	Status       TaskStatus     `json:"status"`
	Input        map[string]any `json:"input,omitempty"`
	Output       *TaskOutput    `json:"output,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Error        string         `json:"error,omitempty"`

	// Retry mechanism - persist until success
	RetryCount    int            `json:"retry_count"`             // Number of attempts made
	MaxRetries    int            `json:"max_retries"`             // Maximum retry attempts (default: 5)
	ErrorHistory  []ErrorAttempt `json:"error_history,omitempty"` // History of errors for learning
	RetryStrategy RetryStrategy  `json:"retry_strategy"`          // How to retry on failure
}

// ErrorAttempt tracks a failed attempt for learning
type ErrorAttempt struct {
	AttemptNumber int       `json:"attempt_number"`
	Error         string    `json:"error"`
	Timestamp     time.Time `json:"timestamp"`
	Context       string    `json:"context,omitempty"`  // What was tried
	Analysis      string    `json:"analysis,omitempty"` // AI analysis of what went wrong
}

// RetryStrategy defines how to handle failures
type RetryStrategy string

const (
	RetryImmediate RetryStrategy = "immediate" // Retry immediately with same approach
	RetryWithFix   RetryStrategy = "with_fix"  // Analyze error and adjust approach
	RetryDifferent RetryStrategy = "different" // Try a completely different approach
	RetryEscalate  RetryStrategy = "escalate"  // Ask for human help
)

// TaskType categorizes the kind of work a task involves
type TaskType string

const (
	TaskPlan           TaskType = "plan"            // Create build plan
	TaskArchitecture   TaskType = "architecture"    // Design architecture
	TaskGenerateFile   TaskType = "generate_file"   // Generate a single file
	TaskGenerateAPI    TaskType = "generate_api"    // Generate API endpoint
	TaskGenerateUI     TaskType = "generate_ui"     // Generate UI component
	TaskGenerateSchema TaskType = "generate_schema" // Generate database schema
	TaskTest           TaskType = "test"            // Write/run tests
	TaskReview         TaskType = "review"          // Review generated code
	TaskFix            TaskType = "fix"             // Fix issues found
	TaskDeploy         TaskType = "deploy"          // Configure deployment
)

// TaskStatus represents the state of a task
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"     // Not started
	TaskInProgress TaskStatus = "in_progress" // Currently being worked on
	TaskCompleted  TaskStatus = "completed"   // Successfully finished
	TaskFailed     TaskStatus = "failed"      // Failed to complete
	TaskCancelled  TaskStatus = "cancelled"   // Manually cancelled
)

// TaskOutput contains the results of a completed task
type TaskOutput struct {
	Files       []GeneratedFile `json:"files,omitempty"`
	Messages    []string        `json:"messages,omitempty"`
	Suggestions []string        `json:"suggestions,omitempty"`
	Metrics     map[string]any  `json:"metrics,omitempty"`
}

// GeneratedFile represents a file created by an agent
type GeneratedFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	IsNew    bool   `json:"is_new"` // True if created, false if modified
}

// Build represents an entire app-building session
type Build struct {
	ID                  string            `json:"id"`
	UserID              uint              `json:"user_id"`
	ProjectID           *uint             `json:"project_id,omitempty"`
	Status              BuildStatus       `json:"status"`
	Mode                BuildMode         `json:"mode"`
	PowerMode           PowerMode         `json:"power_mode"`
	ProviderMode        string            `json:"provider_mode,omitempty"` // platform or byok
	RequirePreviewReady bool              `json:"require_preview_ready,omitempty"`
	Description         string            `json:"description"` // User's app description
	TechStack           *TechStack        `json:"tech_stack,omitempty"`
	Plan                *BuildPlan        `json:"plan,omitempty"`
	Agents              map[string]*Agent `json:"agents"`
	Tasks               []*Task           `json:"tasks"`
	Checkpoints         []*Checkpoint     `json:"checkpoints"`
	Progress            int               `json:"progress"` // 0-100
	// Guardrails
	MaxAgents                 int        `json:"max_agents,omitempty"`
	MaxRetries                int        `json:"max_retries,omitempty"`
	MaxRequests               int        `json:"max_requests,omitempty"`
	MaxTokensPerRequest       int        `json:"max_tokens_per_request,omitempty"`
	RequestsUsed              int        `json:"requests_used,omitempty"`
	ReadinessRecoveryAttempts int        `json:"readiness_recovery_attempts,omitempty"`
	PhasedPipelineComplete    bool       `json:"phased_pipeline_complete,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
	CompletedAt               *time.Time `json:"completed_at,omitempty"`
	Error                     string     `json:"error,omitempty"`

	mu sync.RWMutex
}

// BuildStatus represents the overall state of a build
type BuildStatus string

const (
	BuildPending    BuildStatus = "pending"     // Waiting to start
	BuildPlanning   BuildStatus = "planning"    // Creating build plan
	BuildInProgress BuildStatus = "in_progress" // Actively building
	BuildTesting    BuildStatus = "testing"     // Running tests
	BuildReviewing  BuildStatus = "reviewing"   // Code review phase
	BuildCompleted  BuildStatus = "completed"   // Successfully finished
	BuildFailed     BuildStatus = "failed"      // Build failed
	BuildCancelled  BuildStatus = "cancelled"   // Manually cancelled
)

// BuildMode determines how the build is executed
type BuildMode string

const (
	ModeFast BuildMode = "fast" // Quick build, ~3-5 minutes
	ModeFull BuildMode = "full" // Comprehensive build, 10+ minutes
)

// PowerMode controls which AI models are used during the build
type PowerMode string

const (
	PowerMax      PowerMode = "max"      // Latest, most powerful models (Claude Opus, GPT-5.2-Codex, Gemini 3 Pro)
	PowerBalanced PowerMode = "balanced" // Best balance of quality and speed (Sonnet 4.5, GPT-5, Gemini 3 Flash)
	PowerFast     PowerMode = "fast"     // Cheapest, fastest models (Haiku 4.5, GPT-4o-mini, Gemini 2.5 Flash Lite)
)

// CreditMultiplier returns the credit usage multiplier for a power mode
func (pm PowerMode) CreditMultiplier() float64 {
	switch pm {
	case PowerMax:
		return 2.0 // 2.0x credits — premium models
	case PowerBalanced:
		return 1.8 // 1.8x credits — mid-tier models
	default:
		return 1.6 // 1.6x credits — budget models
	}
}

// BuildPlan contains the structured plan for building an app
type BuildPlan struct {
	ID            string        `json:"id"`
	BuildID       string        `json:"build_id"`
	AppType       string        `json:"app_type"` // web, api, fullstack, etc.
	TechStack     TechStack     `json:"tech_stack"`
	Features      []Feature     `json:"features"`
	DataModels    []DataModel   `json:"data_models"`
	APIEndpoints  []APIEndpoint `json:"api_endpoints"`
	Components    []UIComponent `json:"components"`
	Files         []PlannedFile `json:"files"`
	EstimatedTime time.Duration `json:"estimated_time"`
	CreatedAt     time.Time     `json:"created_at"`
}

// TechStack defines the technologies to use
type TechStack struct {
	Frontend string   `json:"frontend"` // React, Vue, Next.js, etc.
	Backend  string   `json:"backend"`  // Node.js, Go, Python, etc.
	Database string   `json:"database"` // PostgreSQL, MongoDB, etc.
	Styling  string   `json:"styling"`  // Tailwind, CSS Modules, etc.
	Extras   []string `json:"extras"`   // Additional libraries
}

// Feature represents a feature to implement
type Feature struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Priority     int      `json:"priority"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// DataModel represents a database entity
type DataModel struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Fields      []ModelField `json:"fields"`
	Relations   []Relation   `json:"relations,omitempty"`
}

// ModelField represents a field in a data model
type ModelField struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Required   bool   `json:"required"`
	Unique     bool   `json:"unique"`
	Default    any    `json:"default,omitempty"`
	Validation string `json:"validation,omitempty"`
}

// Relation represents a relationship between models
type Relation struct {
	Type   string `json:"type"` // hasOne, hasMany, belongsTo, manyToMany
	Target string `json:"target"`
	Field  string `json:"field"`
}

// APIEndpoint represents an API route to generate
type APIEndpoint struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Auth        bool              `json:"auth"`
	Input       map[string]string `json:"input,omitempty"`
	Output      string            `json:"output,omitempty"`
}

// UIComponent represents a frontend component to generate
type UIComponent struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // page, component, layout
	Props       []string `json:"props,omitempty"`
	State       []string `json:"state,omitempty"`
}

// PlannedFile represents a file to be generated
type PlannedFile struct {
	Path         string   `json:"path"`
	Type         string   `json:"type"` // frontend, backend, config, etc.
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// Checkpoint represents a saved state during the build
type Checkpoint struct {
	ID          string          `json:"id"`
	BuildID     string          `json:"build_id"`
	Number      int             `json:"number"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Files       []GeneratedFile `json:"files"`
	Progress    int             `json:"progress"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Message types for WebSocket communication
type WSMessageType string

const (
	WSAgentSpawned    WSMessageType = "agent:spawned"
	WSAgentWorking    WSMessageType = "agent:working"
	WSAgentProgress   WSMessageType = "agent:progress"
	WSAgentCompleted  WSMessageType = "agent:completed"
	WSAgentError      WSMessageType = "agent:error"
	WSAgentMessage    WSMessageType = "agent:message"
	WSBuildStarted    WSMessageType = "build:started"
	WSBuildProgress   WSMessageType = "build:progress"
	WSBuildCheckpoint WSMessageType = "build:checkpoint"
	WSBuildCompleted  WSMessageType = "build:completed"
	WSBuildError      WSMessageType = "build:error"
	WSFileCreated     WSMessageType = "file:created"
	WSFileUpdated     WSMessageType = "file:updated"
	WSCodeGenerated   WSMessageType = "code:generated"
	WSTerminalOutput  WSMessageType = "terminal:output"
	WSPreviewReady    WSMessageType = "preview:ready"
	WSUserMessage     WSMessageType = "user:message"
	WSLeadResponse    WSMessageType = "lead:response"
)

// WSMessage is the structure for WebSocket messages
type WSMessage struct {
	Type      WSMessageType `json:"type"`
	BuildID   string        `json:"build_id"`
	AgentID   string        `json:"agent_id,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Data      any           `json:"data"`
}

// BuildRequest is the input for starting a new build
type BuildRequest struct {
	Description         string     `json:"description"`
	Prompt              string     `json:"prompt,omitempty"` // Detailed build prompt (falls back to Description)
	Mode                BuildMode  `json:"mode"`
	PowerMode           PowerMode  `json:"power_mode,omitempty"`    // max, balanced, fast — controls model quality
	ProviderMode        string     `json:"provider_mode,omitempty"` // platform or byok
	RequirePreviewReady bool       `json:"require_preview_ready,omitempty"`
	ProjectName         string     `json:"project_name,omitempty"`
	TechStack           *TechStack `json:"tech_stack,omitempty"` // Optional override
}

// BuildResponse is returned when a build is created
type BuildResponse struct {
	BuildID      string `json:"build_id"`
	WebSocketURL string `json:"websocket_url"`
	Status       string `json:"status"`
}

// ChatMessage represents a message in the build chat
type ChatMessage struct {
	ID        string    `json:"id"`
	BuildID   string    `json:"build_id"`
	Role      string    `json:"role"` // user, lead, agent
	AgentID   string    `json:"agent_id,omitempty"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}
