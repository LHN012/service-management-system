package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"sms/internal/config"
	"sms/internal/deploy"
	"sms/internal/service"
	"sms/internal/store"
	"sms/internal/system"
	"sms/linux/internal/agent"
	"sms/linux/internal/setup"
)

const Version = "0.1.0"

var errExitShell = errors.New("exit shell")

type CLI struct {
	Root    string
	In      io.Reader
	Out     io.Writer
	Err     io.Writer
	Reader  *bufio.Reader
	Store   *store.Store
	Scanner *system.Scanner
	Deploy  *deploy.Engine
}

func New(root string, input io.Reader, output, errorOutput io.Writer) *CLI {
	projectStore := store.New(root)
	return &CLI{
		Root: root, In: input, Out: output, Err: errorOutput, Reader: bufio.NewReader(input),
		Store: projectStore, Scanner: system.NewScanner(), Deploy: deploy.New(root, projectStore),
	}
}

func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		c.printHelp()
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp()
		return nil
	case "version", "--version":
		fmt.Fprintf(c.Out, "sms %s\n", Version)
		return nil
	case "init":
		return c.runInit()
	case "start":
		return c.startAgent()
	case "stop":
		return c.stopAgent()
	case "restart":
		return c.restartAgent()
	case "status":
		return c.agentStatus()
	case "agent":
		return agent.Run(c.Root)
	case "enter":
		return c.enter()
	case "p":
		return c.projectCommand(args[1:])
	case "pr":
		return c.processCommand(args[1:])
	case "st", "sp", "rst":
		return c.lifecycleCommand(args[0], args[1:])
	case "dp":
		return c.deployCommand(args[1:])
	case "exit", "quit":
		return errExitShell
	default:
		return fmt.Errorf("unknown command %q; run 'help'", args[0])
	}
}

func (c *CLI) manager() (*service.Manager, error) {
	appConfig, err := config.Load(c.Root)
	if err != nil {
		return nil, err
	}
	return service.New(c.Root, c.Store, c.Scanner, time.Duration(appConfig.StopTimeoutSeconds)*time.Second), nil
}

func (c *CLI) runInit() error {
	report, err := setup.Initialize(c.Root)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "Initialized: %s\n\nEnvironment:\n", report.Root)
	missingRequired := false
	for _, dependency := range report.Dependencies {
		status := "missing"
		if dependency.Found {
			status = dependency.Path
		}
		kind := "optional"
		if dependency.Required {
			kind = "required"
			if !dependency.Found {
				missingRequired = true
			}
		}
		fmt.Fprintf(c.Out, "  %-10s %-8s %s\n", dependency.Name, kind, status)
	}
	if missingRequired {
		fmt.Fprintln(c.Out, "\nInitialization completed with missing required dependencies.")
	}
	return nil
}

func (c *CLI) startAgent() error {
	pid, err := agent.Start(c.Root)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "Agent started (pid %d). Business processes are not started automatically.\n", pid)
	return nil
}

func (c *CLI) stopAgent() error {
	if err := agent.Stop(c.Root); err != nil {
		return err
	}
	fmt.Fprintln(c.Out, "Agent stopped. Managed business processes were left running.")
	return nil
}

func (c *CLI) restartAgent() error {
	status, _ := agent.ReadStatus(c.Root)
	if status.Running {
		if err := agent.Stop(c.Root); err != nil {
			return err
		}
	}
	return c.startAgent()
}

func (c *CLI) agentStatus() error {
	status, err := agent.ReadStatus(c.Root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !status.Running {
		fmt.Fprintln(c.Out, "Agent: stopped")
		return nil
	}
	fmt.Fprintf(c.Out, "Agent: running (pid %d)\nStarted: %s\nLast scan: %s\nProjects: %d\n",
		status.PID, formatTime(status.StartedAt), formatTime(status.LastScanAt), status.Projects)
	return nil
}

func (c *CLI) enter() error {
	fmt.Fprintln(c.Out, "Service Management System shell. Type 'help' for commands, 'exit' to leave.")
	for {
		fmt.Fprint(c.Out, "sms> ")
		line, err := c.Reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			runErr := c.Run(strings.Fields(line))
			if errors.Is(runErr, errExitShell) {
				return nil
			}
			if runErr != nil {
				fmt.Fprintln(c.Err, "error:", runErr)
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

func (c *CLI) printHelp() {
	fmt.Fprintln(c.Out, `Service Management System (Linux)

Service:
  init                         initialize directories and environment report
  start | stop | restart       manage the background agent only
  status                       show agent status
  enter                        open the interactive shell

Projects:
  p -c                         create a project interactively
  p -l                         list projects and detected status
  p -s <project>               show project configuration and runtime
  p -e <project>               edit project.yml with $EDITOR
  p -d <project>               delete project management data

Processes:
  st|sp|rst <project>          start, stop, or restart a project
  st|sp|rst <project>-backend  operate all backends
  st|sp|rst <project>-front    operate all frontends
  st|sp|rst <project>-<name>   operate one configured component
  pr -l                        list detected processes and ports
  pr -p <port>                 show a port occupant
  pr -s                        rebuild all project runtime state

Deployment:
  dp <project> [rule]          preview and confirm deployment`)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "never"
	}
	return value.Format(time.RFC3339)
}
