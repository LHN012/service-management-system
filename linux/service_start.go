package main

import (
	// bufio 用来在遇到已运行服务时逐个询问是否重启。
	"bufio"
	// encoding/json 用来保存每个服务的运行 PID。
	"encoding/json"
	// errors 用来区分 lsof 不存在和 lsof 未查到端口。
	"errors"
	// fmt 用来输出启动结果。
	"fmt"
	// io 让启动流程可以写入终端，也方便后面测试。
	"io"
	// os 用来创建日志文件、读取 /proc。
	"os"
	// exec 用来执行服务启动命令、kill、lsof。
	"os/exec"
	// filepath 用来拼接 runtime 和日志路径。
	"path/filepath"
	// strconv 用来处理 PID。
	"strconv"
	// strings 用来识别命令和处理用户确认。
	"strings"
	// time 用来给启动命令一点存活检测时间。
	"time"
)

// ProjectRuntime 保存一个项目里由 sms 启动过的服务 PID。
type ProjectRuntime struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Services      map[string]ServiceRuntime `json:"services"`
}

const currentRuntimeSchemaVersion = 1

// ServiceRuntime 保存单个服务的运行信息。
type ServiceRuntime struct {
	ProcessIdentity
	PID       int    `json:"pid"`
	Command   string `json:"command"`
	LogPath   string `json:"logPath"`
	StartedAt string `json:"startedAt"`
}

var errTerminationTimeout = errors.New("process did not exit after SIGTERM")

// isStartCommand 判断当前输入是不是启动服务命令。
// 支持：
//
//	st <project>
//	st -all <project>
//	st -i <project> <service>
func isStartCommand(fields []string) bool {
	if len(fields) != 2 && len(fields) != 3 && len(fields) != 4 {
		return false
	}
	if fields[0] != "st" && fields[0] != "start" {
		return false
	}
	if len(fields) == 2 {
		return true
	}
	if len(fields) == 3 {
		return fields[1] == "-all"
	}
	return fields[1] == "-i"
}

// startServices 按命令参数选择服务并启动。
func startServices(scanner *bufio.Scanner, output io.Writer, root string, fields []string) error {
	projectName, services, err := selectServicesForLifecycle(root, fields, "st")
	if err != nil {
		return err
	}

	runtime, err := loadProjectRuntime(root, projectName)
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return fmt.Errorf("no services selected")
	}

	var startErrors []error
	for _, service := range services {
		if err := startOneService(scanner, output, root, projectName, service, runtime); err != nil {
			startErrors = append(startErrors, fmt.Errorf("start %s: %w", service.Name, err))
			continue
		}
	}

	if err := saveProjectRuntime(root, projectName, runtime); err != nil {
		startErrors = append(startErrors, fmt.Errorf("save runtime: %w", err))
	}
	return errors.Join(startErrors...)
}

// selectServicesForLifecycle 按命令参数选择服务。
// st/sp <project> 使用 managed=true 的服务；-all 使用全部服务；-i 使用指定服务。
func selectServicesForLifecycle(root string, fields []string, commandName string) (string, []Service, error) {
	projectName := ""
	services := []Service{}

	switch {
	case len(fields) == 2:
		projectName = fields[1]
		project, err := loadProjectForStart(root, projectName)
		if err != nil {
			return "", nil, err
		}
		services = managedServices(project)
		if len(services) == 0 {
			return "", nil, fmt.Errorf("project has no managed services, run bd -svc %s first", project.Name)
		}
	case len(fields) == 3 && fields[1] == "-all":
		projectName = fields[2]
		project, err := loadProjectForStart(root, projectName)
		if err != nil {
			return "", nil, err
		}
		services = project.Services
	case len(fields) == 4 && fields[1] == "-i":
		projectName = fields[2]
		project, err := loadProjectForStart(root, projectName)
		if err != nil {
			return "", nil, err
		}
		index := findServiceIndex(project, fields[3])
		if index < 0 {
			return "", nil, fmt.Errorf("service not found: %s", fields[3])
		}
		services = []Service{project.Services[index]}
	default:
		return "", nil, fmt.Errorf("usage: %s <project> | %s -all <project> | %s -i <project> <service>", commandName, commandName, commandName)
	}

	return projectName, services, nil
}

func loadProjectForStart(root string, projectName string) (Project, error) {
	if err := validateProjectName(projectName); err != nil {
		return Project{}, err
	}
	project, _, err := loadProject(root, projectName)
	return project, err
}

