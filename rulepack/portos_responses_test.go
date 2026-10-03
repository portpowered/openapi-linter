package rulepack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

const portosErrorSchemas = `components:
  schemas:
    ErrorResponse:
      type: object
      required: [code, message, type, family]
      properties:
        code: {type: string}
        message: {type: string}
        type: {type: string}
        family: {type: integer, enum: [400, 500], x-enum-varnames: [CLIENT_ERROR, SERVER_ERROR]}
    Sort:
      type: object
      additionalProperties: false
      required: [direction, key]
      properties:
        direction: {type: string, enum: [ASCENDING, DESCENDING], x-enum-varnames: [ASCENDING, DESCENDING]}
        key: {type: string}
`
const sharedError = `{description: Failed, content: {application/json: {schema: {$ref: '#/components/schemas/ErrorResponse'}}}}`
const conditionalHeaders = `parameters: [{name: If-Match, in: header, schema: {type: string}}], `
const sortBody = `requestBody: {content: {application/json: {schema: {type: object, properties: {sorts: {type: array, items: {$ref: '#/components/schemas/Sort'}}}}}}}`
const batchRequest = `requestBody: {content: {application/json: {schema: {type: object, required: [items], properties: {items: {type: array, items: {type: object, required: [id], properties: {id: {type: string}}}}}}}}}`
const batchResponse = `responses: {'200': {content: {application/json: {schema: {type: object, required: [results, errors], properties: {results: {type: array, items: {type: object, required: [id], properties: {id: {type: string}}}}, errors: {type: array, items: {type: object, required: [id, error], properties: {id: {type: string}, error: {$ref: '#/components/schemas/ErrorResponse'}}}}}}}}}}`

func responsePolicySource(path, method, id, extra string) string {
	return apiHeader + fmt.Sprintf("paths: {'%s': {%s: {operationId: %s, %s}}}\n", path, method, id, extra) + portosErrorSchemas
}

func TestPortosResponsePolicyBehavior(t *testing.T) {
	cases := []struct {
		name, id, path, method, operation string
		findings                          int
	}{
		{"200", "success-status", "/v2/groups", "get", "responses: {'200': {description: OK}}", 0},
		{"202", "success-status", "/v2/groups", "post", "responses: {'202': {description: Accepted}}", 0},
		{"both plus errors", "success-status", "/v2/groups", "post", "responses: {'200': {}, '202': {}, '400': {}}", 0},
		{"201 forbidden", "success-status", "/v2/groups", "post", "responses: {'201': {}}", 2},
		{"204 forbidden", "success-status", "/v2/groups", "delete", "responses: {'200': {}, '204': {}}", 1},
		{"wildcard forbidden", "success-status", "/v2/groups", "get", "responses: {'2XX': {}}", 2},
		{"success missing", "success-status", "/v2/groups", "get", "responses: {'400': {}}", 1},
		{"ETag conflict", "etag-conflict", "/v2/groups/{id}", "patch", conditionalHeaders + "responses: {'200': {}, '409': {}}", 0},
		{"ETag missing conflict", "etag-conflict", "/v2/groups/{id}", "patch", conditionalHeaders + "responses: {'200': {}}", 1},
		{"ETag 412", "etag-conflict", "/v2/groups/{id}", "patch", conditionalHeaders + "responses: {'200': {}, '409': {}, '412': {}}", 1},
		{"unconditional 412", "etag-conflict", "/v2/groups/{id}", "patch", "responses: {'200': {}, '412': {}}", 0},
		{"If None Match", "etag-conflict", "/v2/groups/{id}", "patch", strings.ReplaceAll(conditionalHeaders, "If-Match", "if-none-match") + "responses: {'409': {}}", 0},
		{"query parameter is not header", "etag-conflict", "/v2/groups/{id}", "patch", strings.ReplaceAll(conditionalHeaders, "in: header", "in: query") + "responses: {'200': {}}", 0},
		{"shared errors", "error-contract", "/v2/groups", "get", "responses: {'200': {}, '400': " + sharedError + ", '4XX': " + sharedError + ", '500': " + sharedError + ", default: " + sharedError + "}", 0},
		{"inline error rejected", "error-contract", "/v2/groups", "get", "responses: {'400': {content: {application/json: {schema: {type: object}}}}}", 1},
		{"error body missing", "error-contract", "/v2/groups", "get", "responses: {'409': {description: Conflict}}", 1},
		{"success body not error", "error-contract", "/v2/groups", "get", "responses: {'200': {content: {application/json: {schema: {type: string}}}}}", 0},
		{"kebab", "path-kebab-case", "/v2/my-groups/{groupId}", "get", "responses: {'200': {}}", 0},
		{"capitalized", "path-kebab-case", "/v2/MyGroups", "get", "responses: {'200': {}}", 1},
		{"underscore", "path-kebab-case", "/v2/my_groups", "get", "responses: {'200': {}}", 1},
		{"major prefix", "version-prefix", "/v3/groups", "get", "responses: {'200': {}}", 0},
		{"major root", "version-prefix", "/v12", "get", "responses: {'200': {}}", 0},
		{"missing prefix", "version-prefix", "/groups", "get", "responses: {'200': {}}", 1},
		{"minor prefix", "version-prefix", "/v2.1/groups", "get", "responses: {'200': {}}", 1},
		{"delete explicit", "delete-idempotent", "/v2/groups/{id}", "delete", "x-portos-idempotent: true, responses: {'200': {}}", 0},
		{"delete missing declaration", "delete-idempotent", "/v2/groups/{id}", "delete", "responses: {'200': {}}", 1},
		{"delete absent failures", "delete-idempotent", "/v2/groups/{id}", "delete", "x-portos-idempotent: true, responses: {'200': {}, '404': {}, '410': {}}", 2},
		{"get can be missing", "delete-idempotent", "/v2/groups/{id}", "get", "responses: {'404': {}}", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := "GroupGet"
			if c.method == "delete" {
				id = "GroupDelete"
			}
			if got := len(checkAPI(t, "portos."+c.id, responsePolicySource(c.path, c.method, id, c.operation), "")); got != c.findings {
				t.Fatalf("got %d want %d", got, c.findings)
			}
		})
	}
}

