// APEX.BUILD Code Execution Sandbox
// Secure, isolated code execution environment with resource limits

package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// ExecutionResult contains the result of a code execution
type ExecutionResult struct {
	ID           string        `json:"id"`
	Status       string        `json:"status"` // running, completed, failed, timeout, killed
	Output       string        `json:"output"`
	ErrorOutput  string        `json:"error_output"`
	ExitCode     int           `json:"exit_code"`
	Duration     time.Duration `json:"duration_ms"`
	DurationMs   int64         `json:"duration_milliseconds"`
	MemoryUsed   int64         `json:"memory_used_bytes"`
	CPUTime      int64         `json:"cpu_time_ms"`
	Language     string        `json:"language"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	TimedOut     bool          `json:"timed_out"`
	Killed       bool          `json:"killed"`
	TempDir      string        `json:"-"`
	CompileError string        `json:"compile_error,omitempty"`
}

// SandboxConfig contains configuration for the execution sandbox
type SandboxConfig struct {
	// Timeout for execution (default 30 seconds)
	Timeout time.Duration

	// Memory limit in bytes (default 256MB)
	MemoryLimit int64

	// CPU time limit in seconds (default 30)
	CPULimit int64

	// Maximum output size in bytes (default 1MB)
	MaxOutputSize int64

	// Working directory for execution
	WorkDir string

	// Environment variables
	Environment map[string]string

	// Allow network access (default false for security)
	AllowNetwork bool

	// User ID to run as (for Unix systems)
	RunAsUser int

	// Group ID to run as (for Unix systems)
	RunAsGroup int

	// Enable strict isolation (firejail/bubblewrap if available)
	StrictIsolation bool
}

// DefaultSandboxConfig returns the default sandbox configuration
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		Timeout:         30 * time.Second,
		MemoryLimit:     256 * 1024 * 1024, // 256MB
		CPULimit:        30,                 // 30 seconds CPU time
		MaxOutputSize:   1024 * 1024,        // 1MB
		AllowNetwork:    false,
		RunAsUser:       -1,
		RunAsGroup:      -1,
		StrictIsolation: false,
	}
}

// Sandbox manages secure code execution
type Sandbox struct {
	config *SandboxConfig

	// Active executions
	executions     map[string]*activeExecution
	executionsMu   sync.RWMutex

	// Base temp directory for all executions
	baseTempDir string

	// Resource usage tracker
	resourceTracker *ResourceTracker
}

// activeExecution tracks a running execution
type activeExecution struct {
	ID        string
	Cmd       *exec.Cmd
	Cancel    context.CancelFunc
	StartTime time.Time
	TempDir   string
}

// ResourceTracker tracks resource usage across executions
type ResourceTracker struct {
	totalExecutions     int64
	totalCPUTime        int64
	totalMemoryUsed     int64
	concurrentExecs     int32
	maxConcurrentExecs  int32
	mu                  sync.Mutex
}

// NewSandbox creates a new execution sandbox
func NewSandbox(config *SandboxConfig) (*Sandbox, error) {
	if config == nil {
		config = DefaultSandboxConfig()
	}

	// Create base temp directory
	baseTempDir := filepath.Join(os.TempDir(), "apex-build-sandbox")
	if err := os.MkdirAll(baseTempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox temp directory: %w", err)
	}

	return &Sandbox{
		config:          config,
		executions:      make(map[string]*activeExecution),
		baseTempDir:     baseTempDir,
		resourceTracker: &ResourceTracker{maxConcurrentExecs: 100},
	}, nil
}

// Execute runs code in an isolated sandbox
func (s *Sandbox) Execute(ctx context.Context, language, code string, stdin string) (*ExecutionResult, error) {
	// Generate unique execution ID
	execID := uuid.New().String()

	// Create isolated temp directory for this execution
	tempDir, err := os.MkdirTemp(s.baseTempDir, fmt.Sprintf("exec-%s-", execID[:8]))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Ensure cleanup
	defer func() {
		if err := s.cleanup(tempDir); err != nil {
			fmt.Printf("Warning: failed to cleanup temp directory %s: %v\n", tempDir, err)
		}
	}()

	// Get the appropriate runner
	runner, err := GetRunner(language)
	if err != nil {
		return &ExecutionResult{
			ID:          execID,
			Status:      "failed",
			ErrorOutput: err.Error(),
			ExitCode:    1,
			Language:    language,
			StartedAt:   time.Now(),
		}, nil
	}

	// Write code to temp file
	filename, err := runner.WriteCode(tempDir, code)
	if err != nil {
		return &ExecutionResult{
			ID:          execID,
			Status:      "failed",
			ErrorOutput: fmt.Sprintf("Failed to write code: %v", err),
			ExitCode:    1,
			Language:    language,
			StartedAt:   time.Now(),
		}, nil
	}

	// Build the command
	cmd, compileErr := runner.BuildCommand(tempDir, filename)
	if compileErr != nil {
		return &ExecutionResult{
			ID:           execID,
			Status:       "failed",
			CompileError: compileErr.Error(),
			ErrorOutput:  compileErr.Error(),
			ExitCode:     1,
			Language:     language,
			StartedAt:    time.Now(),
		}, nil
	}

	// Execute with timeout
	return s.executeCommand(ctx, execID, cmd, tempDir, language, stdin)
}

// ExecuteFile runs a file in the sandbox
func (s *Sandbox) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	execID := uuid.New().String()

	// Detect language from file extension
	language := detectLanguageFromFile(filepath)
	if language == "" {
		return &ExecutionResult{
			ID:          execID,
			Status:      "failed",
			ErrorOutput: "Could not detect language from file extension",
			ExitCode:    1,
			StartedAt:   time.Now(),
		}, nil
	}

	// Get runner
	runner, err := GetRunner(language)
	if err != nil {
		return &ExecutionResult{
			ID:          execID,
			Status:      "failed",
			ErrorOutput: err.Error(),
			ExitCode:    1,
			Language:    language,
			StartedAt:   time.Now(),
		}, nil
	}

	// Create temp directory for any compiled output
	tempDir, err := os.MkdirTemp(s.baseTempDir, fmt.Sprintf("exec-%s-", execID[:8]))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer s.cleanup(tempDir)

	// Build command for the file
	cmd, compileErr := runner.BuildCommandForFile(filepath, tempDir, args)
	if compileErr != nil {
		return &ExecutionResult{
			ID:           execID,
			Status:       "failed",
			CompileError: compileErr.Error(),
			ErrorOutput:  compileErr.Error(),
			ExitCode:     1,
			Language:     language,
			StartedAt:    time.Now(),
		}, nil
	}

	return s.executeCommand(ctx, execID, cmd, tempDir, language, stdin)
}

// executeCommand runs a command with resource limits and timeout
func (s *Sandbox) executeCommand(ctx context.Context, execID string, cmd *exec.Cmd, tempDir, language, stdin string) (*ExecutionResult, error) {
	// Create execution context with timeout
	timeout := s.config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Track this execution
	activeExec := &activeExecution{
		ID:        execID,
		Cmd:       cmd,
		Cancel:    cancel,
		StartTime: time.Now(),
		TempDir:   tempDir,
	}

	s.executionsMu.Lock()
	s.executions[execID] = activeExec
	s.executionsMu.Unlock()

	defer func() {
		s.executionsMu.Lock()
		delete(s.executions, execID)
		s.executionsMu.Unlock()
	}()

	// Setup stdio capture
	var stdout, stderr bytes.Buffer
	stdoutWriter := &limitedWriter{w: &stdout, limit: s.config.MaxOutputSize}
	stderrWriter := &limitedWriter{w: &stderr, limit: s.config.MaxOutputSize}

	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	// Provide stdin if specified
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Set working directory
	if s.config.WorkDir != "" {
		cmd.Dir = s.config.WorkDir
	} else {
		cmd.Dir = tempDir
	}

	// Apply resource limits and isolation
	s.applyResourceLimits(cmd)

	// Set environment
	cmd.Env = s.buildEnvironment()

	// Start execution
	startTime := time.Now()
	result := &ExecutionResult{
		ID:        execID,
		Language:  language,
		StartedAt: startTime,
		TempDir:   tempDir,
	}

	if err := cmd.Start(); err != nil {
		result.Status = "failed"
		result.ErrorOutput = fmt.Sprintf("Failed to start process: %v", err)
		result.ExitCode = 1
		return result, nil
	}

	// Wait for completion or timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-execCtx.Done():
		// Timeout or cancellation
		if cmd.Process != nil {
			// Send SIGTERM first for graceful shutdown
			cmd.Process.Signal(syscall.SIGTERM)

			// Give it a moment to terminate gracefully
			select {
			case <-done:
				// Process terminated
			case <-time.After(2 * time.Second):
				// Force kill
				cmd.Process.Kill()
			}
		}

		completedAt := time.Now()
		result.CompletedAt = &completedAt
		result.Duration = time.Since(startTime)
		result.DurationMs = result.Duration.Milliseconds()
		result.Output = stdout.String()
		result.ErrorOutput = stderr.String()

		if execCtx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.TimedOut = true
			result.ExitCode = 124 // Standard timeout exit code
		} else {
			result.Status = "killed"
			result.Killed = true
			result.ExitCode = 137 // Standard killed exit code
		}

		return result, nil

	case err := <-done:
		// Process completed
		completedAt := time.Now()
		result.CompletedAt = &completedAt
		result.Duration = time.Since(startTime)
		result.DurationMs = result.Duration.Milliseconds()
		result.Output = stdout.String()
		result.ErrorOutput = stderr.String()

		if err != nil {
			// Check if it's an exit error
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
				result.Status = "failed"

				// Try to get resource usage
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						switch status.Signal() {
						case syscall.SIGKILL:
							result.Status = "killed"
							result.Killed = true
							result.ErrorOutput = "Process killed (possible memory limit exceeded)"
						case syscall.SIGXCPU:
							result.Status = "timeout"
							result.TimedOut = true
							result.ErrorOutput = "CPU time limit exceeded"
						}
					}
				}
			} else {
				result.Status = "failed"
				result.ExitCode = 1
				result.ErrorOutput = err.Error()
			}
		} else {
			result.Status = "completed"
			result.ExitCode = 0
		}

		// Get resource usage on Unix systems
		if cmd.ProcessState != nil {
			if rusage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
				result.MemoryUsed = rusage.Maxrss * 1024 // Convert KB to bytes
				result.CPUTime = rusage.Utime.Nano()/1e6 + rusage.Stime.Nano()/1e6
			}
		}

		return result, nil
	}
}

// applyResourceLimits applies resource limits to the command
func (s *Sandbox) applyResourceLimits(cmd *exec.Cmd) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}

	// Create a new process group so we can kill all children
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// On Linux, we can use prlimit for more precise control
	// For now, we use ulimit-style limits via shell wrapper
	if s.config.MemoryLimit > 0 || s.config.CPULimit > 0 {
		originalArgs := cmd.Args
		originalPath := cmd.Path

		// Memory limit in KB for ulimit
		memLimitKB := s.config.MemoryLimit / 1024
		if memLimitKB == 0 {
			memLimitKB = 256 * 1024 // 256MB default
		}

		// Build shell command with limits
		var limitCmd string
		if runtime.GOOS == "linux" {
			// Linux: use ulimit
			limitCmd = fmt.Sprintf("ulimit -v %d -t %d 2>/dev/null; ", memLimitKB, s.config.CPULimit)
		} else {
			// macOS: ulimit -v doesn't work well, use -m instead
			limitCmd = fmt.Sprintf("ulimit -m %d -t %d 2>/dev/null; ", memLimitKB, s.config.CPULimit)
		}

		// Escape the original command
		escapedArgs := make([]string, len(originalArgs))
		for i, arg := range originalArgs {
			escapedArgs[i] = escapeShellArg(arg)
		}
		fullCmd := limitCmd + strings.Join(escapedArgs, " ")

		cmd.Path = "/bin/bash"
		cmd.Args = []string{"bash", "-c", fullCmd}

		// Keep original for error messages
		_ = originalPath
	}
}

// buildEnvironment creates the environment for execution
func (s *Sandbox) buildEnvironment() []string {
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		"HOME=" + s.baseTempDir,
		"TMPDIR=" + s.baseTempDir,
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
	}

	// Add user-specified environment variables
	for k, v := range s.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Disable network if required (this is a hint; real isolation needs more)
	if !s.config.AllowNetwork {
		env = append(env, "no_proxy=*")
		env = append(env, "NO_PROXY=*")
	}

	return env
}

// Kill terminates a running execution
func (s *Sandbox) Kill(execID string) error {
	s.executionsMu.RLock()
	activeExec, exists := s.executions[execID]
	s.executionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("execution %s not found", execID)
	}

	// Cancel the context
	activeExec.Cancel()

	// Kill the process and its children
	if activeExec.Cmd.Process != nil {
		// Kill the process group
		pgid, err := syscall.Getpgid(activeExec.Cmd.Process.Pid)
		if err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			activeExec.Cmd.Process.Kill()
		}
	}

	return nil
}

// GetStatus returns the status of an execution
func (s *Sandbox) GetStatus(execID string) (string, bool) {
	s.executionsMu.RLock()
	_, exists := s.executions[execID]
	s.executionsMu.RUnlock()

	if exists {
		return "running", true
	}
	return "not_found", false
}

// GetActiveExecutions returns the number of active executions
func (s *Sandbox) GetActiveExecutions() int {
	s.executionsMu.RLock()
	defer s.executionsMu.RUnlock()
	return len(s.executions)
}

// cleanup removes temporary files and directories
func (s *Sandbox) cleanup(tempDir string) error {
	if tempDir == "" || !strings.HasPrefix(tempDir, s.baseTempDir) {
		return nil
	}

	// Give processes a moment to release files
	time.Sleep(100 * time.Millisecond)

	return os.RemoveAll(tempDir)
}

// Cleanup cleans up all sandbox resources
func (s *Sandbox) Cleanup() error {
	// Kill all active executions
	s.executionsMu.Lock()
	for id := range s.executions {
		s.Kill(id)
	}
	s.executionsMu.Unlock()

	// Remove base temp directory
	return os.RemoveAll(s.baseTempDir)
}

// limitedWriter wraps a writer and limits the amount of data written
type limitedWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written >= lw.limit {
		return len(p), nil // Silently discard
	}

	remaining := lw.limit - lw.written
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}

// escapeShellArg escapes a string for safe use in a shell command
func escapeShellArg(arg string) string {
	// Single quote the argument and escape any existing single quotes
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

// detectLanguageFromFile detects the programming language from file extension
func detectLanguageFromFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return ""
	}
}

// SupportedLanguage represents a supported programming language
type SupportedLanguage struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Extensions  []string `json:"extensions"`
	Compiled    bool     `json:"compiled"`
	Available   bool     `json:"available"`
	Description string   `json:"description"`
}

// GetSupportedLanguages returns all supported programming languages
func GetSupportedLanguages() []SupportedLanguage {
	languages := []SupportedLanguage{
		{
			ID:          "javascript",
			Name:        "JavaScript",
			Extensions:  []string{".js", ".mjs"},
			Compiled:    false,
			Description: "JavaScript/Node.js runtime",
		},
		{
			ID:          "typescript",
			Name:        "TypeScript",
			Extensions:  []string{".ts"},
			Compiled:    false,
			Description: "TypeScript with ts-node",
		},
		{
			ID:          "python",
			Name:        "Python",
			Extensions:  []string{".py"},
			Compiled:    false,
			Description: "Python 3 interpreter",
		},
		{
			ID:          "go",
			Name:        "Go",
			Extensions:  []string{".go"},
			Compiled:    true,
			Description: "Go programming language",
		},
		{
			ID:          "rust",
			Name:        "Rust",
			Extensions:  []string{".rs"},
			Compiled:    true,
			Description: "Rust programming language",
		},
		{
			ID:          "c",
			Name:        "C",
			Extensions:  []string{".c"},
			Compiled:    true,
			Description: "C with GCC compiler",
		},
		{
			ID:          "cpp",
			Name:        "C++",
			Extensions:  []string{".cpp", ".cc", ".cxx"},
			Compiled:    true,
			Description: "C++ with G++ compiler",
		},
		{
			ID:          "java",
			Name:        "Java",
			Extensions:  []string{".java"},
			Compiled:    true,
			Description: "Java with OpenJDK",
		},
		{
			ID:          "ruby",
			Name:        "Ruby",
			Extensions:  []string{".rb"},
			Compiled:    false,
			Description: "Ruby interpreter",
		},
		{
			ID:          "php",
			Name:        "PHP",
			Extensions:  []string{".php"},
			Compiled:    false,
			Description: "PHP interpreter",
		},
	}

	// Check version and availability for each language
	for i := range languages {
		languages[i].Version, languages[i].Available = checkLanguageAvailability(languages[i].ID)
	}

	return languages
}

// checkLanguageAvailability checks if a language runtime is available
func checkLanguageAvailability(language string) (version string, available bool) {
	var cmd *exec.Cmd

	switch language {
	case "javascript":
		cmd = exec.Command("node", "--version")
	case "typescript":
		cmd = exec.Command("npx", "ts-node", "--version")
	case "python":
		cmd = exec.Command("python3", "--version")
	case "go":
		cmd = exec.Command("go", "version")
	case "rust":
		cmd = exec.Command("rustc", "--version")
	case "c":
		cmd = exec.Command("gcc", "--version")
	case "cpp":
		cmd = exec.Command("g++", "--version")
	case "java":
		cmd = exec.Command("java", "-version")
	case "ruby":
		cmd = exec.Command("ruby", "--version")
	case "php":
		cmd = exec.Command("php", "--version")
	default:
		return "unknown", false
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "not installed", false
	}

	// Extract version from output
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		// Clean up version string
		version = strings.TrimSpace(lines[0])
		// Truncate if too long
		if len(version) > 50 {
			version = version[:50] + "..."
		}
	}

	return version, true
}
