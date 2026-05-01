// APEX.BUILD Container Sandbox Tests
// Security and functionality tests for container-based code execution

package execution

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("Docker-backed sandbox integration tests are skipped under -race")
	}

	config := DefaultContainerSandboxConfig()
	remoteHost := strings.HasPrefix(strings.ToLower(strings.TrimSpace(config.DockerHost)), "ssh://")
	if remoteHost && strings.EqualFold(strings.TrimSpace(os.Getenv("APEX_RUN_REMOTE_DOCKER_TESTS")), "true") {
		t.Log("running Docker sandbox test against configured remote Docker host")
	} else if remoteHost {
		t.Skip("remote Docker sandbox integration tests require APEX_RUN_REMOTE_DOCKER_TESTS=true")
	}
	sandbox := &ContainerSandbox{config: config}
	if !sandbox.checkDockerAvailable() {
		t.Skip("Docker not available, skipping container sandbox tests")
	}
}

func TestDefaultContainerSandboxConfigUsesRemoteDockerEnv(t *testing.T) {
	t.Setenv("APEX_EXECUTION_DOCKER_HOST", "ssh://apex-hostinger-runner")
	t.Setenv("APEX_EXECUTION_DOCKER_CONTEXT", "apex-hostinger")
	t.Setenv("APEX_PREVIEW_DOCKER_HOST", "")
	t.Setenv("APEX_PREVIEW_DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")

	config := DefaultContainerSandboxConfig()
	if config.DockerHost != "ssh://apex-hostinger-runner" {
		t.Fatalf("DockerHost = %q, want ssh://apex-hostinger-runner", config.DockerHost)
	}
	if config.DockerContext != "apex-hostinger" {
		t.Fatalf("DockerContext = %q, want apex-hostinger", config.DockerContext)
	}
}

func TestContainerSandboxDockerEnvUsesConfiguredRemoteHost(t *testing.T) {
	sandbox := &ContainerSandbox{
		config: &ContainerSandboxConfig{
			DockerHost:    "ssh://apex-hostinger-runner",
			DockerContext: "apex-hostinger",
		},
	}

	env := sandbox.dockerEnv()
	if got := executionEnvValue(env, "DOCKER_HOST"); got != "ssh://apex-hostinger-runner" {
		t.Fatalf("DOCKER_HOST = %q, want ssh://apex-hostinger-runner", got)
	}
	if got := executionEnvValue(env, "DOCKER_CONTEXT"); got != "apex-hostinger" {
		t.Fatalf("DOCKER_CONTEXT = %q, want apex-hostinger", got)
	}
}

func TestContainerSandboxDetectsRemoteDockerHost(t *testing.T) {
	sandbox := &ContainerSandbox{
		config: &ContainerSandboxConfig{
			DockerHost: "ssh://apex-hostinger-runner",
		},
	}

	if !sandbox.usesRemoteDocker() {
		t.Fatal("expected ssh docker host to be treated as remote")
	}
}

func TestBuildDockerArgsUsesNamedVolumeForRemoteWorkspace(t *testing.T) {
	sandbox := &ContainerSandbox{config: DefaultContainerSandboxConfig()}
	exec := &containerExecution{ID: "abcdef1234567890", Language: "javascript"}
	limits := &LanguageResourceLimits{
		MemoryLimit: 128 * 1024 * 1024,
		CPULimit:    0.5,
		PidsLimit:   64,
		TmpfsSize:   "32m",
	}

	args := sandbox.buildDockerArgs(exec, limits, "apex-sandbox-javascript:latest", containerRunOptions{
		MountVolume:   "apex-work-abcdef123456",
		MountReadOnly: true,
		WorkDir:       "/work",
		Command:       []string{"node", "main.js"},
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "type=volume,source=apex-work-abcdef123456,target=/work,readonly") {
		t.Fatalf("expected docker args to use readonly named volume, got %v", args)
	}
	if strings.Contains(joined, ":/work:") {
		t.Fatalf("expected docker args to avoid local bind mount for named volume, got %v", args)
	}
}

func TestBuildDockerArgsSkipsPackageCacheForRemoteDocker(t *testing.T) {
	sandbox := &ContainerSandbox{
		config: &ContainerSandboxConfig{
			DockerHost:          "ssh://apex-hostinger-runner",
			EnableReadOnlyRoot:  true,
			EnablePackageCache:  true,
			DisableNetwork:      true,
			DefaultMemoryLimit:  128 * 1024 * 1024,
			DefaultCPULimit:     0.5,
			DefaultPidsLimit:    64,
			TmpfsSize:           "32m",
			MaxConcurrentExecs:  1,
			LanguageLimits:      map[string]*LanguageResourceLimits{},
			EnableAuditLog:      false,
			ImagePrefix:         "apex-sandbox",
			DefaultTimeout:      30 * time.Second,
			DropAllCapabilities: true,
			NoNewPrivileges:     true,
		},
		pkgCache: NewPackageCacheManager(t.TempDir(), true),
	}
	exec := &containerExecution{ID: "abcdef1234567890", Language: "go"}
	limits := &LanguageResourceLimits{
		MemoryLimit: 128 * 1024 * 1024,
		CPULimit:    0.5,
		PidsLimit:   64,
		TmpfsSize:   "32m",
	}

	args := sandbox.buildDockerArgs(exec, limits, "apex-sandbox-go:latest", containerRunOptions{
		MountVolume: "apex-work-abcdef123456",
		WorkDir:     "/work",
		Command:     []string{"go", "run", "main.go"},
	})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "/cache/go-build") || strings.Contains(joined, "GOCACHE=/cache/go-build") {
		t.Fatalf("expected remote docker args to skip host package cache mounts, got %v", args)
	}
}

