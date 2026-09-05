package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"testing"
)

func TestSchemaNamingConvention_InvalidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-naming-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())

	// Should flag create_widget_request, endpoint-group-response, createEndpointRequest
	// Should NOT flag ValidSchema
	ruleViolations := filterByRule(violations, "schema-naming-convention")

	if len(ruleViolations) != 3 {
		t.Fatalf("expected 3 violations, got %d: %v", len(ruleViolations), ruleViolations)
	}

	// Check that each violation has the schema name in the path
	expectedPaths := map[string]bool{
		"#/components/schemas/create_widget_request":   false,
		"#/components/schemas/endpoint-group-response": false,
		"#/components/schemas/createEndpointRequest":   false,
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

func TestSchemaNamingConvention_ValidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-naming-valid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())

	ruleViolations := filterByRule(violations, "schema-naming-convention")
	if len(ruleViolations) != 0 {
		t.Fatalf("expected 0 violations for valid PascalCase schemas, got %d: %v", len(ruleViolations), ruleViolations)
	}
}

func TestSchemaNamingConvention_ViolationMessage(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/schema-naming-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.SchemaNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())

	// Find the snake_case violation and check message content
	for _, v := range violations {
		if v.Path == "#/components/schemas/create_widget_request" {
			if v.RuleName != "schema-naming-convention" {
				t.Errorf("expected rule name 'schema-naming-convention', got %q", v.RuleName)
			}
			// Message should include the schema name and expected format
			if v.Message == "" {
				t.Error("violation message should not be empty")
			}
			return
		}
	}
	t.Error("expected to find violation for create_widget_request")
}

func filterByRule(violations []linter.Violation, ruleName string) []linter.Violation {
	var filtered []linter.Violation
	for _, v := range violations {
		if v.RuleName == ruleName {
			filtered = append(filtered, v)
		}
	}
	return filtered
}
