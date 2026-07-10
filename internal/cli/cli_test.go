package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitCreateAndListProject(t *testing.T) {
	root := t.TempDir()
	input := strings.NewReader("demo\n\n\n\n0\n0\n0\nyes\n")
	var output bytes.Buffer
	application := New(root, input, &output, &output)
	if err := application.Run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run([]string{"p", "-c"}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run([]string{"p", "-l"}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "Created") || !strings.Contains(text, "demo") || !strings.Contains(text, "manageMode: external") {
		t.Fatalf("unexpected output:\n%s", text)
	}
}

func TestResolveTargetPrefersFullProjectCode(t *testing.T) {
	root := t.TempDir()
	application := New(root, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	for _, project := range []struct{ code, name string }{{"demo", "Demo"}, {"demo-front", "Demo Front"}} {
		if err := application.Store.Save(minimalProject(project.code, project.name)); err != nil {
			t.Fatal(err)
		}
	}
	resolved, group, component, err := application.resolveTarget("demo-front")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Code != "demo-front" || group != "" || component != "" {
		t.Fatalf("resolved=%s group=%s component=%s", resolved.Code, group, component)
	}
}
