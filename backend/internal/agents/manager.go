// Package agents - Agent Manager
// This component spawns, tracks, and manages AI agents during builds.
package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"apex-build/internal/ai"
	"apex-build/pkg/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errBuildNotActive      = errors.New("build not active")
	errBuildBudgetExceeded = errors.New("build request budget exceeded")
)

const insufficientCreditsBuildMessage = "Build paused: Your account has insufficient credits. Please add credits in Settings or contact support."

type consensusDecision string

const (
	decisionRetrySame      consensusDecision = "retry_same"
	decisionSwitchProvider consensusDecision = "switch_provider"
	decisionSpawnSolver    consensusDecision = "spawn_solver"
	decisionAbort          consensusDecision = "abort"
)

type providerVote struct {
	Provider  ai.AIProvider
	Decision  consensusDecision
	Rationale string
}

// AgentManager handles the lifecycle and coordination of AI agents
type AgentManager struct {
	agents      map[string]*Agent
	builds      map[string]*Build
	taskQueue   chan *Task
	resultQueue chan *TaskResult
	subscribers map[string][]chan *WSMessage
	aiRouter    AIRouter
	db          *gorm.DB // Database connection for persisting completed builds
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// AIRouter interface for communicating with AI providers
type AIRouter interface {
	Generate(ctx context.Context, provider ai.AIProvider, prompt string, opts GenerateOptions) (*ai.AIResponse, error)
	GetAvailableProviders() []ai.AIProvider
	GetAvailableProvidersForUser(userID uint) []ai.AIProvider
	HasConfiguredProviders() bool
}

// GenerateOptions for AI generation requests
type GenerateOptions struct {
	UserID       uint
	MaxTokens    int
	Temperature  float64
	SystemPrompt string
	Context      []Message
	PowerMode    PowerMode // Controls which model tier is used (max/balanced/fast)
	// Platform-key app builds should not route through user BYOK state.
	UsePlatformKeys bool
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
func NewAgentManager(aiRouter AIRouter, db ...*gorm.DB) *AgentManager {
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
	if len(db) > 0 && db[0] != nil {
		am.db = db[0]
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

	powerMode := req.PowerMode
	if powerMode == "" {
		powerMode = PowerFast // Default to cheapest models when not explicitly set
	}
	providerMode := strings.ToLower(strings.TrimSpace(req.ProviderMode))
	if providerMode != "byok" {
		providerMode = "platform"
	}

	build := &Build{
		ID:                  buildID,
		UserID:              userID,
		Status:              BuildPending,
		Mode:                mode,
		PowerMode:           powerMode,
		ProviderMode:        providerMode,
		RequirePreviewReady: req.RequirePreviewReady,
		Description:         req.Description,
		TechStack:           req.TechStack,
		Agents:              make(map[string]*Agent),
		Tasks:               make([]*Task, 0),
		Checkpoints:         make([]*Checkpoint, 0),
		Progress:            0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Apply guardrails for cost control
	maxAgents, maxRetries, maxRequests, maxTokens := am.defaultBuildLimitsForBuild(build)
	// Ensure power mode token scaling isn't capped by conservative defaults
	if !am.hasTokenLimitOverride(mode) {
		minTokens := am.getPowerModeTokenCap(powerMode)
		if maxTokens < minTokens {
			maxTokens = minTokens
		}
	}
	build.MaxAgents = maxAgents
	build.MaxRetries = maxRetries
	build.MaxRequests = maxRequests
	build.MaxTokensPerRequest = maxTokens

	am.builds[buildID] = build
	am.persistBuildSnapshot(build, nil)

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
	am.persistBuildSnapshot(build, nil)

	log.Printf("Build %s status updated, broadcasting", buildID)

	// Broadcast build started
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildStarted,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":        string(BuildPlanning),
			"description":   build.Description,
			"mode":          string(build.Mode),
			"power_mode":    string(build.PowerMode),
			"provider_mode": build.ProviderMode,
		},
	})

	usePlatformKeys := am.buildUsesPlatformKeys(build)
	if !usePlatformKeys && !am.userHasActiveBYOKKey(build.UserID) {
		build.mu.Lock()
		build.Status = BuildFailed
		build.Error = "No active BYOK providers configured for this user"
		build.UpdatedAt = time.Now()
		build.mu.Unlock()
		am.persistBuildSnapshot(build, nil)
		am.broadcast(buildID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   buildID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":   "No active BYOK providers configured",
				"details": "This build requested provider_mode=byok, but no active BYOK keys are configured for the current user.",
			},
		})
		return fmt.Errorf("no active BYOK providers configured")
	}

	availableProviders := am.getAvailableProvidersWithGracePeriodForBuild(build)
	if len(availableProviders) == 0 {
		build.mu.Lock()
		build.Status = BuildFailed
		build.Error = "No AI providers available"
		build.UpdatedAt = time.Now()
		build.mu.Unlock()
		am.persistBuildSnapshot(build, nil)
		am.broadcast(buildID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   buildID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":   "No AI providers available",
				"details": "Please check your AI provider configuration. For platform mode, configure Claude/OpenAI/Gemini keys. For BYOK mode, verify your user BYOK provider is configured and reachable.",
			},
		})
		return fmt.Errorf("no AI providers available")
	}

	// Select best provider for lead agent
	// PRIORITY: Ollama (BYOK/Local) > Claude > GPT > Gemini
	// If the user has brought their own local model, we MUST use it to avoid platform costs.
	leadProvider := availableProviders[0]
	useOllama := false

	// Check for Ollama first
	for _, p := range availableProviders {
		if p == ai.ProviderOllama {
			leadProvider = ai.ProviderOllama
			useOllama = true
			break
		}
	}

	// If not using Ollama, fall back to standard platform hierarchy
	if !useOllama {
		for _, p := range availableProviders {
			if p == ai.ProviderClaude {
				leadProvider = ai.ProviderClaude
				break
			}
			if p == ai.ProviderGPT4 && leadProvider != ai.ProviderClaude {
				leadProvider = ai.ProviderGPT4
			}
		}
	}

	log.Printf("Spawning lead agent with provider: %s (available: %v)", leadProvider, availableProviders)

	// Spawn the lead agent with selected provider
	var leadAgent *Agent
	var err error
	providerOrder := make([]ai.AIProvider, 0, len(availableProviders))
	providerOrder = append(providerOrder, leadProvider)
	for _, provider := range availableProviders {
		if provider != leadProvider {
			providerOrder = append(providerOrder, provider)
		}
	}
	for _, provider := range providerOrder {
		leadAgent, err = am.spawnAgent(buildID, RoleLead, provider)
		if err == nil {
			log.Printf("Successfully spawned lead agent with %s", provider)
			break
		}
		log.Printf("Failed to spawn lead agent with %s: %v, trying next provider", provider, err)
	}

	if err != nil {
		build.mu.Lock()
		build.Status = BuildFailed
		build.Error = fmt.Sprintf("Failed to spawn lead agent: %v", err)
		build.UpdatedAt = time.Now()
		build.mu.Unlock()
		am.persistBuildSnapshot(build, nil)

		am.broadcast(buildID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   buildID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":   "Failed to spawn lead agent with any available provider",
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
		MaxRetries:  build.MaxRetries,
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
			"model":       leadAgent.Model,
		},
	})

	// Queue the planning task
	am.taskQueue <- planTask

	// Start build timeout goroutine - fail cleanly if build exceeds SLA.
	go am.buildTimeoutHandler(buildID)

	// Start inactivity monitor - fail build if no AI activity for 45 seconds
	go am.inactivityMonitor(buildID)

	log.Printf("Build %s started with lead agent %s, planning task %s queued", buildID, leadAgent.ID, planTask.ID)
	return nil
}

// buildTimeoutHandler fails builds that run past timeout instead of marking them as completed.
func (am *AgentManager) buildTimeoutHandler(buildID string) {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return
	}

	timeout := am.buildTimeoutForBuild(build)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-am.ctx.Done():
		return
	}

	build.mu.RLock()
	status := build.Status
	build.mu.RUnlock()

	// If build is still in progress, fail it as timeout.
	if status == BuildPlanning || status == BuildInProgress || status == BuildTesting || status == BuildReviewing {
		log.Printf("Build %s timed out after %v, marking as failed", buildID, timeout)
		am.failBuildOnTimeout(buildID, timeout)
	}
}

func (am *AgentManager) buildTimeoutForMode(mode BuildMode) time.Duration {
	defaultSeconds := 240 // fast: 4 minutes
	envKey := "BUILD_TIMEOUT_FAST_SECONDS"
	if mode == ModeFull {
		defaultSeconds = 600 // full: 10 minutes
		envKey = "BUILD_TIMEOUT_FULL_SECONDS"
	}

	// Local Ollama (single-provider, dev mode) is substantially slower than cloud providers.
	// Raise the default timeout only when the operator has not already set an explicit timeout.
	if _, explicitlySet := os.LookupEnv(envKey); !explicitlySet && am.isLocalDevSingleOllamaProfile() {
		if mode == ModeFull {
			defaultSeconds = 1800 // 30 minutes for local full builds
		} else {
			defaultSeconds = 900 // 15 minutes for local fast builds
		}
	}

	seconds := envInt(envKey, defaultSeconds)
	if seconds < 30 {
		seconds = 30
	}

	return time.Duration(seconds) * time.Second
}

func (am *AgentManager) buildTimeoutForBuild(build *Build) time.Duration {
	if build == nil {
		return am.buildTimeoutForMode(ModeFast)
	}

	defaultSeconds := 240 // fast: 4 minutes
	envKey := "BUILD_TIMEOUT_FAST_SECONDS"
	if build.Mode == ModeFull {
		defaultSeconds = 600 // full: 10 minutes
		envKey = "BUILD_TIMEOUT_FULL_SECONDS"
	}

	explicitSeconds, explicitlySet := os.LookupEnv(envKey)
	if !explicitlySet {
		switch {
		case am.isLocalDevSingleOllamaProfile():
			if build.Mode == ModeFull {
				defaultSeconds = 1800
			} else {
				defaultSeconds = 900
			}
		case am.isLocalDevStrictPreviewBuild(build):
			// Strict preview-ready verification (including install/build/preview smoke)
			// needs more headroom locally, even with cloud providers.
			if build.Mode == ModeFull {
				defaultSeconds = 1800
			} else {
				defaultSeconds = 900
			}
		}
	}

	seconds := defaultSeconds
	if explicitlySet {
		if parsed, err := strconv.Atoi(strings.TrimSpace(explicitSeconds)); err == nil {
			seconds = parsed
		}
	}
	if seconds < 30 {
		seconds = 30
	}

	return time.Duration(seconds) * time.Second
}

func (am *AgentManager) isLocalDevStrictPreviewBuild(build *Build) bool {
	if am == nil || build == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production") {
		return false
	}
	return build.RequirePreviewReady
}

func (am *AgentManager) isLocalDevSingleOllamaProfile() bool {
	if am == nil || am.aiRouter == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production") {
		return false
	}
	if strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")) == "" {
		return false
	}

	providers := am.aiRouter.GetAvailableProviders()
	return len(providers) == 1 && providers[0] == ai.ProviderOllama
}

func (am *AgentManager) buildUsesPlatformKeys(build *Build) bool {
	if build == nil {
		return true
	}
	return strings.ToLower(strings.TrimSpace(build.ProviderMode)) != "byok"
}

func (am *AgentManager) userHasActiveBYOKKey(userID uint) bool {
	if am == nil || am.db == nil || userID == 0 {
		return false
	}

	var count int64
	if err := am.db.Model(&models.UserAPIKey{}).
		Where("user_id = ? AND is_active = ? AND deleted_at IS NULL", userID, true).
		Count(&count).Error; err != nil {
		log.Printf("Failed to check BYOK keys for user %d: %v", userID, err)
		return false
	}
	return count > 0
}

func (am *AgentManager) getAvailableProvidersWithGracePeriodForBuild(build *Build) []ai.AIProvider {
	if build != nil && !am.buildUsesPlatformKeys(build) {
		providers := am.aiRouter.GetAvailableProvidersForUser(build.UserID)
		if len(providers) == 0 {
			log.Printf("No BYOK providers available for user %d", build.UserID)
		}
		return providers
	}
	return am.getAvailableProvidersWithGracePeriod()
}

func (am *AgentManager) getCurrentlyAvailableProvidersForBuild(build *Build) []ai.AIProvider {
	if build != nil && !am.buildUsesPlatformKeys(build) {
		return am.aiRouter.GetAvailableProvidersForUser(build.UserID)
	}
	return am.aiRouter.GetAvailableProviders()
}

// inactivityMonitor checks for build inactivity and broadcasts errors if AI isn't responding
func (am *AgentManager) inactivityMonitor(buildID string) {
	// Check every 15 seconds
	checkInterval := 15 * time.Second
	inactivityThreshold := 120 * time.Second
	maxInactivityWarnings := 5
	warningCount := 0

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		am.mu.RLock()
		build, exists := am.builds[buildID]
		am.mu.RUnlock()

		if !exists {
			return
		}

		build.mu.RLock()
		status := build.Status
		lastUpdate := build.UpdatedAt
		build.mu.RUnlock()

		// Stop monitoring if build is complete or failed
		if status == BuildCompleted || status == BuildFailed || status == BuildCancelled {
			return
		}

		// Check if there's been any activity
		timeSinceUpdate := time.Since(lastUpdate)
		if timeSinceUpdate > inactivityThreshold {
			warningCount++
			log.Printf("Build %s: No activity for %v (warning %d/%d)", buildID, timeSinceUpdate.Round(time.Second), warningCount, maxInactivityWarnings)

			// Broadcast inactivity warning to frontend
			am.broadcast(buildID, &WSMessage{
				Type:      WSBuildProgress,
				BuildID:   buildID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"phase":              "waiting",
					"message":            fmt.Sprintf("Waiting for AI response... (no activity for %v)", timeSinceUpdate.Round(time.Second)),
					"inactivity_warning": true,
					"warning_count":      warningCount,
				},
			})

			// If too many warnings, broadcast an error
			if warningCount >= maxInactivityWarnings {
				log.Printf("Build %s: Inactivity threshold exceeded, broadcasting AI availability warning", buildID)
				am.broadcast(buildID, &WSMessage{
					Type:      WSBuildError,
					BuildID:   buildID,
					Timestamp: time.Now(),
					Data: map[string]any{
						"error":           "AI providers may be unavailable or rate-limited",
						"details":         "No AI activity detected. The build will continue trying but may take longer than expected. Check your API key configuration.",
						"recoverable":     true,
						"inactivity_time": timeSinceUpdate.Round(time.Second).String(),
					},
				})
				// Reset warning count but keep monitoring
				warningCount = 0
			}
		} else {
			// Activity detected, reset warning count
			warningCount = 0
		}
	}
}

// failBuildOnTimeout marks a timed-out build as failed and preserves partial artifacts.
func (am *AgentManager) failBuildOnTimeout(buildID string, timeout time.Duration) {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return
	}

	build.mu.Lock()
	if build.Status == BuildCompleted || build.Status == BuildFailed || build.Status == BuildCancelled {
		build.mu.Unlock()
		return
	}

	now := time.Now()
	build.CompletedAt = &now
	build.UpdatedAt = now
	build.Status = BuildFailed
	if strings.TrimSpace(build.Error) == "" {
		build.Error = fmt.Sprintf("Build timed out after %v before all tasks completed", timeout.Round(time.Second))
	}

	cancelledTasks := 0
	// Cancel any pending/in-progress tasks to stop the pipeline.
	for _, task := range build.Tasks {
		if task.Status == TaskPending || task.Status == TaskInProgress {
			task.Status = TaskCancelled
			cancelledTasks++
		}
	}
	progress := build.Progress
	build.mu.Unlock()

	am.createCheckpoint(build, "Build Timed Out", "Build exceeded allowed execution time and was stopped")

	allFiles := am.collectGeneratedFiles(build)

	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildError,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"status":                string(BuildFailed),
			"error":                 "Build timed out",
			"details":               fmt.Sprintf("Build exceeded timeout of %v and was stopped before completion.", timeout.Round(time.Second)),
			"progress":              progress,
			"timed_out":             true,
			"files_count":           len(allFiles),
			"files":                 allFiles,
			"cancelled_tasks":       cancelledTasks,
			"recoverable":           false,
			"quality_gate_required": true,
			"quality_gate_passed":   false,
			"quality_gate_stage":    "validation",
		},
	})

	log.Printf("Build %s timed out: marked failed with %d files and %d cancelled tasks", buildID, len(allFiles), cancelledTasks)

	// Persist to database
	am.persistCompletedBuild(build, allFiles)
}

// spawnAgent creates a new AI agent with a specific role
func (am *AgentManager) spawnAgent(buildID string, role AgentRole, provider ai.AIProvider) (*Agent, error) {
	am.mu.Lock()

	build, exists := am.builds[buildID]
	if !exists {
		am.mu.Unlock()
		return nil, fmt.Errorf("build %s not found", buildID)
	}

	agentID := uuid.New().String()
	now := time.Now()

	model := selectModelForPowerMode(provider, build.PowerMode)
	if model == "" {
		model = "auto"
	}

	agent := &Agent{
		ID:        agentID,
		Role:      role,
		Provider:  provider,
		Model:     model,
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
			"model":    model,
		},
	})

	log.Printf("Spawned %s agent %s (provider: %s) for build %s", role, agentID, provider, buildID)
	return agent, nil
}

// SpawnAgentTeam creates the full team of agents for a build
func (am *AgentManager) SpawnAgentTeam(buildID string) error {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()
	if !exists {
		return fmt.Errorf("build %s not found", buildID)
	}

	availableProviders := am.getCurrentlyAvailableProvidersForBuild(build)

	if len(availableProviders) == 0 {
		if am.buildUsesPlatformKeys(build) {
			return fmt.Errorf("no AI providers available - please check API key configuration")
		}
		return fmt.Errorf("no AI providers available for this user BYOK build - please check BYOK key configuration")
	}

	// Define mandatory and optional roles.
	// Reviewer is mandatory so every build gets an explicit quality gate.
	mandatoryRoles := []AgentRole{
		RoleArchitect,
		RoleDatabase,
		RoleBackend,
		RoleFrontend,
		RoleTesting,
		RoleReviewer,
	}
	optionalRoles := []AgentRole{
		RolePlanner,
	}

	roles := append([]AgentRole{}, mandatoryRoles...)

	// Local single-provider Ollama builds benefit from a smaller team to reduce latency and timeout risk.
	// Keep architecture + code generation + review, and skip database/testing specialists by default.
	if am.isLocalDevSingleOllamaProfile() {
		mandatoryRoles = []AgentRole{
			RoleArchitect,
			RoleBackend,
			RoleFrontend,
			RoleReviewer,
		}
		roles = append([]AgentRole{}, mandatoryRoles...)
		log.Printf("Build %s using reduced local Ollama agent profile (%d roles)", buildID, len(mandatoryRoles))
	} else if am.isLocalDevStrictPreviewBuild(build) {
		// Local strict preview builds already run deterministic frontend/backend build verification.
		// Skipping the testing specialist reduces token spend and avoids test-framework drift.
		mandatoryRoles = []AgentRole{
			RoleArchitect,
			RoleDatabase,
			RoleBackend,
			RoleFrontend,
			RoleReviewer,
		}
		roles = append([]AgentRole{}, mandatoryRoles...)
		log.Printf("Build %s using reduced local strict-preview agent profile (%d roles, testing agent disabled)", buildID, len(mandatoryRoles))
	}

	// Enforce max agents (excluding lead)
	if build.MaxAgents > 0 {
		allowed := build.MaxAgents - 1
		if allowed < 0 {
			allowed = 0
		}

		if allowed < len(mandatoryRoles) {
			log.Printf("Build %s max_agents=%d is below mandatory orchestration floor (%d); enforcing mandatory roles",
				buildID, build.MaxAgents, len(mandatoryRoles)+1)
			allowed = len(mandatoryRoles)
		}

		remaining := allowed - len(mandatoryRoles)
		if remaining > 0 {
			if remaining > len(optionalRoles) {
				remaining = len(optionalRoles)
			}
			roles = append(roles, optionalRoles[:remaining]...)
		}
	} else {
		roles = append(roles, optionalRoles...)
	}

	// Determine provider assignments based on availability
	providerAssignments := am.assignProvidersToRoles(availableProviders, roles)

	// Broadcast provider availability status
	providerNames := make([]string, len(availableProviders))
	for i, p := range availableProviders {
		providerNames[i] = string(p)
	}
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":               "provider_check",
			"available_providers": providerNames,
			"provider_count":      len(availableProviders),
			"message":             fmt.Sprintf("Using %d available AI provider(s): %v", len(availableProviders), providerNames),
		},
	})

	// Spawn agents with resilience for single-provider scenarios
	isSingleProvider := len(availableProviders) == 1
	failedRoles := make([]AgentRole, 0)

	for _, role := range roles {
		provider, ok := providerAssignments[role]
		if !ok {
			continue
		}

		err := am.spawnAgentWithRetries(buildID, role, provider, isSingleProvider)
		if err != nil {
			log.Printf("Warning: failed to spawn %s agent with %s: %v", role, provider, err)

			if !isSingleProvider {
				// Try with fallback providers if available
				var fallbackSucceeded bool
				for _, fallback := range availableProviders {
					if fallback != provider {
						log.Printf("Trying fallback provider %s for %s agent", fallback, role)
						err = am.spawnAgentWithRetries(buildID, role, fallback, false)
						if err == nil {
							fallbackSucceeded = true
							break
						}
					}
				}
				if !fallbackSucceeded {
					failedRoles = append(failedRoles, role)
				}
			} else {
				// Single provider scenario: collect failed roles for lead agent handling
				failedRoles = append(failedRoles, role)
			}
		}
	}

	// Handle failed roles in single-provider scenarios
	if isSingleProvider && len(failedRoles) > 0 {
		return am.handleSingleProviderFailures(buildID, failedRoles, availableProviders[0])
	} else if len(failedRoles) > 0 {
		return fmt.Errorf("failed to spawn agents for roles: %v", failedRoles)
	}

	log.Printf("Successfully spawned agent team for build %s with %d providers", buildID, len(availableProviders))
	return nil
}

