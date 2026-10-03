# OpenAPI linter

`openapilint` is a standalone Go command and library for configurable OpenAPI and YAML-schema checks. Customer rules and bundled checks use the same public registry, analyzer, diagnostics, and suppression interfaces.

## Run

Use Go 1.24.2 or newer:

```sh
go build -o openapilint ./cmd/openapilint
./openapilint --rules examples/rules.yaml examples/specs
```

Inputs are YAML or JSON files, or directories searched recursively. `--root` defaults to the working directory. Inputs and local reference targets must stay within that root, including symlinks. Remote and absolute reference targets are rejected. Use `--kind schema` for standalone YAML-schema files; directory inputs must contain only documents of the selected kind.

OpenAPI 3.0 and 3.1 are the supported document versions. The command builds the parsed model and reports parsing/reference errors; it includes offline document-structure and example validation but does not claim complete semantic conformance. Schemas and cross-file references are exposed through libopenapi. The loader permits array/polymorphic circular references, and model-build failures otherwise produce an execution error. Pure reference cycles may fail to build.

## Rule packs

```yaml
version: 1
rules:
  - id: customer.operation-style
    check: openapi.operation-id
    severity: error
    options:
      required: true
      pattern: '^[a-z][A-Za-z0-9]*$'
```

An explicit pack executes its resolved rules, including named/local sets from `extends`. The default OpenAPI pack is `openapi:recommended`; `.openapilint.yaml` at the selected root is discovered automatically. Schema mode has an empty default pack; supply your schema checks explicitly. Use `--only id,other-id` to select from a validated pack. See [rule-pack reference](docs/rule-packs.md) for checks, scope, and suppressions.

Text diagnostics contain file, line, rule ID, message, and JSON pointer. `--format json` returns a deterministic array of diagnostics. Exit codes: 0 for success (including warning/info findings), 1 for error findings, 2 for configuration or execution failure. Linting is read-only; this release has no autofix command.

## Customer executable checks

YAML configures available checks. New executable logic is registered in a customer-built Go command; no executable plugins are downloaded or loaded from YAML. The [custom example](examples/custom/main.go) registers a check beside the stock registry:

```sh
go run ./examples/custom --rules examples/custom/rules.yaml examples/specs
```

Implement the public `Analyzer`, or adapt a `Rule`/`SchemaRule` visitor using `rulepack.AdaptRule`/`AdaptSchemaRule`. `Pass.Documents` contains the rule's selected inputs. Use `Pass.Report` for findings and `Pass.ReportError` for operational failures. `Pass.Root` supplies the filesystem boundary; custom code is trusted Go code, and must respect that boundary itself. Each check receives a separate pass; a composite analyzer can share data within its pass through `SetData` and `Data`.

`LoadDocument` and `LoadSchemaDocument` enforce the root/reference policy used by the command. The lower-level `Linter.LoadSpec` and `LoadSchema` APIs retain direct-caller filesystem semantics; applications using those APIs own their access policy. Standalone-schema checks inspect YAML structure rather than resolving schemas into OpenAPI models.

## Install and remove

Install an exact version with `go install github.com/portpowered/openapi-linter/cmd/openapilint@v0.1.0`, or download the platform archive and `checksums.txt` from releases. Verify its SHA-256 checksum, unpack it, and put the executable on PATH. Archives cover Linux, macOS, and Windows on amd64 and arm64. Hosted CI executes tests on Linux, macOS, and Windows; other architectures receive cross-build verification.

Release archives report the tag through `--version`; a Go installation reports `dev` unless version linker flags are supplied. Uninstall by removing that executable. No background service is installed.

## Development

See [development](docs/development.md). Tests and examples are self-contained and require no parent service repository or private credentials.

## Rule and usability roadmap

See the [OpenAPI and Markdown linter plan](docs/linter-roadmap.md) for the existing-system comparison, proposed rule additions, customer activation UX, rule-set composition, testing, and rollout. It records implemented rules and UX, Portos requirements with rule IDs, and the remaining customer pilot work.

## Compose Portos policy

Use `init --preset portos-defaults` to activate general recommendations plus internal Portos policy in one version 1 configuration. `rules list`, `presets list`, and `config explain` make rule activation and overrides inspectable. See the rule-pack reference for baselines, SARIF, the warning failure threshold, and breaking default/configuration changes.

The expanded rules and composition UX are available in this source checkout; pin the next release containing these changes when deploying them to CI. Existing v0.1.0 installation examples refer to the earlier released baseline.

## Development checks

Run `make` (or `make verify`) for formatting validation, build, vet, all standard Go linters, race-enabled tests and a 95% statement coverage gate, plus quality-tool tests. Install Python 3, Go and golangci-lint v2.14.0 first. Windows users can set `PYTHON=python`; Unix installations may prefer `PYTHON=python3`. `GOLANGCI_LINT` can point to the pinned executable.

Individual targets are `make test`, `make lint`, `make coverage`, `make coverage-check`, `make fmt-check`, and `make vet`. `make fmt` applies formatting. Coverage is statement-weighted across both library and command/example packages, using `-coverpkg=./...` so integration tests count library execution; no files or packages are removed from the report. The gate compares the unrounded value to 95%. `coverage.out` is local output and CI uploads each platform/toolchain profile.

CI runs the same coverage/lint policy on Linux, macOS and Windows for each supported Go version. The explicit `linters.default: standard` configuration enables errcheck, govet, ineffassign, staticcheck and unused, with no preset issue exclusions. See the [official standard linter list](https://golangci-lint.run/docs/welcome/quick-start/) and [pinned release](https://github.com/golangci/golangci-lint/releases/tag/v2.14.0). Lint failures are fixed rather than baselined.

See [Google API rules and analysis](docs/google-api.md) for `openapi:google`, `google-defaults`, 16 checks and their five paired contract groups. These are opt-in REST projections of the AIPs.

The [Portos response contracts](docs/portos-api.md) enforce 200/202 success statuses, 409 ETag conflicts, canonical error bodies, versioned kebab-case paths, declared delete idempotency, structured sorts, and synchronous batch outcomes. They are included in `portos-defaults`.
