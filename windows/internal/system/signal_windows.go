//go:build windows

package system

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
	createNoWindow        = 0x08000000
	stillActive           = 259
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess | createNoWindow,
		HideWindow:    true,
	}
}

func setHidden(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow, HideWindow: true}
}

func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(handle)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}

func terminate(pid int) error {
	cmd := exec.Command("taskkill.exe", "/PID", strconv.Itoa(pid), "/T")
	setHidden(cmd)
	return cmd.Run()
}

func forceKill(pid int) error {
	cmd := exec.Command("taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F")
	setHidden(cmd)
	return cmd.Run()
}
