package rulepack

import (
	"context"
	"fmt"
	linter "github.com/portpowered/openapi-linter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const canonicalBatchOptions = "items-field: operations\nitem-id-field: requestId\nerror-schema: BatchItemError\nflat-error-layout: true"

func canonicalBatchSource(union string) string {
	return apiHeader + `paths:
  /endpoint-groups-manage:
    post:
      operationId: UpsertGroupsBatch
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Request'}
      responses:
        '200':
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Response'}
components:
  schemas:
    Request:
      type: object
      required: [operations]
      properties:
        operations:
          type: array
          items: {$ref: '#/components/schemas/Operation'}
    Operation:
      ` + union + `:
        - {$ref: '#/components/schemas/Create'}
        - {$ref: '#/components/schemas/Delete'}
    Base:
      type: object
      required: [requestId]
      properties:
        requestId: {$ref: '#/components/schemas/RequestId'}
    Create:
      allOf:
        - {$ref: '#/components/schemas/Base'}
        - type: object
          required: [operation, name]
          properties:
            operation: {type: string, enum: [create]}
            name: {type: string}
    Delete:
      type: object
      required: [requestId, operation, id]
      properties:
        requestId: {type: string, minLength: 1}
        operation: {type: string, enum: [delete]}
        id: {type: string}
    RequestId: {type: string, minLength: 1}
    Response:
      type: object
      required: [results, errors]
      properties:
        results:
          type: array
          items:
            type: object
            required: [requestId]
            properties:
              requestId: {$ref: '#/components/schemas/RequestId'}
        errors:
          type: array
          items: {$ref: '#/components/schemas/BatchItemError'}
    BatchItemError:
      type: object
      additionalProperties: false
      required: [requestId, code, family, status, message]
      properties:
        requestId: {$ref: '#/components/schemas/RequestId'}
        code: {$ref: '#/components/schemas/ErrorCode'}
        family: {$ref: '#/components/schemas/ErrorFamily'}
        message: {type: string}
        status: {type: integer, minimum: 400, maximum: 599}
        details: {type: object}
        resourceId: {type: string}
    ErrorCode: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}
    ErrorFamily: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}
`
}

func TestCanonicalBatchUnionAndFlatErrors(t *testing.T) {
	for _, union := range []string{"oneOf", "anyOf"} {
		for _, path := range []string{"/endpoint-groups-manage", "/endpoints-manage", "/routes-manage", "/auth-links-batch"} {
			source := strings.ReplaceAll(canonicalBatchSource(union), "/endpoint-groups-manage", path)
			for _, rule := range []string{"portos.batch-contract", "portos.batch-outcomes"} {
				t.Run(union+path+rule, func(t *testing.T) {
					options := canonicalBatchOptions
					if rule == "portos.batch-contract" {
						options = "items-field: operations\nitem-id-field: requestId"
					}
					if findings := checkAPI(t, rule, source, options); len(findings) != 0 {
						t.Fatalf("canonical contract: %#v", findings)
					}
				})
			}
		}
	}
}

func TestBatchUnionRejectsAnyInvalidAlternative(t *testing.T) {
	for _, union := range []string{"oneOf", "anyOf"} {
		for _, test := range []struct{ name, old, new string }{
			{"missing declared id", "        requestId: {type: string, minLength: 1}", "        otherId: {type: string, minLength: 1}"},
			{"nullable id", "        requestId: {type: string, minLength: 1}", "        requestId: {type: string, nullable: true}"},
			{"wrong id type", "        requestId: {type: string, minLength: 1}", "        requestId: {type: integer}"},
			{"missing required id", "required: [requestId, operation, id]", "required: [operation, id]"},
			{"scalar alternative", "    Delete:\n      type: object", "    Delete:\n      type: string"},
			{"unconstrained alternative", "        - {$ref: '#/components/schemas/Delete'}", "        - {}"},
			{"empty union", "      " + union + ":\n        - {$ref: '#/components/schemas/Create'}\n        - {$ref: '#/components/schemas/Delete'}", "      " + union + ": []"},
		} {
			t.Run(union+"/"+test.name, func(t *testing.T) {
				source := strings.Replace(canonicalBatchSource(union), test.old, test.new, 1)
				findings := checkAPI(t, "portos.batch-outcomes", source, canonicalBatchOptions)
				if len(findings) != 1 || !strings.Contains(findings[0].Pointer, "responses/200/content/application~1json") {
					t.Fatalf("invalid alternative accepted or unlocated: %#v", findings)
				}
				if test.name != "missing required id" {
					if findings := checkAPI(t, "portos.batch-contract", source, "items-field: operations\nitem-id-field: requestId"); len(findings) != 1 {
						t.Fatalf("invalid batch item accepted: %#v", findings)
					}
				}
			})
		}
	}
}

func TestFlatBatchErrorAndOutcomeFailures(t *testing.T) {
	for _, test := range []struct{ name, old, new string }{
		{"missing errors", "required: [results, errors]", "required: [results]"},
		{"nonempty errors", "        errors:\n          type: array", "        errors:\n          type: array\n          minItems: 1"},
		{"result id mismatch", "              requestId: {$ref: '#/components/schemas/RequestId'}", "              requestId: {type: integer}"},
		{"failure id mismatch", "        requestId: {$ref: '#/components/schemas/RequestId'}\n        code:", "        requestId: {type: integer}\n        code:"},
		{"failure id not required", "required: [requestId, code, family, status, message]", "required: [code, family, status, message]"},
		{"code not required", "required: [requestId, code, family, status, message]", "required: [requestId, family, status, message]"},
		{"family not required", "required: [requestId, code, family, status, message]", "required: [requestId, code, status, message]"},
		{"message not required", "required: [requestId, code, family, status, message]", "required: [requestId, code, family, status]"},
		{"code type", "ErrorCode: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}", "ErrorCode: {type: integer, enum: [400, 500]}"},
		{"code enum", "ErrorCode: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}", "ErrorCode: {type: string, enum: [400, 500]}"},
		{"family type", "ErrorFamily: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}", "ErrorFamily: {type: integer, enum: [400, 500]}"},
		{"family enum", "ErrorFamily: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}", "ErrorFamily: {type: string, enum: [NOT_FOUND, 500]}"},
		{"family unconstrained", "ErrorFamily: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}", "ErrorFamily: {type: string}"},
		{"family empty", "ErrorFamily: {type: string, enum: [NOT_FOUND, PRECONDITION_FAILED]}", "ErrorFamily: {type: string, enum: []}"},
		{"message type", "message: {type: string}", "message: {type: integer}"},
		{"fixed message", "message: {type: string}", "message: {type: string, enum: [Failed]}"},
		{"unnamed error", "items: {$ref: '#/components/schemas/BatchItemError'}", "items: {type: object, required: [requestId, code, family, message], properties: {requestId: {type: string}, code: {type: string}, family: {type: string, enum: [BAD_REQUEST]}, message: {type: string}}}"},
		{"nested response union malformed", "schema: {$ref: '#/components/schemas/Response'}", "schema: {anyOf: [{$ref: '#/components/schemas/Response'}, {type: string}]}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(canonicalBatchSource("oneOf"), test.old, test.new, 1)
			if source == canonicalBatchSource("oneOf") {
				t.Fatal("mutation did not apply")
			}
			if findings := checkAPI(t, "portos.batch-outcomes", source, canonicalBatchOptions); len(findings) != 1 {
				t.Fatalf("malformed outcome accepted: %#v", findings)
			}
		})
	}
}

