package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"testing"
)

func TestSchemaPropertyCamelCase_InvalidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-property-camelcase-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaPropertyCamelCase{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "schema-property-camelcase")

	// Should flag: widget_name, widget_count (snake_case), PascalProperty, nested_object
	// Should NOT flag: validField (camelCase)
	if len(ruleViolations) != 4 {
		t.Fatalf("expected 4 violations, got %d: %v", len(ruleViolations), ruleViolations)
	}

	expectedPaths := map[string]bool{
		"#/components/schemas/WidgetConfig/properties/widget_name":    false,
		"#/components/schemas/WidgetConfig/properties/widget_count":   false,
		"#/components/schemas/NestedSchema/properties/PascalProperty": false,
		"#/components/schemas/NestedSchema/properties/nested_object":  false,
	}

	for _, v := range ruleViolations {
		if _, ok := expectedPaths[v.Path]; !ok {
			t.Errorf("unexpected violation path: %q", v.Path)
		} else {
			expectedPaths[v.Path] = true
		}
		if v.Message == "" {
			t.Error("violation message should not be empty")
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("expected violation for path %q but none found", path)
		}
	}
}

func TestSchemaPropertyCamelCase_ValidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-property-camelcase-valid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaPropertyCamelCase{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "schema-property-camelcase")

	if len(ruleViolations) != 0 {
		t.Fatalf("expected 0 violations for valid camelCase properties, got %d: %v", len(ruleViolations), ruleViolations)
	}
}

func TestSchemaPropertyCamelCase_ViolationMessage(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-property-camelcase-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaPropertyCamelCase{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())

	for _, v := range violations {
		if v.Path == "#/components/schemas/WidgetConfig/properties/widget_name" {
			if v.RuleName != "schema-property-camelcase" {
				t.Errorf("expected rule name 'schema-property-camelcase', got %q", v.RuleName)
			}
			if v.Message == "" {
				t.Error("violation message should not be empty")
			}
			return
		}
	}
	t.Error("expected to find violation for widget_name property")
}
