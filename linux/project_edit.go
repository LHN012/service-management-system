package main

import (
	// bufio 用来复用主循环里的按行输入能力。
	"bufio"
	// fmt 用来输出编辑前后的提示和错误。
	"fmt"
	// io 让编辑流程可以写入终端，也方便后面测试。
	"io"
	// os 用来重命名项目目录。
	"os"
	// filepath 用来拼接项目目录路径。
	"path/filepath"
	// strconv 用来编辑端口时做数字转换。
	"strconv"
	// strings 用来识别命令和处理空格。
	"strings"
)

// isEditProjectCommand 判断当前输入是不是编辑项目命令。
// 支持：
//
//	edit project demo
//	ed -pj demo
func isEditProjectCommand(fields []string) bool {
	if len(fields) != 3 {
		return false
	}

	first := strings.ToLower(fields[0])
	second := strings.ToLower(fields[1])

	return (first == "edit" && second == "project") || (first == "ed" && second == "-pj")
}

// isEditServiceCommand 判断当前输入是不是编辑指定服务命令。
// 支持：
//
//	edit service demo api
//	ed -svc demo api
func isEditServiceCommand(fields []string) bool {
	if len(fields) != 4 {
		return false
	}

	first := strings.ToLower(fields[0])
	second := strings.ToLower(fields[1])

	return (first == "edit" && second == "service") || (first == "ed" && second == "-svc")
}

// editProject 编辑项目本身。
// 当前项目只有 name 这个项目级字段，所以这里实际做的是重命名项目。
func editProject(scanner *bufio.Scanner, output io.Writer, root string, name string) error {
	if err := validateProjectName(name); err != nil {
		return err
	}

	project, _, err := loadProject(root, name)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "editing project: %s\n", project.Name)
	newName, err := promptWithDefault(scanner, output, "new project name", project.Name)
	if err != nil {
		return err
	}
	if err := validateProjectName(newName); err != nil {
		return err
	}

	return renameProject(output, root, name, newName)
}

func renameProject(output io.Writer, root string, oldName string, newName string) error {
	if err := validateProjectName(oldName); err != nil {
		return err
	}
	if err := validateProjectName(newName); err != nil {
		return err
	}
	project, _, err := loadProject(root, oldName)
	if err != nil {
		return err
	}
	if oldName == newName {
		fmt.Fprintf(output, "project unchanged: %s\n", oldName)
		return nil
	}

	if err := renameProjectDir(root, oldName, newName); err != nil {
		return err
	}
	project.Name = newName
	configPath, err := saveProject(root, project)
	if err != nil {
		if rollbackErr := renameProjectDir(root, newName, oldName); rollbackErr != nil {
			return fmt.Errorf("save renamed project: %v; rollback directory: %w", err, rollbackErr)
		}
		return fmt.Errorf("save renamed project: %w", err)
	}

	fmt.Fprintf(output, "project renamed: %s -> %s\n", oldName, newName)
	fmt.Fprintf(output, "project saved: %s\n", configPath)
	return nil
}

