package main

import (
	// bufio 用来按行读取用户输入。
	"bufio"
	// fmt 用来打印文字。
	"fmt"
	// io 让 run 函数可以接收不同输入输出，后面写测试会方便。
	"io"
	// os 提供标准输入、标准输出、退出程序等能力。
	"os"
	// filepath 用来拼接不同系统上的文件路径。
	"path/filepath"
	// strings 用来处理用户输入里的空格和大小写。
	"strings"
)

// main 是 Go 程序的入口。
// 当你在终端执行 sms 时，系统最终会从这里开始跑。
func main() {
	// os.Stdin 表示终端输入，os.Stdout 表示终端输出。
	// run 返回错误时，打印到 os.Stderr，并用非 0 状态码退出。
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 是真正的交互模式。
// 它会一直显示 sms>，读取一行命令，然后根据命令执行不同逻辑。
func run(input io.Reader, output io.Writer) error {
	// Scanner 可以从 input 中一行一行读取文本。
	// 在真实运行时，input 就是你的键盘输入。
	scanner := bufio.NewScanner(input)
	root := appRoot()

	fmt.Fprintln(output, "Service Management System")
	fmt.Fprintln(output, "Type help for commands, exit to quit.")

	// for 后面什么条件都不写，表示无限循环。
	// 只有遇到 return，程序才会从这个循环里退出。
	for {
		fmt.Fprint(output, "sms> ")

		// Scan 会等待用户输入一行。
		// 如果读取失败或输入结束，比如按 Ctrl+D，就会返回 false。
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			fmt.Fprintln(output)
			return nil
		}

		// TrimSpace 去掉命令前后的空格。
		// 用户输入 "  help  " 时，也能当成 help 处理。
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			// 空命令不处理，直接回到下一轮 sms>。
			continue
		}

		// fields 会按空格拆分命令。
		// 例如 create project demo 会被拆成 3 段。
		fields := strings.Fields(line)
		command := strings.ToLower(fields[0])

		// switch 用来做命令分发。
		// 命令本身忽略大小写，项目名仍保留用户输入的原样。
		switch {
		case command == "help":
			printHelp(output)
		case command == "exit" || command == "quit":
			// return nil 表示正常结束 run，也就是退出 sms 模式。
			fmt.Fprintln(output, "bye")
			return nil
		case isCreateProjectCommand(fields):
			if err := createProject(scanner, output, root, fields[len(fields)-1]); err != nil {
				fmt.Fprintf(output, "create project failed: %v\n", err)
			}
		case isListProjectsCommand(fields):
			if err := listProjects(output, root, fields); err != nil {
				fmt.Fprintf(output, "list projects failed: %v\n", err)
			}
		case isEditProjectCommand(fields):
			if err := editProject(scanner, output, root, fields[len(fields)-1]); err != nil {
				fmt.Fprintf(output, "edit project failed: %v\n", err)
			}
		case isEditServiceCommand(fields):
			if err := editService(scanner, output, root, fields[2], fields[3]); err != nil {
				fmt.Fprintf(output, "edit service failed: %v\n", err)
			}
		case isAddServiceCommand(fields):
			if err := addService(scanner, output, root, fields[len(fields)-1]); err != nil {
				fmt.Fprintf(output, "add service failed: %v\n", err)
			}
		case isBindServiceCommand(fields):
			if err := bindServices(scanner, output, root, fields[len(fields)-1]); err != nil {
				fmt.Fprintf(output, "bind service failed: %v\n", err)
			}
		default:
			fmt.Fprintf(output, "unknown command: %s\n", line)
		}
	}
}

// printHelp 负责输出帮助信息。
// 后面新增命令时，也应该把命令说明补到这里。
func printHelp(output io.Writer) {
	fmt.Fprintln(output, "Available commands:")
	fmt.Fprintln(output, "  help        show this help")
	fmt.Fprintln(output, "  create project <name>")
	fmt.Fprintln(output, "  cr -pj <name>")
	fmt.Fprintln(output, "  list | ls")
	fmt.Fprintln(output, "  list -i <name> | ls -i <name>")
	fmt.Fprintln(output, "  list -s <name> | ls -s <name>")
	fmt.Fprintln(output, "  edit project <name> | ed -pj <name>")
	fmt.Fprintln(output, "  edit service <project> <service> | ed -svc <project> <service>")
	fmt.Fprintln(output, "  add service <project> | add -svc <project>")
	fmt.Fprintln(output, "  bind service <project> | bd -svc <project>")
	fmt.Fprintln(output, "  exit        leave sms mode")
	fmt.Fprintln(output, "  quit        leave sms mode")
}

// appRoot 返回 sms-linux 的工作目录。
// 在仓库根目录运行时，它会使用 ./sms-linux。
// 如果已经 cd 到 sms-linux 里运行，它会使用当前目录。
func appRoot() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}

	if filepath.Base(workingDir) == "sms-linux" {
		return workingDir
	}

	smsLinuxDir := filepath.Join(workingDir, "sms-linux")
	if _, err := os.Stat(smsLinuxDir); err == nil {
		return smsLinuxDir
	}

	return workingDir
}
