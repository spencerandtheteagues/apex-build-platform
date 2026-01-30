// APEX.BUILD Sandbox Interface
// Provides a unified interface for code execution with automatic fallback

package execution

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// CodeExecutor defines the interface for code execution sandboxes
type CodeExecutor interface {
	// Execute runs code in the sandbox
	Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error)

	// ExecuteFile runs a file in the sandbox
	ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error)

	// Kill terminates a running execution
	Kill(execID string) error

	// GetActiveExecutions returns the count of active executions
	GetActiveExecutions() int

	// Cleanup releases all sandbox resources
	Cleanup() error
}

// SandboxType represents the type of sandbox to use
type SandboxType string

const (
	SandboxTypeAuto      SandboxType = "auto"
	SandboxTypeContainer SandboxType = "container"
	SandboxTypeProcess   SandboxType = "process"
)

// SandboxFactory creates and manages code execution sandboxes
type SandboxFactory struct {
	containerSandbox *ContainerSandbox
	processSandbox   *Sandbox
	preferContainer  bool
	mu               sync.RWMutex
	initialized      bool
}

// SandboxFactoryConfig configures the sandbox factory
type SandboxFactoryConfig struct {
	// PreferContainer attempts to use container sandbox when available
	PreferContainer bool

	// ContainerConfig for container-based execution
	ContainerConfig *ContainerSandboxConfig

	// ProcessConfig for process-based execution (fallback)
	ProcessConfig *SandboxConfig

	// ForceContainer fails if container sandbox is unavailable
	ForceContainer bool
}

// DefaultSandboxFactoryConfig returns a production-ready configuration
func DefaultSandboxFactoryConfig() *SandboxFactoryConfig {
	return &SandboxFactoryConfig{
		PreferContainer: true,
		ContainerConfig: DefaultContainerSandboxConfig(),
		ProcessConfig:   DefaultSandboxConfig(),
		ForceContainer:  false,
	}
}

var (
	globalFactory *SandboxFactory
	factoryOnce   sync.Once
)

// GetSandboxFactory returns the global sandbox factory instance
func GetSandboxFactory() *SandboxFactory {
	factoryOnce.Do(func() {
		globalFactory = &SandboxFactory{
			preferContainer: true,
		}
	})
	return globalFactory
}

// NewSandboxFactory creates a new sandbox factory
func NewSandboxFactory(config *SandboxFactoryConfig) (*SandboxFactory, error) {
	if config == nil {
		config = DefaultSandboxFactoryConfig()
	}

	factory := &SandboxFactory{
		preferContainer: config.PreferContainer,
	}

	// Try to initialize container sandbox if preferred
	if config.PreferContainer {
		containerSandbox, err := NewContainerSandbox(config.ContainerConfig)
		if err != nil {
			if config.ForceContainer {
				return nil, fmt.Errorf("container sandbox required but unavailable: %w", err)
			}
			// Log warning but continue with process sandbox
			fmt.Printf("Warning: container sandbox unavailable (%v), falling back to process sandbox\n", err)
		} else {
			factory.containerSandbox = containerSandbox
		}
	}

	// Always initialize process sandbox as fallback
	processSandbox, err := NewSandbox(config.ProcessConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize process sandbox: %w", err)
	}
	factory.processSandbox = processSandbox
	factory.initialized = true

	return factory, nil
}

// Initialize initializes the factory with default configuration
func (f *SandboxFactory) Initialize(config *SandboxFactoryConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.initialized {
		return nil
	}

	if config == nil {
		config = DefaultSandboxFactoryConfig()
	}

	// Try container sandbox
	if config.PreferContainer {
		containerSandbox, err := NewContainerSandbox(config.ContainerConfig)
		if err != nil {
			if config.ForceContainer {
				return fmt.Errorf("container sandbox required but unavailable: %w", err)
			}
			fmt.Printf("Container sandbox unavailable: %v\n", err)
		} else {
			f.containerSandbox = containerSandbox
			f.preferContainer = true
		}
	}

	// Initialize process sandbox
	processSandbox, err := NewSandbox(config.ProcessConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize process sandbox: %w", err)
	}
	f.processSandbox = processSandbox
	f.initialized = true

	return nil
}

// GetExecutor returns the appropriate code executor
func (f *SandboxFactory) GetExecutor(sandboxType SandboxType) (CodeExecutor, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.initialized {
		return nil, fmt.Errorf("sandbox factory not initialized")
	}

	switch sandboxType {
	case SandboxTypeContainer:
		if f.containerSandbox == nil {
			return nil, fmt.Errorf("container sandbox not available")
		}
		return &containerExecutorAdapter{f.containerSandbox}, nil

	case SandboxTypeProcess:
		return &processExecutorAdapter{f.processSandbox}, nil

	case SandboxTypeAuto:
		fallthrough
	default:
		if f.preferContainer && f.containerSandbox != nil {
			return &containerExecutorAdapter{f.containerSandbox}, nil
		}
		return &processExecutorAdapter{f.processSandbox}, nil
	}
}

// Execute runs code using the preferred sandbox
func (f *SandboxFactory) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	executor, err := f.GetExecutor(SandboxTypeAuto)
	if err != nil {
		return nil, err
	}
	return executor.Execute(ctx, language, code, stdin)
}

// ExecuteSecure runs code using container sandbox only (fails if unavailable)
func (f *SandboxFactory) ExecuteSecure(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	executor, err := f.GetExecutor(SandboxTypeContainer)
	if err != nil {
		return nil, fmt.Errorf("secure execution requires container sandbox: %w", err)
	}
	return executor.Execute(ctx, language, code, stdin)
}

