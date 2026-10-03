package rulepack

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

func optionNames(id string) []string {
	switch id {
	case "openapi.operation-id":
		return []string{"required", "pattern"}
	case "openapi.operation-success-response":
		return []string{"classes"}
	case "openapi.auth-required":
		return []string{"public-operations"}
	case "openapi.server-policy":
		return []string{"protocols", "hosts"}
	case "openapi.error-response":
		return []string{"error-schema"}
	case "openapi.pagination":
		return []string{"collection-operations", "next-token-field", "max-results-field", "pagination-field"}
	case "portos.operation-vocabulary":
		return []string{"operations", "modifiers"}
	case "portos.query-graph":
		return []string{"query-schema"}
	case "portos.name-schema":
		return []string{"name-schema"}
	case "portos.description-schema":
		return []string{"description-schema"}
	case "portos.date-fields":
		return []string{"date-fields"}
	case "portos.async-response":
		return []string{"async-schema", "item-id-field"}
	case "portos.collection-response":
		return []string{"results-field", "pagination-field"}
	case "portos.pagination-response":
		return []string{"max-results-field", "next-token-field", "pagination-field"}
	case "portos.pagination-request":
		return []string{"max-results-field", "next-token-field", "pagination-field"}
	case "portos.batch-contract":
		return []string{"items-field", "results-field", "item-id-field"}
	}
	return nil
}
func validateCheckOptions(id string, n yaml.Node) error {
	if n.Kind == 0 {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("options must be a mapping")
	}
	allowed := map[string]bool{}
	for _, key := range optionNames(id) {
		allowed[key] = true
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if !allowed[n.Content[i].Value] {
			return fmt.Errorf("check %s has no option %q", id, n.Content[i].Value)
		}
	}
	return nil
}
