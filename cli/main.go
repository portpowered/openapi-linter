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
	var baselineErr error
	args, baselineErr = rulepack.BaselineArgs(args)
	if baselineErr != nil {
		fmt.Fprintln(errOut, baselineErr)
		return 2
	}
	if handled, code := rulepack.Manage(args, ".openapilint.yaml", registry, out, errOut); handled {
		return code
	}
	flags := flag.NewFlagSet("openapilint", flag.ContinueOnError)
	flags.SetOutput(errOut)
	baseline := flags.String("baseline", "", "known findings file")
	baselineWrite := flags.String("baseline-write", "", "explicit baseline destination")
	baselineOverwrite := flags.Bool("baseline-overwrite", false, "explicitly replace baseline")
	failOn := flags.String("fail-on", "error", "failure threshold: error or warning")
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
	if *failOn != "error" && *failOn != "warning" {
		return fail(fmt.Errorf("fail-on must be error or warning"))
	}
	if *baseline != "" && *baselineWrite != "" {
		return fail(fmt.Errorf("baseline read and write cannot be combined"))
	}
	if flags.NArg() == 0 {
		return fail(fmt.Errorf("at least one input file or directory is required"))
	}
	if *format != "text" && *format != "json" && *format != "sarif" {
		return fail(fmt.Errorf("unknown format %q", *format))
	}
	if *kind != "openapi" && *kind != "schema" {
		return fail(fmt.Errorf("unknown kind %q", *kind))
	}
	pack := rulepack.DefaultPack()
	if *kind == "schema" {
		pack = rulepack.Pack{Version: 1, Rules: []rulepack.Rule{}}
	}
	if *config == "" {
		candidate := filepath.Join(*root, ".openapilint.yaml")
		if _, err := os.Stat(candidate); err == nil {
			*config = candidate
		} else if !os.IsNotExist(err) {
			return fail(err)
		}
	}
	if *config != "" {
		var err error
		pack, err = rulepack.Load(*config, *root)
		if err != nil {
			return fail(err)
		}
	}
	if err := registry.ValidateKind(pack, *kind); err != nil {
		return fail(err)
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
			if wanted[rule.ID] && (rule.Enabled == nil || *rule.Enabled) {
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
	if *baselineWrite != "" {
		if err = rulepack.WriteBaseline(*baselineWrite, *root, diagnostics, *baselineOverwrite); err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "Baseline written to %s\n", *baselineWrite)
		return 0
	}
	if *baseline != "" {
		var known int
		diagnostics, known, err = rulepack.ApplyBaseline(*baseline, *root, diagnostics)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(errOut, "%d known findings in baseline\n", known)
	}
	if *format == "json" {
		err = json.NewEncoder(out).Encode(diagnostics)
	} else if *format == "sarif" {
		err = json.NewEncoder(out).Encode(rulepack.SARIF(diagnostics))
	} else {
		for _, d := range diagnostics {
			if _, err = fmt.Fprintf(out, "%s:%d: %s: %s: %s (%s)\n", d.Path, d.Line, d.Severity, d.RuleID, d.Message, d.Pointer); err != nil {
				break
			}
		}
	}
	if err != nil {
		return fail(err)
	}
	for _, d := range diagnostics {
		if d.Severity == linter.SeverityError || (*failOn == "warning" && d.Severity == linter.SeverityWarning) {
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
