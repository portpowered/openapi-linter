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

// Google checks inspect the REST/JSON projection of AIPs, not protobuf annotations.
var googleChecks = []string{
	"google.method-name", "google.http-method", "google.version-path",
	"google.standard-path", "google.request-body", "google.resource-response",
	"google.list-request", "google.list-response", "google.update-mask",
	"google.custom-method", "google.resource-name", "google.field-names",
	"google.boolean-names", "google.timestamp", "google.error-response",
	"google.long-running-response",
}

func registerGoogle(r *Registry) error {
	for _, id := range googleChecks {
		if err := r.Register(id, googleFactory(id)); err != nil {
			return err
		}
		if err := r.SetDescriptor(Descriptor{ID: id, Kind: "openapi", Category: "google", Title: strings.ReplaceAll(strings.TrimPrefix(id, "google."), "-", " "), Severity: "warning", Presets: []string{"google-defaults", "openapi:google"}, Options: map[string]string{}, Defaults: map[string]any{}, Guidance: "See docs/google-api.md for AIP sources, examples, paired checks, and REST projection limits."}); err != nil {
			return err
		}
	}
	return nil
}

type googleCheck struct{ id string }

func (c googleCheck) ID() string { return c.id }

func googleFactory(id string) Factory {
	return func(n yaml.Node) (linter.Analyzer, error) {
		if n.Kind != 0 && (n.Kind != yaml.MappingNode || len(n.Content) != 0) {
			return nil, fmt.Errorf("check %s accepts no options", id)
		}
		return googleCheck{id}, nil
	}
}

var googleMethodName = regexp.MustCompile(`^[A-Z][a-z]+(?:[A-Z][A-Za-z0-9]*)+$`)
var googleFieldName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
var googleVersionPath = regexp.MustCompile(`^/v[1-9][0-9]*(?:(?:alpha|beta)(?:[1-9][0-9]*)?)?(?:/|$)`)
var googleVariable = regexp.MustCompile(`\{[^{}]+\}$`)

func googleStandard(op operation) string {
	if strings.Contains(op.path, ":") {
		return ""
	}
	id := op.op.child("operationId").value()
	for _, verb := range []string{"Get", "List", "Create", "Update", "Delete"} {
		if strings.HasPrefix(id, verb) && len(id) > len(verb) && id[len(verb)] >= 'A' && id[len(verb)] <= 'Z' {
			return verb
		}
	}
	return ""
}

func googleParameters(g *graph, op operation) map[string]site {
	result := map[string]site{}
	for _, list := range []site{op.item.child("parameters"), op.op.child("parameters")} {
		for _, raw := range list.list() {
			p := g.resolve(raw)
			result[p.child("in").value()+":"+p.child("name").value()] = p
		}
	}
	return result
}

func googleSchemaType(g *graph, s site, want string) bool {
	return g.resolve(s).child("type").value() == want
}

func googleResourceName(g *graph, s site) bool {
	return g.object(s) && googleSchemaType(g, g.properties(s)["name"], "string")
}

