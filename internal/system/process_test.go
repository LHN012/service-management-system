package system

import (
	"testing"

	"sms/internal/model"
)

func TestMatchesBackendUsesCommandCWDAndPort(t *testing.T) {
	backend := model.Backend{
		Name: "api", WorkDir: "/opt/demo", ExpectedPorts: []int{8080},
		Match: model.MatchRule{CommandContains: "demo.jar", CWD: "/opt/demo"},
	}
	process := ProcessInfo{PID: 12, Command: "java -jar demo.jar", CWD: "/opt/demo", Ports: []int{8080}}
	score, matched := MatchesBackend(backend, process)
	if score != 9 || len(matched) != 3 {
		t.Fatalf("score=%d matched=%v", score, matched)
	}
}

func TestMatchesBackendRequiresAHighConfidenceMatch(t *testing.T) {
	backend := model.Backend{Name: "api", WorkDir: "/opt/demo", Match: model.MatchRule{CommandContains: "demo.jar"}}
	process := ProcessInfo{PID: 12, Command: "python app.py", CWD: "/tmp"}
	score, _ := MatchesBackend(backend, process)
	if score != 0 {
		t.Fatalf("score=%d", score)
	}
}
