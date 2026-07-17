package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunDeployPlansAndAppliesWithBackup(t *testing.T) {
	root := t.TempDir()
	if _, err := saveProject(root, Project{Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "projects", "demo", "deploy-files", "api.jar")
	if err := os.WriteFile(source, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "api.jar")
	if err := os.WriteFile(target, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	plan := commandRequest{Action: "deploy.plan", Project: "demo", Source: "api.jar", Target: target}
	if err := runDeploy(bufio.NewScanner(strings.NewReader("")), &output, root, plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Deployment plan:") {
		t.Fatalf("plan output=%q", output.String())
	}
	data, _ := os.ReadFile(target)
	if string(data) != "old" {
		t.Fatalf("plan changed target to %q", data)
	}

	output.Reset()
	apply := commandRequest{Action: "deploy.apply", Project: "demo", Source: "api.jar", Target: target, Yes: true}
	if err := runDeploy(bufio.NewScanner(strings.NewReader("")), &output, root, apply); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" || !strings.Contains(output.String(), "backup:") {
		t.Fatalf("content=%q output=%q", data, output.String())
	}
}

func TestCommitStagedTargetRestoresPreviousTargetOnFailure(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := commitStagedTarget(filepath.Join(directory, "missing-stage"), target); err == nil {
		t.Fatal("commit unexpectedly succeeded")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("target was not restored: %q", data)
	}
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "payload.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(archivePath, filepath.Join(directory, "extract")); err == nil {
		t.Fatal("path traversal archive was accepted")
	}
}

func TestReplaceTargetDirectoryPreservesRootMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux permission test")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	target := filepath.Join(directory, "target")
	if err := os.Mkdir(source, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "file"), []byte("data"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := replaceTarget(source, target); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("target mode=%o, want 750", info.Mode().Perm())
	}
}
