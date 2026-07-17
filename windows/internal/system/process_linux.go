//go:build linux

package system

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	pidPattern      = regexp.MustCompile(`pid=(\d+)`)
	endpointPattern = regexp.MustCompile(`(?:\[[^]]+\]|[^\s:]+):(\d+)`)
)

func listProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	portsByPID := listeningPorts()
	users := map[string]string{}
	var processes []ProcessInfo
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		base := filepath.Join("/proc", entry.Name())
		commandBytes, err := os.ReadFile(filepath.Join(base, "cmdline"))
		if err != nil || len(commandBytes) == 0 {
			continue
		}
		command := strings.TrimSpace(string(bytes.ReplaceAll(commandBytes, []byte{0}, []byte{' '})))
		cwd, _ := os.Readlink(filepath.Join(base, "cwd"))
		name, uid := readStatus(filepath.Join(base, "status"))
		username := uid
		if cached, ok := users[uid]; ok {
			username = cached
		} else if account, lookupErr := user.LookupId(uid); lookupErr == nil {
			username = account.Username
			users[uid] = username
		}
		ports := append([]int(nil), portsByPID[pid]...)
		sort.Ints(ports)
		processes = append(processes, ProcessInfo{PID: pid, Name: name, Command: command, CWD: cwd, User: username, Ports: ports})
	}
	return processes, nil
}

func readStatus(path string) (name, uid string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "Name":
			name = fields[1]
		case "Uid":
			uid = fields[1]
		}
	}
	return name, uid
}

func listeningPorts() map[int][]int {
	result := map[int][]int{}
	output, err := exec.Command("ss", "-ltnpH").CombinedOutput()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(output), "\n") {
		pidMatch := pidPattern.FindStringSubmatch(line)
		if len(pidMatch) != 2 {
			continue
		}
		pid, _ := strconv.Atoi(pidMatch[1])
		endpoints := endpointPattern.FindAllStringSubmatch(line, -1)
		if len(endpoints) == 0 {
			continue
		}
		port, err := strconv.Atoi(endpoints[0][1])
		if err != nil || port == 0 {
			continue
		}
		if !containsPort(result[pid], port) {
			result[pid] = append(result[pid], port)
		}
	}
	return result
}

func StartCommand(command, workDir, logPath string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command("/bin/sh", "-lc", command)
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
	cmd := exec.Command("/bin/sh", "-lc", command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ProcessAlive(pid int) bool {
	return processAlive(pid)
}

func Terminate(pid int) error { return terminate(pid) }
func ForceKill(pid int) error { return forceKill(pid) }

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

func ParseSSLine(line string) (int, int, error) {
	pidMatch := pidPattern.FindStringSubmatch(line)
	endpoints := endpointPattern.FindAllStringSubmatch(line, -1)
	if len(pidMatch) != 2 || len(endpoints) == 0 {
		return 0, 0, errors.New("line does not contain pid and listening endpoint")
	}
	pid, err := strconv.Atoi(pidMatch[1])
	if err != nil {
		return 0, 0, err
	}
	port, err := strconv.Atoi(endpoints[0][1])
	if err != nil {
		return 0, 0, err
	}
	if pid < 1 || port < 1 {
		return 0, 0, fmt.Errorf("invalid pid or port")
	}
	return pid, port, nil
}