// assignProvidersToRoles distributes providers to agent roles based on availability
func (am *AgentManager) assignProvidersToRoles(providers []ai.AIProvider, roles []AgentRole) map[AgentRole]ai.AIProvider {
	assignments := make(map[AgentRole]ai.AIProvider)

	// Build a quick lookup for availability
	available := make(map[ai.AIProvider]bool)
	for _, p := range providers {
		available[p] = true
	}

	// CAPABILITY-BASED LEAD SELECTION
	// The most capable available model becomes the lead, regardless of type
	leadProvider := am.selectLeadProvider(providers)
	log.Printf("Assigning providers to roles: %d providers available (lead=%s)", len(providers), leadProvider)

	// Helper to pick the first available provider in preference order
	pick := func(preferences ...ai.AIProvider) ai.AIProvider {
		for _, p := range preferences {
			if available[p] {
				return p
			}
		}
		return leadProvider
	}

	// Role-to-provider preferences with graceful fallback when a preferred provider is unavailable.
	for _, role := range roles {
		switch role {
		case RolePlanner, RoleArchitect, RoleReviewer:
			assignments[role] = pick(ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok, ai.ProviderOllama)
		case RoleFrontend, RoleBackend, RoleDatabase, RoleSolver:
			assignments[role] = pick(ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGemini, ai.ProviderGrok, ai.ProviderOllama)
		case RoleTesting:
			assignments[role] = pick(ai.ProviderGemini, ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGrok, ai.ProviderOllama)
		default:
			assignments[role] = leadProvider
		}
	}

	// Explicit policy enforcement:
	// - Claude owns architecture/planning/review
	// - GPT owns coding/build roles
	// - Gemini owns testing
	// This only applies when each preferred provider is actually available.
	for _, role := range roles {
		switch role {
		case RolePlanner, RoleArchitect, RoleReviewer:
			if available[ai.ProviderClaude] {
				assignments[role] = ai.ProviderClaude
			}
		case RoleFrontend, RoleBackend, RoleDatabase, RoleSolver:
			if available[ai.ProviderGPT4] {
				assignments[role] = ai.ProviderGPT4
			}
		case RoleTesting:
			if available[ai.ProviderGemini] {
				assignments[role] = ai.ProviderGemini
			}
		}
	}

	if !available[ai.ProviderClaude] {
		log.Printf("Provider policy notice: Claude unavailable, architecture roles will use fallback providers")
	}
	if !available[ai.ProviderGPT4] {
		log.Printf("Provider policy notice: OpenAI unavailable, coding roles will use fallback providers")
	}
	if !available[ai.ProviderGemini] {
		log.Printf("Provider policy notice: Gemini unavailable, testing role will use fallback providers")
	}

	// Log final assignments
	for role, provider := range assignments {
		log.Printf("Agent %s -> Provider %s", role, provider)
	}

	return assignments
}

// selectLeadProvider chooses the most capable provider from available options
// Lead provider handles critical planning, architecture, and decision-making tasks
func (am *AgentManager) selectLeadProvider(providers []ai.AIProvider) ai.AIProvider {
	// Provider capability ranking (highest to lowest)
	// Claude: Best for reasoning, planning, architecture decisions
	// GPT-4: Strong for complex code generation and problem-solving
	// Gemini: Good general purpose model
	// Grok: Alternative reasoning model
	// Ollama: Local model (capability depends on underlying model, assume good but not top)

	capabilityRank := map[ai.AIProvider]int{
		ai.ProviderClaude: 5, // Highest capability for reasoning and planning
		ai.ProviderGPT4:   4, // Strong for code generation and complex tasks
		ai.ProviderGemini: 3, // Good general purpose
		ai.ProviderGrok:   2, // Alternative option
		ai.ProviderOllama: 1, // Local model (good but depends on specific model)
	}

	var bestProvider ai.AIProvider
	bestRank := 0

	for _, provider := range providers {
		if rank := capabilityRank[provider]; rank > bestRank {
			bestRank = rank
			bestProvider = provider
		}
	}

	log.Printf("Provider capability analysis: selected %s (rank %d) from %v",
		bestProvider, bestRank, providers)

	return bestProvider
}

// spawnAgentWithRetries attempts to spawn an agent with retry logic for transient failures
func (am *AgentManager) spawnAgentWithRetries(buildID string, role AgentRole, provider ai.AIProvider, isSingleProvider bool) error {
	maxRetries := 1 // Default retries for multi-provider scenarios
	if isSingleProvider {
		maxRetries = 3 // More retries for single-provider scenarios
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := am.spawnAgent(buildID, role, provider)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < maxRetries {
			delay := time.Duration(attempt) * time.Second
			log.Printf("Agent spawn attempt %d/%d failed for %s with %s, retrying in %v: %v",
				attempt, maxRetries, role, provider, delay, err)
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to spawn %s agent after %d attempts: %w", role, maxRetries, lastErr)
}

// handleSingleProviderFailures manages failed role spawning in single-provider scenarios
// Instead of failing the build, we notify that the lead agent will handle these roles
func (am *AgentManager) handleSingleProviderFailures(buildID string, failedRoles []AgentRole, provider ai.AIProvider) error {
	if len(failedRoles) == 0 {
		return nil
	}

	// Log the failure and mitigation strategy
	log.Printf("Single provider scenario: %d agent roles failed to spawn, lead agent will handle these roles: %v",
		len(failedRoles), failedRoles)

	// Broadcast the mitigation message to the frontend
	roleNames := make([]string, len(failedRoles))
	for i, role := range failedRoles {
		roleNames[i] = string(role)
	}

	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":   "agent_spawn_mitigation",
			"message": fmt.Sprintf("Some specialized agents couldn't be created, but the lead agent will handle %s tasks to ensure build completion", roleNames),
			"warning": true,
		},
	})

	// In a single-provider scenario, we don't fail the build entirely
	// The lead agent will coordinate and handle these tasks
	return nil
}

// getAvailableProvidersWithGracePeriod attempts to get providers with retries for startup scenarios
func (am *AgentManager) getAvailableProvidersWithGracePeriod() []ai.AIProvider {
	// Try immediate check first
	providers := am.aiRouter.GetAvailableProviders()
	if len(providers) > 0 {
		return providers
	}

	// If no providers are configured at all, fail fast instead of waiting through
	// the startup grace period (which only helps when providers exist but health
	// checks haven't reported yet).
	if !am.aiRouter.HasConfiguredProviders() {
		log.Printf("No AI providers configured; skipping startup grace period")
		return providers
	}

	// If no providers initially, wait with grace period for health checks to complete
	log.Printf("No platform providers available immediately, waiting for health checks...")

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		time.Sleep(time.Duration(attempt*2) * time.Second) // 2s, 4s, 6s delays
		providers = am.aiRouter.GetAvailableProviders()
		if len(providers) > 0 {
			log.Printf("Providers became available after %d attempts: %v", attempt, providers)
			return providers
		}
		log.Printf("Grace period attempt %d/%d: still no providers available", attempt, maxAttempts)
	}

	log.Printf("No platform providers available after grace period")
	return providers // Will be empty
}

// AssignTask assigns a task to a specific agent
func (am *AgentManager) AssignTask(agentID string, task *Task) error {
	am.mu.RLock()
	agent, exists := am.agents[agentID]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	am.mu.RLock()
	build, buildExists := am.builds[agent.BuildID]
	am.mu.RUnlock()
	if !buildExists {
		return fmt.Errorf("build %s not found", agent.BuildID)
	}

	build.mu.RLock()
	buildInactive := build.Status == BuildFailed || build.Status == BuildCancelled || build.Status == BuildCompleted
	build.mu.RUnlock()
	if buildInactive {
		task.Status = TaskCancelled
		return errBuildNotActive
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
			"agent_role":  string(agent.Role),
			"provider":    string(agent.Provider),
			"model":       agent.Model,
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

	// Apply retry strategy if this is a retry attempt
	if task.RetryCount > 0 {
		strategy := string(task.RetryStrategy)
		switch strategy {
		case "backoff":
			delay := time.Duration(task.RetryCount) * 2 * time.Second
			log.Printf("Backoff strategy: waiting %v before retry (attempt %d)", delay, task.RetryCount)
			time.Sleep(delay)
		case "switch_provider":
			newProvider := am.getNextFallbackProvider(agent.Provider)
			if newProvider != agent.Provider {
				oldProvider := agent.Provider
				agent.mu.Lock()
				agent.Provider = newProvider
				agent.Model = selectModelForPowerMode(newProvider, build.PowerMode)
				agent.mu.Unlock()
				log.Printf("Switch provider strategy: %s → %s (model: %s)", oldProvider, newProvider, agent.Model)
				am.broadcast(agent.BuildID, &WSMessage{
					Type:      "agent:provider_switched",
					BuildID:   agent.BuildID,
					AgentID:   agent.ID,
					Timestamp: time.Now(),
					Data: map[string]any{
						"old_provider": string(oldProvider),
						"new_provider": string(newProvider),
						"model":        agent.Model,
						"reason":       "retry_strategy",
					},
				})
			}
		case "reduce_context":
			log.Printf("Reduce context strategy: will use 75%% tokens (attempt %d)", task.RetryCount)
		case "fix_and_retry":
			log.Printf("Fix and retry strategy: injecting fix guidance (attempt %d)", task.RetryCount)
		default:
			log.Printf("Standard retry (attempt %d/%d)", task.RetryCount, task.MaxRetries)
		}
	}

	// Guardrails: stop if build already failed/cancelled
	build.mu.Lock()
	if build.Status == BuildFailed || build.Status == BuildCancelled {
		build.mu.Unlock()
		task.MaxRetries = 0
		task.Status = TaskCancelled
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   errBuildNotActive,
		}
		return
	}

	// Guardrails: enforce per-build request budget
	if build.MaxRequests > 0 && build.RequestsUsed >= build.MaxRequests {
		build.Status = BuildFailed
		build.Error = "Build budget exceeded (request limit)"
		build.UpdatedAt = time.Now()
		build.mu.Unlock()
		task.MaxRetries = 0
		task.Status = TaskCancelled

		am.cancelPendingTasks(build)
		am.persistBuildSnapshot(build, nil)
		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":        "Build budget exceeded",
				"details":      "Maximum AI requests reached for this build",
				"reason":       "budget_exceeded",
				"max_requests": build.MaxRequests,
				"used":         build.RequestsUsed,
				"recoverable":  false,
			},
		})

		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   errBuildBudgetExceeded,
		}
		return
	}

	build.RequestsUsed++
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	// Broadcast that the agent is thinking with task-specific detail
	thinkingContent := fmt.Sprintf("%s agent is working on %s", agent.Role, string(task.Type))
	if task.Description != "" {
		thinkingContent = fmt.Sprintf("%s agent is analyzing: %s", agent.Role, task.Description)
	}
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      "agent:thinking",
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role": agent.Role,
			"provider":   agent.Provider,
			"model":      agent.Model,
			"task_id":    task.ID,
			"task_type":  string(task.Type),
			"content":    thinkingContent,
		},
	})

	// Build the prompt based on task type
	prompt := am.buildTaskPrompt(task, build, agent)
	systemPrompt := am.getSystemPrompt(agent.Role, build)
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
			"model":      agent.Model,
			"task_id":    task.ID,
			"task_type":  string(task.Type),
			"content":    fmt.Sprintf("%s agent is generating code with %s...", agent.Role, agent.Provider),
		},
	})

	log.Printf("Calling AI router for task %s with provider %s", task.ID, agent.Provider)

	// Role-aware token limits scaled by power mode
	maxTokens := am.getMaxTokensForRole(agent.Role, build.PowerMode)
	if am.isLocalDevStrictPreviewBuild(build) {
		switch agent.Role {
		case RoleArchitect, RoleDatabase, RoleTesting, RoleSolver:
			// Complex strict preview builds frequently truncate schema/seed/test outputs.
			// Give these roles more room to return complete files.
			boosted := maxTokens * 3 / 2
			if boosted > maxTokens {
				maxTokens = boosted
			}
		}
	}
	if task.RetryCount > 0 && taskHasRecentTruncationError(task) {
		// Prefer a larger response budget when the prior attempt was truncated.
		boosted := maxTokens * 3 / 2
		if boosted > maxTokens {
			maxTokens = boosted
		}
	}
	if build.MaxTokensPerRequest > 0 && build.MaxTokensPerRequest < maxTokens {
		maxTokens = build.MaxTokensPerRequest
	}
	// Apply reduce_context strategy: cut tokens by 25% on retry
	if task.RetryCount > 0 && string(task.RetryStrategy) == "reduce_context" {
		maxTokens = maxTokens * 3 / 4
	}

	// Role-aware temperature
	temperature := am.getTemperatureForRole(agent.Role)

	response, err := am.aiRouter.Generate(ctx, agent.Provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		MaxTokens:       maxTokens,
		Temperature:     temperature,
		SystemPrompt:    systemPrompt,
		PowerMode:       build.PowerMode,
		UsePlatformKeys: am.buildUsesPlatformKeys(build),
	})

	if err != nil {
		log.Printf("AI generation failed for task %s: %v", task.ID, err)
		nextRetryCount := task.RetryCount + 1
		willRetry := nextRetryCount < task.MaxRetries && !am.isNonRetriableAIError(err)

		// Broadcast the error
		am.broadcast(agent.BuildID, &WSMessage{
			Type:      "agent:generation_failed",
			BuildID:   agent.BuildID,
			AgentID:   agent.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"agent_role":  agent.Role,
				"provider":    agent.Provider,
				"model":       agent.Model,
				"task_id":     task.ID,
				"error":       err.Error(),
				"retry_count": task.RetryCount,
				"max_retries": task.MaxRetries,
				"will_retry":  willRetry,
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

	if response == nil || response.Content == "" {
		log.Printf("AI generation returned empty response for task %s", task.ID)
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   fmt.Errorf("AI generation returned empty response"),
		}
		return
	}

	modelUsed := ai.GetModelUsed(response, nil)
	if modelUsed != "" && modelUsed != "unknown" {
		agent.mu.Lock()
		agent.Model = modelUsed
		agent.mu.Unlock()
	}

	log.Printf("AI generation succeeded for task %s (response_length: %d)", task.ID, len(response.Content))

	// Parse the response into task output
	output := am.parseTaskOutput(task.Type, response.Content)

	// Broadcast completion thought with actual model and file details
	fileNames := make([]string, 0, len(output.Files))
	for _, f := range output.Files {
		fileNames = append(fileNames, f.Path)
	}
	completionContent := fmt.Sprintf("Completed %s — generated %d file(s)", string(task.Type), len(output.Files))
	if len(fileNames) > 0 && len(fileNames) <= 5 {
		completionContent += ": " + strings.Join(fileNames, ", ")
	} else if len(fileNames) > 5 {
		completionContent += ": " + strings.Join(fileNames[:5], ", ") + fmt.Sprintf(" (+%d more)", len(fileNames)-5)
	}
	displayModel := modelUsed
	if displayModel == "" || displayModel == "unknown" {
		displayModel = agent.Model
	}
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      "agent:thinking",
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role": agent.Role,
			"provider":   agent.Provider,
			"model":      displayModel,
			"task_id":    task.ID,
			"task_type":  string(task.Type),
			"content":    completionContent,
		},
	})

	// Broadcast code generated with file count
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      WSCodeGenerated,
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role":  agent.Role,
			"provider":    agent.Provider,
			"model":       agent.Model,
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

// processResult handles a task result with retry logic and build verification
func (am *AgentManager) processResult(result *TaskResult) {
	if result == nil {
		return
	}

	am.mu.RLock()
	agent, agentExists := am.agents[result.AgentID]
	am.mu.RUnlock()

	if !agentExists {
		log.Printf("Warning: result for unknown agent %s", result.AgentID)
		return
	}

	agent.mu.Lock()
	task := agent.CurrentTask
	if task == nil || task.ID != result.TaskID {
		// Stale/out-of-order result for a task this agent is no longer executing.
		agent.mu.Unlock()
		log.Printf("Dropping stale task result for agent %s: result task=%s current task=%v", result.AgentID, result.TaskID, func() string {
			if task == nil {
				return "<nil>"
			}
			return task.ID
		}())
		return
	}
	if task.Status == TaskCancelled {
		agent.mu.Unlock()
		log.Printf("Dropping result for cancelled task %s (agent %s)", task.ID, result.AgentID)
		return
	}

	build, buildErr := am.GetBuild(agent.BuildID)
	buildInactive := false
	if buildErr == nil {
		build.mu.RLock()
		buildInactive = build.Status == BuildFailed || build.Status == BuildCancelled || build.Status == BuildCompleted
		build.mu.RUnlock()
	}

	if result.Error != nil && (errors.Is(result.Error, errBuildNotActive) || errors.Is(result.Error, errBuildBudgetExceeded)) {
		if task != nil && task.Status != TaskCompleted {
			task.Status = TaskCancelled
			task.Error = result.Error.Error()
		}
		agent.Status = StatusError
		agent.Error = result.Error.Error()
		agent.UpdatedAt = time.Now()
		agent.mu.Unlock()

		if errors.Is(result.Error, errBuildBudgetExceeded) && buildErr == nil {
			am.checkBuildCompletion(build)
		}
		return
	}

	// Drop stale in-flight results after build termination to prevent retry storms.
	if buildInactive {
		if task != nil && task.Status != TaskCompleted && task.Status != TaskFailed {
			task.Status = TaskCancelled
		}
		agent.UpdatedAt = time.Now()
		agent.mu.Unlock()
		return
	}

	if result.Success {
		if result.Output == nil {
			log.Printf("Warning: successful task result for agent %s has nil output", result.AgentID)
			agent.mu.Unlock()
			return
		}

		// NEW: Verify generated code before marking complete (for code generation tasks)
		if task != nil && am.isCodeGenerationTask(task.Type) && result.Output != nil {
			agent.mu.Unlock() // Release lock during verification

			// Run quick build verification on generated code
			verificationPassed, verifyErrors := am.verifyGeneratedCode(agent.BuildID, result.Output)

			agent.mu.Lock()
			if !verificationPassed {
				log.Printf("Build verification failed for task %s: %v", task.ID, verifyErrors)

				// Track verification failure for learning
				task.ErrorHistory = append(task.ErrorHistory, ErrorAttempt{
					AttemptNumber: task.RetryCount + 1,
					Error:         fmt.Sprintf("Build verification failed: %v", verifyErrors),
					Timestamp:     time.Now(),
					Context:       "build_verification",
				})

				// If we can retry, add verification errors to context and retry
				if task.RetryCount < task.MaxRetries {
					task.RetryCount++
					task.Status = TaskPending
					task.Input["verification_errors"] = verifyErrors
					task.Input["retry_guidance"] = "Previous code failed build verification. Fix the following errors:"

					agent.Status = StatusWorking
					agent.mu.Unlock()

					// Broadcast retry with verification context
					am.broadcast(agent.BuildID, &WSMessage{
						Type:      "agent:verification_failed",
						BuildID:   agent.BuildID,
						AgentID:   agent.ID,
						Timestamp: time.Now(),
						Data: map[string]any{
							"task_id":     result.TaskID,
							"errors":      verifyErrors,
							"retry_count": task.RetryCount,
							"max_retries": task.MaxRetries,
							"message":     "Build verification failed, retrying with error context...",
						},
					})

					am.taskQueue <- task
					return
				}

				// Max retries exceeded with verification failures
				result.Success = false
				result.Error = fmt.Errorf("build verification failed after %d attempts: %v", task.RetryCount, verifyErrors)
			}
		}

		// If still successful after verification
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
					"task_id":  result.TaskID,
					"success":  true,
					"output":   result.Output,
					"verified": true,
				},
			})

			// Handle task completion - may trigger next tasks
			if task != nil {
				am.handleTaskCompletion(agent.BuildID, task, result.Output)
			}
			return
		}
	}

	// Handle failure case (continued from original)
	if result.Success {
		// This block handles the case where verification changed success to failure
		agent.mu.Unlock()
		am.handleTaskFailure(agent, task, result)
		return
	} else {
		// FAILURE HANDLING - Learn from error and retry
		if task == nil {
			agent.Status = StatusError
			agent.Error = result.Error.Error()
			agent.mu.Unlock()
			return
		}

		// MaxRetries == 0 means no retries
		if task.MaxRetries < 0 {
			task.MaxRetries = 0
		}

		// Track the error for learning
		errorMsg := result.Error.Error()
		errorAttempt := ErrorAttempt{
			AttemptNumber: task.RetryCount + 1,
			Error:         errorMsg,
			Timestamp:     time.Now(),
			Context:       fmt.Sprintf("Attempt %d of %d", task.RetryCount+1, task.MaxRetries),
		}
		task.ErrorHistory = append(task.ErrorHistory, errorAttempt)
		task.RetryCount++
		insufficientCredits := isInsufficientCreditsErrorMessage(errorMsg)
		nonRetriable := am.isNonRetriableAIError(result.Error)
		retryStrategy := am.determineRetryStrategy(errorMsg, task)
		if nonRetriable {
			retryStrategy = "non_retriable"
		}

		// Collaborative incident mode: providers discuss and vote on recovery strategy.
		if !insufficientCredits && buildErr == nil && (nonRetriable || task.RetryCount >= 1 || strings.Contains(strings.ToLower(errorMsg), "all_providers_failed")) {
			decision, votes := am.runFailureConsensus(build, agent, task, result.Error, retryStrategy)
			if task.Input == nil {
				task.Input = map[string]any{}
			}
			task.Input["consensus_decision"] = string(decision)
			task.Input["consensus_votes"] = votes

			switch decision {
			case decisionSwitchProvider:
				retryStrategy = "switch_provider"
				nonRetriable = false
			case decisionRetrySame:
				if retryStrategy == "non_retriable" {
					retryStrategy = "standard_retry"
				}
				nonRetriable = false
			case decisionSpawnSolver:
				nonRetriable = true
				task.MaxRetries = task.RetryCount
			case decisionAbort:
				nonRetriable = true
				task.MaxRetries = task.RetryCount
			}
		}

		// Check if we should retry
		if task.RetryCount < task.MaxRetries && !nonRetriable {
			// Analyze error and prepare for retry
			log.Printf("Task %s failed (attempt %d/%d): %v. Retrying...",
				task.ID, task.RetryCount, task.MaxRetries, result.Error)

			// Set status back to pending for retry
			task.Status = TaskPending
			task.Error = "" // Clear error for retry
			task.RetryStrategy = RetryStrategy(retryStrategy)

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
					"retry_count":   task.RetryCount,
					"max_retries":   task.MaxRetries,
					"agent_role":    agent.Role,
					"error":         result.Error.Error(),
					"error_history": task.ErrorHistory,
					"message":       fmt.Sprintf("Learning from error, retrying (%d/%d)...", task.RetryCount, task.MaxRetries),
					"model":         agent.Model,
					"provider":      agent.Provider,
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
					"model":      agent.Model,
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
			finalMessage := "Task failed after multiple retry attempts. Consider breaking down the task or providing more guidance."
			if nonRetriable {
				finalMessage = "Task failed due to a non-retriable provider/model configuration error."
			}
			if insufficientCredits {
				finalMessage = insufficientCreditsBuildMessage
			}

			agent.Status = StatusError
			if insufficientCredits {
				agent.Error = insufficientCreditsBuildMessage
			} else {
				agent.Error = fmt.Sprintf("Failed after %d attempts: %s", task.RetryCount, errorMsg)
			}
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
					"message":       finalMessage,
				},
			})

			am.enqueueRecoveryTask(agent.BuildID, task, result.Error)
			if build, err := am.GetBuild(agent.BuildID); err == nil {
				am.updateBuildProgress(build)
				am.checkBuildCompletion(build)
			}
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
	case TaskGenerateFile, TaskGenerateUI, TaskGenerateAPI, TaskGenerateSchema, TaskArchitecture:
		// Code/architecture generated - broadcast files and update progress
		am.handleFileGeneration(build, output)
	case TaskTest:
		// Tests completed - check results
		am.handleTestCompletion(build, output)
	case TaskReview:
		// Review completed - apply fixes if needed
		am.handleReviewCompletion(build, output)
	case TaskFix:
		// Any fix task is followed by fresh tests and review before completion.
		am.schedulePostFixValidation(build, task)
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
	build.Progress = 20
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	// Spawn the full agent team
	if err := am.SpawnAgentTeam(build.ID); err != nil {
		log.Printf("Error spawning agent team: %v", err)
		build.mu.Lock()
		build.Status = BuildFailed
		build.Error = fmt.Sprintf("Failed to spawn agent team: %v", err)
		build.UpdatedAt = time.Now()
		build.mu.Unlock()
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

	// Broadcast agent team spawned with status transition
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":    "agents_spawned",
			"message":  "Agent team assembled and ready",
			"progress": 20,
			"status":   "in_progress",
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
				"content":  file.Content,
				"language": file.Language,
				"size":     file.Size,
			},
		})
	}
}

