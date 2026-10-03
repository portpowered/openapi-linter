package linter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeContractFile(t *testing.T, root, name, source string) string {
	t.Helper()
	file := filepath.Join(root, name)
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestStandaloneSchemaLoadingAndDiscovery(t *testing.T) {
	root := t.TempDir()
	file := writeContractFile(t, root, "widget.YAML", "type: object\nx-namespace: example\nproperties: {name: {type: string}}\n")
	writeContractFile(t, root, "other.yml", "type: string\n")
	writeContractFile(t, root, "ignored.txt", "ignored\n")
	files, err := DiscoverSchemas(root)
	if err != nil || len(files) != 2 {
		t.Fatalf("discovery: %#v %v", files, err)
	}
	schema, err := LoadSchema(file)
	if err != nil || schema.Content["x-namespace"] != "example" {
		t.Fatalf("extensions: %#v %v", schema, err)
	}
	doc, err := LoadSchemaDocument(context.Background(), file, root)
	if err != nil || doc.Schema.Content["type"] != "object" || doc.Model != nil || doc.Line("#/properties/name") != 3 {
		t.Fatalf("loaded schema: %#v %v", doc, err)
	}
	if _, err := DiscoverSchemas(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
	if _, err := LoadSchema(filepath.Join(root, "missing.yaml")); err == nil {
		t.Fatal("missing file accepted")
	}
	for _, source := range []string{"[", "[]", "null", "type: object\n---\ntype: string\n", "type: object\n$ref: 123\n", "$ref: 'http://example.invalid/schema'\n", "$ref: 'item.yaml?query'\n", "$ref: '%xx'\n", "$ref: 'missing.yaml'\n"} {
		bad := writeContractFile(t, root, "bad.yaml", source)
		if _, err := LoadSchemaDocument(context.Background(), bad, root); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
	bad := writeContractFile(t, root, "bad.yaml", "[")
	if _, err := LoadSchema(bad); err == nil {
		t.Fatal("invalid YAML accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadSchemaDocument(ctx, file, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := LoadDocument(ctx, file, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("OpenAPI cancellation: %v", err)
	}
	if _, err := LoadDocument(context.Background(), file, root); err == nil {
		t.Fatal("standalone schema accepted as OpenAPI")
	}
	// Cyclic local references are visited once during source validation.
	a := writeContractFile(t, root, "a.yaml", "$ref: b.yaml\n")
	writeContractFile(t, root, "b.yaml", "$ref: a.yaml\n")
	if _, _, err := validateReferences(context.Background(), a, root, map[string]bool{}); err != nil {
		t.Fatalf("bounded cycle: %v", err)
	}
}

func TestDocumentPointerAndPassIsolation(t *testing.T) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("items:\n  - name: first\n'~/': value\n"), &node); err != nil {
		t.Fatal(err)
	}
	doc := Document{Root: &node}
	for _, c := range []struct {
		pointer string
		line    int
	}{{"#", 1}, {"#/items/0/name", 2}, {"#/~0~1", 3}, {"#/items/-1", 0}, {"#/items/no", 0}, {"#/items/9", 0}, {"#/missing", 0}, {"#/items/0/name/child", 0}, {"invalid", 0}} {
		if got := doc.Line(c.pointer); got != c.line {
			t.Fatalf("%s: %d want %d", c.pointer, got, c.line)
		}
	}
	if (&Document{}).Line("#") != 0 {
		t.Fatal("missing AST location")
	}
	pass := NewPass(nil)
	pass.SetData("cache", 123)
	if got, ok := pass.Data("cache"); !ok || got != 123 {
		t.Fatal("pass data missing")
	}
	if _, ok := pass.Data("absent"); ok {
		t.Fatal("unexpected pass data")
	}
	pass.Report(Diagnostic{Message: "original"})
	copy := pass.Diagnostics()
	copy[0].Message = "changed"
	if pass.Diagnostics()[0].Message != "original" {
		t.Fatal("diagnostic slice alias")
	}
	pass.ReportError(nil)
	if pass.Err() != nil {
		t.Fatal("nil reported error")
	}
	var loader Linter
	if loader.Document() != nil {
		t.Fatal("unloaded model populated")
	}
}

func TestRootChecksMissingAncestors(t *testing.T) {
	root := t.TempDir()
	if err := CheckPathRoot("", filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	if err := CheckPathRoot(root, filepath.Join(root, "new", "nested", "file.yaml")); err != nil {
		t.Fatalf("missing inside path: %v", err)
	}
	if err := CheckPathRoot(root, filepath.Join(filepath.Dir(root), "outside", "file.yaml")); err == nil {
		t.Fatal("missing outside path accepted")
	}
	if err := CheckPathRoot(filepath.Join(root, "missing"), root); err == nil {
		t.Fatal("missing root accepted")
	}
	if err := CheckPathRoot(root, string([]byte{0})); err == nil {
		t.Fatal("invalid target accepted")
	}
}
