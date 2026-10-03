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

type googleMap = map[string]any

func googleType(typ string) googleMap { return googleMap{"type": typ} }
func googleObject(props googleMap) googleMap {
	return googleMap{"type": "object", "properties": props}
}
func googleResponse(s googleMap) googleMap {
	return googleMap{"description": "Response", "content": googleMap{"application/json": googleMap{"schema": s}}}
}
func googleParam(name, typ string) googleMap {
	return googleMap{"name": name, "in": "query", "schema": googleType(typ)}
}
func googleResource() googleMap {
	return googleObject(googleMap{"name": googleType("string"), "createTime": googleMap{"type": "string", "format": "date-time"}, "disabled": googleType("boolean")})
}
func googleList() googleMap {
	return googleObject(googleMap{"books": googleMap{"type": "array", "items": googleResource()}, "nextPageToken": googleType("string")})
}
func googleOperation(id string, response googleMap) googleMap {
	return googleMap{"operationId": id, "responses": googleMap{"200": googleResponse(response), "400": googleResponse(googleObject(googleMap{"error": googleObject(googleMap{"code": googleType("integer"), "message": googleType("string")})}))}}
}
func googleSource(t *testing.T, path, method string, op googleMap, components googleMap) string {
	t.Helper()
	root := googleMap{"openapi": "3.0.3", "info": googleMap{"title": "Books", "version": "1"}, "paths": googleMap{path: googleMap{method: op}}}
	if components != nil {
		root["components"] = googleMap{"schemas": components}
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGoogleChecksExpectedBehavior(t *testing.T) {
	tests := []struct {
		id     string
		valid  func() (string, string, googleMap, googleMap)
		mutate func(*string, *string, googleMap, googleMap)
	}{
		{"method-name", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) { op["operationId"] = "get_book" }},
		{"http-method", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(_ *string, m *string, _ googleMap, _ googleMap) { *m = "post" }},
		{"version-path", func() (string, string, googleMap, googleMap) {
			return "/v1beta/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(p *string, _ *string, _ googleMap, _ googleMap) { *p = "/books/{name}" }},
		{"standard-path", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(p *string, _ *string, _ googleMap, _ googleMap) { *p = "/v1/books" }},
		{"request-body", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["requestBody"] = googleMap{"content": googleMap{"application/json": googleMap{"schema": googleResource()}}}
		}},
		{"resource-response", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["responses"] = googleMap{"200": googleResponse(googleType("string"))}
		}},
		{"list-request", func() (string, string, googleMap, googleMap) {
			op := googleOperation("ListBooks", googleList())
			op["parameters"] = []any{googleParam("pageSize", "integer"), googleParam("pageToken", "string")}
			return "/v1/books", "get", op, nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["parameters"] = []any{googleParam("pageSize", "string")}
		}},
		{"list-response", func() (string, string, googleMap, googleMap) {
			return "/v1/books", "get", googleOperation("ListBooks", googleList()), nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["responses"] = googleMap{"200": googleResponse(googleObject(googleMap{"books": googleMap{"type": "array", "items": googleResource()}}))}
		}},
		{"update-mask", func() (string, string, googleMap, googleMap) {
			op := googleOperation("UpdateBook", googleResource())
			op["parameters"] = []any{googleParam("updateMask", "string")}
			return "/v1/books/{name}", "patch", op, nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			p := googleParam("updateMask", "string")
			p["required"] = true
			op["parameters"] = []any{p}
		}},
		{"custom-method", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}:archive", "post", googleOperation("ArchiveBook", googleResource()), nil
		}, func(p *string, _ *string, _ googleMap, _ googleMap) { *p = "/v1/books/{name}/archive" }},
		{"resource-name", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["responses"] = googleMap{"200": googleResponse(googleObject(googleMap{"name": googleType("integer")}))}
		}},
		{"field-names", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), googleMap{"Book": googleResource()}
		}, func(_ *string, _ *string, _ googleMap, comp googleMap) {
			comp["Book"] = googleObject(googleMap{"display_name": googleType("string")})
		}},
		{"boolean-names", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), googleMap{"Book": googleResource()}
		}, func(_ *string, _ *string, _ googleMap, comp googleMap) {
			comp["Book"] = googleObject(googleMap{"isDisabled": googleType("boolean")})
		}},
		{"timestamp", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), googleMap{"Book": googleResource()}
		}, func(_ *string, _ *string, _ googleMap, comp googleMap) {
			comp["Book"] = googleObject(googleMap{"createTime": googleType("integer")})
		}},
		{"error-response", func() (string, string, googleMap, googleMap) {
			return "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["responses"] = googleMap{"400": googleResponse(googleObject(googleMap{"message": googleType("string")}))}
		}},
		{"long-running-response", func() (string, string, googleMap, googleMap) {
			op := googleOperation("CreateBook", googleObject(googleMap{"name": googleType("string"), "done": googleType("boolean"), "error": googleObject(googleMap{}), "response": googleObject(googleMap{})}))
			op["x-google-long-running"] = true
			return "/v1/books", "post", op, nil
		}, func(_ *string, _ *string, op googleMap, _ googleMap) {
			op["responses"] = googleMap{"200": googleResponse(googleResource())}
		}},
	}
	if len(tests) != len(googleChecks) {
		t.Fatal("every Google rule requires its own valid and invalid fixture")
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			p, m, op, components := tc.valid()
			if got := checkAPI(t, "google."+tc.id, googleSource(t, p, m, op, components), ""); len(got) != 0 {
				t.Fatalf("valid: %#v", got)
			}
			tc.mutate(&p, &m, op, components)
			if got := checkAPI(t, "google."+tc.id, googleSource(t, p, m, op, components), ""); len(got) == 0 {
				t.Fatal("invalid fixture was accepted")
			}
		})
	}
}

