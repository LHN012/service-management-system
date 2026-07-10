package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"sms/internal/model"
	"sms/internal/store"
)

func (c *CLI) projectCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: p -c | -l | -s <project> | -e <project> | -d <project>")
	}
	switch args[0] {
	case "-c":
		return c.createProject()
	case "-l":
		return c.listProjects()
	case "-s":
		if len(args) != 2 {
			return fmt.Errorf("usage: p -s <project>")
		}
		return c.showProject(args[1])
	case "-e":
		if len(args) != 2 {
			return fmt.Errorf("usage: p -e <project>")
		}
		return c.editProject(args[1])
	case "-d":
		if len(args) != 2 {
			return fmt.Errorf("usage: p -d <project>")
		}
		return c.deleteProject(args[1])
	default:
		return fmt.Errorf("unknown project option %q", args[0])
	}
}

func (c *CLI) createProject() error {
	fmt.Fprintln(c.Out, "Create project. Enter !cancel at any prompt to stop.")
	code, err := c.prompt("Project code", "", true)
	if err != nil {
		return err
	}
	if _, err := c.Store.Load(code); err == nil {
		return fmt.Errorf("project %q already exists", code)
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	name, err := c.prompt("Project name", code, true)
	if err != nil {
		return err
	}
	mode, err := c.prompt("Manage mode (external/internal)", "external", true)
	if err != nil {
		return err
	}
	description, err := c.prompt("Description", "", false)
	if err != nil {
		return err
	}
	project := model.Project{Code: code, Name: name, ManageMode: mode, Description: description}

	backendCount, err := c.countPrompt("Backend count")
	if err != nil {
		return err
	}
	for i := 0; i < backendCount; i++ {
		backend, err := c.promptBackend(i + 1)
		if err != nil {
			return err
		}
		project.Backends = append(project.Backends, backend)
	}
	frontendCount, err := c.countPrompt("Frontend count")
	if err != nil {
		return err
	}
	for i := 0; i < frontendCount; i++ {
		frontend, err := c.promptFrontend(i + 1)
		if err != nil {
			return err
		}
		project.Frontends = append(project.Frontends, frontend)
	}
	ruleCount, err := c.countPrompt("Deploy rule count")
	if err != nil {
		return err
	}
	for i := 0; i < ruleCount; i++ {
		rule, err := c.promptDeployRule(i + 1)
		if err != nil {
			return err
		}
		project.DeployRules = append(project.DeployRules, rule)
	}
	if err := project.Validate(); err != nil {
		return err
	}
	data, _ := yaml.Marshal(project)
	fmt.Fprintf(c.Out, "\nConfiguration preview:\n%s\n", data)
	confirmed, err := c.confirm("Save project")
	if err != nil {
		return err
	}
	if !confirmed {
		return errCancelled
	}
	if err := c.Store.Save(project); err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "Created %s\n", c.Store.ProjectPath(project.Code))
	return nil
}

func (c *CLI) promptBackend(index int) (model.Backend, error) {
	prefix := fmt.Sprintf("Backend %d", index)
	name, err := c.prompt(prefix+" name", fmt.Sprintf("backend-%d", index), true)
	if err != nil {
		return model.Backend{}, err
	}
	runtimeName, err := c.prompt(prefix+" runtime", "java", true)
	if err != nil {
		return model.Backend{}, err
	}
	workDir, err := c.prompt(prefix+" workDir", "", true)
	if err != nil {
		return model.Backend{}, err
	}
	startCommand, err := c.prompt(prefix+" startCommand", "", true)
	if err != nil {
		return model.Backend{}, err
	}
	stopCommand, err := c.prompt(prefix+" stopCommand (blank uses SIGTERM)", "", false)
	if err != nil {
		return model.Backend{}, err
	}
	portText, err := c.prompt(prefix+" expected ports (comma separated)", "", false)
	if err != nil {
		return model.Backend{}, err
	}
	ports, err := parsePorts(portText)
	if err != nil {
		return model.Backend{}, err
	}
	commandContains, err := c.prompt(prefix+" command match", "", len(ports) == 0)
	if err != nil {
		return model.Backend{}, err
	}
	health, err := c.prompt(prefix+" health check URL", "", false)
	if err != nil {
		return model.Backend{}, err
	}
	return model.Backend{
		Name: name, Runtime: runtimeName, WorkDir: workDir, StartCommand: startCommand,
		StopCommand: stopCommand, StopMode: "graceful", ExpectedPorts: ports, HealthCheck: health,
		Match: model.MatchRule{CommandContains: commandContains, CWD: workDir},
	}, nil
}

func (c *CLI) promptFrontend(index int) (model.Frontend, error) {
	prefix := fmt.Sprintf("Frontend %d", index)
	name, err := c.prompt(prefix+" name", fmt.Sprintf("frontend-%d", index), true)
	if err != nil {
		return model.Frontend{}, err
	}
	mode, err := c.prompt(prefix+" nginx mode (shared/dedicated)", "shared", true)
	if err != nil {
		return model.Frontend{}, err
	}
	rootDir, err := c.prompt(prefix+" rootDir", "", true)
	if err != nil {
		return model.Frontend{}, err
	}
	conf, err := c.prompt(prefix+" nginxConf", "", true)
	if err != nil {
		return model.Frontend{}, err
	}
	reload, err := c.prompt(prefix+" reloadCommand", "nginx -s reload", mode == "shared")
	if err != nil {
		return model.Frontend{}, err
	}
	start := ""
	stop := ""
	if mode == "dedicated" {
		start, err = c.prompt(prefix+" startCommand", "", false)
		if err != nil {
			return model.Frontend{}, err
		}
		stop, err = c.prompt(prefix+" stopCommand", "nginx -s quit", true)
		if err != nil {
			return model.Frontend{}, err
		}
	}
	portText, err := c.prompt(prefix+" expected ports (comma separated)", "80", false)
	if err != nil {
		return model.Frontend{}, err
	}
	ports, err := parsePorts(portText)
	if err != nil {
		return model.Frontend{}, err
	}
	return model.Frontend{Name: name, NginxMode: mode, RootDir: rootDir, NginxConf: conf, StartCommand: start, ReloadCommand: reload, StopCommand: stop, ExpectedPorts: ports}, nil
}

