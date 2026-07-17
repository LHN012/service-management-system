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
	rootArgs, commandArgs, err := splitInvocationArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if wantsStartupHelp(commandArgs) {
		printStartupUsage(os.Stdout)
		return
	}
	if wantsVersion(commandArgs) {
		printVersion(os.Stdout)
		return
	}

	interactive := len(commandArgs) == 0
	if len(commandArgs) > 0 && (strings.EqualFold(commandArgs[0], "shell") || strings.EqualFold(commandArgs[0], "sh")) {
		if len(commandArgs) != 1 {
			fmt.Fprintln(os.Stderr, "usage: sms [--root <directory>] shell")
			os.Exit(1)
		}
		interactive = true
	}

	root, err := resolveAppRoot(rootArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if wantsDoctor(commandArgs) {
		if err := runDoctor(os.Stdout, root); err != nil {
			os.Exit(1)
		}
		return
	}
	logger, err := newOperationLogger(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize operation log: %v\n", err)
		os.Exit(1)
	}
	mode := "command"
	if interactive {
		mode = "shell"
	}
	writeLogWarning(os.Stderr, logger.Log("sms.start", "", "success", "root="+root+" mode="+mode))

	var runErr error
	if interactive {
		runErr = run(os.Stdin, os.Stdout, root, logger)
	} else {
		runErr = runDirect(os.Stdin, os.Stdout, root, logger, commandArgs)
	}
	result := "success"
	detail := ""
	if runErr != nil {
		result = "failed"
		detail = runErr.Error()
	}
	writeLogWarning(os.Stderr, logger.Log("sms.stop", "", result, detail))
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

// run 是真正的交互模式。
// 它会一直显示 sms>，读取一行命令，然后根据命令执行不同逻辑。
func run(input io.Reader, output io.Writer, root string, logger *OperationLogger) error {
	// Scanner 可以从 input 中一行一行读取文本。
	// 在真实运行时，input 就是你的键盘输入。
	scanner := bufio.NewScanner(input)

	fmt.Fprintln(output, "SMS Linux")
	fmt.Fprintf(output, "Root: %s\n", root)
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
		request, commandErr := parseCommand(fields)
		if commandErr != nil {
			recordCommand(output, logger, "command.unknown", line, commandErr)
			fmt.Fprintf(output, "error: %v\n", commandErr)
			continue
		}
		if request.Action == "sms.exit" {
			recordCommand(output, logger, request.Action, line, nil)
			fmt.Fprintln(output, "bye")
			return nil
		}

		commandErr = executeCommand(scanner, output, root, request)
		recordCommand(output, logger, request.Action, line, commandErr)
		if commandErr != nil {
			fmt.Fprintf(output, "error: %v\n", commandErr)
		}
	}
}

func runDirect(input io.Reader, output io.Writer, root string, logger *OperationLogger, fields []string) error {
	command := strings.Join(fields, " ")
	request, err := parseCommand(fields)
	if err != nil {
		recordCommand(output, logger, "command.unknown", command, err)
		return err
	}
	if request.Action == "sms.exit" {
		recordCommand(output, logger, request.Action, command, nil)
		return nil
	}

	scanner := bufio.NewScanner(input)
	err = executeCommand(scanner, output, root, request)
	recordCommand(output, logger, request.Action, command, err)
	return err
}

