package linter_test

import (
	"context"
	linter "github.com/portpowered/openapi-linter"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const referenceSpec = `openapi: 3.0.3
info: {title: Reference API, version: 1.0.0}
paths: {}
components:
  schemas:
    Item:
      $ref: '%s'
`

func TestReferenceBoundaries(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "api.yaml")
	outside := filepath.Join(parent, "outside.yaml")
	if err := os.WriteFile(outside, []byte("type: string\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"../outside.yaml", "https://example.invalid/schema.yaml"} {
		content := strings.Replace(referenceSpec, "%s", ref, 1)
		if err := os.WriteFile(input, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := linter.LoadDocument(context.Background(), input, root); err == nil {
			t.Fatalf("accepted reference %s", ref)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "item.yaml"), []byte("type: string\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte(strings.Replace(referenceSpec, "%s", "item.yaml", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := linter.LoadDocument(context.Background(), input, root); err != nil {
		t.Fatalf("local reference: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "alias.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(input, []byte(strings.Replace(referenceSpec, "%s", "alias.yaml", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := linter.LoadDocument(context.Background(), input, root); err == nil {
		t.Fatal("accepted symlink escape")
	}
}
