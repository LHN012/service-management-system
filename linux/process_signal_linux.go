//go:build linux

package main

import (
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func configureServiceProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func signalServiceRuntime(runtime ServiceRuntime, force bool) error {
	signal := unix.SIGTERM
	if force {
		signal = unix.SIGKILL
	}

	if runtime.ProcessGroupID > 1 {
		if runtime.ProcessGroupID == unix.Getpgrp() {
			return fmt.Errorf("refusing to signal SMS process group %d", runtime.ProcessGroupID)
		}
		if err := unix.Kill(-runtime.ProcessGroupID, signal); err == nil || err == unix.ESRCH {
			return nil
		} else {
			return fmt.Errorf("signal process group %d: %w", runtime.ProcessGroupID, err)
		}
	}
	if err := unix.Kill(runtime.PID, signal); err != nil && err != unix.ESRCH {
		return err
	}
	return nil
}

func serviceRuntimeAlive(runtime ServiceRuntime) bool {
	if runtime.ProcessGroupID > 1 {
		err := unix.Kill(-runtime.ProcessGroupID, 0)
		return err == nil || err == unix.EPERM
	}
	err := unix.Kill(runtime.PID, 0)
	return err == nil || err == unix.EPERM
}
