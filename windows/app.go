package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sms/internal/audit"
	"sms/internal/config"
	"sms/internal/deploy"
	"sms/internal/model"
	"sms/internal/service"
	"sms/internal/store"
	"sms/internal/system"
)

type App struct {
	ctx       context.Context
	root      string
	store     *store.Store
	scanner   *system.Scanner
	manager   *service.Manager
	deployer  *deploy.Engine
	startedAt time.Time
	lastScan  time.Time
	plans     map[string]*deploy.Plan
	stop      chan struct{}
	stopOnce  sync.Once
	mu        sync.Mutex
}

func NewApp() (*App, error) {
	root, err := windowsDataRoot()
	if err != nil {
		return nil, err
	}
	if err := initializeWindowsRoot(root); err != nil {
		return nil, err
	}
	appConfig, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	projectStore := store.NewAt(root, filepath.Join(root, "data", "projects"))
	scanner := system.NewScanner()
	return &App{
		root: root, store: projectStore, scanner: scanner,
		manager:  service.New(root, projectStore, scanner, time.Duration(appConfig.StopTimeoutSeconds)*time.Second),
		deployer: deploy.New(root, projectStore), startedAt: time.Now(), plans: map[string]*deploy.Plan{}, stop: make(chan struct{}),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.scanLoop()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopOnce.Do(func() { close(a.stop) })
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, plan := range a.plans {
		plan.Close()
		delete(a.plans, id)
	}
}

func (a *App) GetDashboard() (Dashboard, error) {
	projects, err := a.store.List()
	if err != nil {
		return Dashboard{}, err
	}
	processes, processErr := a.scanner.Processes()
	if processErr != nil {
		processes = nil
	}
	summaries := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		runtimeState := a.scanner.ScanProjectWithProcesses(project, processes)
		_ = a.store.SaveRuntime(runtimeState)
		summary := summarizeProject(project, runtimeState)
		if processErr != nil {
			summary.Status = "unknown"
		}
		summaries = append(summaries, summary)
	}
	portCount := 0
	for _, process := range processes {
		portCount += len(process.Ports)
	}
	a.mu.Lock()
	a.lastScan = time.Now()
	lastScan := a.lastScan
	a.mu.Unlock()
	return Dashboard{
		Agent:    AgentSummary{Status: "running", Mode: "desktop-agent", StartedAt: a.startedAt, LastScan: lastScan},
		Projects: summaries, Environment: environmentReport(), Recent: readRecentActivity(a.root, 12),
		ProcessCount: len(processes), PortCount: portCount, DataRoot: a.root,
	}, nil
}

func (a *App) ListProjects() ([]ProjectSummary, error) {
	dashboard, err := a.GetDashboard()
	return dashboard.Projects, err
}

func (a *App) GetProject(code string) (ProjectDetail, error) {
	project, err := a.store.Resolve(code)
	if err != nil {
		return ProjectDetail{}, err
	}
	runtimeState, scanErr := a.manager.Scan(project)
	if scanErr != nil {
		runtimeState = model.Runtime{ProjectCode: project.Code, LastScanAt: time.Now()}
	}
	return ProjectDetail{Project: project, Runtime: runtimeState}, nil
}

func (a *App) SaveProject(project model.Project) (ProjectDetail, error) {
	if err := a.store.Save(project); err != nil {
		return ProjectDetail{}, err
	}
	_ = audit.Write(a.root, audit.Entry{Command: "gui", Target: project.Code, Action: "save_project", Result: "success"})
	return a.GetProject(project.Code)
}

func (a *App) DeleteProject(code string) error {
	project, err := a.store.Resolve(code)
	if err != nil {
		return err
	}
	if err := a.store.Delete(project.Code); err != nil {
		return err
	}
	return audit.Write(a.root, audit.Entry{Command: "gui", Target: project.Code, Action: "delete_project", Confirmations: 1, Result: "success"})
}

func (a *App) Scan() (Dashboard, error) {
	_, err := a.manager.ScanAll()
	if err != nil {
		return Dashboard{}, err
	}
	_ = audit.Write(a.root, audit.Entry{Command: "gui", Action: "scan", Result: "success"})
	return a.GetDashboard()
}

func (a *App) StartTarget(target string) (OperationResult, error) {
	return a.operate("start", target, false)
}

func (a *App) StopTarget(target string, force bool) (OperationResult, error) {
	return a.operate("stop", target, force)
}

func (a *App) RestartTarget(target string, force bool) (OperationResult, error) {
	return a.operate("restart", target, force)
}

