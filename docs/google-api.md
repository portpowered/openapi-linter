# Google API rules

The opt-in `openapi:google` pack contains **16 independent checks** for the REST and JSON contracts that OpenAPI can describe. `google-defaults` composes these checks with `openapi:recommended`. Google rules use warning severity because adopting a design standard requires review of existing APIs. They do not change the default pack or the Portos rules.

This is an OpenAPI projection of selected [Google AIPs](https://google.aip.dev/general), not a claim to implement all Google standards. Google's [API Linter](https://linter.aip.dev/) works on protobuf descriptors and has many annotation and code-generation checks that have no OpenAPI equivalent. The counts below refer to this implementation, not the total number of Google requirements.

## Activation and customer workflow

Initialize a config and inspect what it activates:

```sh
openapilint init --preset google-defaults --output .openapilint.yaml
openapilint presets describe openapi:google
openapilint rules describe google.list-request
openapilint config explain --rules .openapilint.yaml --root . --path api.yaml
openapilint --root . --fail-on warning api.yaml
```

The config has a single version:

```yaml
version: 1
extends:
  - google-defaults
overrides:
  - id: google.version-path
    severity: error
  - id: google.timestamp
    enabled: false
```

Use ordinary scopes and reasoned JSON-pointer suppressions for intentional exceptions. Every Google check accepts an empty options map only; unsupported options are configuration errors. Request methods are identified from an UpperCamelCase `operationId` prefix, and custom methods from their colon path suffix. Callbacks and webhooks are excluded from operation policy. Define operation IDs using the RPC names, such as `ListBooks`, rather than client SDK aliases such as `books.list`.

## Implemented checks and source requirements

| Checklist and rule ID | Expected contract | Primary source |
| --- | --- | --- |
| [x] `google.method-name` | UpperCamelCase method shape containing an action and resource name; reject the `Async` word. This checks spelling shape, not English verb semantics. | [AIP-190](https://google.aip.dev/190), [AIP-136](https://google.aip.dev/136) |
| [x] `google.http-method` | Standard Get and List use GET, Create uses POST, Update uses PATCH, and Delete uses DELETE. | [AIP-131](https://google.aip.dev/131), [132](https://google.aip.dev/132), [133](https://google.aip.dev/133), [134](https://google.aip.dev/134), [135](https://google.aip.dev/135) |
| [x] `google.version-path` | Request paths begin with a major version such as `/v1`, `/v1beta`, or `/v1beta1`; minor and patch versions fail. | [AIP-185](https://google.aip.dev/185) |
| [x] `google.standard-path` | Get, Update, and Delete end in a resource variable; List and Create end in a collection literal. Singleton resources may use the explicit marker described below. | [AIP-131](https://google.aip.dev/131), [132](https://google.aip.dev/132), [133](https://google.aip.dev/133), [134](https://google.aip.dev/134), [135](https://google.aip.dev/135), [156](https://google.aip.dev/156) |
| [x] `google.request-body` | Get, List, Delete, and other GET operations omit bodies. Create and Update declare a required resource object body. | [AIP-131](https://google.aip.dev/131), [132](https://google.aip.dev/132), [133](https://google.aip.dev/133), [134](https://google.aip.dev/134), [135](https://google.aip.dev/135) |
| [x] `google.resource-response` | Synchronous Get, Create, and Update declare an object success response. Resource identity is checked separately. | [AIP-131](https://google.aip.dev/131), [133](https://google.aip.dev/133), [134](https://google.aip.dev/134) |
| [x] `google.list-request` | List declares optional integer `pageSize` and optional string `pageToken` query parameters. | [AIP-132](https://google.aip.dev/132), [AIP-158](https://google.aip.dev/158) |
| [x] `google.list-response` | List declares an object with one array of resource objects and string `nextPageToken`. | [AIP-132](https://google.aip.dev/132), [AIP-158](https://google.aip.dev/158) |
| [x] `google.update-mask` | Update declares optional string `updateMask` in query parameters, matching the JSON representation of FieldMask. | [AIP-134](https://google.aip.dev/134), [AIP-161](https://google.aip.dev/161), [ProtoJSON](https://protobuf.dev/programming-guides/json/) |
| [x] `google.custom-method` | Custom operations use GET or POST and a matching `:verb` suffix, or `:verbNoun` for stateless methods. Avoid standard method verbs except the documented LongRunning variant; reject a defined set of preposition words. | [AIP-136](https://google.aip.dev/136) |
| [x] `google.resource-name` | Synchronous resource responses and explicitly marked resource schemas declare string `name`. | [AIP-122](https://google.aip.dev/122) |
| [x] `google.field-names` | Declared JSON properties use lowerCamelCase. ProtoJSON Any `@type` and arbitrary map keys are allowed. | [AIP-140](https://google.aip.dev/140), [ProtoJSON](https://protobuf.dev/programming-guides/json/) |
| [x] `google.boolean-names` | Boolean properties omit the `is` prefix; allow `isNew` for the documented reserved-word exception. | [AIP-140](https://google.aip.dev/140) |
| [x] `google.timestamp` | Properties ending in `Time`, such as `createTime`, declare string `date-time`, the RFC 3339 representation of Timestamp. | [AIP-142](https://google.aip.dev/142), [ProtoJSON](https://protobuf.dev/programming-guides/json/) |
| [x] `google.error-response` | Document an HTTP error response with a JSON `error` object containing integer `code` and string `message`. These are the common REST envelope fields; this check does not prove runtime status mapping. | [AIP-193](https://google.aip.dev/193), [Google Cloud error model](https://cloud.google.com/apis/design/errors) |
| [x] `google.long-running-response` | An explicitly marked operation declares an Operation object with string `name`, boolean `done`, object `error`, and object `response`. These fields are declared, not all required simultaneously. | [AIP-151](https://google.aip.dev/151), [Operation definition](https://github.com/googleapis/googleapis/blob/master/google/longrunning/operations.proto) |

The REST projection deliberately uses `pageSize`, `pageToken`, `nextPageToken`, and `updateMask`. Protobuf AIPs use their snake_case source names. ProtoJSON defines the canonical lowerCamelCase names. Applying source-level protobuf casing directly to JSON would create conflicting advice.

## Paired contracts

There are **five paired requirement groups**, implemented through the independently selectable checks above. These are relationships, not five additional checks:

1. A standard operation's verb and target path must agree: `google.http-method` plus `google.standard-path`.
2. A mutation's body and success response must describe resource objects: `google.request-body` plus `google.resource-response`. `google.resource-name` supplies the identity constraint.
3. A List request and response must expose both sides of pagination: `google.list-request` plus `google.list-response`.
4. Partial update support requires the PATCH method and optional mask together: `google.http-method` plus `google.update-mask`.
5. Explicit long-running operation intent and its success schema must agree: the `x-google-long-running` marker plus `google.long-running-response`. This marker also suppresses synchronous resource response checks for that operation.

For example, `ListBooks` with no `pageToken` and no `nextPageToken` produces independent request and response findings. A fix on either side leaves the other finding active. Paired checks retain their own IDs so customers can adopt them separately.

## Explicit intent and examples

These two extensions are local linter conventions, not Google-defined OpenAPI extensions:

```yaml
paths:
  /v1/settings:
    get:
      operationId: GetSettings
      x-google-singleton: true
  /v1/books:
    post:
      operationId: CreateBook
      x-google-long-running: true
components:
  schemas:
    Book:
      type: object
      x-google-resource: true
      properties:
        name:
          type: string
        createTime:
          type: string
          format: date-time
```

`x-google-singleton: true` permits a literal singleton resource path for Get or Update and rejects Create or Delete for that singleton. `x-google-resource: true` enables resource identity checking for a component that may not appear in a standard response. The ordinary graph traversal follows bounded local references, object properties, arrays, dictionaries, and composition branches. Shared and recursive schemas are visited once per check.

A valid List request has optional query parameters `pageSize: integer` and `pageToken: string`. Its success schema has `books: array` with object items and `nextPageToken: string`. A body on that GET, a required page token, or an integer next-page token fails its relevant check. See `rulepack/google_checks_test.go` for executable examples for every rule.

## Boundaries and deferred requirements

These checks cannot determine whether the service actually enforces page limits, returns opaque tokens, honors update masks, populates resources correctly, maps every error status correctly, or prevents both long-running `error` and `response` from being populated. Those require runtime contract tests. Schema inspection also cannot prove English singular/plural semantics, appropriate business verbs, resource uniqueness, stable ordering, side effects, consistency, authorization, or backward compatibility across releases.

Protobuf package names, field numbers, method signatures, resource annotations and patterns, HTTP transcoding annotations, and enum zero-value conventions remain the responsibility of the protobuf API Linter. Full resource-name extraction uses annotated URI templates; OpenAPI describes the expanded REST path, so this pack checks terminal resource versus collection shape rather than imposing protobuf binding syntax. PUT-only replacement APIs need a reasoned exception to the PATCH preference. Existing custom JSON names also need explicit exceptions to canonical casing.

No claim is made that 16 checks exhaust the standards. This first set covers deterministic shape requirements available in OpenAPI and uses existing core checks for reference correctness, duplicate operation IDs, path parameters, examples, and documentation. Authentication and publication packs can be composed separately.

## Tests and acceptance

Each of the 16 checks has a passing and failing fixture. Additional tests cover required and malformed mutation bodies, absent success schemas, undocumented errors, singleton exceptions, stateless actions, map keys, boolean exceptions, nested schemas, callbacks, webhooks, unsupported options, cancellation, duplicate registration, and offline external reference attribution. Both `openapi:google` and `google-defaults` compile and run in the tests. Their paired List request and response defects must produce both rule IDs.

The default verification target and CI enforce the repository coverage threshold and standard Go linters. Performance and runtime compatibility claims need representative customer specs and integration tests; schema tests alone do not establish them.
