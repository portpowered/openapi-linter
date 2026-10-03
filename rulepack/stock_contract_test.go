package rulepack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

type contractVisitor struct{}

func (contractVisitor) Name() string                                        { return "customer.visitor" }
func (contractVisitor) VisitSchema(string, *base.Schema) []linter.Violation { return nil }
func (contractVisitor) VisitPath(string, *v3.PathItem) []linter.Violation   { return nil }
func (contractVisitor) VisitOperation(path, method string, _ *v3.Operation) []linter.Violation {
	return []linter.Violation{{RuleName: "customer.visitor", Path: path + " " + method, Message: "operation reviewed"}}
}

type contractSchemaVisitor struct{}

func (contractSchemaVisitor) Name() string { return "customer.schema" }
func (contractSchemaVisitor) VisitSchema(path string, _ *linter.SchemaDocument) []linter.Violation {
	return []linter.Violation{{RuleName: "customer.schema", Path: path, Message: "root reviewed"}, {RuleName: "customer.schema", Pointer: "#/properties/id", Message: "field reviewed"}}
}

func stockContractDocument(t *testing.T, root, filename, source string) *linter.Document {
	t.Helper()
	file := filepath.Join(root, filename)
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(source), &node); err != nil {
		t.Fatal(err)
	}
	return &linter.Document{Path: file, Source: []byte(source), Root: &node, Model: &v3.Document{}}
}

func TestStockVisitorContracts(t *testing.T) {
	root := t.TempDir()
	raw := stockContractDocument(t, root, "api.yaml", "openapi: 3.0.3\ninfo: {title: Example, version: '1'}\npaths:\n  /records:\n    get:\n      responses: {'200': {description: OK}}\n")
	doc, err := linter.LoadDocument(context.Background(), raw.Path, root)
	if err != nil {
		t.Fatal(err)
	}
	check := AdaptRule(contractVisitor{})
	if check.ID() != "customer.visitor" {
		t.Fatal("adapter changed visitor identity")
	}
	pass := linter.NewPass([]*linter.Document{{Path: "schema.yaml"}, doc})
	check.Analyze(context.Background(), pass)
	findings := pass.Diagnostics()
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1records/get" || findings[0].Path != doc.Path || findings[0].Line != 6 || findings[0].Context != "/records GET" || findings[0].Severity != linter.SeverityError {
		t.Fatalf("visitor lost source/context attribution: %#v", findings)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{doc})
	check.Analyze(ctx, pass)
	if len(pass.Diagnostics()) != 0 {
		t.Fatal("canceled visitor ran")
	}
}

func TestStandaloneSchemaVisitorContracts(t *testing.T) {
	doc := stockContractDocument(t, t.TempDir(), "schema.yaml", "type: object\nproperties:\n  id: {type: string}\n")
	doc.Model = nil
	doc.Schema = &linter.SchemaDocument{Content: map[string]any{"type": "object"}}
	check := AdaptSchemaRule(contractSchemaVisitor{})
	if check.ID() != "customer.schema" {
		t.Fatal("adapter changed schema identity")
	}
	pass := linter.NewPass([]*linter.Document{{Model: &v3.Document{}}, doc})
	check.Analyze(context.Background(), pass)
	findings := pass.Diagnostics()
	if len(findings) != 2 || findings[0].Pointer != "#" || findings[0].Line != 1 || findings[1].Pointer != "#/properties/id" || findings[1].Line != 3 || findings[0].Context != doc.Path {
		t.Fatalf("schema fallback/source mapping failed: %#v", findings)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{doc})
	check.Analyze(ctx, pass)
	if len(pass.Diagnostics()) != 0 {
		t.Fatal("canceled schema visitor ran")
	}
}

