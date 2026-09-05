package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"strings"
	"testing"
)

func TestOperationIDNamingConvention_InvalidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/operation-id-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.OperationIDNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "operation-id-naming-convention")

	// Should flag: ListEndpoints (PascalCase), endpoint_create (snake_case), missing operationId
	// Should NOT flag: deleteEndpoint (valid camelCase)
	if len(ruleViolations) != 3 {
		t.Fatalf("expected 3 violations, got %d: %v", len(ruleViolations), ruleViolations)
	}

	var foundPascal, foundSnake, foundMissing bool
	for _, v := range ruleViolations {
		switch {
		case strings.Contains(v.Message, "ListEndpoints"):
			foundPascal = true
		case strings.Contains(v.Message, "endpoint_create"):
			foundSnake = true
		case strings.Contains(v.Message, "missing"):
			foundMissing = true
		}
	}

	if !foundPascal {
		t.Error("expected violation for PascalCase operationId 'ListEndpoints'")
	}
	if !foundSnake {
		t.Error("expected violation for snake_case operationId 'endpoint_create'")
	}
	if !foundMissing {
		t.Error("expected violation for missing operationId")
	}
}

func TestOperationIDNamingConvention_ValidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/operation-id-valid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.OperationIDNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "operation-id-naming-convention")

	if len(ruleViolations) != 0 {
		t.Fatalf("expected 0 violations for valid camelCase operationIds, got %d: %v", len(ruleViolations), ruleViolations)
	}
}

func TestOperationIDNamingConvention_ViolationMessage(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/operation-id-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.OperationIDNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "operation-id-naming-convention")

	for _, v := range ruleViolations {
		if strings.Contains(v.Message, "ListEndpoints") {
			// Check violation includes path, method, and operationId
			if v.RuleName != "operation-id-naming-convention" {
				t.Errorf("expected rule name 'operation-id-naming-convention', got %q", v.RuleName)
			}
			if !strings.Contains(v.Path, "/endpoints") {
				t.Errorf("expected path to contain '/endpoints', got %q", v.Path)
			}
			if !strings.Contains(v.Path, "GET") {
				t.Errorf("expected path to contain method 'GET', got %q", v.Path)
			}
			return
		}
	}
	t.Error("expected to find violation for PascalCase operationId 'ListEndpoints'")
}
