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

func portosResponseRule(id string) bool {
	switch id {
	case "portos.success-status", "portos.etag-conflict", "portos.error-contract", "portos.path-kebab-case", "portos.delete-idempotent", "portos.version-prefix", "portos.query-sorts", "portos.batch-outcomes":
		return true
	}
	return false
}

func optionsFor(id string) apiOptions {
	o := defaultOptions()
	if id == "portos.error-contract" || id == "portos.batch-outcomes" {
		o.ErrorSchema = "ErrorResponse"
	}
	return o
}

func enumExactly(s site, values ...string) bool {
	entries := s.child("enum").list()
	if len(entries) != len(values) {
		return false
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !contains(values, e.value()) || seen[e.value()] {
			return false
		}
		seen[e.value()] = true
	}
	return true
}

// Wire-protocol discriminants are deliberately closed; business enums stay open.
func portosFixedEnum(s site) bool {
	fixed := strings.HasSuffix(s.ptr, "/properties/direction") && s.child("type").value() == "string" && enumExactly(s, "ASCENDING", "DESCENDING") || strings.HasSuffix(s.ptr, "/properties/family") && s.child("type").value() == "integer" && enumExactly(s, "400", "500") && s.child("enum").list()[0].n.Tag == "!!int" && s.child("enum").list()[1].n.Tag == "!!int"
	if !fixed {
		return false
	}
	names := s.child("x-enum-varnames").list()
	if len(names) != 2 {
		return false
	}
	return names[0].value() != names[1].value() && regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(names[0].value()) && regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(names[1].value())
}

func requiredProperties(g *graph, schema site) map[string]bool {
	required := map[string]bool{}
	visited := map[*yaml.Node]bool{}
	var walk func(site)
	walk = func(s site) {
		s = g.resolve(s)
		if s.n == nil || visited[s.n] {
			return
		}
		visited[s.n] = true
		for _, p := range s.child("required").list() {
			required[p.value()] = true
		}
		for _, p := range s.child("allOf").list() {
			walk(p)
		}
	}
	walk(schema)
	return required
}

func errorShape(g *graph, schema site) bool {
	if !g.object(schema) {
		return false
	}
	p := g.properties(schema)
	required := requiredProperties(g, schema)
	for _, key := range []string{"code", "message", "type", "family"} {
		if !required[key] {
			return false
		}
	}
	for _, key := range []string{"code", "message", "type"} {
		if g.resolve(p[key]).child("type").value() != "string" {
			return false
		}
	}
	message := g.resolve(p["message"])
	if message.child("enum").n != nil || message.child("const").n != nil {
		return false
	}
	family := g.resolve(p["family"])
	return family.child("type").value() == "integer" && enumExactly(family, "400", "500") && family.child("enum").list()[0].n.Tag == "!!int" && family.child("enum").list()[1].n.Tag == "!!int"
}

func sortShape(g *graph, schema site) bool {
	schema = g.resolve(schema)
	p := g.properties(schema)
	required := requiredProperties(g, schema)
	return g.object(schema) && len(p) == 2 && required["direction"] && required["key"] && (schema.child("additionalProperties").value() == "false" || schema.child("unevaluatedProperties").value() == "false") && g.resolve(p["key"]).child("type").value() == "string" && g.resolve(p["direction"]).child("type").value() == "string" && enumExactly(g.resolve(p["direction"]), "ASCENDING", "DESCENDING")
}