func TestGoogleComposedPreset(t *testing.T) {
	r := NewRegistry()
	if err := RegisterStock(r); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"openapi:google", "google-defaults"} {
		pack, ok := Preset(name)
		if !ok {
			t.Fatal(name)
		}
		program, err := r.Compile(pack)
		if err != nil {
			t.Fatal(err)
		}
		op := googleOperation("ListBooks", googleList())
		op["parameters"] = []any{googleParam("pageSize", "integer"), googleParam("pageToken", "string")}
		source := googleSource(t, "/v1/books", "get", op, nil)
		root := t.TempDir()
		file := filepath.Join(root, "api.yaml")
		if err = os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		var node yaml.Node
		if err = yaml.Unmarshal([]byte(source), &node); err != nil {
			t.Fatal(err)
		}
		doc := &linter.Document{Path: file, Source: []byte(source), Root: &node, Model: &v3.Document{}}
		findings, err := program.Run(context.Background(), root, []*linter.Document{doc})
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range findings {
			if strings.HasPrefix(finding.RuleID, "google.") {
				t.Fatalf("%s: %#v", name, finding)
			}
		}
		if name == "google-defaults" && len(findings) == 0 {
			t.Fatal("recommended documentation checks should still run")
		}
		// A paired contract fails independently on both request and response sides.
		op["parameters"] = []any{}
		op["responses"] = googleMap{"200": googleResponse(googleObject(googleMap{}))}
		source = googleSource(t, "/v1/books", "get", op, nil)
		if err = yaml.Unmarshal([]byte(source), &node); err != nil {
			t.Fatal(err)
		}
		doc.Source = []byte(source)
		doc.Root = &node
		findings, err = program.Run(context.Background(), root, []*linter.Document{doc})
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, f := range findings {
			seen[f.RuleID] = true
		}
		if !seen["google.list-request"] || !seen["google.list-response"] {
			t.Fatalf("missing paired findings: %#v", findings)
		}
	}
}

