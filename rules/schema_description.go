package rules

import (
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/portpowered/openapi-linter"
)

// SchemaRequiresDescription validates that every schema component has a
// non-empty description field.
type SchemaRequiresDescription struct{}

func (r *SchemaRequiresDescription) Name() string {
	return "schema-requires-description"
}

func (r *SchemaRequiresDescription) VisitSchema(schemaName string, schema *base.Schema) []linter.Violation {
	if schema == nil {
		return nil
	}
	if strings.TrimSpace(schema.Description) == "" {
		return []linter.Violation{{
			RuleName: r.Name(),
			Path:     "#/components/schemas/" + schemaName,
			Message:  fmt.Sprintf("schema %q is missing a required description", schemaName),
		}}
	}
	return nil
}

func (r *SchemaRequiresDescription) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	return nil
}

func (r *SchemaRequiresDescription) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	return nil
}
