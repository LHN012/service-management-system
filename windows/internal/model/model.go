package model

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Project struct {
	Code        string       `yaml:"code" json:"code"`
	Name        string       `yaml:"name" json:"name"`
	ManageMode  string       `yaml:"manageMode" json:"manageMode"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Backends    []Backend    `yaml:"backends,omitempty" json:"backends,omitempty"`
	Frontends   []Frontend   `yaml:"frontends,omitempty" json:"frontends,omitempty"`
	DeployRules []DeployRule `yaml:"deployRules,omitempty" json:"deployRules,omitempty"`
}

type MatchRule struct {
	CommandContains string `yaml:"commandContains,omitempty" json:"commandContains,omitempty"`
	CWD             string `yaml:"cwd,omitempty" json:"cwd,omitempty"`
}

type Backend struct {
	Name          string    `yaml:"name" json:"name"`
	Runtime       string    `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	WorkDir       string    `yaml:"workDir" json:"workDir"`
	StartCommand  string    `yaml:"startCommand" json:"startCommand"`
	StopCommand   string    `yaml:"stopCommand,omitempty" json:"stopCommand,omitempty"`
	StopMode      string    `yaml:"stopMode,omitempty" json:"stopMode,omitempty"`
	StopTimeout   int       `yaml:"stopTimeout,omitempty" json:"stopTimeout,omitempty"`
	ExpectedPorts []int     `yaml:"expectedPorts,omitempty" json:"expectedPorts,omitempty"`
	HealthCheck   string    `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`
	Match         MatchRule `yaml:"match,omitempty" json:"match,omitempty"`
	Disabled      bool      `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

type Frontend struct {
	Name          string `yaml:"name" json:"name"`
	NginxMode     string `yaml:"nginxMode" json:"nginxMode"`
	RootDir       string `yaml:"rootDir" json:"rootDir"`
	NginxConf     string `yaml:"nginxConf" json:"nginxConf"`
	StartCommand  string `yaml:"startCommand,omitempty" json:"startCommand,omitempty"`
	ReloadCommand string `yaml:"reloadCommand,omitempty" json:"reloadCommand,omitempty"`
	StopCommand   string `yaml:"stopCommand,omitempty" json:"stopCommand,omitempty"`
	ExpectedPorts []int  `yaml:"expectedPorts,omitempty" json:"expectedPorts,omitempty"`
	Disabled      bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

type DeployRule struct {
	Name          string `yaml:"name" json:"name"`
	Source        string `yaml:"source" json:"source"`
	TargetDir     string `yaml:"targetDir" json:"targetDir"`
	TargetName    string `yaml:"targetName,omitempty" json:"targetName,omitempty"`
	Type          string `yaml:"type" json:"type"`
	ArchiveFormat string `yaml:"archiveFormat,omitempty" json:"archiveFormat,omitempty"`
	StripTopLevel string `yaml:"stripTopLevel,omitempty" json:"stripTopLevel,omitempty"`
	ContentPath   string `yaml:"contentPath,omitempty" json:"contentPath,omitempty"`
	ReplaceMode   string `yaml:"replaceMode,omitempty" json:"replaceMode,omitempty"`
	Backup        bool   `yaml:"backup" json:"backup"`
}

type Runtime struct {
	ProjectCode string           `json:"projectCode"`
	LastScanAt  time.Time        `json:"lastScanAt"`
	Processes   []ProcessRuntime `json:"processes"`
}

type ProcessRuntime struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	PID       int      `json:"pid,omitempty"`
	Ports     []int    `json:"ports,omitempty"`
	Command   string   `json:"command,omitempty"`
	CWD       string   `json:"cwd,omitempty"`
	MatchedBy []string `json:"matchedBy,omitempty"`
	Message   string   `json:"message,omitempty"`
}

func (p Project) Validate() error {
	if !codePattern.MatchString(p.Code) {
		return fmt.Errorf("project code %q must match %s", p.Code, codePattern.String())
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("project name is required")
	}
	if p.ManageMode != "external" && p.ManageMode != "internal" {
		return fmt.Errorf("manageMode must be external or internal")
	}
	seen := map[string]string{}
	for i, backend := range p.Backends {
		if err := validateComponentName(backend.Name, "backend", seen); err != nil {
			return err
		}
		if strings.TrimSpace(backend.WorkDir) == "" || strings.TrimSpace(backend.StartCommand) == "" {
			return fmt.Errorf("backend %d requires workDir and startCommand", i+1)
		}
		if backend.Match.CommandContains == "" && backend.Match.CWD == "" && len(backend.ExpectedPorts) == 0 {
			return fmt.Errorf("backend %q requires at least one process match rule or expected port", backend.Name)
		}
		if err := validatePorts(backend.ExpectedPorts); err != nil {
			return fmt.Errorf("backend %q: %w", backend.Name, err)
		}
	}
	for _, frontend := range p.Frontends {
		if err := validateComponentName(frontend.Name, "frontend", seen); err != nil {
			return err
		}
		if frontend.NginxMode != "shared" && frontend.NginxMode != "dedicated" {
			return fmt.Errorf("frontend %q nginxMode must be shared or dedicated", frontend.Name)
		}
		if err := validatePorts(frontend.ExpectedPorts); err != nil {
			return fmt.Errorf("frontend %q: %w", frontend.Name, err)
		}
	}
	rules := map[string]struct{}{}
	for _, rule := range p.DeployRules {
		if strings.TrimSpace(rule.Name) == "" {
			return fmt.Errorf("deploy rule name is required")
		}
		if _, ok := rules[rule.Name]; ok {
			return fmt.Errorf("duplicate deploy rule %q", rule.Name)
		}
		rules[rule.Name] = struct{}{}
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("deploy rule %q: %w", rule.Name, err)
		}
	}
	return nil
}

func (r DeployRule) Validate() error {
	source := filepath.ToSlash(filepath.Clean(r.Source))
	if source == "deploy-files" || !strings.HasPrefix(source, "deploy-files/") {
		return fmt.Errorf("source must point to an entry inside deploy-files")
	}
	if !strings.HasPrefix(r.TargetDir, "/") && !filepath.IsAbs(r.TargetDir) {
		return fmt.Errorf("targetDir must be an absolute Linux path")
	}
	switch r.Type {
	case "file", "directory", "archive":
	default:
		return fmt.Errorf("type must be file, directory, or archive")
	}
	if r.Type == "archive" {
		strip := r.StripTopLevel
		if strip == "" {
			strip = "auto"
		}
		if strip != "auto" && strip != "always" && strip != "never" {
			return fmt.Errorf("stripTopLevel must be auto, always, or never")
		}
	}
	if r.ReplaceMode != "" && r.ReplaceMode != "entries" {
		return fmt.Errorf("replaceMode only supports entries in this version")
	}
	return nil
}

func validateComponentName(name, kind string, seen map[string]string) error {
	if !codePattern.MatchString(name) {
		return fmt.Errorf("%s name %q must match %s", kind, name, codePattern.String())
	}
	if previous, ok := seen[name]; ok {
		return fmt.Errorf("component name %q is used by both %s and %s", name, previous, kind)
	}
	seen[name] = kind
	return nil
}

func validatePorts(ports []int) error {
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid port %d", port)
		}
	}
	return nil
}

func (p Project) OverallStatus(runtime Runtime) string {
	if len(runtime.Processes) == 0 {
		return "stopped"
	}
	running, failed := 0, 0
	for _, process := range runtime.Processes {
		switch process.Status {
		case "running":
			running++
		case "failed", "unknown":
			failed++
		}
	}
	if running == len(runtime.Processes) {
		return "running"
	}
	if running > 0 {
		return "partial"
	}
	if failed > 0 {
		return "unknown"
	}
	return "stopped"
}
