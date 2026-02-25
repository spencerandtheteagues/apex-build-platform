// Package core — contracts.go
//
// PUBLIC API CONTRACTS for integration between Claude Sonnet 4.6 half
// and Codex 5.3 half. These interfaces define the boundary.
//
// CODEX 5.3 IMPLEMENTS:
//   - SandboxManager (sandbox v2, per-project infra, container lifecycle)
//   - CheckpointStore (Postgres-backed persistence for FSM checkpoints)
//   - SmokeTestRunner (execute tests inside sandbox)
//   - DeploymentService (push to cloud)
//
// CLAUDE SONNET 4.6 PROVIDES:
//   - AgentFSM (state machine engine)
//   - BuildValidator (placeholder scan, syntax check, validation)
//   - GuaranteeEngine (retry/rollback orchestration)
//   - All frontend UI components
package core

import (
	"context"
	"time"
)

// --- SandboxManager Contract ---
// Codex 5.3 implements this. Claude's GuaranteeEngine calls it.

// SandboxManager controls isolated execution environments.
type SandboxManager interface {
	// CreateSandbox provisions a new sandbox for a build.
	CreateSandbox(ctx context.Context, cfg SandboxConfig) (SandboxInstance, error)

	// DestroySandbox tears down a sandbox and reclaims resources.
	DestroySandbox(ctx context.Context, sandboxID string) error

	// ExecInSandbox runs a command inside a sandbox.
	ExecInSandbox(ctx context.Context, sandboxID string, cmd string, timeout time.Duration) (ExecResult, error)

	// WriteFile writes content to a file inside the sandbox.
	WriteFile(ctx context.Context, sandboxID string, path string, content []byte) error

	// ReadFile reads content from a file inside the sandbox.
	ReadFile(ctx context.Context, sandboxID string, path string) ([]byte, error)

	// SnapshotSandbox captures the current sandbox state for rollback.
	SnapshotSandbox(ctx context.Context, sandboxID string) (string, error) // returns snapshot ID

	// RestoreSandbox restores a sandbox to a previous snapshot.
	RestoreSandbox(ctx context.Context, sandboxID string, snapshotID string) error
}

// SandboxConfig defines parameters for creating a sandbox.
type SandboxConfig struct {
	ProjectID   string            `json:"project_id"`
	Language    string            `json:"language"`
	Framework   string            `json:"framework,omitempty"`
	Image       string            `json:"image,omitempty"`     // Docker image
	MemoryMB    int               `json:"memory_mb"`
	CPUMillis   int               `json:"cpu_millis"`
	TimeoutSec  int               `json:"timeout_sec"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	MountPaths  []string          `json:"mount_paths,omitempty"`
}

// SandboxInstance represents a running sandbox.
type SandboxInstance struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // "ready", "running", "stopped", "error"
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	CreatedAt time.Time `json:"created_at"`
}

// ExecResult is the outcome of executing a command in a sandbox.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out"`
	Duration time.Duration `json:"duration"`
}

// --- DeploymentService Contract ---
// Codex 5.3 implements this.

// DeploymentService handles pushing builds to production infrastructure.
type DeploymentService interface {
	// Deploy pushes a build to the target environment.
	Deploy(ctx context.Context, cfg DeployConfig) (DeployResult, error)

	// GetDeployStatus checks the status of a deployment.
	GetDeployStatus(ctx context.Context, deployID string) (DeployResult, error)

	// Rollback reverts to a previous deployment.
	Rollback(ctx context.Context, deployID string) error
}

// DeployConfig defines deployment parameters.
type DeployConfig struct {
	BuildID      string            `json:"build_id"`
	ProjectID    string            `json:"project_id"`
	Environment  string            `json:"environment"` // "preview", "staging", "production"
	Provider     string            `json:"provider"`    // "render", "fly", "vercel", "k8s"
	BuildArtifacts []string        `json:"build_artifacts"`
	EnvVars      map[string]string `json:"env_vars,omitempty"`
}

// DeployResult is the outcome of a deployment.
type DeployResult struct {
	DeployID  string    `json:"deploy_id"`
	Status    string    `json:"status"` // "pending", "building", "deploying", "live", "failed"
	URL       string    `json:"url,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// --- Terminal Multiplexer Contract ---
// Codex 5.3 implements this.

// TerminalMultiplexer manages multiple terminal sessions.
type TerminalMultiplexer interface {
	// CreateSession opens a new terminal session.
	CreateSession(ctx context.Context, sandboxID string) (string, error) // returns session ID

	// SendInput sends keystrokes to a session.
	SendInput(ctx context.Context, sessionID string, data []byte) error

	// GetOutput reads output from a session.
	GetOutput(ctx context.Context, sessionID string) ([]byte, error)

	// ResizeSession changes terminal dimensions.
	ResizeSession(ctx context.Context, sessionID string, cols, rows int) error

	// CloseSession terminates a session.
	CloseSession(ctx context.Context, sessionID string) error
}

// --- E2E Test Framework Contract ---
// Codex 5.3 implements this.

// E2ETestRunner executes end-to-end tests against a deployed build.
type E2ETestRunner interface {
	// RunSuite executes a test suite and returns results.
	RunSuite(ctx context.Context, cfg E2ETestConfig) (*E2ETestResult, error)
}

// E2ETestConfig defines parameters for E2E testing.
type E2ETestConfig struct {
	TargetURL    string        `json:"target_url"`
	TestSuite    string        `json:"test_suite"`    // suite name or path
	Browser      string        `json:"browser"`       // "chromium", "firefox", "webkit"
	Timeout      time.Duration `json:"timeout"`
	Screenshots  bool          `json:"screenshots"`
}

// E2ETestResult is the outcome of an E2E test suite.
type E2ETestResult struct {
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Skipped   int           `json:"skipped"`
	Duration  time.Duration `json:"duration"`
	Failures  []string      `json:"failures,omitempty"`
	Report    string        `json:"report,omitempty"` // HTML report path
}
