package rulepack

import (
	"gopkg.in/yaml.v3"
	"reflect"
	"strings"
)

func registerAdditional(registry *Registry) error {
	for _, id := range []string{"openapi.spec-structure",
		"openapi.operation-id-unique",
		"openapi.path-parameters",
		"openapi.parameters-unique",
		"openapi.path-syntax",
		"openapi.security-references",
		"openapi.operation-summary",
		"openapi.operation-description",
		"openapi.parameter-description",
		"openapi.operation-success-response",
		"openapi.examples-valid",
		"openapi.enum-values",
		"openapi.server-variables",
		"openapi.tags-defined",
		"openapi.info-description",
		"openapi.ref-siblings",
		"openapi.unused-components",
		"openapi.auth-required",
		"openapi.server-policy",
		"openapi.error-response",
		"openapi.info-contact",
		"openapi.info-license",
		"openapi.pagination",
		"portos.operation-vocabulary",
		"portos.query-request",
		"portos.async-response",
		"portos.collection-response",
		"portos.pagination-response",
		"portos.path-description",
		"portos.pagination-request",
		"portos.open-enums",
		"portos.date-fields",
		"portos.batch-contract",
		"portos.query-graph",
		"portos.name-schema",
		"portos.description-schema"} {
		if e := registry.Register(id, apiFactory(id)); e != nil {
			return e
		}
	}
	for _, d := range registry.Catalog() {
		if !strings.Contains(d.ID, ".") {
			continue
		}
		if !strings.HasPrefix(d.ID, "openapi.") && !strings.HasPrefix(d.ID, "schema.") && !strings.HasPrefix(d.ID, "portos.") {
			continue
		}
		d.Kind = "openapi"
		if strings.HasPrefix(d.ID, "schema.") {
			d.Kind = "schema"
		}
		d.Title = strings.ReplaceAll(strings.SplitN(d.ID, ".", 2)[1], "-", " ")
		d.Guidance = "See docs/rule-packs.md and docs/linter-roadmap.md for activation, scope, examples, and exceptions."
		d.Options = map[string]string{}
		defaults := defaultOptions()
		encoded, _ := yaml.Marshal(defaults)
		allDefaults := map[string]any{}
		_ = yaml.Unmarshal(encoded, &allDefaults)
		d.Defaults = map[string]any{}
		for _, key := range optionNames(d.ID) {
			d.Defaults[key] = allDefaults[key]
		}
		if d.ID == "openapi.operation-id" {
			d.Defaults = map[string]any{"required": true}
		}
		typ := reflect.TypeOf(defaults)
		types := map[string]string{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			types[field.Tag.Get("yaml")] = field.Type.String()
		}

		for _, key := range optionNames(d.ID) {
			d.Options[key] = types[key]
			if d.Options[key] == "" {
				d.Options[key] = "See rule-pack reference"
			}
		}
		if d.ID == "openapi.operation-id" {
			d.Options = map[string]string{"required": "bool", "pattern": "string"}
		}
		d.Presets = []string{}
		for _, name := range PresetNames() {
			p, _ := Preset(name)
			for _, rule := range p.Rules {
				if rule.Check == d.ID {
					d.Presets = append(d.Presets, name)
					break
				}
			}
		}
		d.Category = strings.SplitN(d.ID, ".", 2)[0]
		d.Severity = "warning"
		if d.Category == "portos" || d.Kind == "schema" {
			d.Severity = "error"
		}
		defaultPack, _ := Preset(DefaultPresetName())
		for _, rule := range defaultPack.Rules {
			if rule.Check == d.ID {
				d.Severity = string(rule.Severity)
			}
		}
		d.Fixable = false

		if e := registry.SetDescriptor(d); e != nil {
			return e
		}
	}
	return registerGoogle(registry)
}
