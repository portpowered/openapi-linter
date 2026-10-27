package rulepack

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
)

func portosUpsertRule(id string) bool {
	switch id {
	case "portos.write-methods", "portos.upsert-headers", "portos.path-patterns", "portos.message-request":
		return true
	}
	return false
}

func validateUpsertOptions(id string, options apiOptions) error {
	if id == "portos.upsert-headers" && strings.TrimSpace(options.IdempotencyHeader) == "" {
		return fmt.Errorf("idempotency-header must not be blank")
	}
	var patterns []string
	switch id {
	case "portos.path-patterns":
		patterns = options.Patterns
	case "portos.message-request":
		patterns = options.Paths
		if len(options.Statuses) == 0 {
			return fmt.Errorf("statuses must not be empty")
		}
		seen := map[string]bool{}
		for _, status := range options.Statuses {
			if strings.TrimSpace(status) == "" || seen[status] {
				return fmt.Errorf("statuses must contain unique non-blank values")
			}
			seen[status] = true
		}
	default:
		return nil
	}
	if len(patterns) == 0 {
		return fmt.Errorf("%s requires non-empty path patterns", id)
	}
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("path patterns must not be blank")
		}
		if id == "portos.path-patterns" && (!strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$")) {
			return fmt.Errorf("path patterns must be anchored with ^ and $")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid path pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func matchesAnyPath(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(path) {
			return true
		}
	}
	return false
}

// Source suffixes participate in baseline fingerprints, so they must survive relocation.
func sourceLocation(root string, source site) string {
	absolute, err := filepath.Abs(root)
	if err == nil {
		if relative, err := filepath.Rel(absolute, source.file); err == nil {
			return filepath.ToSlash(relative) + source.ptr
		}
	}
	return filepath.Base(source.file) + source.ptr
}

func operationEmitter(pass *linter.Pass, doc *linter.Document, root site, op operation, ruleID string) func(site, string) {
	return func(s site, message string) {
		pointer := s.ptr
		if s.file != root.file {
			// Each root path and method gets its own pointer even when references share a file.
			pointer = root.child("paths").child(op.path).child(op.method).ptr
			if s.file == op.op.file && (s.ptr == op.op.ptr || strings.HasPrefix(s.ptr, op.op.ptr+"/")) {
				pointer += strings.TrimPrefix(s.ptr, op.op.ptr)
			}
			message += " (" + sourceLocation(pass.Root, s) + ")"
		}
		line := doc.Line(pointer)
		if line == 0 {
			line = doc.Line(root.child("paths").child(op.path).ptr)
		}
		pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: pointer, Line: line, RuleID: ruleID, Message: message, Severity: linter.SeverityError})
	}
}

func (c apiCheck) analyzePortosUpsert(ctx context.Context, pass *linter.Pass) {
	for _, doc := range pass.Documents {
		if ctx.Err() != nil {
			return
		}
		if doc.Model == nil {
			continue
		}
		if doc.Root == nil {
			pass.ReportError(fmt.Errorf("check %s requires source parsed with LoadDocument: %s", c.id, doc.Path))
			continue
		}
		file, _ := filepath.Abs(doc.Path)
		root := site{n: unwrap(doc.Root), file: file, ptr: "#"}
		files := graphFiles(ctx)
		files[file] = root.n
		g := graph{root: pass.Root, files: files}
		if c.id == "portos.path-patterns" {
			for _, entry := range root.child("paths").pairs() {
				if !strings.HasPrefix(entry.key, "x-") && !matchesAnyPath(entry.key, c.options.Patterns) {
					pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: entry.value.ptr, Line: doc.Line(entry.value.ptr), RuleID: c.id, Message: "path must match one of the configured anchored patterns", Severity: linter.SeverityError})
				}
			}
		} else {
			for _, op := range g.operations(root) {
				if ctx.Err() != nil {
					return
				}
				if op.requestPath {
					c.checkUpsertOperation(&g, op, operationEmitter(pass, doc, root, op, c.id))
				}
			}
		}
		if g.err != nil {
			pass.ReportError(g.err)
		}
	}
}

func (c apiCheck) checkUpsertOperation(g *graph, op operation, emit func(site, string)) {
	switch c.id {
	case "portos.write-methods":
		if op.method != "get" && op.method != "head" && op.method != "options" && op.method != "post" && op.method != "delete" {
			emit(op.op, "only GET, HEAD, OPTIONS, POST and DELETE are allowed; replace PUT/PATCH writes with a collection POST")
		}
	case "portos.upsert-headers":
		if op.method != "post" && op.method != "delete" {
			return
		}
		if !requiredHeader(g, op, c.options.IdempotencyHeader) {
			emit(op.op, "POST and DELETE operations must declare a required "+c.options.IdempotencyHeader+" header")
		}
		segment := op.path[strings.LastIndex(op.path, "/")+1:]
		unconditional := op.method == "delete" || strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
		replace := unconditional
		g.bodies(op.op, true, func(mt site) {
			replace = replace || g.properties(mt.child("schema"))["id"].n != nil
		})
		if unconditional && !requiredHeader(g, op, "If-Match") {
			emit(op.op, "item POST and DELETE operations must declare a required If-Match header")
		} else if replace && !hasHeader(g, op, "If-Match") {
			emit(op.op, "id-bearing collection POST operations must declare an If-Match header for replacement")
		}
	case "portos.message-request":
		if matchesAnyPath(op.path, c.options.Paths) {
			c.checkMessageRequest(g, op, emit)
		}
	}
}

func (c apiCheck) checkMessageRequest(g *graph, op operation, emit func(site, string)) {
	response := g.resolve(op.op.child("responses").child("200"))
	if response.n == nil || len(response.child("content").pairs()) == 0 {
		emit(op.op, "message requests require a 200 response with results, errors and status")
		return
	}
	for _, media := range response.child("content").pairs() {
		schema := g.resolve(media.value.child("schema"))
		if !messageEnvelopeObject(g, schema) {
			emit(schema, "message request response must be an object envelope")
			continue
		}
		properties := g.properties(schema)
		required := requiredProperties(g, schema)
		for _, field := range []string{"results", "errors"} {
			if !required[field] || g.resolve(properties[field]).child("type").value() != "array" {
				emit(schema, "message request response requires a "+field+" array")
			}
		}
		status := g.resolve(properties["status"])
		if !required["status"] || status.child("type").value() != "string" || !enumExactly(status, c.options.Statuses...) {
			where := schema
			if status.n != nil {
				where = status
			}
			emit(where, "message request status must be a required string enum exactly matching ["+strings.Join(c.options.Statuses, ", ")+"]")
		}
	}
}

func messageEnvelopeObject(g *graph, schema site) bool {
	seen := map[*yaml.Node]bool{}
	object := false
	var walk func(site) bool
	walk = func(s site) bool {
		s = g.resolve(s)
		if s.n == nil || seen[s.n] {
			return true
		}
		seen[s.n] = true
		if typ := s.child("type"); typ.n != nil {
			if typ.value() != "object" {
				return false
			}
			object = true
		}
		for _, parent := range s.child("allOf").list() {
			if !walk(parent) {
				return false
			}
		}
		return true
	}
	return walk(schema) && object
}
