package rules

import (
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	linter "github.com/portpowered/openapi-linter"
	"go.yaml.in/yaml/v4"
)

func TestVisitorAbsentModels(t *testing.T) {
	for _, rule := range []linter.Rule{&OperationIDNamingConvention{}, &RequestResponseRequiresExample{}, &SchemaRequiresDescription{}, &SchemaPropertyCamelCase{}} {
		if got := rule.VisitOperation("/books", "GET", nil); len(got) != 0 {
			t.Fatalf("%s absent operation: %#v", rule.Name(), got)
		}
		if got := rule.VisitSchema("Book", nil); len(got) != 0 {
			t.Fatalf("%s absent schema: %#v", rule.Name(), got)
		}
	}
	for _, rule := range []linter.SchemaRule{&NoAnonymousObjects{}, &PropertyCamelCase{}} {
		if got := rule.VisitSchema("book.yaml", nil); len(got) != 0 {
			t.Fatalf("%s absent schema: %#v", rule.Name(), got)
		}
		for _, props := range []any{nil, []any{}, "wrong shape"} {
			if got := rule.VisitSchema("book.yaml", &linter.SchemaDocument{Content: map[string]any{"properties": props}}); len(got) != 0 {
				t.Fatalf("%s nonmapping properties: %#v", rule.Name(), got)
			}
		}
	}
}

func TestStandalonePropertyNestedArrayContracts(t *testing.T) {
	doc := &linter.SchemaDocument{Content: map[string]any{"properties": map[string]any{
		"legacyValue":        false,
		"x-custom-name":      map[string]any{"properties": map[string]any{"ignored_name": map[string]any{}}},
		"items":              map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"snake_name": map[string]any{"type": "string"}}}},
		"bareMap":            map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
		"malformedNested":    map[string]any{"type": "object", "properties": false},
		"malformedItems":     map[string]any{"type": "array", "items": false},
		"malformedItemProps": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": false}},
		"referenceItems":     map[string]any{"type": "array", "items": map[string]any{"$ref": "NameValue.yaml"}},
	}}}
	got := (&PropertyCamelCase{}).VisitSchema("book.yaml", doc)
	if len(got) != 1 || !strings.Contains(got[0].Message, "properties.items.items.properties.snake_name") {
		t.Fatalf("nested array field: %#v", got)
	}
	got = (&NoAnonymousObjects{}).VisitSchema("book.yaml", doc)
	if len(got) != 1 || !strings.Contains(got[0].Message, "properties.items.items") {
		t.Fatalf("nested array object: %#v", got)
	}
	if got = (&PathNamingConvention{}).VisitPath("/", nil); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestSchemaPropertyStandardsExceptions(t *testing.T) {
	props := orderedmap.New[string, *base.SchemaProxy]()
	for _, name := range []string{"x-custom", "client_id", "$schema", "x", "", "camelCase", "Bad_Name"} {
		props.Set(name, nil)
	}
	got := (&SchemaPropertyCamelCase{}).VisitSchema("Book", &base.Schema{Properties: props})
	if len(got) != 1 || !strings.Contains(got[0].Path, "Bad_Name") {
		t.Fatalf("standard exceptions: %#v", got)
	}
	if got = (&SchemaPropertyCamelCase{}).VisitSchema("Book", &base.Schema{}); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestDefaultResponsesAndAbsentMediaExampleContracts(t *testing.T) {
	rule := &RequestResponseRequiresExample{}
	media := orderedmap.New[string, *v3.MediaType]()
	media.Set("application/json", &v3.MediaType{})
	media.Set("application/problem+json", nil)
	codes := orderedmap.New[string, *v3.Response]()
	codes.Set("204", &v3.Response{Description: "No content"})
	codes.Set("404", nil)
	op := &v3.Operation{Responses: &v3.Responses{Codes: codes, Default: &v3.Response{Content: media}}}
	got := rule.VisitOperation("/books", "GET", op)
	if len(got) != 2 {
		t.Fatalf("default response missing examples: %#v", got)
	}
	for _, finding := range got {
		if !strings.Contains(finding.Message, "response default") || finding.Path != "GET /books" {
			t.Fatal(finding)
		}
	}
	examples := orderedmap.New[string, *base.Example]()
	examples.Set("book", &base.Example{Value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "A book"}})
	media.Set("application/json", &v3.MediaType{Examples: examples})
	media.Set("application/problem+json", &v3.MediaType{Example: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}})
	if got = rule.VisitOperation("/books", "GET", op); len(got) != 0 {
		t.Fatalf("explicit examples should pass: %#v", got)
	}
	if mediaTypeHasExample(nil) {
		t.Fatal("absent media model has no example")
	}
}