// editService 编辑某个项目下的指定服务。
func editService(scanner *bufio.Scanner, output io.Writer, root string, projectName string, serviceName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}
	if err := validateServiceName(serviceName); err != nil {
		return err
	}

	project, _, err := loadProject(root, projectName)
	if err != nil {
		return err
	}

	index := findServiceIndex(project, serviceName)
	if index < 0 {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	service := project.Services[index]
	fmt.Fprintf(output, "editing service: %s/%s\n", project.Name, service.Name)

	newName, err := promptWithDefault(scanner, output, "name", service.Name)
	if err != nil {
		return err
	}
	if err := validateServiceName(newName); err != nil {
		return err
	}
	if newName != service.Name && findServiceIndex(project, newName) >= 0 {
		return fmt.Errorf("service already exists: %s", newName)
	}

	newStartPath, err := promptWithDefault(scanner, output, "start path", service.StartPath)
	if err != nil {
		return err
	}
	if newStartPath != service.StartPath {
		newStartPath, err = normalizeExistingStartPath(newStartPath)
		if err != nil {
			return err
		}
	}

	newPort, err := promptPortWithDefault(scanner, output, service.Port)
	if err != nil {
		return err
	}

	commandLabel := "start command (empty keep, auto for inferred)"
	newStartCommand, err := prompt(scanner, output, fmt.Sprintf("%s [%s]: ", commandLabel, service.StartCommand))
	if err != nil {
		return err
	}

	newRestartMode, newRestartCommand, err := inputRestartStrategyWithDefault(scanner, output, service, newStartPath)
	if err != nil {
		return err
	}

	service.Name = newName
	service.StartPath = newStartPath
	service.Port = newPort

	switch {
	case newStartCommand == "":
		if service.CommandSource == "auto" && newStartPath != project.Services[index].StartPath {
			service.StartCommand, err = inferStartCommand(service.StartPath)
			if err != nil {
				return err
			}
		}
	case strings.EqualFold(newStartCommand, "auto"):
		service.StartCommand, err = inferStartCommand(service.StartPath)
		if err != nil {
			return err
		}
		service.CommandSource = "auto"
	default:
		service.StartCommand = newStartCommand
		service.CommandSource = "custom"
	}
	service.RestartMode = newRestartMode
	service.RestartCommand = newRestartCommand

	project.Services[index] = service

	configPath, err := saveProject(root, project)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "service saved: %s\n", configPath)
	return nil
}

// promptWithDefault 读取一个字段；直接回车时保留原值。
func promptWithDefault(scanner *bufio.Scanner, output io.Writer, label string, current string) (string, error) {
	value, err := prompt(scanner, output, fmt.Sprintf("%s [%s]: ", label, current))
	if err != nil {
		return "", err
	}
	if value == "" {
		return current, nil
	}
	return value, nil
}

// promptPortWithDefault 读取端口；直接回车时保留原值。
func promptPortWithDefault(scanner *bufio.Scanner, output io.Writer, current int) (int, error) {
	value, err := prompt(scanner, output, fmt.Sprintf("port [%d]: ", current))
	if err != nil {
		return 0, err
	}
	if value == "" {
		return current, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("port must be a number")
	}
	if err := validatePort(port); err != nil {
		return 0, err
	}
	return port, nil
}

// inputRestartStrategyWithDefault 编辑已有服务的重启策略。
func inputRestartStrategyWithDefault(scanner *bufio.Scanner, output io.Writer, service Service, startPath string) (string, string, error) {
	inferredMode, inferredCommand := inferRestartCommand(startPath)
	defaultMode := service.RestartMode
	if defaultMode == "" {
		defaultMode = inferredMode
	}

	mode, err := promptWithDefault(scanner, output, "restart mode (kill-start/command)", defaultMode)
	if err != nil {
		return "", "", err
	}
	mode, err = normalizeRestartMode(mode)
	if err != nil {
		return "", "", err
	}

	if mode != "command" {
		return mode, "", nil
	}

	defaultCommand := service.RestartCommand
	if defaultCommand == "" {
		defaultCommand = inferredCommand
	}
	command, err := promptWithDefault(scanner, output, "restart command", defaultCommand)
	if err != nil {
		return "", "", err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", "", fmt.Errorf("restart command is required when restart mode is command")
	}

	return mode, command, nil
}

// findServiceIndex 按服务名查找服务位置。
func findServiceIndex(project Project, name string) int {
	for index, service := range project.Services {
		if service.Name == name {
			return index
		}
	}
	return -1
}

// renameProjectDir 重命名项目目录。
func renameProjectDir(root string, oldName string, newName string) error {
	oldDir := filepath.Join(root, "projects", oldName)
	newDir := filepath.Join(root, "projects", newName)

	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("project already exists: %s", newName)
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.Rename(oldDir, newDir)
}
