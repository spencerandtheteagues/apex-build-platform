package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// E2BSandbox provides E2B-managed microVM sandbox execution.
// Sandbox lifecycle and command execution are delegated to a small Node helper
// that uses the official E2B SDK, which stays compatible with the current API.
type E2BSandbox struct {
	apiKey        string
	helperCommand []string
	executions    map[string]*e2bExecution
	mu            sync.RWMutex
}

// e2bExecution tracks an active E2B sandbox execution.
type e2bExecution struct {
	ID        string
	SandboxID string
	Language  string
	StartTime time.Time
}

type e2bHelperRequest struct {
	Action    string `json:"action"`
	SandboxID string `json:"sandboxId,omitempty"`
	Command   string `json:"command,omitempty"`
	Path      string `json:"path,omitempty"`
	Content   string `json:"content,omitempty"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type e2bHelperResponse struct {
	SandboxID string `json:"sandboxId,omitempty"`
	ExitCode  int    `json:"exitCode,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	OK        bool   `json:"ok,omitempty"`
	Killed    bool   `json:"killed,omitempty"`
	Error     string `json:"error,omitempty"`
}

type runCommandResponse struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// NewE2BSandbox creates a new E2B sandbox executor.
func NewE2BSandbox(apiKey string) (*E2BSandbox, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("E2B API key is required")
	}

	helperCommand, err := defaultE2BHelperCommand()
	if err != nil {
		return nil, err
	}

	return &E2BSandbox{
		apiKey:        apiKey,
		helperCommand: helperCommand,
		executions:    make(map[string]*e2bExecution),
	}, nil
}

func defaultE2BHelperCommand() ([]string, error) {
	if override := strings.TrimSpace(os.Getenv("E2B_RUNNER_PATH")); override != "" {
		return []string{"node", override}, nil
	}

	candidates := make([]string, 0, 3)
	if executablePath, err := os.Executable(); err == nil && strings.TrimSpace(executablePath) != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), "e2b-runner", "exec.mjs"))
	}
	candidates = append(candidates, "e2b-runner/exec.mjs", "backend/e2b-runner/exec.mjs")

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return []string{"node", candidate}, nil
		}
	}

	return nil, fmt.Errorf("E2B runner not found; expected one of %s", strings.Join(candidates, ", "))
}

func (e *E2BSandbox) runHelper(ctx context.Context, request e2bHelperRequest) (*e2bHelperResponse, error) {
	if len(e.helperCommand) == 0 {
		return nil, fmt.Errorf("E2B helper command is not configured")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal helper request: %w", err)
	}

	cmd := exec.CommandContext(ctx, e.helperCommand[0], e.helperCommand[1:]...)
	cmd.Env = append(os.Environ(), "E2B_API_KEY="+e.apiKey)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			return nil, fmt.Errorf("E2B helper action %q failed: %w", request.Action, err)
		}
		return nil, fmt.Errorf("E2B helper action %q failed: %s", request.Action, detail)
	}

	var response e2bHelperResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to decode E2B helper response: %w", err)
	}
	if strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("%s", strings.TrimSpace(response.Error))
	}

	return &response, nil
}

// Execute runs code in an E2B sandbox.
func (e *E2BSandbox) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	return e.ExecuteWithID(ctx, "", language, code, stdin)
}

// ExecuteWithID runs code in an E2B sandbox with a specific execution ID.
func (e *E2BSandbox) ExecuteWithID(ctx context.Context, execID, language, code, stdin string) (*ExecutionResult, error) {
	if strings.TrimSpace(execID) == "" {
		execID = uuid.New().String()
	}

	startTime := time.Now()
	result := &ExecutionResult{
		ID:        execID,
		Language:  language,
		StartedAt: startTime,
		Status:    "running",
	}

	sandboxID, _, _, err := e.createSandbox(ctx)
	if err != nil {
		result.Status = "failed"
		result.ErrorOutput = fmt.Sprintf("Failed to create E2B sandbox: %v", err)
		result.ExitCode = 1
		return result, nil
	}

	execution := &e2bExecution{
		ID:        execID,
		SandboxID: sandboxID,
		Language:  language,
		StartTime: startTime,
	}

	e.mu.Lock()
	e.executions[execID] = execution
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.executions, execID)
		e.mu.Unlock()
		_ = e.killSandbox(context.Background(), sandboxID, "")
	}()

	filename, setupCmd, runCmd, err := e.getLanguageCommands(language, code)
	if err != nil {
		result.Status = "failed"
		result.ErrorOutput = err.Error()
		result.ExitCode = 1
		return result, nil
	}

	if err := e.uploadFile(ctx, sandboxID, "", filename, code); err != nil {
		result.Status = "failed"
		result.ErrorOutput = fmt.Sprintf("Failed to upload code: %v", err)
		result.ExitCode = 1
		return result, nil
	}

	if setupCmd != "" {
		setupResult, err := e.runCommand(ctx, sandboxID, "", setupCmd)
		if err != nil {
			result.Status = "failed"
			result.ErrorOutput = fmt.Sprintf("Setup failed: %v", err)
			result.ExitCode = 1
			return result, nil
		}
		if setupResult.ExitCode != 0 {
			result.Status = "failed"
			result.CompileError = setupResult.Stderr
			result.ErrorOutput = setupResult.Stderr
			result.ExitCode = setupResult.ExitCode
			return result, nil
		}
	}

	commandResult, err := e.runCommand(ctx, sandboxID, "", runCmd)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			result.Status = "timeout"
			result.ErrorOutput = ctxErr.Error()
			result.ExitCode = 124
			result.TimedOut = ctxErr == context.DeadlineExceeded
			result.Killed = !result.TimedOut
		} else {
			result.Status = "failed"
			result.ErrorOutput = fmt.Sprintf("Execution failed: %v", err)
			result.ExitCode = 1
		}
		return result, nil
	}

	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = time.Since(startTime)
	result.DurationMs = result.Duration.Milliseconds()
	result.Output = commandResult.Stdout
	result.ErrorOutput = commandResult.Stderr
	result.ExitCode = commandResult.ExitCode

	if commandResult.ExitCode == 0 {
		result.Status = "completed"
	} else {
		result.Status = "failed"
	}

	return result, nil
}

