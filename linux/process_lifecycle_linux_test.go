//go:build linux

package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestTerminateRuntimeProcessRequiresMatchingIdentity(t *testing.T) {
	command := exec.Command("sh", "-c", "exec sleep 30")
	configureServiceProcess(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()

	identity, err := captureProcessIdentity(command.Process.Pid)
	if err != nil {
		_ = command.Process.Kill()
		<-done
		t.Fatal(err)
	}
	runtimeState := ServiceRuntime{ProcessIdentity: identity, PID: command.Process.Pid}
	reaped := false
	t.Cleanup(func() {
		if serviceRuntimeAlive(runtimeState) {
			_ = signalServiceRuntime(runtimeState, true)
		}
		if reaped {
			return
		}
		select {
		case <-done:
			reaped = true
		case <-time.After(2 * time.Second):
		}
	})

	tampered := runtimeState
	tampered.StartTicks++
	if err := terminateRuntimeProcess(tampered, true, 500*time.Millisecond); err == nil {
		t.Fatal("mismatched identity was signaled")
	}
	if !serviceRuntimeAlive(runtimeState) {
		t.Fatal("process exited after rejected identity")
	}

	if err := terminateRuntimeProcess(runtimeState, false, 3*time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		reaped = true
	case <-time.After(2 * time.Second):
		t.Fatal("process was not reaped")
	}
}
