package rules

import (
	"fmt"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/portpowered/openapi-linter"
)

// RequestResponseRequiresExample validates that every request body and response
// media type includes an example or examples field.
type RequestResponseRequiresExample struct{}

func (r *RequestResponseRequiresExample) Name() string {
	return "request-response-requires-example"
}

func (r *RequestResponseRequiresExample) VisitSchema(_ string, _ *base.Schema) []linter.Violation {
	return nil
}

func (r *RequestResponseRequiresExample) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	return nil
}

func (r *RequestResponseRequiresExample) VisitOperation(path string, method string, op *v3high.Operation) []linter.Violation {
	if op == nil {
		return nil
	}
	var violations []linter.Violation
	location := fmt.Sprintf("%s %s", method, path)

	// Check request body content for examples.
	if op.RequestBody != nil && op.RequestBody.Content != nil {
		for pair := op.RequestBody.Content.First(); pair != nil; pair = pair.Next() {
			mt := pair.Value()
			if !mediaTypeHasExample(mt) {
				violations = append(violations, linter.Violation{
					RuleName: r.Name(),
					Path:     location,
					Message:  fmt.Sprintf("request body content type %q is missing an example or examples", pair.Key()),
				})
			}
		}
	}

	// Check response content for examples.
	if op.Responses != nil && op.Responses.Codes != nil {
		for codePair := op.Responses.Codes.First(); codePair != nil; codePair = codePair.Next() {
			resp := codePair.Value()
			if resp == nil || resp.Content == nil {
				continue
			}
			for mtPair := resp.Content.First(); mtPair != nil; mtPair = mtPair.Next() {
				mt := mtPair.Value()
				if !mediaTypeHasExample(mt) {
					violations = append(violations, linter.Violation{
						RuleName: r.Name(),
						Path:     location,
						Message:  fmt.Sprintf("response %s content type %q is missing an example or examples", codePair.Key(), mtPair.Key()),
					})
				}
			}
		}
	}

	if op.Responses != nil && op.Responses.Default != nil && op.Responses.Default.Content != nil {
		for pair := op.Responses.Default.Content.First(); pair != nil; pair = pair.Next() {
			if !mediaTypeHasExample(pair.Value()) {
				violations = append(violations, linter.Violation{RuleName: r.Name(), Path: location, Message: fmt.Sprintf("response default content type %q is missing an example or examples", pair.Key())})
			}
		}
	}

	return violations
}

// mediaTypeHasExample returns true if the media type has either an Example
// node or a non-empty Examples map.
func mediaTypeHasExample(mt *v3high.MediaType) bool {
	if mt == nil {
		return false
	}
	if mt.Example != nil {
		return true
	}
	if mt.Examples != nil && mt.Examples.Len() > 0 {
		return true
	}
	return false
}
