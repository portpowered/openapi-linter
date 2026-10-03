package linter

import (
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
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
	out := []Violation{}
	active := map[*v3high.PathItem]bool{}
	var visit func(string, string, *v3high.PathItem)
	visit = func(path, pointer string, item *v3high.PathItem) {
		if item == nil || active[item] {
			return
		}
		active[item] = true
		defer delete(active, item)
		for _, rule := range e.rules {
			out = append(out, located(rule.VisitPath(path, item), pointer)...)
		}
		for pair := item.GetOperations().First(); pair != nil; pair = pair.Next() {
			method, op := pair.Key(), pair.Value()
			opPointer := pointer + "/" + strings.ToLower(method)
			for _, rule := range e.rules {
				out = append(out, located(rule.VisitOperation(path, strings.ToUpper(method), op), opPointer)...)
			}
			if op.Callbacks != nil {
				for callback := op.Callbacks.First(); callback != nil; callback = callback.Next() {
					if callback.Value().Expression == nil {
						continue
					}
					for expression := callback.Value().Expression.First(); expression != nil; expression = expression.Next() {
						visit(expression.Key(), opPointer+"/callbacks/"+PointerSegment(callback.Key())+"/"+PointerSegment(expression.Key()), expression.Value())
					}
				}
			}
		}
	}
	if doc.Paths != nil && doc.Paths.PathItems != nil {
		for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			visit(pair.Key(), "#/paths/"+PointerSegment(pair.Key()), pair.Value())
		}
	}
	if doc.Webhooks != nil {
		for pair := doc.Webhooks.First(); pair != nil; pair = pair.Next() {
			visit(pair.Key(), "#/webhooks/"+PointerSegment(pair.Key()), pair.Value())
		}
	}
	return out
}

func located(violations []Violation, pointer string) []Violation {
	for i := range violations {
		if violations[i].Pointer == "" {
			violations[i].Pointer = pointer
		}
	}
	return violations
}
