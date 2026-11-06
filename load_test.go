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

func TestRepeatedReferencePathsObserveLaterLoads(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.yaml")
	shared := filepath.Join(root, "shared.yaml")
	write := func(file, source string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := "type: object\nproperties:\n  first: {$ref: 'shared.yaml'}\n  second: {$ref: './shared.yaml'}\n"
	write(input, source)
	write(shared, "type: string\n")
	load := func(wantFailure bool) {
		t.Helper()
		doc, err := linter.LoadSchemaDocument(context.Background(), input, root)
		if wantFailure {
			if err == nil {
				t.Fatal("accepted changed reference graph")
			}
		} else if err != nil || string(doc.Source) != source || doc.Root == nil {
			t.Fatalf("repeated reference source: %v", err)
		}
	}
	load(false)
	write(shared, "type: object\nproperties:\n  escape: {$ref: '../outside.yaml'}\n")
	load(true)
	write(shared, "type: string\n")
	load(false)
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}
	load(true)
	write(shared, "type: string\n")
	load(false)
}

func TestReferenceAliasesRemainConfinedOnEveryLoad(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "schema.yaml")
	allowed := filepath.Join(root, "allowed.yaml")
	outside := filepath.Join(parent, "outside.yaml")
	alias := filepath.Join(root, "alias.yaml")
	for file, source := range map[string]string{
		input:   "type: object\nproperties:\n  first: {$ref: 'allowed.yaml'}\n  second: {$ref: 'alias.yaml'}\n  third: {$ref: 'alias.yaml'}\n",
		allowed: "type: string\n", outside: "type: number\n",
	} {
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(allowed, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := linter.LoadSchemaDocument(context.Background(), input, root); err != nil {
		t.Fatalf("safe alias of visited file: %v", err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := linter.LoadSchemaDocument(context.Background(), input, root); err == nil {
		t.Fatal("a new load reused an obsolete safe alias")
	}
}
