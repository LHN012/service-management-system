package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveAppRootUsesExplicitDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMS_ROOT", "")

	resolved, err := resolveAppRoot([]string{"--root", root})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Clean(root) {
		t.Fatalf("resolved root %q, want %q", resolved, root)
	}
}

func TestResolveAppRootUsesEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMS_ROOT", root)

	resolved, err := resolveAppRoot(nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Clean(root) {
		t.Fatalf("resolved root %q, want %q", resolved, root)
	}
}

func TestAppRootUsesExecutableDirectory(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "sms")
	if resolved := appRootFromExecutable(executable); resolved != root {
		t.Fatalf("app root %q, want %q", resolved, root)
	}
}

func TestSplitInvocationArgs(t *testing.T) {
	rootArgs, commandArgs, err := splitInvocationArgs([]string{"--root", "/opt/sms", "p", "ls"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rootArgs, []string{"--root", "/opt/sms"}) {
		t.Fatalf("root args = %v", rootArgs)
	}
	if !reflect.DeepEqual(commandArgs, []string{"p", "ls"}) {
		t.Fatalf("command args = %v", commandArgs)
	}
}

func TestStandaloneDiagnosticInvocationDetection(t *testing.T) {
	if !wantsVersion([]string{"v"}) || !wantsVersion([]string{"VERSION"}) {
		t.Fatal("version invocation was not detected")
	}
	if !wantsDoctor([]string{"DOCTOR"}) {
		t.Fatal("doctor invocation was not detected")
	}
	if wantsDoctor([]string{"doctor", "extra"}) || wantsVersion([]string{"v", "extra"}) {
		t.Fatal("invalid diagnostic invocation was accepted")
	}
}

func TestRunDirectShortProjectList(t *testing.T) {
	root := t.TempDir()
	logger, err := newOperationLogger(root)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runDirect(strings.NewReader(""), &output, root, logger, []string{"p", "ls"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "no projects") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestRunShowsPortableShellHeader(t *testing.T) {
	root := t.TempDir()
	logger, err := newOperationLogger(root)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := run(strings.NewReader("exit\n"), &output, root, logger); err != nil {
		t.Fatal(err)
	}

	want := "SMS Linux\nRoot: " + root + "\nType help for commands, exit to quit."
	if !strings.Contains(output.String(), want) {
		t.Fatalf("shell header = %q, want it to contain %q", output.String(), want)
	}
}

func TestRunRecordsSuccessfulAndFailedCommands(t *testing.T) {
	root := t.TempDir()
	logger, err := newOperationLogger(root)
	if err != nil {
		t.Fatal(err)
	}
	logger.now = func() time.Time {
		return time.Date(2026, time.July, 15, 20, 0, 0, 0, time.Local)
	}

	input := strings.NewReader("help\nunknown-command\nexit\n")
	var output bytes.Buffer
	if err := run(input, &output, root, logger); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "logs", "sms-2026-07.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d log entries, want 3: %s", len(lines), data)
	}

	wantActions := []string{"help", "command.unknown", "sms.exit"}
	wantResults := []string{"success", "failed", "success"}
	for index, line := range lines {
		var entry OperationLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Action != wantActions[index] || entry.Result != wantResults[index] {
			t.Fatalf("entry %d = %#v", index, entry)
		}
	}
}
