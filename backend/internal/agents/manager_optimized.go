// Package agents - Optimized Agent Manager
// Fixes memory leaks with proper context cancellation, subscriber cleanup,
// TTL-based eviction, and periodic cleanup goroutines

package agents

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// BuildTTL is the time-to-live for completed builds
	BuildTTL = 30 * time.Minute

	// InactiveBuildTTL is the TTL for builds with no activity
	InactiveBuildTTL = 15 * time.Minute

	// CleanupInterval is how often to run cleanup
	CleanupInterval = 5 * time.Minute

	// SubscriberBufferSize is the channel buffer size for subscribers
	SubscriberBufferSize = 100

	// MaxSubscribersPerBuild limits subscribers per build
	MaxSubscribersPerBuild = 50
)

// OptimizedAgentManager handles the lifecycle and coordination of AI agents
// with proper resource cleanup and memory management
type OptimizedAgentManager struct {
	agents      map[string]*Agent
	builds      map[string]*ManagedBuild
	taskQueue   chan *Task
	resultQueue chan *TaskResult
	subscribers map[string][]*managedSubscriber
	aiRouter    AIRouter
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// ManagedBuild extends Build with lifecycle management
type ManagedBuild struct {
	*Build

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	lastActive time.Time
	isEvicted  bool
	evictedAt  *time.Time

	// Resource tracking
	goroutineCount int
	subscriberCount int
}

// managedSubscriber wraps a subscriber channel with metadata
type managedSubscriber struct {
	ch        chan *WSMessage
	buildID   string
	createdAt time.Time
	lastUsed  time.Time
	closed    bool
	mu        sync.RWMutex
}

// NewOptimizedAgentManager creates a new optimized agent manager
func NewOptimizedAgentManager(aiRouter AIRouter) *OptimizedAgentManager {
	ctx, cancel := context.WithCancel(context.Background())

	am := &OptimizedAgentManager{
		agents:      make(map[string]*Agent),
		builds:      make(map[string]*ManagedBuild),
		taskQueue:   make(chan *Task, 100),
		resultQueue: make(chan *TaskResult, 100),
		subscribers: make(map[string][]*managedSubscriber),
		aiRouter:    aiRouter,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start background workers with proper tracking
	am.wg.Add(3)
	go am.taskDispatcherOptimized()
	go am.resultProcessorOptimized()
	go am.cleanupWorker()

	log.Println("Optimized Agent Manager initialized")
	return am
}

// CreateBuildOptimized creates a new build with proper resource management
func (am *OptimizedAgentManager) CreateBuildOptimized(userID uint, req *BuildRequest) (*ManagedBuild, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	buildID := uuid.New().String()
	now := time.Now()

	// Create build-specific context for cancellation
	buildCtx, buildCancel := context.WithCancel(am.ctx)

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

	managedBuild := &ManagedBuild{
		Build:      build,
		ctx:        buildCtx,
		cancel:     buildCancel,
		lastActive: now,
	}

	am.builds[buildID] = managedBuild

	log.Printf("Created managed build %s for user %d", buildID, userID)
	return managedBuild, nil
}

// SubscribeOptimized adds a channel to receive build updates with proper tracking
func (am *OptimizedAgentManager) SubscribeOptimized(buildID string, ch chan *WSMessage) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check subscriber limit
	if len(am.subscribers[buildID]) >= MaxSubscribersPerBuild {
		return fmt.Errorf("max subscribers reached for build %s", buildID)
	}

	subscriber := &managedSubscriber{
		ch:        ch,
		buildID:   buildID,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}

	am.subscribers[buildID] = append(am.subscribers[buildID], subscriber)

	// Update build subscriber count
	if build, exists := am.builds[buildID]; exists {
		build.subscriberCount++
	}

	return nil
}

// UnsubscribeOptimized removes a channel with proper cleanup
func (am *OptimizedAgentManager) UnsubscribeOptimized(buildID string, ch chan *WSMessage) {
	am.mu.Lock()
	defer am.mu.Unlock()

	subs := am.subscribers[buildID]
	for i, sub := range subs {
		if sub.ch == ch {
			// Mark as closed
			sub.mu.Lock()
			sub.closed = true
			sub.mu.Unlock()

			// Remove from slice
			am.subscribers[buildID] = append(subs[:i], subs[i+1:]...)

			// Update build subscriber count
			if build, exists := am.builds[buildID]; exists {
				build.subscriberCount--
			}
			break
		}
	}

	// Clean up empty subscriber slices
	if len(am.subscribers[buildID]) == 0 {
		delete(am.subscribers, buildID)
	}
}

// broadcastOptimized sends messages to subscribers with closed channel handling
func (am *OptimizedAgentManager) broadcastOptimized(buildID string, msg *WSMessage) {
	am.mu.RLock()
	subs := am.subscribers[buildID]
	am.mu.RUnlock()

	closedSubs := make([]*managedSubscriber, 0)

	for _, sub := range subs {
		sub.mu.RLock()
		if sub.closed {
			sub.mu.RUnlock()
			closedSubs = append(closedSubs, sub)
			continue
		}
		sub.mu.RUnlock()

		// Non-blocking send with timeout
		select {
		case sub.ch <- msg:
			sub.mu.Lock()
			sub.lastUsed = time.Now()
			sub.mu.Unlock()
		default:
			// Channel full, mark for cleanup
			closedSubs = append(closedSubs, sub)
		}
	}

	// Clean up closed/unresponsive subscribers
	if len(closedSubs) > 0 {
		am.mu.Lock()
		for _, closed := range closedSubs {
			am.removeSubscriberLocked(buildID, closed)
		}
		am.mu.Unlock()
	}
}

// removeSubscriberLocked removes a subscriber (must hold write lock)
func (am *OptimizedAgentManager) removeSubscriberLocked(buildID string, sub *managedSubscriber) {
	subs := am.subscribers[buildID]
	for i, s := range subs {
		if s == sub {
			am.subscribers[buildID] = append(subs[:i], subs[i+1:]...)
			if build, exists := am.builds[buildID]; exists {
				build.subscriberCount--
			}
			break
		}
	}

	if len(am.subscribers[buildID]) == 0 {
		delete(am.subscribers, buildID)
	}
}

// taskDispatcherOptimized processes tasks with proper context handling
func (am *OptimizedAgentManager) taskDispatcherOptimized() {
	defer am.wg.Done()

	for {
		select {
		case <-am.ctx.Done():
			log.Println("Task dispatcher shutting down")
			return
		case task, ok := <-am.taskQueue:
			if !ok {
				return
			}
			am.executeTaskWithContext(task)
		}
	}
}

// executeTaskWithContext runs a task with proper context cancellation
func (am *OptimizedAgentManager) executeTaskWithContext(task *Task) {
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

	// Get build context for cancellation
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

	// Use build context for cancellation
	ctx := build.ctx

	// Execute task with goroutine tracking
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()

		// Track goroutine
		build.mu.Lock()
		build.goroutineCount++
		build.lastActive = time.Now()
		build.mu.Unlock()

		defer func() {
			build.mu.Lock()
			build.goroutineCount--
			build.mu.Unlock()
		}()

		// Execute with context
		am.executeTaskCore(ctx, task, agent, build.Build)
	}()
}

