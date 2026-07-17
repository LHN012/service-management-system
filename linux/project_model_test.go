package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSaveProjectRejectsUnsafeAndDuplicateServiceNames(t *testing.T) {
	root := t.TempDir()
	service := Service{
		Name: "../api", StartPath: "/opt/api.jar", Port: 8080,
		StartCommand: "java -jar /opt/api.jar", RestartMode: "kill-start",
	}
	if _, err := saveProject(root, Project{Name: "demo", Services: []Service{service}}); err == nil {
		t.Fatal("unsafe service name was accepted")
	}

	service.Name = "api"
	if _, err := saveProject(root, Project{Name: "demo", Services: []Service{service, service}}); err == nil {
		t.Fatal("duplicate service names were accepted")
	}
}

func TestCreateProjectAllowsNoServices(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	if err := createProject(bufio.NewScanner(strings.NewReader("0\n")), &output, root, "demo"); err != nil {
		t.Fatal(err)
	}
	project, _, err := loadProject(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Services) != 0 {
		t.Fatalf("services=%d, want 0", len(project.Services))
	}
}

func TestAtomicWriteFileReplacesCompleteContent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("atomic replacement behavior is verified on Linux")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := atomicWriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(path, []byte("new content"), 0o640); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new content" {
		t.Fatalf("content=%q", data)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".sms-write-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestLoadProjectMigratesUnversionedConfigAndRejectsFutureSchema(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(projectDir, "project.json")
	if err := os.WriteFile(configPath, []byte(`{"name":"demo","services":[]}`), 0o640); err != nil {
		t.Fatal(err)
	}
	project, _, err := loadProject(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if project.SchemaVersion != currentProjectSchemaVersion {
		t.Fatalf("schemaVersion=%d", project.SchemaVersion)
	}

	if err := os.WriteFile(configPath, []byte(`{"schemaVersion":99,"name":"demo","services":[]}`), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadProject(root, "demo"); err == nil {
		t.Fatal("future schema was accepted")
	}
}
