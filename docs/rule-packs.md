# Rule-pack reference

A pack is one YAML document containing `version: 1`, `rules`, and optional `suppressions`. Rule entries require `id` and `check`, with optional `description`, `severity`, `include`, `exclude`, and `options`. Severity is `error` by default, or `warning`/`info`. Empty rule lists intentionally run no checks. Unknown fields/checks, duplicate IDs, bad options, and unsupported versions are errors.

Scopes are root-relative slash paths. `*`, `?`, and character classes match within segments; trailing `/**` selects a directory recursively. Other recursive glob forms are not supported. Paths must be normalized and cannot be absolute or contain backslashes. Exclusions take precedence. Cross-file analyzers receive only the selected input documents, and reported diagnostic files must belong to that selection.

Suppressions require `rule`, `path` pattern, and a nonempty `reason`; optional `line` selects a one-based line, while zero/omitted line suppresses the entire matching file. They work identically for stock and customer diagnostics.

## Stock checks

| Check | Input kind | Options |
| --- | --- | --- |
| `openapi.operation-id` | OpenAPI | `required` (default true), optional Go regexp `pattern` |
| `openapi.path-naming` | OpenAPI | None; kebab-case path segments, with standard well-known exceptions |
| `openapi.schema-naming` | OpenAPI | None; PascalCase schema names |
| `openapi.schema-description` | OpenAPI | None; required descriptions |
| `openapi.request-response-example` | OpenAPI | None; request/response example checks |
| `openapi.property-camel-case` | OpenAPI | None; schema property naming |
| `schema.no-anonymous-objects` | Schema | None; references instead of anonymous nested objects |
| `schema.property-camel-case` | Schema | None; property naming |

Checks act on their supported input kind. Choosing schema mode with only OpenAPI checks produces no findings; use checks matching the selected kind. Service-specific query, capability, flow-node, and CLI-contract policy is not installed in this library.

## Locations and errors

`Diagnostic.Path` is an input filename. `Pointer` is a JSON pointer into that document, and `Line` is its source line when available (zero otherwise). Adapted visitor checks report the visited schema/path/operation as their location; more precise customer checks can report a field pointer. `Context` preserves an optional check-specific human location. Findings on resolved schemas are attributed to the component/reference site being checked.

Use `Pass.ReportError` when analysis cannot complete. It produces exit 2 rather than a suppressible policy finding. Cancellation also fails execution. The API does not sandbox customer Go checks; customer code must enforce its intended filesystem/network policy.
