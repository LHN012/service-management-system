//go:build windows

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type windowsSnapshot struct {
	Processes []struct {
		ProcessID      int    `json:"ProcessId"`
		Name           string `json:"Name"`
		CommandLine    string `json:"CommandLine"`
		ExecutablePath string `json:"ExecutablePath"`
	} `json:"Processes"`
	Ports []struct {
		LocalPort     int `json:"LocalPort"`
		OwningProcess int `json:"OwningProcess"`
	} `json:"Ports"`
}

const snapshotScript = `$ErrorActionPreference='SilentlyContinue'; $processes=@(Get-CimInstance Win32_Process | Select-Object ProcessId,Name,CommandLine,ExecutablePath); $ports=@(Get-NetTCPConnection -State Listen | Select-Object LocalPort,OwningProcess); [pscustomobject]@{Processes=$processes;Ports=$ports} | ConvertTo-Json -Depth 4 -Compress`

func listProcesses() ([]ProcessInfo, error) {
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", snapshotScript)
	setHidden(command)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("collect Windows process snapshot: %w", err)
	}
	var snapshot windowsSnapshot
	if err := json.Unmarshal(output, &snapshot); err != nil {
		return nil, fmt.Errorf("parse Windows process snapshot: %w", err)
	}
	ports := map[int][]int{}
	for _, entry := range snapshot.Ports {
		if entry.OwningProcess > 0 && entry.LocalPort > 0 && !containsPort(ports[entry.OwningProcess], entry.LocalPort) {
			ports[entry.OwningProcess] = append(ports[entry.OwningProcess], entry.LocalPort)
		}
	}
	result := make([]ProcessInfo, 0, len(snapshot.Processes))
	for _, process := range snapshot.Processes {
		if process.ProcessID < 1 {
			continue
		}
		cwd := ""
		if process.ExecutablePath != "" {
			cwd = filepath.Dir(process.ExecutablePath)
		}
		processPorts := append([]int(nil), ports[process.ProcessID]...)
		sort.Ints(processPorts)
		result = append(result, ProcessInfo{
			PID: process.ProcessID, Name: process.Name, Command: strings.TrimSpace(process.CommandLine),
			CWD: cwd, Ports: processPorts,
		})
	}
	return result, nil
}

func StartCommand(command, workDir, logPath string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command("cmd.exe", "/D", "/S", "/C", command)
	cmd.Dir = workDir
	cmd.Stdout = log
	cmd.Stderr = log
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		log.Close()
		return 0, err
	}
	pid := cmd.Process.Pid
	go func() {
		_ = cmd.Wait()
		_ = log.Close()
	}()
	return pid, nil
}

func RunCommand(command, workDir string) error {
	cmd := exec.Command("cmd.exe", "/D", "/S", "/C", command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ProcessAlive(pid int) bool { return processAlive(pid) }
func Terminate(pid int) error   { return terminate(pid) }
func ForceKill(pid int) error   { return forceKill(pid) }

func StartAgent(executable string, args []string, logPath string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(executable, args...)
	cmd.Stdout = log
	cmd.Stderr = log
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		log.Close()
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	_ = log.Close()
	return pid, nil
}

func parseWindowsPID(value string) (int, error) {
	pid, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || pid < 1 {
		return 0, fmt.Errorf("invalid pid %q", value)
	}
	return pid, nil
}
