package rulepack

import (
	"context"
	"encoding/json"
	"fmt"
	linter "github.com/portpowered/openapi-linter"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
	"strings"
)

type offlineSchemas struct{ resources map[string]any }

func (l offlineSchemas) Load(uri string) (any, error) {
	if resource, ok := l.resources[uri]; ok {
		return resource, nil
	}
	return nil, fmt.Errorf("schema resource %s unavailable; network loading is disabled", uri)
}

// Compile a bounded schema graph with reference identities preserved, including
// recursion. OpenAPI 3.0 nullable/exclusive bounds are translated to draft 7.
func schemaResource(g *graph, s site, is30, request bool, resources map[string]any) any {
	if s.n == nil {
		return map[string]any{}
	}
	if s.child("$ref").nonblank() {
		target := g.resolve(s)
		uri := "urn:lint:schema:" + fmt.Sprintf("%x", []byte(target.file+target.ptr))
		if _, ok := resources[uri]; !ok {
			resources[uri] = map[string]any{}
			resources[uri] = schemaResource(g, target, is30, request, resources)
		}
		out := map[string]any{"$ref": uri}
		if !is30 {
			copy := *s.n
			copy.Content = nil
			for i := 0; i+1 < len(s.n.Content); i += 2 {
				if s.n.Content[i].Value != "$ref" {
					copy.Content = append(copy.Content, s.n.Content[i], s.n.Content[i+1])
				}
			}
			siblings := schemaResource(g, site{n: &copy, file: s.file, ptr: s.ptr}, is30, request, resources)
			if fields, ok := siblings.(map[string]any); ok {
				for key, value := range fields {
					out[key] = value
				}
			}
		}
		return out
	}
	if s.n.Kind == 8 {
		var value any
		_ = s.n.Decode(&value)
		return value
	}
	out := map[string]any{}
	for _, p := range s.pairs() {
		switch p.key {
		case "properties", "patternProperties", "$defs", "definitions":
			values := map[string]any{}
			for _, entry := range p.value.pairs() {
				values[entry.key] = schemaResource(g, entry.value, is30, request, resources)
			}
			out[p.key] = values
		case "items", "additionalProperties", "unevaluatedProperties", "not", "if", "then", "else", "contains":
			out[p.key] = schemaResource(g, p.value, is30, request, resources)
		case "oneOf", "anyOf", "allOf", "prefixItems":
			values := []any{}
			for _, entry := range p.value.list() {
				values = append(values, schemaResource(g, entry, is30, request, resources))
			}
			out[p.key] = values
		case "nullable", "discriminator", "xml", "example", "examples", "externalDocs": // OpenAPI annotations do not constrain JSON values.
		default:
			var value any
			if e := p.value.n.Decode(&value); e == nil {
				out[p.key] = value
			}
		}
	}
	if is30 {
		if s.child("nullable").value() == "true" && s.child("type").nonblank() {
			out["type"] = []any{s.child("type").value(), "null"}
		}
		for _, bound := range []string{"Minimum", "Maximum"} {
			key := "exclusive" + bound
			if s.child(key).value() == "true" {
				out[key] = out[strings.ToLower(bound)]
				delete(out, strings.ToLower(bound))
			} else if s.child(key).n != nil {
				delete(out, key)
			}
		}
	}
	// Required readOnly fields are not required in requests; writeOnly fields
	// are not required in responses. They remain typed if an example supplies one.
	if req := s.child("required"); req.n != nil {
		required := []any{}
		props := g.properties(s)
		for _, entry := range req.list() {
			field := g.resolve(props[entry.value()])
			skip := request && field.child("readOnly").value() == "true" || !request && field.child("writeOnly").value() == "true"
			if !skip {
				required = append(required, entry.value())
			}
		}
		out["required"] = required
	}
	return out
}
func (c apiCheck) validateExamples(ctx context.Context, g *graph, root site, emit func(site, string), pass *linter.Pass) {
	dialect := root.child("jsonSchemaDialect").value()
	if dialect != "" && dialect != "https://spec.openapis.org/oas/3.1/dialect/base" && dialect != "https://json-schema.org/draft/2020-12/schema" {
		pass.ReportError(fmt.Errorf("unsupported jsonSchemaDialect %s", dialect))
		return
	}
	is30 := strings.HasPrefix(root.child("openapi").value(), "3.0")
	validate := func(schema, example site, request bool) {
		if example.n == nil || schema.n == nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		resources := map[string]any{}
		resource := schemaResource(g, schema, is30, request, resources)
		compiler := jsonschema.NewCompiler()
		compiler.UseLoader(offlineSchemas{resources})
		if is30 {
			compiler.DefaultDraft(jsonschema.Draft7)
		}
		compiler.AssertFormat()
		if e := compiler.AddResource("urn:lint:example", resource); e != nil {
			pass.ReportError(e)
			return
		}
		for uri, value := range resources {
			if e := compiler.AddResource(uri, value); e != nil {
				pass.ReportError(e)
				return
			}
		}
		compiled, e := compiler.Compile("urn:lint:example")
		if e != nil {
			pass.ReportError(fmt.Errorf("cannot validate schema at %s: %w", schema.ptr, e))
			return
		}
		var raw any
		if e = example.n.Decode(&raw); e != nil {
			pass.ReportError(e)
			return
		}
		encoded, e := json.Marshal(raw)
		if e != nil {
			pass.ReportError(e)
			return
		}
		var value any
		if e = json.Unmarshal(encoded, &value); e != nil {
			pass.ReportError(e)
			return
		}
		if e = compiled.Validate(value); e != nil {
			emit(example, "example does not match schema: "+e.Error())
		}
	}
	examples := func(mt site, request bool) {
		validate(mt.child("schema"), mt.child("example"), request)
		for _, e := range mt.child("examples").pairs() {
			entry := g.resolve(e.value)
			if entry.child("externalValue").n == nil {
				validate(mt.child("schema"), entry.child("value"), request)
			}
		}
	}
	for _, op := range g.operations(root) {
		g.bodies(op.op, true, func(mt site) { examples(mt, true) })
		for _, response := range op.op.child("responses").pairs() {
			for _, mt := range g.resolve(response.value).child("content").pairs() {
				examples(mt.value, false)
			}
		}
		for _, parameters := range []site{op.item.child("parameters"), op.op.child("parameters")} {
			for _, p := range parameters.list() {
				examples(g.resolve(p), true)
			}
		}
	}
	visited := map[*yaml.Node]bool{}
	var walk func(site)
	walk = func(s site) {
		if s.n == nil || visited[s.n] {
			return
		}
		visited[s.n] = true
		if s.child("type").n != nil || s.child("properties").n != nil {
			validate(s, s.child("example"), false)
			for _, e := range s.child("examples").list() {
				validate(s, e, false)
			}
		}
		resolved := g.resolve(s)
		if resolved.n != s.n {
			walk(resolved)
		}
		for _, p := range s.pairs() {
			if p.key != "example" && p.key != "examples" {
				walk(p.value)
			}
		}
		for _, item := range s.list() {
			walk(item)
		}
	}
	walk(root)
}
