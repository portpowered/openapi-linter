package rulepack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

func TestWriteMethods_PatchIsAFinding(t *testing.T) {
	source := responsePolicySource("/groups/{id}", "patch", "GroupModify", "responses: {'200': {}}")
	findings := checkAPI(t, "portos.write-methods", source, "")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups~1{id}/patch" {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestWriteMethods_GetPostDeletePass(t *testing.T) {
	for _, method := range []string{"get", "post", "delete"} {
		if findings := checkAPI(t, "portos.write-methods", responsePolicySource("/groups", method, "GroupGet", "responses: {'200': {}}"), ""); len(findings) != 0 {
			t.Fatalf("%s: %#v", method, findings)
		}
	}
}

func TestUpsertHeaders_CreateNeedsIdempotencyKey(t *testing.T) {
	source := responsePolicySource("/groups", "post", "GroupUpsert", "responses: {'200': {}}")
	findings := checkAPI(t, "portos.upsert-headers", source, "")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups/post" {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestUpsertHeaders_ReplaceAndDeleteNeedIfMatch(t *testing.T) {
	for _, method := range []string{"post", "delete"} {
		source := responsePolicySource("/groups/{id}", method, "GroupUpsert", "parameters: [{name: Idempotency-Key, in: header, required: true}], responses: {'200': {}}")
		findings := checkAPI(t, "portos.upsert-headers", source, "")
		if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups~1{id}/"+method || !strings.Contains(findings[0].Message, "If-Match") {
			t.Fatalf("%s: %#v", method, findings)
		}
	}
}

