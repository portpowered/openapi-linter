package rules

import (
	linter "github.com/portpowered/openapi-linter"
	"testing"
)

func TestStandaloneSchemaRulesHandleAbsentMalformedAndNestedProperties(t *testing.T) {
	camel := &PropertyCamelCase{}
	anonymous := &NoAnonymousObjects{}
	for _, content := range []map[string]any{{}, {"properties": "invalid"}, {"properties": map[string]any{"x-custom": "ignored", "scalar": "invalid"}}} {
		doc := &linter.SchemaDocument{Content: content}
		if got := camel.VisitSchema("schema.yaml", doc); len(got) != 0 {
			t.Fatal(got)
		}
		if got := anonymous.VisitSchema("schema.yaml", doc); len(got) != 0 {
			t.Fatal(got)
		}
	}
	nested := &linter.SchemaDocument{Content: map[string]any{"properties": map[string]any{"items": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"snake_case": map[string]any{"type": "string"}}}}, "object": map[string]any{"type": "object", "properties": map[string]any{"nested_name": map[string]any{"type": "string"}}}}}}
	if got := camel.VisitSchema("schema.yaml", nested); len(got) != 2 {
		t.Fatalf("camel %+v", got)
	}
	if got := anonymous.VisitSchema("schema.yaml", nested); len(got) != 2 {
		t.Fatalf("anonymous %+v", got)
	}
	// References and unstructured maps remain accepted, including malformed schema keywords deferred to the structural validator.
	accepted := &linter.SchemaDocument{Content: map[string]any{"properties": map[string]any{"ref": map[string]any{"$ref": "other.yaml"}, "map": map[string]any{"type": "object"}, "list": map[string]any{"type": "array", "items": map[string]any{"$ref": "other.yaml"}}, "broken": map[string]any{"type": "array", "items": "wrong"}}}}
	if got := anonymous.VisitSchema("schema.yaml", accepted); len(got) != 0 {
		t.Fatal(got)
	}
}
