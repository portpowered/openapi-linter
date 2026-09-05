package linter

import (
	"bytes"
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// LoadDocument reads an OpenAPI 3 document with local references confined to root.
// Remote references are rejected, including references in external schema files.
func LoadDocument(ctx context.Context, path, root string) (*Document, error) {
	if root == "" {
		root = "."
	}
	visited := map[string]bool{}
	node, source, err := validateReferences(ctx, path, root, visited)
	if err != nil {
		return nil, err
	}
	var loader Linter
	if err := loader.LoadSpec(path); err != nil {
		return nil, err
	}
	return &Document{Path: path, Source: source, Root: node, Model: loader.Document()}, nil
}

func validateReferences(ctx context.Context, path, root string, visited map[string]bool) (*yaml.Node, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if err := CheckPathRoot(root, path); err != nil {
		return nil, nil, fmt.Errorf("input or reference %q: %w", path, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, nil, err
	}
	if visited[absolute] {
		return nil, nil, nil
	}
	visited[absolute] = true
	source, err := os.ReadFile(absolute)
	if err != nil {
		return nil, nil, err
	}
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	if err := decoder.Decode(&node); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, nil, fmt.Errorf("%s must contain one YAML document", path)
	}
	seen := map[*yaml.Node]bool{}
	var visit func(*yaml.Node) error
	visit = func(n *yaml.Node) error {
		if n == nil || seen[n] {
			return nil
		}
		seen[n] = true
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value != "$ref" {
					continue
				}
				value := n.Content[i+1]
				if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
					return fmt.Errorf("%s:%d: $ref must be a string", path, value.Line)
				}
				ref, err := url.Parse(value.Value)
				if err != nil {
					return err
				}
				if ref.IsAbs() || ref.Host != "" || strings.HasPrefix(ref.Path, "/") || strings.Contains(ref.Path, "\\") || ref.RawQuery != "" {
					return fmt.Errorf("%s:%d: only relative local $ref targets are allowed", path, value.Line)
				}
				if ref.Path != "" {
					target := filepath.Join(filepath.Dir(absolute), filepath.FromSlash(ref.Path))
					if _, _, err := validateReferences(ctx, target, root, visited); err != nil {
						return err
					}
				}
			}
		}
		for _, child := range n.Content {
			if err := visit(child); err != nil {
				return err
			}
		}
		return visit(n.Alias)
	}
	if err := visit(&node); err != nil {
		return nil, nil, err
	}
	return &node, source, nil
}

// LoadSchemaDocument loads a standalone YAML schema under the same reference policy.
func LoadSchemaDocument(ctx context.Context, path, root string) (*Document, error) {
	if root == "" {
		root = "."
	}
	node, source, err := validateReferences(ctx, path, root, map[string]bool{})
	if err != nil {
		return nil, err
	}
	var content map[string]any
	if err := yaml.Unmarshal(source, &content); err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("schema must be a YAML object")
	}
	return &Document{Path: path, Source: source, Root: node, Schema: &SchemaDocument{Content: content}}, nil
}
