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

// UserRoleCategory maps user-facing role categories to backend agent roles.
// These simplify the 10 internal roles into 4 intuitive categories for the UI.
type UserRoleCategory string

const (
	CategoryArchitect UserRoleCategory = "architect" // planner + architect + reviewer
	CategoryCoder     UserRoleCategory = "coder"     // frontend + backend + database
	CategoryTester    UserRoleCategory = "tester"    // testing
	CategoryDevOps    UserRoleCategory = "devops"    // devops + solver
)

// ExpandUserCategory returns the backend agent roles for a user-facing category.
func ExpandUserCategory(cat UserRoleCategory) []AgentRole {
	switch cat {
	case CategoryArchitect:
		return []AgentRole{RolePlanner, RoleArchitect, RoleReviewer}
	case CategoryCoder:
		return []AgentRole{RoleFrontend, RoleBackend, RoleDatabase}
	case CategoryTester:
		return []AgentRole{RoleTesting}
	case CategoryDevOps:
		return []AgentRole{RoleDevOps, RoleSolver}
	default:
		return nil
	}
}

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

	mu sync.RWMutex
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
	Files                      []GeneratedFile       `json:"files,omitempty"`
	DeletedFiles               []string              `json:"deleted_files,omitempty"`
	Messages                   []string              `json:"messages,omitempty"`
	Suggestions                []string              `json:"suggestions,omitempty"`
	Metrics                    map[string]any        `json:"metrics,omitempty"`
	TruncatedFiles             []string              `json:"truncated_files,omitempty"` // file paths whose content was cut off mid-generation
	StructuredPatchBundle      *PatchBundle          `json:"structured_patch_bundle,omitempty"`
	ProviderVerificationReport *VerificationReport   `json:"provider_verification_report,omitempty"`
	Plan                       *BuildPlan            `json:"plan,omitempty"`
	StartAck                   *TaskStartAck         `json:"start_ack,omitempty"`
	Completion                 *TaskCompletionReport `json:"completion,omitempty"`
}

// GeneratedFile represents a file created by an agent
type GeneratedFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	IsNew    bool   `json:"is_new"` // True if created, false if modified
}

type BuildConversationRole string

const (
	ConversationRoleUser   BuildConversationRole = "user"
	ConversationRoleLead   BuildConversationRole = "lead"
	ConversationRoleSystem BuildConversationRole = "system"
)

type BuildConversationKind string

const (
	ConversationKindMessage           BuildConversationKind = "message"
	ConversationKindQuestion          BuildConversationKind = "question"
	ConversationKindDirective         BuildConversationKind = "directive"
	ConversationKindPermissionRequest BuildConversationKind = "permission_request"
	ConversationKindPermissionUpdate  BuildConversationKind = "permission_update"
)

type BuildMessageTargetMode string

const (
	BuildMessageTargetLead      BuildMessageTargetMode = "lead"
	BuildMessageTargetAgent     BuildMessageTargetMode = "agent"
	BuildMessageTargetRole      BuildMessageTargetMode = "role"
	BuildMessageTargetAllAgents BuildMessageTargetMode = "all_agents"
)

type BuildPermissionScope string

const (
	PermissionScopeProgram    BuildPermissionScope = "program"
	PermissionScopeFilesystem BuildPermissionScope = "filesystem"
	PermissionScopeNetwork    BuildPermissionScope = "network"
	PermissionScopeService    BuildPermissionScope = "service"
)

type BuildPermissionDecision string

const (
	PermissionDecisionAsk   BuildPermissionDecision = "ask"
	PermissionDecisionAllow BuildPermissionDecision = "allow"
	PermissionDecisionDeny  BuildPermissionDecision = "deny"
)

type BuildPermissionMode string

const (
	PermissionModeOnce  BuildPermissionMode = "once"
	PermissionModeBuild BuildPermissionMode = "build"
)

type BuildPermissionRequestStatus string