// IsContainerAvailable returns whether container sandbox is available
func (f *SandboxFactory) IsContainerAvailable() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.containerSandbox != nil
}

// GetStats returns combined statistics from all sandboxes
func (f *SandboxFactory) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	stats := map[string]interface{}{
		"container_available": f.containerSandbox != nil,
		"prefer_container":    f.preferContainer,
	}

	if f.containerSandbox != nil {
		containerStats := f.containerSandbox.GetStats()
		stats["container"] = map[string]interface{}{
			"total_executions":     containerStats.TotalExecutions,
			"successful_executions": containerStats.SuccessfulExecs,
			"failed_executions":    containerStats.FailedExecs,
			"timeout_executions":   containerStats.TimeoutExecs,
			"killed_executions":    containerStats.KilledExecs,
			"concurrent_executions": containerStats.ConcurrentExecs,
			"max_concurrent":       containerStats.MaxConcurrentExecs,
		}
	}

	if f.processSandbox != nil {
		stats["process"] = map[string]interface{}{
			"active_executions": f.processSandbox.GetActiveExecutions(),
		}
	}

	return stats
}

// Cleanup releases all sandbox resources
func (f *SandboxFactory) Cleanup() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errs []error

	if f.containerSandbox != nil {
		if err := f.containerSandbox.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("container sandbox cleanup: %w", err))
		}
	}

	if f.processSandbox != nil {
		if err := f.processSandbox.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("process sandbox cleanup: %w", err))
		}
	}

	f.initialized = false

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// containerExecutorAdapter adapts ContainerSandbox to CodeExecutor interface
type containerExecutorAdapter struct {
	sandbox *ContainerSandbox
}

func (a *containerExecutorAdapter) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	return a.sandbox.Execute(ctx, language, code, stdin)
}

func (a *containerExecutorAdapter) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	// Container sandbox doesn't support file execution directly
	// Read file and execute as code
	return nil, fmt.Errorf("file execution not supported in container sandbox - use Execute with code content")
}

func (a *containerExecutorAdapter) Kill(execID string) error {
	return a.sandbox.Kill(execID)
}

func (a *containerExecutorAdapter) GetActiveExecutions() int {
	return a.sandbox.GetActiveExecutions()
}

func (a *containerExecutorAdapter) Cleanup() error {
	return a.sandbox.Cleanup()
}

// processExecutorAdapter adapts Sandbox to CodeExecutor interface
type processExecutorAdapter struct {
	sandbox *Sandbox
}

func (a *processExecutorAdapter) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	return a.sandbox.Execute(ctx, language, code, stdin)
}

func (a *processExecutorAdapter) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	return a.sandbox.ExecuteFile(ctx, filepath, args, stdin)
}

func (a *processExecutorAdapter) Kill(execID string) error {
	return a.sandbox.Kill(execID)
}

func (a *processExecutorAdapter) GetActiveExecutions() int {
	return a.sandbox.GetActiveExecutions()
}

func (a *processExecutorAdapter) Cleanup() error {
	return a.sandbox.Cleanup()
}

// DockerStatus represents the Docker daemon status
type DockerStatus struct {
	Available   bool   `json:"available"`
	Version     string `json:"version,omitempty"`
	APIVersion  string `json:"api_version,omitempty"`
	Error       string `json:"error,omitempty"`
}

// CheckDockerStatus checks if Docker is available and returns status
func CheckDockerStatus() DockerStatus {
	status := DockerStatus{}

	// Check if docker command exists
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		status.Error = "Docker not installed"
		return status
	}

	// Check docker version
	cmd := exec.Command(dockerPath, "version", "--format", "{{.Server.Version}}")
	output, err := cmd.Output()
	if err != nil {
		status.Error = fmt.Sprintf("Docker daemon not accessible: %v", err)
		return status
	}

	status.Available = true
	status.Version = string(output)

	// Get API version
	cmd = exec.Command(dockerPath, "version", "--format", "{{.Server.APIVersion}}")
	output, err = cmd.Output()
	if err == nil {
		status.APIVersion = string(output)
	}

	return status
}

// SandboxCapabilities describes what the sandbox can do
type SandboxCapabilities struct {
	ContainerIsolation bool     `json:"container_isolation"`
	NetworkIsolation   bool     `json:"network_isolation"`
	SeccompEnabled     bool     `json:"seccomp_enabled"`
	ReadOnlyRoot       bool     `json:"read_only_root"`
	ResourceLimits     bool     `json:"resource_limits"`
	SupportedLanguages []string `json:"supported_languages"`
}

// GetCapabilities returns the current sandbox capabilities
func (f *SandboxFactory) GetCapabilities() SandboxCapabilities {
	f.mu.RLock()
	defer f.mu.RUnlock()

	caps := SandboxCapabilities{
		ResourceLimits: true,
		SupportedLanguages: []string{
			"python", "javascript", "typescript", "go", "rust",
			"c", "cpp", "java", "ruby", "php",
		},
	}

	if f.containerSandbox != nil {
		caps.ContainerIsolation = true
		caps.NetworkIsolation = f.containerSandbox.config.DisableNetwork
		caps.SeccompEnabled = f.containerSandbox.config.EnableSeccomp
		caps.ReadOnlyRoot = f.containerSandbox.config.EnableReadOnlyRoot
	}

	return caps
}
