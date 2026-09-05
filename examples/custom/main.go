// A customer command with a check registered through the stock extension API.
package main

import (
	"context"
	"fmt"
	public "github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/cli"
	"github.com/portpowered/openapi-linter/rulepack"
	"gopkg.in/yaml.v3"
	"os"
)

type titleCheck struct {
	Title string `yaml:"title"`
}

var _ public.Analyzer = titleCheck{}

func (titleCheck) ID() string { return "customer.title" }
func (c titleCheck) Analyze(ctx context.Context, pass *public.Pass) {
	for _, doc := range pass.Documents {
		if ctx.Err() != nil {
			return
		}
		if doc.Model == nil {
			continue
		}
		if doc.Model.Info == nil || doc.Model.Info.Title != c.Title {
			pass.Report(public.Diagnostic{Path: doc.Path, Pointer: "#/info/title", Line: doc.Line("#/info/title"), Message: "unexpected API title", Severity: public.SeverityError})
		}
	}
}
func main() {
	registry := rulepack.NewRegistry()
	if err := rulepack.RegisterStock(registry); err != nil {
		panic(err)
	}
	if err := registry.Register("customer.title", func(node yaml.Node) (public.Analyzer, error) {
		var c titleCheck
		if err := rulepack.DecodeOptions(node, &c); err != nil {
			return nil, err
		}
		if c.Title == "" {
			return nil, fmt.Errorf("title is required")
		}
		return c, nil
	}); err != nil {
		panic(err)
	}
	os.Exit(cli.RunWithRegistry(context.Background(), os.Args[1:], os.Stdout, os.Stderr, registry))
}