const (
	PermissionRequestPending BuildPermissionRequestStatus = "pending"
	PermissionRequestAllowed BuildPermissionRequestStatus = "allowed"
	PermissionRequestDenied  BuildPermissionRequestStatus = "denied"
)

type BuildConversationMessage struct {
	ID               string                 `json:"id"`
	Role             BuildConversationRole  `json:"role"`
	Kind             BuildConversationKind  `json:"kind"`
	Content          string                 `json:"content"`
	AgentID          string                 `json:"agent_id,omitempty"`
	AgentRole        string                 `json:"agent_role,omitempty"`
	TargetMode       BuildMessageTargetMode `json:"target_mode,omitempty"`
	TargetAgentID    string                 `json:"target_agent_id,omitempty"`
	TargetAgentRole  string                 `json:"target_agent_role,omitempty"`
	ClientToken      string                 `json:"client_token,omitempty"`
	RequiresResponse bool                   `json:"requires_response,omitempty"`
	Blocking         bool                   `json:"blocking,omitempty"`
	Timestamp        time.Time              `json:"timestamp"`
	Status           string                 `json:"status,omitempty"`
}

type BuildPermissionRule struct {
	ID        string                  `json:"id"`
	Scope     BuildPermissionScope    `json:"scope"`
	Target    string                  `json:"target"`
	Decision  BuildPermissionDecision `json:"decision"`
	Mode      BuildPermissionMode     `json:"mode"`
	Reason    string                  `json:"reason,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type BuildPermissionRequest struct {
	ID              string                       `json:"id"`
	Scope           BuildPermissionScope         `json:"scope"`
	Target          string                       `json:"target"`
	Reason          string                       `json:"reason"`
	CommandPreview  string                       `json:"command_preview,omitempty"`
	RequestedByID   string                       `json:"requested_by_id,omitempty"`
	RequestedByRole string                       `json:"requested_by_role,omitempty"`
	Blocking        bool                         `json:"blocking,omitempty"`
	Status          BuildPermissionRequestStatus `json:"status"`
	Mode            BuildPermissionMode          `json:"mode,omitempty"`
	ResolutionNote  string                       `json:"resolution_note,omitempty"`
	RequestedAt     time.Time                    `json:"requested_at"`
	ResolvedAt      *time.Time                   `json:"resolved_at,omitempty"`
}

type BuildApprovalEventStatus string

const (
	ApprovalEventPending   BuildApprovalEventStatus = "pending"
	ApprovalEventSatisfied BuildApprovalEventStatus = "satisfied"
	ApprovalEventDenied    BuildApprovalEventStatus = "denied"
)

type BuildApprovalEvent struct {
	ID         string                   `json:"id"`
	Kind       string                   `json:"kind"`
	Title      string                   `json:"title"`
	Status     BuildApprovalEventStatus `json:"status"`
	Summary    string                   `json:"summary,omitempty"`
	SourceType string                   `json:"source_type,omitempty"`
	SourceID   string                   `json:"source_id,omitempty"`
	Actor      string                   `json:"actor,omitempty"`
	Timestamp  time.Time                `json:"timestamp"`
}

type BuildInteractionState struct {
	Messages           []BuildConversationMessage `json:"messages,omitempty"`
	SteeringNotes      []string                   `json:"steering_notes,omitempty"`
	PendingRevisions   []string                   `json:"pending_revisions,omitempty"`
	PendingQuestion    string                     `json:"pending_question,omitempty"`
	WaitingForUser     bool                       `json:"waiting_for_user,omitempty"`
	Paused             bool                       `json:"paused,omitempty"`
	PauseReason        string                     `json:"pause_reason,omitempty"`
	PermissionRules    []BuildPermissionRule      `json:"permission_rules,omitempty"`
	PermissionRequests []BuildPermissionRequest   `json:"permission_requests,omitempty"`
	ApprovalEvents     []BuildApprovalEvent       `json:"approval_events,omitempty"`
	AttentionRequired  bool                       `json:"attention_required,omitempty"`
}

type BuildActivityEntry struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	AgentRole  string    `json:"agent_role"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model,omitempty"`
	Type       string    `json:"type"`
	EventType  string    `json:"event_type,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	TaskType   string    `json:"task_type,omitempty"`
	Files      []string  `json:"files,omitempty"`
	FilesCount int       `json:"files_count,omitempty"`
	RetryCount int       `json:"retry_count,omitempty"`
	MaxRetries int       `json:"max_retries,omitempty"`
	IsInternal bool      `json:"is_internal,omitempty"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
}

