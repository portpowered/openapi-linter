package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"strings"
	"testing"
)

func TestRequestResponseRequiresExample_InvalidOperations(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/request-response-example-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.RequestResponseRequiresExample{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "request-response-requires-example")

	// POST /widgets: missing request body example (1) + missing response example (1) = 2
	// GET /widgets: missing response example (1)
	// GET /widgets/{widgetId}: has example (0)
	// Total: 3 violations
	if len(ruleViolations) != 3 {
		t.Fatalf("expected 3 violations, got %d: %v", len(ruleViolations), ruleViolations)
	}

	var foundRequestBody, foundPostResponse, foundGetResponse bool
	for _, v := range ruleViolations {
		switch {
		case strings.Contains(v.Message, "request body"):
			foundRequestBody = true
		case strings.Contains(v.Message, "response") && strings.Contains(v.Path, "POST"):
			foundPostResponse = true
		case strings.Contains(v.Message, "response") && strings.Contains(v.Path, "GET"):
			foundGetResponse = true
		}
	}

	if !foundRequestBody {
		t.Error("expected violation for missing request body example")
	}
	if !foundPostResponse {
		t.Error("expected violation for missing POST response example")
	}
	if !foundGetResponse {
		t.Error("expected violation for missing GET response example")
	}
}

func TestRequestResponseRequiresExample_ValidOperations(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/request-response-example-valid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.RequestResponseRequiresExample{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "request-response-requires-example")

	if len(ruleViolations) != 0 {
		t.Fatalf("expected 0 violations for operations with examples, got %d: %v", len(ruleViolations), ruleViolations)
	}
}

func TestRequestResponseRequiresExample_ViolationMessage(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/request-response-example-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.RequestResponseRequiresExample{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "request-response-requires-example")

	for _, v := range ruleViolations {
		if strings.Contains(v.Message, "request body") {
			if v.RuleName != "request-response-requires-example" {
				t.Errorf("expected rule name 'request-response-requires-example', got %q", v.RuleName)
			}
			if !strings.Contains(v.Path, "POST") {
				t.Errorf("expected path to contain 'POST', got %q", v.Path)
			}
			if !strings.Contains(v.Message, "application/json") {
				t.Errorf("expected message to contain content type, got %q", v.Message)
			}
			return
		}
	}
	t.Error("expected to find violation for missing request body example")
}