func (c apiCheck) analyzePortosResponses(ctx context.Context, pass *linter.Pass) {
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
		emit := func(s site, message string) {
			pointer := s.ptr
			if s.file != file {
				pointer = "#"
				if source, ok := g.sources[s.file]; ok && source.file == file {
					pointer = source.ptr
				}
				message += " (" + s.file + s.ptr + ")"
			}
			pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: pointer, Line: doc.Line(pointer), RuleID: c.id, Message: message, Severity: linter.SeverityError})
		}
		if c.id == "portos.path-kebab-case" || c.id == "portos.version-prefix" {
			for _, entry := range root.child("paths").pairs() {
				if strings.HasPrefix(entry.key, "x-") {
					continue
				}
				item := g.resolve(entry.value)
				if c.id == "portos.version-prefix" {
					if !regexp.MustCompile(`^/v[1-9][0-9]*(?:/|$)`).MatchString(entry.key) {
						emit(item, "paths must begin with a major version prefix such as /v2/groups")
					}
				} else if strings.Trim(entry.key, "/") != "" {
					for _, segment := range strings.Split(strings.Trim(entry.key, "/"), "/") {
						if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
							continue
						}
						if !regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`).MatchString(segment) {
							emit(item, "literal path segments must use lowercase kebab-case")
							break
						}
					}
				}
			}
			if g.err != nil {
				pass.ReportError(g.err)
			}
			continue
		}
		for _, op := range g.operations(root) {
			if ctx.Err() != nil {
				return
			}
			if !op.requestPath {
				continue
			}
			base, batch, async := classify(op, c.options)
			responses := op.op.child("responses")
			switch c.id {
			case "portos.success-status":
				valid := false
				for _, r := range responses.pairs() {
					if r.key == "200" || r.key == "202" {
						valid = true
					} else if success(r.key) {
						emit(r.value, "Portos success responses must use exactly 200 or 202")
					}
				}
				if !valid {
					emit(op.op, "declare a 200 or 202 success response")
				}
			case "portos.etag-conflict":
				conditional := false
				for _, list := range []site{op.item.child("parameters"), op.op.child("parameters")} {
					for _, raw := range list.list() {
						p := g.resolve(raw)
						conditional = conditional || p.child("in").value() == "header" && (strings.EqualFold(p.child("name").value(), "If-Match") || strings.EqualFold(p.child("name").value(), "If-None-Match"))
					}
				}
				if conditional {
					if responses.child("409").n == nil {
						emit(op.op, "conditional ETag requests must declare 409 Conflict")
					}
					if responses.child("412").n != nil {
						emit(responses.child("412"), "Portos ETag conflicts use 409, not 412")
					}
				}
			case "portos.error-contract":
				for _, r := range responses.pairs() {
					if strings.HasPrefix(r.key, "4") || strings.HasPrefix(r.key, "5") || r.key == "default" {
						response := g.resolve(r.value)
						count := 0
						for _, mt := range response.child("content").pairs() {
							count++
							schema := mt.value.child("schema")
							if !g.named(schema, c.options.ErrorSchema) || !errorShape(&g, schema) {
								emit(mt.value, "error responses must reference "+c.options.ErrorSchema+" with required string code, message, type and integer family enum [400, 500]")
							}
						}
						if count == 0 {
							emit(r.value, "error response requires a body referencing "+c.options.ErrorSchema)
						}
					}
				}
			case "portos.delete-idempotent":
				if base == "Delete" || op.method == "delete" {
					if op.op.child("x-portos-idempotent").value() != "true" {
						emit(op.op, "delete operations must declare x-portos-idempotent: true; deleting a missing resource succeeds")
					}
					for _, status := range []string{"404", "410"} {
						if responses.child(status).n != nil {
							emit(responses.child(status), "idempotent delete must not fail for an absent resource")
						}
					}
				}
			case "portos.query-sorts":
				if base == "Query" {
					g.bodies(op.op, true, func(mt site) {
						visited := map[*yaml.Node]bool{}
						var walk func(site)
						walk = func(schema site) {
							schema = g.resolve(schema)
							if schema.n == nil || visited[schema.n] {
								return
							}
							visited[schema.n] = true
							for key, field := range g.properties(schema) {
								if key == c.options.Sorts {
									array := g.resolve(field)
									if array.child("type").value() != "array" || !sortShape(&g, array.child("items")) {
										emit(field, "query sorts must be an array of closed objects with required key:string and direction: ASCENDING or DESCENDING")
									}
								}
								walk(field)
							}
							walk(schema.child("items"))
						}
						walk(mt.child("schema"))
					})
				}
			case "portos.batch-outcomes":
				if batch && !async {
					idType := ""
					invalidRequest := false
					g.bodies(op.op, true, func(mt site) {
						body := mt.child("schema")
						array := g.resolve(g.properties(body)[c.options.Items])
						item := array.child("items")
						if array.child("type").value() != "array" || !g.object(item) || !requiredProperties(&g, body)[c.options.Items] || !requiredProperties(&g, item)[c.options.ItemID] {
							invalidRequest = true
						}
						typ := g.resolve(g.properties(item)[c.options.ItemID]).child("type").value()
						if (typ != "string" && typ != "integer") || (idType != "" && idType != typ) {
							invalidRequest = true
						}
						idType = typ
					})
					count := 0
					g.bodies(op.op, false, func(mt site) {
						count++
						schema := mt.child("schema")
						p := g.properties(schema)
						required := requiredProperties(&g, schema)
						valid := g.object(schema) && required[c.options.Results] && required[c.options.Errors] && idType != "" && !invalidRequest
						for _, field := range []string{c.options.Results, c.options.Errors} {
							array := g.resolve(p[field])
							item := array.child("items")
							props := g.properties(item)
							req := requiredProperties(&g, item)
							valid = valid && array.child("type").value() == "array" && (array.child("minItems").n == nil || array.child("minItems").value() == "0") && g.object(item) && req[c.options.ItemID] && g.resolve(props[c.options.ItemID]).child("type").value() == idType
							if field == c.options.Errors {
								valid = valid && req["error"] && g.named(props["error"], c.options.ErrorSchema) && errorShape(&g, props["error"])
							}
						}
						if !valid {
							emit(mt, "synchronous batch responses require results and errors arrays; every entry requires the request item ID and each error requires a shared "+c.options.ErrorSchema+" object")
						}
					})
					if count == 0 {
						emit(op.op, "synchronous batch requires a structured results and errors response")
					}
				}
			}
		}
		if g.err != nil {
			pass.ReportError(g.err)
		}
	}
}