type BuildClassificationState string

const (
	BuildClassificationStaticReady        BuildClassificationState = "static_ready"
	BuildClassificationUpgradeRequired    BuildClassificationState = "upgrade_required"
	BuildClassificationFullStackCandidate BuildClassificationState = "full_stack_candidate"
)

type BuildCapabilityState struct {
	RequiredCapabilities   []string `json:"required_capabilities,omitempty"`
	RequiresBackendRuntime bool     `json:"requires_backend_runtime,omitempty"`
	RequiresDatabase       bool     `json:"requires_database,omitempty"`
	RequiresStorage        bool     `json:"requires_storage,omitempty"`
	RequiresAuth           bool     `json:"requires_auth,omitempty"`
	RequiresExternalAPI    bool     `json:"requires_external_api,omitempty"`
	RequiresBilling        bool     `json:"requires_billing,omitempty"`
	RequiresRealtime       bool     `json:"requires_realtime,omitempty"`
	RequiresJobs           bool     `json:"requires_jobs,omitempty"`
	RequiresPublish        bool     `json:"requires_publish,omitempty"`
	RequiresBYOK           bool     `json:"requires_byok,omitempty"`
}

type BuildPolicyState struct {
	PlanType           string                   `json:"plan_type,omitempty"`
	Classification     BuildClassificationState `json:"classification,omitempty"`
	UpgradeRequired    bool                     `json:"upgrade_required,omitempty"`
	UpgradeReason      string                   `json:"upgrade_reason,omitempty"`
	RequiredPlan       string                   `json:"required_plan,omitempty"`
	StaticFrontendOnly bool                     `json:"static_frontend_only,omitempty"`
	FullStackEligible  bool                     `json:"full_stack_eligible,omitempty"`
	PublishEnabled     bool                     `json:"publish_enabled,omitempty"`
	BYOKEnabled        bool                     `json:"byok_enabled,omitempty"`
	// MaxPowerMode is the highest power mode allowed for this plan tier.
	// Free → PowerFast only; Builder → PowerBalanced; Pro/Team → PowerMax.
	MaxPowerMode PowerMode `json:"max_power_mode,omitempty"`
}

type BuildBlockerCategory string

const (
	BlockerCategoryApprovals        BuildBlockerCategory = "approvals"
	BlockerCategorySecrets          BuildBlockerCategory = "secrets"
	BlockerCategoryAuth             BuildBlockerCategory = "auth"
	BlockerCategoryBilling          BuildBlockerCategory = "billing"
	BlockerCategoryEnvironment      BuildBlockerCategory = "environment"
	BlockerCategoryExternalAccess   BuildBlockerCategory = "external_access"
	BlockerCategoryDeployment       BuildBlockerCategory = "deployment"
	BlockerCategoryPolicyLimitation BuildBlockerCategory = "policy_platform_limitations"
	BlockerCategoryRuntimeFailure   BuildBlockerCategory = "runtime_failure"
	BlockerCategoryPlanTier         BuildBlockerCategory = "plan_tier"
)

type BuildBlockerSeverity string

const (
	BlockerSeverityInfo     BuildBlockerSeverity = "info"
	BlockerSeverityWarning  BuildBlockerSeverity = "warning"
	BlockerSeverityBlocking BuildBlockerSeverity = "blocking"
)

