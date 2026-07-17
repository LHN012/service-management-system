package main

import (
	// fmt 用来格式化输出。
	"fmt"
	// io 让查看逻辑可以输出到终端，也方便后面测试。
	"io"
	// sort 用来让项目列表按名称稳定排序。
	"sort"
	// strings 用来识别命令。
	"strings"
)

// isListProjectsCommand 判断当前输入是不是项目查看命令。
// 支持：
//
//	list
//	ls
//	list -i demo
//	ls -s demo
func isListProjectsCommand(fields []string) bool {
	if len(fields) != 1 && len(fields) != 3 {
		return false
	}

	command := strings.ToLower(fields[0])
	if command != "list" && command != "ls" {
		return false
	}

	if len(fields) == 1 {
		return true
	}

	flag := strings.ToLower(fields[1])
	return flag == "-i" || flag == "-s"
}

// listProjects 根据参数选择展示方式。
// 无参数展示项目列表，-i 展示项目完整信息，-s 展示项目服务摘要。
func listProjects(output io.Writer, root string, fields []string) error {
	if len(fields) == 1 {
		return printProjectList(output, root)
	}

	name := fields[2]
	if err := validateProjectName(name); err != nil {
		return err
	}

	project, configPath, err := loadProject(root, name)
	if err != nil {
		return err
	}

	switch strings.ToLower(fields[1]) {
	case "-i":
		printProjectInfo(output, project, configPath)
	case "-s":
		printProjectServices(output, project)
	}

	return nil
}

func showProject(output io.Writer, root string, name string) error {
	if err := validateProjectName(name); err != nil {
		return err
	}
	project, configPath, err := loadProject(root, name)
	if err != nil {
		return err
	}
	printProjectInfo(output, project, configPath)
	return nil
}

func listProjectServices(output io.Writer, root string, name string) error {
	if err := validateProjectName(name); err != nil {
		return err
	}
	project, _, err := loadProject(root, name)
	if err != nil {
		return err
	}
	printProjectServices(output, project)
	return nil
}

func showService(output io.Writer, root string, projectName string, serviceName string) error {
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
	fmt.Fprintf(output, "Project: %s\n", project.Name)
	fmt.Fprintf(output, "Service: %s\n", service.Name)
	fmt.Fprintf(output, "Type: %s\n", serviceType(service.StartPath))
	fmt.Fprintf(output, "Port: %d\n", service.Port)
	fmt.Fprintf(output, "Start path: %s\n", service.StartPath)
	fmt.Fprintf(output, "Start command: %s\n", service.StartCommand)
	fmt.Fprintf(output, "Command source: %s\n", service.CommandSource)
	fmt.Fprintf(output, "Restart mode: %s\n", serviceRestartMode(service))
	if serviceRestartMode(service) == "command" {
		fmt.Fprintf(output, "Restart command: %s\n", serviceRestartCommand(service))
	}
	fmt.Fprintf(output, "Managed: %t\n", service.Managed)
	return nil
}

// printProjectList 展示所有已创建项目。
// 这一层用表格展示项目名、服务数量和服务名列表，适合快速扫一眼。
func printProjectList(output io.Writer, root string) error {
	projects, err := loadAllProjects(root)
	if err != nil {
		return err
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	if len(projects) == 0 {
		fmt.Fprintln(output, "no projects")
		return nil
	}

	fmt.Fprintf(output, "%-20s %-8s %s\n", "name", "services", "services-name")
	for _, project := range projects {
		fmt.Fprintf(
			output,
			"%-20s %-8d %s\n",
			project.Name,
			len(project.Services),
			strings.Join(serviceNames(project), ","),
		)
	}
	return nil
}

// printProjectInfo 展示项目完整信息。
// 这里适合确认项目配置有没有录错。
func printProjectInfo(output io.Writer, project Project, configPath string) {
	fmt.Fprintf(output, "Project: %s\n", project.Name)
	fmt.Fprintf(output, "Config: %s\n", configPath)
	fmt.Fprintf(output, "Services: %d\n", len(project.Services))

	for index, service := range project.Services {
		fmt.Fprintf(output, "\n[%d] %s\n", index+1, service.Name)
		fmt.Fprintf(output, "  type: %s\n", serviceType(service.StartPath))
		fmt.Fprintf(output, "  port: %d\n", service.Port)
		fmt.Fprintf(output, "  start path: %s\n", service.StartPath)
		fmt.Fprintf(output, "  start command: %s\n", service.StartCommand)
		fmt.Fprintf(output, "  command source: %s\n", service.CommandSource)
		fmt.Fprintf(output, "  restart mode: %s\n", serviceRestartMode(service))
		if serviceRestartMode(service) == "command" {
			fmt.Fprintf(output, "  restart command: %s\n", serviceRestartCommand(service))
		}
		fmt.Fprintf(output, "  managed: %t\n", service.Managed)
	}
}

// printProjectServices 展示服务摘要。
// 这一层比 -i 更短，后面加启动状态时可以在这里补 pid/status。
func printProjectServices(output io.Writer, project Project) {
	fmt.Fprintf(output, "Services of %s:\n", project.Name)
	for _, service := range project.Services {
		fmt.Fprintf(
			output,
			"  %s  type=%s  port=%d  managed=%t  restart=%s  source=%s\n",
			service.Name,
			serviceType(service.StartPath),
			service.Port,
			service.Managed,
			serviceRestartMode(service),
			service.CommandSource,
		)
	}
}

// serviceNames 提取项目里的服务名。
func serviceNames(project Project) []string {
	names := make([]string, 0, len(project.Services))
	for _, service := range project.Services {
		names = append(names, service.Name)
	}
	return names
}
