package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseFormalAndShortCommands(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   commandRequest
	}{
		{"project create", []string{"project", "create", "demo"}, commandRequest{Action: "project.create", Project: "demo"}},
		{"project new", []string{"p", "new", "demo"}, commandRequest{Action: "project.create", Project: "demo"}},
		{"project list", []string{"project", "list"}, commandRequest{Action: "project.list"}},
		{"project ls", []string{"p", "ls"}, commandRequest{Action: "project.list"}},
		{"project show", []string{"project", "show", "demo"}, commandRequest{Action: "project.show", Project: "demo"}},
		{"project info", []string{"p", "info", "demo"}, commandRequest{Action: "project.show", Project: "demo"}},
		{"project status", []string{"p", "status", "demo"}, commandRequest{Action: "project.status", Project: "demo"}},
		{"project rename", []string{"project", "rename", "demo", "next"}, commandRequest{Action: "project.rename", Project: "demo", NewName: "next"}},
		{"project mv", []string{"p", "mv", "demo", "next"}, commandRequest{Action: "project.rename", Project: "demo", NewName: "next"}},
		{"service add", []string{"service", "add", "demo"}, commandRequest{Action: "service.add", Project: "demo"}},
		{"service list", []string{"s", "ls", "demo"}, commandRequest{Action: "service.list", Project: "demo"}},
		{"service show", []string{"s", "info", "demo", "api"}, commandRequest{Action: "service.show", Project: "demo", Service: "api"}},
		{"service status", []string{"s", "status", "demo", "api"}, commandRequest{Action: "service.status", Project: "demo", Service: "api"}},
		{"service edit", []string{"service", "edit", "demo", "api"}, commandRequest{Action: "service.edit", Project: "demo", Service: "api"}},
		{"service select", []string{"s", "sel", "demo"}, commandRequest{Action: "service.select", Project: "demo"}},
		{"project start", []string{"project", "start", "demo"}, commandRequest{Action: "project.start", Project: "demo"}},
		{"project up all", []string{"p", "up", "demo", "-a"}, commandRequest{Action: "project.start", Project: "demo", All: true}},
		{"project stop all", []string{"project", "stop", "demo", "--all"}, commandRequest{Action: "project.stop", Project: "demo", All: true}},
		{"project stop all force", []string{"p", "down", "demo", "--force", "-a"}, commandRequest{Action: "project.stop", Project: "demo", All: true, Force: true}},
		{"service start", []string{"s", "up", "demo", "api"}, commandRequest{Action: "service.start", Project: "demo", Service: "api"}},
		{"service stop", []string{"service", "stop", "demo", "api"}, commandRequest{Action: "service.stop", Project: "demo", Service: "api"}},
		{"service stop force", []string{"s", "down", "demo", "api", "-f"}, commandRequest{Action: "service.stop", Project: "demo", Service: "api", Force: true}},
		{"deploy list", []string{"d", "ls", "demo"}, commandRequest{Action: "deploy.list", Project: "demo"}},
		{"deploy plan", []string{"deploy", "plan", "demo", "api.jar", "--target", "/opt/api.jar"}, commandRequest{Action: "deploy.plan", Project: "demo", Source: "api.jar", Target: "/opt/api.jar"}},
		{"deploy apply", []string{"deploy", "apply", "demo", "api.jar", "--target", "/opt/api.jar"}, commandRequest{Action: "deploy.apply", Project: "demo", Source: "api.jar", Target: "/opt/api.jar"}},
		{"deploy apply short", []string{"d", "apply", "demo", "api.jar", "-t", "/opt/api.jar"}, commandRequest{Action: "deploy.apply", Project: "demo", Source: "api.jar", Target: "/opt/api.jar"}},
		{"deploy apply yes", []string{"d", "apply", "demo", "api.jar", "--yes", "--target=/opt/api.jar"}, commandRequest{Action: "deploy.apply", Project: "demo", Source: "api.jar", Target: "/opt/api.jar", Yes: true}},
		{"help", []string{"h"}, commandRequest{Action: "help"}},
		{"doctor", []string{"doctor"}, commandRequest{Action: "doctor"}},
		{"version", []string{"v"}, commandRequest{Action: "version"}},
		{"exit", []string{"q"}, commandRequest{Action: "sms.exit"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCommand(test.fields)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParseLegacyCommands(t *testing.T) {
	tests := []struct {
		fields []string
		action string
	}{
		{[]string{"create", "project", "demo"}, "project.create"},
		{[]string{"ls", "-i", "demo"}, "project.show"},
		{[]string{"bd", "-svc", "demo"}, "service.select"},
		{[]string{"st", "-all", "demo"}, "project.start"},
		{[]string{"sp", "-i", "demo", "api"}, "service.stop"},
		{[]string{"dp", "demo"}, "deploy.list"},
	}
	for _, test := range tests {
		request, err := parseCommand(test.fields)
		if err != nil {
			t.Fatal(err)
		}
		if request.Action != test.action {
			t.Fatalf("%v action %q, want %q", test.fields, request.Action, test.action)
		}
	}
}

func TestParseCommandRejectsInvalidArguments(t *testing.T) {
	invalid := [][]string{
		{"project", "start", "demo", "-x"},
		{"project", "stop"},
		{"service", "show", "demo"},
		{"deploy", "apply", "demo", "api.jar", "--wrong", "/opt/api.jar"},
		{"deploy", "plan", "demo", "api.jar", "--target", "/opt/api.jar", "--yes"},
		{"p", "unknown"},
	}
	for _, fields := range invalid {
		if _, err := parseCommand(fields); err == nil {
			t.Fatalf("parseCommand(%v) unexpectedly succeeded", fields)
		}
	}
}

func TestExecuteProjectRenameAndServiceShow(t *testing.T) {
	root := t.TempDir()
	project := Project{
		Name: "demo",
		Services: []Service{{
			Name: "api", StartPath: "/opt/demo/api.jar", Port: 8080,
			StartCommand: "java -jar api.jar", CommandSource: "custom", RestartMode: "kill-start",
		}},
	}
	if _, err := saveProject(root, project); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	renameRequest, err := parseCommand([]string{"p", "mv", "demo", "renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := executeCommand(nil, &output, root, renameRequest); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadProject(root, "renamed"); err != nil {
		t.Fatal(err)
	}

	output.Reset()
	showRequest, err := parseCommand([]string{"service", "show", "renamed", "api"})
	if err != nil {
		t.Fatal(err)
	}
	if err := executeCommand(nil, &output, root, showRequest); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Start command: java -jar api.jar") {
		t.Fatalf("service output missing command: %s", output.String())
	}
}
