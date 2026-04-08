package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	e2bPreviewWorkspaceDir = "/workspace"
	e2bPreviewSandboxTTL   = 30 * time.Minute
	e2bPreviewStartWait    = 60 * time.Second
	e2bPreviewCommandWait  = 10 * time.Second
)

type e2bPreviewRuntime struct {
	apiKey        string
	helperCommand []string
}

type e2bPreviewHelperRequest struct {
	Action    string   `json:"action"`
	SandboxID string   `json:"sandboxId,omitempty"`
	Command   string   `json:"command,omitempty"`
	Path      string   `json:"path,omitempty"`
	Content   string   `json:"content,omitempty"`
	TimeoutMs int      `json:"timeoutMs,omitempty"`
	Cwd       string   `json:"cwd,omitempty"`
	Env       []string `json:"env,omitempty"`
	PID       int      `json:"pid,omitempty"`
	Port      int      `json:"port,omitempty"`
}

type e2bPreviewHelperResponse struct {
	SandboxID string `json:"sandboxId,omitempty"`
	ExitCode  int    `json:"exitCode,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Error     string `json:"error,omitempty"`
	OK        bool   `json:"ok,omitempty"`
	Killed    bool   `json:"killed,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Host      string `json:"host,omitempty"`
	URL       string `json:"url,omitempty"`
}

func newRuntimeBackendFromEnv() (RuntimeBackend, error) {
	apiKey := strings.TrimSpace(os.Getenv("E2B_API_KEY"))
	if apiKey == "" {
		return nil, nil
	}
	return newE2BPreviewRuntime(apiKey)
}

func newE2BPreviewRuntime(apiKey string) (*e2bPreviewRuntime, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("E2B API key is required")
	}

	helperCommand, err := resolveE2BPreviewHelperCommand()
	if err != nil {
		return nil, err
	}

	return &e2bPreviewRuntime{
		apiKey:        apiKey,
		helperCommand: helperCommand,
	}, nil
}

func resolveE2BPreviewHelperCommand() ([]string, error) {
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

func (r *e2bPreviewRuntime) Name() string { return "e2b" }

func (r *e2bPreviewRuntime) RequiresLocalDependencyInstall() bool { return false }

func (r *e2bPreviewRuntime) StartProcess(cfg *ProcessStartConfig) (*ProcessHandle, error) {
	if cfg == nil {
		return nil, fmt.Errorf("process config is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	createResp, err := r.runHelper(ctx, e2bPreviewHelperRequest{
		Action:    "create",
		TimeoutMs: int(e2bPreviewSandboxTTL.Milliseconds()),
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(createResp.SandboxID) == "" {
		return nil, fmt.Errorf("E2B helper did not return a sandbox ID")
	}
	sandboxID := strings.TrimSpace(createResp.SandboxID)

	cleanupOnError := func(inner error) (*ProcessHandle, error) {
		_ = r.killSandbox(context.Background(), sandboxID)
		return nil, inner
	}

	if err := r.uploadWorkspace(ctx, sandboxID, cfg.Dir); err != nil {
		return cleanupOnError(fmt.Errorf("upload workspace: %w", err))
	}
	if err := r.installDependencies(ctx, sandboxID, cfg.Dir, cfg.Env); err != nil {
		return cleanupOnError(err)
	}

	command := shellJoin(cfg.Command, cfg.Args)
	port := extractPortFromEnv(cfg.Env)
	startResp, err := r.runHelper(ctx, e2bPreviewHelperRequest{
		Action:    "start",
		SandboxID: sandboxID,
		Command:   command,
		Cwd:       e2bPreviewWorkspaceDir,
		Env:       cfg.Env,
		Port:      port,
		TimeoutMs: int(e2bPreviewStartWait.Milliseconds()),
	})
	if err != nil {
		return cleanupOnError(fmt.Errorf("start preview process in E2B: %w", err))
	}
	if startResp.PID == 0 {
		return cleanupOnError(fmt.Errorf("E2B helper did not return a process id"))
	}

	stdout := io.NopCloser(strings.NewReader(""))
	stderr := io.NopCloser(strings.NewReader(""))
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			_, _ = r.runHelper(context.Background(), e2bPreviewHelperRequest{
				Action:    "kill_process",
				SandboxID: sandboxID,
				PID:       startResp.PID,
				TimeoutMs: int(e2bPreviewCommandWait.Milliseconds()),
			})
			_ = r.killSandbox(context.Background(), sandboxID)
		})
	}

	return &ProcessHandle{
		Pid:        startResp.PID,
		StdoutPipe: stdout,
		StderrPipe: stderr,
		ReadyURL:   strings.TrimSpace(startResp.URL),
		Wait: func() (int, error) {
			defer stop()
			waitResp, waitErr := r.runHelper(context.Background(), e2bPreviewHelperRequest{
				Action:    "wait",
				SandboxID: sandboxID,
				PID:       startResp.PID,
				TimeoutMs: int(e2bPreviewSandboxTTL.Milliseconds()),
			})
			if waitErr != nil {
				return 1, waitErr
			}
			if waitResp.ExitCode != 0 {
				errMsg := strings.TrimSpace(waitResp.Stderr)
				if errMsg == "" {
					errMsg = fmt.Sprintf("process exited with code %d", waitResp.ExitCode)
				}
				return waitResp.ExitCode, fmt.Errorf("%s", errMsg)
			}
			return waitResp.ExitCode, nil
		},
		SignalStop: stop,
		ForceKill:  stop,
	}, nil
}

