package rulepack

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

func TestOfficialStructuralSchemasReportInvalidLocations(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.0"} {
		source := "openapi: " + version + "\ninfo: {title: Test, version: v1}\npaths:\n  /widgets:\n    get:\n      responses:\n        '200': {description: 42}\nservers: [{url: 'https://{region}.example.test', variables: {region: {enum: [us]}}}]\n"
		findings := checkAPI(t, "openapi.spec-structure", source, "")
		if len(findings) == 0 {
			t.Fatalf("%s accepted invalid description/server variable", version)
		}
		for _, f := range findings {
			if !strings.Contains(f.Message, "OpenAPI structure:") {
				t.Fatalf("unexpected diagnostic %#v", f)
			}
		}
	}
	if _, err := structuralSchema("2.0"); err == nil {
		t.Fatal("unsupported structural schema requested")
	}
}

func TestStructureYAMLConversionFailuresAreOperational(t *testing.T) {
	for _, source := range []string{"openapi: 3.1.0\n1: value\n", "openapi: 3.1.0\ninfo: {title: Test, version: .nan}\n", "openapi: 3.1.0\ninfo: &loop {self: *loop}\n"} {
		var root yaml.Node
		if err := yaml.Unmarshal([]byte(source), &root); err != nil {
			t.Fatal(err)
		}
		if err := (apiCheck{}).validateStructure(site{n: unwrap(&root), ptr: "#"}, func(site, string) { t.Fatal("conversion error became policy finding") }); err == nil {
			t.Fatalf("accepted incompatible source %q", source)
		}
	}
}

func TestCanceledExampleAnalysisProducesNoFindings(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte("openapi: 3.1.0\ncomponents: {schemas: {Value: {type: string, example: 1}}}\n"), &root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "api.yaml")
	g := graph{files: map[string]*yaml.Node{path: unwrap(&root)}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := linter.NewPass(nil)
	(apiCheck{}).validateExamples(ctx, &g, site{n: unwrap(&root), file: path, ptr: "#"}, func(site, string) { t.Fatal("canceled example validation ran") }, p)
	if p.Err() != nil {
		t.Fatalf("canceled direct example validation error: %v", p.Err())
	}
}
