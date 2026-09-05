package rules_test

import (
	"github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"testing"
)

func TestPathNamingConvention_InvalidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/path-naming-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.PathNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "path-naming-convention")

	// Should flag: endpointGroups, deviceList
	// Should NOT flag: endpoint-groups, filesystems, path parameters
	if len(ruleViolations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(ruleViolations), ruleViolations)
	}

	expectedPaths := map[string]bool{
		"/endpointGroups/{groupId}":      false,
		"/systems/{systemId}/deviceList": false,
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

func TestPathNamingConvention_ValidNames(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/path-naming-valid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.PathNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())
	ruleViolations := filterByRule(violations, "path-naming-convention")

	if len(ruleViolations) != 0 {
		t.Fatalf("expected 0 violations for valid kebab-case paths, got %d: %v", len(ruleViolations), ruleViolations)
	}
}

func TestPathNamingConvention_ViolationMessage(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("../testdata/path-naming-invalid.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	rule := &rules.PathNamingConvention{}
	engine.RegisterRule(rule)

	violations := engine.Run(l.Document())

	for _, v := range violations {
		if v.Path == "/endpointGroups/{groupId}" {
			if v.RuleName != "path-naming-convention" {
				t.Errorf("expected rule name 'path-naming-convention', got %q", v.RuleName)
			}
			if v.Message == "" {
				t.Error("violation message should not be empty")
			}
			return
		}
	}
	t.Error("expected to find violation for /endpointGroups/{groupId}")
}
