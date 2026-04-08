//go:build windows

package agents

import "os/exec"

func configurePreviewCheckCommand(cmd *exec.Cmd) {}

func terminatePreviewCheckCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