// selectFixAgent chooses an agent for fix tasks based on preferred roles
func (am *AgentManager) selectFixAgent(build *Build, preferred []AgentRole) *Agent {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	defer build.mu.RUnlock()

	for _, role := range preferred {
		for _, agent := range build.Agents {
			if agent.Role == role && agent.Status != StatusError && agent.Status != StatusTerminated {
				return agent
			}
		}
	}
	// Fallback to any available agent
	for _, agent := range build.Agents {
		if agent.Status != StatusError && agent.Status != StatusTerminated {
			return agent
		}
	}
	return nil
}

func (am *AgentManager) ensureProblemSolverAgent(buildID string) *Agent {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()
	if !exists {
		return nil
	}

	build.mu.RLock()
	for _, agent := range build.Agents {
		if agent.Role == RoleSolver {
			build.mu.RUnlock()
			return agent
		}
	}
	build.mu.RUnlock()

	availableProviders := am.getCurrentlyAvailableProvidersForBuild(build)
	if len(availableProviders) == 0 {
		log.Printf("Build %s: unable to spawn solver agent (no providers available)", buildID)
		return nil
	}

	provider := am.assignProvidersToRoles(availableProviders, []AgentRole{RoleSolver})[RoleSolver]
	solver, err := am.spawnAgent(buildID, RoleSolver, provider)
	if err != nil {
		log.Printf("Build %s: failed to spawn solver agent: %v", buildID, err)
		return nil
	}
	return solver
}

func (am *AgentManager) enqueueRecoveryTask(buildID string, failedTask *Task, err error) {
	if failedTask == nil || err == nil {
		return
	}
	if failedTask.Type == TaskFix {
		if action, ok := failedTask.Input["action"].(string); ok && action == "solve_build_failure" {
			// Avoid recursive recovery loops if the solver task itself fails.
			return
		}
	}

	build, getErr := am.GetBuild(buildID)
	if getErr != nil {
		return
	}

	failedTaskID := failedTask.ID
	failedTaskType := string(failedTask.Type)
	failedTaskDescription := failedTask.Description
	failureMessage := err.Error()

	build.mu.Lock()
	if flag, ok := failedTask.Input["recovery_queued"].(bool); ok && flag {
		build.mu.Unlock()
		return
	}
	if failedTask.Input == nil {
		failedTask.Input = map[string]any{}
	}
	failedTask.Input["recovery_queued"] = true

	recoveryTask := &Task{
		ID:          uuid.New().String(),
		Type:        TaskFix,
		Description: fmt.Sprintf("Investigate and resolve failure in task %s (%s)", failedTaskID, failedTaskType),
		Priority:    99,
		Status:      TaskPending,
		MaxRetries:  build.MaxRetries,
		Input: map[string]any{
			"action":                   "solve_build_failure",
			"failed_task_id":           failedTaskID,
			"failed_task_type":         failedTaskType,
			"failed_task_description":  failedTaskDescription,
			"failure_error":            failureMessage,
			"app_description":          build.Description,
			"retry_strategy":           "fix_and_retry",
			"requires_regression_test": true,
		},
		CreatedAt: time.Now(),
	}
	// Supersede the failed task with an explicit recovery flow so the build can
	// still converge to success if solver + validation tasks pass.
	failedTask.Status = TaskCancelled
	failedTask.Input["superseded_by_recovery"] = recoveryTask.ID

	build.Tasks = append(build.Tasks, recoveryTask)
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	solver := am.ensureProblemSolverAgent(buildID)
	if solver == nil {
		// Fallback: use existing specialists if solver could not be spawned.
		solver = am.selectFixAgent(build, []AgentRole{RoleBackend, RoleFrontend, RoleDatabase, RoleReviewer})
	}
	if solver == nil {
		log.Printf("Build %s: no solver or fallback agent available for recovery task %s", buildID, recoveryTask.ID)
		return
	}

	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":                 "auto_recovery",
			"status":                string(BuildReviewing),
			"message":               "Task failure detected. Launching problem-solving recovery agent...",
			"recovery_task":         recoveryTask.ID,
			"quality_gate_required": true,
			"quality_gate_active":   true,
			"quality_gate_stage":    "validation",
		},
	})

	if assignErr := am.AssignTask(solver.ID, recoveryTask); assignErr != nil {
		log.Printf("Build %s: failed to assign recovery task %s to %s: %v", buildID, recoveryTask.ID, solver.ID, assignErr)
	}
}

func (am *AgentManager) schedulePostFixValidation(build *Build, sourceTask *Task) {
	if build == nil || sourceTask == nil {
		return
	}

	var testAgent, reviewAgent *Agent
	build.mu.RLock()
	for _, agent := range build.Agents {
		switch agent.Role {
		case RoleTesting:
			testAgent = agent
		case RoleReviewer:
			reviewAgent = agent
		}
	}
	build.mu.RUnlock()

	newTasks := make([]*Task, 0, 2)
	now := time.Now()

	if testAgent != nil {
		newTasks = append(newTasks, &Task{
			ID:          uuid.New().String(),
			Type:        TaskTest,
			Description: "Regression test after automated fixes",
			Priority:    88,
			Status:      TaskPending,
			MaxRetries:  build.MaxRetries,
			Input: map[string]any{
				"action":             "regression_test",
				"trigger_task":       sourceTask.ID,
				"app_description":    build.Description,
				"fix_context_action": sourceTask.Input["action"],
			},
			CreatedAt: now,
		})
	}

	if reviewAgent != nil {
		newTasks = append(newTasks, &Task{
			ID:          uuid.New().String(),
			Type:        TaskReview,
			Description: "Final review after automated fixes",
			Priority:    87,
			Status:      TaskPending,
			MaxRetries:  build.MaxRetries,
			Input: map[string]any{
				"action":             "post_fix_review",
				"trigger_task":       sourceTask.ID,
				"app_description":    build.Description,
				"fix_context_action": sourceTask.Input["action"],
			},
			CreatedAt: now,
		})
	}

	if len(newTasks) == 0 {
		return
	}

	build.mu.Lock()
	build.Tasks = append(build.Tasks, newTasks...)
	build.UpdatedAt = time.Now()
	build.mu.Unlock()

	for _, task := range newTasks {
		var assignee *Agent
		if task.Type == TaskTest {
			assignee = testAgent
		} else if task.Type == TaskReview {
			assignee = reviewAgent
		}
		if assignee == nil {
			continue
		}
		if err := am.AssignTask(assignee.ID, task); err != nil {
			log.Printf("Build %s: failed to assign post-fix validation task %s: %v", build.ID, task.ID, err)
		}
	}

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":                 "validation",
			"status":                string(BuildReviewing),
			"message":               "Automated fixes applied. Running regression testing and review.",
			"quality_gate_required": true,
			"quality_gate_active":   true,
			"quality_gate_stage":    "validation",
		},
	})
}

// handleTestCompletion processes test results and creates fix tasks for failures
func (am *AgentManager) handleTestCompletion(build *Build, output *TaskOutput) {
	// Parse test output for failures
	hasFailures := false
	if output != nil {
		for _, msg := range output.Messages {
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "fail") || strings.Contains(lower, "error") ||
				strings.Contains(lower, "assertion") || strings.Contains(lower, "expected") {
				hasFailures = true
				break
			}
		}
		// Also check file content for test failure indicators
		for _, file := range output.Files {
			lower := strings.ToLower(file.Content)
			if strings.Contains(lower, "test failed") || strings.Contains(lower, "tests failing") {
				hasFailures = true
				break
			}
		}
	}

	if hasFailures {
		if !am.canCreateAutomatedFixTask(build, "fix_tests") {
			log.Printf("Test failures detected in build %s, but automated test-fix loop cap reached; proceeding to final validation", build.ID)
			am.broadcast(build.ID, &WSMessage{
				Type:      WSBuildProgress,
				BuildID:   build.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"message":               "Test failures detected, but automated fix loop cap reached. Proceeding to final validation for concrete errors.",
					"phase":                 "testing",
					"status":                string(BuildTesting),
					"quality_gate_required": true,
					"quality_gate_active":   true,
					"quality_gate_stage":    "testing",
				},
			})
			build.mu.Lock()
			build.UpdatedAt = time.Now()
			build.mu.Unlock()
			am.cancelAutomatedRecoveryTasksForLoopCap(build)
			am.checkBuildCompletion(build)
			return
		}

		log.Printf("Test failures detected in build %s, creating fix task", build.ID)

		fixTask := &Task{
			ID:          uuid.New().String(),
			Type:        TaskFix,
			Description: "Fix failing tests identified by testing agent",
			Priority:    85,
			Status:      TaskPending,
			MaxRetries:  build.MaxRetries,
			Input: map[string]any{
				"action":          "fix_tests",
				"test_output":     output.Messages,
				"app_description": build.Description,
				"previous_errors": output.Messages,
				"retry_strategy":  "fix_and_retry",
			},
			CreatedAt: time.Now(),
		}

		build.mu.Lock()
		build.Tasks = append(build.Tasks, fixTask)
		build.UpdatedAt = time.Now()
		build.mu.Unlock()

		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildProgress,
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"message":               "Test failures detected, creating fix task...",
				"phase":                 "testing",
				"status":                string(BuildTesting),
				"fix_task":              fixTask.ID,
				"quality_gate_required": true,
				"quality_gate_active":   true,
				"quality_gate_stage":    "testing",
			},
		})

		agent := am.selectFixAgent(build, []AgentRole{RoleSolver, RoleBackend, RoleFrontend, RoleDatabase, RoleReviewer})
		if agent != nil {
			if err := am.AssignTask(agent.ID, fixTask); err != nil {
				log.Printf("Failed to assign test fix task %s to agent %s: %v", fixTask.ID, agent.ID, err)
			}
		} else {
			log.Printf("No available agent to handle test fix task %s", fixTask.ID)
		}
	}

	build.mu.Lock()
	build.UpdatedAt = time.Now()
	build.mu.Unlock()
}

// handleReviewCompletion processes code review results and creates fix tasks for critical issues
func (am *AgentManager) handleReviewCompletion(build *Build, output *TaskOutput) {
	if output == nil {
		return
	}

	// Parse review output for critical issues
	hasCritical := false
	for _, msg := range output.Messages {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "critical") || strings.Contains(lower, "security vulnerability") ||
			strings.Contains(lower, "injection") || strings.Contains(lower, "xss") ||
			strings.Contains(lower, "authentication bypass") {
			hasCritical = true
			break
		}
	}

	if hasCritical {
		build.mu.RLock()
		readinessRecoveryAttempts := build.ReadinessRecoveryAttempts
		existingBuildError := strings.ToLower(strings.TrimSpace(build.Error))
		build.mu.RUnlock()

		if readinessRecoveryAttempts > 0 && strings.Contains(existingBuildError, "final output validation failed") {
			log.Printf("Critical review issues found in build %s during readiness recovery; skipping review-fix loop and proceeding to final validation", build.ID)
			am.broadcast(build.ID, &WSMessage{
				Type:      WSBuildProgress,
				BuildID:   build.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"message":               "Readiness recovery is already active. Skipping additional review-fix loops and finalizing with current validation errors if they persist.",
					"phase":                 "reviewing",
					"status":                string(BuildReviewing),
					"quality_gate_required": true,
					"quality_gate_active":   true,
					"quality_gate_stage":    "review",
				},
			})
			build.mu.Lock()
			build.UpdatedAt = time.Now()
			build.mu.Unlock()
			am.cancelAutomatedRecoveryTasksForLoopCap(build)
			am.checkBuildCompletion(build)
			return
		}

		if !am.canCreateAutomatedFixTask(build, "fix_review_issues") {
			log.Printf("Critical review issues found in build %s, but automated review-fix loop cap reached; proceeding to final validation", build.ID)
			am.broadcast(build.ID, &WSMessage{
				Type:      WSBuildProgress,
				BuildID:   build.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"message":               "Critical issues found in review, but automated fix loop cap reached. Proceeding to final validation for concrete errors.",
					"phase":                 "reviewing",
					"status":                string(BuildReviewing),
					"quality_gate_required": true,
					"quality_gate_active":   true,
					"quality_gate_stage":    "review",
				},
			})
			build.mu.Lock()
			build.UpdatedAt = time.Now()
			build.mu.Unlock()
			am.cancelAutomatedRecoveryTasksForLoopCap(build)
			am.checkBuildCompletion(build)
			return
		}

		log.Printf("Critical review issues found in build %s, creating fix task", build.ID)

		fixTask := &Task{
			ID:          uuid.New().String(),
			Type:        TaskFix,
			Description: "Fix critical issues found during code review",
			Priority:    95,
			Status:      TaskPending,
			MaxRetries:  build.MaxRetries,
			Input: map[string]any{
				"action":          "fix_review_issues",
				"review_findings": output.Messages,
				"app_description": build.Description,
				"previous_errors": output.Messages,
				"retry_strategy":  "fix_and_retry",
			},
			CreatedAt: time.Now(),
		}

		build.mu.Lock()
		build.Tasks = append(build.Tasks, fixTask)
		build.UpdatedAt = time.Now()
		build.mu.Unlock()

		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildProgress,
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"message":               "Critical issues found in review, creating fix task...",
				"phase":                 "reviewing",
				"status":                string(BuildReviewing),
				"fix_task":              fixTask.ID,
				"quality_gate_required": true,
				"quality_gate_active":   true,
				"quality_gate_stage":    "review",
			},
		})

		agent := am.selectFixAgent(build, []AgentRole{RoleSolver, RoleBackend, RoleFrontend, RoleDatabase, RoleReviewer})
		if agent != nil {
			if err := am.AssignTask(agent.ID, fixTask); err != nil {
				log.Printf("Failed to assign review fix task %s to agent %s: %v", fixTask.ID, agent.ID, err)
			}
		} else {
			log.Printf("No available agent to handle review fix task %s", fixTask.ID)
		}
	}

	build.mu.Lock()
	build.UpdatedAt = time.Now()
	build.mu.Unlock()
}

// cancelPendingTasks marks all pending tasks as cancelled to stop further work
func (am *AgentManager) cancelPendingTasks(build *Build) {
	build.mu.Lock()
	defer build.mu.Unlock()

	for _, task := range build.Tasks {
		if task.Status == TaskPending {
			task.Status = TaskCancelled
		}
	}
}

func (am *AgentManager) canCreateAutomatedFixTask(build *Build, action string) bool {
	if build == nil || strings.TrimSpace(action) == "" {
		return true
	}

	if am.hasRecentOrActiveAutomatedFixTask(build, action) {
		return false
	}

	limit := am.maxAutomatedFixLoops(build, action)
	if limit <= 0 {
		return true
	}

	count := 0
	build.mu.RLock()
	for _, task := range build.Tasks {
		if task == nil || task.Type != TaskFix {
			continue
		}
		taskAction, _ := task.Input["action"].(string)
		if taskAction == action {
			count++
		}
	}
	build.mu.RUnlock()
	return count < limit
}

func (am *AgentManager) hasRecentOrActiveAutomatedFixTask(build *Build, action string) bool {
	if build == nil || strings.TrimSpace(action) == "" {
		return false
	}

	now := time.Now()
	const duplicateCooldown = 20 * time.Second

	build.mu.RLock()
	defer build.mu.RUnlock()

	for _, task := range build.Tasks {
		if task == nil || task.Type != TaskFix {
			continue
		}
		taskAction, _ := task.Input["action"].(string)
		if taskAction != action {
			continue
		}

		if task.Status == TaskPending || task.Status == TaskInProgress {
			return true
		}
		if !task.CreatedAt.IsZero() && now.Sub(task.CreatedAt) <= duplicateCooldown {
			return true
		}
	}
	return false
}

func (am *AgentManager) maxAutomatedFixLoops(build *Build, action string) int {
	defaultLimit := 3
	envKey := "BUILD_MAX_FIX_LOOPS"
	if action == "fix_review_issues" {
		envKey = "BUILD_MAX_REVIEW_FIX_LOOPS"
	} else if action == "fix_tests" {
		envKey = "BUILD_MAX_TEST_FIX_LOOPS"
	}

	limit := envInt(envKey, defaultLimit)
	if am.isLocalDevStrictPreviewBuild(build) && limit < 2 {
		limit = 2
	}
	if limit < 0 {
		limit = 0
	}
	return limit
}

func summarizeReadinessErrorClass(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	joined := strings.ToLower(strings.Join(errors, "; "))
	switch {
	case strings.Contains(joined, "dependency check failed"):
		return "dependency_check"
	case strings.Contains(joined, "missing a build script"):
		return "missing_build_script"
	case strings.Contains(joined, "tsconfig.json is missing"):
		return "missing_tsconfig"
	case strings.Contains(joined, "preview verification build failed"):
		return "preview_build_failed"
	case strings.Contains(joined, "backend verification build failed"):
		return "backend_build_failed"
	case strings.Contains(joined, "unresolved patch/merge markers"):
		return "unresolved_patch_markers"
	case strings.Contains(joined, "missing an html entry point"):
		return "missing_html_entry"
	case strings.Contains(joined, "missing an entry source file"):
		return "missing_frontend_entry"
	default:
		// Fall back to the first readiness error prefix for stable grouping.
		first := strings.TrimSpace(strings.ToLower(errors[0]))
		if idx := strings.Index(first, ":"); idx > 0 {
			first = strings.TrimSpace(first[:idx])
		}
		if len(first) > 120 {
			first = first[:120]
		}
		return first
	}
}

func readinessErrorClassFromBuildError(buildErr string) string {
	s := strings.TrimSpace(strings.ToLower(buildErr))
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(s, "final output validation failed after automated recovery: ")
	s = strings.TrimPrefix(s, "final output validation failed: ")
	return summarizeReadinessErrorClass([]string{s})
}

func extractDependencyRepairHintsFromReadinessErrors(errors []string) []string {
	if len(errors) == 0 {
		return nil
	}
	pkgSet := map[string]bool{}
	specSet := map[string]bool{}
	for _, msg := range errors {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		if m := regexp.MustCompile(`does not declare dependency "([^"]+)"`).FindStringSubmatch(msg); len(m) == 2 {
			pkgSet[m[1]] = true
		}
		if m := regexp.MustCompile(`source imports "([^"]+)"`).FindStringSubmatch(msg); len(m) == 2 {
			specSet[m[1]] = true
		}
	}
	if len(pkgSet) == 0 && len(specSet) == 0 {
		return nil
	}

	pkgs := make([]string, 0, len(pkgSet))
	for pkg := range pkgSet {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	specs := make([]string, 0, len(specSet))
	for spec := range specSet {
		specs = append(specs, spec)
	}
	sort.Strings(specs)

	hints := make([]string, 0, 3)
	if len(pkgs) > 0 {
		hints = append(hints, fmt.Sprintf("Update package.json dependencies/devDependencies to include missing package(s): %s", strings.Join(pkgs, ", ")))
		placementHints := make([]string, 0, len(pkgs))
		for _, pkg := range pkgs {
			if hint := dependencyRepairPlacementHint(pkg); hint != "" {
				placementHints = append(placementHints, hint)
			}
		}
		if len(placementHints) > 0 {
			hints = append(hints, "Package placement guidance: "+strings.Join(placementHints, "; "))
		}
	}
	if len(specs) > 0 {
		hints = append(hints, fmt.Sprintf("Preserve and satisfy imports used by source files (do not remove features just to silence errors): %s", strings.Join(specs, ", ")))
	}
	for _, spec := range specs {
		if spec == "@vitejs/plugin-react" || spec == "@vitejs/plugin-react-swc" {
			hints = append(hints, "If vite.config imports @vitejs/plugin-react* then add it to devDependencies and keep vite.config.ts in ESM syntax (import/export, not require).")
			break
		}
	}
	hints = append(hints, "If a config-only import (jest/vitest config) is not needed for runtime/preview, keep config self-consistent or remove the unused config file and related scripts together.")
	return hints
}

func dependencyRepairPlacementHint(pkg string) string {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return ""
	}
	lower := strings.ToLower(pkg)
	switch {
	case strings.HasPrefix(lower, "@types/"):
		return fmt.Sprintf("%s -> devDependencies", pkg)
	case lower == "typescript" || lower == "vite" || strings.HasPrefix(lower, "@vitejs/"):
		return fmt.Sprintf("%s -> devDependencies", pkg)
	case lower == "vitest" || lower == "jest" || strings.HasPrefix(lower, "@testing-library/"):
		return fmt.Sprintf("%s -> devDependencies (unless tests are intentionally removed)", pkg)
	default:
		return fmt.Sprintf("%s -> dependencies", pkg)
	}
}

