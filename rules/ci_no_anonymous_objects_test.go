package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"testing"
)

func TestNoAnonymousObjects_InlineObjectFlagged(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"value": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"uri": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].RuleName != "ci-no-anonymous-objects" {
		t.Errorf("unexpected rule name: %s", violations[0].RuleName)
	}
	if violations[0].Path != "test.yaml" {
		t.Errorf("unexpected path: %s", violations[0].Path)
	}
}

func TestNoAnonymousObjects_RefObjectPasses(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"value": map[string]any{
					"$ref": "../../../common-schemas/SnapshotValue.yaml",
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestNoAnonymousObjects_NestedInlineObjectFlagged(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"outer": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"inner": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"field": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations (outer + inner), got %d: %v", len(violations), violations)
	}
}

func TestNoAnonymousObjects_NonObjectTypesPassed(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
				"count": map[string]any{
					"type": "integer",
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestNoAnonymousObjects_ArrayItemsInlineObjectFlagged(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for array items inline object, got %d: %v", len(violations), violations)
	}
}

func TestNoAnonymousObjects_ArrayItemsRefPasses(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"$ref": "../../../common-schemas/Item.yaml",
					},
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestNoAnonymousObjects_BareObjectMapPasses(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type": "message",
			"type":   "object",
			"properties": map[string]any{
				"metadata": map[string]any{
					"type":                 "object",
					"description":          "Additional metadata",
					"additionalProperties": true,
				},
			},
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for bare type: object map, got %d: %v", len(violations), violations)
	}
}

func TestNoAnonymousObjects_NoProperties(t *testing.T) {
	rule := &rules.NoAnonymousObjects{}
	schema := &linter.SchemaDocument{
		Content: map[string]any{
			"x-type":      "message",
			"type":        "object",
			"description": "empty schema",
		},
	}

	violations := rule.VisitSchema("test.yaml", schema)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for schema without properties, got %d: %v", len(violations), violations)
	}
}