func (a *App) ListProcesses() ([]ProcessDTO, error) {
	processes, err := a.scanner.Processes()
	if err != nil {
		return nil, err
	}
	result := make([]ProcessDTO, 0, len(processes))
	for _, process := range processes {
		if len(process.Ports) == 0 && !isRelevantProcess(process.Command+" "+process.Name) {
			continue
		}
		result = append(result, ProcessDTO{
			PID: process.PID, Name: process.Name, Command: process.Command, CWD: process.CWD, User: process.User, Ports: process.Ports,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PID < result[j].PID })
	return result, nil
}

func (a *App) GetEnvironment() []EnvironmentItem {
	return environmentReport()
}

func (a *App) GetSettings() (config.Config, error) {
	return config.Load(a.root)
}

func (a *App) SaveSettings(value config.Config) error {
	if err := config.Save(a.root, value); err != nil {
		return err
	}
	a.manager = service.New(a.root, a.store, a.scanner, time.Duration(value.StopTimeoutSeconds)*time.Second)
	return audit.Write(a.root, audit.Entry{Command: "gui", Action: "save_settings", Result: "success"})
}

func (a *App) ReadLog(kind, project string, limit int) ([]string, error) {
	if limit < 1 || limit > 5000 {
		limit = 500
	}
	var path string
	switch kind {
	case "system":
		path = filepath.Join(a.root, "data", "logs", "system.log")
	case "audit":
		path = filepath.Join(a.root, "data", "logs", "audit.log")
	case "project":
		resolved, err := a.store.Resolve(project)
		if err != nil {
			return nil, err
		}
		path = filepath.Join(a.store.ProjectDir(resolved.Code), "logs", "project.log")
	default:
		return nil, fmt.Errorf("unknown log kind %q", kind)
	}
	return tailLines(path, limit)
}

func (a *App) PrepareDeploy(projectCode, ruleName string) (DeployPreview, error) {
	project, err := a.store.Resolve(projectCode)
	if err != nil {
		return DeployPreview{}, err
	}
	var selected *model.DeployRule
	for i := range project.DeployRules {
		if project.DeployRules[i].Name == ruleName {
			selected = &project.DeployRules[i]
			break
		}
	}
	if selected == nil {
		return DeployPreview{}, fmt.Errorf("deploy rule %q not found", ruleName)
	}
	plan, err := a.deployer.Prepare(project, *selected)
	if err != nil {
		return DeployPreview{}, err
	}
	id := fmt.Sprintf("%s-%d", project.Code, time.Now().UnixNano())
	a.mu.Lock()
	a.plans[id] = plan
	a.mu.Unlock()
	return DeployPreview{
		ID: id, ProjectCode: project.Code, Rule: selected.Name, SourcePath: plan.SourcePath,
		ContentRoot: plan.ContentRoot, Changes: plan.Changes, DefaultBackup: selected.Backup,
	}, nil
}

func (a *App) ApplyDeploy(id string, backup bool) (DeployResult, error) {
	a.mu.Lock()
	plan := a.plans[id]
	delete(a.plans, id)
	a.mu.Unlock()
	if plan == nil {
		return DeployResult{}, fmt.Errorf("deployment preview expired")
	}
	defer plan.Close()
	result, err := a.deployer.Apply(plan, backup)
	if err != nil {
		_ = audit.Write(a.root, audit.Entry{Command: "gui", Target: plan.Project.Code, Action: "deploy", Confirmations: 1, Result: "failed", Error: err.Error()})
		return DeployResult{}, err
	}
	_ = audit.Write(a.root, audit.Entry{Command: "gui", Target: plan.Project.Code, Action: "deploy", Confirmations: 1, Result: "success"})
	return DeployResult{Rule: plan.Rule.Name, Changes: len(result.Changes), BackupPaths: result.BackupPaths}, nil
}

func (a *App) CancelDeploy(id string) {
	a.mu.Lock()
	plan := a.plans[id]
	delete(a.plans, id)
	a.mu.Unlock()
	if plan != nil {
		plan.Close()
	}
}

func (a *App) operate(action, target string, force bool) (OperationResult, error) {
	project, group, component, err := a.resolveTarget(target)
	if err != nil {
		return OperationResult{}, err
	}
	var results []service.Result
	switch action {
	case "start":
		results = a.manager.StartProject(project, group, component)
	case "stop":
		results = a.manager.StopProject(project, group, component, force)
	case "restart":
		results = a.manager.StopProject(project, group, component, force)
		if service.ResultsError(results) == nil {
			results = append(results, a.manager.StartProject(project, group, component)...)
		}
	}
	runtimeState, scanErr := a.manager.Scan(project)
	if scanErr != nil {
		return OperationResult{Results: results}, scanErr
	}
	resultName := "success"
	operationErr := service.ResultsError(results)
	if operationErr != nil {
		resultName = "failed"
	}
	entry := audit.Entry{Command: "gui", Target: target, Action: action, Confirmations: 1, Result: resultName}
	if operationErr != nil {
		entry.Error = operationErr.Error()
	}
	_ = audit.Write(a.root, entry)
	return OperationResult{Results: results, Runtime: runtimeState}, operationErr
}

func (a *App) resolveTarget(value string) (model.Project, string, string, error) {
	if project, err := a.store.Resolve(value); err == nil {
		return project, "", "", nil
	}
	projects, err := a.store.List()
	if err != nil {
		return model.Project{}, "", "", err
	}
	var selected *model.Project
	remainder := ""
	for i := range projects {
		prefix := projects[i].Code + "-"
		if strings.HasPrefix(value, prefix) && (selected == nil || len(projects[i].Code) > len(selected.Code)) {
			copy := projects[i]
			selected = &copy
			remainder = strings.TrimPrefix(value, prefix)
		}
	}
	if selected == nil {
		return model.Project{}, "", "", fmt.Errorf("cannot resolve target %q", value)
	}
	if remainder == "backend" || remainder == "front" {
		return *selected, remainder, "", nil
	}
	for _, backend := range selected.Backends {
		if backend.Name == remainder {
			return *selected, "backend", remainder, nil
		}
	}
	for _, frontend := range selected.Frontends {
		if frontend.Name == remainder {
			return *selected, "front", remainder, nil
		}
	}
	return model.Project{}, "", "", fmt.Errorf("project %s has no component %q", selected.Code, remainder)
}

func (a *App) scanLoop() {
	scan := func() {
		_, _ = a.manager.ScanAll()
		a.mu.Lock()
		a.lastScan = time.Now()
		a.mu.Unlock()
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "runtime:updated")
		}
	}
	scan()
	appConfig, _ := config.Load(a.root)
	interval := time.Duration(appConfig.ScanIntervalSeconds) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			scan()
		case <-a.stop:
			return
		}
	}
}