type BuildBlocker struct {
	ID                     string               `json:"id"`
	Title                  string               `json:"title"`
	Type                   string               `json:"type"`
	Category               BuildBlockerCategory `json:"category"`
	Severity               BuildBlockerSeverity `json:"severity"`
	WhoMustAct             string               `json:"who_must_act,omitempty"`
	Summary                string               `json:"summary,omitempty"`
	UnblocksWith           string               `json:"unblocks_with,omitempty"`
	PartialProgressAllowed bool                 `json:"partial_progress_allowed,omitempty"`
	PlanTierRelated        bool                 `json:"plan_tier_related,omitempty"`
}

type BuildApprovalStatus string

const (
	ApprovalStatusNotRequired BuildApprovalStatus = "not_required"
	ApprovalStatusPending     BuildApprovalStatus = "pending"
	ApprovalStatusSatisfied   BuildApprovalStatus = "satisfied"
	ApprovalStatusDenied      BuildApprovalStatus = "denied"
)

type BuildApproval struct {
	ID                      string              `json:"id"`
	Kind                    string              `json:"kind"`
	Title                   string              `json:"title"`
	Status                  BuildApprovalStatus `json:"status"`
	Required                bool                `json:"required"`
	Summary                 string              `json:"summary,omitempty"`
	Reason                  string              `json:"reason,omitempty"`
	SourceType              string              `json:"source_type,omitempty"`
	SourceID                string              `json:"source_id,omitempty"`
	Actor                   string              `json:"actor,omitempty"`
	PartialProgressAllowed  bool                `json:"partial_progress_allowed,omitempty"`
	AcknowledgementRequired bool                `json:"acknowledgement_required,omitempty"`
	PlanTierRelated         bool                `json:"plan_tier_related,omitempty"`
	MismatchDetected        bool                `json:"mismatch_detected,omitempty"`
	MismatchReason          string              `json:"mismatch_reason,omitempty"`
	RequestedAt             time.Time           `json:"requested_at"`
	ResolvedAt              *time.Time          `json:"resolved_at,omitempty"`
}

type BuildSnapshotState struct {
	CurrentPhase        string                   `json:"current_phase,omitempty"`
	QualityGateRequired *bool                    `json:"quality_gate_required,omitempty"`
	QualityGateStatus   string                   `json:"quality_gate_status,omitempty"`
	QualityGateStage    string                   `json:"quality_gate_stage,omitempty"`
	FailureTaxonomy     *BuildFailureTaxonomy    `json:"failure_taxonomy,omitempty"`
	AvailableProviders  []string                 `json:"available_providers,omitempty"`
	CapabilityState     *BuildCapabilityState    `json:"capability_state,omitempty"`
	PolicyState         *BuildPolicyState        `json:"policy_state,omitempty"`
	Blockers            []BuildBlocker           `json:"blockers,omitempty"`
	Approvals           []BuildApproval          `json:"approvals,omitempty"`
	RestoreContext      *BuildRestoreContext     `json:"restore_context,omitempty"`
	Orchestration       *BuildOrchestrationState `json:"orchestration,omitempty"`
}

