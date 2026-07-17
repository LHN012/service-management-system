package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type commandRequest struct {
	Action  string
	Project string
	Service string
	NewName string
	Source  string
	Target  string
	All     bool
	Force   bool
	Yes     bool
}

func parseCommand(fields []string) (commandRequest, error) {
	if len(fields) == 0 {
		return commandRequest{}, fmt.Errorf("command is required")
	}

	resource := strings.ToLower(fields[0])
	switch resource {
	case "help", "h":
		if len(fields) != 1 {
			return commandRequest{}, fmt.Errorf("usage: help")
		}
		return commandRequest{Action: "help"}, nil
	case "exit", "quit", "q":
		if len(fields) != 1 {
			return commandRequest{}, fmt.Errorf("usage: exit")
		}
		return commandRequest{Action: "sms.exit"}, nil
	case "version", "v":
		if len(fields) != 1 {
			return commandRequest{}, fmt.Errorf("usage: version")
		}
		return commandRequest{Action: "version"}, nil
	case "doctor":
		if len(fields) != 1 {
			return commandRequest{}, fmt.Errorf("usage: doctor")
		}
		return commandRequest{Action: "doctor"}, nil
	case "project", "p":
		return parseProjectCommand(fields)
	case "service", "s":
		return parseServiceCommand(fields)
	case "d":
		return parseDeployCommand(fields)
	case "deploy":
		if len(fields) >= 2 && isDeployAction(fields[1]) {
			return parseDeployCommand(fields)
		}
	}

	return parseLegacyCommand(fields)
}

func parseProjectCommand(fields []string) (commandRequest, error) {
	if len(fields) < 2 {
		return commandRequest{}, fmt.Errorf("usage: project <create|list|show|status|rename|start|stop>")
	}
	action := normalizeProjectAction(fields[1])
	switch action {
	case "create":
		if len(fields) != 3 {
			return commandRequest{}, fmt.Errorf("usage: project create <project>")
		}
		return commandRequest{Action: "project.create", Project: fields[2]}, nil
	case "list":
		if len(fields) != 2 {
			return commandRequest{}, fmt.Errorf("usage: project list")
		}
		return commandRequest{Action: "project.list"}, nil
	case "show":
		if len(fields) != 3 {
			return commandRequest{}, fmt.Errorf("usage: project show <project>")
		}
		return commandRequest{Action: "project.show", Project: fields[2]}, nil
	case "status":
		if len(fields) != 3 {
			return commandRequest{}, fmt.Errorf("usage: project status <project>")
		}
		return commandRequest{Action: "project.status", Project: fields[2]}, nil
	case "rename":
		if len(fields) != 4 {
			return commandRequest{}, fmt.Errorf("usage: project rename <project> <new-name>")
		}
		return commandRequest{Action: "project.rename", Project: fields[2], NewName: fields[3]}, nil
	case "start":
		if len(fields) != 3 && len(fields) != 4 {
			return commandRequest{}, fmt.Errorf("usage: project start <project> [--all]")
		}
		all := len(fields) == 4 && (strings.EqualFold(fields[3], "--all") || strings.EqualFold(fields[3], "-a"))
		if len(fields) == 4 && !all {
			return commandRequest{}, fmt.Errorf("usage: project start <project> [--all]")
		}
		return commandRequest{Action: "project.start", Project: fields[2], All: all}, nil
	case "stop":
		if len(fields) < 3 {
			return commandRequest{}, fmt.Errorf("usage: project stop <project> [--all] [--force]")
		}
		all, force, err := parseStopFlags(fields[3:])
		if err != nil {
			return commandRequest{}, fmt.Errorf("usage: project stop <project> [--all] [--force]")
		}
		return commandRequest{Action: "project.stop", Project: fields[2], All: all, Force: force}, nil
	default:
		return commandRequest{}, fmt.Errorf("unknown project action %q", fields[1])
	}
}

func parseServiceCommand(fields []string) (commandRequest, error) {
	if len(fields) < 2 {
		return commandRequest{}, fmt.Errorf("usage: service <add|list|show|status|edit|select|start|stop>")
	}
	action := normalizeServiceAction(fields[1])
	switch action {
	case "add", "list", "select":
		if len(fields) != 3 {
			return commandRequest{}, fmt.Errorf("usage: service %s <project>", action)
		}
		return commandRequest{Action: "service." + action, Project: fields[2]}, nil
	case "show", "status", "edit", "start":
		if len(fields) != 4 {
			return commandRequest{}, fmt.Errorf("usage: service %s <project> <service>", action)
		}
		return commandRequest{Action: "service." + action, Project: fields[2], Service: fields[3]}, nil
	case "stop":
		if len(fields) != 4 && len(fields) != 5 {
			return commandRequest{}, fmt.Errorf("usage: service stop <project> <service> [--force]")
		}
		force := false
		if len(fields) == 5 {
			if !strings.EqualFold(fields[4], "--force") && !strings.EqualFold(fields[4], "-f") {
				return commandRequest{}, fmt.Errorf("usage: service stop <project> <service> [--force]")
			}
			force = true
		}
		return commandRequest{Action: "service.stop", Project: fields[2], Service: fields[3], Force: force}, nil
	default:
		return commandRequest{}, fmt.Errorf("unknown service action %q", fields[1])
	}
}

