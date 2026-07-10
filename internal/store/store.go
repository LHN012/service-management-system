package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"sms/internal/model"
)

var ErrNotFound = errors.New("project not found")

type Store struct {
	Root         string
	ProjectsRoot string
}

func New(root string) *Store {
	return NewAt(root, filepath.Join(root, "projects"))

}

func NewAt(root, projectsRoot string) *Store {
	return &Store{Root: root, ProjectsRoot: projectsRoot}
}

func (s *Store) ProjectsDir() string {
	return s.ProjectsRoot
}

func (s *Store) ProjectDir(code string) string {
	return filepath.Join(s.ProjectsDir(), code)
}

func (s *Store) ProjectPath(code string) string {
	return filepath.Join(s.ProjectDir(code), "project.yml")
}

func (s *Store) List() ([]model.Project, error) {
	entries, err := os.ReadDir(s.ProjectsDir())
	if errors.Is(err, os.ErrNotExist) {
		return []model.Project{}, nil
	}
	if err != nil {
		return nil, err
	}
	projects := make([]model.Project, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project, err := s.Load(entry.Name())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load project %s: %w", entry.Name(), err)
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Code < projects[j].Code })
	return projects, nil
}

func (s *Store) Load(code string) (model.Project, error) {
	var project model.Project
	data, err := os.ReadFile(s.ProjectPath(code))
	if errors.Is(err, os.ErrNotExist) {
		return project, ErrNotFound
	}
	if err != nil {
		return project, err
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		return project, fmt.Errorf("parse project.yml: %w", err)
	}
	if err := project.Validate(); err != nil {
		return project, fmt.Errorf("invalid project.yml: %w", err)
	}
	return project, nil
}

func (s *Store) Resolve(nameOrCode string) (model.Project, error) {
	if project, err := s.Load(nameOrCode); err == nil {
		return project, nil
	}
	projects, err := s.List()
	if err != nil {
		return model.Project{}, err
	}
	var matches []model.Project
	for _, project := range projects {
		if strings.EqualFold(project.Name, nameOrCode) {
			matches = append(matches, project)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return model.Project{}, fmt.Errorf("project name %q is ambiguous; use project code", nameOrCode)
	}
	return model.Project{}, ErrNotFound
}

func (s *Store) Save(project model.Project) error {
	if err := project.Validate(); err != nil {
		return err
	}
	dir := s.ProjectDir(project.Code)
	for _, child := range []string{"deploy-files", filepath.Join("backups", "files"), filepath.Join("backups", "dirs"), "logs", "scripts"} {
		if err := os.MkdirAll(filepath.Join(dir, child), 0o755); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(project)
	if err != nil {
		return err
	}
	return atomicWrite(s.ProjectPath(project.Code), data, 0o644)
}

func (s *Store) Delete(code string) error {
	if _, err := s.Load(code); err != nil {
		return err
	}
	return os.RemoveAll(s.ProjectDir(code))
}

func (s *Store) SaveRuntime(runtime model.Runtime) error {
	data, err := json.MarshalIndent(runtime, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWrite(filepath.Join(s.ProjectDir(runtime.ProjectCode), "runtime.json"), data, 0o644)
}

func (s *Store) LoadRuntime(code string) (model.Runtime, error) {
	var runtime model.Runtime
	data, err := os.ReadFile(filepath.Join(s.ProjectDir(code), "runtime.json"))
	if err != nil {
		return runtime, err
	}
	return runtime, json.Unmarshal(data, &runtime)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sms-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err == nil {
		return nil
	}
	// Windows test hosts cannot replace an existing file with Rename. Linux uses
	// the atomic branch above.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpName, path)
}