func (c googleCheck) Analyze(ctx context.Context, pass *linter.Pass) {
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
		file, err := filepath.Abs(doc.Path)
		if err != nil {
			pass.ReportError(err)
			continue
		}
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
			pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: pointer, Line: doc.Line(pointer), RuleID: c.id, Severity: linter.SeverityWarning, Message: message})
		}
		visited := map[*yaml.Node]bool{}
		var schema func(site)
		schema = func(s site) {
			s = g.resolve(s)
			if s.n == nil || visited[s.n] {
				return
			}
			visited[s.n] = true
			if c.id == "google.resource-name" && s.child("x-google-resource").value() == "true" && !googleResourceName(&g, s) {
				emit(s, "a schema marked x-google-resource requires a string name property (AIP-122)")
			}
			for _, p := range s.child("properties").pairs() {
				switch c.id {
				case "google.field-names":
					if p.key != "@type" && !googleFieldName.MatchString(p.key) {
						emit(p.value, "JSON property "+p.key+" must use lowerCamelCase (AIP-140 / ProtoJSON)")
					}
				case "google.boolean-names":
					if googleSchemaType(&g, p.value, "boolean") && len(p.key) > 2 && strings.HasPrefix(p.key, "is") && p.key[2] >= 'A' && p.key[2] <= 'Z' && p.key != "isNew" {
						emit(p.value, "boolean property "+p.key+" should omit the is prefix (AIP-140)")
					}
				case "google.timestamp":
					if strings.HasSuffix(p.key, "Time") && (!googleSchemaType(&g, p.value, "string") || g.resolve(p.value).child("format").value() != "date-time") {
						emit(p.value, "timestamp property "+p.key+" requires string format date-time (AIP-142)")
					}
				}
				schema(p.value)
			}
			for _, key := range []string{"items", "additionalProperties", "not"} {
				schema(s.child(key))
			}
			for _, key := range []string{"allOf", "anyOf", "oneOf"} {
				for _, child := range s.child(key).list() {
					schema(child)
				}
			}
		}
		for _, s := range root.child("components").child("schemas").pairs() {
			schema(s.value)
		}
		for _, op := range g.operations(root) {
			if !op.requestPath {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			base := googleStandard(op)
			id := op.op.child("operationId").value()
			params := googleParameters(&g, op)
			for _, p := range params {
				schema(p.child("schema"))
			}
			g.bodies(op.op, true, func(mt site) { schema(mt.child("schema")) })
			g.bodies(op.op, false, func(mt site) { schema(mt.child("schema")) })
			response := func(check func(site)) {
				count := 0
				g.bodies(op.op, false, func(mt site) { count++; check(mt.child("schema")) })
				if count == 0 {
					emit(op.op, "provide a structured success response for "+id)
				}
			}
			switch c.id {
			case "google.method-name":
				if !googleMethodName.MatchString(id) || contains(operationWords(id), "Async") {
					emit(op.op, "operationId must use UpperCamelCase VerbNoun without Async (AIP-136/190)")
				}
			case "google.http-method":
				want := map[string]string{"Get": "get", "List": "get", "Create": "post", "Update": "patch", "Delete": "delete"}[base]
				if want != "" && op.method != want {
					emit(op.op, base+" must use "+strings.ToUpper(want)+" (AIP-131 through 135)")
				}
			case "google.version-path":
				if !googleVersionPath.MatchString(op.path) {
					emit(op.op, "request path must begin with /vN, optionally alphaN or betaN (AIP-185)")
				}
			case "google.standard-path":
				member := googleVariable.MatchString(op.path)
				singleton := op.op.child("x-google-singleton").value() == "true"
				if singleton && (base == "Create" || base == "Delete") {
					emit(op.op, "singleton resources must not declare Create or Delete (AIP-156)")
				}
				if ((base == "Get" || base == "Update" || base == "Delete") && !member && !singleton) || ((base == "List" || base == "Create") && member) {
					emit(op.op, "standard method must target a resource variable or collection literal as appropriate (AIP-131 through 135)")
				}
			case "google.request-body":
				body := g.resolve(op.op.child("requestBody"))
				if (base == "Get" || base == "List" || base == "Delete" || op.method == "get") && body.n != nil {
					emit(op.op, "read and Delete methods must omit requestBody (AIP-131/132/135)")
				}
				if base == "Create" || base == "Update" {
					count := 0
					g.bodies(op.op, true, func(mt site) {
						count++
						if !g.object(mt.child("schema")) {
							emit(mt, "Create/Update body must be a resource object (AIP-133/134)")
						}
					})
					if count == 0 || body.child("required").value() != "true" {
						emit(op.op, "Create/Update requires a required resource requestBody (AIP-133/134)")
					}
				}
			case "google.resource-response", "google.resource-name":
				if (base == "Get" || base == "Create" || base == "Update") && op.op.child("x-google-long-running").value() != "true" {
					response(func(s site) {
						if c.id == "google.resource-response" && !g.object(s) {
							emit(s, "standard resource method must return an object (AIP-131/133/134)")
						}
						if c.id == "google.resource-name" && !googleResourceName(&g, s) {
							emit(s, "resource response requires a string name property (AIP-122)")
						}
					})
				}
			case "google.list-request":
				if base == "List" {
					for name, typ := range map[string]string{"pageSize": "integer", "pageToken": "string"} {
						p := params["query:"+name]
						if p.n == nil || !googleSchemaType(&g, p.child("schema"), typ) || p.child("required").value() == "true" {
							emit(op.op, "List requires optional "+typ+" query parameter "+name+" (AIP-132/158)")
						}
					}
				}
			case "google.list-response":
				if base == "List" {
					response(func(s site) {
						arrays := 0
						for _, p := range g.properties(s) {
							if googleSchemaType(&g, p, "array") && g.object(g.resolve(p).child("items")) {
								arrays++
							}
						}
						if !g.object(s) || arrays != 1 || !googleSchemaType(&g, g.properties(s)["nextPageToken"], "string") {
							emit(s, "List response requires one resource array and string nextPageToken (AIP-132/158)")
						}
					})
				}
			case "google.update-mask":
				if base == "Update" {
					p := params["query:updateMask"]
					if p.n == nil || !googleSchemaType(&g, p.child("schema"), "string") || p.child("required").value() == "true" {
						emit(op.op, "Update requires an optional string updateMask query parameter (AIP-134/161 ProtoJSON)")
					}
				}
			case "google.custom-method":
				if base == "" {
					parts := strings.Split(op.path, ":")
					words := operationWords(id)
					verb := ""
					if len(words) != 0 {
						verb = strings.ToLower(words[0][:1]) + words[0][1:]
					}
					fullName := ""
					if id != "" {
						fullName = strings.ToLower(id[:1]) + id[1:]
					}
					if len(parts) != 2 || (parts[1] != verb && parts[1] != fullName) || (op.method != "get" && op.method != "post") {
						emit(op.op, "custom method requires GET/POST and a :verb or stateless :verbNoun matching operationId (AIP-136)")
					}
					if !strings.HasSuffix(id, "LongRunning") && contains([]string{"get", "list", "create", "update", "delete"}, verb) {
						emit(op.op, "custom method should avoid standard method verbs (AIP-136)")
					}
					for _, word := range words {
						if contains([]string{"For", "With", "At", "By", "To", "From", "Of", "In", "On"}, word) {
							emit(op.op, "custom method name must omit preposition "+word+" (AIP-136)")
						}
					}
				}
			case "google.error-response":
				count := 0
				for _, resp := range op.op.child("responses").pairs() {
					if !strings.HasPrefix(resp.key, "4") && !strings.HasPrefix(resp.key, "5") && resp.key != "default" {
						continue
					}
					count++
					content := g.resolve(resp.value).child("content").child("application/json")
					errorSchema := g.properties(content.child("schema"))["error"]
					props := g.properties(errorSchema)
					if !g.object(errorSchema) || !googleSchemaType(&g, props["code"], "integer") || !googleSchemaType(&g, props["message"], "string") {
						emit(resp.value, "JSON error response requires error object with integer code and string message (AIP-193)")
					}
				}
				if count == 0 {
					emit(op.op, "document at least one HTTP error response using the Google JSON error envelope (AIP-193)")
				}
			case "google.long-running-response":
				if op.op.child("x-google-long-running").value() == "true" {
					response(func(s site) {
						props := g.properties(s)
						if !g.object(s) || !googleSchemaType(&g, props["name"], "string") || !googleSchemaType(&g, props["done"], "boolean") || !g.object(props["error"]) || !g.object(props["response"]) {
							emit(s, "long-running success schema requires name, done, error, and response fields with Operation types (AIP-151)")
						}
					})
				}
			}
		}
		if g.err != nil {
			pass.ReportError(g.err)
		}
	}
}
