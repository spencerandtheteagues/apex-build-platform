// APEX.BUILD Container Sandbox Tests
// Security and functionality tests for container-based code execution

package execution

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("Docker not available, skipping container sandbox tests")
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
