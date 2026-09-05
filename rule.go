package linter

import (
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Violation represents a single rule violation found in the spec.
type Violation struct {
	RuleName string
	// Pointer locates the finding in the OpenAPI document; Path preserves human context.
	Pointer string
	Path    string
	Message string
}

// Rule defines a validation rule that can visit different parts of an OpenAPI spec.
// Implementations only need to provide logic for the visitor methods relevant
// to their rule; unused methods should return nil.
type Rule interface {
	// Name returns the unique identifier for this rule.
	Name() string

	// VisitSchema is called for each named schema component.
	// schemaName is the component name (e.g., "CreateWidgetRequest").
	// schema is the resolved schema object.
	VisitSchema(schemaName string, schema *base.Schema) []Violation

	// VisitPath is called for each path in the spec.
	// path is the URL path string (e.g., "/endpoints/{endpointId}").
	// pathItem is the path item object containing operations.
	VisitPath(path string, pathItem *v3high.PathItem) []Violation

	// VisitOperation is called for each operation on each path.
	// path is the URL path, method is the HTTP method (e.g., "GET"),
	// and operation is the operation object.
	VisitOperation(path string, method string, operation *v3high.Operation) []Violation
}
