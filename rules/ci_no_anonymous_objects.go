package rules

import (
	"fmt"

	"github.com/portpowered/openapi-linter"
)

// NoAnonymousObjects validates that standalone schemas do not contain anonymous inline
// object definitions. All objects within properties must use $ref to reference
// a named schema rather than defining type: object with inline properties.
type NoAnonymousObjects struct{}

func (r *NoAnonymousObjects) Name() string {
	return "ci-no-anonymous-objects"
}

func (r *NoAnonymousObjects) VisitSchema(filePath string, schema *linter.SchemaDocument) []linter.Violation {
	if schema == nil {
		return nil
	}
	// Start walking from the root-level properties.
	// The root object itself is named (by the file), so we only check nested properties.
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

// checkProperties walks a properties map and flags any property that is an inline object.
func (r *NoAnonymousObjects) checkProperties(filePath, path string, props map[string]any) []linter.Violation {
	var violations []linter.Violation

	for propName, propDefRaw := range props {
		propDef, ok := propDefRaw.(map[string]any)
		if !ok {
			continue
		}

		propPath := fmt.Sprintf("%s.%s", path, propName)

		// If the property uses $ref, it's fine — skip it.
		if _, hasRef := propDef["$ref"]; hasRef {
			continue
		}

		violations = append(violations, r.checkNode(filePath, propPath, propDef)...)
	}

	return violations
}

// checkNode checks a single schema node for inline object violations and recurses.
func (r *NoAnonymousObjects) checkNode(filePath, path string, node map[string]any) []linter.Violation {
	var violations []linter.Violation

	typeRaw, hasType := node["type"]
	typStr, _ := typeRaw.(string)

	// Flag inline objects: type: object with properties defined inline (not via $ref).
	// Bare type: object without properties (e.g., generic maps with additionalProperties)
	// are not considered anonymous objects — they are unstructured maps.
	if hasType && typStr == "object" {
		nestedPropsRaw, hasProps := node["properties"]
		if hasProps {
			if nestedProps, ok := nestedPropsRaw.(map[string]any); ok {
				violations = append(violations, linter.Violation{
					RuleName: r.Name(),
					Path:     filePath,
					Message:  fmt.Sprintf("anonymous inline object at %s — extract to a named schema or use $ref", path),
				})

				// Also recurse into the inline object's own properties to find nested violations.
				violations = append(violations, r.checkProperties(filePath, path+".properties", nestedProps)...)
			}
		}
	}

	// Check array items for inline objects.
	if hasType && typStr == "array" {
		if itemsRaw, ok := node["items"]; ok {
			if items, ok := itemsRaw.(map[string]any); ok {
				// If items uses $ref, it's fine.
				if _, hasRef := items["$ref"]; !hasRef {
					violations = append(violations, r.checkNode(filePath, path+".items", items)...)
				}
			}
		}
	}

	return violations
}
