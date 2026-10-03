package rulepack

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

type contractAnalyzer struct {
	fn func(context.Context, *linter.Pass)
}

func (contractAnalyzer) ID() string                                    { return "test.logic" }
func (c contractAnalyzer) Analyze(ctx context.Context, p *linter.Pass) { c.fn(ctx, p) }

func TestCompilerRejectsInvalidPolicyAndFactoryContracts(t *testing.T) {
	r := NewRegistry()
	if err := RegisterStock(r); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("nil.factory", func(yaml.Node) (linter.Analyzer, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("failed.factory", func(yaml.Node) (linter.Analyzer, error) { return nil, errors.New("failed") }); err != nil {
		t.Fatal(err)
	}
	for _, pack := range []Pack{
		{Version: 1, Extends: []string{"openapi:core"}},
		{Version: 1, Overrides: []Override{{ID: "unknown"}}},
		{Version: 1, Rules: []Rule{{ID: "", Check: "openapi.operation-id"}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "openapi.operation-id", Severity: "fatal"}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "openapi.operation-id", Include: []string{"../outside"}}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "nil.factory"}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "failed.factory"}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "openapi.operation-id"}}, Suppressions: []Suppression{{Rule: "policy", Path: "api.yaml", Reason: "", Line: -1}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "openapi.operation-id"}}, Suppressions: []Suppression{{Rule: "policy", Path: "../outside", Reason: "invalid path"}}},
		{Version: 1, Rules: []Rule{{ID: "policy", Check: "openapi.operation-id"}}, Suppressions: []Suppression{{Rule: "policy", Path: "api.yaml", Reason: "invalid pointer", Pointer: "/paths"}}},
	} {
		if _, err := r.Compile(pack); err == nil {
			t.Fatalf("accepted invalid pack %#v", pack)
		}
	}
	if err := r.Register("", nil); err == nil {
		t.Fatal("invalid registry entry accepted")
	}
	if err := r.Register("openapi.operation-id", newOperationID); err == nil {
		t.Fatal("duplicate factory accepted")
	}
}

func TestProgramBoundsReportsAndCancellation(t *testing.T) {
	root := t.TempDir()
	doc := &linter.Document{Path: filepath.Join(root, "api.yaml")}
	for _, scenario := range []string{"nil doc", "outside doc", "outside report", "unselected report", "operational", "cancel before", "cancel during"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			docs := []*linter.Document{doc}
			if scenario == "nil doc" {
				docs = []*linter.Document{nil}
			}
			if scenario == "outside doc" {
				docs = []*linter.Document{{Path: filepath.Join(filepath.Dir(root), "outside.yaml")}}
			}
			if scenario == "cancel before" {
				cancel()
			}
			a := contractAnalyzer{func(_ context.Context, p *linter.Pass) {
				switch scenario {
				case "outside report":
					p.Report(linter.Diagnostic{Path: filepath.Join(filepath.Dir(root), "outside.yaml")})
				case "unselected report":
					p.Report(linter.Diagnostic{Path: filepath.Join(root, "unselected.yaml")})
				case "operational":
					p.ReportError(errors.New("cannot analyze"))
				case "cancel during":
					cancel()
				}
			}}
			p := &Program{rules: []compiledRule{{spec: Rule{ID: "policy", Check: "customer", Severity: linter.SeverityError}, analyzer: a}}}
			if findings, err := p.Run(ctx, root, docs); err == nil || len(findings) != 0 {
				t.Fatalf("scenario %s: %#v %v", scenario, findings, err)
			}
		})
	}
}

func TestProgramSortsAndScopesCustomerFindings(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.yaml")
	b := filepath.Join(root, "b.yaml")
	input := []linter.Diagnostic{
		{Path: b, Line: 1, Message: "a"}, {Path: a, Line: 2, Message: "b"}, {Path: a, Line: 1, Message: "z"},
		{Path: a, Line: 1, Message: "a", Pointer: "#/b"}, {Path: a, Line: 1, Message: "a", Pointer: "#/a", Context: "z"}, {Path: a, Line: 1, Message: "a", Pointer: "#/a", Context: "a"},
	}
	p := &Program{rules: []compiledRule{{spec: Rule{ID: "policy", Check: "customer", Severity: linter.SeverityInfo, Origin: "local"}, analyzer: contractAnalyzer{func(_ context.Context, p *linter.Pass) {
		for _, d := range input {
			p.Report(d)
		}
	}}}}}
	findings, err := p.Run(context.Background(), root, []*linter.Document{{Path: a}, {Path: b}})
	if err != nil || len(findings) != 6 {
		t.Fatalf("findings %#v %v", findings, err)
	}
	want := []linter.Diagnostic{input[5], input[4], input[3], input[2], input[1], input[0]}
	for i := range want {
		want[i].RuleID = "policy"
		want[i].CheckID = "customer"
		want[i].Severity = linter.SeverityInfo
		want[i].Origin = "local"
	}
	if !reflect.DeepEqual(findings, want) {
		t.Fatalf("sort/provenance: %#v", findings)
	}
	p.rules[0].spec.Include = []string{"other/**"}
	p.rules[0].analyzer = contractAnalyzer{func(_ context.Context, p *linter.Pass) {
		if len(p.Documents) != 0 {
			t.Fatal("scope not honored")
		}
	}}
	if findings, err := p.Run(context.Background(), root, []*linter.Document{{Path: a}}); err != nil || len(findings) != 0 {
		t.Fatalf("empty scope: %#v %v", findings, err)
	}
}

type contractSchemaRule struct{}

func (contractSchemaRule) Name() string { return "customer.schema" }
func (contractSchemaRule) VisitSchema(path string, _ *linter.SchemaDocument) []linter.Violation {
	return []linter.Violation{{Path: path, RuleName: "customer.schema", Message: "root"}, {Path: path, RuleName: "customer.schema", Pointer: "#/type", Message: "type"}}
}

func TestStandaloneSchemaAdapterLocationsAndCancellation(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte("type: string\n"), &root); err != nil {
		t.Fatal(err)
	}
	doc := &linter.Document{Path: "schema.yaml", Root: &root, Schema: &linter.SchemaDocument{Content: map[string]any{"type": "string"}}}
	a := AdaptSchemaRule(contractSchemaRule{})
	if a.ID() != "customer.schema" {
		t.Fatal("adapter identity lost")
	}
	p := linter.NewPass([]*linter.Document{{}, doc})
	a.Analyze(context.Background(), p)
	findings := p.Diagnostics()
	if len(findings) != 2 || findings[0].Pointer != "#" || findings[1].Pointer != "#/type" || findings[0].Line != 1 {
		t.Fatalf("adapter locations %#v", findings)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p = linter.NewPass([]*linter.Document{doc})
	a.Analyze(ctx, p)
	if len(p.Diagnostics()) != 0 {
		t.Fatal("canceled schema adapter ran")
	}
}