// managedServices 只返回启用了 sms 管理的服务。
func managedServices(project Project) []Service {
	services := make([]Service, 0)
	for _, service := range project.Services {
		if service.Managed {
			services = append(services, service)
		}
	}
	return services
}

// startOneService 启动一个服务；如果已有 PID 存活，则先询问是否重启。
func startOneService(scanner *bufio.Scanner, output io.Writer, root string, projectName string, service Service, runtime ProjectRuntime) error {
	proceed, err := prepareServicePort(scanner, output, root, projectName, service, runtime)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	result, err := startProcess(root, projectName, service)
	if err != nil {
		return err
	}
	runtime.Services[service.Name] = result
	if err := saveProjectRuntime(root, projectName, runtime); err != nil {
		delete(runtime.Services, service.Name)
		cleanupErr := cleanupFailedStart(result)
		return errors.Join(fmt.Errorf("save runtime: %w", err), cleanupErr)
	}

	fmt.Fprintf(output, "started %s, pid=%d, log=%s\n", service.Name, result.PID, result.LogPath)
	printLsof(output, result.PID)
	return nil
}

// prepareServicePort 启动前检查端口。
// 端口被占用时，按服务重启策略决定执行重启命令或杀端口后启动。
func prepareServicePort(scanner *bufio.Scanner, output io.Writer, root string, projectName string, service Service, runtime ProjectRuntime) (bool, error) {
	if err := validatePort(service.Port); err != nil {
		return false, fmt.Errorf("service %s port invalid: %w", service.Name, err)
	}

	pids, err := portListeningPIDs(service.Port)
	if err != nil {
		return false, err
	}
	if len(pids) == 0 {
		if recorded, ok := runtime.Services[service.Name]; ok {
			if serviceRuntimeAlive(recorded) {
				if _, verifyErr := verifyProcessIdentity(recorded.PID, recorded.ProcessIdentity); verifyErr != nil {
					return false, fmt.Errorf("stale runtime for %s: %w", service.Name, verifyErr)
				}
				return false, fmt.Errorf("recorded process for %s is still running as pid %d but is not listening on port %d", service.Name, recorded.PID, service.Port)
			}
			delete(runtime.Services, service.Name)
		}
		return true, nil
	}

	fmt.Fprintf(output, "port %d is occupied by pid(s): %s\n", service.Port, joinInts(pids))
	owners, err := configuredPortOwners(root, service.Port)
	if err != nil {
		return false, err
	}
	if len(owners) > 0 {
		fmt.Fprintf(output, "configured service(s) using port %d:\n", service.Port)
		for _, owner := range owners {
			fmt.Fprintf(output, "  %s\n", owner)
		}
	}

	if serviceRestartMode(service) == "command" {
		command := strings.TrimSpace(serviceRestartCommand(service))
		if command == "" {
			return false, fmt.Errorf("restart command is empty")
		}

		confirmed, err := confirm(scanner, output, fmt.Sprintf("run restart command for %s/%s", projectName, service.Name))
		if err != nil {
			return false, err
		}
		if !confirmed {
			fmt.Fprintf(output, "skip %s, port %d is still occupied\n", service.Name, service.Port)
			return false, nil
		}

		if err := runServiceCommand(command, serviceWorkDir(service)); err != nil {
			return false, err
		}
		if err := waitPortOccupied(service.Port, 5*time.Second); err != nil {
			return false, err
		}

		fmt.Fprintf(output, "restart command completed for %s/%s, port=%d\n", projectName, service.Name, service.Port)
		for _, pid := range pids {
			printLsof(output, pid)
		}
		return false, nil
	}

	recorded, ok := runtime.Services[service.Name]
	if !ok || !containsPID(pids, recorded.PID) {
		return false, fmt.Errorf("port %d is occupied by an unmanaged process; SMS will not stop it", service.Port)
	}
	if _, err := verifyProcessIdentity(recorded.PID, recorded.ProcessIdentity); err != nil {
		return false, err
	}

	confirmed, err := confirm(scanner, output, fmt.Sprintf("stop the verified process for %s/%s and start it again", projectName, service.Name))
	if err != nil {
		return false, err
	}
	if !confirmed {
		fmt.Fprintf(output, "skip %s, port %d is still occupied\n", service.Name, service.Port)
		return false, nil
	}

	allowForce, err := confirm(scanner, output, fmt.Sprintf("send SIGKILL if verified process %d ignores SIGTERM", recorded.PID))
	if err != nil {
		return false, err
	}
	if err := terminateRuntimeProcess(recorded, allowForce, 5*time.Second); err != nil {
		return false, err
	}
	delete(runtime.Services, service.Name)
	if err := waitPortFree(service.Port, 5*time.Second); err != nil {
		return false, fmt.Errorf("verified process stopped but %w", err)
	}

	return true, nil
}

