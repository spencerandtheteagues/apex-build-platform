//go:build !darwin && !linux

package preview

import "os/exec"

func configureHostProcess(_ *exec.Cmd) {}

func signalHostProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func forceKillHostProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
