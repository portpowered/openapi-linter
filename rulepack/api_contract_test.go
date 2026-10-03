package rulepack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

func contractSource(t *testing.T, value any) string {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func contractSite(t *testing.T, source, file string) site {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(source), &node); err != nil {
		t.Fatal(err)
	}
	return site{n: unwrap(&node), file: file, ptr: "#"}
}

func TestAPIOptionsRejectInvalidContracts(t *testing.T) {
	for _, tc := range []struct{ id, source string }{
		{"portos.operation-vocabulary", "unknown: true"},
		{"portos.operation-vocabulary", "operations: false"},
		{"portos.operation-vocabulary", "operations: []"},
		{"openapi.operation-success-response", "classes: []"},
		{"openapi.operation-success-response", "classes: [1]"},
		{"openapi.pagination", "{}"},
		{"openapi.pagination", "collection-operations: []"},
	} {
		var n yaml.Node
		if err := yaml.Unmarshal([]byte(tc.source), &n); err != nil {
			t.Fatal(err)
		}
		if _, err := apiFactory(tc.id)(*n.Content[0]); err == nil {
			t.Fatalf("accepted %s: %s", tc.id, tc.source)
		}
	}
	analyzer, err := apiFactory("openapi.pagination")(*contractSite(t, "collection-operations: [BrowseBooks]", "").n)
	if err != nil || analyzer.ID() != "openapi.pagination" {
		t.Fatalf("valid options: %v", err)
	}
}

