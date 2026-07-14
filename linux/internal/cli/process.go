package cli

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"sms/internal/audit"
	"sms/internal/model"
	"sms/internal/service"
)

func (c *CLI) processCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pr -l | -p <port> | -s")
	}
	switch args[0] {
	case "-l":
		processes, err := c.Scanner.Processes()
		if err != nil {
			return err
		}
		writer := tabwriter.NewWriter(c.Out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "PID\tUSER\tPORTS\tNAME\tCWD\tCOMMAND")
		for _, process := range processes {
			if len(process.Ports) == 0 && !isManagedRuntime(process.Command) {
				continue
			}
			fmt.Fprintf(writer, "%d\t%s\t%v\t%s\t%s\t%s\n", process.PID, process.User, process.Ports, process.Name, process.CWD, process.Command)
		}
		return writer.Flush()
	case "-p":
		if len(args) != 2 {
			return fmt.Errorf("usage: pr -p <port>")
		}
		port, err := strconv.Atoi(args[1])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port %q", args[1])
		}
		processes, err := c.Scanner.Port(port)
		if err != nil {
			return err
		}
		if len(processes) == 0 {
			fmt.Fprintf(c.Out, "Port %d is free or its owner is not visible.\n", port)
			return nil
		}
		for _, process := range processes {
			fmt.Fprintf(c.Out, "port=%d pid=%d user=%s cwd=%s command=%s\n", port, process.PID, process.User, process.CWD, process.Command)
		}
		return nil
	case "-s":
		manager, err := c.manager()
		if err != nil {
			return err
		}
		runtimes, err := manager.ScanAll()
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Out, "Scanned %d projects.\n", len(runtimes))
		return nil
	default:
		return fmt.Errorf("unknown process option %q", args[0])
	}
}

func (c *CLI) lifecycleCommand(action string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: %s <project[-backend|-front|-component]>", action)
	}
	project, group, component, err := c.resolveTarget(args[0])
	if err != nil {
		return err
	}
	confirmations := 0
	if action == "sp" || action == "rst" {
		fmt.Fprintf(c.Out, "Target: project=%s group=%s component=%s\n", project.Code, emptyAs(group, "all"), emptyAs(component, "all"))
		confirmed, err := c.confirm("Confirm stop impact")
		if err != nil {
			return err
		}
		if !confirmed {
			return errCancelled
		}
		confirmations++
		if group == "" && component == "" {
			confirmed, err = c.confirm("Confirm stopping the entire project again")
			if err != nil {
				return err
			}
			if !confirmed {
				return errCancelled
			}
			confirmations++
		}
	}
	manager, err := c.manager()
	if err != nil {
		return err
	}
	var results []service.Result
	switch action {
	case "st":
		results = manager.StartProject(project, group, component)
	case "sp":
		results = manager.StopProject(project, group, component, false)
	case "rst":
		results = manager.StopProject(project, group, component, false)
	}
	if (action == "sp" || action == "rst") && hasStopTimeout(results) {
		force, promptErr := c.confirm("Graceful stop timed out. Force stop the remaining backend processes")
		if promptErr != nil {
			return promptErr
		}
		if force {
			confirmations++
			for i := range results {
				if results[i].Status == "failed" && strings.Contains(results[i].Message, "graceful stop timed out") {
					results[i].Status = "superseded"
				}
			}
			results = append(results, manager.ForceStopProject(project, group, component)...)
		}
	}
	if action == "rst" && service.ResultsError(results) == nil {
		results = append(results, manager.StartProject(project, group, component)...)
	}
	for _, result := range results {
		fmt.Fprintf(c.Out, "%-8s %-12s %-8s %s\n", result.Action, result.Type+"/"+result.Name, result.Status, result.Message)
	}
	resultErr := service.ResultsError(results)
	auditEntry := audit.Entry{Command: action + " " + args[0], Target: args[0], Action: lifecycleAction(action), Confirmations: confirmations, Result: "success"}
	if resultErr != nil {
		auditEntry.Result = "failed"
		auditEntry.Error = resultErr.Error()
	}
	_ = audit.Write(c.Root, auditEntry)
	if _, scanErr := manager.Scan(project); scanErr != nil {
		fmt.Fprintln(c.Err, "warning: runtime refresh failed:", scanErr)
	}
	return resultErr
}

func (c *CLI) resolveTarget(value string) (model.Project, string, string, error) {
	if project, err := c.Store.Resolve(value); err == nil {
		return project, "", "", nil
	}
	projects, err := c.Store.List()
	if err != nil {
		return model.Project{}, "", "", err
	}
	var selected *model.Project
	remainder := ""
	for i := range projects {
		prefix := projects[i].Code + "-"
		if strings.HasPrefix(value, prefix) && (selected == nil || len(projects[i].Code) > len(selected.Code)) {
			copy := projects[i]
			selected = &copy
			remainder = strings.TrimPrefix(value, prefix)
		}
	}
	if selected == nil {
		return model.Project{}, "", "", fmt.Errorf("cannot resolve target %q", value)
	}
	if remainder == "backend" || remainder == "front" {
		return *selected, remainder, "", nil
	}
	for _, backend := range selected.Backends {
		if backend.Name == remainder {
			return *selected, "backend", remainder, nil
		}
	}
	for _, frontend := range selected.Frontends {
		if frontend.Name == remainder {
			return *selected, "front", remainder, nil
		}
	}
	return model.Project{}, "", "", fmt.Errorf("project %s has no component %q", selected.Code, remainder)
}

func isManagedRuntime(command string) bool {
	lower := strings.ToLower(command)
	for _, name := range []string{"java", "python", "node", "nginx", "gunicorn", "uvicorn"} {
		if strings.Contains(lower, name) {
			return true
		}
	}
	return false
}

func lifecycleAction(short string) string {
	switch short {
	case "st":
		return "start"
	case "sp":
		return "stop"
	case "rst":
		return "restart"
	default:
		return short
	}
}

func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func hasStopTimeout(results []service.Result) bool {
	for _, result := range results {
		if result.Status == "failed" && strings.Contains(result.Message, "graceful stop timed out") {
			return true
		}
	}
	return false
}
