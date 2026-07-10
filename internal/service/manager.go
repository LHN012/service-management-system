package service

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sms/internal/model"
	"sms/internal/store"
	"sms/internal/system"
)

var ErrAlreadyRunning = errors.New("process is already running")

type Result struct {
	Name    string
	Type    string
	Action  string
	Status  string
	Message string
}

type Manager struct {
	Root        string
	Store       *store.Store
	Scanner     *system.Scanner
	StopTimeout time.Duration
}

func New(root string, projectStore *store.Store, scanner *system.Scanner, stopTimeout time.Duration) *Manager {
	return &Manager{Root: root, Store: projectStore, Scanner: scanner, StopTimeout: stopTimeout}
}

func (m *Manager) Scan(project model.Project) (model.Runtime, error) {
	runtime, err := m.Scanner.ScanProject(project)
	if err != nil {
		return runtime, err
	}
	if err := m.Store.SaveRuntime(runtime); err != nil {
		return runtime, err
	}
	return runtime, nil
}

func (m *Manager) ScanAll() ([]model.Runtime, error) {
	projects, err := m.Store.List()
	if err != nil {
		return nil, err
	}
	processes, err := m.Scanner.Processes()
	if err != nil {
		return nil, err
	}
	runtimes := make([]model.Runtime, 0, len(projects))
	for _, project := range projects {
		runtime := m.Scanner.ScanProjectWithProcesses(project, processes)
		if err := m.Store.SaveRuntime(runtime); err != nil {
			return runtimes, fmt.Errorf("save runtime %s: %w", project.Code, err)
		}
		runtimes = append(runtimes, runtime)
	}
	return runtimes, nil
}

func (m *Manager) StartBackend(project model.Project, backend model.Backend) Result {
	result := Result{Name: backend.Name, Type: "backend", Action: "start", Status: "failed"}
	if backend.Disabled {
		result.Status = "skipped"
		result.Message = "disabled"
		return result
	}
	processes, err := m.Scanner.Processes()
	if err != nil {
		result.Message = err.Error()
		return result
	}
	for _, process := range processes {
		score, _ := system.MatchesBackend(backend, process)
		if score >= 3 {
			result.Status = "running"
			result.Message = fmt.Sprintf("already running (pid %d)", process.PID)
			return result
		}
	}
	for _, port := range backend.ExpectedPorts {
		for _, process := range processes {
			if hasPort(process.Ports, port) {
				result.Message = fmt.Sprintf("port %d is occupied by pid %d: %s", port, process.PID, process.Command)
				return result
			}
		}
	}
	if info, err := os.Stat(backend.WorkDir); err != nil || !info.IsDir() {
		result.Message = fmt.Sprintf("workDir is not available: %s", backend.WorkDir)
		return result
	}
	logPath := filepath.Join(m.Store.ProjectDir(project.Code), "logs", backend.Name+".out.log")
	pid, err := system.StartCommand(backend.StartCommand, backend.WorkDir, logPath)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		processes, scanErr := m.Scanner.Processes()
		if scanErr != nil {
			continue
		}
		for _, process := range processes {
			score, _ := system.MatchesBackend(backend, process)
			if score >= 3 {
				if backend.HealthCheck != "" && !healthCheckOK(backend.HealthCheck) {
					continue
				}
				result.Status = "running"
				result.Message = fmt.Sprintf("pid %d, ports %v", process.PID, process.Ports)
				return result
			}
		}
	}
	result.Message = fmt.Sprintf("command started as pid %d but process matching timed out; see %s", pid, logPath)
	return result
}

