package rules

import (
	"testing"

	"github.com/portpowered/openapi-linter"
)

func TestPropertyCamelCase_CamelCasePasses(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"properties": map[string]any{
				"myProperty":   map[string]any{"type": "string"},
				"anotherField": map[string]any{"type": "integer"},
				"simpleValue":  map[string]any{"type": "boolean"},
				"a":            map[string]any{"type": "string"},
			},
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for camelCase properties, got %d: %v", len(violations), violations)
	}
}

func TestPropertyCamelCase_KebabCaseFails(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"properties": map[string]any{
				"my-property": map[string]any{"type": "string"},
			},
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for kebab-case, got %d", len(violations))
	}
	if violations[0].RuleName != "ci-property-camelcase" {
		t.Errorf("expected rule name ci-property-camelcase, got %s", violations[0].RuleName)
	}
}

func TestPropertyCamelCase_SnakeCaseFails(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"properties": map[string]any{
				"my_property": map[string]any{"type": "string"},
			},
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for snake_case, got %d", len(violations))
	}
}

func TestPropertyCamelCase_XPrefixSkipped(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"properties": map[string]any{
				"x-custom-field": map[string]any{"type": "string"},
				"x-another":      map[string]any{"type": "string"},
			},
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for x- prefixed properties, got %d", len(violations))
	}
}

func TestPropertyCamelCase_PascalCaseFails(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"properties": map[string]any{
				"MyProperty": map[string]any{"type": "string"},
			},
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for PascalCase, got %d", len(violations))
	}
}

func TestPropertyCamelCase_NoProperties(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"type": "object",
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for schema without properties, got %d", len(violations))
	}
}

func TestPropertyCamelCase_NestedProperties(t *testing.T) {
	rule := &PropertyCamelCase{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"properties": map[string]any{
				"validName": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"nested_bad": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for nested snake_case, got %d: %v", len(violations), violations)
	}
}
