package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	linter "github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/cli"
	"github.com/portpowered/openapi-linter/rulepack"
	"gopkg.in/yaml.v3"
)

type unavailableOutput struct{}

func (unavailableOutput) Write([]byte) (int, error) { return 0, errors.New("output closed") }

func TestCLIArgumentAndOutputContracts(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	config := filepath.Join(root, "policy.yaml")
	write(t, input, spec)
	write(t, config, "version: 1\nrules: []\n")
	for _, args := range [][]string{{"--unknown"}, {"baseline"}, {"--fail-on", "info", input}, {"--baseline", "a", "--baseline-write", "b", input}, {"--format", "xml", input}, {"--kind", "proto", input}, {"--root", root, "--only", "missing", input}, {"--root", root, filepath.Join(root, "missing.yaml")}, {"--root", root, "--rules", config, "--baseline", filepath.Join(root, "missing.json"), input}, {"--root", root, "--rules", config, "--baseline-write", root, input}} {
		if code, _, _ := run(args...); code != 2 {
			t.Fatalf("args %#v returned %d", args, code)
		}
	}
	if code, out, _ := run("--version"); code != 0 || out == "" {
		t.Fatalf("version: %d %q", code, out)
	}
	for _, args := range [][]string{{"--version"}, {"rules", "list"}, {"--root", root, "--rules", config, "--format", "json", input}, {"--root", root, "--rules", config, "--format", "sarif", input}, {"--root", root, "--baseline-write", filepath.Join(root, "debt.json"), input}} {
		if code := cli.Run(context.Background(), args, unavailableOutput{}, io.Discard); code != 2 {
			t.Fatalf("write failure %#v returned %d", args, code)
		}
	}
	// An empty discovered input directory is an operational error.
	empty := filepath.Join(root, "empty")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := run("--root", root, empty); code != 2 {
		t.Fatal("empty inputs accepted")
	}
	if code := cli.Run(context.Background(), []string{"--root", root, "--baseline", filepath.Join(root, "debt.json"), input}, io.Discard, unavailableOutput{}); code != 2 {
		t.Fatal("baseline status output failure accepted")
	}
}

func TestOnlyPreservesSelectedExceptions(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	config := filepath.Join(root, "policy.yaml")
	write(t, input, spec)
	write(t, config, `version: 1
rules:
  - {id: naming, check: openapi.operation-id, options: {pattern: '^lowercase$'}}
  - {id: documentation, check: openapi.operation-summary}
suppressions:
  - {rule: naming, path: api.yaml, reason: approved name}
  - {rule: documentation, path: api.yaml, reason: generated endpoint}
`)
	if code, out, errOut := run("--root", root, "--rules", config, "--only", "naming", input); code != 0 || out != "" {
		t.Fatalf("selected exception lost: %d %s %s", code, out, errOut)
	}
}

func TestSchemaModeUsesSchemaRules(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "schema.yaml")
	config := filepath.Join(root, "policy.yaml")
	write(t, input, "type: object\nproperties:\n  snake_case: {type: string}\n")
	if code, _, errOut := run("--root", root, "--kind", "schema", input); code != 0 {
		t.Fatalf("default schema mode: %d %s", code, errOut)
	}
	write(t, config, "version: 1\nextends: [schema:conventions]\nrules: []\n")
	if code, out, errOut := run("--root", root, "--kind", "schema", "--rules", config, input); code != 1 || out == "" {
		t.Fatalf("schema rules: %d %s %s", code, out, errOut)
	}
	if code, _, _ := run("--root", root, "--rules", config, input); code != 2 {
		t.Fatal("wrong kind accepted")
	}
}

type failingAnalyzer struct{}

func (failingAnalyzer) ID() string { return "customer.failure" }
func (failingAnalyzer) Analyze(_ context.Context, p *linter.Pass) {
	p.ReportError(errors.New("analysis unavailable"))
}

func TestCustomerOperationalFailuresCannotBecomeFindings(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	config := filepath.Join(root, "policy.yaml")
	write(t, input, spec)
	write(t, config, "version: 1\nrules: [{id: policy, check: customer.failure}]\n")
	r := rulepack.NewRegistry()
	if err := r.Register("customer.failure", func(yaml.Node) (linter.Analyzer, error) { return failingAnalyzer{}, nil }); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.RunWithRegistry(context.Background(), []string{"--root", root, "--rules", config, input}, &out, &errOut, r)
	if code != 2 || out.Len() != 0 || errOut.Len() == 0 {
		t.Fatalf("operational failure: %d %s %s", code, &out, &errOut)
	}
}