func TestPortosSortAndBatchContracts(t *testing.T) {
	for _, c := range []struct {
		name, id, op string
		findings     int
	}{
		{"sort objects", "query-sorts", sortBody + ", responses: {'200': {}}", 0},
		{"sort omitted", "query-sorts", "requestBody: {content: {application/json: {schema: {type: object}}}}", 0},
		{"sort primitive", "query-sorts", strings.ReplaceAll(sortBody, "{$ref: '#/components/schemas/Sort'}", "{type: string}"), 1},
		{"sort scalar", "query-sorts", strings.ReplaceAll(sortBody, "type: array", "type: string"), 1},
		{"batch outcomes", "batch-outcomes", batchRequest + ", " + batchResponse, 0},
		{"batch forbids empty errors", "batch-outcomes", batchRequest + ", " + strings.ReplaceAll(batchResponse, "errors: {type: array", "errors: {type: array, minItems: 1"), 1},
		{"batch missing errors", "batch-outcomes", batchRequest + ", " + strings.ReplaceAll(batchResponse, "required: [results, errors]", "required: [results]"), 1},
		{"batch id mismatch", "batch-outcomes", batchRequest + ", " + strings.ReplaceAll(batchResponse, "id: {type: string}", "id: {type: integer}"), 1},
		{"batch error inline", "batch-outcomes", batchRequest + ", " + strings.ReplaceAll(batchResponse, "{$ref: '#/components/schemas/ErrorResponse'}", "{type: object}"), 1},
		{"batch missing response", "batch-outcomes", batchRequest + ", responses: {'200': {}}", 1},
		{"batch missing request", "batch-outcomes", batchResponse, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			operation := "GroupQuery"
			if c.id == "batch-outcomes" {
				operation = "GroupBatchModify"
			}
			if got := len(checkAPI(t, "portos."+c.id, responsePolicySource("/v2/groups", "post", operation, c.op), "")); got != c.findings {
				t.Fatalf("got %d want %d", got, c.findings)
			}
		})
	}
	async := responsePolicySource("/v2/groups", "post", "GroupBatchModifyAsync", "responses: {'202': {}}")
	if got := checkAPI(t, "portos.batch-outcomes", async, ""); len(got) != 0 {
		t.Fatalf("async incorrectly requires synchronous outcomes %#v", got)
	}
	for _, replace := range []struct{ from, to string }{
		{"enum: [ASCENDING, DESCENDING]", "enum: [ASCENDING, DESCEDNGING]"},
		{"required: [direction, key]", "required: [direction]"},
		{"additionalProperties: false", "additionalProperties: true"},
		{"key: {type: string}", "key: {type: integer}"},
		{"key: {type: string}", "key: {type: string}\n        extra: {type: string}"},
	} {
		source := strings.ReplaceAll(responsePolicySource("/v2/groups", "post", "GroupQuery", sortBody), replace.from, replace.to)
		if got := len(checkAPI(t, "portos.query-sorts", source, "")); got != 1 {
			t.Fatalf("sort mutation %v: %d", replace, got)
		}
	}
}

