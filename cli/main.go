// Package cli provides stock and customer-registry OpenAPI commands.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	linter "github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rulepack"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var Version = "dev"

func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	registry := rulepack.NewRegistry()
	if err := rulepack.RegisterStock(registry); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	return RunWithRegistry(ctx, args, out, errOut, registry)
}
func RunWithRegistry(ctx context.Context, args []string, out, errOut io.Writer, registry *rulepack.Registry) int {
	flags := flag.NewFlagSet("openapilint", flag.ContinueOnError)
	flags.SetOutput(errOut)
	kind := flags.String("kind", "openapi", "input kind: openapi or schema")
	config := flags.String("rules", "", "YAML rule pack")
	root := flags.String("root", ".", "input and reference root")
	format := flags.String("format", "text", "text or json")
	only := flags.String("only", "", "comma-separated rule IDs")
	version := flags.Bool("version", false, "print version")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *version {
		fmt.Fprintln(out, "openapilint", Version)
		return 0
	}
	fail := func(err error) int { fmt.Fprintln(errOut, err); return 2 }
	if flags.NArg() == 0 {
		return fail(fmt.Errorf("at least one input file or directory is required"))
	}
	if *format != "text" && *format != "json" {
		return fail(fmt.Errorf("unknown format %q", *format))
	}
	if *kind != "openapi" && *kind != "schema" {
		return fail(fmt.Errorf("unknown kind %q", *kind))
	}
	pack := rulepack.DefaultPack()
	if *kind == "schema" {
		pack = rulepack.Pack{Version: 1, Rules: []rulepack.Rule{}}
	}
	if *config != "" {
		file, err := os.Open(*config)
		if err != nil {
			return fail(err)
		}
		pack, err = rulepack.Decode(file)
		file.Close()
		if err != nil {
			return fail(err)
		}
	}
	if _, err := registry.Compile(pack); err != nil {
		return fail(err)
	}
	if *only != "" {
		wanted := map[string]bool{}
		for _, id := range strings.Split(*only, ",") {
			wanted[strings.TrimSpace(id)] = true
		}
		selected := []rulepack.Rule{}
		found := map[string]bool{}
		for _, rule := range pack.Rules {
			if wanted[rule.ID] {
				selected = append(selected, rule)
				found[rule.ID] = true
				delete(wanted, rule.ID)
			}
		}
		if len(wanted) > 0 {
			return fail(fmt.Errorf("--only contains a rule not present in the pack"))
		}
		pack.Rules = selected
		suppressions := []rulepack.Suppression{}
		for _, s := range pack.Suppressions {
			if found[s.Rule] {
				suppressions = append(suppressions, s)
			}
		}
		pack.Suppressions = suppressions
	}
	program, err := registry.Compile(pack)
	if err != nil {
		return fail(err)
	}
	files, err := DiscoverFiles(flags.Args())
	if err != nil {
		return fail(err)
	}
	if len(files) == 0 {
		return fail(fmt.Errorf("no YAML or JSON input files found"))
	}
	documents := []*linter.Document{}
	for _, file := range files {
		loader := linter.LoadDocument
		if *kind == "schema" {
			loader = linter.LoadSchemaDocument
		}
		doc, err := loader(ctx, file, *root)
		if err != nil {
			return fail(err)
		}
		documents = append(documents, doc)
	}
	diagnostics, err := program.Run(ctx, *root, documents)
	if err != nil {
		return fail(err)
	}
	if *format == "json" {
		err = json.NewEncoder(out).Encode(diagnostics)
	} else {
		for _, d := range diagnostics {
			if _, err = fmt.Fprintf(out, "%s:%d: %s: %s (%s)\n", d.Path, d.Line, d.RuleID, d.Message, d.Pointer); err != nil {
				break
			}
		}
	}
	if err != nil {
		return fail(err)
	}
	for _, d := range diagnostics {
		if d.Severity == linter.SeverityError {
			return 1
		}
	}
	return 0
}

// DiscoverFiles recursively finds YAML/JSON inputs and deduplicates their paths.
func DiscoverFiles(inputs []string) ([]string, error) {
	found := map[string]bool{}
	for _, input := range inputs {
		err := filepath.WalkDir(input, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".yaml", ".yml", ".json":
				found[filepath.Clean(path)] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	result := make([]string, 0, len(found))
	for path := range found {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}