func (am *AgentManager) cancelAutomatedRecoveryTasksForLoopCap(build *Build) {
	if build == nil {
		return
	}

	isRecoveryAction := func(action string) bool {
		switch strings.TrimSpace(action) {
		case "fix_review_issues", "fix_tests", "regression_test", "post_fix_review":
			return true
		default:
			return false
		}
	}

	build.mu.Lock()
	defer build.mu.Unlock()

	cancelled := 0
	for _, task := range build.Tasks {
		if task == nil {
			continue
		}
		if task.Status != TaskPending && task.Status != TaskInProgress {
			continue
		}
		action, _ := task.Input["action"].(string)
		if !isRecoveryAction(action) {
			continue
		}
		task.Status = TaskCancelled
		cancelled++
	}
	if cancelled > 0 {
		build.UpdatedAt = time.Now()
		log.Printf("Build %s: cancelled %d automated recovery task(s) after loop cap", build.ID, cancelled)
	}
}

// updateBuildProgress calculates and updates overall build progress
func (am *AgentManager) updateBuildProgress(build *Build) {
	build.mu.Lock()
	if len(build.Tasks) == 0 {
		build.mu.Unlock()
		return
	}

	completed := 0
	for _, task := range build.Tasks {
		if task.Status == TaskCompleted {
			completed++
		}
	}

	// Scale task progress into the 20-100 range (20% is the planning baseline)
	taskProgress := (completed * 80) / len(build.Tasks)
	progress := 20 + taskProgress

	// Also track progress by worker-agent completion so long-running tasks don't look stuck.
	workerTotal := 0
	workerDone := 0
	for _, agent := range build.Agents {
		if agent.Role == RoleLead {
			continue
		}
		workerTotal++
		if agent.Status == StatusCompleted || agent.Status == StatusError {
			workerDone++
		}
	}
	if workerTotal > 0 {
		agentProgress := 20 + (workerDone*70)/workerTotal
		if agentProgress > progress {
			progress = agentProgress
		}
	}

	if build.Status != BuildCompleted && progress > 99 {
		progress = 99
	}

	build.Progress = progress
	build.UpdatedAt = time.Now()
	status := build.Status
	qualityStage := ""
	qualityActive := false
	switch status {
	case BuildTesting:
		qualityStage = "testing"
		qualityActive = true
	case BuildReviewing:
		qualityStage = "review"
		qualityActive = true
	}

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"progress":              progress,
			"status":                string(status),
			"tasks_completed":       completed,
			"tasks_total":           len(build.Tasks),
			"quality_gate_required": true,
			"quality_gate_active":   qualityActive,
			"quality_gate_stage":    qualityStage,
		},
	})
	build.mu.Unlock()

	// Persist rolling progress so recent builds can be resumed after restart/login.
	am.persistBuildSnapshot(build, nil)
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

	build.mu.RLock()
	phasedPipelineComplete := build.PhasedPipelineComplete
	readinessRecoveryAttempts := build.ReadinessRecoveryAttempts
	currentStatus := build.Status
	build.mu.RUnlock()
	if !phasedPipelineComplete && readinessRecoveryAttempts == 0 &&
		currentStatus != BuildTesting && currentStatus != BuildReviewing {
		// Avoid transient "all tasks complete" windows between phased task batches
		// (e.g. phase N finished, phase N+1 not yet queued) from triggering final validation.
		return
	}

	build.mu.Lock()
	now := time.Now()
	build.UpdatedAt = now

	status := build.Status
	if status != BuildFailed && status != BuildCancelled {
		if anyFailed {
			status = BuildFailed
			build.Status = status
			build.CompletedAt = &now
		}
	}
	progress := build.Progress
	buildError := strings.TrimSpace(build.Error)
	lastTaskError := ""
	if buildError == "" && anyFailed {
		for i := len(build.Tasks) - 1; i >= 0; i-- {
			task := build.Tasks[i]
			if task == nil || task.Status != TaskFailed {
				continue
			}
			if msg := strings.TrimSpace(task.Error); msg != "" {
				lastTaskError = msg
				break
			}
			for j := len(task.ErrorHistory) - 1; j >= 0; j-- {
				if msg := strings.TrimSpace(task.ErrorHistory[j].Error); msg != "" {
					lastTaskError = msg
					break
				}
			}
			if lastTaskError != "" {
				break
			}
		}
	}
	build.mu.Unlock()

	allFiles := am.collectGeneratedFiles(build)

	if status != BuildFailed && status != BuildCancelled {
		readinessErrors := am.validateFinalBuildReadiness(build, allFiles)
		if len(readinessErrors) > 0 {
			errorSummary := strings.Join(readinessErrors, "; ")
			currentErrorClass := summarizeReadinessErrorClass(readinessErrors)
			now = time.Now()

			build.mu.Lock()
			priorErrorClass := readinessErrorClassFromBuildError(build.Error)
			repeatedClass := build.ReadinessRecoveryAttempts > 0 && priorErrorClass != "" && priorErrorClass == currentErrorClass
			if repeatedClass {
				build.Status = BuildFailed
				build.CompletedAt = &now
				build.UpdatedAt = now
				if build.Progress > 99 {
					build.Progress = 99
				}
				buildError = fmt.Sprintf("Final output validation failed after repeated recovery (%s): %s", currentErrorClass, errorSummary)
				build.Error = buildError
				status = build.Status
				progress = build.Progress
				build.mu.Unlock()
				am.cancelAutomatedRecoveryTasksForLoopCap(build)
				goto completion_finalize
			}

			if build.ReadinessRecoveryAttempts < 1 {
				build.ReadinessRecoveryAttempts++
				build.Status = BuildReviewing
				build.CompletedAt = nil
				build.UpdatedAt = now
				build.Progress = 95
				build.Error = fmt.Sprintf("Final output validation failed: %s", errorSummary)
				progress = build.Progress
				build.mu.Unlock()

				am.broadcast(build.ID, &WSMessage{
					Type:      WSBuildProgress,
					BuildID:   build.ID,
					Timestamp: now,
					Data: map[string]any{
						"phase":                 "validation",
						"status":                string(BuildReviewing),
						"progress":              progress,
						"message":               "Final validation detected non-runnable output. Launching solver recovery pass.",
						"quality_gate_required": true,
						"quality_gate_active":   true,
						"quality_gate_stage":    "validation",
						"validation_errors":     readinessErrors,
					},
				})

				failedTask := &Task{
					ID:          "final_output_validation",
					Type:        TaskReview,
					Description: "Final output validation",
					Status:      TaskFailed,
					Input: map[string]any{
						"validation_errors": readinessErrors,
					},
				}
				if hints := extractDependencyRepairHintsFromReadinessErrors(readinessErrors); len(hints) > 0 {
					failedTask.Input["repair_hints"] = hints
				}
				am.enqueueRecoveryTask(build.ID, failedTask, fmt.Errorf("final output validation failed: %s", errorSummary))
				return
			}

			build.Status = BuildFailed
			build.CompletedAt = &now
			build.UpdatedAt = now
			if build.Progress > 99 {
				build.Progress = 99
			}
			buildError = fmt.Sprintf("Final output validation failed after automated recovery: %s", errorSummary)
			build.Error = buildError
			status = build.Status
			progress = build.Progress
			build.mu.Unlock()
		} else {
			build.mu.Lock()
			if build.Status != BuildFailed && build.Status != BuildCancelled {
				build.Status = BuildCompleted
				build.Progress = 100
				build.CompletedAt = &now
				status = build.Status
				progress = build.Progress
			} else {
				status = build.Status
				progress = build.Progress
			}
			build.mu.Unlock()
		}
	}

completion_finalize:

	if status == BuildCompleted {
		// Persist completion first so a completed_build row exists before auto-linking a project.
		am.persistCompletedBuild(build, allFiles)
		if err := am.ensureProjectLinkedForCompletedBuild(build, allFiles); err != nil {
			log.Printf("Failed to auto-link project for build %s: %v", build.ID, err)
		}

		am.createCheckpoint(build, "Build Complete", "All tasks completed successfully")
		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildCompleted,
			BuildID:   build.ID,
			Timestamp: now,
			Data: map[string]any{
				"status":                string(status),
				"progress":              progress,
				"files_count":           len(allFiles),
				"files":                 allFiles,
				"quality_gate_required": true,
				"quality_gate_passed":   true,
				"quality_gate_stage":    "complete",
			},
		})
		log.Printf("Build %s completed successfully (%d files)", build.ID, len(allFiles))
	} else {
		checkpointName := "Build Failed"
		checkpointDescription := "Build finished with errors"
		errorTitle := "Build failed"

		if status == BuildCancelled {
			checkpointName = "Build Cancelled"
			checkpointDescription = "Build was cancelled"
			errorTitle = "Build cancelled"
		}
		if buildError == "" {
			if status == BuildCancelled {
				buildError = "Build was cancelled before completion."
			} else if lastTaskError != "" {
				buildError = lastTaskError
			} else {
				buildError = "One or more tasks failed before build completion."
			}
		}
		build.mu.Lock()
		if strings.TrimSpace(build.Error) == "" {
			build.Error = buildError
			build.UpdatedAt = time.Now()
		}
		build.mu.Unlock()

		am.createCheckpoint(build, checkpointName, checkpointDescription)
		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   build.ID,
			Timestamp: now,
			Data: map[string]any{
				"status":                string(status),
				"error":                 errorTitle,
				"details":               buildError,
				"recoverable":           false,
				"progress":              progress,
				"files_count":           len(allFiles),
				"files":                 allFiles,
				"quality_gate_required": true,
				"quality_gate_passed":   false,
				"quality_gate_stage":    "validation",
			},
		})
		log.Printf("Build %s finished with status %s (%d files)", build.ID, status, len(allFiles))
	}

	// Persist to database
	am.persistCompletedBuild(build, allFiles)
}

// persistBuildSnapshot upserts the latest build state to the database.
// This runs for both in-progress and completed builds so users can recover state after restarts.
func (am *AgentManager) persistBuildSnapshot(build *Build, files []GeneratedFile) {
	if am.db == nil {
		return
	}
	if build == nil {
		return
	}

	build.mu.RLock()
	techStackJSON := ""
	if build.TechStack != nil {
		if b, err := json.Marshal(build.TechStack); err == nil {
			techStackJSON = string(b)
		}
	}

	if files == nil {
		files = am.collectGeneratedFiles(build)
	}

	filesJSON := "[]"
	if len(files) > 0 {
		if b, err := json.Marshal(files); err == nil {
			filesJSON = string(b)
		}
	}

	var durationMs int64
	if build.CompletedAt != nil {
		durationMs = build.CompletedAt.Sub(build.CreatedAt).Milliseconds()
	}

	projectName := ""
	if build.Plan != nil {
		projectName = build.Plan.AppType
	}

	snapshot := &models.CompletedBuild{
		BuildID:     build.ID,
		UserID:      build.UserID,
		ProjectID:   build.ProjectID,
		ProjectName: projectName,
		Description: build.Description,
		Status:      string(build.Status),
		Mode:        string(build.Mode),
		PowerMode:   string(build.PowerMode),
		TechStack:   techStackJSON,
		FilesJSON:   filesJSON,
		FilesCount:  len(files),
		Progress:    build.Progress,
		DurationMs:  durationMs,
		Error:       build.Error,
		CompletedAt: build.CompletedAt,
	}
	build.mu.RUnlock()

	now := time.Now()
	snapshot.UpdatedAt = now
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = build.CreatedAt
	}

	err := am.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "build_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"user_id":      snapshot.UserID,
			"project_id":   snapshot.ProjectID,
			"project_name": snapshot.ProjectName,
			"description":  snapshot.Description,
			"status":       snapshot.Status,
			"mode":         snapshot.Mode,
			"power_mode":   snapshot.PowerMode,
			"tech_stack":   snapshot.TechStack,
			"files_json":   snapshot.FilesJSON,
			"files_count":  snapshot.FilesCount,
			"total_cost":   snapshot.TotalCost,
			"progress":     snapshot.Progress,
			"duration_ms":  snapshot.DurationMs,
			"error":        snapshot.Error,
			"completed_at": snapshot.CompletedAt,
			"updated_at":   now,
		}),
	}).Create(snapshot).Error

	if err != nil {
		log.Printf("Failed to persist build snapshot %s: %v", build.ID, err)
	}
}

// persistCompletedBuild remains as a compatibility alias used by orchestrator paths.
func (am *AgentManager) persistCompletedBuild(build *Build, files []GeneratedFile) {
	am.persistBuildSnapshot(build, files)
}

// ensureProjectLinkedForCompletedBuild creates (or reuses) a project for a completed build
// and stores the project_id on both the in-memory build and completed_build snapshot.
func (am *AgentManager) ensureProjectLinkedForCompletedBuild(build *Build, files []GeneratedFile) error {
	if am.db == nil || build == nil {
		return nil
	}
	if files == nil {
		files = am.collectGeneratedFiles(build)
	}
	if len(files) == 0 {
		return nil
	}

	build.mu.RLock()
	if build.ProjectID != nil && *build.ProjectID != 0 {
		build.mu.RUnlock()
		return nil
	}
	buildID := build.ID
	userID := build.UserID
	description := strings.TrimSpace(build.Description)
	projectName := "Generated App"
	if build.Plan != nil && strings.TrimSpace(build.Plan.AppType) != "" {
		projectName = strings.TrimSpace(build.Plan.AppType)
	} else if description != "" {
		projectName = description
	}
	if len(projectName) > 100 {
		projectName = strings.TrimSpace(projectName[:100])
	}
	frameworkHint := ""
	if build.TechStack != nil {
		frameworkHint = strings.ToLower(strings.TrimSpace(build.TechStack.Frontend))
	}
	buildCreatedAt := build.CreatedAt
	build.mu.RUnlock()

	if projectName == "" {
		projectName = "Generated App"
	}

	language := detectGeneratedProjectLanguage(files)
	framework := detectGeneratedProjectFramework(frameworkHint)

	return am.db.Transaction(func(tx *gorm.DB) error {
		// Idempotency: if the completed build already has a linked project, reuse it.
		var existing models.CompletedBuild
		if err := tx.Select("project_id").Where("build_id = ?", buildID).First(&existing).Error; err == nil {
			if existing.ProjectID != nil && *existing.ProjectID != 0 {
				projectID := *existing.ProjectID
				build.mu.Lock()
				build.ProjectID = &projectID
				build.UpdatedAt = time.Now()
				build.mu.Unlock()
				return nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		project := models.Project{
			Name:          projectName,
			Description:   description,
			Language:      language,
			Framework:     framework,
			OwnerID:       userID,
			IsPublic:      false,
			RootDirectory: "/",
			Environment:   map[string]interface{}{},
			Dependencies:  map[string]interface{}{},
			BuildConfig: map[string]interface{}{
				"source":          "build_completion",
				"source_build_id": buildID,
				"auto_linked":     true,
			},
		}

		if err := tx.Create(&project).Error; err != nil {
			return err
		}

		projectFiles := buildProjectFilesFromGenerated(project.ID, userID, files)
		if len(projectFiles) > 0 {
			if err := tx.Create(&projectFiles).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&models.CompletedBuild{}).
			Where("build_id = ?", buildID).
			Update("project_id", project.ID).Error; err != nil {
			return err
		}

		projectID := project.ID
		build.mu.Lock()
		build.ProjectID = &projectID
		if build.UpdatedAt.Before(buildCreatedAt) {
			build.UpdatedAt = time.Now()
		}
		build.mu.Unlock()

		return nil
	})
}

func detectGeneratedProjectLanguage(files []GeneratedFile) string {
	hasTS := false
	hasJS := false
	hasPy := false
	hasGo := false
	hasRust := false
	hasHTML := false
	hasCSS := false

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Path))
		switch ext {
		case ".tsx", ".ts":
			hasTS = true
		case ".jsx", ".js", ".mjs", ".cjs":
			hasJS = true
		case ".py":
			hasPy = true
		case ".go":
			hasGo = true
		case ".rs":
			hasRust = true
		case ".html":
			hasHTML = true
		case ".css":
			hasCSS = true
		}
	}

	switch {
	case hasTS:
		return "typescript"
	case hasJS:
		return "javascript"
	case hasPy:
		return "python"
	case hasGo:
		return "go"
	case hasRust:
		return "rust"
	case hasHTML:
		return "html"
	case hasCSS:
		return "css"
	default:
		return "javascript"
	}
}

func detectGeneratedProjectFramework(frontendHint string) string {
	switch {
	case strings.Contains(frontendHint, "next"):
		return "next"
	case strings.Contains(frontendHint, "react"):
		return "react"
	case strings.Contains(frontendHint, "vue"):
		return "vue"
	case strings.Contains(frontendHint, "svelte"):
		return "svelte"
	case strings.Contains(frontendHint, "angular"):
		return "angular"
	default:
		return ""
	}
}

func buildProjectFilesFromGenerated(projectID, userID uint, files []GeneratedFile) []models.File {
	projectFiles := make([]models.File, 0, len(files))
	seen := make(map[string]struct{}, len(files))

	for _, generated := range files {
		path := sanitizeFilePath(generated.Path)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}

		name := filepath.Base(path)
		if name == "." || name == "/" || name == "" {
			continue
		}

		mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		projectFiles = append(projectFiles, models.File{
			ProjectID:  projectID,
			Name:       name,
			Path:       path,
			Type:       "file",
			MimeType:   mimeType,
			Content:    generated.Content,
			Size:       int64(len(generated.Content)),
			LastEditBy: userID,
		})
	}

	return projectFiles
}

// createCheckpoint saves a checkpoint of the current build state
func (am *AgentManager) createCheckpoint(build *Build, name, description string) *Checkpoint {
	build.mu.Lock()
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
	build.mu.Unlock()

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildCheckpoint,
		BuildID:   build.ID,
		Timestamp: checkpoint.CreatedAt,
		Data: map[string]any{
			"checkpoint_id": checkpoint.ID,
			"number":        checkpoint.Number,
			"name":          name,
			"description":   description,
			"progress":      checkpoint.Progress,
		},
	})
	am.persistBuildSnapshot(build, nil)

	return checkpoint
}

// collectGeneratedFiles gathers all generated files from completed tasks
func (am *AgentManager) collectGeneratedFiles(build *Build) []GeneratedFile {
	filesByPath := make(map[string]GeneratedFile)
	orderedPaths := make([]string, 0)

	for _, task := range build.Tasks {
		if task.Output == nil || !am.isCodeGenerationTask(task.Type) {
			continue
		}
		for _, file := range task.Output.Files {
			if strings.TrimSpace(file.Path) == "" || strings.TrimSpace(file.Content) == "" {
				continue
			}
			sanitized := sanitizeFilePath(file.Path)
			if sanitized == "" {
				continue
			}
			file.Path = sanitized
			file.Content = normalizeGeneratedFileContent(file.Path, file.Content)

			existing, exists := filesByPath[sanitized]
			if !exists {
				filesByPath[sanitized] = file
				orderedPaths = append(orderedPaths, sanitized)
				continue
			}

			// Prefer fuller file content when the same path appears multiple times.
			if len(strings.TrimSpace(file.Content)) > len(strings.TrimSpace(existing.Content)) {
				filesByPath[sanitized] = file
			}
		}
	}

	if len(orderedPaths) == 0 {
		return []GeneratedFile{}
	}

	hasRealFiles := false
	for _, path := range orderedPaths {
		if !isGeneratedArtifactPath(path) {
			hasRealFiles = true
			break
		}
	}

	result := make([]GeneratedFile, 0, len(orderedPaths))
	for _, path := range orderedPaths {
		// Drop parser fallback artifacts when real project files exist.
		if hasRealFiles && isGeneratedArtifactPath(path) {
			continue
		}
		result = append(result, filesByPath[path])
	}
	return result
}

func normalizeGeneratedFileContent(path, content string) string {
	if strings.TrimSpace(content) == "" || !strings.Contains(content, "''") {
		content = normalizeGeneratedTypeScriptInteropPatterns(path, content)
		return normalizeGeneratedTSConfigBuildExcludes(path, content)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".json", ".css", ".scss", ".html", ".md", ".yml", ".yaml":
	default:
		return content
	}

	// Heuristic repair for a common model corruption pattern where single quotes are doubled
	// throughout code/config files (e.g. import { defineConfig } from ''vite'').
	// Only apply when multiple code-like indicators are present to avoid changing intentional SQL escaping.
	indicators := []string{
		"from ''", "import ''", "require(''", "''./", "''../", "''@",
		" = ''", ": ''", "'';", "''}", "''react", "''vite", "''/api",
	}
	hits := 0
	for _, needle := range indicators {
		if strings.Contains(content, needle) {
			hits++
		}
	}
	if hits < 2 {
		content = normalizeGeneratedTypeScriptInteropPatterns(path, content)
		return normalizeGeneratedTSConfigBuildExcludes(path, content)
	}

	content = strings.ReplaceAll(content, "''", "'")
	content = normalizeGeneratedTypeScriptInteropPatterns(path, content)
	return normalizeGeneratedTSConfigBuildExcludes(path, content)
}

func normalizeGeneratedTypeScriptInteropPatterns(path, content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	normalizedPath := strings.ToLower(strings.TrimSpace(path))
	// Common `pg` typing mismatch emitted by models:
	// `import pg, { QueryResultRow } from 'pg'` combined with `pg.QueryResult<T>`
	// can fail under some tsconfig/module-resolution combos.
	if strings.HasSuffix(normalizedPath, "config/database.ts") &&
		strings.Contains(content, "import pg, { QueryResultRow } from 'pg';") &&
		strings.Contains(content, "Promise<pg.QueryResult<T>>") &&
		strings.Contains(content, "pool.query<T>(") {
		content = strings.Replace(content, "import pg, { QueryResultRow } from 'pg';", "import pg from 'pg';", 1)
		content = strings.Replace(content,
			"export async function query<T extends QueryResultRow>(text: string, params?: any[]): Promise<pg.QueryResult<T>> {",
			"export async function query<T extends pg.QueryResultRow = pg.QueryResultRow>(text: string, params?: any[]): Promise<pg.QueryResult<T>> {",
			1,
		)
	}

	return content
}