func (m *Manager) ForceStopBackend(backend model.Backend) Result {
	result := Result{Name: backend.Name, Type: "backend", Action: "force-stop", Status: "failed"}
	processes, err := m.matchingBackendProcesses(backend)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	if len(processes) == 0 {
		result.Status, result.Message = "stopped", "not running"
		return result
	}
	for _, process := range processes {
		if err := system.ForceKill(process.PID); err != nil {
			result.Message = fmt.Sprintf("force kill pid %d: %v", process.PID, err)
			return result
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		remaining, _ := m.matchingBackendProcesses(backend)
		if len(remaining) == 0 {
			result.Status, result.Message = "stopped", "force stopped"
			return result
		}
	}
	result.Message = "process still detected after SIGKILL"
	return result
}

func (m *Manager) StopBackend(project model.Project, backend model.Backend, force bool) Result {
	result := Result{Name: backend.Name, Type: "backend", Action: "stop", Status: "failed"}
	if backend.StopCommand != "" {
		if err := system.RunCommand(backend.StopCommand, backend.WorkDir); err != nil {
			result.Message = fmt.Sprintf("stopCommand failed: %v", err)
			return result
		}
	}
	processes, err := m.matchingBackendProcesses(backend)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	if len(processes) == 0 {
		result.Status = "stopped"
		result.Message = "not running"
		return result
	}
	if backend.StopCommand == "" {
		for _, process := range processes {
			if err := system.Terminate(process.PID); err != nil {
				result.Message = fmt.Sprintf("send SIGTERM to pid %d: %v", process.PID, err)
				return result
			}
		}
	}
	timeout := m.StopTimeout
	if backend.StopTimeout > 0 {
		timeout = time.Duration(backend.StopTimeout) * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		remaining, _ := m.matchingBackendProcesses(backend)
		if len(remaining) == 0 {
			result.Status = "stopped"
			result.Message = "gracefully stopped"
			return result
		}
	}
	if !force {
		result.Message = "graceful stop timed out"
		return result
	}
	remaining, _ := m.matchingBackendProcesses(backend)
	for _, process := range remaining {
		if err := system.ForceKill(process.PID); err != nil {
			result.Message = fmt.Sprintf("force kill pid %d: %v", process.PID, err)
			return result
		}
	}
	result.Status = "stopped"
	result.Message = "force stopped after timeout"
	return result
}

func (m *Manager) StartFrontend(project model.Project, frontend model.Frontend) Result {
	result := Result{Name: frontend.Name, Type: "frontend", Action: "start", Status: "failed"}
	if frontend.Disabled {
		result.Status, result.Message = "skipped", "disabled"
		return result
	}
	if frontend.RootDir != "" {
		if info, err := os.Stat(frontend.RootDir); err != nil || !info.IsDir() {
			result.Message = "rootDir is not available: " + frontend.RootDir
			return result
		}
	}
	if frontend.NginxConf != "" {
		if _, err := os.Stat(frontend.NginxConf); err != nil {
			result.Message = "nginxConf is not available: " + frontend.NginxConf
			return result
		}
	}
	command := frontend.ReloadCommand
	if frontend.NginxMode == "dedicated" {
		command = frontend.StartCommand
		if command == "" {
			command = fmt.Sprintf("nginx -c %q", frontend.NginxConf)
		}
	}
	if command == "" {
		result.Message = "no start or reload command configured"
		return result
	}
	if err := system.RunCommand("nginx -t", m.Root); err != nil {
		result.Message = fmt.Sprintf("nginx configuration test failed: %v", err)
		return result
	}
	if err := system.RunCommand(command, m.Root); err != nil {
		result.Message = err.Error()
		return result
	}
	result.Status = "running"
	result.Message = "nginx command completed"
	return result
}

func (m *Manager) StopFrontend(project model.Project, frontend model.Frontend) Result {
	result := Result{Name: frontend.Name, Type: "frontend", Action: "stop", Status: "skipped"}
	if frontend.NginxMode == "shared" {
		result.Message = "shared nginx is not stopped; disable the site and reload explicitly"
		return result
	}
	if frontend.StopCommand == "" {
		result.Status = "failed"
		result.Message = "dedicated nginx stopCommand is not configured"
		return result
	}
	if err := system.RunCommand(frontend.StopCommand, m.Root); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result
	}
	result.Status = "stopped"
	result.Message = "stop command completed"
	return result
}

func (m *Manager) StartProject(project model.Project, group, component string) []Result {
	var results []Result
	if group == "" || group == "backend" {
		for _, backend := range project.Backends {
			if component == "" || component == backend.Name {
				results = append(results, m.StartBackend(project, backend))
			}
		}
	}
	if group == "" || group == "front" {
		for _, frontend := range project.Frontends {
			if component == "" || component == frontend.Name {
				results = append(results, m.StartFrontend(project, frontend))
			}
		}
	}
	return results
}

func (m *Manager) StopProject(project model.Project, group, component string, force bool) []Result {
	var results []Result
	if group == "" || group == "front" {
		for i := len(project.Frontends) - 1; i >= 0; i-- {
			frontend := project.Frontends[i]
			if component == "" || component == frontend.Name {
				results = append(results, m.StopFrontend(project, frontend))
			}
		}
	}
	if group == "" || group == "backend" {
		for i := len(project.Backends) - 1; i >= 0; i-- {
			backend := project.Backends[i]
			if component == "" || component == backend.Name {
				results = append(results, m.StopBackend(project, backend, force))
			}
		}
	}
	return results
}

func (m *Manager) ForceStopProject(project model.Project, group, component string) []Result {
	var results []Result
	if group != "" && group != "backend" {
		return results
	}
	for i := len(project.Backends) - 1; i >= 0; i-- {
		backend := project.Backends[i]
		if component == "" || component == backend.Name {
			results = append(results, m.ForceStopBackend(backend))
		}
	}
	return results
}

func (m *Manager) matchingBackendProcesses(backend model.Backend) ([]system.ProcessInfo, error) {
	processes, err := m.Scanner.Processes()
	if err != nil {
		return nil, err
	}
	var matches []system.ProcessInfo
	for _, process := range processes {
		score, _ := system.MatchesBackend(backend, process)
		if score >= 3 {
			matches = append(matches, process)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].PID > matches[j].PID })
	return matches, nil
}

func hasPort(ports []int, target int) bool {
	for _, port := range ports {
		if port == target {
			return true
		}
	}
	return false
}

func healthCheckOK(url string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 400
}

func ResultsError(results []Result) error {
	var failures []string
	for _, result := range results {
		if result.Status == "failed" {
			failures = append(failures, result.Name+": "+result.Message)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}