type BuildRestoreContext struct {
	SubscriptionPlan            string            `json:"subscription_plan,omitempty"`
	ProviderMode                string            `json:"provider_mode,omitempty"`
	ActiveOwnerInstanceID       string            `json:"active_owner_instance_id,omitempty"`
	ActiveOwnerHeartbeatAt      *time.Time        `json:"active_owner_heartbeat_at,omitempty"`
	RequirePreviewReady         bool              `json:"require_preview_ready,omitempty"`
	RequestsUsed                int               `json:"requests_used,omitempty"`
	ReadinessRecoveryAttempts   int               `json:"readiness_recovery_attempts,omitempty"`
	PreviewVerificationAttempts int               `json:"preview_verification_attempts,omitempty"`
	CompileValidationPassed     bool              `json:"compile_validation_passed,omitempty"`
	CompileValidationAttempts   int               `json:"compile_validation_attempts,omitempty"`
	CompileValidationRepairs    int               `json:"compile_validation_repairs,omitempty"`
	CompileValidationStartedAt  *time.Time        `json:"compile_validation_started_at,omitempty"`
	MaxAgents                   int               `json:"max_agents,omitempty"`
	MaxRetries                  int               `json:"max_retries,omitempty"`
	MaxRequests                 int               `json:"max_requests,omitempty"`
	MaxTokensPerRequest         int               `json:"max_tokens_per_request,omitempty"`
	PhasedPipelineComplete      bool              `json:"phased_pipeline_complete,omitempty"`
	DiffMode                    bool              `json:"diff_mode,omitempty"`
	RoleAssignments             map[string]string `json:"role_assignments,omitempty"`
	ProviderModelOverrides      map[string]string `json:"provider_model_overrides,omitempty"`
	TechStack                   *TechStack        `json:"tech_stack,omitempty"`
	Plan                        *BuildPlan        `json:"plan,omitempty"`
}

// Build represents an entire app-building session
type Build struct {
	ID                  string            `json:"id"`
	UserID              uint              `json:"user_id"`
	ProjectID           *uint             `json:"project_id,omitempty"`
	Status              BuildStatus       `json:"status"`
	Mode                BuildMode         `json:"mode"`
	PowerMode           PowerMode         `json:"power_mode"`
	SubscriptionPlan    string            `json:"subscription_plan,omitempty"`
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
	MaxAgents                   int                   `json:"max_agents,omitempty"`
	MaxRetries                  int                   `json:"max_retries,omitempty"`
	MaxRequests                 int                   `json:"max_requests,omitempty"`
	MaxTokensPerRequest         int                   `json:"max_tokens_per_request,omitempty"`
	RequestsUsed                int                   `json:"requests_used,omitempty"`
	ReadinessRecoveryAttempts   int                   `json:"readiness_recovery_attempts,omitempty"`
	PreviewVerificationAttempts int                   `json:"preview_verification_attempts,omitempty"`
	CompileValidationPassed     bool                  `json:"compile_validation_passed,omitempty"`
	CompileValidationAttempts   int                   `json:"compile_validation_attempts,omitempty"`
	CompileValidationRepairs    int                   `json:"compile_validation_repairs,omitempty"`
	CompileValidationStartedAt  *time.Time            `json:"-"`
	PhasedPipelineComplete      bool                  `json:"phased_pipeline_complete,omitempty"`
	DiffMode                    bool                  `json:"diff_mode,omitempty"`        // When true, changes require user review before applying
	RoleAssignments             map[string]string     `json:"role_assignments,omitempty"` // User-specified provider per role category
	ProviderModelOverrides      map[string]string     `json:"provider_model_overrides,omitempty"`
	Interaction                 BuildInteractionState `json:"interaction,omitempty"`
	ActivityTimeline            []BuildActivityEntry  `json:"activity_timeline,omitempty"`
	SnapshotState               BuildSnapshotState    `json:"snapshot_state,omitempty"`
	SnapshotFiles               []GeneratedFile       `json:"-"`
	CreatedAt                   time.Time             `json:"created_at"`
	UpdatedAt                   time.Time             `json:"updated_at"`
	CompletedAt                 *time.Time            `json:"completed_at,omitempty"`
	Error                       string                `json:"error,omitempty"`
	FinalizationInProgress      bool                  `json:"-"`

	mu sync.RWMutex
}

// BuildStatus represents the overall state of a build
type BuildStatus string

