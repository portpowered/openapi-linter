package rulepack

import (
	"context"
	"encoding/json"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	linter "github.com/portpowered/openapi-linter"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkAPI(t *testing.T, id, source, options string) []linter.Diagnostic {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	if e := os.WriteFile(file, []byte(source), 0600); e != nil {
		t.Fatal(e)
	}
	var node yaml.Node
	if e := yaml.Unmarshal([]byte(source), &node); e != nil {
		t.Fatal(e)
	}
	r := NewRegistry()
	if e := RegisterStock(r); e != nil {
		t.Fatal(e)
	}
	var opts yaml.Node
	if options != "" {
		if e := yaml.Unmarshal([]byte(options), &opts); e != nil {
			t.Fatal(e)
		}
		opts = *opts.Content[0]
	}
	p, e := r.Compile(Pack{Version: 1, Rules: []Rule{{ID: "customer.check", Check: id, Options: opts}}})
	if e != nil {
		t.Fatal(e)
	}
	findings, e := p.Run(context.Background(), root, []*linter.Document{{Path: file, Source: []byte(source), Root: &node, Model: &v3.Document{}}})
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range findings {
		if f.RuleID != "customer.check" || f.Severity != linter.SeverityError || f.Line < 1 || f.Pointer == "" {
			t.Fatalf("invalid diagnostic %#v", f)
		}
	}
	return findings
}

func TestEditorSchemaAcceptsShippedPacksAndRejectsUnknownFields(t *testing.T) {
	r := NewRegistry()
	if e := RegisterStock(r); e != nil {
		t.Fatal(e)
	}
	data, e := json.Marshal(r.ConfigurationSchema())
	if e != nil {
		t.Fatal(e)
	}
	var resource any
	if e = json.Unmarshal(data, &resource); e != nil {
		t.Fatal(e)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(offlineSchemas{})
	if e = compiler.AddResource("urn:lint:config", resource); e != nil {
		t.Fatal(e)
	}
	schema, e := compiler.Compile("urn:lint:config")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range PresetNames() {
		pack, _ := rawPreset(name)
		data, e := yaml.Marshal(pack)
		if e != nil {
			t.Fatal(e)
		}
		var value any
		if e = yaml.Unmarshal(data, &value); e != nil {
			t.Fatal(e)
		}
		data, e = json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(data, &value); e != nil {
			t.Fatal(e)
		}
		if e = schema.Validate(value); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
	}
	if e = schema.Validate(map[string]any{"version": float64(1), "unknown": true}); e == nil {
		t.Fatal("accepted unknown config field")
	}
}

const apiHeader = "openapi: 3.1.0\ninfo: {title: Test, version: '1'}\n"

func TestCorrectnessCases(t *testing.T) {
	cases := []struct {
		id, source string
		count      int
	}{
		{"openapi.operation-id-unique", "paths:\n  /pets:\n    get: {operationId: getPets}\n    post: {operationId: getPets}\n", 1},
		{"openapi.parameters-unique", "paths:\n  /pets/{id}:\n    parameters: [{name: id, in: path, required: true}]\n    get:\n      parameters: [{name: id, in: path, required: true}]\n", 0},
		{"openapi.path-parameters", "paths:\n  /pets/{id}:\n    get:\n      parameters: [{name: other, in: path, required: false}]\n", 3},
		{"openapi.security-references", "components:\n  securitySchemes:\n    token: {type: http, scheme: bearer}\nsecurity: [{token: []}]\npaths: { /pets: {get: {}}}\n", 0},
		{"openapi.auth-required", "security: [{}]\npaths: { /pets: {get: {}}}\n", 1},
		{"openapi.enum-values", "paths: {}\ncomponents: {schemas: {Choice: {type: string, enum: [a, a, 3]}}}\n", 2},
		{"openapi.ref-siblings", "paths: {}\ncomponents: {schemas: {Pet: {type: string}, Alias: {$ref: '#/components/schemas/Pet', description: Fine}}}\n", 0},
		{"openapi.server-variables", "paths: {}\nservers: [{url: 'https://{host}', variables: {host: {default: a, enum: [b]}}}]\n", 1},
		{"openapi.unused-components", "paths: {/pets: {get: {responses: {'200': {content: {application/json: {schema: {$ref: '#/components/schemas/Pet'}}}}}}}}\ncomponents: {schemas: {Pet: {type: object}, Unused: {type: string}}}\n", 1},
	}
	for _, tt := range cases {
		t.Run(tt.id, func(t *testing.T) {
			got := checkAPI(t, tt.id, apiHeader+tt.source, "")
			if len(got) != tt.count {
				t.Fatalf("want %d got %#v", tt.count, got)
			}
		})
	}
}
func TestPortosPolicies(t *testing.T) {
	cases := []struct {
		id, source string
		count      int
	}{
		{"portos.operation-vocabulary", "paths: {/pets: {get: {operationId: petBatchQueryAsync}}}\n", 0},
		{"portos.operation-vocabulary", "paths: {/pets: {post: {operationId: createPet}}}\n", 2},
		{"portos.query-request", "paths: {/pets: {get: {operationId: petQuery}}}\n", 2},
		{"portos.path-description", "paths: {/pets: {description: Pets, get: {operationId: petGet}}}\n", 0},
		{"portos.open-enums", "paths: {}\ncomponents: {schemas: {Choice: {type: string, x-extensible-enum: [a, b], x-enum-varnames: [A, B]}}}\n", 0},
		{"portos.date-fields", "paths: {}\ncomponents: {schemas: {Pet: {type: object, properties: {createdAt: {type: integer}, format: {type: string}}}}}\n", 1},
		{"portos.name-schema", "paths: {}\ncomponents: {schemas: {Pet: {type: object, properties: {name: {type: string}}}}}\n", 1},
		{"portos.collection-response", "paths: {/pets: {get: {operationId: petList, responses: {'200': {content: {application/json: {schema: {type: array, items: {type: string}}}}}}}}}\n", 2},
		{"portos.pagination-response", "paths: {/pets: {get: {operationId: petList, responses: {'200': {content: {application/json: {schema: {type: object, properties: {paginationContext: {type: object, properties: {nextToken: {type: string}, maxResults: {type: integer}}}}}}}}}}}}\n", 0},
		{"portos.async-response", "paths: {/pets: {post: {operationId: petSendAsync, responses: {'202': {content: {application/json: {schema: {$ref: '#/components/schemas/AsyncIdentifier'}}}}}}}}\ncomponents: {schemas: {AsyncIdentifier: {type: object, properties: {id: {type: string}}}}}\n", 0},
	}
	for _, tt := range cases {
		t.Run(tt.id, func(t *testing.T) {
			got := checkAPI(t, tt.id, apiHeader+tt.source, "")
			if len(got) != tt.count {
				t.Fatalf("want %d got %#v", tt.count, got)
			}
		})
	}
}
func TestExamplesAreValidatedNotOnlyPresent(t *testing.T) {
	source := apiHeader + `paths:
  /pets:
    get:
      responses:
        '200':
          content:
            application/json:
              schema: {type: object, required: [id], properties: {id: {type: integer}}}
              example: {id: bad}
`
	got := checkAPI(t, "openapi.examples-valid", source, "")
	if len(got) != 1 || !strings.Contains(got[0].Pointer, "example") {
		t.Fatalf("%#v", got)
	}
	source = strings.Replace(source, "id: bad", "id: 3", 1)
	if got = checkAPI(t, "openapi.examples-valid", source, ""); len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}
func TestAllEmbeddedPresetsCompile(t *testing.T) {
	r := NewRegistry()
	if e := RegisterStock(r); e != nil {
		t.Fatal(e)
	}
	for _, name := range PresetNames() {
		p, _ := Preset(name)
		if _, e := r.Compile(p); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
	}
}
