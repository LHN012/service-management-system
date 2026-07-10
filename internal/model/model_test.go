package model

import "testing"

func TestProjectValidate(t *testing.T) {
	project := Project{
		Code: "demo", Name: "Demo", ManageMode: "external",
		Backends: []Backend{{
			Name: "api", WorkDir: "/opt/demo", StartCommand: "java -jar demo.jar",
			ExpectedPorts: []int{8080}, Match: MatchRule{CommandContains: "demo.jar"},
		}},
		DeployRules: []DeployRule{{
			Name: "api-jar", Source: "deploy-files/demo.jar", TargetDir: "/opt/demo",
			TargetName: "demo.jar", Type: "file", Backup: true,
		}},
	}
	if err := project.Validate(); err != nil {
		t.Fatalf("valid project rejected: %v", err)
	}
}

func TestDeployRuleRejectsSourceEscape(t *testing.T) {
	rule := DeployRule{Name: "bad", Source: "deploy-files/../../etc/passwd", TargetDir: "/tmp", Type: "file"}
	if err := rule.Validate(); err == nil {
		t.Fatal("expected source escape validation error")
	}
}

func TestProjectRejectsDuplicateComponentNames(t *testing.T) {
	project := Project{
		Code: "demo", Name: "Demo", ManageMode: "external",
		Backends:  []Backend{{Name: "web", WorkDir: "/tmp", StartCommand: "run", ExpectedPorts: []int{1}}},
		Frontends: []Frontend{{Name: "web", NginxMode: "shared", ExpectedPorts: []int{80}}},
	}
	if err := project.Validate(); err == nil {
		t.Fatal("expected duplicate component validation error")
	}
}