func (r *e2bPreviewRuntime) installDependencies(ctx context.Context, sandboxID, localDir string, env []string) error {
	run := func(command string) error {
		resp, err := r.runHelper(ctx, e2bPreviewHelperRequest{
			Action:    "run",
			SandboxID: sandboxID,
			Command:   command,
			Cwd:       e2bPreviewWorkspaceDir,
			Env:       env,
			TimeoutMs: int((3 * time.Minute).Milliseconds()),
		})
		if err != nil {
			return err
		}
		if resp.ExitCode != 0 {
			msg := strings.TrimSpace(resp.Stderr)
			if msg == "" {
				msg = strings.TrimSpace(resp.Stdout)
			}
			if msg == "" {
				msg = fmt.Sprintf("command %q failed with exit code %d", command, resp.ExitCode)
			}
			return fmt.Errorf("%s", msg)
		}
		return nil
	}

	if fileExists(filepath.Join(localDir, "package.json")) {
		if err := run("npm install --prefer-offline --no-audit --no-fund --loglevel=error"); err != nil {
			return fmt.Errorf("remote npm install failed: %w", err)
		}
	}
	if fileExists(filepath.Join(localDir, "requirements.txt")) {
		if err := run("python3 -m pip install -r requirements.txt -q --break-system-packages"); err != nil {
			return fmt.Errorf("remote pip install failed: %w", err)
		}
	}
	if fileExists(filepath.Join(localDir, "go.mod")) {
		if err := run("go mod download"); err != nil {
			return fmt.Errorf("remote go mod download failed: %w", err)
		}
	}
	if fileExists(filepath.Join(localDir, "Cargo.toml")) {
		if err := run("cargo fetch"); err != nil {
			return fmt.Errorf("remote cargo fetch failed: %w", err)
		}
	}

	return nil
}

func (r *e2bPreviewRuntime) uploadWorkspace(ctx context.Context, sandboxID, localDir string) error {
	if strings.TrimSpace(localDir) == "" {
		return fmt.Errorf("workspace directory is required")
	}

	return filepath.WalkDir(localDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = r.runHelper(ctx, e2bPreviewHelperRequest{
			Action:    "write",
			SandboxID: sandboxID,
			Path:      pathJoin(e2bPreviewWorkspaceDir, relativePath),
			Content:   string(content),
			TimeoutMs: int(e2bPreviewCommandWait.Milliseconds()),
		})
		return err
	})
}

func (r *e2bPreviewRuntime) killSandbox(ctx context.Context, sandboxID string) error {
	if strings.TrimSpace(sandboxID) == "" {
		return nil
	}
	_, err := r.runHelper(ctx, e2bPreviewHelperRequest{
		Action:    "kill",
		SandboxID: sandboxID,
		TimeoutMs: int(e2bPreviewCommandWait.Milliseconds()),
	})
	return err
}

func (r *e2bPreviewRuntime) runHelper(ctx context.Context, request e2bPreviewHelperRequest) (*e2bPreviewHelperResponse, error) {
	if len(r.helperCommand) == 0 {
		return nil, fmt.Errorf("E2B helper command is not configured")
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal helper request: %w", err)
	}

	cmd := exec.CommandContext(ctx, r.helperCommand[0], r.helperCommand[1:]...)
	cmd.Env = append(os.Environ(), "E2B_API_KEY="+r.apiKey)
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

	var response e2bPreviewHelperResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("decode E2B helper response: %w", err)
	}
	if strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("%s", strings.TrimSpace(response.Error))
	}

	return &response, nil
}

func shellJoin(command string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	if strings.TrimSpace(command) != "" {
		parts = append(parts, shellQuote(command))
	}
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func extractPortFromEnv(env []string) int {
	for _, item := range env {
		if !strings.HasPrefix(item, "PORT=") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimPrefix(item, "PORT="))
		if err == nil && port > 0 {
			return port
		}
	}
	return 0
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func pathJoin(base, relative string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(base), "/")
	trimmedRelative := strings.TrimLeft(strings.TrimSpace(relative), "/")
	if trimmedBase == "" {
		return "/" + trimmedRelative
	}
	if trimmedRelative == "" {
		return trimmedBase
	}
	return trimmedBase + "/" + trimmedRelative
}
