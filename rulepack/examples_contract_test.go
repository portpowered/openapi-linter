package rulepack

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

func TestExamplesRespectDialectAndDirection(t *testing.T) {
	cases := []struct {
		name, version, schema, example string
		findings                       int
	}{
		{"boolean false schema", "3.1.0", "false", "1", 1},
		{"boolean true schema", "3.1.0", "true", "1", 0},
		{"nullable", "3.0.3", "{type: string, nullable: true}", "null", 0},
		{"nonnullable", "3.0.3", "{type: string}", "null", 1},
		{"exclusive lower", "3.0.3", "{type: number, minimum: 2, exclusiveMinimum: true}", "2", 1},
		{"inclusive lower", "3.0.3", "{type: number, minimum: 2, exclusiveMinimum: false}", "2", 0},
		{"exclusive upper", "3.0.3", "{type: number, maximum: 2, exclusiveMaximum: true}", "2", 1},
		{"ref siblings enforced", "3.1.0", "{$ref: '#/components/schemas/Number', maximum: 3}", "4", 1},
		{"ref siblings ignored in 30", "3.0.3", "{$ref: '#/components/schemas/Number', maximum: 3}", "4", 0},
		{"union nullable", "3.1.0", "{type: [string, 'null']}", "null", 0},
		{"recursive finite instance", "3.1.0", "{$ref: '#/components/schemas/Node'}", "{value: root, child: {value: leaf}}", 0},
		{"recursive invalid leaf", "3.1.0", "{$ref: '#/components/schemas/Node'}", "{value: root, child: {value: 123}}", 1},
		{"request read only omitted", "3.1.0", "{type: object, required: [id, secret], properties: {id: {type: string, readOnly: true}, secret: {type: string, writeOnly: true}}}", "{secret: password}", 0},
		{"request secret still required", "3.1.0", "{type: object, required: [id, secret], properties: {id: {type: string, readOnly: true}, secret: {type: string, writeOnly: true}}}", "{}", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := "openapi: " + c.version + "\ninfo: {title: Test, version: v1}\npaths:\n  /widgets:\n    post:\n      requestBody:\n        content:\n          application/json:\n            schema: " + c.schema + "\n            example: " + c.example + "\n      responses: {'204': {description: Done}}\ncomponents:\n  schemas:\n    Number: {type: number}\n    Node:\n      type: object\n      required: [value]\n      properties:\n        value: {type: string}\n        child: {$ref: '#/components/schemas/Node'}\n"
			if got := len(checkAPI(t, "openapi.examples-valid", source, "")); got != c.findings {
				t.Fatalf("got %d findings, want %d", got, c.findings)
			}
		})
	}
}

func TestExampleLocationsAndResponseWriteOnly(t *testing.T) {
	source := `openapi: 3.1.0
info: {title: Test, version: v1}
paths:
  /widgets:
    parameters:
      - in: query
        name: count
        schema: {type: integer}
        examples:
          invalid: {value: text}
          remote: {externalValue: 'https://example.invalid/not-fetched'}
    get:
      responses:
        default:
          description: Result
          content:
            application/json:
              schema:
                type: object
                required: [id, secret]
                properties:
                  id: {type: string, readOnly: true}
                  secret: {type: string, writeOnly: true}
              examples:
                valid: {value: {id: abc}}
                invalid: {$ref: '#/components/examples/MissingID'}
components:
  examples:
    MissingID: {value: {secret: hidden}}
  schemas:
    Labels:
      type: array
      items: {type: string}
      examples: [[good], [42]]
`
	findings := checkAPI(t, "openapi.examples-valid", source, "")
	if len(findings) != 3 {
		t.Fatalf("expected parameter, response and schema examples: %#v", findings)
	}
	for _, f := range findings {
		if !strings.Contains(f.Pointer, "examples") {
			t.Fatalf("wrong example location: %#v", f)
		}
	}
}

func TestExampleValidationOperationalFailures(t *testing.T) {
	for _, source := range []string{
		"openapi: 3.1.0\njsonSchemaDialect: https://example.invalid/custom\npaths: {}\n",
		"openapi: 3.1.0\ncomponents: {schemas: {Bad: {type: invalid, example: 1}}}\n",
		"openapi: 3.1.0\ncomponents: {schemas: {Bad: {type: object, example: {1: value}}}}\n",
		"openapi: 3.1.0\ncomponents: {schemas: {Bad: {type: number, example: .nan}}}\n",
		"openapi: 3.1.0\ncomponents: {schemas: {Bad: {type: array, example: &loop [*loop]}}}\n",
	} {
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(source), &node); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		file := filepath.Join(root, "api.yaml")
		g := graph{root: root, files: map[string]*yaml.Node{file: unwrap(&node)}}
		pass := linter.NewPass(nil)
		apiCheck{id: "openapi.examples-valid"}.validateExamples(context.Background(), &g, site{n: unwrap(&node), file: file, ptr: "#"}, func(site, string) { t.Fatal("operational failure reported as finding") }, pass)
		if pass.Err() == nil {
			t.Fatalf("missing operational error for %s", source)
		}
	}
	if _, err := (offlineSchemas{}).Load("https://example.invalid/schema"); err == nil {
		t.Fatal("offline loader fetched absent resource")
	}
	if got, err := (offlineSchemas{map[string]any{"urn:test": true}}).Load("urn:test"); err != nil || got != true {
		t.Fatalf("local resource: %v, %v", got, err)
	}
}

func TestExamplesPreserveCompositeSchemaConstraints(t *testing.T) {
	source := `openapi: 3.1.0
info: {title: Composite, version: v1}
paths:
  /values:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              $defs:
                Score: {type: integer, minimum: 0}
              properties:
                score: {$ref: '#/paths/~1values/post/requestBody/content/application~1json/schema/$defs/Score'}
                tuple: {type: array, prefixItems: [{type: string}, {type: integer}], items: false}
                values: {type: array, contains: {type: integer}, minContains: 1}
                label: {allOf: [{type: string}, {not: {const: forbidden}}]}
                choice: {oneOf: [{type: string}, {type: integer}]}
                fallback: {anyOf: [{type: string}, {type: boolean}]}
                mode: {type: string}
              patternProperties:
                '^x_': {type: integer}
              additionalProperties: false
              if: {properties: {mode: {const: high}}, required: [mode]}
              then: {required: [score]}
              else: {properties: {score: {maximum: 3}}}
            examples:
              valid: {value: {score: 2, tuple: [name, 1], values: [1], label: allowed, choice: 1, fallback: true, x_count: 2}}
              negative: {value: {score: -1}}
              tuple: {value: {tuple: [name, wrong]}}
              contains: {value: {values: [text]}}
              pattern: {value: {x_count: wrong}}
              extra: {value: {unknown: 1}}
              conditional: {value: {mode: high}}
              inverse: {value: {score: 4}}
              forbidden: {value: {label: forbidden}}
      responses: {'204': {description: Accepted}}
`
	if got := len(checkAPI(t, "openapi.examples-valid", source, "")); got != 8 {
		t.Fatalf("composite constraints: %d want 8", got)
	}
}