func TestRemoteWorkspaceVolumeNameSanitizesExecutionID(t *testing.T) {
	got := remoteWorkspaceVolumeName("Exec ID With Spaces/And Symbols!")
	if got != "apex-work-exec-id-with-spaces-and-symbols" {
		t.Fatalf("volume name = %q", got)
	}
}

func TestNewContainerSandbox(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false // Disable for tests

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create container sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	if sandbox == nil {
		t.Fatal("Sandbox should not be nil")
	}

	if !sandbox.dockerAvailable {
		t.Fatal("Docker should be available")
	}
}

func TestContainerSandboxUsesIsolatedTempRoots(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	first, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create first sandbox: %v", err)
	}
	defer first.Cleanup()

	second, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create second sandbox: %v", err)
	}
	defer second.Cleanup()

	if first.baseTempDir == second.baseTempDir {
		t.Fatalf("Expected unique temp roots, both sandboxes used %q", first.baseTempDir)
	}

	if _, err := os.Stat(first.baseTempDir); err != nil {
		t.Fatalf("First sandbox temp root missing: %v", err)
	}
	if _, err := os.Stat(second.baseTempDir); err != nil {
		t.Fatalf("Second sandbox temp root missing: %v", err)
	}

	if err := first.Cleanup(); err != nil {
		t.Fatalf("Failed to cleanup first sandbox: %v", err)
	}

	if _, err := os.Stat(second.baseTempDir); err != nil {
		t.Fatalf("Second sandbox temp root should survive first cleanup: %v", err)
	}
}

