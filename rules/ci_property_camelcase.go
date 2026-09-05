package rules

import (
	"fmt"

	"github.com/portpowered/openapi-linter"
)

// PropertyCamelCase validates that all property names within standalone schemas use
// camelCase. Properties prefixed with x- (OpenAPI extensions) are excluded.
type PropertyCamelCase struct{}

func (r *PropertyCamelCase) Name() string {
	return "ci-property-camelcase"
}

func (r *PropertyCamelCase) VisitSchema(filePath string, schema *linter.SchemaDocument) []linter.Violation {
	propsRaw, ok := schema.Content["properties"]
	if !ok {
		return nil
	}

	props, ok := propsRaw.(map[string]any)
	if !ok {
		return nil
	}

	return r.checkProperties(filePath, "properties", props)
}

// checkProperties walks a properties map and checks each property name for camelCase.
func (r *PropertyCamelCase) checkProperties(filePath, path string, props map[string]any) []linter.Violation {
	var violations []linter.Violation

	for propName, propDefRaw := range props {
		// Skip x- prefixed properties (OpenAPI extensions)
		if len(propName) >= 2 && propName[:2] == "x-" {
			continue
		}

		propPath := fmt.Sprintf("%s.%s", path, propName)

		// Check this property name
		if !propertyCamelCaseRegex.MatchString(propName) {
			violations = append(violations, linter.Violation{
				RuleName: r.Name(),
				Path:     filePath,
				Message:  fmt.Sprintf("property %q at %s must be camelCase", propName, propPath),
			})
		}

		// Recurse into nested properties (inline objects or $ref won't have properties here)
		propDef, ok := propDefRaw.(map[string]any)
		if !ok {
			continue
		}

		if nestedPropsRaw, ok := propDef["properties"]; ok {
			if nestedProps, ok := nestedPropsRaw.(map[string]any); ok {
				violations = append(violations, r.checkProperties(filePath, propPath+".properties", nestedProps)...)
			}
		}

		// Check array items properties
		if itemsRaw, ok := propDef["items"]; ok {
			if items, ok := itemsRaw.(map[string]any); ok {
				if itemPropsRaw, ok := items["properties"]; ok {
					if itemProps, ok := itemPropsRaw.(map[string]any); ok {
						violations = append(violations, r.checkProperties(filePath, propPath+".items.properties", itemProps)...)
					}
				}
			}
		}
	}

	return violations
}
