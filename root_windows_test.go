package linter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalDrivePath(t *testing.T) {
	directory := t.TempDir()
	if !ordinaryDrivePath(directory) {
		t.Skip("temporary directory is outside a local drive")
	}
	file := filepath.Join(directory, "Mixed Case é.yaml")
	if err := os.WriteFile(file, []byte("type: string\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{directory, strings.ToLower(file)} {
		want, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := canonicalDrivePath(path); got != want {
			t.Fatalf("canonical path for %q = %q, want %q", path, got, want)
		}
	}
	if _, err := resolveExistingPath(filepath.Join(directory, "missing.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing path error = %v", err)
	}
}

func TestOrdinaryDrivePath(t *testing.T) {
	for _, path := range []string{"file.yaml", "C:file.yaml", "C:\\file.yaml:stream", "\\\\server\\share\\file.yaml", "\\\\?\\C:\\file.yaml"} {
		if ordinaryDrivePath(path) {
			t.Errorf("accepted %q for native resolution", path)
		}
	}
}
