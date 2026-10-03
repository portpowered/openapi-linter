package rulepack

import (
	"context"
	"fmt"
	linter "github.com/portpowered/openapi-linter"
	"github.com/portpowered/openapi-linter/rules"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"regexp"
)

type visitor struct{ rule linter.Rule }

var _ linter.Analyzer = visitor{}

func (v visitor) ID() string { return v.rule.Name() }
func (v visitor) Analyze(ctx context.Context, pass *linter.Pass) {
	engine := &linter.Engine{}
	engine.RegisterRule(v.rule)
	for _, doc := range pass.Documents {
		if ctx.Err() != nil {
			return
		}
		if doc.Model == nil {
			continue
		}
		for _, finding := range engine.Run(doc.Model) {
			pass.Report(linter.Diagnostic{Context: finding.Path, Path: doc.Path, Pointer: finding.Pointer, Line: doc.Line(finding.Pointer), RuleID: finding.RuleName, Message: finding.Message, Severity: linter.SeverityError})
		}
	}
}

// AdaptRule exposes the public OpenAPI visitor contract as a pack analyzer.
func AdaptRule(rule linter.Rule) linter.Analyzer { return visitor{rule} }

func RegisterStock(registry *Registry) error {
	checks := map[string]func() linter.Rule{
		"openapi.path-naming":              func() linter.Rule { return &rules.PathNamingConvention{} },
		"openapi.schema-naming":            func() linter.Rule { return &rules.SchemaNamingConvention{} },
		"openapi.schema-description":       func() linter.Rule { return &rules.SchemaRequiresDescription{} },
		"openapi.request-response-example": func() linter.Rule { return &rules.RequestResponseRequiresExample{} },
		"openapi.property-camel-case":      func() linter.Rule { return &rules.SchemaPropertyCamelCase{} },
	}
	for name, constructor := range checks {
		if err := registry.Register(name, func(node yaml.Node) (linter.Analyzer, error) {
			if err := DecodeOptions(node, &struct{}{}); err != nil {
				return nil, err
			}
			return AdaptRule(constructor()), nil
		}); err != nil {
			return err
		}
	}
	for name, check := range map[string]linter.SchemaRule{"schema.no-anonymous-objects": &rules.NoAnonymousObjects{}, "schema.property-camel-case": &rules.PropertyCamelCase{}} {
		if err := registry.Register(name, func(node yaml.Node) (linter.Analyzer, error) {
			if err := DecodeOptions(node, &struct{}{}); err != nil {
				return nil, err
			}
			return AdaptSchemaRule(check), nil
		}); err != nil {
			return err
		}
	}
	if err := registry.Register("openapi.operation-id", newOperationID); err != nil {
		return err
	}
	return registerAdditional(registry)
}
func DefaultPack() Pack { p, _ := Preset("openapi:recommended"); return p }

type operationID struct {
	required bool
	pattern  *regexp.Regexp
}

var _ linter.Analyzer = operationID{}

func (operationID) ID() string { return "openapi.operation-id" }
func newOperationID(node yaml.Node) (linter.Analyzer, error) {
	options := struct {
		Required bool   `yaml:"required"`
		Pattern  string `yaml:"pattern"`
	}{Required: true}
	if err := DecodeOptions(node, &options); err != nil {
		return nil, err
	}
	check := operationID{required: options.Required}
	if options.Pattern != "" {
		var err error
		check.pattern, err = regexp.Compile(options.Pattern)
		if err != nil {
			return nil, err
		}
	}
	return check, nil
}
func (c operationID) Analyze(ctx context.Context, pass *linter.Pass) {
	for _, doc := range pass.Documents {
		if ctx.Err() != nil {
			return
		}
		if doc.Model == nil {
			continue
		}
		if doc.Root == nil {
			pass.ReportError(fmt.Errorf("check %s requires source parsed with LoadDocument: %s", c.ID(), doc.Path))
			continue
		}
		file, _ := filepath.Abs(doc.Path)
		files := graphFiles(ctx)
		files[file] = unwrap(doc.Root)
		g := graph{root: pass.Root, files: files}
		root := site{n: unwrap(doc.Root), file: file, ptr: "#"}
		for _, op := range g.operations(root) {
			id := op.op.child("operationId").value()
			message := ""
			if id == "" && c.required {
				message = "operation is missing a required operationId"
			} else if id != "" && c.pattern != nil && !c.pattern.MatchString(id) {
				message = fmt.Sprintf("operationId %q must match %s", id, c.pattern)
			}
			if message != "" {
				pointer := op.op.ptr
				if op.op.file != file {
					if source, ok := g.sources[op.op.file]; ok {
						pointer = source.ptr
					}
				}
				pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: pointer, Line: doc.Line(pointer), RuleID: c.ID(), Message: message, Severity: linter.SeverityError})
			}
		}
		if g.err != nil {
			pass.ReportError(g.err)
		}
	}
}

// AdaptSchemaRule gives standalone-schema checks the same rule-pack handling.
func AdaptSchemaRule(rule linter.SchemaRule) linter.Analyzer { return schemaVisitor{rule} }

type schemaVisitor struct{ rule linter.SchemaRule }

var _ linter.Analyzer = schemaVisitor{}

func (v schemaVisitor) ID() string { return v.rule.Name() }
func (v schemaVisitor) Analyze(ctx context.Context, pass *linter.Pass) {
	for _, doc := range pass.Documents {
		if ctx.Err() != nil {
			return
		}
		if doc.Schema == nil {
			continue
		}
		for _, finding := range v.rule.VisitSchema(doc.Path, doc.Schema) {
			pointer := finding.Pointer
			if pointer == "" {
				pointer = "#"
			}
			pass.Report(linter.Diagnostic{Context: finding.Path, Path: doc.Path, Pointer: pointer, Line: doc.Line(pointer), RuleID: finding.RuleName, Message: finding.Message, Severity: linter.SeverityError})
		}
	}
}
