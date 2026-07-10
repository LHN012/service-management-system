//go:build !linux && !windows

package system

import (
	"os"
	"os/exec"
)

func setDetached(cmd *exec.Cmd) {}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	return err == nil && process != nil
}

func terminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func forceKill(pid int) error { return terminate(pid) }
