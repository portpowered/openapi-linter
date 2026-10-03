package rulepack

import (
	"encoding/json"
	"strings"
	"testing"

	linter "github.com/portpowered/openapi-linter"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func configurationContractSchema(t *testing.T, r *Registry) *jsonschema.Schema {
	t.Helper()
	encoded, err := json.Marshal(r.ConfigurationSchema())
	if err != nil {
		t.Fatal(err)
	}
	var resource any
	if err = json.Unmarshal(encoded, &resource); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("urn:test:config", resource); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("urn:test:config")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validateConfigurationJSON(t *testing.T, schema *jsonschema.Schema, raw string) error {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	return schema.Validate(value)
}

func TestMetadataRegistryContracts(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"", "  "} {
		if err := r.Register(id, newOperationID); err == nil {
			t.Fatal("blank identity accepted")
		}
	}
	if err := r.Register("customer.empty", nil); err == nil {
		t.Fatal("nil factory accepted")
	}
	if _, ok := r.Describe("missing"); ok {
		t.Fatal("unknown rule described")
	}
	if err := r.SetDescriptor(Descriptor{ID: "missing"}); err == nil {
		t.Fatal("unknown descriptor accepted")
	}
	for _, id := range []string{"z.customer", "a.customer"} {
		if err := r.Register(id, newOperationID); err != nil {
			t.Fatal(err)
		}
	}
	if fallback, ok := r.Describe("a.customer"); !ok || fallback.Kind != "custom" || fallback.Title != "a.customer" || fallback.Guidance == "" {
		t.Fatalf("incomplete public factory discovery: %#v", fallback)
	}
	if err := r.SetDescriptor(Descriptor{ID: "a.customer", Title: "Customer quality", Guidance: "See customer docs."}); err != nil {
		t.Fatal(err)
	}
	descriptors := r.Catalog()
	if len(descriptors) != 2 || descriptors[0].ID != "a.customer" || descriptors[0].Kind != "custom" || descriptors[0].Title != "Customer quality" || descriptors[1].ID != "z.customer" {
		t.Fatalf("catalog order/custom metadata failed: %#v", descriptors)
	}
	if err := r.Register("a.customer", newOperationID); err == nil {
		t.Fatal("duplicate identity accepted")
	}
}

func TestEditorSchemaEmptyAndCustomRegistry(t *testing.T) {
	empty := configurationContractSchema(t, NewRegistry())
	if err := validateConfigurationJSON(t, empty, `{"version":1,"rules":[]}`); err != nil {
		t.Fatal(err)
	}
	if err := validateConfigurationJSON(t, empty, `{"version":1,"rules":[{"id":"x","check":"missing"}]}`); err == nil {
		t.Fatal("empty registry accepted executable check")
	}
	r := NewRegistry()
	if err := r.Register("customer.options", func(yaml.Node) (linter.Analyzer, error) { return operationID{}, nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDescriptor(Descriptor{ID: "customer.options", Options: map[string]string{"mapping": "map[string]string", "list": "[]string", "enabled": "*bool", "count": "int", "name": "string", "extension": "Customer-specific value"}}); err != nil {
		t.Fatal(err)
	}
	schema := configurationContractSchema(t, r)
	valid := `{"version":1,"rules":[{"id":"review","check":"customer.options","options":{"mapping":{"from":"to"},"list":["one"],"enabled":true,"count":3,"name":"policy","extension":{"customer":true},"undocumented":4}}]}`
	if err := validateConfigurationJSON(t, schema, valid); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []struct{ from, to string }{
		{`"mapping":{"from":"to"}`, `"mapping":{"from":7}`},
		{`"list":["one"]`, `"list":[7]`},
		{`"enabled":true`, `"enabled":"yes"`},
		{`"count":3`, `"count":2.5`},
		{`"name":"policy"`, `"name":false`},
	} {
		if err := validateConfigurationJSON(t, schema, strings.Replace(valid, replacement.from, replacement.to, 1)); err == nil {
			t.Fatalf("editor accepted invalid option type: %+v", replacement)
		}
	}
}

func TestStockEditorSchemaContracts(t *testing.T) {
	r := NewRegistry()
	if err := RegisterStock(r); err != nil {
		t.Fatal(err)
	}
	schema := configurationContractSchema(t, r)
	valid := `{"version":1,"rules":[{"id":"ids","check":"openapi.operation-id","options":{"required":false,"pattern":"^[A-Z]"}}],"suppressions":[{"rule":"ids","path":"api.yaml","pointer":"#/paths/~1records/get","reason":"Legacy operation","line":0}]}`
	if err := validateConfigurationJSON(t, schema, valid); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []struct{ from, to string }{
		{`"version":1`, `"version":2`},
		{`"required":false`, `"required":"no"`},
		{`"pattern":"^[A-Z]"`, `"pattern":false`},
		{`"reason":"Legacy operation"`, `"reason":" "`},
		{`"line":0`, `"line":-1`},
		{`"pointer":"#/paths/~1records/get"`, `"pointer":"paths/records"`},
		{`"options":{"required":false,"pattern":"^[A-Z]"}`, `"options":{"unknown":true}`},
	} {
		if err := validateConfigurationJSON(t, schema, strings.Replace(valid, replacement.from, replacement.to, 1)); err == nil {
			t.Fatalf("editor accepted invalid configuration: %+v", replacement)
		}
	}
	for _, source := range []string{
		`{"version":1,"rules":[{"id":"pages","check":"openapi.pagination"}]}`,
		`{"version":1,"rules":[{"id":"pages","check":"openapi.pagination","options":{}}]}`,
	} {
		if err := validateConfigurationJSON(t, schema, source); err == nil {
			t.Fatal("pagination prerequisites missing")
		}
	}
	if err := validateConfigurationJSON(t, schema, `{"version":1,"rules":[{"id":"pages","check":"openapi.pagination","options":{"collection-operations":["ListRecords"]}}]}`); err != nil {
		t.Fatal(err)
	}
}

func TestRuleJSONMatchesPublicOptions(t *testing.T) {
	var options yaml.Node
	if err := yaml.Unmarshal([]byte("required: false\npattern: '^Get'\n"), &options); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []Rule{{ID: "x", Check: "openapi.operation-id"}, {ID: "y", Check: "openapi.operation-id", Options: *options.Content[0]}} {
		encoded, err := json.Marshal(rule)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err = json.Unmarshal(encoded, &value); err != nil {
			t.Fatal(err)
		}
		if value["severity"] != "error" || value["enabled"] != true || value["id"] != rule.ID {
			t.Fatalf("public defaults lost: %s", encoded)
		}
		if rule.ID == "y" {
			opts, ok := value["options"].(map[string]any)
			if !ok || opts["required"] != false || opts["pattern"] != "^Get" {
				t.Fatalf("exposed YAML internals instead of options: %s", encoded)
			}
		}
	}
}