const (
	BuildPending        BuildStatus = "pending"         // Waiting to start
	BuildPlanning       BuildStatus = "planning"        // Creating build plan
	BuildInProgress     BuildStatus = "in_progress"     // Actively building
	BuildTesting        BuildStatus = "testing"         // Running tests
	BuildReviewing      BuildStatus = "reviewing"       // Code review phase
	BuildCompleted      BuildStatus = "completed"       // Successfully finished
	BuildFailed         BuildStatus = "failed"          // Build failed
	BuildCancelled      BuildStatus = "cancelled"       // Manually cancelled
	BuildAwaitingReview BuildStatus = "awaiting_review" // Waiting for user to review proposed diffs
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
	PowerMax      PowerMode = "max"      // Highest-quality configured provider routes
	PowerBalanced PowerMode = "balanced" // Mid-tier quality/speed balance (Sonnet 4.6, GPT-4.1, Gemini 3 Flash, Grok 3)
	PowerFast     PowerMode = "fast"     // Cheapest mini tier (Haiku 4.5, GPT-4o-mini, Gemini 2.5 Flash Lite, Grok 3 Mini)
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
	ID            string          `json:"id"`
	BuildID       string          `json:"build_id"`
	AppType       string          `json:"app_type"` // web, api, fullstack, etc.
	DeliveryMode  string          `json:"delivery_mode,omitempty"`
	TechStack     TechStack       `json:"tech_stack"`
	Features      []Feature       `json:"features"`
	DataModels    []DataModel     `json:"data_models"`
	APIEndpoints  []APIEndpoint   `json:"api_endpoints"`
	Components    []UIComponent   `json:"components"`
	Files         []PlannedFile   `json:"files"`
	ScaffoldFiles []GeneratedFile `json:"scaffold_files,omitempty"`
	SpecHash      string          `json:"spec_hash,omitempty"`
	ScaffoldID    string          `json:"scaffold_id,omitempty"`
	TemplateID    string          `json:"template_id,omitempty"` // detected app blueprint (e.g. "ai-saas", "crm")
	// SecondaryTemplateIDs are optional supporting blueprints, such as AI SaaS + Landing Page.
	SecondaryTemplateIDs []string               `json:"secondary_template_ids,omitempty"`
	Source               string                 `json:"source,omitempty"`
	Ownership            []BuildOwnership       `json:"ownership,omitempty"`
	EnvVars              []BuildEnvVar          `json:"env_vars,omitempty"`
	Acceptance           []BuildAcceptanceCheck `json:"acceptance,omitempty"`
	WorkOrders           []BuildWorkOrder       `json:"work_orders,omitempty"`
	APIContract          *BuildAPIContract      `json:"api_contract,omitempty"`
	Preflight            []BuildPreflightCheck  `json:"preflight,omitempty"`
	EstimatedTime        time.Duration          `json:"estimated_time"`
	CreatedAt            time.Time              `json:"created_at"`
}

type BuildOwnership struct {
	Path    string    `json:"path"`
	Role    AgentRole `json:"role"`
	Purpose string    `json:"purpose,omitempty"`
}

