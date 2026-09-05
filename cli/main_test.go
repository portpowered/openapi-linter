package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	linter "github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/cli"
	"github.com/portpowered/openapi-linter/rulepack"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const spec = `openapi: 3.0.3
info:
  title: Customer API
  version: 1.0.0
paths:
  /widgets:
    get:
      operationId: ListWidgets
      responses:
        '200':
          description: Success
`

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}
func TestCustomerPackIdentityLocationAndSuppression(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	config := filepath.Join(root, "rules.yaml")
	write(t, input, spec)
	pack := `version: 1
rules:
  - id: customer.naming
    check: openapi.operation-id
    severity: warning
    options:
      pattern: '^[a-z][A-Za-z0-9]*$'
`
	write(t, config, pack)
	args := []string{"--root", root, "--rules", config, "--format", "json", input}
	code, out, stderr := run(args...)
	if code != 0 || stderr != "" {
		t.Fatalf("%d %s", code, stderr)
	}
	var diagnostics []linter.Diagnostic
	if err := json.Unmarshal([]byte(out), &diagnostics); err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].RuleID != "customer.naming" || diagnostics[0].Severity != linter.SeverityWarning || diagnostics[0].Pointer != "#/paths/~1widgets/get" || diagnostics[0].Line != 8 {
		t.Fatalf("%+v", diagnostics)
	}
	_, again, _ := run(args...)
	if out != again {
		t.Fatal("nondeterministic output")
	}
	write(t, config, strings.Replace(pack, "warning", "error", 1))
	if code, _, _ := run(args...); code != 1 {
		t.Fatalf("error exit=%d", code)
	}
	write(t, config, pack+`suppressions:
  - rule: customer.naming
    path: api.yaml
    reason: Legacy public operation ID
`)
	if code, out, stderr := run(args...); code != 0 || strings.TrimSpace(out) != "[]" {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
}
func TestInvalidPackAndInputs(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	config := filepath.Join(root, "rules.yaml")
	write(t, input, spec)
	for _, pack := range []string{
		"version: 2\nrules: []\n",
		"version: 1\nunknown: true\nrules: []\n",
		"version: 1\nrules: [{id: x, check: missing}]\n",
		"version: 1\nrules: [{id: x, check: openapi.operation-id, options: {unknown: true}}]\n",
		"version: 1\nrules: [{id: x, check: openapi.operation-id}, {id: x, check: openapi.operation-id}]\n",
	} {
		write(t, config, pack)
		if code, _, stderr := run("--root", root, "--rules", config, input); code != 2 {
			t.Fatalf("%d %s", code, stderr)
		}
	}
	write(t, config, "version: 1\nrules: []\n")
	if code, out, stderr := run("--root", root, "--rules", config, input); code != 0 || out != "" {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	if code, _, _ := run(); code != 2 {
		t.Fatalf("missing input exit=%d", code)
	}
}

type customerCheck struct {
	Word string `yaml:"word"`
}

func (customerCheck) ID() string { return "customer.text" }
func (c customerCheck) Analyze(_ context.Context, pass *linter.Pass) {
	for _, doc := range pass.Documents {
		if !bytes.Contains(doc.Source, []byte(c.Word)) {
			pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: "#/info", Line: doc.Line("#/info"), Message: "missing customer word", Severity: linter.SeverityError})
		}
	}
}

var _ linter.Analyzer = customerCheck{}

func TestCustomerExecutableUsesSameCommand(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	config := filepath.Join(root, "rules.yaml")
	write(t, input, spec)
	write(t, config, "version: 1\nrules:\n  - id: customer.policy\n    check: customer.text\n    severity: info\n    options: {word: Banana}\n")
	registry := rulepack.NewRegistry()
	if err := rulepack.RegisterStock(registry); err != nil {
		t.Fatal(err)
	}
	err := registry.Register("customer.text", func(node yaml.Node) (linter.Analyzer, error) {
		var c customerCheck
		if err := rulepack.DecodeOptions(node, &c); err != nil {
			return nil, err
		}
		if c.Word == "" {
			return nil, fmt.Errorf("word required")
		}
		return c, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.RunWithRegistry(context.Background(), []string{"--root", root, "--rules", config, input}, &out, &errOut, registry)
	if code != 0 || !strings.Contains(out.String(), "customer.policy: missing customer word") || errOut.Len() != 0 {
		t.Fatalf("%d %s %s", code, &out, &errOut)
	}
}

func TestSupportedOpenAPIVersions(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			input := filepath.Join(root, "api.yaml")
			write(t, input, strings.Replace(spec, "3.0.3", version, 1))
			if code, _, stderr := run("--root", root, input); code != 0 {
				t.Fatalf("%d %s", code, stderr)
			}
		})
	}
}
