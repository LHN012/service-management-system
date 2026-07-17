package main

import (
	// bufio 用来复用主循环里的按行输入能力。
	"bufio"
	// fmt 用来拼接错误信息和输出创建结果。
	"fmt"
	// io 让项目创建流程可以写入终端，也方便后面测试。
	"io"
	// os 用来创建目录和写文件。
	"os"
	// filepath 用来拼接 projects/<项目名>/project.json。
	"path/filepath"
	// strconv 用来把用户输入的服务数量从字符串转成数字。
	"strconv"
	// strings 用来处理空格和命令大小写。
	"strings"
)

// isCreateProjectCommand 判断当前输入是不是创建项目命令。
// 支持两种写法：
//
//	create project demo
//	cr -pj demo
func isCreateProjectCommand(fields []string) bool {
	if len(fields) != 3 {
		return false
	}

	first := strings.ToLower(fields[0])
	second := strings.ToLower(fields[1])

	return (first == "create" && second == "project") || (first == "cr" && second == "-pj")
}

// isAddServiceCommand 判断当前输入是不是新增服务命令。
// 支持：
//
//	add service demo
//	add -svc demo
func isAddServiceCommand(fields []string) bool {
	if len(fields) != 3 {
		return false
	}

	first := strings.ToLower(fields[0])
	second := strings.ToLower(fields[1])

	return first == "add" && (second == "service" || second == "-svc")
}

// createProject 按问题逐步收集项目配置，并保存到 projects/<项目名>/project.json。
func createProject(scanner *bufio.Scanner, output io.Writer, root string, name string) error {
	if err := validateProjectName(name); err != nil {
		return err
	}

	projectDir := filepath.Join(root, "projects", name)
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("project already exists: %s", name)
	}

	project := Project{Name: name, Services: []Service{}}
	configPath, err := saveProject(root, project)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "project created: %s\n", configPath)

	countText, err := prompt(scanner, output, "service count (0 to skip): ")
	if err != nil {
		return err
	}

	serviceCount, err := strconv.Atoi(countText)
	if err != nil || serviceCount < 0 {
		return fmt.Errorf("service count must be zero or a positive number")
	}

	project.Services = make([]Service, 0, serviceCount)

	for i := 1; i <= serviceCount; i++ {
		fmt.Fprintf(output, "\nservice %d/%d\n", i, serviceCount)

		service, err := inputService(scanner, output)
		if err != nil {
			return err
		}
		if findServiceIndex(project, service.Name) >= 0 {
			return fmt.Errorf("service already exists: %s", service.Name)
		}
		project.Services = append(project.Services, service)
		if _, err := saveProject(root, project); err != nil {
			return err
		}
		fmt.Fprintf(output, "service saved: %s\n", service.Name)
	}
	return nil
}

// addService 给已有项目新增一个服务。
func addService(scanner *bufio.Scanner, output io.Writer, root string, projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	project, _, err := loadProject(root, projectName)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "adding service to project: %s\n", project.Name)
	service, err := inputService(scanner, output)
	if err != nil {
		return err
	}
	if findServiceIndex(project, service.Name) >= 0 {
		return fmt.Errorf("service already exists: %s", service.Name)
	}

	project.Services = append(project.Services, service)

	configPath, err := saveProject(root, project)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "service added: %s\n", configPath)
	return nil
}

// prompt 输出一个问题，并读取用户输入的一行。
func prompt(scanner *bufio.Scanner, output io.Writer, label string) (string, error) {
	fmt.Fprint(output, label)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.ErrUnexpectedEOF
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// promptRequired 和 prompt 类似，但不允许用户直接回车。
func promptRequired(scanner *bufio.Scanner, output io.Writer, label string) (string, error) {
	value, err := prompt(scanner, output, label)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", strings.TrimSpace(label))
	}
	return value, nil
}

// inputService 录入一个服务。
// 创建项目和给已有项目新增服务都复用这套问题。
func inputService(scanner *bufio.Scanner, output io.Writer) (Service, error) {
	serviceName, err := promptRequired(scanner, output, "name: ")
	if err != nil {
		return Service{}, err
	}
	if err := validateServiceName(serviceName); err != nil {
		return Service{}, err
	}

	startPath, err := promptRequired(scanner, output, "start path: ")
	if err != nil {
		return Service{}, err
	}
	startPath, err = normalizeExistingStartPath(startPath)
	if err != nil {
		return Service{}, err
	}

	port, err := promptPort(scanner, output, "port: ")
	if err != nil {
		return Service{}, err
	}

	customCommand, err := prompt(scanner, output, "custom start command (empty for auto): ")
	if err != nil {
		return Service{}, err
	}

	startCommand := strings.TrimSpace(customCommand)
	commandSource := "custom"
	if startCommand == "" {
		startCommand, err = inferStartCommand(startPath)
		if err != nil {
			return Service{}, fmt.Errorf("service %s: %w", serviceName, err)
		}
		commandSource = "auto"
	}

	restartMode, restartCommand, err := inputRestartStrategy(scanner, output, startPath)
	if err != nil {
		return Service{}, err
	}

	return Service{
		Name:           serviceName,
		StartPath:      startPath,
		Port:           port,
		StartCommand:   startCommand,
		CommandSource:  commandSource,
		RestartMode:    restartMode,
		RestartCommand: restartCommand,
	}, nil
}

// promptPort 读取并校验端口。
func promptPort(scanner *bufio.Scanner, output io.Writer, label string) (int, error) {
	value, err := promptRequired(scanner, output, label)
	if err != nil {
		return 0, err
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

// inputRestartStrategy 录入端口占用时的处理策略。
// kill-start 表示杀端口后启动；command 表示执行配置的重启命令。
func inputRestartStrategy(scanner *bufio.Scanner, output io.Writer, startPath string) (string, string, error) {
	defaultMode, defaultCommand := inferRestartCommand(startPath)
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
