//go:build darwin || linux

package execution

import (
	"os/exec"
	"syscall"
)

func classifyProcessExit(exitErr *exec.ExitError, result *ExecutionResult) {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		switch status.Signal() {
		case syscall.SIGKILL:
			result.Status = "killed"
			result.Killed = true
			result.ErrorOutput = "Process killed (possible memory limit exceeded)"
		case syscall.SIGXCPU:
			result.Status = "timeout"
			result.TimedOut = true
			result.ErrorOutput = "CPU time limit exceeded"
		}
	}
}

func collectProcessUsage(cmd *exec.Cmd, result *ExecutionResult) {
	if cmd.ProcessState == nil {
		return
	}

	if rusage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		result.MemoryUsed = rusage.Maxrss * 1024 // Convert KB to bytes.
		result.CPUTime = rusage.Utime.Nano()/1e6 + rusage.Stime.Nano()/1e6
	}
}

func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func signalProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
}

func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}

	_ = cmd.Process.Kill()
}
