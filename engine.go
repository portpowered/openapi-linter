package linter

import (
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"sort"
	"strings"
)

// Engine executes public visitor checks over an OpenAPI document.
type Engine struct{ rules []Rule }

// RegisterRule adds a REST API rule to the engine.
func (e *Engine) RegisterRule(r Rule) {
	e.rules = append(e.rules, r)
}

// Run walks the document's schemas, paths, and operations, calling each
// registered rule's visitor methods and collecting all violations.
func (e *Engine) Run(doc *v3high.Document) []Violation {
	var violations []Violation

	violations = append(violations, e.visitSchemas(doc)...)
	violations = append(violations, e.visitPaths(doc)...)

	return violations
}

func (e *Engine) visitSchemas(doc *v3high.Document) []Violation {
	var violations []Violation

	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil
	}

	for pair := doc.Components.Schemas.First(); pair != nil; pair = pair.Next() {
		schemaName := pair.Key()
		schemaProxy := pair.Value()

		schema, err := schemaProxy.BuildSchema()
		if err != nil {
			continue
		}

		for _, rule := range e.rules {
			violations = append(violations, located(rule.VisitSchema(schemaName, schema), "#/components/schemas/"+PointerSegment(schemaName))...)
		}
	}

	return violations
}

func (e *Engine) visitPaths(doc *v3high.Document) []Violation {
	var violations []Violation

	if doc.Paths == nil {
		return nil
	}

	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		pathStr := pair.Key()
		pathItem := pair.Value()

		for _, rule := range e.rules {
			violations = append(violations, located(rule.VisitPath(pathStr, pathItem), "#/paths/"+PointerSegment(pathStr))...)
		}

		violations = append(violations, e.visitOperations(pathStr, pathItem)...)
	}

	return violations
}

func (e *Engine) visitOperations(path string, pathItem *v3high.PathItem) []Violation {
	var violations []Violation

	operations := map[string]*v3high.Operation{
		"GET":     pathItem.Get,
		"POST":    pathItem.Post,
		"PUT":     pathItem.Put,
		"PATCH":   pathItem.Patch,
		"DELETE":  pathItem.Delete,
		"HEAD":    pathItem.Head,
		"OPTIONS": pathItem.Options,
		"TRACE":   pathItem.Trace,
	}

	methods := make([]string, 0, len(operations))
	for method := range operations {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		op := operations[method]
		if op == nil {
			continue
		}
		for _, rule := range e.rules {
			violations = append(violations, located(rule.VisitOperation(path, method, op), "#/paths/"+PointerSegment(path)+"/"+strings.ToLower(method))...)
		}
	}

	return violations
}

func located(violations []Violation, pointer string) []Violation {
	for i := range violations {
		if violations[i].Pointer == "" {
			violations[i].Pointer = pointer
		}
	}
	return violations
}