func normalizeGeneratedTSConfigBuildExcludes(path, content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	normalizedPath := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if filepath.Base(normalizedPath) != "tsconfig.json" {
		return content
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return content
	}

	compilerOptions, _ := cfg["compilerOptions"].(map[string]any)
	changed := false

	// Frontend TS/Vite apps frequently fail with TS2792 on package imports when the model
	// emits `module: "ESNext"` but omits `moduleResolution`.
	if compilerOptions != nil {
		_, hasModuleResolution := compilerOptions["moduleResolution"]
		moduleValue, _ := compilerOptions["module"].(string)
		jsxValue, _ := compilerOptions["jsx"].(string)
		if !hasModuleResolution &&
			strings.EqualFold(strings.TrimSpace(moduleValue), "ESNext") &&
			strings.TrimSpace(jsxValue) != "" {
			compilerOptions["moduleResolution"] = "Node"
			cfg["compilerOptions"] = compilerOptions
			changed = true
		}
	}

	isBackendTSConfig := strings.Contains(normalizedPath, "backend/") || strings.Contains(normalizedPath, "/api/") ||
		strings.HasPrefix(normalizedPath, "backend/") || strings.HasPrefix(normalizedPath, "api/")
	if !isBackendTSConfig {
		if !changed {
			return content
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return content
		}
		return string(b)
	}

	include, _ := cfg["include"].([]any)
	hasSrcInclude := false
	for _, v := range include {
		s, _ := v.(string)
		s = strings.ToLower(strings.TrimSpace(s))
		if strings.Contains(s, "src") {
			hasSrcInclude = true
			break
		}
	}
	if !hasSrcInclude {
		if !changed {
			return content
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return content
		}
		return string(b)
	}

	exclude, _ := cfg["exclude"].([]any)
	existing := make(map[string]bool, len(exclude))
	for _, v := range exclude {
		if s, ok := v.(string); ok {
			existing[s] = true
		}
	}

	for _, pattern := range []string{
		"src/**/__tests__/**",
		"src/**/*.test.ts",
		"src/**/*.test.tsx",
		"src/**/*.spec.ts",
		"src/**/*.spec.tsx",
	} {
		if existing[pattern] {
			continue
		}
		exclude = append(exclude, pattern)
		changed = true
	}
	if !changed {
		return content
	}
	cfg["exclude"] = exclude

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return content
	}
	return string(b)
}

// agentPriority pairs an agent with its execution priority
type agentPriority struct {
	agent    *Agent
	priority int
}

// queuePlanTasks creates and queues tasks in phased order so upstream outputs
// (architecture, schema) are available as context for downstream agents.
func (am *AgentManager) queuePlanTasks(build *Build) {
	log.Printf("queuePlanTasks called for build %s", build.ID)

	build.mu.RLock()
	agents := make(map[string]*Agent)
	for k, v := range build.Agents {
		agents[k] = v
	}
	description := build.Description
	build.mu.RUnlock()

	// Collect non-lead agents
	allAgents := make([]agentPriority, 0)
	for _, agent := range agents {
		// Planner and solver are on-demand specialists, not part of the default phased pipeline.
		if agent.Role == RoleLead || agent.Role == RolePlanner || agent.Role == RoleSolver {
			continue
		}
		allAgents = append(allAgents, agentPriority{
			agent:    agent,
			priority: am.getPriorityForRole(agent.Role),
		})
	}

	// Group agents by execution phase:
	//   Phase 1: Architect (produces architecture plan for all downstream agents)
	//   Phase 2: Database  (produces schema for backend)
	//   Phase 3: Backend + Frontend (parallel, both get architecture context; backend also gets schema)
	//   Phase 4: Testing   (needs all generated files)
	//   Phase 5: Review    (runs after tests for final quality gate)
	var archAgents, dbAgents, codeAgents, testAgents, reviewAgents []agentPriority
	for _, ap := range allAgents {
		switch ap.agent.Role {
		case RoleArchitect:
			archAgents = append(archAgents, ap)
		case RoleDatabase:
			dbAgents = append(dbAgents, ap)
		case RoleTesting:
			testAgents = append(testAgents, ap)
		case RoleReviewer:
			reviewAgents = append(reviewAgents, ap)
		default:
			codeAgents = append(codeAgents, ap)
		}
	}

	// Execute phases in order in a goroutine (non-blocking)
	go am.executePhasedTasks(build, description, archAgents, dbAgents, codeAgents, testAgents, reviewAgents)

	log.Printf("Started phased task execution for build %s (%d agents)", build.ID, len(allAgents))
}

// executePhasedTasks runs agent tasks in sequential phases, waiting for each
// phase to complete before starting the next. This ensures context flows properly.
func (am *AgentManager) executePhasedTasks(build *Build, description string,
	archAgents, dbAgents, codeAgents, testAgents, reviewAgents []agentPriority) {

	phases := []struct {
		name         string
		key          string
		status       BuildStatus
		qualityStage string
		agents       []agentPriority
	}{
		{name: "Architecture", key: "architecture", status: BuildInProgress, qualityStage: "", agents: archAgents},
		{name: "Database Schema", key: "database", status: BuildInProgress, qualityStage: "", agents: dbAgents},
		{name: "Code Generation", key: "code_generation", status: BuildInProgress, qualityStage: "", agents: codeAgents},
		{name: "Testing", key: "testing", status: BuildTesting, qualityStage: "testing", agents: testAgents},
		{name: "Review", key: "review", status: BuildReviewing, qualityStage: "review", agents: reviewAgents},
	}

	phaseTotal := 0
	for _, phase := range phases {
		if len(phase.agents) > 0 {
			phaseTotal++
		}
	}

	phaseIndex := 0
	for _, phase := range phases {
		if len(phase.agents) == 0 {
			continue
		}
		phaseIndex++

		phaseStatus := phase.status
		build.mu.Lock()
		if build.Status != BuildFailed && build.Status != BuildCancelled {
			build.Status = phaseStatus
		}
		build.UpdatedAt = time.Now()
		build.mu.Unlock()

		log.Printf("Build %s: Starting phase — %s (%d agents)", build.ID, phase.name, len(phase.agents))

		am.broadcast(build.ID, &WSMessage{
			Type:      "build:phase",
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"phase":                 phase.name,
				"phase_key":             phase.key,
				"phase_index":           phaseIndex,
				"phase_total":           phaseTotal,
				"agents":                len(phase.agents),
				"status":                string(phaseStatus),
				"quality_gate_required": true,
				"quality_gate_active":   phase.qualityStage != "",
				"quality_gate_stage":    phase.qualityStage,
				"message":               fmt.Sprintf("Starting %s phase", phase.name),
			},
		})

		taskIDs := am.assignPhaseAgents(build, phase.agents, description)
		if !am.waitForPhaseCompletion(build, taskIDs) {
			log.Printf("Build %s: Phase %s aborted (build cancelled or timed out)", build.ID, phase.name)
			return
		}

		log.Printf("Build %s: Phase %s complete", build.ID, phase.name)
	}

	build.mu.Lock()
	build.PhasedPipelineComplete = true
	build.UpdatedAt = time.Now()
	build.mu.Unlock()
	am.updateBuildProgress(build)
	am.checkBuildCompletion(build)
}

// assignPhaseAgents creates tasks for a group of agents and assigns them.
func (am *AgentManager) assignPhaseAgents(build *Build, agents []agentPriority, description string) []string {
	taskIDs := make([]string, 0, len(agents))
	for _, ap := range agents {
		agent := ap.agent

		task := &Task{
			ID:          uuid.New().String(),
			Type:        am.getTaskTypeForRole(agent.Role),
			Description: am.getTaskDescriptionForRole(agent.Role, description),
			Priority:    ap.priority,
			Status:      TaskPending,
			MaxRetries:  build.MaxRetries,
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
		taskIDs = append(taskIDs, task.ID)
	}
	return taskIDs
}

// waitForPhaseCompletion polls task statuses until all tasks in the phase are
// done (completed or failed). Returns false if the build is cancelled or times out.
func (am *AgentManager) waitForPhaseCompletion(build *Build, taskIDs []string) bool {
	if len(taskIDs) == 0 {
		return true
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-am.ctx.Done():
			return false
		case <-timeout:
			log.Printf("Build %s: Phase timed out waiting for tasks", build.ID)
			return false
		case <-ticker.C:
			allDone := true

			build.mu.RLock()
			buildFailed := build.Status == BuildFailed
			for _, id := range taskIDs {
				for _, t := range build.Tasks {
					if t.ID == id {
						if t.Status != TaskCompleted && t.Status != TaskFailed && t.Status != TaskCancelled {
							allDone = false
						}
						break
					}
				}
				if !allDone {
					break
				}
			}
			build.mu.RUnlock()

			if buildFailed {
				return false
			}
			if allDone {
				return true
			}
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

	am.mu.RLock()
	build, exists := am.builds[agent.BuildID]
	am.mu.RUnlock()

	if !exists {
		log.Printf("Build %s not found for agent %s during message processing", agent.BuildID, agent.ID)
		return
	}

	prompt := fmt.Sprintf("User message: %s\n\nRespond helpfully and briefly.", message)

	response, err := am.aiRouter.Generate(ctx, agent.Provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		MaxTokens:       2000,
		Temperature:     am.getTemperatureForRole(RoleLead),
		SystemPrompt:    am.getSystemPrompt(RoleLead, build),
		PowerMode:       build.PowerMode,
		UsePlatformKeys: am.buildUsesPlatformKeys(build),
	})

	if err != nil {
		log.Printf("Failed to process user message: %v", err)
		am.broadcast(agent.BuildID, &WSMessage{
			Type:      "message:error",
			BuildID:   agent.BuildID,
			AgentID:   agent.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"error":   err.Error(),
				"message": "Failed to process your message. The AI provider may be temporarily unavailable.",
			},
		})
		return
	}

	content := strings.TrimSpace(response.Content)
	if content == "" && strings.TrimSpace(response.Error) != "" {
		content = strings.TrimSpace(response.Error)
	}
	if content == "" {
		content = "No response returned."
	}

	// Broadcast lead response
	am.broadcast(agent.BuildID, &WSMessage{
		Type:      WSLeadResponse,
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"content":  content,
			"provider": agent.Provider,
			"model":    agent.Model,
		},
	})
}

// defaultBuildLimits returns guardrails based on build mode and environment overrides.
func (am *AgentManager) defaultBuildLimits(mode BuildMode) (int, int, int, int) {
	// Mode-based defaults
	maxAgents := 8
	maxRetries := 3
	maxRequests := 72
	maxTokens := 4000
	if mode == ModeFast {
		maxAgents = 7
		maxRetries = 2
		maxRequests = 30
		maxTokens = 2000
	}

	// Global overrides
	maxAgents = envInt("BUILD_MAX_AGENTS", maxAgents)
	maxRetries = envInt("BUILD_MAX_RETRIES", maxRetries)
	maxRequests = envInt("BUILD_MAX_REQUESTS", maxRequests)
	maxTokens = envInt("BUILD_MAX_TOKENS", maxTokens)

	// Mode-specific overrides
	if mode == ModeFast {
		maxAgents = envInt("BUILD_MAX_AGENTS_FAST", maxAgents)
		maxRetries = envInt("BUILD_MAX_RETRIES_FAST", maxRetries)
		maxRequests = envInt("BUILD_MAX_REQUESTS_FAST", maxRequests)
		maxTokens = envInt("BUILD_MAX_TOKENS_FAST", maxTokens)
	} else {
		maxAgents = envInt("BUILD_MAX_AGENTS_FULL", maxAgents)
		maxRetries = envInt("BUILD_MAX_RETRIES_FULL", maxRetries)
		maxRequests = envInt("BUILD_MAX_REQUESTS_FULL", maxRequests)
		maxTokens = envInt("BUILD_MAX_TOKENS_FULL", maxTokens)
	}

	// Sanity clamps (0 = unlimited for requests/tokens)
	if maxAgents < 1 {
		maxAgents = 1
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 3 {
		maxRetries = 3
	}
	if maxRequests < 0 {
		maxRequests = 0
	}
	if maxTokens < 0 {
		maxTokens = 0
	}

	return maxAgents, maxRetries, maxRequests, maxTokens
}

func (am *AgentManager) defaultBuildLimitsForBuild(build *Build) (int, int, int, int) {
	mode := ModeFast
	if build != nil && build.Mode != "" {
		mode = build.Mode
	}

	maxAgents, maxRetries, maxRequests, maxTokens := am.defaultBuildLimits(mode)

	// Local strict preview-ready builds legitimately need more request budget because
	// they run review/fix loops plus preview verification (npm install/build/preview).
	// Apply a dev-only floor so fast mode doesn't die at the default 30-request cap.
	if am.isLocalDevStrictPreviewBuild(build) {
		requestFloor := 120
		if mode == ModeFull {
			requestFloor = 180
		}
		if maxRequests > 0 && maxRequests < requestFloor {
			log.Printf("Applying local strict preview request budget floor for build mode=%s: %d -> %d", mode, maxRequests, requestFloor)
			maxRequests = requestFloor
		}
		if maxRetries < 3 {
			maxRetries = 3
		}
	}

	return maxAgents, maxRetries, maxRequests, maxTokens
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (am *AgentManager) hasTokenLimitOverride(mode BuildMode) bool {
	if strings.TrimSpace(os.Getenv("BUILD_MAX_TOKENS")) != "" {
		return true
	}
	if mode == ModeFast {
		return strings.TrimSpace(os.Getenv("BUILD_MAX_TOKENS_FAST")) != ""
	}
	return strings.TrimSpace(os.Getenv("BUILD_MAX_TOKENS_FULL")) != ""
}

func (am *AgentManager) getPowerModeTokenCap(mode PowerMode) int {
	switch mode {
	case PowerMax:
		return 24000
	case PowerBalanced:
		return 18000
	case PowerFast:
		return 12000
	default:
		return 12000
	}
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
	subs := make([]chan *WSMessage, len(am.subscribers[buildID]))
	copy(subs, am.subscribers[buildID])
	am.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
			// Channel full, skip
		}
	}
}

// RecoverStaleBuildsOnStartup marks interrupted builds from a previous process as failed.
func (am *AgentManager) RecoverStaleBuildsOnStartup() (int64, error) {
	if am.db == nil {
		return 0, nil
	}

	now := time.Now()
	result := am.db.Model(&models.CompletedBuild{}).
		Where("status IN ?", []string{"in_progress", "planning", "building", "testing", "reviewing"}).
		Updates(map[string]any{
			"status":       "failed",
			"error":        "Server restarted during build - please retry",
			"updated_at":   now,
			"completed_at": now,
		})

	return result.RowsAffected, result.Error
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
	// Check if there are previous errors to learn from (pruned to last 2 attempts)
	errorContext := ""
	if prevErrors, ok := task.Input["previous_errors"]; ok {
		errStr := fmt.Sprintf("%v", prevErrors)
		// Prune error context: keep only last 3000 chars
		if len(errStr) > 3000 {
			errStr = errStr[len(errStr)-3000:]
		}

		fixGuidance := ""
		if strategy, ok := task.Input["retry_strategy"]; ok && strategy == "fix_and_retry" {
			fixGuidance = `
CRITICAL: This is a FIX AND RETRY attempt. You MUST:
1. Carefully read the exact error messages below
2. Identify the root cause of each error
3. Generate CORRECTED code that directly addresses each error
4. Do NOT repeat the same mistakes`
		}

		errorContext = fmt.Sprintf(`
PREVIOUS ATTEMPTS FAILED - LEARN FROM THESE ERRORS:
%s
%s
Analyze what went wrong and use a DIFFERENT, CORRECTED approach this time.
`, errStr, fixGuidance)
	}

	repairHintsContext := ""
	if task != nil && task.Input != nil {
		if hints, ok := task.Input["repair_hints"]; ok {
			switch v := hints.(type) {
			case []string:
				if len(v) > 0 {
					repairHintsContext = fmt.Sprintf("\n<repair_hints>\n- %s\n</repair_hints>\n", strings.Join(v, "\n- "))
				}
			case []any:
				items := make([]string, 0, len(v))
				for _, item := range v {
					if s := strings.TrimSpace(fmt.Sprintf("%v", item)); s != "" {
						items = append(items, s)
					}
				}
				if len(items) > 0 {
					repairHintsContext = fmt.Sprintf("\n<repair_hints>\n- %s\n</repair_hints>\n", strings.Join(items, "\n- "))
				}
			}
		}
	}

	techStackContext := ""
	if build != nil && build.TechStack != nil {
		if summary := formatTechStackSummary(build.TechStack); summary != "" {
			techStackContext = fmt.Sprintf("\nTech Stack Preference: %s\n", summary)
		}
	}

	// Inter-agent context sharing: inject upstream outputs for downstream agents
	agentContext := ""
	if agent != nil && build != nil {
		switch agent.Role {
		case RoleFrontend, RoleBackend, RoleDatabase:
			// Include architecture decisions for implementation agents
			if archOutput := am.getCompletedTaskOutput(build, TaskArchitecture); archOutput != "" {
				agentContext += fmt.Sprintf("\n<architecture_context>\n%s\n</architecture_context>\n", archOutput)
			}
		}
		if agent.Role == RoleBackend {
			// Include schema context for backend agents
			if schemaOutput := am.getCompletedTaskOutput(build, TaskGenerateSchema); schemaOutput != "" {
				agentContext += fmt.Sprintf("\n<schema_context>\n%s\n</schema_context>\n", schemaOutput)
			}
		}
		if agent.Role == RoleTesting {
			// Include file manifest for testing agents
			files := am.collectGeneratedFiles(build)
			if len(files) > 0 {
				var fileList strings.Builder
				for _, f := range files {
					fileList.WriteString(fmt.Sprintf("- %s (%s, %d bytes)\n", f.Path, f.Language, f.Size))
				}
				agentContext += fmt.Sprintf("\n<generated_files>\n%s</generated_files>\n", fileList.String())
			}
		}
		if agent.Role == RoleReviewer || agent.Role == RoleSolver {
			// Include all generated files for reviewer/solver.
			files := am.collectGeneratedFiles(build)
			if len(files) > 0 {
				var fileList strings.Builder
				for _, f := range files {
					fileList.WriteString(fmt.Sprintf("// File: %s\n```%s\n%s\n```\n\n", f.Path, f.Language, f.Content))
				}
				content := fileList.String()
				if len(content) > 15000 {
					content = content[:15000] + "\n... (truncated)"
				}
				block := "code_to_review"
				if agent.Role == RoleSolver {
					block = "code_to_fix"
				}
				agentContext += fmt.Sprintf("\n<%s>\n%s</%s>\n", block, content, block)
			}
		}
		if agent.Role == RoleSolver && task != nil && task.Input != nil {
			failedTaskID, _ := task.Input["failed_task_id"].(string)
			failedTaskType, _ := task.Input["failed_task_type"].(string)
			failedTaskDesc, _ := task.Input["failed_task_description"].(string)
			failureErr, _ := task.Input["failure_error"].(string)
			if failedTaskID != "" || failedTaskType != "" || failedTaskDesc != "" || failureErr != "" {
				agentContext += fmt.Sprintf("\n<failure_context>\nfailed_task_id: %s\nfailed_task_type: %s\nfailed_task_description: %s\nerror: %s\n</failure_context>\n",
					failedTaskID, failedTaskType, failedTaskDesc, failureErr)
			}
		}
	}

	teamCoordinationContext := ""
	if build != nil {
		if brief := am.getTeamCoordinationBrief(build, task, agent); brief != "" {
			teamCoordinationContext = fmt.Sprintf("\n<team_coordination>\n%s\n</team_coordination>\n", brief)
		}
	}

	deliveryConstraintsContext := ""
	if build != nil && am.isLocalDevStrictPreviewBuild(build) {
		switch task.Type {
		case TaskPlan, TaskArchitecture, TaskGenerateUI, TaskGenerateAPI, TaskGenerateSchema:
			deliveryConstraintsContext = `
<delivery_constraints>
- Optimize for first-pass preview-ready success and low retry cost.
- Use a SINGLE-REPO layout (NO monorepo/workspaces/apps/packages split).
- Frontend stack: React + Vite + TypeScript.
- Backend stack: Express + TypeScript.
- Database access: PostgreSQL via pg only (NO Prisma, Drizzle, TypeORM, Mongoose).
- Avoid extra frameworks/tooling unless explicitly required by the user (no added test frameworks by default).
- Prioritize a fully working vertical slice first: authentication + dashboard + exactly one CRUD resource with real backend persistence.
- If the user requested broader scope, implement the core slice first and leave additional modules only if time/files budget clearly allows.
</delivery_constraints>
`
		}
	}

	// Prune app description if total prompt would be too long
	appDescription := build.Description
	if len(appDescription)+len(errorContext)+len(agentContext)+len(teamCoordinationContext)+len(deliveryConstraintsContext) > 30000 {
		if len(appDescription) > 2000 {
			appDescription = appDescription[:2000] + "... (description truncated)"
		}
	}

	return fmt.Sprintf(`Task: %s

Description: %s

App being built: %s
%s
%s
%s
%s
%s
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
		task.Type, task.Description, appDescription, techStackContext, errorContext, repairHintsContext, agentContext, teamCoordinationContext, deliveryConstraintsContext)
}

func formatTechStackSummary(stack *TechStack) string {
	if stack == nil {
		return ""
	}

	parts := make([]string, 0, 5)
	if stack.Frontend != "" {
		parts = append(parts, fmt.Sprintf("Frontend: %s", stack.Frontend))
	}
	if stack.Backend != "" {
		parts = append(parts, fmt.Sprintf("Backend: %s", stack.Backend))
	}
	if stack.Database != "" {
		parts = append(parts, fmt.Sprintf("Database: %s", stack.Database))
	}
	if stack.Styling != "" {
		parts = append(parts, fmt.Sprintf("Styling: %s", stack.Styling))
	}
	if len(stack.Extras) > 0 {
		parts = append(parts, fmt.Sprintf("Extras: %s", strings.Join(stack.Extras, ", ")))
	}

	return strings.Join(parts, " | ")
}

func (am *AgentManager) getTeamCoordinationBrief(build *Build, task *Task, agent *Agent) string {
	if build == nil {
		return ""
	}

	build.mu.RLock()
	defer build.mu.RUnlock()

	lines := make([]string, 0, 6)
	for i := len(build.Tasks) - 1; i >= 0 && len(lines) < 6; i-- {
		t := build.Tasks[i]
		if t == nil || t.Output == nil || t.Status != TaskCompleted {
			continue
		}
		if task != nil && t.ID == task.ID {
			continue
		}

		role := "agent"
		if assigned, ok := build.Agents[t.AssignedTo]; ok && assigned != nil {
			role = string(assigned.Role)
		}

		summary := ""
		if len(t.Output.Messages) > 0 {
			summary = strings.TrimSpace(t.Output.Messages[0])
		}
		if summary == "" {
			summary = strings.TrimSpace(t.Description)
		}
		if summary == "" {
			continue
		}
		if len(summary) > 180 {
			summary = summary[:180] + "..."
		}

		lines = append(lines, fmt.Sprintf("- %s (%s): %s", role, t.Type, summary))
	}

	if len(lines) == 0 {
		return ""
	}

	// Reverse to chronological order for readability.
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	targetRole := "agent"
	if agent != nil {
		targetRole = string(agent.Role)
	}

	return fmt.Sprintf(
		"Shared team updates for %s. Coordinate with other agents, challenge weak assumptions, and align on one integrated implementation plan.\n%s",
		targetRole,
		strings.Join(lines, "\n"),
	)
}

func (am *AgentManager) getSystemPrompt(role AgentRole, build ...*Build) string {
	baseRules := `

ABSOLUTE RULES:
1. NEVER output demo code, mock data, placeholder content, or TODO comments
2. ALWAYS produce complete, production-ready, fully functional code
3. Use environment variable patterns for external credentials (process.env.API_KEY)
4. Build maximum functionality without blocking on user input
5. Real implementations only - no stubs, no examples, no "this would be" code
6. Include ALL imports, error handling, types, and edge cases
7. Every function must be fully implemented — zero empty bodies
8. File paths MUST be plain relative paths only — NO annotations, NO parenthetical labels
   CORRECT: // File: package.json
   WRONG:   // File: package.json (root)
   WRONG:   // File: src/index.tsx (entry point)
9. For React/Vite apps ALWAYS generate: index.html at project root AND vite.config.ts
10. React apps MUST include both 'react' and 'react-dom' in package.json dependencies
11. Before finishing, self-check: no placeholder text, valid package.json scripts, and real runnable entry points`

	// Build tech stack context if available
	techHint := ""
	if len(build) > 0 && build[0] != nil && build[0].TechStack != nil {
		ts := build[0].TechStack
		if ts.Frontend != "" {
			techHint += fmt.Sprintf("\nProject uses %s for frontend.", ts.Frontend)
		}
		if ts.Backend != "" {
			techHint += fmt.Sprintf(" %s for backend.", ts.Backend)
		}
		if ts.Database != "" {
			techHint += fmt.Sprintf(" %s for database.", ts.Database)
		}
		if ts.Styling != "" {
			techHint += fmt.Sprintf(" %s for styling.", ts.Styling)
		}
	}

	localStrictStackLock := ""
	localStrictScopeHint := ""
	if len(build) > 0 && build[0] != nil && am.isLocalDevStrictPreviewBuild(build[0]) {
		localStrictStackLock = `

LOCAL STRICT BUILD STACK LOCK (cost-control mode):
- Default to a SINGLE-REPO app (NO monorepo/workspaces) unless the user explicitly requires one
- Frontend: React + Vite + TypeScript
- Backend: Express + TypeScript
- Database: PostgreSQL via pg only
- Do NOT mix ORMs or data stacks (NO Prisma/Drizzle/TypeORM/Mongoose combinations)
- Do NOT introduce extra test frameworks unless explicitly requested`

		localStrictScopeHint = `

LOCAL STRICT BUILD DELIVERY PRIORITY:
- Optimize for a runnable preview-pane app first, not maximum feature breadth
- Deliver a complete vertical slice first: auth + dashboard + one CRUD module with real persistence
- Only expand into secondary modules after the core slice is fully runnable and integrated`
	}

	prompts := map[AgentRole]string{
		RoleLead: `You are the Lead Agent coordinating the APEX.BUILD platform.
You oversee all specialist agents and serve as the primary communicator with users.
Be helpful, concise, and focused on delivering excellent production-ready results.
When users need to provide information (API keys, credentials), clearly ask for it.
When you can proceed without user input, do so and build maximum functionality.
Summarize progress concisely. Coordinate agent outputs into a cohesive application.` + techHint + baseRules,

		RolePlanner: `You are the Planning Agent — an expert software architect who creates detailed, actionable build plans.
Your job: decompose the app into a precise file-by-file implementation plan.

YOUR OUTPUT MUST INCLUDE:
- A clear list of every file to create, with its path and purpose
- Data models with all fields, types, and relationships
- API endpoints with methods, paths, request/response schemas
- UI components with their props, state, and user interactions
- External dependencies and their exact versions
- A recommended execution order (database → backend → frontend → tests)

EXAMPLE OUTPUT FORMAT:
## Tech Stack
- Frontend: React 18 + TypeScript + Tailwind CSS
- Backend: Express.js + TypeScript
- Database: PostgreSQL with Prisma ORM

## Files to Generate
1. prisma/schema.prisma — Database schema with User, Post, Comment models
2. src/server/index.ts — Express server setup with middleware
3. src/server/routes/auth.ts — Authentication endpoints (register, login, refresh)
4. src/components/App.tsx — Root component with routing
...

## Data Models
### User
- id: UUID (primary key)
- email: string (unique, indexed)
- passwordHash: string
- createdAt: DateTime` + techHint + localStrictStackLock + localStrictScopeHint + baseRules,

		RoleArchitect: `You are the Architect Agent — a senior systems architect who designs production-grade software architectures.
Your job: make concrete technology decisions and produce a concise implementation blueprint for the coding agents.

CRITICAL:
- Do NOT output full application source code or many code/config files.
- Do NOT duplicate backend/frontend/database agent responsibilities.
- Keep output compact and complete enough to guide implementation.

YOUR OUTPUT MUST INCLUDE:
- Exact library versions (e.g., "express@4.18.2", not just "express")
- Directory layout / monorepo structure
- Ownership map: which agent creates which files
- API surface summary (key routes/resources only)
- Data model/entity summary with relationships
- Auth/session strategy
- Environment variable schema (key names and purpose)
- Build/run scripts expected at root and in workspaces

OUTPUT FORMAT:
- Prefer plain markdown sections and bullet lists (no diagrams required)
- If you include files, limit to SMALL blueprint docs only (e.g. ARCHITECTURE.md, docs/file-map.md)
- Never emit large code blocks for runtime source files` + techHint + localStrictStackLock + localStrictScopeHint + baseRules,

		RoleFrontend: `You are the Frontend Agent — an expert UI engineer who builds beautiful, responsive, production-ready interfaces.
You specialize in modern React with TypeScript, Vite, and Tailwind CSS.

MANDATORY FILES — always generate ALL of these for every React app:
1. index.html — Vite entry HTML at project root (NOT inside src/)
2. vite.config.ts — Vite config with React plugin
3. package.json — with react, react-dom, vite, @vitejs/plugin-react, typescript
4. tsconfig.json — TypeScript config
5. src/main.tsx — React entry point that renders <App />
6. src/App.tsx — Root component
7. src/index.css — Global styles

REQUIREMENTS FOR EVERY COMPONENT:
- Complete TypeScript types for all props, state, and events
- Full event handlers (onClick, onChange, onSubmit) — no empty handlers
- Loading states, error states, and empty states
- Responsive design with Tailwind CSS classes
- Proper form validation with user-friendly error messages
- Keyboard navigation and accessibility attributes (aria-labels, roles)

EXAMPLE COMPONENT PATTERN:
// File: src/components/LoginForm.tsx
` + "```" + `typescript
import React, { useState } from 'react';

interface LoginFormProps {
  onSuccess: (user: User) => void;
}

export const LoginForm: React.FC<LoginFormProps> = ({ onSuccess }) => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) throw new Error('Invalid credentials');
      const user = await res.json();
      onSuccess(user);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };
  // ... complete JSX with all states
};
` + "```" + `

