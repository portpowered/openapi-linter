# Rule packs

Use `version: 1` for every pack. This is the only supported configuration schema. Named sets have no version suffix; pin the executable release in CI and review `packs/manifest.json` hashes when upgrading. There is no compatibility branch for previous default behavior.

## Discover and activate

Commands below use COMMAND as the executable name and CONFIG as its conventional root configuration. Replace these placeholders as shown in the tool section below.

```sh
COMMAND rules list --format json
COMMAND rules describe CHECK_ID
COMMAND presets list
COMMAND presets describe portos-defaults
COMMAND presets export portos-defaults
COMMAND init --preset portos-defaults --output CONFIG
COMMAND config schema
COMMAND config validate --rules CONFIG --root .
COMMAND config explain --rules CONFIG --root . --path INPUT_PATH
COMMAND --rules CONFIG --root . INPUT_PATH
```

A check is executable logic; a rule is one configured instance with a customer-visible ID. A rule set is a reusable pack. `rules list` lists available checks; `config explain` lists configured rules and whether they apply to a supplied path. `describe` includes options, defaults, kind, preset membership, severity guidance, and fix support. `docs/rule-catalog.json` is a readable catalog snapshot. `config schema` exports an editor JSON Schema generated from registry options; `docs/rule-pack.schema.json` is the stock snapshot. Runtime validation additionally enforces root boundaries, IDs, regex/options semantics, and kind applicability. Customer commands use the same public registry and can add descriptors with `Registry.SetDescriptor`.

`init` refuses to overwrite a file unless `--overwrite` is supplied. An explicit `--rules` wins. Otherwise the command looks only for CONFIG at `--root`; it does not read home/ancestor configs. Invalid discovered config fails instead of falling back. An empty explicit rule list with no extends intentionally disables policy checks, while parse/execution failures still fail.

## Compose and override

This is supported single-version configuration:

```yaml
version: 1
extends:
  - portos-defaults
  - ./.lint/company.yaml
overrides:
  - id: RULE_ID_FROM_PRESET
    severity: warning
  - id: ANOTHER_RULE_ID_FROM_PRESET
    enabled: false
rules: []
suppressions: []
```

Replace the two example IDs with actual configured IDs from `config explain`; unknown IDs are errors. A local company pack uses the same schema and can extend sets, add its own instances, and override inherited instances. Imports are relative to the containing pack, bounded by the selected root including symlinks. All scope paths remain relative to the lint root. Explicit root configuration must also be within that root.

Dependencies are resolved in declared order, with repeated identical dependencies loaded once. Cycles and duplicate configured IDs are errors; use `overrides` rather than defining the same ID in two packs. Each pack's overrides apply after its dependencies and its added rules. Root overrides run last. Independent instances of the same check use distinct IDs; choose disjoint scopes if overlapping findings would be redundant.

Overrides can change enabled state, severity, include, exclude, and options, but not check identity. Supplied options maps and scope lists replace whole fields, while omitted fields inherit. `options: {}` resets to check defaults. The complete resolved configuration is validated, including disabled rules and rules omitted by `--only`. Each check rejects unsupported option keys. Unknown fields/checks, bad severities/options, duplicate IDs, and unsupported configuration versions fail before linting.

Severity is error, warning, or info, default error. A rule requires id and check; optional fields are description, enabled, severity, include, exclude, and options. Include/exclude patterns are root-relative slash paths: `*`, `?`, and character classes match within a segment, and a trailing `/**` selects a subtree. Exclusions win. No absolute paths/backslashes or other recursive glob forms are supported.

`--only id,other-id` selects configured enabled instance IDs after validation. It does not activate an arbitrary installed check. An unknown/disabled selection fails. Kind mismatches fail instead of silently producing no findings; custom checks without kind metadata remain allowed.

## Exceptions and CI adoption

```yaml
suppressions:
  - rule: RULE_ID
    path: apis/legacy.yaml
    line: 42
    reason: Temporary migration exception tracked in API-123
```