// printHelp 负责输出帮助信息。
// 后面新增命令时，也应该把命令说明补到这里。
func printHelp(output io.Writer) {
	fmt.Fprintln(output, "Available commands:")
	fmt.Fprintln(output, "  project create <project>                 p new <project>")
	fmt.Fprintln(output, "  project list                             p ls")
	fmt.Fprintln(output, "  project show <project>                   p info <project>")
	fmt.Fprintln(output, "  project status <project>                 p status <project>")
	fmt.Fprintln(output, "  project rename <project> <new-name>      p mv <project> <new-name>")
	fmt.Fprintln(output, "  service add <project>                    s add <project>")
	fmt.Fprintln(output, "  service list <project>                   s ls <project>")
	fmt.Fprintln(output, "  service show <project> <service>         s info <project> <service>")
	fmt.Fprintln(output, "  service status <project> <service>       s status <project> <service>")
	fmt.Fprintln(output, "  service edit <project> <service>         s edit <project> <service>")
	fmt.Fprintln(output, "  service select <project>                 s sel <project>")
	fmt.Fprintln(output, "  project start <project> [--all]          p up <project> [-a]")
	fmt.Fprintln(output, "  project stop <project> [--all] [--force]")
	fmt.Fprintln(output, "                                           p down <project> [-a] [-f]")
	fmt.Fprintln(output, "  service start <project> <service>        s up <project> <service>")
	fmt.Fprintln(output, "  service stop <project> <service> [--force]")
	fmt.Fprintln(output, "                                           s down <project> <service> [-f]")
	fmt.Fprintln(output, "  deploy list <project>                    d ls <project>")
	fmt.Fprintln(output, "  deploy plan <project> <source> --target <path>")
	fmt.Fprintln(output, "  deploy apply <project> <source> --target <path> [--yes]")
	fmt.Fprintln(output, "                                           d apply <project> <source> -t <path> [-y]")
	fmt.Fprintln(output, "  doctor")
	fmt.Fprintln(output, "  version | v")
	fmt.Fprintln(output, "  help | h")
	fmt.Fprintln(output, "  exit | q")
}

func wantsStartupHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help")
}

func wantsVersion(args []string) bool {
	return len(args) == 1 && (strings.EqualFold(args[0], "version") || strings.EqualFold(args[0], "v"))
}

func wantsDoctor(args []string) bool {
	return len(args) == 1 && strings.EqualFold(args[0], "doctor")
}

func printStartupUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintln(output, "  sms [--root <directory>] [shell|sh]")
	fmt.Fprintln(output, "  sms [--root <directory>] <resource> <action> [arguments]")
	fmt.Fprintln(output, "Environment: SMS_ROOT can also specify the data directory.")
}

func splitInvocationArgs(args []string) ([]string, []string, error) {
	rootArgs := make([]string, 0, 2)
	index := 0
	for index < len(args) {
		argument := args[index]
		switch {
		case argument == "--root" || argument == "-r":
			if index+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s requires a directory", argument)
			}
			rootArgs = append(rootArgs, argument, args[index+1])
			index += 2
		case strings.HasPrefix(argument, "--root="):
			rootArgs = append(rootArgs, argument)
			index++
		default:
			return rootArgs, args[index:], nil
		}
	}
	return rootArgs, nil, nil
}

// resolveAppRoot uses an explicit root first, then SMS_ROOT, then the executable directory.
func resolveAppRoot(args []string) (string, error) {
	root := ""
	rootSet := false
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--root" || argument == "-r":
			if rootSet {
				return "", fmt.Errorf("root can only be specified once")
			}
			if index+1 >= len(args) {
				return "", fmt.Errorf("%s requires a directory", argument)
			}
			index++
			root = args[index]
			rootSet = true
		case strings.HasPrefix(argument, "--root="):
			if rootSet {
				return "", fmt.Errorf("root can only be specified once")
			}
			root = strings.TrimPrefix(argument, "--root=")
			rootSet = true
		default:
			return "", fmt.Errorf("unknown startup argument %q; use --help", argument)
		}
	}

	if rootSet && strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("SMS root directory is required")
	}
	if strings.TrimSpace(root) == "" {
		root = strings.TrimSpace(os.Getenv("SMS_ROOT"))
	}
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = executableAppRoot()
		if err != nil {
			return "", err
		}
	}
	return normalizeAppRoot(root)
}

func executableAppRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	return appRootFromExecutable(executable), nil
}

func appRootFromExecutable(executable string) string {
	return filepath.Dir(executable)
}

func normalizeAppRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("SMS root directory is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("SMS root %s: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("SMS root is not a directory: %s", absolute)
	}
	return filepath.Clean(absolute), nil
}

func recordCommand(output io.Writer, logger *OperationLogger, action string, command string, commandErr error) {
	result := "success"
	detail := ""
	if commandErr != nil {
		result = "failed"
		detail = commandErr.Error()
	}
	writeLogWarning(output, logger.Log(action, command, result, detail))
}

func writeLogWarning(output io.Writer, err error) {
	if err != nil {
		fmt.Fprintf(output, "warning: write operation log: %v\n", err)
	}
}