func TestStockFactoryContracts(t *testing.T) {
	for _, id := range []string{"openapi.path-naming", "schema.no-anonymous-objects", "openapi.operation-id", "openapi.spec-structure"} {
		t.Run("duplicate "+id, func(t *testing.T) {
			r := NewRegistry()
			if err := r.Register(id, newOperationID); err != nil {
				t.Fatal(err)
			}
			if err := RegisterStock(r); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("duplicate accepted: %v", err)
			}
		})
	}
	r := NewRegistry()
	if err := RegisterStock(r); err != nil {
		t.Fatal(err)
	}
	var opts yaml.Node
	if err := yaml.Unmarshal([]byte("unsupported: true"), &opts); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"openapi.path-naming", "openapi.schema-naming", "openapi.schema-description", "openapi.request-response-example", "openapi.property-camel-case", "schema.no-anonymous-objects", "schema.property-camel-case"} {
		if _, err := r.factories[id](*opts.Content[0]); err == nil {
			t.Fatalf("unknown options accepted by %s", id)
		}
		if check, err := r.factories[id](yaml.Node{}); err != nil || check.ID() == "" {
			t.Fatalf("valid stock factory %s: %v", id, err)
		}
	}
	if len(DefaultPack().Rules) == 0 {
		t.Fatal("empty default rules")
	}
}

func TestOperationIDOptionsAndBehavior(t *testing.T) {
	for _, opts := range []string{"pattern: '['", "unknown: true", "required: [true]", "[required]"} {
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(opts), &node); err != nil {
			t.Fatal(err)
		}
		if _, err := newOperationID(*node.Content[0]); err == nil {
			t.Fatalf("invalid operation options accepted: %s", opts)
		}
	}
	for _, tc := range []struct {
		operation, options string
		count              int
	}{
		{"", "", 1},
		{"", "required: false", 0},
		{"operationId: GetRecords", "pattern: '^Get[A-Z]\\w+$'", 0},
		{"operationId: get_records", "pattern: '^Get[A-Z]\\w+$'", 1},
		{"operationId: get_records", "", 0},
	} {
		got := checkAPI(t, "openapi.operation-id", "openapi: 3.0.3\ninfo: {title: Example, version: '1'}\npaths:\n  /records:\n    get:\n      "+tc.operation+"\n      responses: {'200': {description: OK}}\n", tc.options)
		if len(got) != tc.count {
			t.Fatalf("%+v produced %#v", tc, got)
		}
	}
}

func TestOperationIDFailuresAndReferences(t *testing.T) {
	check, err := newOperationID(yaml.Node{})
	if err != nil || check.ID() != "openapi.operation-id" {
		t.Fatal(err)
	}
	pass := linter.NewPass([]*linter.Document{{Path: "schema.yaml"}, {Path: "api.yaml", Model: &v3.Document{}}})
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil || len(pass.Diagnostics()) != 0 || !strings.Contains(pass.Err().Error(), "LoadDocument") {
		t.Fatalf("missing AST must be operational failure: %v", pass.Err())
	}
	root := t.TempDir()
	doc := stockContractDocument(t, root, "api.yaml", "openapi: 3.0.3\npaths:\n  /records: {$ref: missing.yaml}\n")
	pass = linter.NewPass([]*linter.Document{doc})
	pass.Root = root
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("missing local reference was hidden")
	}
	stockContractDocument(t, root, "paths.yaml", "get:\n  responses: {'200': {description: OK}}\n")
	doc = stockContractDocument(t, root, "external.yaml", "openapi: 3.0.3\npaths:\n  /records: {$ref: paths.yaml}\n")
	pass = linter.NewPass([]*linter.Document{doc})
	pass.Root = root
	check.Analyze(context.Background(), pass)
	findings := pass.Diagnostics()
	if pass.Err() != nil || len(findings) != 1 || findings[0].Pointer != "#/paths/~1records" || findings[0].Line != 3 {
		t.Fatalf("external finding must point to importing site: %#v %v", findings, pass.Err())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{doc})
	check.Analyze(ctx, pass)
	if pass.Err() != nil || len(pass.Diagnostics()) != 0 {
		t.Fatal("canceled operation check ran")
	}
}