func TestGoogleBodyAndResponseBoundaries(t *testing.T) {
	for _, id := range []string{"CreateBook", "UpdateBook"} {
		op := googleOperation(id, googleResource())
		op["requestBody"] = googleMap{"required": true, "content": googleMap{"application/json": googleMap{"schema": googleResource()}}}
		if got := checkAPI(t, "google.request-body", googleSource(t, "/v1/books", "post", op, nil), ""); len(got) != 0 {
			t.Fatal(got)
		}
		op["requestBody"] = googleMap{"content": googleMap{"application/json": googleMap{"schema": googleType("string")}}}
		if got := checkAPI(t, "google.request-body", googleSource(t, "/v1/books", "post", op, nil), ""); len(got) < 2 {
			t.Fatalf("missing required/body shape findings: %#v", got)
		}
		delete(op, "requestBody")
		if got := checkAPI(t, "google.request-body", googleSource(t, "/v1/books", "post", op, nil), ""); len(got) == 0 {
			t.Fatal("missing body accepted")
		}
	}
	for _, id := range []string{"resource-response", "resource-name", "list-response", "long-running-response"} {
		op := googleOperation("GetBook", googleResource())
		if id == "list-response" {
			op["operationId"] = "ListBooks"
		}
		if id == "long-running-response" {
			op["x-google-long-running"] = true
		}
		op["responses"] = googleMap{"204": googleMap{"description": "No content"}}
		if got := checkAPI(t, "google."+id, googleSource(t, "/v1/books", "get", op, nil), ""); len(got) == 0 {
			t.Fatal("missing response accepted")
		}
	}
	op := googleOperation("GetBook", googleResource())
	op["responses"] = googleMap{"200": googleResponse(googleResource())}
	if got := checkAPI(t, "google.error-response", googleSource(t, "/v1/books/{name}", "get", op, nil), ""); len(got) == 0 {
		t.Fatal("undocumented errors accepted")
	}
}

func TestGoogleApplicabilityAndSchemaTraversal(t *testing.T) {
	// Singleton resources need no terminal path variable. Custom methods may be
	// stateless; their full VerbNoun suffix is allowed. Neither is a List method.
	op := googleOperation("GetSettings", googleResource())
	op["x-google-singleton"] = true
	if got := checkAPI(t, "google.standard-path", googleSource(t, "/v1/settings", "get", op, nil), ""); len(got) != 0 {
		t.Fatal(got)
	}
	op = googleOperation("TranslateText", googleResource())
	if got := checkAPI(t, "google.custom-method", googleSource(t, "/v1/projects/{project}:translateText", "post", op, nil), ""); len(got) != 0 {
		t.Fatal(got)
	}
	for _, method := range []struct {
		id, path string
		bad      bool
	}{{"ListBooks", "/v1/books:list", true}, {"ArchiveBookForUser", "/v1/books/{name}:archive", true}, {"CreateBookLongRunning", "/v1/books:create", false}, {"", "/v1/books:archive", true}} {
		got := checkAPI(t, "google.custom-method", googleSource(t, method.path, "post", googleOperation(method.id, googleResource()), nil), "")
		if (len(got) > 0) != method.bad {
			t.Fatalf("%s: %#v", method.id, got)
		}
	}
	for _, id := range []string{"CreateSettings", "DeleteSettings"} {
		singleton := googleOperation(id, googleResource())
		singleton["x-google-singleton"] = true
		if got := checkAPI(t, "google.standard-path", googleSource(t, "/v1/settings", "post", singleton, nil), ""); len(got) == 0 {
			t.Fatal("singleton mutation accepted")
		}
	}
	for _, id := range []string{"list-request", "list-response", "update-mask", "long-running-response"} {
		if got := checkAPI(t, "google."+id, googleSource(t, "/v1/projects/{project}:translateText", "post", op, nil), ""); len(got) != 0 {
			t.Fatal(got)
		}
	}
	// Dictionary keys are not JSON field names. Boolean reserved-word exception
	// and non-timestamp duration fields are deliberately left alone.
	comp := googleMap{"Book": googleObject(googleMap{"@type": googleType("string"), "isNew": googleType("boolean"), "isLabel": googleType("string"), "runtime": googleType("integer"), "labels": googleMap{"type": "object", "additionalProperties": googleType("string")}, "nested": googleMap{"allOf": []any{googleObject(googleMap{"displayName": googleType("string")})}, "anyOf": []any{googleType("string")}, "oneOf": []any{googleType("integer")}}})}
	for _, id := range []string{"field-names", "boolean-names", "timestamp"} {
		if got := checkAPI(t, "google."+id, googleSource(t, "/v1/books/{name}", "get", googleOperation("GetBook", googleResource()), comp), ""); len(got) != 0 {
			t.Fatal(got)
		}
	}
	comp["Book"] = googleMap{"type": "object", "x-google-resource": true, "properties": googleMap{"name": googleType("integer")}}
	if got := checkAPI(t, "google.resource-name", googleSource(t, "/v1/books:archive", "post", googleOperation("ArchiveBook", googleResource()), comp), ""); len(got) == 0 {
		t.Fatal("marked resource must have string name")
	}
	// Runtime callback expressions and webhook paths are outside the request API.
	root := googleMap{"openapi": "3.1.0", "info": googleMap{"title": "Callbacks", "version": "1"}, "paths": googleMap{"/v1/books:archive": googleMap{"post": googleMap{"operationId": "ArchiveBook", "callbacks": googleMap{"done": googleMap{"{$request.body#/url}": googleMap{"post": googleMap{"operationId": "bad_name"}}}}}}}, "webhooks": googleMap{"unversioned": googleMap{"post": googleMap{"operationId": "bad_name"}}}}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"method-name", "version-path"} {
		if got := checkAPI(t, "google."+id, string(data), ""); len(got) != 0 {
			t.Fatal(got)
		}
	}
}

