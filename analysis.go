package linter

import (
	"context"
	"errors"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Document exposes both the parsed OpenAPI model and original source locations.
type Document struct {
	// Schema is populated for standalone YAML-schema inputs instead of Model.
	Schema *SchemaDocument
	Path   string
	Source []byte
	Model  *v3.Document
	Root   *yaml.Node
}

// Line resolves a JSON pointer into the original source. Zero means unavailable.
func (d *Document) Line(pointer string) int {
	n := d.Root
	if n == nil {
		return 0
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	pointer = strings.TrimPrefix(pointer, "#")
	if pointer == "" {
		return n.Line
	}
	if !strings.HasPrefix(pointer, "/") {
		return 0
	}
	for _, segment := range strings.Split(pointer[1:], "/") {
		key := strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		var next *yaml.Node
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == key {
					next = n.Content[i+1]
					break
				}
			}
		} else if n.Kind == yaml.SequenceNode {
			i, err := strconv.Atoi(key)
			if err == nil && i >= 0 && i < len(n.Content) {
				next = n.Content[i]
			}
		}
		if next == nil {
			return 0
		}
		n = next
	}
	return n.Line
}

type Diagnostic struct {
	// Context preserves a check-specific human-readable location.
	Context  string
	Path     string
	Pointer  string
	Line     int
	RuleID   string
	Message  string
	Severity Severity
}

// Analyzer implements a stock or customer check over the selected input documents.
type Analyzer interface {
	ID() string
	Analyze(context.Context, *Pass)
}
type Pass struct {
	Documents   []*Document
	Root        string
	diagnostics []Diagnostic
	errors      []error
	data        map[string]any
}

func NewPass(documents []*Document) *Pass     { return &Pass{Documents: documents, data: map[string]any{}} }
func (p *Pass) Report(d Diagnostic)           { p.diagnostics = append(p.diagnostics, d) }
func (p *Pass) Diagnostics() []Diagnostic     { return append([]Diagnostic(nil), p.diagnostics...) }
func (p *Pass) SetData(key string, value any) { p.data[key] = value }
func (p *Pass) Data(key string) (any, bool)   { value, ok := p.data[key]; return value, ok }

// PointerSegment escapes a key for use in a JSON pointer.
func PointerSegment(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// ReportError stops pack execution with an operational failure, rather than a lint finding.
func (p *Pass) ReportError(err error) {
	if err != nil {
		p.errors = append(p.errors, err)
	}
}
func (p *Pass) Err() error { return errors.Join(p.errors...) }