func TestUpsertHeaders_DeclaredHeadersPass(t *testing.T) {
	source := responsePolicySource("/groups/{id}", "post", "GroupUpsert", "parameters: [{name: If-Match, in: header, required: true}, {name: Replay-Key, in: header, required: true}], responses: {'200': {}}")
	if findings := checkAPI(t, "portos.upsert-headers", source, "idempotency-header: Replay-Key"); len(findings) != 0 {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestPathPatterns_UnexpectedPathIsAFinding(t *testing.T) {
	source := responsePolicySource("/groups/rename", "post", "GroupUpsert", "responses: {'200': {}}")
	findings := checkAPI(t, "portos.path-patterns", source, "patterns: ['^/groups(?:/\\{id\\})?$']")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups~1rename" {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestPathPatterns_CollectionAndItemPass(t *testing.T) {
	for _, path := range []string{"/groups", "/groups/{id}"} {
		if findings := checkAPI(t, "portos.path-patterns", responsePolicySource(path, "get", "GroupGet", "responses: {'200': {}}"), "patterns: ['^/groups(?:/\\{id\\})?$']"); len(findings) != 0 {
			t.Fatalf("%s: %#v", path, findings)
		}
	}
}

const messageRequestResponse = `responses: {'200': {content: {application/json: {schema: {type: object, required: [results, errors, status], properties: {results: {type: array, items: {type: object}}, errors: {type: array, items: {type: object}}, status: {type: string, enum: [QUEUED, IN_PROGRESS, SUCCEEDED, PARTIALLY_SUCCEEDED, FAILED]}}}}}}}`

func TestMessageRequest_MissingStatusEnumValue(t *testing.T) {
	source := responsePolicySource("/message-requests", "post", "MessageRequestUpsert", strings.Replace(messageRequestResponse, "PARTIALLY_SUCCEEDED, ", "", 1))
	findings := checkAPI(t, "portos.message-request", source, "")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1message-requests/post/responses/200/content/application~1json/schema/properties/status" {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestMessageRequest_CompleteEnvelopePasses(t *testing.T) {
	source := responsePolicySource("/message-requests", "post", "MessageRequestUpsert", messageRequestResponse)
	if findings := checkAPI(t, "portos.message-request", source, ""); len(findings) != 0 {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestEtagConflict_RequiresPreconditionStatuses(t *testing.T) {
	source := responsePolicySource("/groups/{id}", "post", "GroupUpsert", conditionalHeaders+"responses: {'409': {}}")
	findings := checkAPI(t, "portos.etag-conflict", source, "")
	if len(findings) != 2 {
		t.Fatalf("got %#v, want two missing precondition statuses", findings)
	}
	for i, status := range []string{"412", "428"} {
		if findings[i].Pointer != "#/paths/~1groups~1{id}/post" || !strings.Contains(findings[i].Message, status) {
			t.Fatalf("finding %d: %#v", i, findings[i])
		}
	}
}

func TestEtagConflict_AllowsInFlightConflictWithIdempotencyKey(t *testing.T) {
	headers := "parameters: [{name: If-Match, in: header}, {name: Idempotency-Key, in: header, required: true}], "
	source := responsePolicySource("/groups", "post", "GroupUpsert", headers+"responses: {'409': {}, '412': {}, '428': {}}")
	if findings := checkAPI(t, "portos.etag-conflict", source, ""); len(findings) != 0 {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestDeleteIdempotent_RequiresIdempotencyKeyAllows404(t *testing.T) {
	source := responsePolicySource("/groups/{id}", "delete", "GroupDelete", "responses: {'404': {}}")
	findings := checkAPI(t, "portos.delete-idempotent", source, "")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups~1{id}/delete" || !strings.Contains(findings[0].Message, "Idempotency-Key") {
		t.Fatalf("unexpected findings %#v", findings)
	}
	source = strings.Replace(source, "operationId: GroupDelete,", "operationId: GroupDelete, parameters: [{name: Idempotency-Key, in: header, required: true}],", 1)
	if findings := checkAPI(t, "portos.delete-idempotent", source, ""); len(findings) != 0 {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestUpsertHeaders_ComposedCollectionIDAndInheritedHeaders(t *testing.T) {
	source := apiHeader + `paths:
  /groups:
    parameters: [{$ref: '#/components/parameters/Replay'}]
    post:
      requestBody:
        content:
          application/json:
            schema:
              allOf: [{$ref: '#/components/schemas/Group'}]
      responses: {'200': {}}
components:
  parameters:
    Replay: {name: idempotency-key, in: header, required: true}
  schemas:
    Group: {type: object, properties: {id: {type: string}}}
`
	findings := checkAPI(t, "portos.upsert-headers", source, "")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups/post" || !strings.Contains(findings[0].Message, "If-Match") {
		t.Fatalf("unexpected findings %#v", findings)
	}
	source = strings.Replace(source, "    post:", "    post:\n      parameters: [{name: IF-MATCH, in: header}]", 1)
	if findings := checkAPI(t, "portos.upsert-headers", source, ""); len(findings) != 0 {
		t.Fatalf("inherited and case-insensitive headers: %#v", findings)
	}
}

func TestUpsertOptions_InvalidConfigurationsRejected(t *testing.T) {
	for _, test := range []struct{ id, options string }{
		{"portos.path-patterns", ""},
		{"portos.path-patterns", "patterns: []"},
		{"portos.path-patterns", "patterns: ['']"},
		{"portos.path-patterns", "patterns: ['groups']"},
		{"portos.path-patterns", "patterns: ['^[$']"},
		{"portos.message-request", "paths: []"},
		{"portos.message-request", "paths: ['[']"},
		{"portos.message-request", "statuses: []"},
		{"portos.message-request", "statuses: ['']"},
		{"portos.message-request", "statuses: [FAILED, FAILED]"},
		{"portos.upsert-headers", "idempotency-header: ' '"},
		{"portos.upsert-headers", "unknown: true"},
	} {
		var options yaml.Node
		if test.options != "" {
			if err := yaml.Unmarshal([]byte(test.options), &options); err != nil {
				t.Fatal(err)
			}
			options = *options.Content[0]
		}
		if _, err := apiFactory(test.id)(options); err == nil {
			t.Fatalf("accepted %s %s", test.id, test.options)
		}
	}
}

func TestMessageRequest_EnvelopeViolationsAndCustomOptions(t *testing.T) {
	for _, test := range []struct{ from, to, pointer string }{
		{"'200'", "'202'", "#/paths/~1message-requests/post"},
		{"type: array", "type: string", "#/paths/~1message-requests/post/responses/200/content/application~1json/schema"},
		{"[results, errors, status]", "[results, errors]", "#/paths/~1message-requests/post/responses/200/content/application~1json/schema/properties/status"},
		{"status: {type: string, enum: [QUEUED, IN_PROGRESS, SUCCEEDED, PARTIALLY_SUCCEEDED, FAILED]}", "other: {type: string}", "#/paths/~1message-requests/post/responses/200/content/application~1json/schema"},
	} {
		source := responsePolicySource("/message-requests", "post", "MessageRequestUpsert", strings.Replace(messageRequestResponse, test.from, test.to, 1))
		findings := checkAPI(t, "portos.message-request", source, "")
		if len(findings) != 1 || findings[0].Pointer != test.pointer {
			t.Fatalf("%s: %#v", test.from, findings)
		}
	}
	source := responsePolicySource("/custom/messages", "get", "MessageRequestGet", strings.Replace(messageRequestResponse, "QUEUED, IN_PROGRESS, SUCCEEDED, PARTIALLY_SUCCEEDED, FAILED", "WAITING, DONE", 1))
	if findings := checkAPI(t, "portos.message-request", source, "paths: ['^/custom/messages$']\nstatuses: [DONE, WAITING]"); len(findings) != 0 {
		t.Fatalf("custom paths/statuses: %#v", findings)
	}
	if findings := checkAPI(t, "portos.message-request", source, ""); len(findings) != 0 {
		t.Fatalf("unmatched path: %#v", findings)
	}
}

func TestUpsertChecks_IgnoreWebhookAndExtensionPaths(t *testing.T) {
	source := apiHeader + `paths: {x-extra: {}}
webhooks:
  Event:
    patch:
      responses: {'204': {}}
`
	for _, id := range []string{"portos.write-methods", "portos.upsert-headers", "portos.path-patterns", "portos.message-request"} {
		options := ""
		if id == "portos.path-patterns" {
			options = "patterns: ['^/groups$']"
		}
		if findings := checkAPI(t, id, source, options); len(findings) != 0 {
			t.Fatalf("%s: %#v", id, findings)
		}
	}
	if findings := checkAPI(t, "portos.upsert-headers", responsePolicySource("/groups", "get", "GroupGet", "responses: {'200': {}}"), ""); len(findings) != 0 {
		t.Fatalf("read headers: %#v", findings)
	}
}

func TestUpsertChecks_SourceBoundaries(t *testing.T) {
	check := apiCheck{id: "portos.write-methods"}
	pass := linter.NewPass([]*linter.Document{{Path: "api.yaml", Model: &v3.Document{}}})
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("missing source accepted")
	}
	pass = linter.NewPass([]*linter.Document{{Path: "api.yaml"}})
	check.Analyze(context.Background(), pass)
	if pass.Err() != nil || len(pass.Diagnostics()) != 0 {
		t.Fatal("standalone schema checked as API")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = linter.NewPass([]*linter.Document{{Path: "api.yaml", Model: &v3.Document{}}})
	check.Analyze(ctx, pass)
	if pass.Err() != nil || len(pass.Diagnostics()) != 0 {
		t.Fatal("cancelled analyzer ran")
	}
	var source yaml.Node
	if err := yaml.Unmarshal([]byte(apiHeader+"paths: {'/groups': {$ref: 'https://example.invalid/path.yaml'}}"), &source); err != nil {
		t.Fatal(err)
	}
	pass = linter.NewPass([]*linter.Document{{Path: "api.yaml", Root: &source, Model: &v3.Document{}}})
	check.Analyze(context.Background(), pass)
	if pass.Err() == nil {
		t.Fatal("remote reference accepted")
	}
}

func TestAPIReferenceFindings_BaselinesSurviveRelocation(t *testing.T) {
	for _, id := range []string{"portos.upsert-headers", "portos.etag-conflict", "portos.collection-response", "portos.write-methods"} {
		var previous string
		for i := 0; i < 2; i++ {
			root := t.TempDir()
			file := filepath.Join(root, "api.yaml")
			source := apiHeader + "paths: {'/groups': {$ref: 'groups.yaml'}}\n"
			for name, content := range map[string]string{"api.yaml": source, "groups.yaml": "post: {operationId: GroupList, parameters: [{name: If-Match, in: header}], responses: {'200': {}}}\noptions: {responses: {'200': {}}}\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var parsed yaml.Node
			if err := yaml.Unmarshal([]byte(source), &parsed); err != nil {
				t.Fatal(err)
			}
			pass := linter.NewPass([]*linter.Document{{Path: file, Root: &parsed, Model: &v3.Document{}}})
			pass.Root = root
			(apiCheck{id: id, options: optionsFor(id)}).Analyze(context.Background(), pass)
			findings := pass.Diagnostics()
			if pass.Err() != nil || len(findings) == 0 {
				t.Fatalf("%s: %#v %v", id, findings, pass.Err())
			}
			pointer := "#/paths/~1groups"
			if id == "portos.write-methods" {
				pointer += "/options"
			} else if id == "portos.upsert-headers" || id == "portos.etag-conflict" {
				pointer += "/post"
			}
			if findings[0].Pointer != pointer || findings[0].Line == 0 || strings.Contains(findings[0].Message, root) {
				t.Fatalf("non-portable attribution %#v", findings[0])
			}
			current, err := fingerprint(root, findings[0])
			if err != nil {
				t.Fatal(err)
			}
			if i > 0 && previous != current {
				t.Fatalf("%s fingerprint changed after relocation", id)
			}
			previous = current
		}
	}
}

func TestUpsertHeaders_RequiredAndConditionalParameters(t *testing.T) {
	for _, test := range []struct {
		name, path, method, parameters, body string
		want                                 int
	}{
		{"optional replay", "/groups", "post", "[{name: Idempotency-Key, in: header, required: false}]", "", 1},
		{"unspecified replay", "/groups", "post", "[{name: Idempotency-Key, in: header}]", "", 1},
		{"optional delete etag", "/groups/{id}", "delete", "[{name: Idempotency-Key, in: header, required: true}, {name: If-Match, in: header, required: false}]", "", 1},
		{"unspecified item etag", "/groups/{id}", "post", "[{name: Idempotency-Key, in: header, required: true}, {name: If-Match, in: header}]", "", 1},
		{"conditional collection etag", "/groups", "post", "[{name: Idempotency-Key, in: header, required: true}, {name: If-Match, in: header, required: false}]", "requestBody: {content: {application/json: {schema: {type: object, properties: {id: {type: string}}}}}}, ", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := responsePolicySource(test.path, test.method, "GroupUpsert", "parameters: "+test.parameters+", "+test.body+"responses: {'200': {}}")
			if findings := checkAPI(t, "portos.upsert-headers", source, ""); len(findings) != test.want {
				t.Fatalf("got %#v, want %d", findings, test.want)
			}
		})
	}
	source := apiHeader + `paths:
  /groups:
    parameters: [{name: Idempotency-Key, in: header, required: true}]
    post:
      parameters: [{name: idempotency-key, in: header, required: false}]
      responses: {'200': {}}
`
	if findings := checkAPI(t, "portos.upsert-headers", source, ""); len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups/post" {
		t.Fatalf("optional override bypassed required header: %#v", findings)
	}
}

func TestMessageRequest_RequiresObjectEnvelopeIncludingComposition(t *testing.T) {
	for _, body := range []string{
		strings.Replace(messageRequestResponse, "type: object, required: [results", "type: string, required: [results", 1),
		strings.Replace(messageRequestResponse, "type: object, required: [results", "required: [results", 1),
		strings.Replace(messageRequestResponse, "type: object, required: [results", "allOf: [{type: string}], required: [results", 1),
	} {
		findings := checkAPI(t, "portos.message-request", responsePolicySource("/message-requests", "post", "MessageRequestUpsert", body), "")
		if len(findings) != 1 || findings[0].Pointer != "#/paths/~1message-requests/post/responses/200/content/application~1json/schema" {
			t.Fatalf("invalid object envelope accepted: %#v", findings)
		}
	}
	body := strings.Replace(messageRequestResponse, "type: object, required: [results", "allOf: [{type: object}, {required: [results, errors, status]}], required: [results", 1)
	if findings := checkAPI(t, "portos.message-request", responsePolicySource("/message-requests", "post", "MessageRequestUpsert", body), ""); len(findings) != 0 {
		t.Fatalf("composed object envelope rejected: %#v", findings)
	}
}

func TestUpsertChecks_SharedReferenceSuppressionsStayOperationSpecific(t *testing.T) {
	for _, fragment := range []string{"first", "second"} {
		root := t.TempDir()
		file := filepath.Join(root, "api.yaml")
		source := apiHeader + "paths:\n  /alpha: {$ref: 'paths.yaml#/first'}\n  /beta: {$ref: 'paths.yaml#/" + fragment + "'}\n"
		item := `
  parameters: [{name: Idempotency-Key, in: header, required: true}]
  post:
    requestBody: {content: {application/json: {schema: {type: object, properties: {id: {type: string}}}}}}
    responses: {'200': {}}
  delete: {responses: {'200': {}}}
  options: {responses: {'200': {}}}
`
		for name, content := range map[string]string{"api.yaml": source, "paths.yaml": "first:" + item + "second:" + item} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		var parsed yaml.Node
		if err := yaml.Unmarshal([]byte(source), &parsed); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"portos.upsert-headers", "portos.message-request", "portos.write-methods"} {
			registry := NewRegistry()
			if err := RegisterStock(registry); err != nil {
				t.Fatal(err)
			}
			rule := Rule{ID: id, Check: id}
			method, want := "post", 3
			if id == "portos.message-request" {
				var options yaml.Node
				if err := yaml.Unmarshal([]byte("paths: ['^/(alpha|beta)$']"), &options); err != nil {
					t.Fatal(err)
				}
				rule.Options = *options.Content[0]
				want = 5 // OPTIONS is also a matching operation.
			} else if id == "portos.write-methods" {
				method, want = "options", 1
			}
			pack := Pack{Version: 1, Rules: []Rule{rule}, Suppressions: []Suppression{{Rule: id, Path: "api.yaml", Pointer: "#/paths/~1alpha/" + method, Reason: "only this operation is exempt"}}}
			program, err := registry.Compile(pack)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := program.Run(context.Background(), root, []*linter.Document{{Path: file, Source: []byte(source), Root: &parsed, Model: &v3.Document{}}})
			if err != nil || len(findings) != want {
				t.Fatalf("%s/%s: %#v %v", id, fragment, findings, err)
			}
			beta := false
			for _, finding := range findings {
				beta = beta || finding.Pointer == "#/paths/~1beta/"+method
				if finding.Pointer == "#/paths/~1alpha/"+method {
					t.Fatalf("suppressed operation was retained: %#v", finding)
				}
			}
			if !beta {
				t.Fatalf("%s/%s suppression hid beta: %#v", id, fragment, findings)
			}
		}
	}
}

func TestMessageRequest_SiblingSchemaReferenceUsesOperationPointer(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	source := apiHeader + "paths: {'/message-requests': {$ref: 'paths.yaml#/item'}}\n"
	item := `item:
  post:
    responses:
      '200':
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Result'}
components:
  schemas:
    Result: {type: string}
`
	for name, content := range map[string]string{"api.yaml": source, "paths.yaml": item} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal([]byte(source), &parsed); err != nil {
		t.Fatal(err)
	}
	pass := linter.NewPass([]*linter.Document{{Path: file, Source: []byte(source), Root: &parsed, Model: &v3.Document{}}})
	pass.Root = root
	(apiCheck{id: "portos.message-request", options: optionsFor("portos.message-request")}).Analyze(context.Background(), pass)
	findings := pass.Diagnostics()
	if pass.Err() != nil || len(findings) != 1 || findings[0].Pointer != "#/paths/~1message-requests/post" || !strings.Contains(findings[0].Message, "paths.yaml#/components/schemas/Result") {
		t.Fatalf("invalid sibling-schema attribution %#v %v", findings, pass.Err())
	}
}

func TestPreconditionAndDelete_SharedReferenceSuppressionsStayOperationSpecific(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "api.yaml")
	source := apiHeader + "paths: {'/alpha': {$ref: 'paths.yaml'}, '/beta': {$ref: 'paths.yaml'}}\n"
	item := "post: {parameters: [{name: If-Match, in: header}], responses: {'200': {}}}\ndelete: {responses: {'200': {}}}\n"
	for name, content := range map[string]string{"api.yaml": source, "paths.yaml": item} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal([]byte(source), &parsed); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id, method string
		want       int
	}{
		{"portos.etag-conflict", "post", 2},
		{"portos.delete-idempotent", "delete", 1},
	} {
		registry := NewRegistry()
		if err := RegisterStock(registry); err != nil {
			t.Fatal(err)
		}
		pack := Pack{Version: 1, Rules: []Rule{{ID: test.id, Check: test.id}}, Suppressions: []Suppression{{Rule: test.id, Path: "api.yaml", Pointer: "#/paths/~1alpha/" + test.method, Reason: "only alpha is exempt"}}}
		program, err := registry.Compile(pack)
		if err != nil {
			t.Fatal(err)
		}
		findings, err := program.Run(context.Background(), root, []*linter.Document{{Path: file, Source: []byte(source), Root: &parsed, Model: &v3.Document{}}})
		if err != nil || len(findings) != test.want {
			t.Fatalf("%s: %#v %v", test.id, findings, err)
		}
		for _, finding := range findings {
			if finding.Pointer != "#/paths/~1beta/"+test.method {
				t.Fatalf("shared source attribution lost beta: %#v", finding)
			}
		}
	}
}
