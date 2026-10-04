# Get started with OpenAPI Linter

Use the CLI to check API contracts in your local workflow and CI, or use the [Go library](library.md) to integrate the same checks into another application.

## Install and run

```sh
go install github.com/portpowered/openapi-linter/cmd/openapilint@latest
openapilint --version
openapilint init --preset openapi:recommended --output .openapilint.yaml
openapilint --root . api.yaml
```

Pin the selected module version for production. The CLI discovers `.openapilint.yaml` from the document root; use `--rules path/to/config.yaml` for explicit configuration. Inputs can be files or directories. `--root` bounds configuration, inputs and local references. Unsupported remote references fail analysis rather than being silently treated as valid contracts.

## Choose policy

```yaml
version: 1
extends: [google-defaults]
overrides:
  - id: google.version-path
    severity: error
```

`openapi:recommended` selects general contract checks. `google-defaults` composes selected [Google API rules](google-api.md). `portos-defaults` composes the internal [Portos contracts](portos-api.md). Choose conventions deliberately: different design policies can disagree about field names and error envelopes.

`id` identifies a configured instance, while `check` names its implementation. Overrides target IDs already present in the resolved pack. Supplied options mappings are replaced as a whole. See [rule packs](rule-packs.md) for scoping, suppressions and baselines, and the [rule reference](rules/index.md) for exact option names.

```sh
openapilint rules list
openapilint rules describe portos.batch-outcomes
openapilint presets list
openapilint config explain --root . --path api.yaml
openapilint --root . --format json api.yaml
openapilint --root . --format sarif api.yaml
```

## Standalone schemas

Standalone YAML and JSON schemas use a separate input kind:

```sh
openapilint init --kind schema --preset schema:conventions --output schema-rules.yaml
openapilint --kind schema --rules schema-rules.yaml --root . schemas
openapilint rules list --kind schema
```

Do not apply OpenAPI operation checks to standalone schemas. The CLI validates the pack's input kind and reports mismatches as configuration errors.

## CI and results

Use `--fail-on warning` when warnings should block CI. Exit 0 means the chosen threshold passed, exit 1 means findings reached that threshold, and exit 2 means analysis or configuration failed. A schema finding is separate from an unresolved reference or malformed configuration.

OpenAPI lint checks declared shapes and examples. It cannot prove handler idempotency, live authentication, ETag enforcement or actual Batch item correlation. The Portos and Google guides identify the corresponding service contract tests.