// ExecuteFile is not supported in E2B sandbox - use Execute with code content.
func (e *E2BSandbox) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	return nil, fmt.Errorf("file execution not supported in E2B sandbox - use Execute with code content")
}

// Kill terminates an E2B sandbox execution.
func (e *E2BSandbox) Kill(execID string) error {
	e.mu.RLock()
	execution, exists := e.executions[execID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("execution %s not found", execID)
	}

	return e.killSandbox(context.Background(), execution.SandboxID, "")
}

// GetActiveExecutions returns the number of active executions.
func (e *E2BSandbox) GetActiveExecutions() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.executions)
}

// Cleanup releases all sandbox resources.
func (e *E2BSandbox) Cleanup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errs []error
	for id, execution := range e.executions {
		if err := e.killSandbox(context.Background(), execution.SandboxID, ""); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup execution %s: %w", id, err))
		}
	}

	e.executions = make(map[string]*e2bExecution)

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// createSandbox creates a new E2B sandbox.
func (e *E2BSandbox) createSandbox(ctx context.Context) (string, string, string, error) {
	response, err := e.runHelper(ctx, e2bHelperRequest{
		Action:    "create",
		TimeoutMs: int((30 * time.Second).Milliseconds()),
	})
	if err != nil {
		return "", "", "", err
	}
	if strings.TrimSpace(response.SandboxID) == "" {
		return "", "", "", fmt.Errorf("E2B helper did not return a sandboxId")
	}
	return response.SandboxID, "", "", nil
}

// runCommand executes a command in the E2B sandbox.
func (e *E2BSandbox) runCommand(ctx context.Context, sandboxID, accessToken, command string) (*runCommandResponse, error) {
	response, err := e.runHelper(ctx, e2bHelperRequest{
		Action:    "run",
		SandboxID: sandboxID,
		Command:   command,
		TimeoutMs: int((30 * time.Second).Milliseconds()),
	})
	if err != nil {
		return nil, err
	}
	return &runCommandResponse{
		ExitCode: response.ExitCode,
		Stdout:   response.Stdout,
		Stderr:   response.Stderr,
	}, nil
}

// uploadFile uploads a file to the E2B sandbox via the SDK-backed helper.
func (e *E2BSandbox) uploadFile(ctx context.Context, sandboxID, accessToken, path, content string) error {
	response, err := e.runHelper(ctx, e2bHelperRequest{
		Action:    "write",
		SandboxID: sandboxID,
		Path:      path,
		Content:   content,
	})
	if err != nil {
		return err
	}
	if !response.OK {
		return fmt.Errorf("E2B helper did not confirm file write")
	}
	return nil
}

// killSandbox terminates an E2B sandbox.
func (e *E2BSandbox) killSandbox(ctx context.Context, sandboxID, accessToken string) error {
	response, err := e.runHelper(ctx, e2bHelperRequest{
		Action:    "kill",
		SandboxID: sandboxID,
	})
	if err != nil {
		return err
	}
	if !response.Killed {
		return fmt.Errorf("E2B helper did not confirm sandbox termination")
	}
	return nil
}

// getLanguageCommands returns the filename, setup command, and run command for a language.
func (e *E2BSandbox) getLanguageCommands(language, code string) (string, string, string, error) {
	language = strings.ToLower(strings.TrimSpace(language))

	aliases := map[string]string{
		"js":        "javascript",
		"node":      "javascript",
		"nodejs":    "javascript",
		"ts":        "typescript",
		"py":        "python",
		"python3":   "python",
		"golang":    "go",
		"rs":        "rust",
		"c++":       "cpp",
		"cplusplus": "cpp",
		"rb":        "ruby",
	}

	if alias, ok := aliases[language]; ok {
		language = alias
	}

	switch language {
	case "javascript":
		return "/code/main.js", "", "cd /code && node main.js", nil
	case "typescript":
		return "/code/main.ts", "cd /code && npm init -y && npm install -g typescript ts-node", "cd /code && ts-node main.ts", nil
	case "python":
		return "/code/main.py", "", "cd /code && python3 main.py", nil
	case "go":
		return "/code/main.go", "cd /code && go mod init main", "cd /code && go run main.go", nil
	case "rust":
		return "/code/main.rs", "cd /code && cargo init --name main", "cd /code && cargo run", nil
	case "java":
		return "/code/Main.java", "", "cd /code && javac Main.java && java Main", nil
	case "c":
		return "/code/main.c", "", "cd /code && gcc -o main main.c && ./main", nil
	case "cpp":
		return "/code/main.cpp", "", "cd /code && g++ -o main main.cpp && ./main", nil
	case "ruby":
		return "/code/main.rb", "", "cd /code && ruby main.rb", nil
	case "php":
		return "/code/main.php", "", "cd /code && php main.php", nil
	default:
		return "", "", "", fmt.Errorf("unsupported language: %s", language)
	}
}
