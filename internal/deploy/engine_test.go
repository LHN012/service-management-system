package deploy

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sms/internal/model"
	"sms/internal/store"
)

func TestArchiveAutoRootProducesSameTargetForWrappedAndFlat(t *testing.T) {
	for _, test := range []struct {
		name  string
		files map[string]string
		root  string
	}{
		{name: "wrapped", root: "dist", files: map[string]string{"dist/index.html": "index", "dist/assets/app.js": "app"}},
		{name: "flat", root: "extracted", files: map[string]string{"index.html": "index", "assets/app.js": "app"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			projectStore := store.New(root)
			target := filepath.Join(root, "target")
			rule := model.DeployRule{
				Name: "web", Source: "deploy-files/web.zip", TargetDir: target, Type: "archive",
				ArchiveFormat: "auto", StripTopLevel: "auto", ReplaceMode: "entries", Backup: true,
			}
			project := model.Project{Code: "demo", Name: "Demo", ManageMode: "external", DeployRules: []model.DeployRule{rule}}
			if err := projectStore.Save(project); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(projectStore.ProjectDir(project.Code), "deploy-files", "web.zip")
			writeZip(t, archivePath, test.files)
			mustWrite(t, filepath.Join(target, "assets", "old.js"), "old")
			mustWrite(t, filepath.Join(target, "keep.txt"), "keep")

			engine := New(root, projectStore)
			plan, err := engine.Prepare(project, rule)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			if filepath.Base(plan.ContentRoot) != test.root {
				t.Fatalf("content root = %s", plan.ContentRoot)
			}
			if len(plan.Changes) != 2 {
				t.Fatalf("changes = %#v", plan.Changes)
			}
			result, err := engine.Apply(plan, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.BackupPaths) != 2 {
				t.Fatalf("backup paths = %#v", result.BackupPaths)
			}
			assertContent(t, filepath.Join(target, "index.html"), "index")
			assertContent(t, filepath.Join(target, "assets", "app.js"), "app")
			assertContent(t, filepath.Join(target, "keep.txt"), "keep")
			if _, err := os.Stat(filepath.Join(target, "assets", "old.js")); !os.IsNotExist(err) {
				t.Fatalf("old directory content was not removed: %v", err)
			}
		})
	}
}

func TestArchiveRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	projectStore := store.New(root)
	target := filepath.Join(root, "target")
	rule := model.DeployRule{Name: "bad", Source: "deploy-files/bad.zip", TargetDir: target, Type: "archive", StripTopLevel: "auto"}
	project := model.Project{Code: "demo", Name: "Demo", ManageMode: "external", DeployRules: []model.DeployRule{rule}}
	if err := projectStore.Save(project); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(projectStore.ProjectDir(project.Code), "deploy-files", "bad.zip"), map[string]string{"../escape.txt": "bad"})
	_, err := New(root, projectStore).Prepare(project, rule)
	if err == nil || !strings.Contains(err.Error(), "escapes extraction root") {
		t.Fatalf("expected traversal error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("archive wrote outside extraction root: %v", err)
	}
}

func TestFileDeployCreatesSiblingBackup(t *testing.T) {
	root := t.TempDir()
	projectStore := store.New(root)
	targetDir := filepath.Join(root, "application")
	rule := model.DeployRule{Name: "api", Source: "deploy-files/api.jar", TargetDir: targetDir, TargetName: "api.jar", Type: "file", Backup: true}
	project := model.Project{Code: "demo", Name: "Demo", ManageMode: "external", DeployRules: []model.DeployRule{rule}}
	if err := projectStore.Save(project); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(projectStore.ProjectDir(project.Code), "deploy-files", "api.jar"), "new")
	mustWrite(t, filepath.Join(targetDir, "api.jar"), "old")
	engine := New(root, projectStore)
	plan, err := engine.Prepare(project, rule)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	result, err := engine.Apply(plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.BackupPaths) != 1 {
		t.Fatalf("backup paths = %#v", result.BackupPaths)
	}
	assertContent(t, filepath.Join(targetDir, "api.jar"), "new")
	assertContent(t, result.BackupPaths[0], "old")
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o644)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != expected {
		t.Fatalf("%s = %q, want %q", path, data, expected)
	}
}
