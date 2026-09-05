// Package linter provides an OpenAPI specification linter that validates
// API specs using independently registered checks.
package linter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Linter loads and holds a parsed OpenAPI document model for validation.
type Linter struct {
	document *libopenapi.DocumentModel[v3high.Document]
}

// LoadSpec reads an OpenAPI YAML file from the given path and parses it
// into a libopenapi high-level document model.
func (l *Linter) LoadSpec(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading spec file: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving absolute path: %w", err)
	}
	baseDir := filepath.Dir(absPath)

	config := &datamodel.DocumentConfiguration{
		BasePath:                            baseDir,
		AllowFileReferences:                 true,
		AllowRemoteReferences:               false,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
	}

	doc, err := libopenapi.NewDocumentWithConfiguration(data, config)
	if err != nil {
		return fmt.Errorf("parsing OpenAPI document: %w", err)
	}

	model, errs := doc.BuildV3Model()
	if model == nil {
		return fmt.Errorf("building V3 model: %v", errs)
	}
	if errs != nil {
		return fmt.Errorf("building V3 model: %w", errs)
	}

	l.document = model
	return nil
}

// Document returns the parsed high-level V3 document, or nil if no spec has been loaded.
func (l *Linter) Document() *v3high.Document {
	if l.document == nil {
		return nil
	}
	return &l.document.Model
}
