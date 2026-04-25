//go:build !darwin && !linux

package execution

import "os/exec"

func classifyProcessExit(_ *exec.ExitError, _ *ExecutionResult) {}

func collectProcessUsage(_ *exec.Cmd, _ *ExecutionResult) {}

func configureProcessGroup(_ *exec.Cmd) {}

func signalProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
