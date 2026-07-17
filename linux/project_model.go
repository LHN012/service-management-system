package main

import (
	// bytes 用来接收 JSON 编码结果。
	"bytes"
	// encoding/json 用来读取和保存 project.json。
	"encoding/json"
	// fmt 用来拼接错误信息。
	"fmt"
	// os 用来读取 project.json、创建项目目录和写文件。
	"os"
	// filepath 用来拼接 projects/<项目名>/project.json。
	"path/filepath"
	// strings 用来处理空格、路径后缀和项目名校验。
	"strings"
)

// Project 表示一个项目的配置。
// 现在先只保存项目名和它下面有哪些服务。
type Project struct {
	SchemaVersion int       `json:"schemaVersion"`
	Name          string    `json:"name"`
	Services      []Service `json:"services"`
}

const currentProjectSchemaVersion = 1

// Service 表示项目里的一个服务。
// StartCommand 是 sms 最终会拿去启动服务的命令。
type Service struct {
	Name           string `json:"name"`
	StartPath      string `json:"startPath"`
	Port           int    `json:"port"`
	StartCommand   string `json:"startCommand"`
	CommandSource  string `json:"commandSource"`
	RestartMode    string `json:"restartMode"`
	RestartCommand string `json:"restartCommand,omitempty"`
	Managed        bool   `json:"managed"`
}

// loadProject 读取某个项目的 project.json。
func loadProject(root string, name string) (Project, string, error) {
	configPath := filepath.Join(root, "projects", name, "project.json")

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return Project{}, configPath, fmt.Errorf("project not found: %s", name)
	}
	if err != nil {
		return Project{}, configPath, err
	}

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return Project{}, configPath, err
	}
	if project.Name == "" {
		project.Name = name
	}
	if project.SchemaVersion == 0 {
		project.SchemaVersion = currentProjectSchemaVersion
	}
	if project.SchemaVersion != currentProjectSchemaVersion {
		return Project{}, configPath, fmt.Errorf("unsupported project schema version: %d", project.SchemaVersion)
	}
	if project.Name != name {
		return Project{}, configPath, fmt.Errorf("project name %q does not match directory %q", project.Name, name)
	}
	if err := validateProject(project); err != nil {
		return Project{}, configPath, fmt.Errorf("invalid project configuration: %w", err)
	}

	return project, configPath, nil
}

