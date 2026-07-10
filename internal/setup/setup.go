package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"sms/internal/config"
)

type Dependency struct {
	Name     string
	Required bool
	Found    bool
	Path     string
}

type Report struct {
	Root         string
	Dependencies []Dependency
}

func Initialize(root string) (Report, error) {
	dirs := []string{
		"conf", "projects", filepath.Join("data", "runtime"), filepath.Join("data", "backups"),
		filepath.Join("data", "logs"), "templates", filepath.Join("tmp", "deploy"), "bin",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return Report{}, err
		}
	}
	configPath := filepath.Join(root, "conf", "app.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		data, marshalErr := yaml.Marshal(config.Default())
		if marshalErr != nil {
			return Report{}, marshalErr
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			return Report{}, err
		}
	}
	loggingPath := filepath.Join(root, "conf", "logging.yml")
	if _, err := os.Stat(loggingPath); os.IsNotExist(err) {
		if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o644); err != nil {
			return Report{}, err
		}
	}

	dependencies := []Dependency{
		{Name: "sh", Required: true}, {Name: "ps", Required: true}, {Name: "ss", Required: true},
		{Name: "tar"}, {Name: "gzip"}, {Name: "unzip"}, {Name: "nginx"},
		{Name: "java"}, {Name: "python3"}, {Name: "node"},
	}
	for i := range dependencies {
		path, err := exec.LookPath(dependencies[i].Name)
		dependencies[i].Found = err == nil
		dependencies[i].Path = path
	}
	sort.SliceStable(dependencies, func(i, j int) bool {
		if dependencies[i].Required != dependencies[j].Required {
			return dependencies[i].Required
		}
		return dependencies[i].Name < dependencies[j].Name
	})

	report := Report{Root: root, Dependencies: dependencies}
	if err := writeReport(root, report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func writeReport(root string, report Report) error {
	path := filepath.Join(root, "data", "logs", "init-report.txt")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "root: %s\n", report.Root); err != nil {
		return err
	}
	for _, dependency := range report.Dependencies {
		status := "missing"
		if dependency.Found {
			status = dependency.Path
		}
		required := "optional"
		if dependency.Required {
			required = "required"
		}
		if _, err := fmt.Fprintf(file, "%-10s %-8s %s\n", dependency.Name, required, status); err != nil {
			return err
		}
	}
	return nil
}