type BuildEnvVar struct {
	Name     string `json:"name"`
	Example  string `json:"example,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
	Required bool   `json:"required"`
}

type BuildAcceptanceCheck struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Owner       AgentRole `json:"owner"`
	Required    bool      `json:"required"`
}

type BuildPreflightCheck struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Required    bool   `json:"required"`
}

type BuildAPIContract struct {
	FrontendPort int           `json:"frontend_port,omitempty"`
	BackendPort  int           `json:"backend_port,omitempty"`
	APIBaseURL   string        `json:"api_base_url,omitempty"`
	CORSOrigins  []string      `json:"cors_origins,omitempty"`
	Endpoints    []APIEndpoint `json:"endpoints,omitempty"`
}

type BuildWorkOrder struct {
	Role             AgentRole `json:"role"`
	Summary          string    `json:"summary"`
	OwnedFiles       []string  `json:"owned_files,omitempty"`
	RequiredFiles    []string  `json:"required_files,omitempty"`
	ForbiddenFiles   []string  `json:"forbidden_files,omitempty"`
	AcceptanceChecks []string  `json:"acceptance_checks,omitempty"`
	RequiredOutputs  []string  `json:"required_outputs,omitempty"`
}

type TaskStartAck struct {
	Summary          string   `json:"summary"`
	OwnedFiles       []string `json:"owned_files,omitempty"`
	Dependencies     []string `json:"dependencies,omitempty"`
	AcceptanceChecks []string `json:"acceptance_checks,omitempty"`
	Blockers         []string `json:"blockers,omitempty"`
}

type TaskCompletionReport struct {
	Summary         string   `json:"summary"`
	CreatedFiles    []string `json:"created_files,omitempty"`
	ModifiedFiles   []string `json:"modified_files,omitempty"`
	CompletedChecks []string `json:"completed_checks,omitempty"`
	RemainingRisks  []string `json:"remaining_risks,omitempty"`
	Blockers        []string `json:"blockers,omitempty"`
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
	Restorable  bool            `json:"restorable"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Message types for WebSocket communication
type WSMessageType string

const (
	WSBuildStarted          WSMessageType = "build:started"
	WSBuildState            WSMessageType = "build:state"
	WSBuildPhase            WSMessageType = "build:phase"
	WSBuildProgress         WSMessageType = "build:progress"
	WSBuildCheckpoint       WSMessageType = "build:checkpoint"
	WSBuildCompleted        WSMessageType = "build:completed"
	WSBuildError            WSMessageType = "build:error"
	WSAgentSpawned          WSMessageType = "agent:spawned"
	WSAgentWorking          WSMessageType = "agent:working"
	WSAgentProgress         WSMessageType = "agent:progress"
	WSAgentCompleted        WSMessageType = "agent:completed"
	WSAgentError            WSMessageType = "agent:error"
	WSAgentMessage          WSMessageType = "agent:message"
	WSAgentThinking         WSMessageType = "agent:thinking"
	WSAgentAction           WSMessageType = "agent:action"
	WSAgentOutput           WSMessageType = "agent:output"
	WSAgentGenerationFailed WSMessageType = "agent:generation_failed"
	WSAgentGenerating       WSMessageType = "agent:generating"
	WSAgentRetrying         WSMessageType = "agent:retrying"
	WSAgentProviderSwitched WSMessageType = "agent:provider_switched"
	WSFileCreated           WSMessageType = "file:created"
	WSFileUpdated           WSMessageType = "file:updated"
	WSCodeGenerated         WSMessageType = "code:generated"
	WSTerminalOutput        WSMessageType = "terminal:output"
	WSPreviewReady          WSMessageType = "preview:ready"
	WSUserMessage           WSMessageType = "user:message"
	WSLeadResponse          WSMessageType = "lead:response"

	// FSM integration message types (bridged from core.AgentFSM)
	WSBuildFSMStarted        WSMessageType = "build:fsm:started"
	WSBuildFSMInitialized    WSMessageType = "build:fsm:initialized"
	WSBuildFSMPlanReady      WSMessageType = "build:fsm:plan_ready"
	WSBuildFSMStepComplete   WSMessageType = "build:fsm:step_complete"
	WSBuildFSMAllSteps       WSMessageType = "build:fsm:all_steps_complete"
	WSBuildFSMValidationPass WSMessageType = "build:fsm:validation_pass"
	WSBuildFSMValidationFail WSMessageType = "build:fsm:validation_fail"
	WSBuildFSMRetryExhausted WSMessageType = "build:fsm:retry_exhausted"
	WSBuildFSMRollbackDone   WSMessageType = "build:fsm:rollback_complete"
	WSBuildFSMRollbackFail   WSMessageType = "build:fsm:rollback_failed"
	WSBuildFSMPaused         WSMessageType = "build:fsm:paused"
	WSBuildFSMResumed        WSMessageType = "build:fsm:resumed"
	WSBuildFSMCancelled      WSMessageType = "build:fsm:cancelled"
	WSBuildFSMFatalError     WSMessageType = "build:fsm:fatal_error"
	WSBuildFSMCheckpoint     WSMessageType = "build:fsm:checkpoint_created"
	WSBuildFSMRollback       WSMessageType = "build:fsm:rollback"
	WSBuildGuaranteeResult   WSMessageType = "build:guarantee:result"

	// Spend tracking & budget events
	WSSpendUpdate    WSMessageType = "spend:update"
	WSBudgetExceeded WSMessageType = "budget:exceeded"
	WSBudgetWarning  WSMessageType = "budget:warning"

	// Diff workflow events
	WSProposeDiff            WSMessageType = "agent:propose-diff"
	WSEditsApplied           WSMessageType = "build:edits-applied"
	WSAwaitingReview         WSMessageType = "build:awaiting-review"
	WSProtectedPath          WSMessageType = "agent:protected-path"
	WSBuildRollback          WSMessageType = "build:rollback"
	WSBuildInteraction       WSMessageType = "build:interaction"
	WSBuildUserInputRequired WSMessageType = "build:user-input-required"
	WSBuildUserInputResolved WSMessageType = "build:user-input-resolved"
	WSBuildPermissionRequest WSMessageType = "build:permission-request"
	WSBuildPermissionUpdate  WSMessageType = "build:permission-update"

	// Glass-box orchestration telemetry events. These are additive visibility
	// events derived from real orchestration artifacts; existing clients may
	// safely ignore them.
	WSGlassWarRoomCritiqueStarted  WSMessageType = "glassbox:war_room_critique_started"
	WSGlassWarRoomCritiqueResolved WSMessageType = "glassbox:war_room_critique_resolved"
	WSGlassWorkOrderCompiled       WSMessageType = "glassbox:work_order_compiled"
	WSGlassProviderRouteSelected   WSMessageType = "glassbox:provider_route_selected"
	WSGlassDeterministicGatePassed WSMessageType = "glassbox:deterministic_gate_passed"
	WSGlassDeterministicGateFailed WSMessageType = "glassbox:deterministic_gate_failed"
	WSGlassHydraCandidateStarted   WSMessageType = "glassbox:hydra_candidate_started"
	WSGlassHydraCandidatePassed    WSMessageType = "glassbox:hydra_candidate_passed"
	WSGlassHydraCandidateFailed    WSMessageType = "glassbox:hydra_candidate_failed"
	WSGlassHydraWinnerSelected     WSMessageType = "glassbox:hydra_winner_selected"
	WSGlassPatchReviewRequired     WSMessageType = "glassbox:patch_review_required"
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
	Description            string            `json:"description"`
	Prompt                 string            `json:"prompt,omitempty"` // Detailed build prompt (falls back to Description)
	WireframeImage         string            `json:"wireframe_image,omitempty"`
	WireframeDescription   string            `json:"wireframe_description,omitempty"`
	Mode                   BuildMode         `json:"mode"`
	PowerMode              PowerMode         `json:"power_mode,omitempty"`    // max, balanced, fast — controls model quality
	ProviderMode           string            `json:"provider_mode,omitempty"` // platform or byok
	RequirePreviewReady    bool              `json:"require_preview_ready,omitempty"`
	ProjectName            string            `json:"project_name,omitempty"`
	TechStack              *TechStack        `json:"tech_stack,omitempty"`       // Optional override
	DiffMode               bool              `json:"diff_mode,omitempty"`        // When true, proposed changes require user approval
	RoleAssignments        map[string]string `json:"role_assignments,omitempty"` // Optional: user-specified provider per role category (architect→claude, coder→gpt4, etc.)
	ProviderModelOverrides map[string]string `json:"provider_model_overrides,omitempty"`
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
