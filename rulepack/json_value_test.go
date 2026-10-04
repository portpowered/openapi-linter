package rulepack

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestJSONValueRequiresStringKeys(t *testing.T) {
	for _, source := range []string{
		"{1: value}",
		"{nested: {true: value}}",
		"{items: [{1: value}]}",
		"{base: &base {1: value}, copy: *base}",
		"{base: &base {1: value}, copy: {<<: *base}}",
		"{value: .nan}",
		"&loop {self: *loop}",
	} {
		t.Run(source, func(t *testing.T) {
			var node yaml.Node
			if err := yaml.Unmarshal([]byte(source), &node); err != nil {
				t.Fatal(err)
			}
			if _, err := jsonValue(&node); err == nil {
				t.Fatal("accepted a value incompatible with JSON")
			}
		})
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("{'1': value, items: [{key: value}, null, 1], alias: &base {key: value}, copy: *base}"), &node); err != nil {
		t.Fatal(err)
	}
	value, err := jsonValue(&node)
	if err != nil {
		t.Fatal(err)
	}
	if value.(map[string]any)["1"] != "value" {
		t.Fatalf("quoted string key changed: %#v", value)
	}
}
