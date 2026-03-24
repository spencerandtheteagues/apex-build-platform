package execution

import (
	"context"
	"fmt"
	"sync"
)

// E2BProvider provides E2B sandbox instances on demand
type E2BProvider struct {
	apiKey    string
	sandboxes map[string]*E2BSandbox
	mu        sync.RWMutex
}

// NewE2BProvider creates a new E2B provider
func NewE2BProvider(apiKey string) (*E2BProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("E2B API key is required")
	}

	return &E2BProvider{
		apiKey:    apiKey,
		sandboxes: make(map[string]*E2BSandbox),
	}, nil
}

// GetSandbox returns a sandbox instance (creates one if needed)
func (p *E2BProvider) GetSandbox() (*E2BSandbox, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// For simplicity, create a new sandbox for each execution
	// In production, you might want to pool sandboxes
	sandbox, err := NewE2BSandbox(p.apiKey)
	if err != nil {
		return nil, err
	}

	return sandbox, nil
}

// Execute runs code using an E2B sandbox
func (p *E2BProvider) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	sandbox, err := p.GetSandbox()
	if err != nil {
		return nil, err
	}

	return sandbox.Execute(ctx, language, code, stdin)
}

// ExecuteWithID runs code using an E2B sandbox with a specific execution ID
func (p *E2BProvider) ExecuteWithID(ctx context.Context, execID, language, code, stdin string) (*ExecutionResult, error) {
	sandbox, err := p.GetSandbox()
	if err != nil {
		return nil, err
	}

	return sandbox.ExecuteWithID(ctx, execID, language, code, stdin)
}

// ExecuteFile is not supported in E2B
func (p *E2BProvider) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	return nil, fmt.Errorf("file execution not supported in E2B provider - use Execute with code content")
}

// Kill stops an execution (E2B sandboxes are ephemeral)
func (p *E2BProvider) Kill(execID string) error {
	// E2B sandboxes are created per execution and auto-cleaned up
	// Nothing to kill explicitly
	return nil
}

// GetActiveExecutions returns 0 since E2B executions are ephemeral
func (p *E2BProvider) GetActiveExecutions() int {
	return 0
}

// Cleanup releases all resources
func (p *E2BProvider) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clean up any tracked sandboxes
	var errs []error
	for id, sandbox := range p.sandboxes {
		if err := sandbox.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup sandbox %s: %w", id, err))
		}
	}

	p.sandboxes = make(map[string]*E2BSandbox)

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// GetStats returns statistics about the E2B provider
func (p *E2BProvider) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"provider":    "e2b",
		"api_key_set": p.apiKey != "",
		"sandboxes":   len(p.sandboxes),
	}
}

// E2BExecutorAdapter adapts E2BProvider to the CodeExecutor interface
type E2BExecutorAdapter struct {
	provider *E2BProvider
}

// NewE2BExecutorAdapter creates a new E2B executor adapter
func NewE2BExecutorAdapter(apiKey string) (*E2BExecutorAdapter, error) {
	provider, err := NewE2BProvider(apiKey)
	if err != nil {
		return nil, err
	}

	return &E2BExecutorAdapter{
		provider: provider,
	}, nil
}

// Execute runs code using E2B
func (a *E2BExecutorAdapter) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	return a.provider.Execute(ctx, language, code, stdin)
}

// ExecuteWithID runs code using E2B with specific execution ID
func (a *E2BExecutorAdapter) ExecuteWithID(ctx context.Context, execID, language, code, stdin string) (*ExecutionResult, error) {
	return a.provider.ExecuteWithID(ctx, execID, language, code, stdin)
}

// ExecuteFile is not supported
func (a *E2BExecutorAdapter) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	return a.provider.ExecuteFile(ctx, filepath, args, stdin)
}

// Kill stops an execution
func (a *E2BExecutorAdapter) Kill(execID string) error {
	return a.provider.Kill(execID)
}

// GetActiveExecutions returns the number of active executions
func (a *E2BExecutorAdapter) GetActiveExecutions() int {
	return a.provider.GetActiveExecutions()
}

// Cleanup releases all resources
func (a *E2BExecutorAdapter) Cleanup() error {
	return a.provider.Cleanup()
}

// Stats returns provider statistics
func (a *E2BExecutorAdapter) Stats() map[string]interface{} {
	return a.provider.GetStats()
}