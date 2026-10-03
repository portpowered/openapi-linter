package rules

import (
	"fmt"
	"regexp"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/portpowered/openapi-linter"
)

// camelCaseRegex matches strings starting with a lowercase letter followed by
// any combination of letters and digits (camelCase).
var camelCaseRegex = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

// OperationIDNamingConvention validates that all operationId values follow
// camelCase convention (e.g., createEndpoint) and that every operation has
// an operationId defined.
type OperationIDNamingConvention struct{}

func (r *OperationIDNamingConvention) Name() string {
	return "operation-id-naming-convention"
}

func (r *OperationIDNamingConvention) VisitSchema(_ string, _ *base.Schema) []linter.Violation {
	return nil
}

func (r *OperationIDNamingConvention) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	return nil
}

func (r *OperationIDNamingConvention) VisitOperation(path string, method string, op *v3high.Operation) []linter.Violation {
	if op == nil {
		return nil
	}
	var violations []linter.Violation

	if op.OperationId == "" {
		violations = append(violations, linter.Violation{
			RuleName: r.Name(),
			Path:     fmt.Sprintf("%s %s", method, path),
			Message:  "operation is missing a required operationId",
		})
		return violations
	}

	if !camelCaseRegex.MatchString(op.OperationId) {
		violations = append(violations, linter.Violation{
			RuleName: r.Name(),
			Path:     fmt.Sprintf("%s %s", method, path),
			Message:  fmt.Sprintf("operationId %q must be camelCase (e.g., %q)", op.OperationId, "createEndpoint"),
		})
	}

	return violations
}