Suppressions require an existing rule ID, root-relative path pattern, and nonblank reason. Omitted/zero line covers the file. Suppressed fixes are removed as well as findings. Source parse, reference boundary, and execution failures cannot be suppressed. Markdown additionally supports `<!-- marklint-disable-next-line RULE_ID reason: explanation -->` immediately before a finding. The ID must match the configured rule instance, the reason must be nonblank, and directives inside code do not suppress findings. OpenAPI pack suppressions can add `pointer: "#/paths/~1widgets/get"` to match exactly that diagnostic pointer; omitted pointers cover all locations selected by the path and line.

```sh
COMMAND baseline create --output baseline.json --rules CONFIG --root . INPUT_PATH
COMMAND --baseline baseline.json --rules CONFIG --root . INPUT_PATH
COMMAND --fail-on warning --rules CONFIG --root . INPUT_PATH
COMMAND --format sarif --rules CONFIG --root . INPUT_PATH
```

Baseline creation is explicit and read-only with respect to inputs. Replacement requires `--overwrite` on baseline create (or `--baseline-overwrite` with `--baseline-write`). Ordinary lint never updates the baseline. Fingerprints contain root-relative file identity, instance/check identity, message, and source evidence; OpenAPI also includes the pointer. Counts matter: a newly introduced duplicate is new debt. Moving unchanged lines remains recognized when message/pointer identity is unchanged; changed evidence produces a new finding. Known counts go to stderr and new findings remain in the selected output format. This is conservative debt matching, not an AST-aware API comparison.

Exit codes: 0 for success, 1 for findings at the selected failure threshold, 2 for configuration/execution failure. Default threshold is error; `--fail-on warning` includes warnings without changing their labels. JSON is an ordered diagnostics array including configured RuleID, CheckID, Origin, Severity, file/location, and message. SARIF 2.1 output uses those same findings. For editors, use the machine-readable catalog and diagnostics rather than a separate policy engine.

## Distribution and extensions

First-party packs are readable YAML embedded in the binary; SHA-256 manifests and YAML sources are included in release archives. `presets describe` includes the source content hash and resolved membership. `presets export` prints the original YAML, including composition references. Customer packs are checked-in local YAML. No config downloads code, packs, or external URLs during a lint run.

New executable checks require a customer-built Go command using `Registry.Register` and `RunWithRegistry`. Stock and customer checks have the same options/pass/diagnostic capabilities. Use `rulepack.Load(filename, root)` before `Compile` for composition; `Decode` alone parses raw YAML. Root policy is enforced by the stock loaders; custom Go code is trusted and must follow its intended filesystem/network policy itself.

## OpenAPI command and sets

COMMAND is `openapilint`, CONFIG is `.openapilint.yaml`, and INPUT_PATH is an API entrypoint or a directory containing API entrypoints. The default is openapi:recommended. Other sets: openapi:core, openapi:documentation, openapi:conventions, openapi:security, openapi:publication, openapi:maintenance, schema:conventions, portos:internal, and portos-defaults. Do not point an API run at a mixed directory of rule configs/component fragments. In standalone `--kind schema` mode the implicit pack is empty; activate schema:conventions explicitly.

```sh
openapilint --rules examples/portos/rules.yaml examples/portos/api.yaml
```

## OpenAPI rules and options

The machine-readable catalog lists all registered IDs and defaults. Most correctness checks have no options: operation-id-unique, path-parameters, parameters-unique, path-syntax, security-references, spec-structure, enum-values, server-variables, tags-defined, ref-siblings, and unused-components. Description/summary/info-presence checks also take no options.

| Check | Options and behavior |
| --- | --- |
| openapi.operation-id | required defaults true; optional Go regexp pattern |
| openapi.operation-success-response | classes list defaults ['2', '3']; supported values 2, 3, 4, 5 |
| openapi.examples-valid | None; local JSON-compatible examples; remote externalValue is not fetched |
| openapi.auth-required | public-operations: operation ID exceptions; effective anonymous alternatives are unauthenticated |
| openapi.server-policy | protocols defaults [https]; optional exact hosts list; substitute declared variable defaults before checking |
| openapi.error-response | optional error-schema component name; documented 4xx/5xx/default response policy |
| openapi.pagination | required collection-operations list of IDs; optional next-token-field, max-results-field, pagination-field |
| openapi.path-naming | None; kebab-case paths with documented well-known exceptions |
| openapi.schema-naming | None; PascalCase components |
| openapi.schema-description | None; nonblank component descriptions |
| openapi.request-response-example | None; media example presence, including default responses; schema-only examples do not satisfy this media-level policy |
| openapi.property-camel-case | None; schema property case |
| schema.no-anonymous-objects and schema.property-camel-case | None; standalone YAML structure checks, not JSON Schema validation |

