package main

import (
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
	Name     string    `json:"name"`
	Services []Service `json:"services"`
}

// Service 表示项目里的一个服务。
// StartCommand 是 sms 最终会拿去启动服务的命令。
type Service struct {
	Name          string `json:"name"`
	StartPath     string `json:"startPath"`
	StartCommand  string `json:"startCommand"`
	CommandSource string `json:"commandSource"`
	Managed       bool   `json:"managed"`
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
	projectDir := filepath.Join(root, "projects", project.Name)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	configPath := filepath.Join(projectDir, "project.json")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return "", err
	}

	return configPath, nil
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
		return fmt.Sprintf("nginx -c %q", startPath), nil
	default:
		return "", fmt.Errorf("cannot infer start command from path: %s", startPath)
	}
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
	return nil
}
