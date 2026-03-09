package core

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// ExecSmokeRunner runs smoke test commands as local subprocesses.
// This is used by BuildValidator when RunSmokeTest is enabled.
type ExecSmokeRunner struct {
	// WorkDir is the directory in which the command runs.
	WorkDir string
}

// NewExecSmokeRunner creates a runner that executes commands in workDir.
func NewExecSmokeRunner(workDir string) *ExecSmokeRunner {
	return &ExecSmokeRunner{WorkDir: workDir}
}

// RunSmokeTest executes command as a shell command with a timeout.
// Returns stdout+stderr combined, the exit code, and any exec error.
func (r *ExecSmokeRunner) RunSmokeTest(ctx context.Context, command string, timeout time.Duration) (string, int, error) {
	if command == "" {
		return "", 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Split into shell + args for cross-platform compatibility
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if r.WorkDir != "" {
		cmd.Dir = r.WorkDir
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := strings.TrimSpace(buf.String())

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // exit code captured, not an exec error
		}
		// context deadline counts as exitCode -1
		if ctx.Err() != nil {
			return output, -1, ctx.Err()
		}
	}

	return output, exitCode, err
}
