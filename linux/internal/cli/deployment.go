package cli

import (
	"fmt"
	"strings"

	"sms/internal/audit"
	"sms/internal/deploy"
	"sms/internal/model"
)

func (c *CLI) deployCommand(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: dp <project> [rule]")
	}
	project, err := c.Store.Resolve(args[0])
	if err != nil {
		return err
	}
	rules, err := c.selectDeployRules(project, args)
	if err != nil {
		return err
	}
	plans := make([]*deploy.Plan, 0, len(rules))
	defer func() {
		for _, plan := range plans {
			plan.Close()
		}
	}()
	for _, rule := range rules {
		plan, err := c.Deploy.Prepare(project, rule)
		if err != nil {
			return fmt.Errorf("prepare rule %s: %w", rule.Name, err)
		}
		plans = append(plans, plan)
	}
	fmt.Fprintln(c.Out, "\nDeployment preview:")
	for _, plan := range plans {
		fmt.Fprintf(c.Out, "\nRule: %s\n  source: %s\n  content root: %s\n  backup default: %t\n", plan.Rule.Name, plan.SourcePath, plan.ContentRoot, plan.Rule.Backup)
		for _, change := range plan.Changes {
			fmt.Fprintf(c.Out, "  %-7s %s\n", change.Action, change.Target)
		}
	}
	backupMode, err := c.prompt("Backup mode (default/all/none)", "default", true)
	if err != nil {
		return err
	}
	if backupMode != "default" && backupMode != "all" && backupMode != "none" {
		return fmt.Errorf("backup mode must be default, all, or none")
	}
	confirmed, err := c.confirm("Apply this deployment")
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(c.Out, "Preview completed; target directories were not modified.")
		return nil
	}
	for _, plan := range plans {
		backup := plan.Rule.Backup
		if backupMode == "all" {
			backup = true
		} else if backupMode == "none" {
			backup = false
		}
		result, err := c.Deploy.Apply(plan, backup)
		if err != nil {
			_ = audit.Write(c.Root, audit.Entry{Command: strings.Join(append([]string{"dp"}, args...), " "), Target: project.Code, Action: "deploy", Confirmations: 1, Result: "failed", Error: err.Error()})
			return fmt.Errorf("apply rule %s: %w", plan.Rule.Name, err)
		}
		fmt.Fprintf(c.Out, "Applied %s (%d changes)\n", plan.Rule.Name, len(result.Changes))
		for _, path := range result.BackupPaths {
			fmt.Fprintf(c.Out, "  backup: %s\n", path)
		}
	}
	_ = audit.Write(c.Root, audit.Entry{Command: strings.Join(append([]string{"dp"}, args...), " "), Target: project.Code, Action: "deploy", Confirmations: 1, Result: "success"})
	return nil
}

func (c *CLI) selectDeployRules(project model.Project, args []string) ([]model.DeployRule, error) {
	if len(project.DeployRules) == 0 {
		return nil, fmt.Errorf("project %s has no deploy rules", project.Code)
	}
	if len(args) == 2 {
		for _, rule := range project.DeployRules {
			if rule.Name == args[1] {
				return []model.DeployRule{rule}, nil
			}
		}
		return nil, fmt.Errorf("project %s has no deploy rule %q", project.Code, args[1])
	}
	fmt.Fprintln(c.Out, "Deploy rules:")
	for i, rule := range project.DeployRules {
		fmt.Fprintf(c.Out, "  %d. %-16s %-9s %s -> %s\n", i+1, rule.Name, rule.Type, rule.Source, rule.TargetDir)
	}
	selection, err := c.prompt("Select rule names (comma separated) or all", "all", true)
	if err != nil {
		return nil, err
	}
	if selection == "all" {
		return append([]model.DeployRule(nil), project.DeployRules...), nil
	}
	wanted := map[string]bool{}
	for _, value := range strings.Split(selection, ",") {
		wanted[strings.TrimSpace(value)] = true
	}
	var selected []model.DeployRule
	for _, rule := range project.DeployRules {
		if wanted[rule.Name] {
			selected = append(selected, rule)
			delete(wanted, rule.Name)
		}
	}
	if len(wanted) > 0 {
		var unknown []string
		for name := range wanted {
			unknown = append(unknown, name)
		}
		return nil, fmt.Errorf("unknown deploy rules: %s", strings.Join(unknown, ", "))
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no deploy rules selected")
	}
	return selected, nil
}