func TestFlatBatchLayoutIsExplicitAndStrict(t *testing.T) {
	source := canonicalBatchSource("oneOf")
	for _, options := range []string{"items-field: operations\nitem-id-field: requestId\nerror-schema: BatchItemError", strings.ReplaceAll(canonicalBatchOptions, "true", "false")} {
		if findings := checkAPI(t, "portos.batch-outcomes", source, options); len(findings) != 1 {
			t.Fatalf("flat layout silently enabled: %#v", findings)
		}
	}
	for _, options := range []string{"flat-error-layout: yes-please", "flat-error-layout: []"} {
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(options), &node); err != nil {
			t.Fatal(err)
		}
		if _, err := apiFactory("portos.batch-outcomes")(*node.Content[0]); err == nil {
			t.Fatal("invalid layout option accepted")
		}
	}
	var node yaml.Node
	_ = yaml.Unmarshal([]byte("flat-error-layout: true"), &node)
	if _, err := apiFactory("portos.batch-contract")(*node.Content[0]); err == nil {
		t.Fatal("layout option leaked to other rules")
	}
}

func TestBatchComposedPropertyDoesNotHideInvalidStringConstraints(t *testing.T) {
	for _, field := range []string{"code", "family", "message"} {
		old := fmt.Sprintf("        %s: ", field)
		replacement := fmt.Sprintf("        %s: {anyOf: [{type: string, enum: [VALID]}, {type: integer}]} #", field)
		source := strings.Replace(canonicalBatchSource("anyOf"), old, replacement, 1)
		if findings := checkAPI(t, "portos.batch-outcomes", source, canonicalBatchOptions); len(findings) != 1 {
			t.Fatalf("%s invalid alternative hidden: %#v", field, findings)
		}
	}
}

