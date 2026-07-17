package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcessIdentity records enough Linux process metadata to detect PID reuse.
type ProcessIdentity struct {
	BootID         string `json:"bootId,omitempty"`
	StartTicks     uint64 `json:"startTicks,omitempty"`
	ProcessGroupID int    `json:"processGroupId,omitempty"`
	Executable     string `json:"executable,omitempty"`
	WorkingDir     string `json:"workingDir,omitempty"`
	CommandLine    string `json:"commandLine,omitempty"`
}

func captureProcessIdentity(pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("invalid pid: %d", pid)
	}

	bootData, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read Linux boot id: %w", err)
	}
	statData, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process stat for pid %d: %w", pid, err)
	}
	startTicks, processGroupID, err := parseProcStat(string(statData))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("parse process stat for pid %d: %w", pid, err)
	}

	identity := ProcessIdentity{
		BootID:         strings.TrimSpace(string(bootData)),
		StartTicks:     startTicks,
		ProcessGroupID: processGroupID,
	}
	identity.Executable, _ = os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	identity.WorkingDir, _ = os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd"))
	if commandLine, readErr := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline")); readErr == nil {
		identity.CommandLine = strings.TrimSpace(strings.ReplaceAll(string(commandLine), "\x00", " "))
	}
	return identity, nil
}

func verifyProcessIdentity(pid int, expected ProcessIdentity) (ProcessIdentity, error) {
	if expected.BootID == "" || expected.StartTicks == 0 {
		return ProcessIdentity{}, fmt.Errorf("runtime record has no process identity; refusing to signal pid %d", pid)
	}

	current, err := captureProcessIdentity(pid)
	if err != nil {
		return ProcessIdentity{}, err
	}
	if current.BootID != expected.BootID || current.StartTicks != expected.StartTicks {
		return current, fmt.Errorf("pid %d identity changed; refusing to signal it", pid)
	}
	if expected.ProcessGroupID > 0 && current.ProcessGroupID != expected.ProcessGroupID {
		return current, fmt.Errorf("pid %d process group changed; refusing to signal it", pid)
	}
	return current, nil
}

// parseProcStat reads pgrp (field 5) and starttime (field 22). The command in
// parentheses may contain spaces, so fields cannot be split from the start.
func parseProcStat(value string) (uint64, int, error) {
	closing := strings.LastIndex(value, ")")
	if closing < 0 || closing+1 >= len(value) {
		return 0, 0, fmt.Errorf("invalid /proc stat format")
	}
	fields := strings.Fields(value[closing+1:])
	if len(fields) <= 19 {
		return 0, 0, fmt.Errorf("incomplete /proc stat data")
	}
	processGroupID, err := strconv.Atoi(fields[2])
	if err != nil || processGroupID <= 0 {
		return 0, 0, fmt.Errorf("invalid process group id")
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || startTicks == 0 {
		return 0, 0, fmt.Errorf("invalid process start time")
	}
	return startTicks, processGroupID, nil
}