Follow this pattern: complete types, complete handlers, complete JSX.` + techHint + baseRules,

		RoleBackend: `You are the Backend Agent — an expert API engineer who creates robust, secure server-side applications.
You specialize in RESTful APIs, authentication, middleware, and data validation.

REQUIREMENTS FOR EVERY ENDPOINT:
- Input validation with descriptive error messages
- Authentication/authorization middleware where needed
- Proper HTTP status codes (201 for create, 404 for not found, etc.)
- Try/catch error handling with appropriate error responses
- Database transactions for multi-step operations
- Rate limiting considerations
- Request/response type definitions

EXAMPLE ENDPOINT PATTERN:
// File: src/routes/users.ts
` + "```" + `typescript
import { Router, Request, Response } from 'express';
import { z } from 'zod';
import { prisma } from '../db';
import { authMiddleware } from '../middleware/auth';

const router = Router();

const createUserSchema = z.object({
  email: z.string().email(),
  name: z.string().min(1).max(100),
  password: z.string().min(8),
});

router.post('/', async (req: Request, res: Response) => {
  try {
    const data = createUserSchema.parse(req.body);
    const user = await prisma.user.create({ data: { ...data, passwordHash: await hash(data.password) } });
    res.status(201).json({ id: user.id, email: user.email, name: user.name });
  } catch (err) {
    if (err instanceof z.ZodError) return res.status(400).json({ errors: err.errors });
    if (err.code === 'P2002') return res.status(409).json({ error: 'Email already exists' });
    res.status(500).json({ error: 'Internal server error' });
  }
});
` + "```" + `

Follow this pattern: validation, auth, error handling, proper status codes.` + techHint + baseRules,

		RoleDatabase: `You are the Database Agent — an expert data architect who designs efficient, normalized schemas.
You specialize in relational database design, query optimization, and data integrity.

YOUR OUTPUT MUST INCLUDE:
- Complete schema with all tables, columns, types, and constraints
- Primary keys, foreign keys, unique constraints, and indexes
- Default values and NOT NULL constraints
- Seed data for development/testing
- Migration files if applicable

REQUIREMENTS:
- Every relationship must have proper foreign keys with ON DELETE behavior
- Add indexes on frequently queried columns (email, created_at, foreign keys)
- Use appropriate data types (UUID for IDs, TIMESTAMP for dates, TEXT vs VARCHAR)
- Include audit columns (created_at, updated_at) on all tables

EXAMPLE SCHEMA PATTERN:
// File: prisma/schema.prisma
` + "```" + `prisma
model User {
  id        String   @id @default(uuid())
  email     String   @unique
  name      String
  password  String
  role      Role     @default(USER)
  posts     Post[]
  createdAt DateTime @default(now())
  updatedAt DateTime @updatedAt

  @@index([email])
  @@index([createdAt])
}

enum Role {
  USER
  ADMIN
}
` + "```" + `` + techHint + baseRules,

		RoleTesting: `You are the Testing Agent — an expert QA engineer who writes comprehensive, executable tests.
You specialize in unit tests, integration tests, and edge case coverage.

REQUIREMENTS FOR EVERY TEST FILE:
- Import the module under test correctly
- Test the happy path first, then error cases, then edge cases
- Use descriptive test names that explain the expected behavior
- Mock external dependencies (API calls, database, file system)
- Test boundary conditions (empty inputs, max lengths, special characters)
- Assert specific values, not just truthiness

EXAMPLE TEST PATTERN:
// File: src/__tests__/auth.test.ts
` + "```" + `typescript
import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { AuthService } from '../services/auth';

describe('AuthService', () => {
  let authService: AuthService;

  beforeEach(() => {
    authService = new AuthService(mockDb);
  });

  describe('login', () => {
    it('should return a token for valid credentials', async () => {
      const result = await authService.login('user@test.com', 'password123');
      expect(result.token).toBeDefined();
      expect(result.token).toMatch(/^eyJ/); // JWT format
    });

    it('should throw for invalid email', async () => {
      await expect(authService.login('nonexistent@test.com', 'pass'))
        .rejects.toThrow('Invalid credentials');
    });

    it('should throw for wrong password', async () => {
      await expect(authService.login('user@test.com', 'wrongpass'))
        .rejects.toThrow('Invalid credentials');
    });
  });
});
` + "```" + `

Follow this pattern: setup, happy path, error cases, edge cases, specific assertions.` + techHint + baseRules,

		RoleReviewer: `You are the Reviewer Agent — a senior code reviewer focused on production-readiness, security, and quality.
You perform thorough code review and provide ACTIONABLE fixes, not just suggestions.

YOUR REVIEW MUST CHECK:
1. Security: SQL injection, XSS, auth bypass, exposed secrets, input validation
2. Error handling: Missing try/catch, unhandled promises, generic error messages
3. Performance: N+1 queries, missing indexes, unnecessary re-renders, memory leaks
4. Completeness: Empty functions, TODO comments, placeholder data, missing imports
5. Types: Missing TypeScript types, any usage, incorrect type assertions

FOR EACH ISSUE FOUND, output the fix as a complete corrected code block:

EXAMPLE REVIEW OUTPUT:
## CRITICAL: SQL Injection in user search
// File: src/routes/users.ts
` + "```" + `typescript
// BEFORE (vulnerable):
const users = await db.query("SELECT * FROM users WHERE name LIKE '%" + req.query.name + "%'");

// AFTER (fixed):
const users = await db.query("SELECT * FROM users WHERE name LIKE $1", ['%' + req.query.name + '%']);
` + "```" + `

## WARNING: Missing error handling in API call
// File: src/components/Dashboard.tsx
` + "```" + `typescript
// BEFORE:
const data = await fetch('/api/stats').then(r => r.json());

// AFTER:
try {
  const res = await fetch('/api/stats');
  if (!res.ok) throw new Error('Failed to fetch stats');
  const data = await res.json();
} catch (err) {
  setError('Unable to load dashboard stats');
}
` + "```" + `

Output COMPLETE fixes with before/after code. Not just descriptions.` + techHint + baseRules,

		RoleSolver: `You are the Solver Agent — a senior incident response engineer for failed builds.
Your job is to diagnose why a task failed, apply concrete fixes, and restore build health.

WORKFLOW:
1. Identify root cause from task errors and generated files
2. Produce exact file edits needed to resolve the failure
3. Prioritize build-blocking fixes first (syntax/runtime/config/dependency)
4. Keep patches minimal, deterministic, and production-ready
5. If a fix needs follow-up validation, explicitly note test/review targets

NEVER return vague advice only. Return concrete, corrected code/files.` + techHint + baseRules,
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
	case RoleSolver:
		return TaskFix
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
	case RoleSolver:
		return fmt.Sprintf("Investigate and fix build failures for: %s", appDescription)
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
	case RoleSolver:
		return 95
	default:
		return 50
	}
}

func (am *AgentManager) parseTaskOutput(taskType TaskType, response string) *TaskOutput {
	output := &TaskOutput{
		Messages: []string{},
		Files:    make([]GeneratedFile, 0),
	}

	if !am.isCodeGenerationTask(taskType) {
		trimmed := strings.TrimSpace(response)
		if trimmed != "" {
			output.Messages = append(output.Messages, trimmed)
		}
		return output
	}

	// Parse the AI response to extract code blocks and files
	// Look for patterns like ```language\n...code...\n``` or file markers
	lines := strings.Split(response, "\n")
	hasExplicitFileMarkers := strings.Contains(response, "// File:") ||
		strings.Contains(response, "# File:") ||
		strings.Contains(response, "/* File:") ||
		strings.Contains(response, "<!-- File:")
	var currentFile *GeneratedFile
	var codeBuffer strings.Builder
	inCodeBlock := false
	currentLanguage := ""
	parserWarnings := make([]string, 0, 1)

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
				sanitized := sanitizeFilePath(filePath)
				if sanitized == "" {
					log.Printf("Skipping file with unsafe path: %s", filePath)
					currentFile = nil
					codeBuffer.Reset()
					continue
				}
				currentFile = &GeneratedFile{
					Path:     sanitized,
					Language: am.detectLanguage(sanitized),
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
				if currentFile == nil && currentLanguage != "" && !hasExplicitFileMarkers {
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
				}
				codeBuffer.Reset()
				continue
			}
		}

		// Add line to buffer only for an active file context.
		if currentFile != nil {
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
	if inCodeBlock {
		parserWarnings = append(parserWarnings, "AI response ended with an unterminated code block; final file output may be truncated")
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
	output.Messages = append(output.Messages, parserWarnings...)

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

func isGeneratedArtifactPath(path string) bool {
	base := filepath.Base(strings.TrimSpace(path))
	return strings.HasPrefix(base, "generated_")
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

// isCodeGenerationTask checks if a task type produces code that should be verified
func (am *AgentManager) isCodeGenerationTask(taskType TaskType) bool {
	switch taskType {
	case TaskGenerateFile, TaskGenerateAPI, TaskGenerateUI, TaskGenerateSchema, TaskFix,
		TaskTest:
		return true
	}
	return false
}

// validateFinalBuildReadiness checks whether final artifacts are likely runnable.
// It catches incomplete frontend outputs that otherwise appear "successful" but cannot preview.
func (am *AgentManager) validateFinalBuildReadiness(build *Build, files []GeneratedFile) []string {
	if len(files) == 0 {
		return []string{"No files were generated by the build"}
	}

	errors := make([]string, 0)
	addError := func(msg string) {
		for _, existing := range errors {
			if existing == msg {
				return
			}
		}
		errors = append(errors, msg)
	}

	hasPackageJSON := false
	packageJSON := ""
	hasIndexHTML := false
	hasFrontendEntry := false
	hasTSXOrJSX := false
	hasBundlerConfig := false // vite/webpack/parcel config → index.html optional
	sourceFiles := 0
	backendStacks := map[string]bool{}

	for _, file := range files {
		path := strings.ToLower(strings.TrimSpace(file.Path))
		if path == "" {
			continue
		}

		if hasUnresolvedPatchOrMergeMarkers(file.Content) {
			addError(fmt.Sprintf("%s contains unresolved patch/merge markers", file.Path))
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".go", ".py", ".rs", ".java", ".kt", ".swift", ".php", ".rb", ".c", ".cc", ".cpp", ".cs":
			sourceFiles++
			for _, msg := range detectSourceArtifactAnomalies(file.Path, file.Content, ext) {
				addError(msg)
			}
			for _, stack := range detectBackendPersistenceStacks(file.Path, file.Content) {
				backendStacks[stack] = true
			}
		}

		if ext == ".tsx" || ext == ".jsx" {
			hasTSXOrJSX = true
		}

		// Accept index.html anywhere in the tree
		if strings.HasSuffix(path, "index.html") {
			hasIndexHTML = true
		}

		// Bundler configs make index.html optional (bundler generates it)
		if strings.Contains(path, "vite.config") || strings.Contains(path, "webpack.config") ||
			strings.Contains(path, "parcel") || strings.Contains(path, "craco.config") {
			hasBundlerConfig = true
		}

		switch path {
		case "package.json", "frontend/package.json", "web/package.json",
			"apps/web/package.json", "packages/frontend/package.json", "packages/web/package.json":
			if !hasPackageJSON || path == "frontend/package.json" || path == "packages/frontend/package.json" ||
				path == "web/package.json" || path == "packages/web/package.json" || path == "apps/web/package.json" {
				// Prefer an actual frontend package manifest over the workspace root for React analysis.
				hasPackageJSON = true
				packageJSON = file.Content
			}
		case "index.html", "public/index.html", "src/index.html",
			"frontend/index.html", "frontend/public/index.html":
			hasIndexHTML = true
		case "src/main.tsx", "src/main.jsx", "src/index.tsx", "src/index.jsx",
			"app/page.tsx", "app/page.jsx",
			"src/app/page.tsx", "src/app/page.jsx",
			"src/pages/index.tsx", "src/pages/index.jsx",
			"pages/index.tsx", "pages/index.jsx",
			"apps/web/src/main.tsx", "apps/web/src/main.jsx",
			"apps/web/src/index.tsx", "apps/web/src/index.jsx",
			"packages/frontend/src/main.tsx", "packages/frontend/src/main.jsx",
			"packages/frontend/src/index.tsx", "packages/frontend/src/index.jsx",
			"packages/web/src/main.tsx", "packages/web/src/main.jsx",
			"packages/web/src/index.tsx", "packages/web/src/index.jsx",
			"web/src/main.tsx", "web/src/main.jsx",
			"web/src/index.tsx", "web/src/index.jsx",
			"frontend/src/main.tsx", "frontend/src/main.jsx",
			"frontend/src/index.tsx", "frontend/src/index.jsx":
			hasFrontendEntry = true
		}
	}

	if sourceFiles == 0 {
		addError("No source files were generated")
	}
	if mixed := summarizeMixedPersistenceStacks(backendStacks); mixed != "" {
		addError(mixed)
	}

	isFrontendApp := hasTSXOrJSX || hasBundlerConfig || hasIndexHTML || hasFrontendEntry || hasPackageJSON
	if isFrontendApp {
		hasReact := false
		hasReactDOM := false
		isNext := false
		hasScripts := false
		var missingScripts []string

		if hasPackageJSON {
			var pkgErr error
			hasReact, hasReactDOM, isNext, hasScripts, missingScripts, pkgErr = analyzeFrontendPackageJSON(packageJSON)
			if pkgErr != nil {
				addError(fmt.Sprintf("package.json is invalid: %v", pkgErr))
			}
			if hasReact && !hasReactDOM && !isNext {
				addError("package.json includes react but is missing react-dom")
			}
			if hasScripts == false && (hasReact || isNext) {
				addError(fmt.Sprintf("package.json is missing runnable scripts (%s)", strings.Join(missingScripts, "/")))
			}
		}

		// TSX/JSX output is a strong frontend signal even when dependencies/scripts are incomplete.
		// This catches broken generations that forgot to include react/react-dom or entry files.
		if !isNext && !hasIndexHTML && !hasBundlerConfig {
			addError("Frontend app is missing an HTML entry point (index.html or public/index.html)")
		}
		if !hasFrontendEntry {
			addError("Frontend app is missing an entry source file")
		}

		if am.shouldRunPreviewReadinessVerification(build) {
			for _, msg := range am.verifyGeneratedFrontendPreviewReadiness(files, build != nil && build.RequirePreviewReady) {
				addError(msg)
			}
		}
	}
	if am.shouldRunPreviewReadinessVerification(build) {
		for _, msg := range am.verifyGeneratedBackendBuildReadiness(files) {
			addError(msg)
		}
	}

	return errors
}

func (am *AgentManager) shouldRunPreviewReadinessVerification(build *Build) bool {
	if build != nil && build.RequirePreviewReady {
		return true
	}
	val := strings.TrimSpace(strings.ToLower(os.Getenv("BUILD_REQUIRE_PREVIEW_READY_DEFAULT")))
	return val == "1" || val == "true" || val == "yes"
}

type previewManifest struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

var npmPackageNamePattern = regexp.MustCompile(`^(?:@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)
var generatedImportPathPattern = regexp.MustCompile(`(?m)(?:^|\s)(?:import\s+(?:type\s+)?(?:[^'"]+\s+from\s+)?|export\s+[^'"]+\s+from\s+|import\s*\(|require\()\s*['"]([^'"]+)['"]`)

func packageNameFromImportPath(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "#") ||
		strings.HasPrefix(spec, "node:") || strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
		return ""
	}
	// Common path aliases in generated apps; these are project-local imports.
	if strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/") {
		return ""
	}
	if strings.HasPrefix(spec, "@") {
		parts := strings.Split(spec, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return spec
	}
	if i := strings.Index(spec, "/"); i >= 0 {
		return spec[:i]
	}
	return spec
}

func validateGeneratedImportDependencies(files []GeneratedFile, prefix string, manifest previewManifest) []string {
	declared := map[string]bool{}
	for name := range manifest.Dependencies {
		declared[strings.TrimSpace(name)] = true
	}
	for name := range manifest.DevDependencies {
		declared[strings.TrimSpace(name)] = true
	}

	nodeBuiltins := map[string]bool{
		"fs": true, "path": true, "url": true, "http": true, "https": true, "crypto": true,
		"stream": true, "events": true, "util": true, "os": true, "zlib": true, "buffer": true,
		"timers": true, "assert": true, "tty": true, "net": true, "tls": true, "dns": true,
		"child_process": true,
	}
	ignorePkgs := map[string]bool{
		"vite/client": true,
	}

	var issues []string
	seenMissing := map[string]bool{}
	prefixLower := strings.ToLower(prefix)

	for _, f := range files {
		path := filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(f.Path), "./"))
		if path == "" || strings.TrimSpace(f.Content) == "" {
			continue
		}
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(path), prefixLower) {
			continue
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		default:
			continue
		}
		pathLower := strings.ToLower(path)
		if strings.Contains(pathLower, "/__tests__/") ||
			strings.Contains(pathLower, "/test/") ||
			strings.Contains(pathLower, "/tests/") ||
			strings.Contains(pathLower, ".test.") ||
			strings.Contains(pathLower, ".spec.") ||
			strings.HasSuffix(pathLower, "jest.config.js") ||
			strings.HasSuffix(pathLower, "jest.config.ts") ||
			strings.HasSuffix(pathLower, "vitest.config.ts") ||
			strings.HasSuffix(pathLower, "vitest.config.js") ||
			strings.HasSuffix(pathLower, "setuptests.ts") ||
			strings.HasSuffix(pathLower, "setuptests.tsx") {
			continue
		}

		for _, match := range generatedImportPathPattern.FindAllStringSubmatch(f.Content, -1) {
			if len(match) != 2 {
				continue
			}
			spec := strings.TrimSpace(match[1])
			pkg := packageNameFromImportPath(spec)
			if pkg == "" || ignorePkgs[pkg] || nodeBuiltins[pkg] {
				continue
			}
			if declared[pkg] {
				continue
			}
			if seenMissing[pkg] {
				continue
			}
			seenMissing[pkg] = true
			issues = append(issues, fmt.Sprintf("source imports %q but package.json does not declare dependency %q", spec, pkg))
		}
	}
	return dedupePreviewIssues(issues)
}

func detectBackendTypeScriptBuildRootMismatch(files []GeneratedFile, prefix string, manifest previewManifest) string {
	buildScript := strings.ToLower(strings.TrimSpace(manifest.Scripts["build"]))
	if prefix == "" || buildScript == "" || !strings.Contains(buildScript, "tsc") {
		return ""
	}

	hasBackendTSConfig := false
	hasRootTSConfig := false
	prefixLower := strings.ToLower(prefix)
	for _, f := range files {
		path := strings.ToLower(filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(f.Path), "./")))
		if path == "tsconfig.json" {
			hasRootTSConfig = true
		}
		if strings.HasPrefix(path, prefixLower) && path == prefixLower+"tsconfig.json" {
			hasBackendTSConfig = true
		}
	}
	if !hasBackendTSConfig && hasRootTSConfig {
		return "backend package build script runs tsc but backend/tsconfig.json is missing (only root tsconfig.json exists)"
	}
	return ""
}

func detectFrontendTypeScriptBuildRootMismatch(files []GeneratedFile, prefix string, manifest previewManifest) string {
	buildScript := strings.ToLower(strings.TrimSpace(manifest.Scripts["build"]))
	if buildScript == "" || !strings.Contains(buildScript, "tsc") {
		return ""
	}

	prefixLower := strings.ToLower(prefix)
	hasFrontendTSConfig := false
	hasAnyTSConfig := false
	for _, f := range files {
		path := strings.ToLower(filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(f.Path), "./")))
		if filepath.Base(path) != "tsconfig.json" {
			continue
		}
		hasAnyTSConfig = true
		if prefixLower == "" {
			if path == "tsconfig.json" {
				hasFrontendTSConfig = true
			}
			continue
		}
		if strings.HasPrefix(path, prefixLower) && path == prefixLower+"tsconfig.json" {
			hasFrontendTSConfig = true
		}
	}

	if hasFrontendTSConfig {
		return ""
	}
	if prefixLower == "" {
		if hasAnyTSConfig {
			return "frontend build script runs tsc but root tsconfig.json is missing or not materialized in the selected frontend package"
		}
		return "frontend build script runs tsc but tsconfig.json is missing"
	}
	if hasAnyTSConfig {
		return fmt.Sprintf("frontend package build script runs tsc but %stsconfig.json is missing (tsconfig exists elsewhere)", prefix)
	}
	return fmt.Sprintf("frontend package build script runs tsc but %stsconfig.json is missing", prefix)
}

