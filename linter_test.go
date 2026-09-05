package linter_test

import (
	"github.com/portpowered/openapi-linter"
	"testing"
)

func TestLoadSpec_ParsesStandaloneFixture(t *testing.T) {
	l := &linter.Linter{}

	err := l.LoadSpec("testdata/test-schema.yaml")
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	doc := l.Document()
	if doc == nil {
		t.Fatal("expected non-nil document after LoadSpec")
	}

	// Assert at least one path exists
	if doc.Paths == nil || doc.Paths.PathItems.Len() == 0 {
		t.Fatal("expected at least one path in the parsed spec")
	}

	// Assert at least one schema component exists
	if doc.Components == nil || doc.Components.Schemas == nil || doc.Components.Schemas.Len() == 0 {
		t.Fatal("expected at least one schema component in the parsed spec")
	}

	t.Logf("Parsed spec: %d paths, %d schemas",
		doc.Paths.PathItems.Len(),
		doc.Components.Schemas.Len(),
	)
}

func TestLoadSpec_ReturnsErrorForMissingFile(t *testing.T) {
	l := &linter.Linter{}

	err := l.LoadSpec("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