func parseDeployCommand(fields []string) (commandRequest, error) {
	if len(fields) < 2 {
		return commandRequest{}, fmt.Errorf("usage: deploy <list|plan|apply>")
	}
	action := normalizeDeployAction(fields[1])
	switch action {
	case "list":
		if len(fields) != 3 {
			return commandRequest{}, fmt.Errorf("usage: deploy list <project>")
		}
		return commandRequest{Action: "deploy.list", Project: fields[2]}, nil
	case "plan", "apply":
		project, source, target, yes, err := parseDeployArguments(fields, action == "apply")
		if err != nil {
			return commandRequest{}, err
		}
		return commandRequest{Action: "deploy." + action, Project: project, Source: source, Target: target, Yes: yes}, nil
	default:
		return commandRequest{}, fmt.Errorf("unknown deploy action %q", fields[1])
	}
}

func parseDeployArguments(fields []string, allowYes bool) (string, string, string, bool, error) {
	usageErr := func() (string, string, string, bool, error) {
		yesUsage := ""
		if allowYes {
			yesUsage = " [--yes]"
		}
		return "", "", "", false, fmt.Errorf("usage: deploy %s <project> <source> --target <absolute-target>%s", strings.ToLower(fields[1]), yesUsage)
	}
	if len(fields) < 5 {
		return usageErr()
	}

	target := ""
	yes := false
	for index := 4; index < len(fields); index++ {
		argument := fields[index]
		switch {
		case strings.EqualFold(argument, "--target") || strings.EqualFold(argument, "-t"):
			if target != "" || index+1 >= len(fields) {
				return usageErr()
			}
			index++
			target = fields[index]
		case strings.HasPrefix(strings.ToLower(argument), "--target="):
			if target != "" {
				return usageErr()
			}
			target = argument[len("--target="):]
		case allowYes && (strings.EqualFold(argument, "--yes") || strings.EqualFold(argument, "-y")):
			if yes {
				return usageErr()
			}
			yes = true
		default:
			return usageErr()
		}
	}
	if strings.TrimSpace(target) == "" {
		return usageErr()
	}
	return fields[2], fields[3], target, yes, nil
}

func parseStopFlags(fields []string) (bool, bool, error) {
	all := false
	force := false
	for _, field := range fields {
		switch strings.ToLower(field) {
		case "--all", "-a":
			if all {
				return false, false, fmt.Errorf("duplicate --all")
			}
			all = true
		case "--force", "-f":
			if force {
				return false, false, fmt.Errorf("duplicate --force")
			}
			force = true
		default:
			return false, false, fmt.Errorf("unknown stop flag: %s", field)
		}
	}
	return all, force, nil
}

func parseLegacyCommand(fields []string) (commandRequest, error) {
	if isCreateProjectCommand(fields) {
		return commandRequest{Action: "project.create", Project: fields[len(fields)-1]}, nil
	}
	if isListProjectsCommand(fields) {
		if len(fields) == 1 {
			return commandRequest{Action: "project.list"}, nil
		}
		if strings.EqualFold(fields[1], "-i") {
			return commandRequest{Action: "project.show", Project: fields[2]}, nil
		}
		return commandRequest{Action: "service.list", Project: fields[2]}, nil
	}
	if isEditProjectCommand(fields) {
		return commandRequest{Action: "project.edit", Project: fields[len(fields)-1]}, nil
	}
	if isEditServiceCommand(fields) {
		return commandRequest{Action: "service.edit", Project: fields[2], Service: fields[3]}, nil
	}
	if isAddServiceCommand(fields) {
		return commandRequest{Action: "service.add", Project: fields[len(fields)-1]}, nil
	}
	if isBindServiceCommand(fields) {
		return commandRequest{Action: "service.select", Project: fields[len(fields)-1]}, nil
	}

	command := strings.ToLower(fields[0])
	if command == "st" || command == "start" {
		return parseLegacyLifecycle(fields, "start")
	}
	if command == "sp" || command == "stop" {
		return parseLegacyLifecycle(fields, "stop")
	}
	if command == "dp" || command == "deploy" {
		switch len(fields) {
		case 2:
			return commandRequest{Action: "deploy.list", Project: fields[1]}, nil
		case 4:
			return commandRequest{Action: "deploy.apply", Project: fields[1], Source: fields[2], Target: fields[3]}, nil
		default:
			return commandRequest{}, fmt.Errorf("usage: deploy list <project> | deploy apply <project> <source> --target <target>")
		}
	}
	return commandRequest{}, fmt.Errorf("unknown command: %s", strings.Join(fields, " "))
}