// executeTaskCore is the core task execution logic
func (am *OptimizedAgentManager) executeTaskCore(ctx context.Context, task *Task, agent *Agent, build *Build) {
	// Check context before starting
	select {
	case <-ctx.Done():
		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   ctx.Err(),
		}
		return
	default:
	}

	// Build prompt
	prompt := am.buildTaskPromptFromBuild(task, build, agent)
	systemPrompt := am.getSystemPromptForRole(agent.Role)

	// Execute with timeout
	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	response, err := am.aiRouter.Generate(taskCtx, agent.Provider, prompt, GenerateOptions{
		MaxTokens:    8000,
		Temperature:  0.7,
		SystemPrompt: systemPrompt,
		PowerMode:    build.PowerMode,
	})

	if err != nil {
		// Check if error is due to cancellation
		if ctx.Err() != nil {
			am.resultQueue <- &TaskResult{
				TaskID:  task.ID,
				AgentID: agent.ID,
				Success: false,
				Error:   fmt.Errorf("task cancelled: %w", ctx.Err()),
			}
			return
		}

		am.resultQueue <- &TaskResult{
			TaskID:  task.ID,
			AgentID: agent.ID,
			Success: false,
			Error:   err,
		}
		return
	}

	// Parse output
	output := am.parseTaskOutputFromResponse(task.Type, response)

	am.resultQueue <- &TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output:  output,
	}
}

