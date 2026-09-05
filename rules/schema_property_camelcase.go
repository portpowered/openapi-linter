package rules

import (
	"fmt"
	"regexp"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/portpowered/openapi-linter"
)

// propertyCamelCaseRegex matches camelCase property names: starts with a lowercase
// letter followed by any combination of letters and digits.
var propertyCamelCaseRegex = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

// wellKnownPropertyNames contains property names defined by external standards
// (e.g., OAuth 2.0 RFC 7591, OpenAPI extensions) that use snake_case by convention
// and cannot be renamed.
var wellKnownPropertyNames = map[string]bool{
	// OAuth 2.0 Dynamic Client Registration (RFC 7591)
	"client_id":                       true,
	"client_name":                     true,
	"client_uri":                      true,
	"client_secret":                   true,
	"client_secret_expires_at":        true,
	"logo_uri":                        true,
	"redirect_uris":                   true,
	"grant_types":                     true,
	"response_types":                  true,
	"token_endpoint_auth_method":      true,
	"token_endpoint_auth_signing_alg": true,
	"tos_uri":                         true,
	"policy_uri":                      true,
	"jwks_uri":                        true,
	"software_id":                     true,
	"software_version":                true,
	// OAuth 2.0 Device Authorization Grant (RFC 8628)
	"device_code":           true,
	"user_code":             true,
	"verification_uri":      true,
	"verification_uri_full": true,
	// OAuth 2.0 Token Response (RFC 6749)
	"access_token":  true,
	"token_type":    true,
	"expires_in":    true,
	"refresh_token": true,
	// OpenID Connect
	"id_token": true,
	// OpenAPI extension prefix
	// x- prefixed properties are handled separately
}

// SchemaPropertyCamelCase validates that all property names within schema
// components use camelCase. Properties defined by external standards (e.g.,
// OAuth RFC fields) and OpenAPI extension properties (x-*) are excluded.
type SchemaPropertyCamelCase struct{}

func (r *SchemaPropertyCamelCase) Name() string {
	return "schema-property-camelcase"
}

func (r *SchemaPropertyCamelCase) VisitSchema(schemaName string, schema *base.Schema) []linter.Violation {
	var violations []linter.Violation

	if schema.Properties == nil {
		return nil
	}

	for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
		propName := pair.Key()

		// Skip OpenAPI extension properties
		if len(propName) >= 2 && propName[:2] == "x-" {
			continue
		}

		// Skip well-known standard property names (e.g., OAuth RFC fields)
		if wellKnownPropertyNames[propName] {
			continue
		}

		// Skip single-character properties and $ prefixed properties
		if len(propName) <= 1 || propName[0] == '$' {
			continue
		}

		if !propertyCamelCaseRegex.MatchString(propName) {
			violations = append(violations, linter.Violation{
				RuleName: r.Name(),
				Path:     fmt.Sprintf("#/components/schemas/%s/properties/%s", schemaName, propName),
				Message:  fmt.Sprintf("property %q in schema %q must be camelCase (e.g., %q)", propName, schemaName, "myPropertyName"),
			})
		}
	}

	return violations
}

func (r *SchemaPropertyCamelCase) VisitPath(_ string, _ *v3high.PathItem) []linter.Violation {
	return nil
}

func (r *SchemaPropertyCamelCase) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	return nil
}