func TestContainerSandboxExecutePython(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	code := `print("Hello from Python sandbox!")`

	result, err := sandbox.Execute(ctx, "python", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'. Error: %s", result.Status, result.ErrorOutput)
	}

	if !strings.Contains(result.Output, "Hello from Python sandbox!") {
		t.Errorf("Expected output to contain 'Hello from Python sandbox!', got '%s'", result.Output)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
}

func TestContainerSandboxExecuteJavaScript(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	code := `console.log("Hello from JavaScript sandbox!");`

	result, err := sandbox.Execute(ctx, "javascript", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'. Error: %s", result.Status, result.ErrorOutput)
	}

	if !strings.Contains(result.Output, "Hello from JavaScript sandbox!") {
		t.Errorf("Expected output to contain 'Hello from JavaScript sandbox!', got '%s'", result.Output)
	}
}

func TestContainerSandboxExecuteGo(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	code := `
package main

import "fmt"

func main() {
	fmt.Println("Hello from Go sandbox!")
}
`

	result, err := sandbox.Execute(ctx, "go", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'. Error: %s", result.Status, result.ErrorOutput)
	}

	if !strings.Contains(result.Output, "Hello from Go sandbox!") {
		t.Errorf("Expected output to contain 'Hello from Go sandbox!', got '%s'", result.Output)
	}
}

func TestContainerSandboxExecuteWorkspaceCommand(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.py"), []byte(`
import os
print(os.getenv("PROJECT_GREETING", "missing"))
`), 0644); err != nil {
		t.Fatalf("Failed to write workspace file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sandbox.ExecuteWorkspaceCommand(
		ctx,
		"python",
		workspaceDir,
		`printf "ready" > out.txt && python3 main.py`,
		"",
		map[string]string{"PROJECT_GREETING": "workspace-command"},
	)
	if err != nil {
		t.Fatalf("Workspace execution failed: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("Expected status 'completed', got '%s'. Error: %s", result.Status, result.ErrorOutput)
	}

	if !strings.Contains(result.Output, "workspace-command") {
		t.Fatalf("Expected output to contain env value, got %q", result.Output)
	}

	outBytes, err := os.ReadFile(filepath.Join(workspaceDir, "out.txt"))
	if err != nil {
		t.Fatalf("Expected workspace command to create out.txt: %v", err)
	}
	if string(outBytes) != "ready" {
		t.Fatalf("Expected out.txt to contain ready, got %q", string(outBytes))
	}
}

func TestContainerSandboxTimeout(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false
	config.LanguageLimits["python"].Timeout = 5 * time.Second

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Code that runs forever
	code := `
import time
while True:
    time.sleep(1)
`

	result, err := sandbox.Execute(ctx, "python", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.Status != "timeout" {
		t.Errorf("Expected status 'timeout', got '%s'", result.Status)
	}

	if !result.TimedOut {
		t.Error("Expected TimedOut to be true")
	}
}

func TestContainerSandboxMemoryLimit(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false
	// Set low memory limit
	config.LanguageLimits["python"].MemoryLimit = 64 * 1024 * 1024 // 64MB

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Code that tries to allocate lots of memory
	code := `
data = []
for i in range(100):
    data.append("x" * (10 * 1024 * 1024))  # 10MB per iteration
print("Done")
`

	result, err := sandbox.Execute(ctx, "python", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Should fail or be killed due to memory limit
	if result.Status == "completed" && strings.Contains(result.Output, "Done") {
		t.Error("Expected memory limit to prevent completion")
	}
}

func TestContainerSandboxNetworkDisabled(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false
	config.DisableNetwork = true

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Code that tries to make a network request
	code := `
import urllib.request
try:
    response = urllib.request.urlopen('https://www.google.com', timeout=5)
    print("Network access allowed - SECURITY ISSUE!")
except Exception as e:
    print(f"Network blocked: {type(e).__name__}")
`

	result, err := sandbox.Execute(ctx, "python", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if strings.Contains(result.Output, "SECURITY ISSUE") {
		t.Error("Network access should be blocked in sandbox")
	}

	if !strings.Contains(result.Output, "Network blocked") && result.Status != "failed" {
		t.Logf("Output: %s, Error: %s", result.Output, result.ErrorOutput)
	}
}

func TestContainerSandboxReadOnlyFilesystem(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false
	config.EnableReadOnlyRoot = true

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Code that tries to write to root filesystem
	code := `
import os
try:
    with open('/etc/malicious', 'w') as f:
        f.write('pwned')
    print("Write allowed - SECURITY ISSUE!")
except Exception as e:
    print(f"Write blocked: {type(e).__name__}")
`

	result, err := sandbox.Execute(ctx, "python", code, "")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if strings.Contains(result.Output, "SECURITY ISSUE") {
		t.Error("Write to root filesystem should be blocked")
	}
}

func TestContainerSandboxStdin(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	code := `
name = input("Enter your name: ")
print(f"Hello, {name}!")
`

	result, err := sandbox.Execute(ctx, "python", code, "Claude\n")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'. Error: %s", result.Status, result.ErrorOutput)
	}

	if !strings.Contains(result.Output, "Hello, Claude!") {
		t.Errorf("Expected output to contain 'Hello, Claude!', got '%s'", result.Output)
	}
}

func TestContainerSandboxConcurrentExecutions(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false
	config.MaxConcurrentExecs = 5

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	// Run multiple executions concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	type result struct {
		idx    int
		result *ExecutionResult
		err    error
	}

	results := make(chan result, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			code := fmt.Sprintf(`print("Execution %c")`, rune('A'+idx))
			r, e := sandbox.Execute(ctx, "python", code, "")
			results <- result{idx: idx, result: r, err: e}
		}(i)
	}

	for i := 0; i < 3; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("Execution %d failed: %v", r.idx, r.err)
		}
		if r.result.Status != "completed" {
			t.Errorf("Execution %d: expected status 'completed', got '%s'", r.idx, r.result.Status)
		}
	}
}

func TestContainerSandboxKill(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	// Start a long-running execution
	ctx := context.Background()

	code := `
import time
for i in range(100):
    print(f"Iteration {i}")
    time.sleep(1)
`

	done := make(chan *ExecutionResult, 1)
	go func() {
		result, _ := sandbox.Execute(ctx, "python", code, "")
		done <- result
	}()

	// Wait for execution to start
	time.Sleep(3 * time.Second)

	// Get active executions
	activeExecs := sandbox.GetActiveExecutions()
	if activeExecs == 0 {
		t.Skip("Execution completed before we could kill it")
	}

	// Kill should work
	sandbox.executionsMu.RLock()
	var execID string
	for id := range sandbox.executions {
		execID = id
		break
	}
	sandbox.executionsMu.RUnlock()

	if execID != "" {
		err = sandbox.Kill(execID)
		if err != nil {
			t.Errorf("Kill failed: %v", err)
		}
	}

	// Wait for result
	select {
	case result := <-done:
		if result.Status != "killed" && result.Status != "timeout" {
			// Might be killed or might timeout depending on timing
			t.Logf("Result status: %s", result.Status)
		}
	case <-time.After(30 * time.Second):
		t.Error("Execution did not stop after kill")
	}
}

func TestContainerSandboxStats(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run a few executions
	sandbox.Execute(ctx, "python", `print("test1")`, "")
	sandbox.Execute(ctx, "python", `print("test2")`, "")
	sandbox.Execute(ctx, "python", `invalid python code`, "")

	stats := sandbox.GetStats()

	if stats.TotalExecutions < 3 {
		t.Errorf("Expected at least 3 total executions, got %d", stats.TotalExecutions)
	}

	if stats.SuccessfulExecs < 2 {
		t.Errorf("Expected at least 2 successful executions, got %d", stats.SuccessfulExecs)
	}

	if stats.FailedExecs < 1 {
		t.Errorf("Expected at least 1 failed execution, got %d", stats.FailedExecs)
	}
}

func TestPipeCommandsReturnsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	producer := exec.CommandContext(ctx, "sh", "-c", "while :; do printf 'workspace-data'; sleep 1; done")
	consumer := exec.CommandContext(ctx, "sh", "-c", "sleep 30")

	start := time.Now()
	err := pipeCommands(ctx, producer, consumer, "pack workspace", "stage workspace")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("pipeCommands returned too slowly after cancellation: %s", elapsed)
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected context deadline error, got %v", err)
	}
}

func TestRunCommandWithSoftDeadlineReturnsBeforeHungCommand(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30")

	start := time.Now()
	_, err := runCommandWithSoftDeadline(cmd, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected soft deadline error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("soft deadline returned too slowly: %s", elapsed)
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected context deadline error, got %v", err)
	}
}

func TestDockerStatus(t *testing.T) {
	status := CheckDockerStatus()

	if status.Available {
		if status.Version == "" {
			t.Error("Docker available but version is empty")
		}
		t.Logf("Docker version: %s, API: %s", status.Version, status.APIVersion)
	} else {
		t.Logf("Docker not available: %s", status.Error)
	}
}

func TestSandboxFactory(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultSandboxFactoryConfig()
	config.ContainerConfig.EnableAuditLog = false

	factory, err := NewSandboxFactory(config)
	if err != nil {
		t.Fatalf("Failed to create sandbox factory: %v", err)
	}
	defer factory.Cleanup()

	if !factory.IsContainerAvailable() {
		t.Error("Container sandbox should be available")
	}

	caps := factory.GetCapabilities()
	if !caps.ContainerIsolation {
		t.Error("Container isolation should be enabled")
	}

	if !caps.NetworkIsolation {
		t.Error("Network isolation should be enabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := factory.Execute(ctx, "python", `print("factory test")`, "")
	if err != nil {
		t.Fatalf("Factory execute failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}
}

// BenchmarkContainerSandboxExecution benchmarks container execution overhead
func BenchmarkContainerSandboxExecution(b *testing.B) {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		b.Skip("Docker not available")
	}

	config := DefaultContainerSandboxConfig()
	config.EnableAuditLog = false

	sandbox, err := NewContainerSandbox(config)
	if err != nil {
		b.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()

	ctx := context.Background()
	code := `print("benchmark")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sandbox.Execute(ctx, "python", code, "")
	}
}