func summarizeProject(project model.Project, runtimeState model.Runtime) ProjectSummary {
	summary := ProjectSummary{
		Code: project.Code, Name: project.Name, ManageMode: project.ManageMode,
		Status: project.OverallStatus(runtimeState), Backends: len(project.Backends), Frontends: len(project.Frontends),
		LastScanAt: runtimeState.LastScanAt,
	}
	ports := map[int]struct{}{}
	for _, process := range runtimeState.Processes {
		if process.Status == "running" && process.Type == "backend" {
			summary.RunningBackends++
		}
		if process.Status == "running" && process.Type == "frontend" {
			summary.RunningFrontends++
		}
		for _, port := range process.Ports {
			ports[port] = struct{}{}
		}
	}
	for port := range ports {
		summary.Ports = append(summary.Ports, port)
	}
	sort.Ints(summary.Ports)
	return summary
}

func windowsDataRoot() (string, error) {
	if value := os.Getenv("SMS_WINDOWS_ROOT"); value != "" {
		return filepath.Abs(value)
	}
	base := os.Getenv("ProgramData")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "ServiceManagementSystem"), nil
}

func initializeWindowsRoot(root string) error {
	for _, dir := range []string{
		"conf", filepath.Join("data", "projects"), filepath.Join("data", "runtime"),
		filepath.Join("data", "backups"), filepath.Join("data", "logs"), "templates", filepath.Join("tmp", "deploy"),
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(root, "conf", "app.yml")); errors.Is(err, os.ErrNotExist) {
		if err := config.Save(root, config.Default()); err != nil {
			return err
		}
	}
	logging := filepath.Join(root, "conf", "logging.yml")
	if _, err := os.Stat(logging); errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(logging, []byte("level: info\n"), 0o644)
	}
	return nil
}

func environmentReport() []EnvironmentItem {
	items := []struct {
		name     string
		command  string
		required bool
	}{
		{"PowerShell", "powershell.exe", true}, {"Java", "java.exe", false}, {"Python", "python.exe", false},
		{"Node", "node.exe", false}, {"Nginx", "nginx.exe", false}, {"Tar", "tar.exe", false}, {"7-Zip", "7z.exe", false},
	}
	result := make([]EnvironmentItem, 0, len(items))
	for _, item := range items {
		path, err := exec.LookPath(item.command)
		status := "missing"
		if err == nil {
			status = "ready"
		}
		result = append(result, EnvironmentItem{Name: item.name, Status: status, Path: path, Required: item.required})
	}
	return result
}

func readRecentActivity(root string, limit int) []ActivityItem {
	lines, err := tailLines(filepath.Join(root, "data", "logs", "audit.log"), limit)
	if err != nil {
		return []ActivityItem{}
	}
	result := make([]ActivityItem, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		var entry struct {
			Time   time.Time `json:"time"`
			Action string    `json:"action"`
			Target string    `json:"target"`
			Result string    `json:"result"`
			Error  string    `json:"error"`
		}
		if json.Unmarshal([]byte(lines[i]), &entry) == nil {
			result = append(result, ActivityItem(entry))
		}
	}
	return result
}

func tailLines(path string, limit int) ([]string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		if len(lines) == limit {
			copy(lines, lines[1:])
			lines[len(lines)-1] = scanner.Text()
		} else {
			lines = append(lines, scanner.Text())
		}
	}
	return lines, scanner.Err()
}

func isRelevantProcess(value string) bool {
	lower := strings.ToLower(value)
	for _, name := range []string{"java", "python", "node", "nginx", "dotnet", "go.exe"} {
		if strings.Contains(lower, name) {
			return true
		}
	}
	return false
}
