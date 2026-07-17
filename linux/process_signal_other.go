//go:build !linux

package main

import (
	"fmt"
	"os/exec"
)

func configureServiceProcess(command *exec.Cmd) {}

func signalServiceRuntime(runtime ServiceRuntime, force bool) error {
	return fmt.Errorf("service signaling is only supported on Linux")
}

func serviceRuntimeAlive(runtime ServiceRuntime) bool {
	return processAlive(runtime.PID)
}
