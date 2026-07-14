package main

import (
	// bufio 用来复用主循环里的按行输入能力。
	"bufio"
	// fmt 用来输出绑定过程和错误。
	"fmt"
	// io 让绑定流程可以写入终端，也方便后面测试。
	"io"
	// os 用来读取真实终端按键。
	"os"
	// strconv 用来把用户输入的服务编号转成数字。
	"strconv"
	// strings 用来识别命令、拆分服务选择。
	"strings"
)

// isBindServiceCommand 判断当前输入是不是绑定服务管理命令。
// 支持：
//
//	bind service demo
//	bd -svc demo
func isBindServiceCommand(fields []string) bool {
	if len(fields) != 3 {
		return false
	}

	first := strings.ToLower(fields[0])
	second := strings.ToLower(fields[1])

	return (first == "bind" && second == "service") || (first == "bd" && second == "-svc")
}

// bindServices 选择项目里的服务，启用或关闭 sms 管理。
func bindServices(scanner *bufio.Scanner, output io.Writer, root string, projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	project, _, err := loadProject(root, projectName)
	if err != nil {
		return err
	}
	if len(project.Services) == 0 {
		return fmt.Errorf("project has no services")
	}

	indexes, err := selectServices(scanner, output, project)
	if err != nil {
		return err
	}

	selected := make(map[int]bool)
	for _, index := range indexes {
		selected[index] = true
	}

	for index := range project.Services {
		project.Services[index].Managed = selected[index]
	}

	configPath, err := saveProject(root, project)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "service bindings saved: %s\n", configPath)
	return nil
}

// printServiceBindTable 展示可绑定服务。
func printServiceBindTable(output io.Writer, project Project) {
	fmt.Fprintf(output, "%-5s %-16s %-8s\n", "index", "name", "managed")
	for index, service := range project.Services {
		fmt.Fprintf(
			output,
			"%-5d %-16s %-8t\n",
			index+1,
			service.Name,
			service.Managed,
		)
	}
}

// selectServices 选择需要启用管理的服务。
// 真实终端里使用上下键和空格；非终端输入时回退到文本选择，方便脚本和测试。
func selectServices(scanner *bufio.Scanner, output io.Writer, project Project) ([]int, error) {
	fmt.Fprintf(output, "services of %s:\n", project.Name)

	if isInteractiveTerminal() {
		return selectServicesWithKeyboard(output, project)
	}

	printServiceBindTable(output, project)
	selection, err := promptRequired(scanner, output, "select services (name/index/comma/all): ")
	if err != nil {
		return nil, err
	}
	return parseServiceSelection(project, selection)
}

// isInteractiveTerminal 判断当前是否可以读取真实按键。
func isInteractiveTerminal() bool {
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

// selectServicesWithKeyboard 使用上下键和空格进行多选。
func selectServicesWithKeyboard(output io.Writer, project Project) ([]int, error) {
	oldState, err := enterRawMode(os.Stdin)
	if err != nil {
		return nil, err
	}
	defer restoreTerminal(os.Stdin, oldState)

	selected := make([]bool, len(project.Services))
	for index, service := range project.Services {
		selected[index] = service.Managed
	}

	cursor := 0
	fmt.Fprintln(output, "Use ↑/↓ to move, Space to select, Enter to confirm.")
	fmt.Fprint(output, "\033[?25l")
	defer fmt.Fprint(output, "\033[?25h")

	renderServiceChoices(output, project, selected, cursor)
	buffer := make([]byte, 3)

	for {
		count, err := os.Stdin.Read(buffer)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			continue
		}

		switch {
		case buffer[0] == ' ':
			selected[cursor] = !selected[cursor]
		case buffer[0] == '\r' || buffer[0] == '\n':
			fmt.Fprintln(output)
			return selectedIndexes(selected)
		case count >= 3 && buffer[0] == 0x1b && buffer[1] == '[' && buffer[2] == 'A':
			if cursor > 0 {
				cursor--
			}
		case count >= 3 && buffer[0] == 0x1b && buffer[1] == '[' && buffer[2] == 'B':
			if cursor < len(project.Services)-1 {
				cursor++
			}
		}

		fmt.Fprintf(output, "\033[%dA", len(project.Services))
		renderServiceChoices(output, project, selected, cursor)
	}
}

// renderServiceChoices 渲染可选服务行。
func renderServiceChoices(output io.Writer, project Project, selected []bool, cursor int) {
	for index, service := range project.Services {
		pointer := "  "
		if index == cursor {
			pointer = "> "
		}

		marker := "[ ]"
		if selected[index] {
			marker = "[x]"
		}

		fmt.Fprintf(output, "\033[2K\r%s%s %s\n", pointer, marker, service.Name)
	}
}

// selectedIndexes 返回已选择的服务下标。
func selectedIndexes(selected []bool) ([]int, error) {
	indexes := make([]int, 0)
	for index, ok := range selected {
		if ok {
			indexes = append(indexes, index)
		}
	}
	if len(indexes) == 0 {
		return nil, fmt.Errorf("no services selected")
	}
	return indexes, nil
}

// parseServiceSelection 把用户输入的服务选择转换成服务下标。
// 支持 all、服务名、服务编号，以及逗号分隔的混合输入。
func parseServiceSelection(project Project, selection string) ([]int, error) {
	selection = strings.TrimSpace(selection)
	if strings.EqualFold(selection, "all") {
		indexes := make([]int, 0, len(project.Services))
		for index := range project.Services {
			indexes = append(indexes, index)
		}
		return indexes, nil
	}

	seen := make(map[int]bool)
	indexes := make([]int, 0)

	for _, part := range strings.Split(selection, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		index, err := serviceSelectionIndex(project, part)
		if err != nil {
			return nil, err
		}
		if !seen[index] {
			seen[index] = true
			indexes = append(indexes, index)
		}
	}

	if len(indexes) == 0 {
		return nil, fmt.Errorf("no services selected")
	}
	return indexes, nil
}

// serviceSelectionIndex 解析单个服务选择项。
func serviceSelectionIndex(project Project, value string) (int, error) {
	if number, err := strconv.Atoi(value); err == nil {
		index := number - 1
		if index < 0 || index >= len(project.Services) {
			return -1, fmt.Errorf("service index out of range: %s", value)
		}
		return index, nil
	}

	index := findServiceIndex(project, value)
	if index < 0 {
		return -1, fmt.Errorf("service not found: %s", value)
	}
	return index, nil
}