func TestReferenceGraphOfflineContracts(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	for _, tc := range []struct{ name, source string }{
		{"remote", "$ref: https://example.com/schema.yaml"},
		{"malformed-url", "$ref: '%z'"},
		{"absolute", "$ref: /outside.yaml"},
		{"escape", "$ref: ../outside.yaml"},
		{"anchor", "$ref: '#SomeAnchor'"},
		{"missing-pointer", "$ref: '#/missing'"},
		{"missing-file", "$ref: missing.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := contractSite(t, tc.source, file)
			g := graph{root: root, files: map[string]*yaml.Node{file: s.n}}
			g.resolve(s)
			if g.err == nil {
				t.Fatal("invalid reference accepted")
			}
		})
	}
	badFile := filepath.Join(root, "bad.yaml")
	if err := os.WriteFile(badFile, []byte("[invalid:"), 0600); err != nil {
		t.Fatal(err)
	}
	s := contractSite(t, "$ref: bad.yaml", file)
	g := graph{root: root, files: map[string]*yaml.Node{file: s.n}}
	g.resolve(s)
	if g.err == nil {
		t.Fatal("invalid external YAML accepted")
	}
	// Cyclic references terminate; escaped pointer segments resolve exactly.
	s = contractSite(t, "components:\n  schemas:\n    A: {$ref: '#/components/schemas/B'}\n    B: {$ref: '#/components/schemas/A'}\n    'a/b~c': {type: string}\nref: {$ref: '#/components/schemas/a~1b~0c'}\n", file)
	g = graph{root: root, files: map[string]*yaml.Node{file: s.n}}
	if got := g.resolve(s.child("components").child("schemas").child("A")); got.n == nil || g.err != nil {
		t.Fatalf("cycle: %v", g.err)
	}
	if got := g.resolve(s.child("ref")).child("type").value(); got != "string" {
		t.Fatalf("escaped pointer: %s", got)
	}
	// Resolve the same external source once and retain bounded cache/source data.
	external := filepath.Join(root, "value.yaml")
	if err := os.WriteFile(external, []byte("type: object\nproperties:\n  name: {type: string}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s = contractSite(t, "one: {$ref: value.yaml}\ntwo: {$ref: value.yaml}\n", file)
	g = graph{root: root, files: map[string]*yaml.Node{file: s.n}}
	first := g.resolve(s.child("one"))
	second := g.resolve(s.child("two"))
	if first.n != second.n || len(g.files) != 2 || g.sources[external].ptr != "#/one" {
		t.Fatal("external graph cache or attribution unstable")
	}
	// allOf inherits properties and canonical identity without recursion loops.
	s = contractSite(t, "components:\n  schemas:\n    NameValue: {type: object, properties: {value: {type: string}}}\nwrapped:\n  allOf:\n    - {$ref: '#/components/schemas/NameValue'}\n    - {type: object, properties: {locale: {type: string}}}\n", file)
	g = graph{root: root, files: map[string]*yaml.Node{file: s.n}}
	if !g.named(s.child("wrapped"), "NameValue") || len(g.properties(s.child("wrapped"))) != 2 || g.named(s.child("wrapped"), "Wrong") {
		t.Fatal("allOf contract not honored")
	}
	alias := contractSite(t, "value: &x {type: string}\ncopy: *x\n", file)
	if got := unwrap(alias.child("copy").n); got != alias.child("value").n {
		t.Fatal("YAML alias not unwrapped")
	}
}

func TestEnumValueKinds(t *testing.T) {
	for _, tc := range []struct {
		schema, value string
		want          bool
	}{
		{"{}", "'value'", true},
		{"{type: string, nullable: true}", "null", true},
		{"type: [string, 'null']", "null", true},
		{"type: boolean", "true", true},
		{"type: boolean", "'true'", false},
		{"type: number", "1.5", true},
		{"type: number", "2", true},
		{"type: number", "'2'", false},
		{"type: integer", "2", true},
		{"type: integer", "2.0", true},
		{"type: integer", "2.5", false},
		{"type: object", "{key: value}", true},
		{"type: object", "[]", false},
		{"type: array", "[1, 2]", true},
		{"type: array", "{}", false},
		{"type: [string, integer]", "false", false},
	} {
		if got := enumTypeMatches(contractSite(t, tc.schema, ""), contractSite(t, tc.value, "")); got != tc.want {
			t.Fatalf("%s / %s: %v", tc.schema, tc.value, got)
		}
	}
}

func TestAPISecurityTagsAndErrorResponses(t *testing.T) {
	op := googleOperation("GetBook", googleResource())
	op["security"] = []any{googleMap{"OAuth": []any{"read", "missing"}, "Absent": []any{}}}
	op["tags"] = []any{"books", "absent"}
	root := googleMap{"openapi": "3.0.3", "info": googleMap{"title": "Books", "version": "1"}, "paths": googleMap{"/books/{name}": googleMap{"get": op}}, "tags": []any{googleMap{"name": "books"}, googleMap{"name": "books"}}, "components": googleMap{"securitySchemes": googleMap{"OAuth": googleMap{"type": "oauth2", "flows": googleMap{"clientCredentials": googleMap{"tokenUrl": "https://example.com/token", "scopes": googleMap{"read": "Read books"}}}}}}}
	if got := checkAPI(t, "openapi.security-references", contractSource(t, root), ""); len(got) != 2 {
		t.Fatalf("scopes/schemes: %#v", got)
	}
	if got := checkAPI(t, "openapi.tags-defined", contractSource(t, root), ""); len(got) != 2 {
		t.Fatalf("undefined/duplicate tags: %#v", got)
	}
	if got := checkAPI(t, "openapi.error-response", contractSource(t, root), "error-schema: Error"); len(got) != 1 {
		t.Fatalf("named error: %#v", got)
	}
	op["responses"] = googleMap{"200": googleResponse(googleResource())}
	if got := checkAPI(t, "openapi.error-response", contractSource(t, root), ""); len(got) != 1 {
		t.Fatal(got)
	}
	op["responses"] = googleMap{"default": googleResponse(googleMap{"$ref": "#/components/schemas/Error"})}
	root["components"].(googleMap)["schemas"] = googleMap{"Error": googleResource()}
	if got := checkAPI(t, "openapi.error-response", contractSource(t, root), "error-schema: Error"); len(got) != 0 {
		t.Fatal(got)
	}
	op["security"] = []any{googleMap{}}
	if got := checkAPI(t, "openapi.auth-required", contractSource(t, root), "public-operations: [GetBook]"); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestAPIPathAndServerContracts(t *testing.T) {
	op := googleOperation("GetBook", googleResource())
	op["parameters"] = []any{googleMap{"name": "name", "in": "path", "required": true, "schema": googleType("string")}, googleMap{"name": "name", "in": "path", "required": true, "schema": googleType("string")}}
	if got := checkAPI(t, "openapi.parameters-unique", googleSource(t, "/books/{name}", "get", op, nil), ""); len(got) != 1 {
		t.Fatal(got)
	}
	if got := checkAPI(t, "openapi.path-parameters", googleSource(t, "/books/{}", "get", op, nil), ""); len(got) < 2 {
		t.Fatal(got)
	}
	root := googleMap{"openapi": "3.0.3", "paths": googleMap{"/books/{id}": googleMap{"get": op, "post": op}, "/books/{name}": googleMap{"get": op}, "/books?limit=1": googleMap{"get": op}}}
	if got := checkAPI(t, "openapi.path-syntax", contractSource(t, root), ""); len(got) != 2 {
		t.Fatal(got)
	}
	op["responses"] = googleMap{"400": googleResponse(googleResource())}
	if got := checkAPI(t, "openapi.operation-success-response", googleSource(t, "/books", "get", op, nil), ""); len(got) != 1 {
		t.Fatal(got)
	}
	root["servers"] = []any{googleMap{"url": "https://{region}.example.com/{version}", "variables": googleMap{"region": googleMap{"default": "us", "enum": []any{"eu"}}}}}
	if got := checkAPI(t, "openapi.server-variables", contractSource(t, root), ""); len(got) != 2 {
		t.Fatal(got)
	}
	root["servers"] = []any{googleMap{"url": "https://other.example.com"}, googleMap{"url": "http://api.example.com"}, googleMap{"url": "%zz"}}
	if got := checkAPI(t, "openapi.server-policy", contractSource(t, root), "hosts: [api.example.com]"); len(got) != 3 {
		t.Fatal(got)
	}
}

func TestPortosShapeFailuresAndPaginationPairs(t *testing.T) {
	op := googleOperation("QueryBooks", googleResource())
	op["requestBody"] = googleMap{"content": googleMap{"application/json": googleMap{"schema": googleType("string")}}}
	if got := checkAPI(t, "portos.query-request", googleSource(t, "/QueryBooks", "get", op, nil), ""); len(got) != 2 {
		t.Fatal(got)
	}
	if got := checkAPI(t, "portos.query-graph", googleSource(t, "/QueryBooks", "post", op, nil), ""); len(got) < 8 {
		t.Fatal(got)
	}
	if got := checkAPI(t, "portos.pagination-request", googleSource(t, "/QueryBooks", "post", op, nil), ""); len(got) != 1 {
		t.Fatal(got)
	}
	pc := googleObject(googleMap{"nextToken": googleType("string"), "maxResults": googleType("integer")})
	op["requestBody"] = googleMap{"content": googleMap{"application/json": googleMap{"schema": googleObject(googleMap{"paginationContext": pc})}}}
	if got := checkAPI(t, "portos.pagination-request", googleSource(t, "/QueryBooks", "post", op, nil), ""); len(got) != 0 {
		t.Fatal(got)
	}
	op["operationId"] = "ListBooks"
	delete(op, "requestBody")
	for _, id := range []string{"portos.pagination-request", "openapi.pagination"} {
		options := ""
		if id == "openapi.pagination" {
			options = "collection-operations: [ListBooks]"
		}
		if got := checkAPI(t, id, googleSource(t, "/ListBooks", "get", op, nil), options); len(got) != 1 {
			t.Fatal(got)
		}
		op["parameters"] = []any{googleParam("nextToken", "string"), googleParam("maxResults", "integer"), googleMap{"name": "unused", "in": "header", "schema": googleType("string")}}
		if got := checkAPI(t, id, googleSource(t, "/ListBooks", "get", op, nil), options); len(got) != 0 {
			t.Fatal(got)
		}
		delete(op, "parameters")
	}
	if got := checkAPI(t, "portos.pagination-response", googleSource(t, "/ListBooks", "get", op, nil), ""); len(got) != 1 {
		t.Fatal(got)
	}
	op["operationId"] = "BatchSendBooks"
	op["requestBody"] = googleMap{"content": googleMap{"application/json": googleMap{"schema": googleObject(googleMap{})}}}
	if got := checkAPI(t, "portos.batch-contract", googleSource(t, "/BatchSendBooks", "post", op, nil), ""); len(got) != 2 {
		t.Fatal(got)
	}
	items := googleMap{"type": "array", "items": googleObject(googleMap{"id": googleType("string")})}
	op["requestBody"] = googleMap{"content": googleMap{"application/json": googleMap{"schema": googleObject(googleMap{"items": items})}}}
	op["responses"] = googleMap{"200": googleResponse(googleObject(googleMap{"results": items}))}
	if got := checkAPI(t, "portos.batch-contract", googleSource(t, "/BatchSendBooks", "post", op, nil), ""); len(got) != 0 {
		t.Fatal(got)
	}
	op["operationId"] = "BatchAsyncSendBooks"
	if got := checkAPI(t, "portos.batch-contract", googleSource(t, "/BatchAsyncSendBooks", "post", op, nil), ""); len(got) != 0 {
		t.Fatal(got)
	}
	if got := checkAPI(t, "portos.async-response", googleSource(t, "/BatchAsyncSendBooks", "post", op, nil), ""); len(got) != 2 {
		t.Fatal(got)
	}
	if got := checkAPI(t, "portos.operation-vocabulary", googleSource(t, "/BatchAsyncSendBooks", "post", op, nil), "modifiers: []"); len(got) != 2 {
		t.Fatal(got)
	}
	op["operationId"] = "GetListBooks"
	if got := checkAPI(t, "portos.operation-vocabulary", googleSource(t, "/GetListBooks", "get", op, nil), ""); len(got) == 0 {
		t.Fatal("ambiguous base accepted")
	}
	if got := checkAPI(t, "portos.path-description", googleSource(t, "/ListBooks", "get", op, nil), ""); len(got) != 1 {
		t.Fatal(got)
	}
}

func TestAPISchemaPolicyEdgeCases(t *testing.T) {
	root := googleMap{"openapi": "3.0.3", "paths": googleMap{}, "components": googleMap{"schemas": googleMap{"State": googleMap{"type": "string", "enum": []any{}, "x-enum-varnames": []any{"bad-name", "bad-name"}}, "Other": googleMap{"type": "string", "x-extensible-enum": []any{"one", "two"}, "x-enum-varnames": []any{"Same", "Same"}}}}}
	if got := checkAPI(t, "portos.open-enums", contractSource(t, root), ""); len(got) < 4 {
		t.Fatal(got)
	}
	root["components"].(googleMap)["schemas"] = googleMap{"DescriptionValue": googleObject(googleMap{"value": googleType("string")}), "Book": googleObject(googleMap{"description": googleMap{"$ref": "#/components/schemas/DescriptionValue"}})}
	if got := checkAPI(t, "portos.description-schema", contractSource(t, root), ""); len(got) != 0 {
		t.Fatal(got)
	}
	root["components"].(googleMap)["schemas"].(googleMap)["Book"] = googleObject(googleMap{"description": googleType("string")})
	if got := checkAPI(t, "portos.description-schema", contractSource(t, root), ""); len(got) != 1 {
		t.Fatal(got)
	}
	root["components"].(googleMap)["schemas"] = googleMap{"Book": googleResource(), "Alias": googleMap{"$ref": "#/components/schemas/Book", "description": "ignored sibling"}}
	if got := checkAPI(t, "openapi.ref-siblings", contractSource(t, root), ""); len(got) != 1 {
		t.Fatal(got)
	}
	root["openapi"] = "3.1.0"
	if got := checkAPI(t, "openapi.ref-siblings", contractSource(t, root), ""); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestUnusedSecuritySchemesAndAPIExecutionErrors(t *testing.T) {
	op := googleOperation("GetBook", googleResource())
	op["security"] = []any{googleMap{"Local": []any{}}}
	root := googleMap{"openapi": "3.0.3", "paths": googleMap{"/books": googleMap{"get": op}}, "security": []any{googleMap{"Global": []any{}}}, "components": googleMap{"securitySchemes": googleMap{"Local": googleMap{"type": "http", "scheme": "bearer"}, "Global": googleMap{"type": "http", "scheme": "bearer"}, "Unused": googleMap{"type": "http", "scheme": "bearer"}}}}
	if got := checkAPI(t, "openapi.unused-components", contractSource(t, root), ""); len(got) != 2 {
		t.Fatal(got)
	}
	delete(op, "security")
	if got := checkAPI(t, "openapi.unused-components", contractSource(t, root), ""); len(got) != 2 {
		t.Fatal(got)
	}
	check := apiCheck{id: "openapi.enum-values", options: defaultOptions()}
	if check.ID() != "openapi.enum-values" {
		t.Fatal(check.ID())
	}
	pass := linter.NewPass([]*linter.Document{{}, {Path: "api.yaml", Model: &v3.Document{}}})
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("source required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{{Model: &v3.Document{}}})
	check.Analyze(ctx, pass)
	if pass.Err() != nil {
		t.Fatal(pass.Err())
	}
	taskRoot := t.TempDir()
	file := filepath.Join(taskRoot, "api.yaml")
	s := contractSite(t, "openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n    Bad: {$ref: https://example.com/schema.yaml}\n", file)
	pass = linter.NewPass([]*linter.Document{{Path: file, Root: s.n, Model: &v3.Document{}}})
	pass.Root = taskRoot
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil || !strings.Contains(pass.Err().Error(), "unsupported reference") {
		t.Fatal(pass.Err())
	}
}

func TestOperationCallbackRecursionAndPolicyApplicability(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	s := contractSite(t, "paths:\n  /books:\n    get:\n      operationId: ListBooks\n      callbacks:\n        same:\n          '{$request.body#/url}': {$ref: '#/paths/~1books'}\n", file)
	g := graph{root: root, files: map[string]*yaml.Node{file: s.n}}
	if ops := g.operations(s); len(ops) != 1 || g.err != nil {
		t.Fatalf("recursive callback: %#v, %v", ops, g.err)
	}
	// Callback expressions are not request paths; Portos policy must skip them.
	source := "openapi: 3.0.3\npaths:\n  /books:\n    post:\n      operationId: SendBooks\n      callbacks:\n        done:\n          '{$request.body#/url}':\n            post:\n              operationId: UnsupportedAsyncQuery\n              responses: {}\n"
	for _, id := range []string{"portos.operation-vocabulary", "openapi.path-syntax"} {
		if findings := checkAPI(t, id, source, ""); len(findings) != 0 {
			t.Fatal(findings)
		}
	}
	if findings := checkAPI(t, "portos.collection-response", googleSource(t, "/ListBooks", "get", googleMap{"operationId": "ListBooks", "responses": googleMap{"204": googleMap{"description": "No content"}}}, nil), ""); len(findings) != 1 {
		t.Fatal(findings)
	}
	// Reference resolution failures and YAML values incompatible with JSON are
	// operational errors, rather than suppressible lint findings.
	for _, tc := range []struct{ id, source string }{
		{"openapi.operation-id-unique", "openapi: 3.0.3\npaths:\n  /books: {$ref: '#/missing'}\n"},
		{"openapi.spec-structure", "openapi: 3.0.3\npaths: {}\nx-invalid: .nan\n"},
	} {
		s = contractSite(t, tc.source, file)
		pass := linter.NewPass([]*linter.Document{{Path: file, Source: []byte(tc.source), Root: s.n, Model: &v3.Document{}}})
		pass.Root = root
		apiCheck{id: tc.id, options: defaultOptions()}.Analyze(context.Background(), pass)
		if pass.Err() == nil {
			t.Fatalf("%s accepted operational failure", tc.id)
		}
	}
	// A nested external reference's finding points to the importing root site.
	external := filepath.Join(root, "external.yaml")
	if err := os.WriteFile(external, []byte("type: string\nenum: [one, one]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source = "openapi: 3.0.3\npaths: {}\ncomponents:\n  schemas:\n    State: {$ref: external.yaml}\n"
	s = contractSite(t, source, file)
	pass := linter.NewPass([]*linter.Document{{Path: file, Source: []byte(source), Root: s.n, Model: &v3.Document{}}})
	pass.Root = root
	apiCheck{id: "openapi.enum-values", options: defaultOptions()}.Analyze(context.Background(), pass)
	if pass.Err() != nil {
		t.Fatal(pass.Err())
	}
	if got := pass.Diagnostics(); len(got) != 1 || got[0].Pointer != "#/components/schemas/State" {
		t.Fatalf("external location: %#v", got)
	}
}
