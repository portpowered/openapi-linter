package linter_test

import (
	"github.com/portpowered/openapi-linter"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// flagTestSchemaRule is a mock rule that flags any schema named "Test".
type flagTestSchemaRule struct{}

func (r *flagTestSchemaRule) Name() string { return "flag-test-schema" }

func (r *flagTestSchemaRule) VisitSchema(name string, _ *base.Schema) []linter.Violation {
	if name == "Test" {
		return []linter.Violation{{
			RuleName: r.Name(),
			Path:     "#/components/schemas/" + name,
			Message:  "schema name 'Test' is not allowed",
		}}
	}
	return nil
}

func (r *flagTestSchemaRule) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	return nil
}

func (r *flagTestSchemaRule) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	return nil
}

func TestEngine_MockRuleFlagsTestSchema(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("testdata/test-schema.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	engine := &linter.Engine{}
	engine.RegisterRule(&flagTestSchemaRule{})

	violations := engine.Run(l.Document())

	// Should have exactly one violation for the "Test" schema
	var testViolations []linter.Violation
	for _, v := range violations {
		if v.RuleName == "flag-test-schema" {
			testViolations = append(testViolations, v)
		}
	}

	if len(testViolations) != 1 {
		t.Fatalf("expected 1 violation from flag-test-schema rule, got %d", len(testViolations))
	}

	v := testViolations[0]
	if v.Path != "#/components/schemas/Test" {
		t.Errorf("expected path '#/components/schemas/Test', got %q", v.Path)
	}
	if v.Message != "schema name 'Test' is not allowed" {
		t.Errorf("unexpected message: %q", v.Message)
	}
}

func TestEngine_NoViolationsForValidSchemas(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("testdata/test-schema.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	// Register a rule that only flags schemas named "Nonexistent"
	engine := &linter.Engine{}
	engine.RegisterRule(&noopRule{})

	violations := engine.Run(l.Document())
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestEngine_VisitsPathsAndOperations(t *testing.T) {
	l := &linter.Linter{}
	err := l.LoadSpec("testdata/test-schema.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	counter := &countingRule{}
	engine := &linter.Engine{}
	engine.RegisterRule(counter)

	engine.Run(l.Document())

	if counter.schemaCount == 0 {
		t.Error("expected VisitSchema to be called at least once")
	}
	if counter.pathCount == 0 {
		t.Error("expected VisitPath to be called at least once")
	}
	if counter.operationCount == 0 {
		t.Error("expected VisitOperation to be called at least once")
	}

	t.Logf("Visited %d schemas, %d paths, %d operations",
		counter.schemaCount, counter.pathCount, counter.operationCount)
}

// noopRule produces no violations.
type noopRule struct{}

func (r *noopRule) Name() string                                              { return "noop" }
func (r *noopRule) VisitSchema(_ string, _ *base.Schema) []linter.Violation   { return nil }
func (r *noopRule) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation { return nil }
func (r *noopRule) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	return nil
}

// countingRule counts how many times each visitor method is called.
type countingRule struct {
	schemaCount    int
	pathCount      int
	operationCount int
}

func (r *countingRule) Name() string { return "counter" }

func (r *countingRule) VisitSchema(_ string, _ *base.Schema) []linter.Violation {
	r.schemaCount++
	return nil
}

func (r *countingRule) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	r.pathCount++
	return nil
}

func (r *countingRule) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	r.operationCount++
	return nil
}
