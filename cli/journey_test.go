package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortosFixtureAndConfigurationJourney(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run("--rules", filepath.Join(root, "examples/portos/rules.yaml"), "--root", root, filepath.Join(root, "examples/portos/api.yaml"))
	if code != 0 || out != "" {
		t.Fatalf("Portos fixture: %d %s %s", code, out, stderr)
	}
	temp := t.TempDir()
	config := filepath.Join(temp, ".openapilint.yaml")
	file := filepath.Join(temp, "api.yaml")
	baseline := filepath.Join(temp, "baseline.json")
	code, _, stderr = run("init", "--preset", "openapi:core", "--output", config)
	if code != 0 {
		t.Fatal(stderr)
	}
	write(t, file, strings.Replace(spec, "operationId: ListWidgets", "operationId: ''", 1))
	code, out, stderr = run("--root", temp, file)
	if code != 1 || !strings.Contains(out, "openapi.operation-id") {
		t.Fatalf("discovery: %d %s %s", code, out, stderr)
	}
	code, _, stderr = run("baseline", "create", "--output", baseline, "--root", temp, file)
	if code != 0 {
		t.Fatal(stderr)
	}
	code, out, stderr = run("--baseline", baseline, "--format", "json", "--root", temp, file)
	if code != 0 || !strings.Contains(stderr, "known") {
		t.Fatalf("baseline: %d %s %s", code, out, stderr)
	}
	code, out, stderr = run("--format", "sarif", "--root", temp, file)
	var sarif map[string]any
	if code != 1 || json.Unmarshal([]byte(out), &sarif) != nil || sarif["version"] != "2.1.0" {
		t.Fatalf("SARIF: %d %s %s", code, out, stderr)
	}
	code, out, stderr = run("config", "explain", "--rules", config, "--root", temp, "--path", "api.yaml")
	if code != 0 || !strings.Contains(out, "openapi.operation-id") || !strings.Contains(out, "preset:openapi:core") {
		t.Fatalf("explain: %d %s %s", code, out, stderr)
	}
}