// runServiceCommand 执行用户配置的重启命令。
func runServiceCommand(command string, workDir string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workDir
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}

// startProcess 真正执行服务启动命令。
func startProcess(root string, projectName string, service Service) (ServiceRuntime, error) {
	if strings.TrimSpace(service.StartCommand) == "" {
		return ServiceRuntime{}, fmt.Errorf("start command is empty")
	}
	if _, err := os.Stat(service.StartPath); err != nil {
		return ServiceRuntime{}, fmt.Errorf("start path %s: %w", service.StartPath, err)
	}

	logPath := filepath.Join(root, "projects", projectName, "logs", service.Name+".log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return ServiceRuntime{}, err
	}

	script := buildDetachedStartScript(service.StartCommand, logPath)
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = serviceWorkDir(service)
	configureServiceProcess(cmd)

	data, err := cmd.CombinedOutput()
	if err != nil {
		return ServiceRuntime{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(data)))
	}

	pid, err := parsePID(data)
	if err != nil {
		return ServiceRuntime{}, err
	}

	identity, err := captureProcessIdentity(pid)
	if err != nil {
		return ServiceRuntime{}, err
	}
	runtime := ServiceRuntime{
		ProcessIdentity: identity,
		PID:             pid,
		Command:         service.StartCommand,
		LogPath:         logPath,
		StartedAt:       time.Now().Format(time.RFC3339),
	}

	time.Sleep(800 * time.Millisecond)

	if !processAlive(pid) {
		return ServiceRuntime{}, fmt.Errorf("process exited immediately, see log: %s", logPath)
	}
	if err := waitForServicePort(pid, service.Port, 15*time.Second); err != nil {
		startErr := fmt.Errorf("%w, see log: %s", err, logPath)
		if cleanupErr := cleanupFailedStart(runtime); cleanupErr != nil {
			return ServiceRuntime{}, errors.Join(startErr, fmt.Errorf("cleanup failed start: %w", cleanupErr))
		}
		return ServiceRuntime{}, startErr
	}

	return runtime, nil
}

// buildDetachedStartScript 构造生产环境更合适的后台启动脚本。
// nohup 让服务不依赖堡垒机/SSH 会话，echo $! 用来拿到后台服务 PID。
func buildDetachedStartScript(command string, logPath string) string {
	return "nohup sh -c " + shellQuote("exec "+command) +
		" >> " + shellQuote(logPath) +
		" 2>&1 < /dev/null & echo $!"
}

// shellQuote 把字符串包成单引号形式，安全交给 sh -c 使用。
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// parsePID 从 echo $! 的输出中解析 PID。
func parsePID(data []byte) (int, error) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, fmt.Errorf("start command did not return pid")
	}

	fields := strings.Fields(text)
	pid, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return 0, fmt.Errorf("invalid pid output: %s", text)
	}
	return pid, nil
}

// serviceWorkDir 使用启动路径所在目录作为工作目录。
func serviceWorkDir(service Service) string {
	if filepath.IsAbs(service.StartPath) {
		if info, err := os.Stat(service.StartPath); err == nil && info.IsDir() {
			return service.StartPath
		}
		return filepath.Dir(service.StartPath)
	}
	return "."
}

// confirm 读取 y/yes 确认。
func confirm(scanner *bufio.Scanner, output io.Writer, label string) (bool, error) {
	answer, err := prompt(scanner, output, label+"? (y/N): ")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

// processAlive 判断 PID 是否仍在运行。
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run(); err != nil {
		return false
	}

	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err == nil && strings.Contains(string(data), ") Z ") {
		return false
	}

	return true
}

