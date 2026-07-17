package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOperationLoggerWritesStructuredMonthlyLog(t *testing.T) {
	root := t.TempDir()
	logger, err := newOperationLogger(root)
	if err != nil {
		t.Fatal(err)
	}
	logger.now = func() time.Time {
		return time.Date(2026, time.July, 15, 10, 30, 0, 0, time.Local)
	}

	if err := logger.Log("project.list", "ls", "success", ""); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "logs", "sms-2026-07.log"))
	if err != nil {
		t.Fatal(err)
	}
	var entry OperationLogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Action != "project.list" || entry.Command != "ls" || entry.Result != "success" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func TestOperationLoggerArchivesCompletedMonth(t *testing.T) {
	root := t.TempDir()
	logger, err := newOperationLogger(root)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(root, "logs", "sms-2026-06.log")
	oldContent := []byte("previous month\n")
	if err := os.WriteFile(oldPath, oldContent, 0o640); err != nil {
		t.Fatal(err)
	}
	logger.now = func() time.Time {
		return time.Date(2026, time.July, 1, 0, 0, 1, 0, time.Local)
	}

	if err := logger.Log("sms.start", "", "success", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old log still exists: %v", err)
	}

	archivePath := filepath.Join(root, "logs", "archive", "sms-2026-06.tar.gz")
	archivedName, archivedData := readSingleTarGzFile(t, archivePath)
	if archivedName != "sms-2026-06.log" {
		t.Fatalf("unexpected archived name %q", archivedName)
	}
	if string(archivedData) != string(oldContent) {
		t.Fatalf("unexpected archived content %q", archivedData)
	}
}

func readSingleTarGzFile(t *testing.T, path string) (string, []byte) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tarReader.Next(); err != io.EOF {
		t.Fatalf("archive contains extra entries or is invalid: %v", err)
	}
	return header.Name, data
}
