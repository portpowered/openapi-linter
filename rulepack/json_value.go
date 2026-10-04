package rulepack

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// jsonValue preserves JSON's requirement that object keys are strings. Some
// Go versions serialize numeric keys, which would silently change YAML input.
func jsonValue(node *yaml.Node) (any, error) {
	var raw any
	if err := node.Decode(&raw); err != nil {
		return nil, err
	}
	if err := checkJSONKeys(raw); err != nil {
		return nil, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func checkJSONKeys(value any) error {
	switch value := value.(type) {
	case map[any]any:
		return fmt.Errorf("YAML mapping contains a non-string key; JSON object keys must be strings")
	case map[string]any:
		for _, child := range value {
			if err := checkJSONKeys(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := checkJSONKeys(child); err != nil {
				return err
			}
		}
	}
	return nil
}