func terminateRuntimeProcess(runtime ServiceRuntime, force bool, timeout time.Duration) error {
	if _, err := verifyProcessIdentity(runtime.PID, runtime.ProcessIdentity); err != nil {
		return err
	}
	if err := signalServiceRuntime(runtime, false); err != nil {
		return err
	}
	if waitRuntimeExit(runtime, timeout) {
		return nil
	}
	if !force {
		return errTerminationTimeout
	}
	if processAlive(runtime.PID) {
		if _, err := verifyProcessIdentity(runtime.PID, runtime.ProcessIdentity); err != nil {
			return err
		}
	}
	if err := signalServiceRuntime(runtime, true); err != nil {
		return err
	}
	if !waitRuntimeExit(runtime, timeout) {
		return fmt.Errorf("process group %d is still running after SIGKILL", runtime.ProcessGroupID)
	}
	return nil
}

func cleanupFailedStart(runtime ServiceRuntime) error {
	if !serviceRuntimeAlive(runtime) {
		return nil
	}
	return terminateRuntimeProcess(runtime, true, 2*time.Second)
}

func waitRuntimeExit(runtime ServiceRuntime, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !serviceRuntimeAlive(runtime) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !serviceRuntimeAlive(runtime)
}

func containsPID(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

// printLsof 打印该 PID 的端口占用信息。
func printLsof(output io.Writer, pid int) {
	command := exec.Command("lsof", "-Pan", "-p", strconv.Itoa(pid), "-i")
	data, err := command.CombinedOutput()
	if err != nil {
		fmt.Fprintf(output, "lsof: no network ports found for pid=%d or lsof is unavailable\n", pid)
		return
	}
	fmt.Fprint(output, string(data))
}

// portListeningPIDs 返回正在监听指定 TCP 端口的 PID。
func portListeningPIDs(port int) ([]int, error) {
	command := exec.Command("lsof", "-tiTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-n", "-P")
	data, err := command.CombinedOutput()
	text := strings.TrimSpace(string(data))
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, fmt.Errorf("check port %d failed: lsof is unavailable", port)
		}
		if text == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("check port %d failed: %s", port, text)
	}
	if text == "" {
		return nil, nil
	}

	seen := make(map[int]bool)
	pids := make([]int, 0)
	for _, field := range strings.Fields(text) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		if !seen[pid] {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// waitForServicePort 等待新进程监听期望端口。
func waitForServicePort(pid int, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return fmt.Errorf("process exited before port %d was detected", port)
		}

		pids, err := portListeningPIDs(port)
		if err != nil {
			return err
		}
		for _, current := range pids {
			if current == pid {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("expected port %d was not detected for pid %d", port, pid)
}

// waitPortOccupied 等待端口处于监听状态。
func waitPortOccupied(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pids, err := portListeningPIDs(port)
		if err != nil {
			return err
		}
		if len(pids) > 0 {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("port %d is not listening after restart command", port)
}

// configuredPortOwners 根据配置找出使用同端口的服务。
func configuredPortOwners(root string, port int) ([]string, error) {
	projects, err := loadAllProjects(root)
	if err != nil {
		return nil, err
	}

	owners := make([]string, 0)
	for _, project := range projects {
		for _, service := range project.Services {
			if service.Port == port {
				owners = append(owners, project.Name+"/"+service.Name)
			}
		}
	}
	return owners, nil
}

// joinInts 把 PID 列表拼成逗号分隔字符串。
func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func runtimePath(root string, projectName string) string {
	return filepath.Join(root, "projects", projectName, "runtime.json")
}

func loadProjectRuntime(root string, projectName string) (ProjectRuntime, error) {
	runtime := ProjectRuntime{SchemaVersion: currentRuntimeSchemaVersion, Services: make(map[string]ServiceRuntime)}

	data, err := os.ReadFile(runtimePath(root, projectName))
	if os.IsNotExist(err) {
		return runtime, nil
	}
	if err != nil {
		return runtime, err
	}
	if err := json.Unmarshal(data, &runtime); err != nil {
		return runtime, err
	}
	if runtime.SchemaVersion == 0 {
		runtime.SchemaVersion = currentRuntimeSchemaVersion
	}
	if runtime.SchemaVersion != currentRuntimeSchemaVersion {
		return runtime, fmt.Errorf("unsupported runtime schema version: %d", runtime.SchemaVersion)
	}
	if runtime.Services == nil {
		runtime.Services = make(map[string]ServiceRuntime)
	}

	return runtime, nil
}

func saveProjectRuntime(root string, projectName string, runtime ProjectRuntime) error {
	runtime.SchemaVersion = currentRuntimeSchemaVersion
	data, err := marshalPrettyJSON(runtime)
	if err != nil {
		return err
	}

	path := runtimePath(root, projectName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0o644)
}
