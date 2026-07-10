package main

import (
	"os"
	"path/filepath"
	"testing"

	"sms/internal/model"
)

func TestWindowsAppProjectAndDeployFlow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMS_WINDOWS_ROOT", root)
	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(root, "target")
	project := model.Project{
		Code: "demo", Name: "Demo", ManageMode: "external",
		DeployRules: []model.DeployRule{{
			Name: "api", Source: "deploy-files/api.jar", TargetDir: targetDir,
			TargetName: "api.jar", Type: "file", Backup: true,
		}},
	}
	if _, err := app.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "data", "projects", "demo", "deploy-files", "api.jar")
	if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "api.jar"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	preview, err := app.PrepareDeploy("demo", "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 1 || preview.Changes[0].Action != "replace" {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	result, err := app.ApplyDeploy(preview.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changes != 1 || len(result.BackupPaths) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(targetDir, "api.jar"))
	if err != nil || string(data) != "new" {
		t.Fatalf("deployed content=%q err=%v", data, err)
	}
}

func TestWindowsDataLayout(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMS_WINDOWS_ROOT", root)
	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	if got := app.store.ProjectsDir(); got != filepath.Join(root, "data", "projects") {
		t.Fatalf("projects dir=%s", got)
	}
	for _, relative := range []string{filepath.Join("conf", "app.yml"), filepath.Join("data", "runtime"), filepath.Join("tmp", "deploy")} {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
}
