package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
)

// ExecuteRequest describes a sandbox-v2 execution request.
type ExecuteRequest struct {
	ID        string
	ProjectID string
	Language  string
	Code      string
	Stdin     string
	Env       map[string]string
	Files     map[string]string
	Isolation IsolationMode
	Timeout   time.Duration
}

// ExecuteResult describes a sandbox-v2 execution result.
type ExecuteResult struct {
	ID          string        `json:"id"`
	Status      string        `json:"status"`
	Output      string        `json:"output"`
	ErrorOutput string        `json:"error_output"`
	ExitCode    int           `json:"exit_code"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
	DurationMs  int64         `json:"duration_ms"`
	TimedOut    bool          `json:"timed_out"`
	Killed      bool          `json:"killed"`
	ContainerID string        `json:"container_id,omitempty"`
	Image       string        `json:"image,omitempty"`
	Isolation   IsolationMode `json:"isolation"`
}

// Executor is the sandbox-v2 execution interface.
type Executor interface {
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error)
	Kill(execID string) error
	ActiveCount() int
	Stats() map[string]interface{}
	Close() error
}

// DockerExecutor runs sandbox-v2 executions using Docker SDK.
// gVisor runs through Docker runtime=runsc; Firecracker routes through an external proxy command.
type DockerExecutor struct {
	manager *Manager
	client  *client.Client

	mu        sync.RWMutex
	running   map[string]string
	cancels   map[string]context.CancelFunc
	startedAt map[string]time.Time
	total     int64
	success   int64
	failed    int64
	timedOut  int64
	killed    int64
	active    int64
}

// NewDockerExecutor creates a Docker SDK-backed sandbox-v2 executor.
func NewDockerExecutor(manager *Manager) (*DockerExecutor, error) {
	if manager == nil {
		return nil, fmt.Errorf("sandbox v2 manager is required")
	}

	cfg := manager.Config()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithHost(cfg.DockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker sdk client init failed: %w", err)
	}

	return &DockerExecutor{
		manager:   manager,
		client:    cli,
		running:   make(map[string]string),
		cancels:   make(map[string]context.CancelFunc),
		startedAt: make(map[string]time.Time),
	}, nil
}

// Execute runs a code snippet inside sandbox-v2.
func (e *DockerExecutor) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	if strings.TrimSpace(req.Language) == "" {
		return nil, fmt.Errorf("language is required")
	}
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	lang := normalizeLanguage(req.Language)
	template, ok := e.manager.GetTemplate(lang)
	if !ok {
		return nil, fmt.Errorf("sandbox v2 unsupported language: %s", req.Language)
	}

	isolation := req.Isolation
	if isolation == "" {
		isolation = e.manager.Config().DefaultIsolation
	}

	if isolation == IsolationFirecracker {
		return e.executeViaFirecrackerProxy(ctx, req, template)
	}

	return e.executeDocker(ctx, req, template, isolation)
}

func (e *DockerExecutor) executeDocker(ctx context.Context, req ExecuteRequest, template LanguageTemplate, isolation IsolationMode) (*ExecuteResult, error) {
	cfg := e.manager.Config()
	quota := e.manager.EffectiveQuota(req.Language)

	timeout := quota.Timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workspaceDir := filepath.Join(e.manager.WorkspaceRootForProject(req.ProjectID), req.ID)
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sandbox workspace: %w", err)
	}
	defer os.RemoveAll(workspaceDir)

	entryFile, runtimeEnv, err := writeWorkspaceFiles(workspaceDir, template, req)
	if err != nil {
		return nil, err
	}

	imageName := template.Image
	if imageName == "" {
		return nil, fmt.Errorf("missing image for language template %s", template.Language)
	}

	if cfg.PullImages {
		if err := e.ensureImage(execCtx, imageName); err != nil {
			return nil, err
		}
	}

	containerName := "apex-sandbox-v2-" + req.ID[:12]
	hostConfig, envList, err := e.buildHostAndEnv(req, template, runtimeEnv, workspaceDir, isolation, quota)
	if err != nil {
		return nil, err
	}

	cmd := renderCommandTemplate(template.CommandTemplate, entryFile)
	if len(cmd) == 0 {
		return nil, fmt.Errorf("language template %s has empty command", template.Language)
	}

	created, err := e.client.ContainerCreate(execCtx, &container.Config{
		Image:           imageName,
		WorkingDir:      template.WorkDir,
		Cmd:             cmd,
		Env:             envList,
		AttachStdout:    true,
		AttachStderr:    true,
		AttachStdin:     req.Stdin != "",
		OpenStdin:       req.Stdin != "",
		StdinOnce:       req.Stdin != "",
		Tty:             false,
		NetworkDisabled: !cfg.NetworkEnabled,
	}, hostConfig, &network.NetworkingConfig{}, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("docker container create failed: %w", err)
	}

	containerID := created.ID
	e.trackStart(req.ID, containerID, cancel)
	defer e.trackStop(req.ID)
	defer func() {
		_ = e.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	result := &ExecuteResult{
		ID:          req.ID,
		Status:      "running",
		StartedAt:   time.Now(),
		Image:       imageName,
		Isolation:   isolation,
		ContainerID: containerID,
	}

	if err := e.client.ContainerStart(execCtx, containerID, container.StartOptions{}); err != nil {
		e.markFailure(false, false)
		return nil, fmt.Errorf("docker container start failed: %w", err)
	}

	if req.Stdin != "" {
		if err := e.writeStdin(execCtx, containerID, req.Stdin); err != nil {
			// Non-fatal: execution may still succeed
			result.ErrorOutput = "stdin attach warning: " + err.Error()
		}
	}

	waitCh, errCh := e.client.ContainerWait(execCtx, containerID, container.WaitConditionNotRunning)
	var waitResp container.WaitResponse
	waitCompleted := false

	select {
	case <-execCtx.Done():
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			result.Status = "timeout"
			result.TimedOut = true
			result.ExitCode = 124
			_ = e.client.ContainerKill(context.Background(), containerID, "SIGKILL")
		} else {
			result.Status = "killed"
			result.Killed = true
			result.ExitCode = 137
			_ = e.client.ContainerKill(context.Background(), containerID, "SIGKILL")
		}
	case resp := <-waitCh:
		waitResp = resp
		waitCompleted = true
	case err := <-errCh:
		e.markFailure(false, false)
		return nil, fmt.Errorf("docker container wait failed: %w", err)
	}

	output, stderr, logErr := e.readLogs(context.Background(), containerID, quota.MaxOutputBytes)
	if logErr != nil {
		if result.ErrorOutput != "" {
			result.ErrorOutput += "\n"
		}
		result.ErrorOutput += "log read warning: " + logErr.Error()
	}
	result.Output = output
	if stderr != "" {
		if result.ErrorOutput != "" {
			result.ErrorOutput += "\n"
		}
		result.ErrorOutput += stderr
	}

	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = completedAt.Sub(result.StartedAt)
	result.DurationMs = result.Duration.Milliseconds()

	if result.Status == "running" {
		exitCode := 0
		if waitCompleted {
			exitCode = int(waitResp.StatusCode)
		}
		result.ExitCode = exitCode
		if exitCode == 0 {
			result.Status = "completed"
			e.markSuccess()
		} else {
			result.Status = "failed"
			e.markFailure(false, false)
		}
	} else {
		e.markFailure(result.TimedOut, result.Killed)
	}

	atomic.AddInt64(&e.total, 1)
	return result, nil
}

func (e *DockerExecutor) buildHostAndEnv(
	req ExecuteRequest,
	template LanguageTemplate,
	runtimeEnv map[string]string,
	workspaceDir string,
	isolation IsolationMode,
	quota ResourceQuota,
) (*container.HostConfig, []string, error) {
	cfg := e.manager.Config()

	if isolation == IsolationGVisor && cfg.GVisorRuntime == "" {
		return nil, nil, fmt.Errorf("gVisor isolation requested but SANDBOX_V2_GVISOR_RUNTIME is empty")
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: workspaceDir,
			Target: template.WorkDir,
		},
	}

	env := map[string]string{}
	for k, v := range template.Env {
		env[k] = v
	}
	for k, v := range runtimeEnv {
		env[k] = v
	}
	for k, v := range req.Env {
		env[k] = v
	}

	if cfg.EnablePackageCache {
		projectKey := req.ProjectID
		if projectKey == "" {
			projectKey = "shared"
		}
		for _, c := range template.CacheMounts {
			hostPath, err := e.manager.PackageCachePath(projectKey, c.Name)
			if err != nil {
				return nil, nil, fmt.Errorf("cache mount %s: %w", c.Name, err)
			}
			if hostPath == "" {
				continue
			}
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: hostPath,
				Target: c.ContainerPath,
			})
			for k, v := range c.Env {
				env[k] = v
			}
		}
	}

	securityOpt := []string{}
	if cfg.NoNewPrivileges {
		securityOpt = append(securityOpt, "no-new-privileges:true")
	}

	runtimeName := ""
	switch isolation {
	case IsolationDocker:
		runtimeName = ""
	case IsolationGVisor:
		runtimeName = cfg.GVisorRuntime
	default:
		return nil, nil, fmt.Errorf("unsupported docker isolation mode: %s", isolation)
	}
	if runtimeName != "" && !isAllowedRuntime(cfg.AllowedDockerRuntimes, runtimeName) {
		return nil, nil, fmt.Errorf("runtime %q is not allowed", runtimeName)
	}

	pidsLimit := quota.PidsLimit
	if pidsLimit <= 0 {
		pidsLimit = 128
	}
	memoryBytes := quota.MemoryBytes
	if memoryBytes <= 0 {
		memoryBytes = 256 * 1024 * 1024
	}
	nanoCPUs := int64(quota.CPUCores * 1_000_000_000)
	if nanoCPUs <= 0 {
		nanoCPUs = 500_000_000
	}

	hostCfg := &container.HostConfig{
		AutoRemove:     false,
		ReadonlyRootfs: cfg.ReadOnlyRootFS,
		SecurityOpt:    securityOpt,
		CapDrop:        []string{"ALL"},
		Runtime:        runtimeName,
		Mounts:         mounts,
		ShmSize:        cfg.DefaultSharedMemSize,
		NetworkMode:    "none",
		Tmpfs:          map[string]string{"/tmp": fmt.Sprintf("rw,noexec,nosuid,size=%s", cfg.DefaultTmpfsSize)},
		Resources: container.Resources{
			Memory:     memoryBytes,
			MemorySwap: memoryBytes,
			NanoCPUs:   nanoCPUs,
			PidsLimit:  &pidsLimit,
		},
	}
	if cfg.NetworkEnabled {
		hostCfg.NetworkMode = "bridge"
	}

	return hostCfg, flattenEnv(env), nil
}

func (e *DockerExecutor) ensureImage(ctx context.Context, imageName string) error {
	_, _, err := e.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil
	}
	rc, pullErr := e.client.ImagePull(ctx, imageName, image.PullOptions{})
	if pullErr != nil {
		return fmt.Errorf("pull image %s: %w (inspect err: %v)", imageName, pullErr, err)
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	return nil
}

func (e *DockerExecutor) writeStdin(ctx context.Context, containerID, stdin string) error {
	att, err := e.client.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return err
	}
	defer att.Close()
	if _, err := io.WriteString(att.Conn, stdin); err != nil {
		return err
	}
	if cw, ok := interface{}(att.Conn).(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
	return nil
}

func (e *DockerExecutor) readLogs(ctx context.Context, containerID string, limit int64) (string, string, error) {
	rc, err := e.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", "", err
	}
	defer rc.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&limitedWriter{w: &stdout, limit: limit}, &limitedWriter{w: &stderr, limit: limit}, rc)
	if err != nil {
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

func (e *DockerExecutor) executeViaFirecrackerProxy(ctx context.Context, req ExecuteRequest, template LanguageTemplate) (*ExecuteResult, error) {
	cfg := e.manager.Config()
	if strings.TrimSpace(cfg.FirecrackerProxyCmd) == "" {
		return nil, fmt.Errorf("firecracker isolation requested but SANDBOX_V2_FIRECRACKER_PROXY_CMD is not configured")
	}

	payload := map[string]interface{}{
		"id":       req.ID,
		"project":  req.ProjectID,
		"language": normalizeLanguage(req.Language),
		"code":     req.Code,
		"stdin":    req.Stdin,
		"env":      req.Env,
		"files":    req.Files,
		"template": template,
		"quota":    e.manager.EffectiveQuota(req.Language),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "sh", "-lc", cfg.FirecrackerProxyCmd)
	cmd.Stdin = bytes.NewReader(b)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("firecracker proxy failed: %w: %s", err, stderr.String())
	}

	var result ExecuteResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("firecracker proxy returned invalid JSON: %w", err)
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = startedAt
	}
	if result.CompletedAt == nil {
		t := time.Now()
		result.CompletedAt = &t
	}
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.DurationMs = result.Duration.Milliseconds()
	result.Isolation = IsolationFirecracker

	atomic.AddInt64(&e.total, 1)
	switch result.Status {
	case "completed":
		atomic.AddInt64(&e.success, 1)
	case "timeout":
		atomic.AddInt64(&e.timedOut, 1)
	case "killed":
		atomic.AddInt64(&e.killed, 1)
	default:
		atomic.AddInt64(&e.failed, 1)
	}
	return &result, nil
}

// Kill terminates a running execution by ID.
func (e *DockerExecutor) Kill(execID string) error {
	e.mu.RLock()
	containerID, ok := e.running[execID]
	cancel := e.cancels[execID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("sandbox v2 execution %s not found", execID)
	}
	if cancel != nil {
		cancel()
	}
	return e.client.ContainerKill(context.Background(), containerID, "SIGKILL")
}

// ActiveCount returns the number of tracked active executions.
func (e *DockerExecutor) ActiveCount() int {
	return int(atomic.LoadInt64(&e.active))
}

// Stats returns executor counters suitable for operational endpoints.
func (e *DockerExecutor) Stats() map[string]interface{} {
	return map[string]interface{}{
		"active":   atomic.LoadInt64(&e.active),
		"total":    atomic.LoadInt64(&e.total),
		"success":  atomic.LoadInt64(&e.success),
		"failed":   atomic.LoadInt64(&e.failed),
		"timeout":  atomic.LoadInt64(&e.timedOut),
		"killed":   atomic.LoadInt64(&e.killed),
		"backend":  "docker-sdk",
		"features": []string{"docker-sdk", "gvisor-runtime", "firecracker-proxy", "per-language-quotas", "package-cache-mounts"},
	}
}

// Close closes the Docker SDK client and cancels active executions.
func (e *DockerExecutor) Close() error {
	e.mu.RLock()
	ids := make([]string, 0, len(e.cancels))
	for id := range e.cancels {
		ids = append(ids, id)
	}
	e.mu.RUnlock()
	for _, id := range ids {
		_ = e.Kill(id)
	}
	return e.client.Close()
}

func (e *DockerExecutor) trackStart(execID, containerID string, cancel context.CancelFunc) {
	e.mu.Lock()
	e.running[execID] = containerID
	e.cancels[execID] = cancel
	e.startedAt[execID] = time.Now()
	e.mu.Unlock()
	atomic.AddInt64(&e.active, 1)
}

func (e *DockerExecutor) trackStop(execID string) {
	e.mu.Lock()
	delete(e.running, execID)
	delete(e.cancels, execID)
	delete(e.startedAt, execID)
	e.mu.Unlock()
	atomic.AddInt64(&e.active, -1)
}

func (e *DockerExecutor) markSuccess() {
	atomic.AddInt64(&e.success, 1)
}

func (e *DockerExecutor) markFailure(timeout, killed bool) {
	switch {
	case timeout:
		atomic.AddInt64(&e.timedOut, 1)
	case killed:
		atomic.AddInt64(&e.killed, 1)
	default:
		atomic.AddInt64(&e.failed, 1)
	}
}

func renderCommandTemplate(cmd []string, entryFile string) []string {
	out := make([]string, 0, len(cmd))
	for _, part := range cmd {
		out = append(out, strings.ReplaceAll(part, "{{file}}", entryFile))
	}
	return out
}

func writeWorkspaceFiles(workspaceDir string, template LanguageTemplate, req ExecuteRequest) (string, map[string]string, error) {
	files := map[string]string{}
	for k, v := range req.Files {
		files[k] = v
	}

	entryFile := template.FileName
	extraEnv := map[string]string{}
	if entryFile == "" {
		entryFile = "main.txt"
	}
	if req.Code != "" {
		var content string
		content, entryFile, extraEnv = normalizePrimaryCode(template, req.Code)
		files[entryFile] = content
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("no code or files provided")
	}

	for rel, content := range files {
		clean := filepath.Clean(rel)
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return "", nil, fmt.Errorf("invalid file path in request: %s", rel)
		}
		target := filepath.Join(workspaceDir, clean)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return "", nil, err
		}
	}

	return entryFile, extraEnv, nil
}

func normalizePrimaryCode(template LanguageTemplate, code string) (content, fileName string, env map[string]string) {
	lang := normalizeLanguage(template.Language)
	fileName = template.FileName
	env = map[string]string{}
	content = code

	switch lang {
	case "go":
		if !strings.Contains(code, "package ") {
			content = "package main\n\n" + code
		}
	case "rust":
		if !strings.Contains(code, "fn main") {
			content = "fn main() {\n" + code + "\n}\n"
		}
	case "c":
		if !strings.Contains(code, "#include") {
			content = "#include <stdio.h>\n#include <stdlib.h>\n\n" + code
		}
	case "cpp":
		if !strings.Contains(code, "#include") {
			content = "#include <iostream>\nusing namespace std;\n\n" + code
		}
	case "java":
		className := "Main"
		re := regexp.MustCompile(`public\s+class\s+([A-Za-z_][A-Za-z0-9_]*)`)
		if m := re.FindStringSubmatch(code); len(m) > 1 {
			className = m[1]
		} else if !strings.Contains(code, "class Main") {
			content = "public class Main {\n  public static void main(String[] args) {\n" + indentJava(code) + "\n  }\n}\n"
		}
		fileName = className + ".java"
		env["APEX_JAVA_CLASS"] = className
	}

	return content, fileName, env
}

func indentJava(code string) string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

func flattenEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func isAllowedRuntime(allowed []string, runtimeName string) bool {
	if runtimeName == "" {
		return true
	}
	for _, allowedName := range allowed {
		if allowedName == runtimeName {
			return true
		}
	}
	return false
}

type limitedWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.limit <= 0 {
		return lw.w.Write(p)
	}
	if lw.written >= lw.limit {
		return len(p), nil
	}
	remaining := lw.limit - lw.written
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += int64(n)
	if err != nil {
		return n, err
	}
	return len(p), nil
}
