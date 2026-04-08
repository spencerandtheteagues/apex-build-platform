//go:build darwin || linux

package agents

import (
	"os/exec"
	"syscall"
)

func configurePreviewCheckCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminatePreviewCheckCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}

	_ = cmd.Process.Kill()
}
