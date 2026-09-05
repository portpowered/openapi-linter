package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/portpowered/openapi-linter"
)

// kebabCaseRegex matches lowercase letters, digits, and hyphens.
var kebabCaseRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// wellKnownSegments contains path segments defined by external standards
// (e.g., RFC 8615, RFC 7517) that cannot be renamed to kebab-case.
var wellKnownSegments = map[string]bool{
	".well-known": true,
	"jwks.json":   true,
}

// PathNamingConvention validates that all API path segments (excluding path
// parameters and well-known standard segments) use kebab-case.
type PathNamingConvention struct{}

func (r *PathNamingConvention) Name() string {
	return "path-naming-convention"
}

func (r *PathNamingConvention) VisitSchema(_ string, _ *base.Schema) []linter.Violation {
	return nil
}

func (r *PathNamingConvention) VisitPath(path string, _ *v3high.PathItem) []linter.Violation {
	var violations []linter.Violation

	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, seg := range segments {
		if seg == "" {
			continue
		}

		// Skip path parameter segments (e.g., {groupId}, {prefix:.*})
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			continue
		}

		// Skip well-known standard segments (e.g., .well-known, jwks.json)
		if wellKnownSegments[seg] {
			continue
		}

		if !kebabCaseRegex.MatchString(seg) {
			violations = append(violations, linter.Violation{
				RuleName: r.Name(),
				Path:     path,
				Message:  fmt.Sprintf("path segment %q must be kebab-case (e.g., %q)", seg, "endpoint-groups"),
			})
		}
	}

	return violations
}

func (r *PathNamingConvention) VisitOperation(_ string, _ string, _ *v3high.Operation) []linter.Violation {
	return nil
}
