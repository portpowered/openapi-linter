package rules

import (
	"fmt"
	"regexp"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/portpowered/openapi-linter"
)

// pascalCaseRegex matches strings that start with an uppercase letter and contain
// only letters and digits (each "word" starts with an uppercase letter).
var pascalCaseRegex = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)

// SchemaNamingConvention validates that all schema component names follow PascalCase.
type SchemaNamingConvention struct{}

func (r *SchemaNamingConvention) Name() string {
	return "schema-naming-convention"
}

func (r *SchemaNamingConvention) VisitSchema(schemaName string, _ *base.Schema) []linter.Violation {
	if !pascalCaseRegex.MatchString(schemaName) {
		return []linter.Violation{{
			RuleName: r.Name(),
			Path:     "#/components/schemas/" + schemaName,
			Message:  fmt.Sprintf("schema name %q must be PascalCase (e.g., %q)", schemaName, "CreateWidgetRequest"),
		}}
	}
	return nil
}

func (r *SchemaNamingConvention) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	return nil
}

func (r *SchemaNamingConvention) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	return nil
}
