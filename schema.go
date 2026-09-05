package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchemaDocument represents a parsed standalone schema YAML file.
type SchemaDocument struct {
	// Content holds the raw parsed YAML as a map of root-level keys.
	// x-extension fields (e.g., x-type, x-namespace) are accessible as string keys.
	Content map[string]any
}

// DiscoverSchemas recursively finds all .yaml files under the given directory.
func DiscoverSchemas(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".yaml") || strings.HasSuffix(strings.ToLower(info.Name()), ".yml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking schema directory %q: %w", dir, err)
	}

	return files, nil
}

// LoadSchema reads and parses a YAML file into a SchemaDocument.
func LoadSchema(path string) (*SchemaDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema %q: %w", path, err)
	}

	var content map[string]any
	if err := yaml.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("parsing schema %q: %w", path, err)
	}

	return &SchemaDocument{Content: content}, nil
}
