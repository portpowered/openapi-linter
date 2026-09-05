package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"testing"
)

func TestSchemaRequiresDescription_InvalidSchemas(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-description-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaRequiresDescription{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "schema-requires-description")

	// Should flag NoDescription, EmptyDescription, WhitespaceDescription
	// Should NOT flag HasDescription
	if len(ruleViolations) != 3 {
		t.Fatalf("expected 3 violations, got %d: %v", len(ruleViolations), ruleViolations)
	}

	expectedPaths := map[string]bool{
		"#/components/schemas/NoDescription":         false,
		"#/components/schemas/EmptyDescription":      false,
		"#/components/schemas/WhitespaceDescription": false,
	}

	for _, v := range ruleViolations {
		if _, ok := expectedPaths[v.Path]; !ok {
			t.Errorf("unexpected violation path: %q", v.Path)
		} else {
			expectedPaths[v.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("expected violation for path %q but none found", path)
		}
	}
}

func TestSchemaRequiresDescription_ValidSchemas(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-description-valid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaRequiresDescription{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "schema-requires-description")

	if len(ruleViolations) != 0 {
		t.Fatalf("expected 0 violations for schemas with descriptions, got %d: %v", len(ruleViolations), ruleViolations)
	}
}

func TestSchemaRequiresDescription_ViolationMessage(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-description-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaRequiresDescription{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())

	for _, v := range violations {
		if v.Path == "#/components/schemas/NoDescription" {
			if v.RuleName != "schema-requires-description" {
				t.Errorf("expected rule name 'schema-requires-description', got %q", v.RuleName)
			}
			if v.Message == "" {
				t.Error("violation message should not be empty")
			}
			return
		}
	}
	t.Error("expected to find violation for NoDescription schema")
}