func TestPortosSharedErrorShapeAndProtocolEnums(t *testing.T) {
	source := responsePolicySource("/v2/groups", "get", "GroupGet", "responses: {'400': "+sharedError+"}")
	for _, c := range []struct{ from, to string }{
		{"required: [code, message, type, family]", "required: [code, message, type]"},
		{"code: {type: string}", "code: {type: integer}"},
		{"message: {type: string}", "message: {type: string, enum: [Fixed]}"},
		{"type: {type: string}", "type: {type: integer}"},
		{"family: {type: integer, enum: [400, 500]", "family: {type: string, enum: [400, 500]"},
		{"enum: [400, 500]", "enum: ['400', '500']"},
		{"enum: [400, 500]", "enum: [400, 409, 500]"},
	} {
		if got := len(checkAPI(t, "portos.error-contract", strings.ReplaceAll(source, c.from, c.to), "")); got != 1 {
			t.Fatalf("error mutation %v: %d", c, got)
		}
	}
	if got := checkAPI(t, "portos.open-enums", source, ""); len(got) != 0 {
		t.Fatalf("closed protocol enums conflict with business enum policy %#v", got)
	}
	for _, c := range []struct{ from, to string }{
		{"enum: [ASCENDING, DESCENDING]", "enum: [ASCENDING, ASCENDING]"},
		{"x-enum-varnames: [ASCENDING, DESCENDING]", "x-enum-varnames: [ASCENDING]"},
		{"x-enum-varnames: [ASCENDING, DESCENDING]", "x-enum-varnames: [ASCENDING, ASCENDING]"},
		{"x-enum-varnames: [ASCENDING, DESCENDING]", "x-enum-varnames: [bad-name, DESCENDING]"},
	} {
		if got := checkAPI(t, "portos.open-enums", strings.ReplaceAll(source, c.from, c.to), ""); len(got) == 0 {
			t.Fatalf("invalid closed enum escaped: %v", c)
		}
	}
}

func TestPortosPolicyCompositionAndOptions(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := linter.LoadDocument(context.Background(), filepath.Join(root, "examples/portos/api.yaml"), root)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	if err := RegisterStock(r); err != nil {
		t.Fatal(err)
	}
	pack, ok := Preset("portos-defaults")
	if !ok {
		t.Fatal("missing Portos defaults")
	}
	program, err := r.Compile(pack)
	if err != nil {
		t.Fatal(err)
	}
	if findings, err := program.Run(context.Background(), root, []*linter.Document{doc}); err != nil || len(findings) != 0 {
		t.Fatalf("composed fixture: %#v %v", findings, err)
	}
	for _, id := range []string{"portos.error-contract", "portos.query-sorts", "portos.batch-outcomes"} {
		for _, key := range optionNames(id) {
			var options yaml.Node
			if err := yaml.Unmarshal([]byte(key+": ''"), &options); err != nil {
				t.Fatal(err)
			}
			if _, err := apiFactory(id)(*options.Content[0]); err == nil {
				t.Fatalf("blank %s allowed for %s", key, id)
			}
		}
	}
	source := strings.ReplaceAll(responsePolicySource("/v2/groups", "get", "GroupGet", "responses: {'400': "+sharedError+"}"), "ErrorResponse", "CompanyError")
	if got := checkAPI(t, "portos.error-contract", source, "error-schema: CompanyError"); len(got) != 0 {
		t.Fatalf("custom schema ignored %#v", got)
	}
	if got := checkAPI(t, "portos.query-sorts", strings.ReplaceAll(responsePolicySource("/v2/groups", "post", "GroupQuery", sortBody), "sorts:", "orderBy:"), "sorts-field: orderBy"); len(got) != 0 {
		t.Fatalf("custom sort field ignored %#v", got)
	}
}