Core correctness errors and documentation warnings are separate. Naming conventions, required authentication, publication metadata, and component maintenance remain explicitly selected policies. Unused-component checking assumes a complete entrypoint; omit that set for libraries consumed by other API documents. Callback expressions/webhook names are not treated as request path templates.

## Portos shape rules

The canonical defaults are Query, AsyncIdentifier, NameValue, DescriptionValue, nextToken, maxResults, results, paginationContext, items, and id. Shape policy is specifiable through the following independent check options:

| Check | Options |
| --- | --- |
| portos.operation-vocabulary | operations defaults [List, Send, Delete, Query, Get, Modify]; modifiers defaults [Batch, Async] |
| portos.query-request | None; POST with an object body |
| portos.async-response | async-schema and item-id-field |
| portos.collection-response | results-field and pagination-field |
| portos.pagination-response | next-token-field, max-results-field, pagination-field |
| portos.path-description | None; Path Item description, not merely operation description |
| portos.pagination-request | next-token-field, max-results-field, pagination-field; List query parameters or Query body pagination |
| portos.open-enums | None; x-extensible-enum values with corresponding unique x-enum-varnames; reject closed enum |
| portos.date-fields | date-fields: additional explicit names; date/time suffixes are recognized, with date-time format required |
| portos.batch-contract | items-field, results-field, item-id-field |
| portos.query-graph | query-schema, default Query |
| portos.name-schema | name-schema, default NameValue |
| portos.description-schema | description-schema, default DescriptionValue |

Operation classification uses operation ID words; modifiers do not replace the primary operation. A resource prefix/suffix may identify the domain. Structural shape rules follow local references and allOf object properties. Query uses the backend recursive graph: match/lessThan/greaterThan comparators with string key/value, and/or arrays of Query, not referencing Query, and string patternMatch/freeformMatch. Query/List response shapes are checked separately from this query AST.

```yaml
version: 1
extends: [portos-defaults]
overrides:
  - id: portos.name-schema
    options: {name-schema: CompanyNameValue}
  - id: portos.async-response
    options: {async-schema: AsyncIdentifier, item-id-field: asyncId}
rules: []
```

```yaml
type: string
x-extensible-enum: [ACTIVE, INACTIVE]
x-enum-varnames: [ACTIVE, INACTIVE]
```

This is open because the named values annotate the string rather than close it with an enum assertion. Batch IDs, runtime pagination behavior, and async execution results still require service contract tests; this linter checks the advertised shape.

## Validation and source boundaries

Supported API versions are 3.0 and 3.1. Official dated document schemas are embedded for offline structural checks; the 3.1 schema excludes Schema Object validation. See rulepack/schemas/README.md for provenance. Structural plus policy checks do not claim complete semantic conformance.

Examples are checked by a JSON Schema validator. 3.0 nullable and exclusive bounds are translated, recursive references preserved, and readOnly/writeOnly required fields interpreted by direction. Default responses and parameter examples are covered. Schema examples are also checked. Additional properties follow the schema rather than a blanket prohibition. Root jsonSchemaDialect supports the OpenAPI 3.1 base dialect and JSON Schema 2020-12; unsupported custom dialect analysis fails explicitly. Non-JSON payload validation and fetched external examples are not claimed.

Inputs and local reference targets stay within root, including symlinks. Remote/absolute targets fail. Resolved external findings are attributed to the source reference where available, with the target location in the message. Configured checks receive only selected inputs, while references can be read inside root; excluded files are not independently emitted as findings. Custom Go code is trusted and must respect its own boundary.

OpenAPI has no automatic semantic fixes. `Diagnostic.Pointer` and `Line` locate findings; unavailable source locations remain zero rather than invented. Parse/reference failures and failed analysis return exit 2 and cannot be hidden by a baseline or suppression.
