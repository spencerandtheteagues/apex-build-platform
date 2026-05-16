//go:build windows

package agents

import (
	"os/exec"
	"time"
)

func configurePreviewCheckCommand(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
}

func terminatePreviewCheckCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