// resultProcessorOptimized handles results with proper context
func (am *OptimizedAgentManager) resultProcessorOptimized() {
	defer am.wg.Done()

	for {
		select {
		case <-am.ctx.Done():
			log.Println("Result processor shutting down")
			return
		case result, ok := <-am.resultQueue:
			if !ok {
				return
			}
			am.processResultCore(result)
		}
	}
}

// processResultCore handles result processing
func (am *OptimizedAgentManager) processResultCore(result *TaskResult) {
	am.mu.RLock()
	agent, exists := am.agents[result.AgentID]
	am.mu.RUnlock()

	if !exists {
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
	agent.mu.Unlock()

	// Broadcast result
	msgType := WSAgentCompleted
	if !result.Success {
		msgType = WSAgentError
	}

	am.broadcastOptimized(agent.BuildID, &WSMessage{
		Type:      msgType,
		BuildID:   agent.BuildID,
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"task_id": result.TaskID,
			"success": result.Success,
			"error":   getErrorString(result.Error),
		},
	})
}

// cleanupWorker periodically cleans up resources
func (am *OptimizedAgentManager) cleanupWorker() {
	defer am.wg.Done()

	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			log.Println("Cleanup worker shutting down")
			return
		case <-ticker.C:
			am.runCleanup()
		}
	}
}

// runCleanup performs cleanup of expired resources
func (am *OptimizedAgentManager) runCleanup() {
	now := time.Now()
	completedTTL := now.Add(-BuildTTL)
	inactiveTTL := now.Add(-InactiveBuildTTL)

	am.mu.Lock()
	defer am.mu.Unlock()

	buildsToEvict := make([]string, 0)
	subscribersToClean := make(map[string][]*managedSubscriber)

	// Find builds to evict
	for buildID, build := range am.builds {
		shouldEvict := false

		// Evict completed builds after TTL
		if build.Status == BuildCompleted || build.Status == BuildFailed {
			if build.CompletedAt != nil && build.CompletedAt.Before(completedTTL) {
				shouldEvict = true
			}
		}

		// Evict inactive builds
		if build.lastActive.Before(inactiveTTL) && build.subscriberCount == 0 {
			shouldEvict = true
		}

		// Evict builds with no active goroutines and completed status
		if build.goroutineCount == 0 && (build.Status == BuildCompleted || build.Status == BuildFailed) {
			if build.lastActive.Before(now.Add(-5 * time.Minute)) {
				shouldEvict = true
			}
		}

		if shouldEvict {
			buildsToEvict = append(buildsToEvict, buildID)
		}
	}

	// Find stale subscribers
	for buildID, subs := range am.subscribers {
		staleTimeout := now.Add(-10 * time.Minute)
		for _, sub := range subs {
			sub.mu.RLock()
			if sub.lastUsed.Before(staleTimeout) {
				if subscribersToClean[buildID] == nil {
					subscribersToClean[buildID] = make([]*managedSubscriber, 0)
				}
				subscribersToClean[buildID] = append(subscribersToClean[buildID], sub)
			}
			sub.mu.RUnlock()
		}
	}

	// Evict builds
	evictedCount := 0
	for _, buildID := range buildsToEvict {
		am.evictBuildLocked(buildID)
		evictedCount++
	}

	// Clean subscribers
	cleanedSubscribers := 0
	for buildID, subs := range subscribersToClean {
		for _, sub := range subs {
			am.removeSubscriberLocked(buildID, sub)
			cleanedSubscribers++
		}
	}

	if evictedCount > 0 || cleanedSubscribers > 0 {
		log.Printf("Cleanup: evicted %d builds, cleaned %d stale subscribers", evictedCount, cleanedSubscribers)
	}
}

