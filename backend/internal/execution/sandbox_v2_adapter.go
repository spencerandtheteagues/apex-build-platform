package execution

import (
	"context"
	"fmt"
	"os"
	"time"

	sandboxv2 "apex-build/internal/sandbox/v2"
)

type sandboxV2ExecutorAdapter struct {
	manager   *sandboxv2.Manager
	executor  sandboxv2.Executor
	createdAt time.Time
}

func newSandboxV2ExecutorAdapter(cfg *sandboxv2.ManagerConfig) (*sandboxV2ExecutorAdapter, error) {
	manager, err := sandboxv2.NewManager(cfg)
	if err != nil {
		return nil, err
	}
	exec, err := sandboxv2.NewDockerExecutor(manager)
	if err != nil {
		return nil, err
	}
	return &sandboxV2ExecutorAdapter{
		manager:   manager,
		executor:  exec,
		createdAt: time.Now(),
	}, nil
}

func (a *sandboxV2ExecutorAdapter) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	res, err := a.executor.Execute(ctx, sandboxv2.ExecuteRequest{
		Language: language,
		Code:     code,
		Stdin:    stdin,
	})
	if err != nil {
		return nil, err
	}
	return convertSandboxV2Result(res, language), nil
}

func (a *sandboxV2ExecutorAdapter) ExecuteFile(ctx context.Context, filePath string, args []string, stdin string) (*ExecutionResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("sandbox v2 read file: %w", err)
	}
	_ = args // sandbox-v2 file execution path is content-based for now
	lang := detectLanguageFromFile(filePath)
	if lang == "" {
		lang = "plaintext"
	}
	return a.Execute(ctx, lang, string(data), stdin)
}

func (a *sandboxV2ExecutorAdapter) Kill(execID string) error {
	return a.executor.Kill(execID)
}

func (a *sandboxV2ExecutorAdapter) GetActiveExecutions() int {
	return a.executor.ActiveCount()
}

func (a *sandboxV2ExecutorAdapter) Cleanup() error {
	return a.executor.Close()
}

func (a *sandboxV2ExecutorAdapter) Stats() map[string]interface{} {
	stats := a.executor.Stats()
	stats["created_at"] = a.createdAt
	return stats
}

func convertSandboxV2Result(in *sandboxv2.ExecuteResult, language string) *ExecutionResult {
	if in == nil {
		return nil
	}
	return &ExecutionResult{
		ID:          in.ID,
		Status:      in.Status,
		Output:      in.Output,
		ErrorOutput: in.ErrorOutput,
		ExitCode:    in.ExitCode,
		Duration:    in.Duration,
		DurationMs:  in.DurationMs,
		Language:    language,
		StartedAt:   in.StartedAt,
		CompletedAt: in.CompletedAt,
		TimedOut:    in.TimedOut,
		Killed:      in.Killed,
	}
}