func TestBatchConstraintsCheckEverySameTypeAlternative(t *testing.T) {
	for _, field := range []string{"code", "family", "message"} {
		old := fmt.Sprintf("        %s: ", field)
		value := "{anyOf: [{type: string, enum: [VALID]}, {type: string, enum: [404]}]}"
		if field == "message" {
			value = "{anyOf: [{type: string}, {type: string, enum: [Fixed]}]}"
		}
		source := strings.Replace(canonicalBatchSource("oneOf"), old, fmt.Sprintf("        %s: %s #", field, value), 1)
		if findings := checkAPI(t, "portos.batch-outcomes", source, canonicalBatchOptions); len(findings) != 1 {
			t.Fatalf("%s invalid same-type alternative hidden: %#v", field, findings)
		}
	}
}

func TestCanonicalBatchCrossFileUnion(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			root := t.TempDir()
			source := canonicalBatchSource("oneOf")
			if invalid {
				source = strings.Replace(source, "required: [requestId, operation, id]", "required: [operation, id]", 1)
			}
			var document map[string]any
			if err := yaml.Unmarshal([]byte(source), &document); err != nil {
				t.Fatal(err)
			}
			schemas, err := yaml.Marshal(map[string]any{"components": document["components"]})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "schemas.yaml"), schemas, 0600); err != nil {
				t.Fatal(err)
			}
			delete(document, "components")
			main, err := yaml.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			main = []byte(strings.ReplaceAll(string(main), "#/components/schemas/", "schemas.yaml#/components/schemas/"))
			filename := filepath.Join(root, "api.yaml")
			if err := os.WriteFile(filename, main, 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := linter.LoadDocument(context.Background(), filename, root)
			if err != nil {
				t.Fatal(err)
			}
			registry := NewRegistry()
			if err := RegisterStock(registry); err != nil {
				t.Fatal(err)
			}
			var options yaml.Node
			if err := yaml.Unmarshal([]byte(canonicalBatchOptions), &options); err != nil {
				t.Fatal(err)
			}
			program, err := registry.Compile(Pack{Version: 1, Rules: []Rule{{ID: "company.batch", Check: "portos.batch-outcomes", Options: *options.Content[0]}}})
			if err != nil {
				t.Fatal(err)
			}
			findings, err := program.Run(context.Background(), root, []*linter.Document{doc})
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if invalid {
				want = 1
			}
			if len(findings) != want {
				t.Fatalf("cross-file findings: %#v", findings)
			}
			if invalid && (findings[0].Pointer != "#/paths/~1endpoint-groups-manage/post/responses/200/content/application~1json" || findings[0].Line == 0) {
				t.Fatalf("unlocated cross-file diagnostic: %#v", findings)
			}
		})
	}
}

func TestBatchOuterUnionsPreserveCommonArrayItems(t *testing.T) {
	for _, union := range []string{"oneOf", "anyOf"} {
		source := canonicalBatchSource(union)
		requestStart, responseStart := strings.Index(source, "    Request:\n"), strings.Index(source, "    Operation:\n")
		request := strings.Replace(source[requestStart:responseStart], "    Request:\n", "    RequestCopy:\n", 1)
		resultStart, errorStart := strings.Index(source, "    Response:\n"), strings.Index(source, "    BatchItemError:\n")
		response := strings.Replace(source[resultStart:errorStart], "    Response:\n", "    ResponseCopy:\n", 1)
		source += request + response
		source = strings.Replace(source, "schema: {$ref: '#/components/schemas/Request'}", "schema: {"+union+": [{$ref: '#/components/schemas/Request'}, {$ref: '#/components/schemas/RequestCopy'}]}", 1)
		source = strings.Replace(source, "schema: {$ref: '#/components/schemas/Response'}", "schema: {"+union+": [{$ref: '#/components/schemas/Response'}, {$ref: '#/components/schemas/ResponseCopy'}]}", 1)
		if findings := checkAPI(t, "portos.batch-outcomes", source, canonicalBatchOptions); len(findings) != 0 {
			t.Fatalf("%s valid envelope alternatives: %#v", union, findings)
		}
		invalid := strings.Replace(source, response, strings.Replace(response, "        errors:\n          type: array", "        errors:\n          type: array\n          minItems: 1", 1), 1)
		if findings := checkAPI(t, "portos.batch-outcomes", invalid, canonicalBatchOptions); len(findings) != 1 {
			t.Fatalf("%s nonempty errors hidden by another envelope: %#v", union, findings)
		}

	}
}