func TestPortosResponseSourceBoundaries(t *testing.T) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(responsePolicySource("/v2/groups", "get", "GroupGet", "responses: {'400': {content: {application/json: {schema: {$ref: 'https://example.invalid/ErrorResponse.yaml'}}}}}")), &node); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	if err := os.WriteFile(file, []byte("unused"), 0600); err != nil {
		t.Fatal(err)
	}
	check := apiCheck{id: "portos.error-contract", options: optionsFor("portos.error-contract")}
	pass := linter.NewPass([]*linter.Document{{Path: file, Root: &node, Model: &v3.Document{}}})
	pass.Root = root
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("remote error schema reference accepted")
	}
	pass = linter.NewPass([]*linter.Document{{Path: file, Model: &v3.Document{}}})
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("missing source accepted")
	}
	pass = linter.NewPass([]*linter.Document{{Path: file}})
	check.Analyze(context.Background(), pass)
	if pass.Err() != nil || len(pass.Diagnostics()) != 0 {
		t.Fatal("standalone schema analyzed as request paths")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{{Path: file, Root: &node, Model: &v3.Document{}}})
	check.Analyze(ctx, pass)
	if len(pass.Diagnostics()) != 0 || pass.Err() != nil {
		t.Fatal("canceled analysis ran")
	}
}

func TestPortosNestedComposedSortsAndBatchIDs(t *testing.T) {
	source := responsePolicySource("/v2/groups", "post", "GroupQuery", sortBody)
	source = strings.Replace(source, "properties: {sorts: {type: array, items: {$ref: '#/components/schemas/Sort'}}}", "properties: {query: {$ref: '#/components/schemas/Nested'}}", 1)
	source += "    Nested:\n      type: object\n      properties:\n        query: {$ref: '#/components/schemas/Nested'}\n        sorts: {type: array, items: {$ref: '#/components/schemas/Sort'}}\n"
	if got := checkAPI(t, "portos.query-sorts", source, ""); len(got) != 0 {
		t.Fatalf("recursive sorts %#v", got)
	}
	if got := checkAPI(t, "portos.query-sorts", strings.ReplaceAll(source, "sorts: {type: array", "sorts: {type: string"), ""); len(got) != 1 {
		t.Fatalf("nested invalid sorts %#v", got)
	}
	source = responsePolicySource("/v2/groups", "post", "GroupQuery", sortBody)
	source = strings.Replace(source, "      required: [direction, key]", "      allOf:\n        - required: [direction, key]\n        - required: [direction]\n        - {$ref: '#/components/schemas/Sort'}", 1)
	if got := checkAPI(t, "portos.query-sorts", source, ""); len(got) != 0 {
		t.Fatalf("composed required fields %#v", got)
	}
	source = strings.ReplaceAll(source, "additionalProperties: false", "unevaluatedProperties: false")
	if got := checkAPI(t, "portos.query-sorts", source, ""); len(got) != 0 {
		t.Fatalf("closed composed fields %#v", got)
	}
	for _, mutate := range []struct{ from, to string }{
		{"required: [items]", "required: []"},
		{"required: [id]", "required: []"},
		{"id: {type: string}", "id: {type: boolean}"},
		{"items: {type: array", "items: {type: object"},
	} {
		source = responsePolicySource("/v2/groups", "post", "GroupBatchModify", strings.ReplaceAll(batchRequest, mutate.from, mutate.to)+", "+batchResponse)
		if got := checkAPI(t, "portos.batch-outcomes", source, ""); len(got) != 1 {
			t.Fatalf("invalid request ID contract %v: %#v", mutate, got)
		}
	}
	// ID type consistency is checked across every request representation.
	source = responsePolicySource("/v2/groups", "post", "GroupBatchModify", batchRequest+", "+batchResponse)
	var document, other map[string]any
	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(strings.ReplaceAll(batchRequest, "id: {type: string}", "id: {type: integer}")), &other); err != nil {
		t.Fatal(err)
	}
	content := document["paths"].(map[string]any)["/v2/groups"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)["content"].(map[string]any)
	content["application/yaml"] = other["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"]
	data, err := yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if got := checkAPI(t, "portos.batch-outcomes", string(data), ""); len(got) != 1 {
		t.Fatalf("mixed request ID types %#v", got)
	}
	source = responsePolicySource("/v2/groups", "post", "GroupBatchModify", strings.ReplaceAll(batchRequest+", "+batchResponse, "id: {type: string}", "id: {type: integer}"))
	if got := checkAPI(t, "portos.batch-outcomes", source, ""); len(got) != 0 {
		t.Fatalf("consistent integer IDs %#v", got)
	}
}