func (c *CLI) promptDeployRule(index int) (model.DeployRule, error) {
	prefix := fmt.Sprintf("Deploy rule %d", index)
	name, err := c.prompt(prefix+" name", fmt.Sprintf("rule-%d", index), true)
	if err != nil {
		return model.DeployRule{}, err
	}
	typeName, err := c.prompt(prefix+" type (file/directory/archive)", "file", true)
	if err != nil {
		return model.DeployRule{}, err
	}
	source, err := c.prompt(prefix+" source", "deploy-files/", true)
	if err != nil {
		return model.DeployRule{}, err
	}
	targetDir, err := c.prompt(prefix+" targetDir", "", true)
	if err != nil {
		return model.DeployRule{}, err
	}
	rule := model.DeployRule{Name: name, Type: typeName, Source: source, TargetDir: targetDir, Backup: true}
	if typeName == "file" {
		rule.TargetName, err = c.prompt(prefix+" targetName (blank keeps source name)", "", false)
	} else {
		rule.ReplaceMode = "entries"
	}
	if err != nil {
		return model.DeployRule{}, err
	}
	if typeName == "archive" {
		rule.ArchiveFormat = "auto"
		rule.StripTopLevel, err = c.prompt(prefix+" stripTopLevel (auto/always/never)", "auto", true)
		if err != nil {
			return model.DeployRule{}, err
		}
		rule.ContentPath, err = c.prompt(prefix+" contentPath (optional)", "", false)
		if err != nil {
			return model.DeployRule{}, err
		}
	}
	backupValue, err := c.prompt(prefix+" backup (yes/no)", "yes", true)
	if err != nil {
		return model.DeployRule{}, err
	}
	rule.Backup = strings.EqualFold(backupValue, "yes") || strings.EqualFold(backupValue, "y")
	return rule, nil
}

func (c *CLI) listProjects() error {
	projects, err := c.Store.List()
	if err != nil {
		return err
	}
	writer := tabwriter.NewWriter(c.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "CODE\tNAME\tMODE\tBACKENDS\tFRONTENDS\tSTATUS\tPORTS")
	for _, project := range projects {
		status := "unknown"
		ports := "-"
		if runtime, scanErr := c.Scanner.ScanProject(project); scanErr == nil {
			status = project.OverallStatus(runtime)
			var values []string
			for _, process := range runtime.Processes {
				for _, port := range process.Ports {
					values = append(values, strconv.Itoa(port))
				}
			}
			if len(values) > 0 {
				ports = strings.Join(values, ",")
			}
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%d\t%s\t%s\n", project.Code, project.Name, project.ManageMode, len(project.Backends), len(project.Frontends), status, ports)
	}
	return writer.Flush()
}

func (c *CLI) showProject(name string) error {
	project, err := c.Store.Resolve(name)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(project)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "%s\n", data)
	runtime, err := c.Scanner.ScanProject(project)
	if err != nil {
		fmt.Fprintf(c.Out, "Runtime: unavailable (%v)\n", err)
		return nil
	}
	_ = c.Store.SaveRuntime(runtime)
	fmt.Fprintf(c.Out, "Runtime: %s (scanned %s)\n", project.OverallStatus(runtime), formatTime(runtime.LastScanAt))
	for _, process := range runtime.Processes {
		fmt.Fprintf(c.Out, "  %-10s %-8s %-8s pid=%d ports=%v\n", process.Type, process.Name, process.Status, process.PID, process.Ports)
	}
	return nil
}

func (c *CLI) editProject(name string) error {
	project, err := c.Store.Resolve(name)
	if err != nil {
		return err
	}
	path := c.Store.ProjectPath(project.Code)
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	command := exec.Command(editor, path)
	command.Stdin, command.Stdout, command.Stderr = c.In, c.Out, c.Err
	if err := command.Run(); err != nil {
		return err
	}
	if _, err := c.Store.Load(project.Code); err != nil {
		_ = os.WriteFile(path, original, 0o644)
		return fmt.Errorf("edited configuration is invalid and was restored: %w", err)
	}
	fmt.Fprintf(c.Out, "Updated %s\n", path)
	return nil
}

func (c *CLI) deleteProject(name string) error {
	project, err := c.Store.Resolve(name)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "This removes management data under %s but never external application paths.\n", c.Store.ProjectDir(project.Code))
	confirmed, err := c.confirm("Delete project " + project.Code)
	if err != nil {
		return err
	}
	if !confirmed {
		return errCancelled
	}
	return c.Store.Delete(project.Code)
}

func absolutePath(value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	abs, _ := filepath.Abs(value)
	return abs
}
