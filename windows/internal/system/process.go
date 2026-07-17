package system

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sms/internal/model"
)

type ProcessInfo struct {
	PID     int
	Name    string
	Command string
	CWD     string
	User    string
	Ports   []int
}

type Scanner struct{}

func NewScanner() *Scanner { return &Scanner{} }

func (s *Scanner) Processes() ([]ProcessInfo, error) {
	return listProcesses()
}

func (s *Scanner) Port(port int) ([]ProcessInfo, error) {
	processes, err := s.Processes()
	if err != nil {
		return nil, err
	}
	var result []ProcessInfo
	for _, process := range processes {
		if containsPort(process.Ports, port) {
			result = append(result, process)
		}
	}
	return result, nil
}

func (s *Scanner) ScanProject(project model.Project) (model.Runtime, error) {
	processes, err := s.Processes()
	if err != nil {
		return model.Runtime{}, err
	}
	return s.ScanProjectWithProcesses(project, processes), nil
}

func (s *Scanner) ScanProjectWithProcesses(project model.Project, processes []ProcessInfo) model.Runtime {
	runtime := model.Runtime{ProjectCode: project.Code, LastScanAt: time.Now()}
	for _, backend := range project.Backends {
		if backend.Disabled {
			continue
		}
		match, matchedBy := bestBackendMatch(backend, processes)
		entry := model.ProcessRuntime{Name: backend.Name, Type: "backend", Status: "stopped"}
		if match != nil {
			entry.Status = "running"
			entry.PID = match.PID
			entry.Ports = match.Ports
			entry.Command = match.Command
			entry.CWD = match.CWD
			entry.MatchedBy = matchedBy
		}
		runtime.Processes = append(runtime.Processes, entry)
	}
	for _, frontend := range project.Frontends {
		if frontend.Disabled {
			continue
		}
		match, matchedBy := bestFrontendMatch(frontend, processes)
		entry := model.ProcessRuntime{Name: frontend.Name, Type: "frontend", Status: "stopped"}
		if match != nil {
			entry.Status = "running"
			entry.PID = match.PID
			entry.Ports = match.Ports
			entry.Command = match.Command
			entry.CWD = match.CWD
			entry.MatchedBy = matchedBy
		}
		runtime.Processes = append(runtime.Processes, entry)
	}
	return runtime
}

func MatchesBackend(backend model.Backend, process ProcessInfo) (int, []string) {
	score := 0
	var matched []string
	if backend.Match.CommandContains != "" && strings.Contains(process.Command, backend.Match.CommandContains) {
		score += 3
		matched = append(matched, "commandContains")
	}
	matchCWD := backend.Match.CWD
	if matchCWD == "" {
		matchCWD = backend.WorkDir
	}
	if matchCWD != "" && samePath(process.CWD, matchCWD) {
		score += 3
		matched = append(matched, "cwd")
	}
	for _, port := range backend.ExpectedPorts {
		if containsPort(process.Ports, port) {
			score += 3
			matched = append(matched, fmt.Sprintf("port:%d", port))
		}
	}
	return score, matched
}

func bestBackendMatch(backend model.Backend, processes []ProcessInfo) (*ProcessInfo, []string) {
	bestScore := 0
	var best *ProcessInfo
	var bestMatched []string
	for i := range processes {
		score, matched := MatchesBackend(backend, processes[i])
		if score >= 3 && score > bestScore {
			bestScore = score
			copy := processes[i]
			best = &copy
			bestMatched = matched
		}
	}
	return best, bestMatched
}

func bestFrontendMatch(frontend model.Frontend, processes []ProcessInfo) (*ProcessInfo, []string) {
	bestScore := 0
	var best *ProcessInfo
	var bestMatched []string
	for i := range processes {
		score := 0
		var matched []string
		command := strings.ToLower(processes[i].Command + " " + processes[i].Name)
		if strings.Contains(command, "nginx") {
			score++
			matched = append(matched, "process:nginx")
		}
		if frontend.NginxConf != "" && strings.Contains(processes[i].Command, frontend.NginxConf) {
			score += 3
			matched = append(matched, "nginxConf")
		}
		for _, port := range frontend.ExpectedPorts {
			if containsPort(processes[i].Ports, port) {
				score += 3
				matched = append(matched, fmt.Sprintf("port:%d", port))
			}
		}
		if score >= 4 && score > bestScore {
			bestScore = score
			copy := processes[i]
			best = &copy
			bestMatched = matched
		}
	}
	return best, bestMatched
}

func containsPort(ports []int, target int) bool {
	for _, port := range ports {
		if port == target {
			return true
		}
	}
	return false
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftClean := filepath.Clean(left)
	rightClean := filepath.Clean(right)
	return leftClean == rightClean
}

func SortProcesses(processes []ProcessInfo) {
	sort.Slice(processes, func(i, j int) bool {
		if len(processes[i].Ports) == 0 || len(processes[j].Ports) == 0 {
			return processes[i].PID < processes[j].PID
		}
		return processes[i].Ports[0] < processes[j].Ports[0]
	})
}