func parseLegacyLifecycle(fields []string, action string) (commandRequest, error) {
	switch {
	case len(fields) == 2:
		return commandRequest{Action: "project." + action, Project: fields[1]}, nil
	case len(fields) == 3 && strings.EqualFold(fields[1], "-all"):
		return commandRequest{Action: "project." + action, Project: fields[2], All: true}, nil
	case len(fields) == 4 && strings.EqualFold(fields[1], "-i"):
		return commandRequest{Action: "service." + action, Project: fields[2], Service: fields[3]}, nil
	default:
		return commandRequest{}, fmt.Errorf("invalid legacy lifecycle command; use project %s or service %s", action, action)
	}
}

func executeCommand(scanner *bufio.Scanner, output io.Writer, root string, request commandRequest) error {
	if commandMutatesState(request.Action) {
		return withRootLock(root, func() error {
			return executeCommandUnlocked(scanner, output, root, request)
		})
	}
	return executeCommandUnlocked(scanner, output, root, request)
}

func executeCommandUnlocked(scanner *bufio.Scanner, output io.Writer, root string, request commandRequest) error {
	switch request.Action {
	case "help":
		printHelp(output)
		return nil
	case "version":
		printVersion(output)
		return nil
	case "doctor":
		return runDoctor(output, root)
	case "project.create":
		return createProject(scanner, output, root, request.Project)
	case "project.list":
		return printProjectList(output, root)
	case "project.show":
		return showProject(output, root, request.Project)
	case "project.status":
		return printServiceStatus(output, root, request.Project, "")
	case "project.edit":
		return editProject(scanner, output, root, request.Project)
	case "project.rename":
		return renameProject(output, root, request.Project, request.NewName)
	case "service.add":
		return addService(scanner, output, root, request.Project)
	case "service.list":
		return listProjectServices(output, root, request.Project)
	case "service.show":
		return showService(output, root, request.Project, request.Service)
	case "service.status":
		return printServiceStatus(output, root, request.Project, request.Service)
	case "service.edit":
		return editService(scanner, output, root, request.Project, request.Service)
	case "service.select":
		return bindServices(scanner, output, root, request.Project)
	case "project.start", "project.stop":
		verb := "st"
		if request.Action == "project.stop" {
			verb = "sp"
		}
		fields := []string{verb, request.Project}
		if request.All {
			fields = []string{verb, "-all", request.Project}
		}
		if verb == "st" {
			return startServices(scanner, output, root, fields)
		}
		return stopServices(output, root, fields, request.Force)
	case "service.start", "service.stop":
		verb := "st"
		if request.Action == "service.stop" {
			verb = "sp"
		}
		fields := []string{verb, "-i", request.Project, request.Service}
		if verb == "st" {
			return startServices(scanner, output, root, fields)
		}
		return stopServices(output, root, fields, request.Force)
	case "deploy.list":
		return runDeploy(scanner, output, root, request)
	case "deploy.plan", "deploy.apply":
		return runDeploy(scanner, output, root, request)
	default:
		return fmt.Errorf("unsupported command action: %s", request.Action)
	}
}

func commandMutatesState(action string) bool {
	switch action {
	case "project.create", "project.edit", "project.rename",
		"service.add", "service.edit", "service.select",
		"project.start", "project.stop", "service.start", "service.stop",
		"deploy.apply":
		return true
	default:
		return false
	}
}

func normalizeProjectAction(value string) string {
	switch strings.ToLower(value) {
	case "create", "new":
		return "create"
	case "list", "ls":
		return "list"
	case "show", "info":
		return "show"
	case "status":
		return "status"
	case "rename", "mv":
		return "rename"
	case "start", "up":
		return "start"
	case "stop", "down":
		return "stop"
	default:
		return ""
	}
}

func normalizeServiceAction(value string) string {
	switch strings.ToLower(value) {
	case "add":
		return "add"
	case "list", "ls":
		return "list"
	case "show", "info":
		return "show"
	case "status":
		return "status"
	case "edit":
		return "edit"
	case "select", "sel":
		return "select"
	case "start", "up":
		return "start"
	case "stop", "down":
		return "stop"
	default:
		return ""
	}
}

func normalizeDeployAction(value string) string {
	switch strings.ToLower(value) {
	case "list", "ls":
		return "list"
	case "plan":
		return "plan"
	case "apply":
		return "apply"
	default:
		return ""
	}
}

func isDeployAction(value string) bool {
	return normalizeDeployAction(value) != ""
}
