# Portos response and schema contracts

These eight independently selectable rules extend `portos:internal` and therefore `portos-defaults`. They use error severity. The configuration remains `version: 1`. Existing unversioned paths, alternative success statuses and inline error bodies now produce findings when customers activate Portos policy.

```sh
openapilint init --preset portos-defaults --output .openapilint.yaml
openapilint config explain --root . --path api.yaml
openapilint --root . api.yaml
```

## Rule checklist

- [x] `portos.success-status`: every request operation declares 200 or 202. Other 2xx statuses, including 201, 204 and `2XX`, are rejected. Error statuses remain available.
- [x] `portos.etag-conflict`: an operation declaring an `If-Match` or `If-None-Match` header, directly or at its Path Item, must declare 409. Header names are case insensitive. Such operations must not declare 412. This is the chosen Portos convention; other packs retain their own policy.
- [x] `portos.error-contract`: each declared 4xx, 5xx, wildcard `4XX`/`5XX`, or default response has a body schema referencing `ErrorResponse`. The shared object requires string `code`, `message`, and `type`, and integer `family` with enum `[400, 500]`. `message` must not be constrained by enum or const. Additional error metadata may be declared. The rule does not invent error statuses for operations without declared errors.
- [x] `portos.path-kebab-case`: literal request path segments use lowercase kebab-case. `/v2/my-groups/{groupId}` is valid; `/v2/MyGroups` and `/v2/my_groups` are not. Parameter names retain their own schema naming policy. Empty Path Items are checked too.
- [x] `portos.delete-idempotent`: DELETE methods and operations classified as Delete declare `x-portos-idempotent: true`. They must not declare 404 or 410 for absent resources. Successful repeated deletes use 200 or 202 under the success-status rule. This extension records an explicit contract; it is a local linter convention.
- [x] `portos.version-prefix`: request paths begin with a positive major version such as `/v2/groups` or `/v3/groups`. Minor, patch, beta, and missing prefixes fail this internal convention.
- [x] `portos.query-sorts`: when a Query request schema declares `sorts`, it is an array of objects with exactly `direction` and `key`, both required. `key` is a string; `direction` is a string restricted to `ASCENDING` or `DESCENDING`. Objects close extra fields with `additionalProperties: false` or `unevaluatedProperties: false`. Nested and referenced query objects are inspected, with recursion bounded. Queries may omit sorting.
- [x] `portos.batch-outcomes`: synchronous Batch successes declare required `results` and `errors` arrays. Both arrays allow empty lists. Every entry requires the same ID field and type as request items. Each failure additionally requires `error` referencing the same `ErrorResponse` object. Request items and their IDs are required; ID types are string or integer and agree across request representations. Async Batch retains its AsyncIdentifier response instead of these synchronous arrays.

Callbacks and webhooks are excluded from these request-operation policies. Local references and composed object fields are resolved within the existing root boundary. An unresolved or remote reference is an operational failure, not a suppressible policy finding.

## Shared objects

```yaml
ErrorResponse:
  type: object
  required: [code, message, type, family]
  properties:
    code: {type: string}
    message: {type: string}
    type: {type: string}
    family:
      type: integer
      enum: [400, 500]
      x-enum-varnames: [CLIENT_ERROR, SERVER_ERROR]
Sort:
  type: object
  additionalProperties: false
  required: [direction, key]
  properties:
    direction:
      type: string
      enum: [ASCENDING, DESCENDING]
      x-enum-varnames: [ASCENDING, DESCENDING]
    key: {type: string}
```

`portos.open-enums` permits these exact closed protocol discriminants, identified by their `direction`/`family` property names, types and values, with two unique identifier varnames. Other enums continue to require `x-extensible-enum` plus varnames. This avoids requiring an open sort enum that would admit unsupported directions. `DESCEDNGING` was confirmed as a typo; the wire value is `DESCENDING`.

A 409 example is `{code: ETAG_CONFLICT, message: Resource changed, type: etag.conflict, family: 400}`. Codes and types provide stable machine-readable classifications; messages describe the particular failure. The family is the coarse client/server class, not the specific HTTP status.

The complete [Portos fixture](../examples/portos/api.yaml) includes Query, shared errors, Sort, conditional Modify, idempotent Delete, and synchronous Batch request/response schemas.

## Options, overrides and rollout

`portos.error-contract` accepts `error-schema`. `portos.query-sorts` accepts `sorts-field`. `portos.batch-outcomes` accepts `error-schema`, `items-field`, `results-field`, `errors-field`, and `item-id-field`. Defaults are `ErrorResponse`, `sorts`, `items`, `results`, `errors`, and `id`; blank values fail configuration validation. The error member of a failed item remains `error`.

```yaml
version: 1
extends: [portos-defaults]
overrides:
  - id: portos.error-contract
    options: {error-schema: CompanyError}
  - id: portos.batch-outcomes
    options: {error-schema: CompanyError, item-id-field: resourceId}
```

Keep both error-schema overrides aligned when changing the canonical object name. Normal include/exclude scopes, severity overrides, reasoned pointer suppressions, baselines and `--only` apply. Import/override merge behavior is documented in [Rule packs](rule-packs.md).

## Tests and runtime responsibilities

`rulepack/portos_responses_test.go` covers each valid contract, each violation, configurable names, nested/composed/ref schemas, protocol-enum interactions, boundaries, cancellation, and the composed Portos fixture. The default Make/CI checks continue to require race tests, standard Go linters and 95% aggregate module statement coverage.

OpenAPI describes a contract; it cannot execute handlers or prove that result IDs actually match request values. Service contract tests must additionally verify:

1. Delete an existing resource, then delete it again and delete a never-existing resource. Every absent-resource attempt succeeds, leaves the resource absent, and produces no extra side effects. For asynchronous deletes, wait for completion and verify the final state.
2. Submit a stale ETag, verify 409 and the shared error object with family 400, then submit the current ETag and verify success. Cover both conditional headers where supported.
3. Submit mixed successful and failing batch items. Every request ID appears exactly once across results/errors, with no unknown IDs, duplicate outcomes or missing items. Each failed item contains its own shared error data. Include all-success and all-failure batches; the unused list is present and empty.
4. Verify every runtime 4xx/5xx body matches the shared schema and its family agrees with the HTTP status class. Test generic error handling as well as known validation failures.
5. Exercise ascending and descending multi-key sorts and reject invalid directions, unknown keys, and extra fields. Validate version routing with real endpoints.

These service checks are specified here for the backend owners. They are not claimed as executed by the schema linter.
