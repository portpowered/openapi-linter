# Use OpenAPI Linter as a Go library

The root package exposes documents, analyzers and diagnostics. `rulepack` loads and compiles the same configuration used by the CLI. The module name is `github.com/portpowered/openapi-linter`; `openapilint` is the command name.

```sh
go get github.com/portpowered/openapi-linter@latest
```

Pin a reviewed module version in your application's `go.mod`.

## Load and run a pack

```go
package main

import (
    "context"
    "fmt"
    "log"

    linter "github.com/portpowered/openapi-linter"
    "github.com/portpowered/openapi-linter/rulepack"
)

func main() {
    ctx := context.Background()
    root := "."
    registry := rulepack.NewRegistry()
    if err := rulepack.RegisterStock(registry); err != nil {
        log.Fatal(err)
    }
    pack, err := rulepack.Load("openapi:recommended", root)
    if err != nil {
        log.Fatal(err)
    }
    if err := registry.ValidateKind(pack, "openapi"); err != nil {
        log.Fatal(err)
    }
    program, err := registry.Compile(pack)
    if err != nil {
        log.Fatal(err)
    }
    doc, err := linter.LoadDocument(ctx, "api.yaml", root)
    if err != nil {
        log.Fatal(err)
    }
    findings, err := program.Run(ctx, root, []*linter.Document{doc})
    if err != nil {
        log.Fatal(err)
    }
    for _, finding := range findings {
        fmt.Printf("%s:%d %s [%s] %s: %s\n",
            finding.Path, finding.Line, finding.Pointer,
            finding.Severity, finding.RuleID, finding.Message)
    }
}
```

Replace the preset with `.openapilint.yaml` to load customer configuration. Use `LoadDocument` for the source tree, OpenAPI model and source locations required by the stock checks. Pass all selected documents to one `Program.Run` call with a consistent root and a cancellable context.

For standalone schemas, use `LoadSchemaDocument`, `schema:conventions`, and `ValidateKind(pack, "schema")`. The same program and diagnostic contracts apply.

## Handle findings and failures

`Program.Run` returns operational errors separately from findings. A finding contains the configured `RuleID`, implementation `CheckID`, `Origin`, `Severity`, source `Path`, `Pointer`, and `Line`. Your application decides which severity blocks publication; a nil error does not imply an empty findings list.

Use the same document root for configuration and source loading. Local reference traversal respects that boundary. Unsupported remote references and unresolved schemas must remain failures, not suppressed policy findings. See [rule packs](rule-packs.md) for supported scopes and exceptions.

## Customer checks

Implement `linter.Analyzer` with `ID()` and `Analyze(context.Context, *linter.Pass)`. Report findings with `Pass.Report`, and operational failures with `Pass.ReportError`. Register a factory through `Registry.Register` and strictly validate its YAML options with `rulepack.DecodeOptions`. The configured check selects your factory through the ordinary pack machinery.

The [custom command](../examples/custom/main.go) demonstrates registration and `cli.RunWithRegistry`. `rulepack.AdaptRule` and `AdaptSchemaRule` expose the visitor contracts when integrating existing checks. Customer code runs with your application's privileges; configuration does not load remote executable plugins.

The pack integration above and every generated rule configuration are checked in `rulepack/site_contract_test.go`. See the [package reference](https://pkg.go.dev/github.com/portpowered/openapi-linter/rulepack) for exported API contracts.
