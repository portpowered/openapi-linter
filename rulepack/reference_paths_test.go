package rulepack

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReferencePathChecksHaveRunAndRootScope(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "schema.yaml")
	if err := os.WriteFile(file, []byte("type: string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := withGraphCache(context.Background())
	first := graph{root: root, checked: graphPathChecks(ctx)}
	second := graph{root: root, checked: graphPathChecks(ctx)}
	for range 100 {
		if err := first.checkPath(file); err != nil {
			t.Fatal(err)
		}
		if err := second.checkPath(file); err != nil {
			t.Fatal(err)
		}
	}
	if len(graphPathChecks(ctx)) != 1 {
		t.Fatal("root/target pair was not shared across checks")
	}
	otherRoot := t.TempDir()
	outside := graph{root: otherRoot, checked: graphPathChecks(ctx)}
	if err := outside.checkPath(file); err == nil {
		t.Fatal("a previous root's validation authorized an escape")
	}
	if len(graphPathChecks(ctx)) != 1 {
		t.Fatal("a failed path check was retained")
	}
	fresh := withGraphCache(ctx)
	if len(graphPathChecks(fresh)) != 0 || len(graphFiles(fresh)) != 0 {
		t.Fatal("a new run inherited earlier validation or parsed files")
	}
	standalone := graph{root: root}
	if err := standalone.checkPath(file); err != nil || len(standalone.checked) != 1 {
		t.Fatalf("direct analyzer call: %v", err)
	}
}

func TestReferencePathChecksObserveSymlinkChangesInNewRuns(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(root, "schema.yaml")
	if err := os.WriteFile(target, []byte("type: string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := graph{root: root, checked: graphPathChecks(withGraphCache(context.Background()))}
	if err := first.checkPath(target); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(outside, "external.yaml")
	if err := os.WriteFile(external, []byte("type: integer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	second := graph{root: root, checked: graphPathChecks(withGraphCache(context.Background()))}
	if err := second.checkPath(target); err == nil {
		t.Fatal("new run accepted a changed symlink escape")
	}
}
