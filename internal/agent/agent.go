package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sms/internal/config"
	"sms/internal/service"
	"sms/internal/store"
	"sms/internal/system"
)

type Status struct {
	Running    bool      `json:"running"`
	PID        int       `json:"pid,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	LastScanAt time.Time `json:"lastScanAt,omitempty"`
	Projects   int       `json:"projects"`
}

func Start(root string) (int, error) {
	status, _ := ReadStatus(root)
	if status.Running {
		return status.PID, fmt.Errorf("agent is already running with pid %d", status.PID)
	}
	executable, err := os.Executable()
	if err != nil {
		return 0, err
	}
	logPath := filepath.Join(root, "data", "logs", "system.log")
	pid, err := system.StartAgent(executable, []string{"agent", "--root", root}, logPath)
	if err != nil {
		return 0, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		current, _ := ReadStatus(root)
		if current.Running {
			return current.PID, nil
		}
	}
	return pid, fmt.Errorf("agent process %d did not become ready; check %s", pid, logPath)
}

func Stop(root string) error {
	status, err := ReadStatus(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !status.Running {
		return fmt.Errorf("agent is not running")
	}
	if err := system.Terminate(status.PID); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if !system.ProcessAlive(status.PID) {
			_ = os.Remove(pidPath(root))
			return nil
		}
	}
	return fmt.Errorf("agent pid %d did not stop within 10 seconds", status.PID)
}

func ReadStatus(root string) (Status, error) {
	var status Status
	data, err := os.ReadFile(statusPath(root))
	if err == nil {
		_ = json.Unmarshal(data, &status)
	}
	pidData, pidErr := os.ReadFile(pidPath(root))
	if pidErr != nil {
		status.Running = false
		return status, pidErr
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if parseErr != nil {
		return status, parseErr
	}
	status.PID = pid
	status.Running = system.ProcessAlive(pid)
	return status, nil
}

func Run(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "data", "runtime"), 0o755); err != nil {
		return err
	}
	if existing, _ := ReadStatus(root); existing.Running && existing.PID != os.Getpid() {
		return fmt.Errorf("agent is already running with pid %d", existing.PID)
	}
	if err := os.WriteFile(pidPath(root), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		return err
	}
	defer os.Remove(pidPath(root))

	appConfig, err := config.Load(root)
	if err != nil {
		return err
	}
	projectStore := store.New(root)
	manager := service.New(root, projectStore, system.NewScanner(), time.Duration(appConfig.StopTimeoutSeconds)*time.Second)
	startedAt := time.Now()
	scan := func() {
		runtimes, scanErr := manager.ScanAll()
		status := Status{Running: true, PID: os.Getpid(), StartedAt: startedAt, LastScanAt: time.Now(), Projects: len(runtimes)}
		if scanErr == nil {
			_ = writeStatus(root, status)
		}
	}
	scan()
	ticker := time.NewTicker(time.Duration(appConfig.ScanIntervalSeconds) * time.Second)
	defer ticker.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	for {
		select {
		case <-ticker.C:
			scan()
		case <-signals:
			return nil
		}
	}
}

func pidPath(root string) string {
	return filepath.Join(root, "data", "runtime", "sms.pid")
}

func statusPath(root string) string {
	return filepath.Join(root, "data", "runtime", "agent.json")
}

func writeStatus(root string, status Status) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := statusPath(root) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, statusPath(root))
}