func TestPortosResponsePoliciesIgnoreCallbacksAndCheckEmptyPaths(t *testing.T) {
	for _, id := range []string{"success-status", "etag-conflict", "error-contract", "delete-idempotent", "query-sorts", "batch-outcomes"} {
		source := apiHeader + `paths: {}
webhooks:
  Event:
    delete:
      operationId: GroupBatchQueryDelete
      parameters: [{name: If-Match, in: header, schema: {type: string}}]
      responses: {'204': {}, '404': {}}
`
		if got := checkAPI(t, "portos."+id, source, ""); len(got) != 0 {
			t.Fatalf("webhook policy %s: %#v", id, got)
		}
	}
	source := apiHeader + "paths: {'/v2/BadPath': {}, '/v3/groups': {}, x-extra: {}}\n"
	if got := checkAPI(t, "portos.path-kebab-case", source, ""); len(got) != 1 {
		t.Fatalf("empty path names %#v", got)
	}
	if got := checkAPI(t, "portos.version-prefix", source, ""); len(got) != 0 {
		t.Fatalf("empty versioned paths %#v", got)
	}
	source = responsePolicySource("/v2/groups", "post", "GroupModify", "responses: {'200': {}, '409': {}}")
	source = strings.Replace(source, "{post: {operationId", "{parameters: [{name: IF-MATCH, in: header, schema: {type: string}}], post: {operationId", 1)
	if got := checkAPI(t, "portos.etag-conflict", source, ""); len(got) != 0 {
		t.Fatalf("inherited ETag %#v", got)
	}
	if got := checkAPI(t, "portos.etag-conflict", strings.ReplaceAll(source, "'409': {}", "'412': {}"), ""); len(got) != 2 {
		t.Fatalf("inherited ETag conflict mismatch %#v", got)
	}
}

func TestPortosResponseLocalReferenceAttribution(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	source := responsePolicySource("/v2/groups", "get", "GroupGet", "responses: {'400': "+sharedError+"}")
	source = strings.Replace(source, "$ref: '#/components/schemas/ErrorResponse'", "$ref: 'ErrorResponse.yaml'", 1)
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ErrorResponse.yaml"), []byte("type: string\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(source), &node); err != nil {
		t.Fatal(err)
	}
	p := linter.NewPass([]*linter.Document{{Path: file, Root: &node, Model: &v3.Document{}}})
	p.Root = root
	(apiCheck{id: "portos.error-contract", options: optionsFor("portos.error-contract")}).Analyze(context.Background(), p)
	if p.Err() != nil || len(p.Diagnostics()) != 1 || p.Diagnostics()[0].Path != file || !strings.Contains(p.Diagnostics()[0].Pointer, "responses/400") {
		t.Fatalf("local attribution: %#v %v", p.Diagnostics(), p.Err())
	}
}