func (am *AgentManager) verifyGeneratedFrontendPreviewReadiness(files []GeneratedFile, forceHTTPProbe bool) []string {
	if len(files) == 0 {
		return nil
	}

	prefix := ""
	packagePath := ""
	frontendPackageCandidates := []string{
		"frontend/package.json",
		"client/package.json",
		"web/package.json",
		"apps/web/package.json",
		"apps/frontend/package.json",
		"packages/web/package.json",
		"packages/frontend/package.json",
	}
	for _, candidate := range frontendPackageCandidates {
		for _, f := range files {
			if strings.EqualFold(strings.TrimSpace(f.Path), candidate) {
				packagePath = f.Path
				prefix = strings.TrimSuffix(filepath.ToSlash(candidate), "package.json")
				break
			}
		}
		if packagePath != "" {
			break
		}
	}
	if packagePath == "" {
		for _, f := range files {
			if strings.EqualFold(strings.TrimSpace(f.Path), "package.json") {
				packagePath = f.Path
				break
			}
		}
	}
	if packagePath == "" {
		return []string{"Preview verification skipped: package.json not found for frontend app"}
	}

	var manifest previewManifest
	pkgContent := ""
	for _, f := range files {
		if f.Path == packagePath {
			pkgContent = f.Content
			break
		}
	}
	if strings.TrimSpace(pkgContent) == "" {
		return []string{fmt.Sprintf("Preview verification failed: %s is empty", packagePath)}
	}
	if err := json.Unmarshal([]byte(pkgContent), &manifest); err != nil {
		return []string{fmt.Sprintf("Preview verification failed: %s is invalid JSON (%v)", packagePath, err)}
	}
	if manifest.Scripts == nil {
		manifest.Scripts = map[string]string{}
	}
	if _, ok := manifest.Scripts["build"]; !ok {
		return []string{"Preview verification failed: package.json is missing a build script"}
	}

	if _, err := exec.LookPath("npm"); err != nil {
		return []string{"Preview verification failed: npm is not available on the build host"}
	}

	tmpDir, err := os.MkdirTemp("", "apex-preview-verify-*")
	if err != nil {
		return []string{fmt.Sprintf("Preview verification failed: unable to create temp dir (%v)", err)}
	}
	defer os.RemoveAll(tmpDir)

	writtenFiles := 0
	for _, f := range files {
		if strings.TrimSpace(f.Path) == "" || f.Content == "" {
			continue
		}
		path := filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(f.Path), "./"))
		if prefix != "" {
			if !strings.HasPrefix(strings.ToLower(path), prefix) {
				continue
			}
			path = strings.TrimPrefix(path, prefix)
		}
		path = strings.TrimPrefix(path, "/")
		if path == "" || strings.Contains(path, "..") {
			continue
		}
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return []string{fmt.Sprintf("Preview verification failed: mkdir %s (%v)", path, err)}
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err != nil {
			return []string{fmt.Sprintf("Preview verification failed: write %s (%v)", path, err)}
		}
		writtenFiles++
	}
	if writtenFiles == 0 {
		return []string{"Preview verification failed: no frontend files were materialized for verification"}
	}

	issues := make([]string, 0, 3)
	if depIssues := validatePreviewManifestDependencies(manifest); len(depIssues) > 0 {
		for _, issue := range depIssues {
			issues = append(issues, "Preview verification dependency check failed: "+issue)
		}
		return issues
	}
	if importIssues := validateGeneratedImportDependencies(files, prefix, manifest); len(importIssues) > 0 {
		for _, issue := range importIssues {
			issues = append(issues, "Preview verification dependency check failed: "+issue)
		}
		return issues
	}
	if rootMismatch := detectFrontendTypeScriptBuildRootMismatch(files, prefix, manifest); rootMismatch != "" {
		return []string{"Preview verification failed: " + rootMismatch}
	}
	depCount := len(manifest.Dependencies) + len(manifest.DevDependencies)
	if depCount > 0 {
		if out, err := runPreviewCheckCommand(tmpDir, 2*time.Minute, "npm", "install", "--include=dev", "--no-audit", "--no-fund", "--prefer-offline"); err != nil {
			issues = append(issues, fmt.Sprintf("Preview verification install failed: %s", summarizePreviewInstallFailure(out)))
			return issues
		}
	}

	if out, err := runPreviewCheckCommand(tmpDir, 90*time.Second, "npm", "run", "build"); err != nil {
		issues = append(issues, fmt.Sprintf("Preview verification build failed: %s", summarizePreviewBuildFailure(out)))
		return issues
	}

	if buildUsesPreviewHTTPProbe(manifest.Scripts, forceHTTPProbe) {
		if msg := runPreviewHTTPProbe(tmpDir); msg != "" {
			issues = append(issues, msg)
		}
	}

	return issues
}

func (am *AgentManager) verifyGeneratedBackendBuildReadiness(files []GeneratedFile) []string {
	if len(files) == 0 {
		return nil
	}

	backendPackagePaths := []string{
		"backend/package.json",
		"api/package.json",
		"server/package.json",
		"apps/api/package.json",
		"apps/server/package.json",
		"packages/backend/package.json",
		"packages/api/package.json",
	}
	prefix := ""
	packagePath := ""
	for _, candidate := range backendPackagePaths {
		for _, f := range files {
			if strings.EqualFold(strings.TrimSpace(f.Path), candidate) {
				packagePath = f.Path
				prefix = strings.TrimSuffix(filepath.ToSlash(candidate), "package.json")
				break
			}
		}
		if packagePath != "" {
			break
		}
	}
	if packagePath == "" {
		return nil
	}

	var manifest previewManifest
	pkgContent := ""
	for _, f := range files {
		if f.Path == packagePath {
			pkgContent = f.Content
			break
		}
	}
	if strings.TrimSpace(pkgContent) == "" {
		return []string{fmt.Sprintf("Backend verification failed: %s is empty", packagePath)}
	}
	if err := json.Unmarshal([]byte(pkgContent), &manifest); err != nil {
		return []string{fmt.Sprintf("Backend verification failed: %s is invalid JSON (%v)", packagePath, err)}
	}
	if manifest.Scripts == nil {
		manifest.Scripts = map[string]string{}
	}
	if _, ok := manifest.Scripts["build"]; !ok {
		return []string{"Backend verification failed: backend package.json is missing a build script"}
	}

	if _, err := exec.LookPath("npm"); err != nil {
		return []string{"Backend verification failed: npm is not available on the build host"}
	}

	tmpDir, err := os.MkdirTemp("", "apex-backend-verify-*")
	if err != nil {
		return []string{fmt.Sprintf("Backend verification failed: unable to create temp dir (%v)", err)}
	}
	defer os.RemoveAll(tmpDir)

	writtenFiles := 0
	for _, f := range files {
		if strings.TrimSpace(f.Path) == "" || f.Content == "" {
			continue
		}
		path := filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(f.Path), "./"))
		if prefix != "" {
			if !strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix)) {
				continue
			}
			path = strings.TrimPrefix(path, prefix)
		}
		path = strings.TrimPrefix(path, "/")
		if path == "" || strings.Contains(path, "..") {
			continue
		}
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return []string{fmt.Sprintf("Backend verification failed: mkdir %s (%v)", path, err)}
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err != nil {
			return []string{fmt.Sprintf("Backend verification failed: write %s (%v)", path, err)}
		}
		writtenFiles++
	}
	if writtenFiles == 0 {
		return []string{"Backend verification failed: no backend files were materialized for verification"}
	}

	issues := make([]string, 0, 2)
	if depIssues := validatePreviewManifestDependencies(manifest); len(depIssues) > 0 {
		for _, issue := range depIssues {
			issues = append(issues, "Backend verification dependency check failed: "+issue)
		}
		return issues
	}
	if importIssues := validateGeneratedImportDependencies(files, prefix, manifest); len(importIssues) > 0 {
		for _, issue := range importIssues {
			issues = append(issues, "Backend verification dependency check failed: "+issue)
		}
		return issues
	}
	if rootMismatch := detectBackendTypeScriptBuildRootMismatch(files, prefix, manifest); rootMismatch != "" {
		return []string{"Backend verification failed: " + rootMismatch}
	}

	depCount := len(manifest.Dependencies) + len(manifest.DevDependencies)
	if depCount > 0 {
		if out, err := runPreviewCheckCommand(tmpDir, 2*time.Minute, "npm", "install", "--include=dev", "--no-audit", "--no-fund", "--prefer-offline"); err != nil {
			issues = append(issues, fmt.Sprintf("Backend verification install failed: %s", summarizePreviewInstallFailure(out)))
			return issues
		}
	}

	if out, err := runPreviewCheckCommand(tmpDir, 90*time.Second, "npm", "run", "build"); err != nil {
		issues = append(issues, fmt.Sprintf("Backend verification build failed: %s", summarizePreviewBuildFailure(out)))
		return issues
	}

	return nil
}

func buildUsesPreviewHTTPProbe(scripts map[string]string, force bool) bool {
	if scripts == nil {
		return false
	}
	if _, ok := scripts["preview"]; !ok {
		return false
	}
	if force {
		return true
	}
	val := strings.TrimSpace(strings.ToLower(os.Getenv("BUILD_PREVIEW_HTTP_SMOKE")))
	return val == "1" || val == "true" || val == "yes"
}

func runPreviewCheckCommand(workDir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return buf.String(), fmt.Errorf("%s timed out after %s", name, timeout)
	}
	return buf.String(), err
}

func runPreviewHTTPProbe(workDir string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Sprintf("Preview verification preview probe failed: unable to allocate local port (%v)", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "run", "preview", "--", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Dir = workDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("Preview verification preview start failed: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(20 * time.Second)
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return ""
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Sprintf("Preview verification preview probe failed: %s", truncatePreviewCommandOutput(out.String()))
}

func truncatePreviewCommandOutput(output string) string {
	s := strings.TrimSpace(output)
	if s == "" {
		return "command failed with no output"
	}
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

func validatePreviewManifestDependencies(manifest previewManifest) []string {
	issues := make([]string, 0, 4)
	seen := map[string]bool{}
	checkSet := func(kind string, deps map[string]string) {
		for name, version := range deps {
			trimmedName := strings.TrimSpace(name)
			if trimmedName == "" {
				issues = append(issues, fmt.Sprintf("%s contains an empty dependency name", kind))
				continue
			}
			if seen[kind+":"+trimmedName] {
				continue
			}
			seen[kind+":"+trimmedName] = true

			if !npmPackageNamePattern.MatchString(trimmedName) {
				issues = append(issues, fmt.Sprintf("%s contains invalid npm package name %q", kind, trimmedName))
			}
			if strings.Count(trimmedName, "/") > 1 {
				issues = append(issues, fmt.Sprintf("%s contains subpath import %q as a dependency (package.json must use package names only)", kind, trimmedName))
			}
			if strings.TrimSpace(version) == "" {
				issues = append(issues, fmt.Sprintf("%s dependency %q has an empty version", kind, trimmedName))
			}
		}
	}
	checkSet("dependencies", manifest.Dependencies)
	checkSet("devDependencies", manifest.DevDependencies)
	return dedupePreviewIssues(issues)
}

func dedupePreviewIssues(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, msg := range in {
		msg = strings.TrimSpace(msg)
		if msg == "" || seen[msg] {
			continue
		}
		seen[msg] = true
		out = append(out, msg)
	}
	return out
}

func summarizePreviewInstallFailure(output string) string {
	s := strings.TrimSpace(output)
	if s == "" {
		return "command failed with no output"
	}
	// npm 404 package not found is a common model hallucination; extract the package for a clearer error.
	if strings.Contains(s, "npm ERR! code E404") {
		if m := regexp.MustCompile(`404 Not Found - GET .*?/(@?[^\s]+) - Not found`).FindStringSubmatch(s); len(m) == 2 {
			pkg := strings.TrimSpace(m[1])
			if pkg != "" {
				return fmt.Sprintf("package not found on npm registry: %s", pkg)
			}
		}
	}
	// Prefer npm error lines over warning spam (deprecations are common and often not the actual cause).
	errLines := make([]string, 0, 6)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "npm ERR!") {
			errLines = append(errLines, line)
		}
	}
	if len(errLines) > 0 {
		return truncatePreviewCommandOutput(strings.Join(errLines, "\n"))
	}
	// Fallback: tail the output to capture the final failure context rather than leading warnings.
	lines := strings.Split(s, "\n")
	if len(lines) > 8 {
		return truncatePreviewCommandOutput(strings.Join(lines[len(lines)-8:], "\n"))
	}
	return truncatePreviewCommandOutput(s)
}

func summarizePreviewBuildFailure(output string) string {
	s := strings.TrimSpace(output)
	if s == "" {
		return "command failed with no output"
	}

	// Prefer actionable failure lines over noisy warnings (e.g. Vite deprecation banners).
	lines := strings.Split(s, "\n")
	matches := make([]string, 0, 8)
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "warning") || strings.Contains(lower, "deprecated") {
			continue
		}
		if strings.Contains(lower, "error") ||
			strings.Contains(line, "TS") ||
			strings.Contains(line, "ERR!") ||
			strings.Contains(lower, "cannot find module") ||
			strings.Contains(lower, "failed to resolve") ||
			strings.Contains(lower, "rolluperror") {
			matches = append(matches, line)
		}
	}
	if len(matches) > 0 {
		return truncatePreviewCommandOutput(strings.Join(matches, "\n"))
	}

	// Fall back to tail output rather than head so we capture the terminal failure context.
	if len(lines) > 10 {
		return truncatePreviewCommandOutput(strings.Join(lines[len(lines)-10:], "\n"))
	}
	return truncatePreviewCommandOutput(s)
}

func detectSourceArtifactAnomalies(path string, content string, ext string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	errors := make([]string, 0, 2)
	contentLower := strings.ToLower(content)

	// Explanatory prose frequently leaks into source outputs after the model finishes code generation.
	if strings.Contains(content, "This implementation includes:") ||
		strings.Contains(content, "Here is the implementation:") ||
		strings.Contains(content, "Explanation:") {
		errors = append(errors, fmt.Sprintf("%s contains explanatory prose appended to source code", path))
	}

	// Detect obvious frontend/backend framework mixing inside React component files.
	// These are almost never valid in a client entry/component and indicate model drift.
	if ext == ".tsx" || ext == ".jsx" {
		hasReact := strings.Contains(contentLower, "from 'react'") || strings.Contains(contentLower, "from \"react\"") ||
			strings.Contains(contentLower, "react.")
		hasExpress := strings.Contains(contentLower, "from 'express'") || strings.Contains(contentLower, "from \"express\"") ||
			strings.Contains(contentLower, "require('express')") || strings.Contains(contentLower, "require(\"express\")")
		if hasReact && hasExpress {
			errors = append(errors, fmt.Sprintf("%s mixes frontend React and backend Express code in the same source file", path))
		}
	}

	return errors
}

func detectBackendPersistenceStacks(path string, content string) []string {
	pathLower := strings.ToLower(path)
	contentLower := strings.ToLower(content)
	stacks := make([]string, 0, 4)
	add := func(name string) {
		for _, s := range stacks {
			if s == name {
				return
			}
		}
		stacks = append(stacks, name)
	}

	if strings.Contains(pathLower, "drizzle.config") ||
		strings.Contains(contentLower, "drizzle-orm") ||
		strings.Contains(contentLower, "drizzle-kit") {
		add("drizzle")
	}
	if strings.Contains(pathLower, "schema.prisma") ||
		strings.Contains(contentLower, "@prisma/client") ||
		strings.Contains(contentLower, "new prismaclient") {
		add("prisma")
	}
	if strings.Contains(contentLower, "from 'mongoose'") || strings.Contains(contentLower, "from \"mongoose\"") ||
		strings.Contains(contentLower, "new schema<") || strings.Contains(contentLower, "mongoose.connect(") {
		add("mongoose")
	}
	if strings.Contains(contentLower, "from 'sequelize'") || strings.Contains(contentLower, "from \"sequelize\"") ||
		strings.Contains(contentLower, "sequelize.define(") || strings.Contains(contentLower, "new sequelize(") {
		add("sequelize")
	}
	if strings.Contains(contentLower, "from 'typeorm'") || strings.Contains(contentLower, "from \"typeorm\"") ||
		strings.Contains(contentLower, "@entity(") || strings.Contains(contentLower, "datasource(") {
		add("typeorm")
	}
	return stacks
}

func summarizeMixedPersistenceStacks(stacks map[string]bool) string {
	if len(stacks) <= 1 {
		return ""
	}
	names := make([]string, 0, len(stacks))
	for name := range stacks {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Sprintf("Backend app mixes multiple persistence stacks (%s); generated app should use one database ORM/ODM stack consistently", strings.Join(names, ", "))
}

func hasUnresolvedPatchOrMergeMarkers(content string) bool {
	if content == "" {
		return false
	}

	// Common merge conflict markers.
	if strings.Contains(content, "<<<<<<<") || strings.Contains(content, ">>>>>>>") {
		return true
	}

	// Agent patch-style SEARCH/REPLACE marker blocks sometimes leak into output.
	// Require both terms plus a separator/marker signal to reduce false positives.
	hasSearch := strings.Contains(content, "SEARCH:") || strings.Contains(content, "<<<<<<< SEARCH")
	hasReplace := strings.Contains(content, "REPLACE:") || strings.Contains(content, ">>>>>>> REPLACE")
	if hasSearch && hasReplace {
		if strings.Contains(content, "=======") || strings.Contains(content, "<<<<<<< SEARCH") || strings.Contains(content, ">>>>>>> REPLACE") {
			return true
		}
	}

	return false
}

func analyzeFrontendPackageJSON(content string) (hasReact bool, hasReactDOM bool, isNext bool, hasScripts bool, missingScripts []string, err error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false, false, false, false, []string{"build", "dev|preview|start"}, fmt.Errorf("package.json is empty")
	}

	var pkg map[string]any
	if err := json.Unmarshal([]byte(trimmed), &pkg); err != nil {
		return false, false, false, false, []string{"build", "dev|preview|start"}, err
	}

	getObject := func(value any) map[string]any {
		obj, _ := value.(map[string]any)
		return obj
	}

	hasDependency := func(name string) bool {
		sections := []map[string]any{
			getObject(pkg["dependencies"]),
			getObject(pkg["devDependencies"]),
			getObject(pkg["peerDependencies"]),
		}
		for _, section := range sections {
			if section == nil {
				continue
			}
			if _, ok := section[name]; ok {
				return true
			}
		}
		return false
	}

	hasReact = hasDependency("react")
	hasReactDOM = hasDependency("react-dom")
	isNext = hasDependency("next")

	scripts := getObject(pkg["scripts"])
	hasNonEmptyScript := func(name string) bool {
		if scripts == nil {
			return false
		}
		raw, ok := scripts[name]
		value, isString := raw.(string)
		return ok && isString && strings.TrimSpace(value) != ""
	}

	missingScripts = make([]string, 0, 2)
	if !hasNonEmptyScript("build") {
		missingScripts = append(missingScripts, "build")
	}
	if isNext {
		if !hasNonEmptyScript("start") {
			missingScripts = append(missingScripts, "start")
		}
	} else {
		if !(hasNonEmptyScript("dev") || hasNonEmptyScript("preview") || hasNonEmptyScript("start")) {
			missingScripts = append(missingScripts, "dev|preview|start")
		}
	}
	hasScripts = len(missingScripts) == 0

	return hasReact, hasReactDOM, isNext, hasScripts, missingScripts, nil
}