// loadAllProjects 读取 projects 目录下所有有效项目。
func loadAllProjects(root string) ([]Project, error) {
	projectsRoot := filepath.Join(root, "projects")
	entries, err := os.ReadDir(projectsRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	projects := make([]Project, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		project, _, err := loadProject(root, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("%s invalid project: %w", entry.Name(), err)
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// saveProject 把项目配置写回 projects/<项目名>/project.json。
func saveProject(root string, project Project) (string, error) {
	project.SchemaVersion = currentProjectSchemaVersion
	if err := validateProject(project); err != nil {
		return "", err
	}
	projectDir := filepath.Join(root, "projects", project.Name)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", err
	}
	if err := ensureProjectDirs(projectDir); err != nil {
		return "", err
	}

	data, err := marshalPrettyJSON(project)
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(projectDir, "project.json")
	if err := atomicWriteFile(configPath, data, 0o644); err != nil {
		return "", err
	}

	return configPath, nil
}

// ensureProjectDirs 创建项目运行过程中会用到的固定目录。
func ensureProjectDirs(projectDir string) error {
	for _, name := range []string{"deploy-files", "backups", "logs"} {
		if err := os.MkdirAll(filepath.Join(projectDir, name), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// marshalPrettyJSON 输出带缩进、但不转义 && 这类命令字符的 JSON。
func marshalPrettyJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// inferStartCommand 根据启动路径自动生成启动命令。
// 先只兼容 jar、nginx 配置文件、Python 脚本。
func inferStartCommand(startPath string) (string, error) {
	lowerPath := strings.ToLower(startPath)

	switch {
	case strings.HasSuffix(lowerPath, ".jar"):
		return fmt.Sprintf("java -jar %q", startPath), nil
	case strings.HasSuffix(lowerPath, ".py"):
		return fmt.Sprintf("python3 %q", startPath), nil
	case strings.HasSuffix(lowerPath, ".conf") || strings.Contains(lowerPath, "nginx"):
		return fmt.Sprintf("nginx -c %q -g 'daemon off;'", startPath), nil
	default:
		return "", fmt.Errorf("cannot infer start command from path: %s", startPath)
	}
}

func normalizeExistingStartPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("start path is required")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if _, err := os.Stat(absolute); err != nil {
		return "", fmt.Errorf("start path %s: %w", absolute, err)
	}
	return absolute, nil
}

// inferRestartCommand 根据服务类型自动生成重启命令。
// Nginx 生产上优先 reload；其他服务默认杀端口再启动，不需要重启命令。
func inferRestartCommand(startPath string) (string, string) {
	if serviceType(startPath) == "nginx" {
		return "command", fmt.Sprintf("nginx -t -c %q && nginx -s reload -c %q", startPath, startPath)
	}
	return "kill-start", ""
}

// serviceRestartMode 返回服务的实际重启策略。
func serviceRestartMode(service Service) string {
	mode, err := normalizeRestartMode(service.RestartMode)
	if err != nil || mode == "" {
		mode, _ = inferRestartCommand(service.StartPath)
	}
	return mode
}

// serviceRestartCommand 返回服务的实际重启命令。
func serviceRestartCommand(service Service) string {
	if strings.TrimSpace(service.RestartCommand) != "" {
		return service.RestartCommand
	}
	_, command := inferRestartCommand(service.StartPath)
	return command
}

// serviceType 根据启动路径给服务打一个简单类型标签。
func serviceType(startPath string) string {
	lowerPath := strings.ToLower(startPath)

	switch {
	case strings.HasSuffix(lowerPath, ".jar"):
		return "jar"
	case strings.HasSuffix(lowerPath, ".py"):
		return "python"
	case strings.HasSuffix(lowerPath, ".conf") || strings.Contains(lowerPath, "nginx"):
		return "nginx"
	default:
		return "custom"
	}
}

// validateProjectName 做最基本的项目名保护。
// 项目名会变成目录名，所以不能包含路径穿越或常见路径分隔符。
func validateProjectName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("project name cannot contain ..")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return fmt.Errorf("project name contains invalid path characters")
	}
	return nil
}

// validateServiceName 做最基本的服务名保护。
// 服务名会出现在 edit service <项目名> <服务名> 这种命令里，所以先不允许空格。
func validateServiceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("service name is required")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return fmt.Errorf("service name cannot contain spaces")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\:*?"<>|`) {
		return fmt.Errorf("service name contains invalid path characters")
	}
	return nil
}

func validateProject(project Project) error {
	if err := validateProjectName(project.Name); err != nil {
		return err
	}
	seen := make(map[string]bool, len(project.Services))
	for _, service := range project.Services {
		if err := validateServiceName(service.Name); err != nil {
			return fmt.Errorf("service %q: %w", service.Name, err)
		}
		if seen[service.Name] {
			return fmt.Errorf("duplicate service name: %s", service.Name)
		}
		seen[service.Name] = true
		if strings.TrimSpace(service.StartPath) == "" {
			return fmt.Errorf("service %s: start path is required", service.Name)
		}
		if err := validatePort(service.Port); err != nil {
			return fmt.Errorf("service %s: %w", service.Name, err)
		}
		if strings.TrimSpace(service.StartCommand) == "" {
			return fmt.Errorf("service %s: start command is required", service.Name)
		}
		mode, err := normalizeRestartMode(service.RestartMode)
		if err != nil {
			return fmt.Errorf("service %s: %w", service.Name, err)
		}
		if mode == "command" && strings.TrimSpace(serviceRestartCommand(service)) == "" {
			return fmt.Errorf("service %s: restart command is required", service.Name)
		}
	}
	return nil
}

// validatePort 校验服务端口。
func validatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// normalizeRestartMode 规范化重启策略。
func normalizeRestartMode(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "kill", "kill-start":
		return "kill-start", nil
	case "command", "cmd":
		return "command", nil
	default:
		return "", fmt.Errorf("restart mode must be kill-start or command")
	}
}
