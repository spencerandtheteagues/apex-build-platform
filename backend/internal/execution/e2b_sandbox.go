package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// E2BSandbox provides E2B-managed microVM sandbox execution
type E2BSandbox struct {
	apiKey     string
	baseURL    string
	client     *http.Client
	executions map[string]*e2bExecution
	mu         sync.RWMutex
}

// e2bExecution tracks an active E2B sandbox execution
type e2bExecution struct {
	ID        string
	SandboxID string
	ClientID  string
	Language  string
	StartTime time.Time
}

// E2B API request/response types
type createSandboxRequest struct {
	TemplateID string `json:"templateID"`
	Timeout    int    `json:"timeout"`
}

type createSandboxResponse struct {
	SandboxID string `json:"sandboxID"`
	ClientID  string `json:"clientID"`
	EnvdPort  int    `json:"envdPort"`
}

type runCommandRequest struct {
	Cmd     string `json:"cmd"`
	Timeout int    `json:"timeout"`
}

type runCommandResponse struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// NewE2BSandbox creates a new E2B sandbox executor
func NewE2BSandbox(apiKey string) (*E2BSandbox, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("E2B API key is required")
	}

	return &E2BSandbox{
		apiKey:     apiKey,
		baseURL:    "https://api.e2b.dev",
		client:     &http.Client{Timeout: 120 * time.Second},
		executions: make(map[string]*e2bExecution),
	}, nil
}

// Execute runs code in an E2B sandbox
func (e *E2BSandbox) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	return e.ExecuteWithID(ctx, "", language, code, stdin)
}

// ExecuteWithID runs code in an E2B sandbox with a specific execution ID
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

	// Create E2B sandbox
	sandboxID, clientID, err := e.createSandbox(ctx)
	if err != nil {
		result.Status = "failed"
		result.ErrorOutput = fmt.Sprintf("Failed to create E2B sandbox: %v", err)
		result.ExitCode = 1
		return result, nil
	}

	// Track this execution
	exec := &e2bExecution{
		ID:        execID,
		SandboxID: sandboxID,
		ClientID:  clientID,
		Language:  language,
		StartTime: startTime,
	}

	e.mu.Lock()
	e.executions[execID] = exec
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.executions, execID)
		e.mu.Unlock()
		// Kill sandbox when done
		e.killSandbox(context.Background(), sandboxID)
	}()

	// Get file extension and setup command for language
	filename, setupCmd, runCmd, err := e.getLanguageCommands(language, code)
	if err != nil {
		result.Status = "failed"
		result.ErrorOutput = err.Error()
		result.ExitCode = 1
		return result, nil
	}

	// Upload code file
	if err := e.uploadFile(ctx, sandboxID, filename, code); err != nil {
		result.Status = "failed"
		result.ErrorOutput = fmt.Sprintf("Failed to upload code: %v", err)
		result.ExitCode = 1
		return result, nil
	}

	// Run setup command if needed
	if setupCmd != "" {
		setupResult, err := e.runCommand(ctx, sandboxID, setupCmd)
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

	// Run the code
	cmdResult, err := e.runCommand(ctx, sandboxID, runCmd)
	if err != nil {
		result.Status = "failed"
		result.ErrorOutput = fmt.Sprintf("Execution failed: %v", err)
		result.ExitCode = 1
		return result, nil
	}

	// Fill in the result
	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = time.Since(startTime)
	result.DurationMs = result.Duration.Milliseconds()
	result.Output = cmdResult.Stdout
	result.ErrorOutput = cmdResult.Stderr
	result.ExitCode = cmdResult.ExitCode

	if cmdResult.ExitCode == 0 {
		result.Status = "completed"
	} else {
		result.Status = "failed"
	}

	return result, nil
}

// ExecuteFile is not supported in E2B sandbox - use Execute with code content
func (e *E2BSandbox) ExecuteFile(ctx context.Context, filepath string, args []string, stdin string) (*ExecutionResult, error) {
	return nil, fmt.Errorf("file execution not supported in E2B sandbox - use Execute with code content")
}

// Kill terminates an E2B sandbox execution
func (e *E2BSandbox) Kill(execID string) error {
	e.mu.RLock()
	exec, exists := e.executions[execID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("execution %s not found", execID)
	}

	return e.killSandbox(context.Background(), exec.SandboxID)
}

// GetActiveExecutions returns the number of active executions
func (e *E2BSandbox) GetActiveExecutions() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.executions)
}

// Cleanup releases all sandbox resources
func (e *E2BSandbox) Cleanup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errs []error
	for id, exec := range e.executions {
		if err := e.killSandbox(context.Background(), exec.SandboxID); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup execution %s: %w", id, err))
		}
	}

	e.executions = make(map[string]*e2bExecution)

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// createSandbox creates a new E2B sandbox
func (e *E2BSandbox) createSandbox(ctx context.Context) (string, string, error) {
	reqBody := createSandboxRequest{
		TemplateID: "base",
		Timeout:    30,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/sandboxes", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("E2B API error: %d %s", resp.StatusCode, string(body))
	}

	var response createSandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	return response.SandboxID, response.ClientID, nil
}

// runCommand executes a command in the E2B sandbox
func (e *E2BSandbox) runCommand(ctx context.Context, sandboxID, command string) (*runCommandResponse, error) {
	reqBody := runCommandRequest{
		Cmd:     command,
		Timeout: 30,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/sandboxes/%s/commands", e.baseURL, sandboxID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("E2B API error: %d %s", resp.StatusCode, string(body))
	}

	var response runCommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// uploadFile uploads a file to the E2B sandbox
func (e *E2BSandbox) uploadFile(ctx context.Context, sandboxID, path, content string) error {
	url := fmt.Sprintf("%s/sandboxes/%s/files?path=%s", e.baseURL, sandboxID, path)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-API-Key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("E2B API error: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

// killSandbox terminates an E2B sandbox
func (e *E2BSandbox) killSandbox(ctx context.Context, sandboxID string) error {
	url := fmt.Sprintf("%s/sandboxes/%s", e.baseURL, sandboxID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("E2B API error: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

// getLanguageCommands returns the filename, setup command, and run command for a language
func (e *E2BSandbox) getLanguageCommands(language, code string) (string, string, string, error) {
	language = strings.ToLower(strings.TrimSpace(language))

	// Handle aliases
	aliases := map[string]string{
		"js":         "javascript",
		"node":       "javascript",
		"nodejs":     "javascript",
		"ts":         "typescript",
		"py":         "python",
		"python3":    "python",
		"golang":     "go",
		"rs":         "rust",
		"c++":        "cpp",
		"cplusplus":  "cpp",
		"rb":         "ruby",
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