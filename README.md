# OpenAPI linter

`openapilint` is a standalone Go command and library for configurable OpenAPI and YAML-schema checks. Customer rules and bundled checks use the same public registry, analyzer, diagnostics, and suppression interfaces.

## Run

Use Go 1.24.2 or newer:

```sh
go build -o openapilint ./cmd/openapilint
./openapilint --rules examples/rules.yaml examples/specs
```

Inputs are YAML or JSON files, or directories searched recursively. `--root` defaults to the working directory. Inputs and local reference targets must stay within that root, including symlinks. Remote and absolute reference targets are rejected. Use `--kind schema` for standalone YAML-schema files; directory inputs must contain only documents of the selected kind.

OpenAPI 3.0 and 3.1 are the supported document versions. The command builds the parsed model and reports parsing/reference errors; it does not claim complete OpenAPI specification conformance validation. Schemas and cross-file references are exposed through libopenapi. The loader permits array/polymorphic circular references, and model-build failures otherwise produce an execution error. Pure reference cycles may fail to build.

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

An explicit pack executes only its configured rules. The default OpenAPI pack requires operation IDs. Schema mode has an empty default pack; supply your schema checks explicitly. Use `--only id,other-id` to select from a validated pack. See [rule-pack reference](docs/rule-packs.md) for checks, scope, and suppressions.

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