// verifyGeneratedCode performs quick build verification on generated code
func (am *AgentManager) verifyGeneratedCode(buildID string, output *TaskOutput) (bool, []string) {
	if output == nil {
		return true, nil // Nothing to verify
	}

	errors := make([]string, 0)
	for _, msg := range output.Messages {
		if strings.Contains(strings.ToLower(msg), "unterminated code block") {
			errors = append(errors, fmt.Sprintf("AI response parsing warning: %s", msg))
		}
	}
	if len(output.Files) == 0 {
		return len(errors) == 0, errors
	}

	// Quick syntax checks on generated files
	for _, file := range output.Files {
		// Check for obvious problems
		fileErrors := am.quickSyntaxCheck(file)
		errors = append(errors, fileErrors...)
	}

	// Check for common generation failures
	for _, file := range output.Files {
		content := file.Content

		// Check for placeholder code
		placeholders := []string{
			"TODO",
			"FIXME",
			"// TODO:",
			"// FIXME:",
			"throw new Error('Not implemented')",
			"throw new Error(\"Not implemented\")",
			"raise NotImplementedError",
			"panic(\"not implemented\")",
			"// ... rest of implementation",
			"/* placeholder */",
			"[complete file content here]",
			"[complete code here]",
			"your implementation here",
		}
		for _, p := range placeholders {
			if strings.Contains(content, p) {
				errors = append(errors, fmt.Sprintf("%s: Contains placeholder code '%s'", file.Path, p))
			}
		}

		// Check for empty functions/methods
		if am.hasEmptyFunctions(content, file.Language) {
			errors = append(errors, fmt.Sprintf("%s: Contains empty function bodies", file.Path))
		}
	}

	return len(errors) == 0, errors
}

// quickSyntaxCheck performs language-specific syntax validation
func (am *AgentManager) quickSyntaxCheck(file GeneratedFile) []string {
	errors := make([]string, 0)
	content := file.Content

	switch file.Language {
	case "typescript", "javascript":
		// Avoid naive brace/import validation for JS/TS.
		// Template strings, JSX and side-effect imports trigger false positives and
		// cause unnecessary retry loops.
		if strings.Count(content, "```")%2 != 0 {
			errors = append(errors, fmt.Sprintf("%s: Contains unmatched markdown code fence", file.Path))
		}
		if tailErr := detectLikelyTruncatedJSTSFile(file.Path, content); tailErr != "" {
			errors = append(errors, tailErr)
		}

	case "go":
		// Only enforce package declaration for actual Go source files.
		if strings.HasSuffix(strings.ToLower(file.Path), ".go") && !strings.Contains(content, "package ") {
			errors = append(errors, fmt.Sprintf("%s: Missing package declaration", file.Path))
		}

	case "python":
		// Check for syntax issues - basic indentation check
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "\t") && i > 0 {
				prevLine := lines[i-1]
				if strings.HasPrefix(prevLine, "    ") {
					errors = append(errors, fmt.Sprintf("%s:%d: Mixed tabs and spaces", file.Path, i+1))
					break
				}
			}
		}

	case "html":
		contentLower := strings.ToLower(content)
		if strings.Contains(contentLower, "<html") && !strings.Contains(contentLower, "</html>") {
			errors = append(errors, fmt.Sprintf("%s: Incomplete HTML document (missing </html>)", file.Path))
		}
		if strings.Contains(contentLower, "<body") && !strings.Contains(contentLower, "</body>") {
			errors = append(errors, fmt.Sprintf("%s: Incomplete HTML document (missing </body>)", file.Path))
		}

	case "json":
		if !json.Valid([]byte(content)) {
			errors = append(errors, fmt.Sprintf("%s: Invalid JSON syntax", file.Path))
		}
	}

	return errors
}

func detectLikelyTruncatedJSTSFile(path, content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return ""
	}

	// Catch abrupt EOFs that commonly come from token-truncated model output.
	incompleteTailPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^(const|let|var|import|export|return|await|throw|new)\b\s*$`),
		regexp.MustCompile(`^(if|for|while|switch|catch)\b.*$`),
		regexp.MustCompile(`^(async\s+)?function\b.*$`),
	}
	for _, re := range incompleteTailPatterns {
		if re.MatchString(last) {
			return fmt.Sprintf("%s: Likely truncated source file (abrupt EOF after '%s')", path, last)
		}
	}

	if strings.HasSuffix(last, "=") || strings.HasSuffix(last, "=>") ||
		strings.HasSuffix(last, ".") || strings.HasSuffix(last, ",") ||
		strings.HasSuffix(last, ":") || strings.HasSuffix(last, "(") ||
		strings.HasSuffix(last, "[") {
		return fmt.Sprintf("%s: Likely truncated source file (abrupt EOF near '%s')", path, last)
	}

	return ""
}

func taskHasRecentTruncationError(task *Task) bool {
	if task == nil || len(task.ErrorHistory) == 0 {
		return false
	}
	last := strings.ToLower(task.ErrorHistory[len(task.ErrorHistory)-1].Error)
	return strings.Contains(last, "unterminated code block") ||
		strings.Contains(last, "likely truncated source file") ||
		strings.Contains(last, "abrupt eof")
}

// hasEmptyFunctions checks if content has empty function bodies
func (am *AgentManager) hasEmptyFunctions(content string, language string) bool {
	patterns := []string{}

	switch language {
	case "typescript", "javascript":
		patterns = []string{
			`function\s+\w+\s*\([^)]*\)\s*\{\s*\}`,
			`=>\s*\{\s*\}`,
			`async\s+function\s+\w+\s*\([^)]*\)\s*\{\s*\}`,
		}
	case "go":
		patterns = []string{
			`func\s+\w+\s*\([^)]*\)\s*\{[\s]*\}`,
			`func\s+\([^)]+\)\s+\w+\s*\([^)]*\)\s*\{[\s]*\}`,
		}
	case "python":
		patterns = []string{
			`def\s+\w+\s*\([^)]*\)\s*:\s*pass\s*$`,
			`def\s+\w+\s*\([^)]*\)\s*:\s*\.\.\.\s*$`,
		}
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}

	return false
}

// handleTaskFailure processes a failed task with intelligent retry logic
func (am *AgentManager) handleTaskFailure(agent *Agent, task *Task, result *TaskResult) {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	if task == nil {
		agent.Status = StatusError
		agent.Error = result.Error.Error()
		return
	}

	if result.Error != nil && (errors.Is(result.Error, errBuildNotActive) || errors.Is(result.Error, errBuildBudgetExceeded)) {
		task.Status = TaskCancelled
		task.Error = result.Error.Error()
		agent.Status = StatusError
		agent.Error = result.Error.Error()
		agent.UpdatedAt = time.Now()
		return
	}

	// Analyze error for smart retry strategy
	errorMsg := result.Error.Error()
	retryStrategy := am.determineRetryStrategy(errorMsg, task)
	insufficientCredits := isInsufficientCreditsErrorMessage(errorMsg)
	nonRetriable := am.isNonRetriableAIError(result.Error)
	if nonRetriable {
		retryStrategy = "non_retriable"
	}

	// Track error for learning
	task.ErrorHistory = append(task.ErrorHistory, ErrorAttempt{
		AttemptNumber: task.RetryCount + 1,
		Error:         errorMsg,
		Timestamp:     time.Now(),
		Context:       retryStrategy,
	})
	task.RetryCount++

	// Check if we should retry
	if task.RetryCount < task.MaxRetries && !nonRetriable {
		log.Printf("Task %s failed (attempt %d/%d, strategy: %s): %v. Retrying...",
			task.ID, task.RetryCount, task.MaxRetries, retryStrategy, result.Error)

		task.Status = TaskPending
		task.Error = ""
		task.RetryStrategy = RetryStrategy(retryStrategy)
		task.Input["previous_errors"] = task.ErrorHistory
		task.Input["retry_strategy"] = retryStrategy

		agent.Status = StatusWorking
		agent.Error = ""
		agent.UpdatedAt = time.Now()

		// Broadcast retry attempt
		am.broadcast(agent.BuildID, &WSMessage{
			Type:      "agent:retrying",
			BuildID:   agent.BuildID,
			AgentID:   agent.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"task_id":     result.TaskID,
				"attempt":     task.RetryCount,
				"retry_count": task.RetryCount,
				"max_retries": task.MaxRetries,
				"agent_role":  agent.Role,
				"strategy":    retryStrategy,
				"error":       errorMsg,
				"message":     fmt.Sprintf("Retrying with %s strategy (%d/%d)...", retryStrategy, task.RetryCount, task.MaxRetries),
				"provider":    agent.Provider,
				"model":       agent.Model,
			},
		})

		agent.mu.Unlock()
		am.taskQueue <- task
		agent.mu.Lock()
	} else {
		// Max retries exceeded
		finalMessage := "Task failed after multiple retry attempts. Consider breaking down the task or providing more guidance."
		if nonRetriable {
			finalMessage = "Task failed due to a non-retriable provider/model configuration error."
		}
		if insufficientCredits {
			finalMessage = insufficientCreditsBuildMessage
		}

		agent.Status = StatusError
		if insufficientCredits {
			agent.Error = insufficientCreditsBuildMessage
		} else {
			agent.Error = fmt.Sprintf("Failed after %d attempts: %s", task.RetryCount, errorMsg)
		}
		task.Status = TaskFailed
		task.Error = agent.Error
		agent.UpdatedAt = time.Now()

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
				"message":       finalMessage,
			},
		})

		am.enqueueRecoveryTask(agent.BuildID, task, result.Error)
		if build, err := am.GetBuild(agent.BuildID); err == nil {
			am.updateBuildProgress(build)
			am.checkBuildCompletion(build)
		}
	}
}

func (am *AgentManager) isNonRetriableAIError(err error) bool {
	if err == nil {
		return false
	}
	return am.isNonRetriableAIErrorMessage(err.Error())
}

func isInsufficientCreditsErrorMessage(errorMsg string) bool {
	return strings.Contains(strings.ToLower(errorMsg), "insufficient_credits")
}

func (am *AgentManager) isNonRetriableAIErrorMessage(errorMsg string) bool {
	errorLower := strings.ToLower(errorMsg)

	if strings.Contains(errorLower, "build not active") ||
		strings.Contains(errorLower, "build request budget exceeded") ||
		strings.Contains(errorLower, "insufficient_credits") {
		return true
	}

	// This is recoverable by endpoint/model fallback or provider switch.
	if strings.Contains(errorLower, "not a chat model") ||
		strings.Contains(errorLower, "v1/chat/completions endpoint") {
		return false
	}

	if strings.Contains(errorLower, "no ai providers available") ||
		strings.Contains(errorLower, "client not available for provider") ||
		strings.Contains(errorLower, "failed to select provider") {
		return true
	}

	modelNotFound := strings.Contains(errorLower, "model") &&
		(strings.Contains(errorLower, "not found") ||
			strings.Contains(errorLower, "not_found_error") ||
			strings.Contains(errorLower, "unsupported") ||
			strings.Contains(errorLower, "invalid") ||
			strings.Contains(errorLower, "unknown"))

	if modelNotFound || strings.Contains(errorLower, "unsupported for generatecontent") {
		return true
	}

	if strings.Contains(errorLower, "all_providers_failed") &&
		(modelNotFound || strings.Contains(errorLower, "unsupported for generatecontent")) {
		return true
	}

	return false
}

// determineRetryStrategy analyzes an error to pick the best retry approach
func (am *AgentManager) determineRetryStrategy(errorMsg string, task *Task) string {
	errorLower := strings.ToLower(errorMsg)

	if am.isNonRetriableAIErrorMessage(errorMsg) {
		return "non_retriable"
	}

	// Rate limiting - back off
	if strings.Contains(errorLower, "rate limit") || strings.Contains(errorLower, "too many requests") || strings.Contains(errorLower, "429") {
		return "backoff"
	}

	// Provider issues - try different provider
	if strings.Contains(errorLower, "service unavailable") || strings.Contains(errorLower, "503") ||
		strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "connection") {
		return "switch_provider"
	}

	// Context/token issues - simplify request
	if strings.Contains(errorLower, "context length") || strings.Contains(errorLower, "too long") ||
		strings.Contains(errorLower, "max tokens") {
		return "reduce_context"
	}

	// Build verification failures - fix and retry
	if strings.Contains(errorLower, "verification failed") || strings.Contains(errorLower, "build failed") ||
		strings.Contains(errorLower, "syntax error") || strings.Contains(errorLower, "compilation") {
		return "fix_and_retry"
	}

	// Default strategy
	return "standard_retry"
}

func (am *AgentManager) runFailureConsensus(
	build *Build,
	agent *Agent,
	task *Task,
	taskErr error,
	defaultStrategy string,
) (consensusDecision, []providerVote) {
	if build == nil || task == nil || taskErr == nil {
		return am.strategyToDecision(defaultStrategy), nil
	}

	available := am.aiRouter.GetAvailableProviders()
	majorProviders := []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini}
	selected := make([]ai.AIProvider, 0, 3)
	for _, preferred := range majorProviders {
		for _, p := range available {
			if p == preferred {
				selected = append(selected, p)
				break
			}
		}
	}
	if len(selected) == 0 {
		return am.strategyToDecision(defaultStrategy), nil
	}

	fallbackDecision := am.strategyToDecision(defaultStrategy)
	build.mu.RLock()
	description := build.Description
	build.mu.RUnlock()

	votes := make([]providerVote, 0, len(selected))
	for _, provider := range selected {
		ctx, cancel := context.WithTimeout(am.ctx, 45*time.Second)
		prompt := fmt.Sprintf(`You are participating in a build recovery incident vote.

Build context:
- App description: %s
- Failed task type: %s
- Failed task description: %s
- Agent role: %s
- Error: %s
- Default strategy: %s

Choose exactly ONE recovery action:
1) retry_same
2) switch_provider
3) spawn_solver
4) abort

Respond in this exact format:
VOTE: <retry_same|switch_provider|spawn_solver|abort>
RATIONALE: <single short sentence>`,
			description,
			task.Type,
			task.Description,
			agent.Role,
			taskErr.Error(),
			defaultStrategy,
		)

		resp, err := am.aiRouter.Generate(ctx, provider, prompt, GenerateOptions{
			UserID:          build.UserID,
			MaxTokens:       180,
			Temperature:     0.2,
			SystemPrompt:    "You are an incident commander. Vote for the safest path to complete the build.",
			PowerMode:       PowerFast,
			UsePlatformKeys: am.buildUsesPlatformKeys(build),
		})
		cancel()

		vote := providerVote{
			Provider: provider,
			Decision: fallbackDecision,
		}
		if err != nil {
			vote.Rationale = fmt.Sprintf("fallback vote due to provider error: %v", err)
		} else {
			decision, rationale := am.parseConsensusVote(resp.Content, fallbackDecision)
			vote.Decision = decision
			vote.Rationale = rationale
		}
		votes = append(votes, vote)
	}

	if len(votes) == 0 {
		return fallbackDecision, votes
	}

	counts := map[consensusDecision]int{}
	for _, vote := range votes {
		counts[vote.Decision]++
	}

	winning := fallbackDecision
	best := 0
	for decision, count := range counts {
		if count > best {
			best = count
			winning = decision
		}
	}

	majorityNeeded := 2
	if len(votes) == 2 {
		majorityNeeded = 2
	}
	if best < majorityNeeded {
		winning = fallbackDecision
	}

	summary := make([]string, 0, len(votes))
	for _, vote := range votes {
		summary = append(summary, fmt.Sprintf("%s=%s", vote.Provider, vote.Decision))
	}
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":              "incident_consensus",
			"message":            fmt.Sprintf("Provider vote: %s → %s", strings.Join(summary, ", "), winning),
			"consensus_decision": winning,
			"consensus_votes":    votes,
		},
	})
	am.broadcast(build.ID, &WSMessage{
		Type:      "build:phase",
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":   "Incident Consensus",
			"status":  string(BuildReviewing),
			"message": fmt.Sprintf("Team vote selected %s", winning),
		},
	})

	return winning, votes
}

func (am *AgentManager) strategyToDecision(strategy string) consensusDecision {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "switch_provider":
		return decisionSwitchProvider
	case "fix_and_retry", "standard_retry", "backoff", "reduce_context":
		return decisionRetrySame
	case "non_retriable":
		return decisionSpawnSolver
	case "abort":
		return decisionAbort
	default:
		return decisionRetrySame
	}
}

func (am *AgentManager) parseConsensusVote(content string, fallback consensusDecision) (consensusDecision, string) {
	normalized := strings.ToLower(content)
	vote := fallback
	switch {
	case strings.Contains(normalized, "vote: switch_provider"):
		vote = decisionSwitchProvider
	case strings.Contains(normalized, "vote: retry_same"):
		vote = decisionRetrySame
	case strings.Contains(normalized, "vote: spawn_solver"):
		vote = decisionSpawnSolver
	case strings.Contains(normalized, "vote: abort"):
		vote = decisionAbort
	}

	rationale := strings.TrimSpace(content)
	if len(rationale) > 220 {
		rationale = rationale[:220] + "..."
	}
	return vote, rationale
}

// getTemperatureForRole returns the optimal temperature for each agent role
func (am *AgentManager) getTemperatureForRole(role AgentRole) float64 {
	switch role {
	case RolePlanner:
		return 0.4 // Structured, deterministic plans
	case RoleArchitect:
		return 0.5 // Structured design with some creativity
	case RoleDatabase:
		return 0.3 // Precise schema design
	case RoleBackend:
		return 0.6 // Balanced code generation
	case RoleFrontend:
		return 0.7 // Creative UI work
	case RoleTesting:
		return 0.3 // Precise test assertions
	case RoleReviewer:
		return 0.4 // Analytical, precise review
	case RoleSolver:
		return 0.35 // Deterministic root-cause analysis and fixing
	case RoleLead:
		return 0.6 // Balanced communication
	default:
		return 0.7
	}
}

// getMaxTokensForRole returns the optimal max token limit for each agent role
func (am *AgentManager) getMaxTokensForRole(role AgentRole, powerMode ...PowerMode) int {
	// Base token limits per role
	var base int
	switch role {
	case RolePlanner:
		base = 4000 // Structured plan output
	case RoleArchitect:
		base = 6000 // Architecture documents
	case RoleFrontend:
		base = 12000 // Full React components with styling
	case RoleBackend:
		base = 12000 // Full API endpoints with middleware
	case RoleDatabase:
		base = 8000 // Schemas, migrations, seeds
	case RoleTesting:
		base = 8000 // Comprehensive test suites
	case RoleReviewer:
		base = 4000 // Concise review findings
	case RoleSolver:
		base = 10000 // Root-cause analysis plus corrected replacement files
	case RoleLead:
		base = 2000 // Conversation responses
	default:
		base = 8000
	}

	// Scale by power mode — users paying for Max get premium token budgets
	if len(powerMode) > 0 {
		switch powerMode[0] {
		case PowerMax:
			base = base * 2 // 2x tokens: Frontend/Backend get 24K, Architect 12K, etc.
		case PowerBalanced:
			base = base * 3 / 2 // 1.5x tokens: Frontend/Backend get 18K, etc.
		case PowerFast:
			// Keep base limits — fast mode optimizes for speed/cost
		}
	}

	return base
}

// getNextFallbackProvider returns the next provider in the fallback chain
func (am *AgentManager) getNextFallbackProvider(current ai.AIProvider) ai.AIProvider {
	chains := map[ai.AIProvider][]ai.AIProvider{
		ai.ProviderClaude: {ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderOllama},
		ai.ProviderGPT4:   {ai.ProviderClaude, ai.ProviderGemini, ai.ProviderOllama},
		ai.ProviderGemini: {ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderOllama},
		ai.ProviderOllama: {ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini},
	}
	if chain, ok := chains[current]; ok && len(chain) > 0 {
		return chain[0]
	}
	return current
}

// getCompletedTaskOutput finds the first completed task of a given type and returns a truncated summary
func (am *AgentManager) getCompletedTaskOutput(build *Build, taskType TaskType) string {
	build.mu.RLock()
	defer build.mu.RUnlock()

	for _, task := range build.Tasks {
		if task.Type == taskType && task.Status == TaskCompleted && task.Output != nil {
			// Collect file paths and messages
			var summary strings.Builder
			for _, file := range task.Output.Files {
				summary.WriteString(fmt.Sprintf("// File: %s (%s, %d bytes)\n", file.Path, file.Language, file.Size))
			}
			for _, msg := range task.Output.Messages {
				summary.WriteString(msg)
				summary.WriteString("\n")
			}
			result := summary.String()
			// Truncate to 3000 chars
			if len(result) > 3000 {
				result = result[:3000] + "\n... (truncated)"
			}
			return result
		}
	}
	return ""
}

// sanitizeFilePath prevents directory traversal attacks in generated file paths
func sanitizeFilePath(path string) string {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return ""
	}
	// Strip AI-generated annotations like "(root)", "(entry)", "(config)", etc.
	// These appear when the AI labels files with context hints e.g. "package.json (root)"
	if idx := strings.Index(cleaned, " ("); idx != -1 {
		if end := strings.Index(cleaned[idx:], ")"); end != -1 {
			cleaned = strings.TrimSpace(cleaned[:idx] + cleaned[idx+end+1:])
		}
	}
	// Normalize separators to avoid backslash traversal on Windows-style paths
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	// Reject absolute paths or drive letters
	if strings.HasPrefix(cleaned, "/") || (len(cleaned) > 1 && cleaned[1] == ':') {
		return ""
	}
	cleaned = filepath.Clean(cleaned)
	// Reject traversal after cleaning
	if cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return ""
	}
	return cleaned
}