func TestGoogleFactoryAndOperationalFailures(t *testing.T) {
	for _, source := range []string{"unknown: true", "[wrong]"} {
		var n yaml.Node
		if err := yaml.Unmarshal([]byte(source), &n); err != nil {
			t.Fatal(err)
		}
		if _, err := googleFactory("google.method-name")(*n.Content[0]); err == nil {
			t.Fatal("accepted options")
		}
	}
	check, err := googleFactory("google.method-name")(yaml.Node{})
	if err != nil || check.ID() != "google.method-name" {
		t.Fatalf("check: %v, error: %v", check, err)
	}
	r := NewRegistry()
	if err = registerGoogle(r); err != nil {
		t.Fatal(err)
	}
	if err = registerGoogle(r); err == nil {
		t.Fatal("duplicate registration accepted")
	}
	pass := linter.NewPass([]*linter.Document{{Path: "ignored.yaml"}, {Path: "api.yaml", Model: &v3.Document{}}})
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("source root required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{{Model: &v3.Document{}}})
	check.Analyze(ctx, pass)
	if pass.Err() != nil {
		t.Fatal(pass.Err())
	}
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	source := "openapi: 3.0.3\ncomponents:\n  schemas:\n    Book:\n      $ref: missing.yaml\npaths: {}\n"
	var n yaml.Node
	if err = yaml.Unmarshal([]byte(source), &n); err != nil {
		t.Fatal(err)
	}
	pass = linter.NewPass([]*linter.Document{{Path: file, Source: []byte(source), Root: &n, Model: &v3.Document{}}})
	pass.Root = root
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("unresolved reference should fail")
	}
	// Local component references retain their actual source location.
	external := filepath.Join(root, "book.yaml")
	if err = os.WriteFile(external, []byte("type: object\nproperties:\n  bad_field: {type: string}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source = "openapi: 3.0.3\ncomponents:\n  schemas:\n    Book:\n      $ref: book.yaml\npaths: {}\n"
	if err = yaml.Unmarshal([]byte(source), &n); err != nil {
		t.Fatal(err)
	}
	pass = linter.NewPass([]*linter.Document{{Path: file, Source: []byte(source), Root: &n, Model: &v3.Document{}}})
	pass.Root = root
	googleCheck{"google.field-names"}.Analyze(context.Background(), pass)
	if pass.Err() != nil {
		t.Fatal(pass.Err())
	}
	if got := pass.Diagnostics(); len(got) != 1 || got[0].Pointer != "#/components/schemas/Book" || !strings.Contains(got[0].Message, "book.yaml") {
		t.Fatalf("wrong external attribution: %#v", got)
	}
}