// evictBuildLocked evicts a build and its resources (must hold write lock)
func (am *OptimizedAgentManager) evictBuildLocked(buildID string) {
	build, exists := am.builds[buildID]
	if !exists {
		return
	}

	// Cancel build context to stop any running goroutines
	build.cancel()

	// Mark as evicted
	build.isEvicted = true
	now := time.Now()
	build.evictedAt = &now

	// Remove agents
	for agentID := range build.Agents {
		delete(am.agents, agentID)
	}

	// Remove subscribers
	for _, sub := range am.subscribers[buildID] {
		sub.mu.Lock()
		sub.closed = true
		sub.mu.Unlock()
	}
	delete(am.subscribers, buildID)

	// Remove build
	delete(am.builds, buildID)

	log.Printf("Evicted build %s", buildID)
}

// CancelBuild cancels a build and cleans up resources
func (am *OptimizedAgentManager) CancelBuild(buildID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	build, exists := am.builds[buildID]
	if !exists {
		return fmt.Errorf("build %s not found", buildID)
	}

	// Cancel context
	build.cancel()

	// Update status
	build.Status = BuildCancelled
	build.UpdatedAt = time.Now()

	// Broadcast cancellation
	go am.broadcastOptimized(buildID, &WSMessage{
		Type:      "build:cancelled",
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status": "cancelled",
		},
	})

	return nil
}

// GetBuildOptimized retrieves a build with activity update
func (am *OptimizedAgentManager) GetBuildOptimized(buildID string) (*ManagedBuild, error) {
	am.mu.RLock()
	build, exists := am.builds[buildID]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("build %s not found", buildID)
	}

	// Update last active time
	build.mu.Lock()
	build.lastActive = time.Now()
	build.mu.Unlock()

	return build, nil
}

// GetStats returns manager statistics
func (am *OptimizedAgentManager) GetStats() ManagerStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	totalSubscribers := 0
	totalGoroutines := 0

	for _, subs := range am.subscribers {
		totalSubscribers += len(subs)
	}

	for _, build := range am.builds {
		totalGoroutines += build.goroutineCount
	}

	return ManagerStats{
		TotalBuilds:      len(am.builds),
		TotalAgents:      len(am.agents),
		TotalSubscribers: totalSubscribers,
		TotalGoroutines:  totalGoroutines,
		TaskQueueSize:    len(am.taskQueue),
		ResultQueueSize:  len(am.resultQueue),
	}
}

// ManagerStats holds manager statistics
type ManagerStats struct {
	TotalBuilds      int `json:"total_builds"`
	TotalAgents      int `json:"total_agents"`
	TotalSubscribers int `json:"total_subscribers"`
	TotalGoroutines  int `json:"total_goroutines"`
	TaskQueueSize    int `json:"task_queue_size"`
	ResultQueueSize  int `json:"result_queue_size"`
}

// ShutdownOptimized gracefully stops the agent manager
func (am *OptimizedAgentManager) ShutdownOptimized() {
	log.Println("Shutting down Optimized Agent Manager...")

	// Cancel all contexts
	am.cancel()

	// Close channels
	close(am.taskQueue)
	close(am.resultQueue)

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		am.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Optimized Agent Manager shut down cleanly")
	case <-time.After(30 * time.Second):
		log.Println("Optimized Agent Manager shutdown timed out")
	}
}

// Helper methods (stubs for interface compatibility)

func (am *OptimizedAgentManager) buildTaskPromptFromBuild(task *Task, build *Build, agent *Agent) string {
	return fmt.Sprintf("Task: %s\nDescription: %s\nApp: %s", task.Type, task.Description, build.Description)
}

func (am *OptimizedAgentManager) getSystemPromptForRole(role AgentRole) string {
	prompts := map[AgentRole]string{
		RoleLead:     "You are the Lead Agent coordinating the build.",
		RolePlanner:  "You are the Planning Agent creating detailed plans.",
		RoleArchitect: "You are the Architect Agent designing systems.",
		RoleFrontend: "You are the Frontend Agent building UIs.",
		RoleBackend:  "You are the Backend Agent creating APIs.",
		RoleDatabase: "You are the Database Agent designing schemas.",
		RoleTesting:  "You are the Testing Agent writing tests.",
		RoleReviewer: "You are the Reviewer Agent reviewing code.",
	}
	return prompts[role]
}

func (am *OptimizedAgentManager) parseTaskOutputFromResponse(taskType TaskType, response string) *TaskOutput {
	return &TaskOutput{
		Messages: []string{response},
		Files:    []GeneratedFile{},
	}
}

// Note: BuildCancelled is declared in types.go
