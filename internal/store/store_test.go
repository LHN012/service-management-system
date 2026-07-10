package store

import (
	"os"
	"path/filepath"
	"testing"

	"sms/internal/model"
)

func TestSaveLoadAndListProject(t *testing.T) {
	root := t.TempDir()
	projectStore := New(root)
	project := model.Project{Code: "demo", Name: "Demo", ManageMode: "external"}
	if err := projectStore.Save(project); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "projects", "demo", "project.yml"),
		filepath.Join(root, "projects", "demo", "deploy-files"),
		filepath.Join(root, "projects", "demo", "backups", "dirs"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	loaded, err := projectStore.Load("demo")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Demo" {
		t.Fatalf("loaded name = %q", loaded.Name)
	}
	projects, err := projectStore.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Code != "demo" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}
