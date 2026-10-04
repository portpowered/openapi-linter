package rulepack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

func TestSiteRuleConfigurations(t *testing.T) {
	data, err := os.ReadFile("../docs/rule-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var references map[string]struct {
		Options json.RawMessage `json:"example-options"`
	}
	if err := json.Unmarshal(data, &references); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := RegisterStock(registry); err != nil {
		t.Fatal(err)
	}
	if len(references) != len(registry.Catalog()) {
		t.Fatal("rule reference must cover the entire registry")
	}
	for _, descriptor := range registry.Catalog() {
		t.Run(descriptor.ID, func(t *testing.T) {
			var options yaml.Node
			if err := yaml.Unmarshal(references[descriptor.ID].Options, &options); err != nil {
				t.Fatal(err)
			}
			if _, err := registry.Compile(Pack{Version: 1, Rules: []Rule{{ID: descriptor.ID, Check: descriptor.ID, Options: *options.Content[0]}}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLibraryGuideIntegration(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "api.yaml")
	source := "openapi: 3.0.3\ninfo: {title: Example, version: '1', description: Example API}\npaths:\n  /widgets:\n    get:\n      operationId: GetWidgets\n      description: Get widgets\n      responses:\n        '200': {description: Success}\n"
	if err := os.WriteFile(filename, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := RegisterStock(registry); err != nil {
		t.Fatal(err)
	}
	pack, err := Load("openapi:recommended", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateKind(pack, "openapi"); err != nil {
		t.Fatal(err)
	}
	program, err := registry.Compile(pack)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := linter.LoadDocument(context.Background(), filename, root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := program.Run(context.Background(), root, []*linter.Document{doc})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if finding.CheckID == "openapi.operation-summary" && finding.Pointer != "" {
			return
		}
	}
	t.Fatalf("public integration missed summary defect: %#v", findings)
}
