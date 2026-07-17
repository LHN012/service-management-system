//go:build !linux && !windows

package system

import (
	"fmt"
	"os"
	"os/exec"
)

func listProcesses() ([]ProcessInfo, error) {
	return nil, fmt.Errorf("process scanning is only supported on Linux")
}

func StartCommand(command, workDir, logPath string) (int, error) {
	return 0, fmt.Errorf("process start is only supported on Linux")
}

func RunCommand(command, workDir string) error {
	return fmt.Errorf("command execution is only supported on Linux")
}

func ProcessAlive(pid int) bool { return processAlive(pid) }
func Terminate(pid int) error   { return terminate(pid) }
func ForceKill(pid int) error   { return forceKill(pid) }

func StartAgent(executable string, args []string, logPath string) (int, error) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer log.Close()
	cmd := exec.Command(executable, args...)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, cmd.Process.Release()
}
