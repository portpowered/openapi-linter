package rulepack

import (
	"context"
	"fmt"
	linter "github.com/portpowered/openapi-linter"
	"gopkg.in/yaml.v3"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type apiOptions struct {
	Operations           []string `yaml:"operations"`
	Modifiers            []string `yaml:"modifiers"`
	QuerySchema          string   `yaml:"query-schema"`
	NameSchema           string   `yaml:"name-schema"`
	DescriptionSchema    string   `yaml:"description-schema"`
	AsyncSchema          string   `yaml:"async-schema"`
	MaxResults           string   `yaml:"max-results-field"`
	NextToken            string   `yaml:"next-token-field"`
	Results              string   `yaml:"results-field"`
	Pagination           string   `yaml:"pagination-field"`
	ItemID               string   `yaml:"item-id-field"`
	Items                string   `yaml:"items-field"`
	Public               []string `yaml:"public-operations"`
	Protocols            []string `yaml:"protocols"`
	Hosts                []string `yaml:"hosts"`
	Classes              []string `yaml:"classes"`
	ErrorSchema          string   `yaml:"error-schema"`
	DateFields           []string `yaml:"date-fields"`
	CollectionOperations []string `yaml:"collection-operations"`
}
type apiCheck struct {
	id      string
	options apiOptions
}

func (c apiCheck) ID() string { return c.id }
func apiFactory(id string) Factory {
	return func(n yaml.Node) (linter.Analyzer, error) {
		if e := validateCheckOptions(id, n); e != nil {
			return nil, e
		}
		c := apiCheck{id: id, options: defaultOptions()}
		if e := DecodeOptions(n, &c.options); e != nil {
			return nil, e
		}
		if len(c.options.Operations) == 0 || len(c.options.Classes) == 0 {
			return nil, fmt.Errorf("operations and classes must not be empty")
		}
		for _, v := range c.options.Classes {
			if v != "2" && v != "3" && v != "4" && v != "5" {
				return nil, fmt.Errorf("invalid response class %q", v)
			}
		}
		if id == "openapi.pagination" && len(c.options.CollectionOperations) == 0 {
			return nil, fmt.Errorf("pagination requires collection-operations")
		}
		return c, nil
	}
}

// site retains the document owning relative refs and a pointer into that source.
type site struct {
	n         *yaml.Node
	file, ptr string
}

func unwrap(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	if n != nil && n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}
func (s site) child(key string) site {
	n := unwrap(s.n)
	out := site{file: s.file, ptr: s.ptr + "/" + linter.PointerSegment(key)}
	if n == nil {
		return out
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				out.n = n.Content[i+1]
				break
			}
		}
	} else if n.Kind == yaml.SequenceNode {
		index, e := strconv.Atoi(key)
		if e == nil && index >= 0 && index < len(n.Content) {
			out.n = n.Content[index]
		}
	}
	return out
}
func (s site) value() string {
	if s.n == nil {
		return ""
	}
	return s.n.Value
}
func (s site) pairs() []struct {
	key   string
	value site
} {
	out := []struct {
		key   string
		value site
	}{}
	n := unwrap(s.n)
	if n != nil && n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			out = append(out, struct {
				key   string
				value site
			}{n.Content[i].Value, s.child(n.Content[i].Value)})
		}
	}
	return out
}
func (s site) list() []site {
	out := []site{}
	n := unwrap(s.n)
	if n != nil && n.Kind == yaml.SequenceNode {
		for i := range n.Content {
			out = append(out, s.child(strconv.Itoa(i)))
		}
	}
	return out
}
func (s site) nonblank() bool { return strings.TrimSpace(s.value()) != "" }
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

type graphContextKey struct{}

func withGraphCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, graphContextKey{}, map[string]*yaml.Node{})
}
func graphFiles(ctx context.Context) map[string]*yaml.Node {
	if cache, ok := ctx.Value(graphContextKey{}).(map[string]*yaml.Node); ok {
		return cache
	}
	return map[string]*yaml.Node{}
}

type graph struct {
	root    string
	files   map[string]*yaml.Node
	err     error
	sources map[string]site
}

func (g *graph) resolve(s site) site {
	original := s
	if src, ok := g.sources[s.file]; ok {
		original = src
	}
	seen := map[string]bool{}
	for s.n != nil && s.child("$ref").nonblank() {
		ref := s.child("$ref").value()
		u, e := url.Parse(ref)
		if e != nil || u.Scheme != "" || u.Host != "" || filepath.IsAbs(u.Path) {
			g.err = fmt.Errorf("unsupported reference %q", ref)
			return site{}
		}
		file := s.file
		if u.Path != "" {
			file = filepath.Join(filepath.Dir(file), filepath.FromSlash(u.Path))
		}
		file, _ = filepath.Abs(file)
		identity := file + "#" + u.Fragment
		if seen[identity] {
			return s
		}
		seen[identity] = true
		if e = linter.CheckPathRoot(g.root, file); e != nil {
			g.err = e
			return site{}
		}
		n, ok := g.files[file]
		if !ok {
			data, e := os.ReadFile(file)
			if e != nil {
				g.err = e
				return site{}
			}
			var parsed yaml.Node
			if e = yaml.Unmarshal(data, &parsed); e != nil {
				g.err = e
				return site{}
			}
			n = unwrap(&parsed)
			g.files[file] = n
		}
		if g.sources == nil {
			g.sources = map[string]site{}
		}
		if file != original.file {
			if _, ok := g.sources[file]; !ok {
				g.sources[file] = original
			}
		}
		s = site{n: n, file: file, ptr: "#"}
		if u.Fragment != "" {
			if !strings.HasPrefix(u.Fragment, "/") {
				g.err = fmt.Errorf("unsupported reference anchor %q", ref)
				return site{}
			}
			for _, key := range strings.Split(u.Fragment[1:], "/") {
				key = strings.ReplaceAll(strings.ReplaceAll(key, "~1", "/"), "~0", "~")
				s = s.child(key)
			}
		}
		if s.n == nil {
			g.err = fmt.Errorf("unresolved reference %q", ref)
		}
	}
	return s
}
func (g *graph) properties(s site) map[string]site {
	out := map[string]site{}
	visited := map[*yaml.Node]bool{}
	var visit func(site)
	visit = func(s site) {
		s = g.resolve(s)
		if s.n == nil || visited[s.n] {
			return
		}
		visited[s.n] = true
		for _, p := range s.child("properties").pairs() {
			out[p.key] = p.value
		}
		for _, parent := range s.child("allOf").list() {
			visit(parent)
		}
	}
	visit(s)
	return out
}
func (g *graph) object(s site) bool {
	s = g.resolve(s)
	return s.child("type").value() == "object" || len(g.properties(s)) > 0
}
func refName(s site, name string) bool {
	ref := s.child("$ref").value()
	return strings.HasSuffix(ref, "/"+name) || strings.HasSuffix(ref, "/"+name+".yaml") || strings.HasSuffix(ref, "/"+name+".yml") || ref == name+".yaml"
}
func (g *graph) named(s site, name string) bool {
	if refName(s, name) {
		return true
	}
	for _, p := range g.resolve(s).child("allOf").list() {
		if refName(p, name) {
			return true
		}
	}
	return false
}

type operation struct {
	path, method string
	item, op     site
	requestPath  bool
}

func (g *graph) operations(root site) []operation {
	out := []operation{}
	seen := map[*yaml.Node]bool{}
	var paths func(site, bool)
	paths = func(items site, requestPath bool) {
		for _, entry := range items.pairs() {
			item := g.resolve(entry.value)
			if item.n == nil || seen[item.n] {
				continue
			}
			seen[item.n] = true
			deferDelete := item.n
			for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"} {
				op := item.child(method)
				if op.n == nil {
					continue
				}
				out = append(out, operation{entry.key, method, item, op, requestPath})
				for _, cb := range op.child("callbacks").pairs() {
					paths(g.resolve(cb.value), false)
				}
			}
			delete(seen, deferDelete)
		}
	}
	paths(root.child("paths"), true)
	paths(root.child("webhooks"), false)
	return out
}
func success(code string) bool { return strings.HasPrefix(code, "2") }
func (g *graph) bodies(op site, request bool, fn func(site)) {
	if request {
		body := g.resolve(op.child("requestBody"))
		for _, m := range body.child("content").pairs() {
			fn(m.value)
		}
	} else {
		for _, resp := range op.child("responses").pairs() {
			if success(resp.key) {
				for _, m := range g.resolve(resp.value).child("content").pairs() {
					fn(m.value)
				}
			}
		}
	}
}
func operationWords(id string) []string {
	re := regexp.MustCompile(`[A-Z]+[a-z]*|[a-z]+|[0-9]+`)
	return re.FindAllString(id, -1)
}
func classify(op operation, options apiOptions) (base string, batch, async bool) {
	id := op.op.child("operationId").value()
	for _, word := range operationWords(id) {
		for _, allowed := range options.Operations {
			if strings.EqualFold(word, allowed) {
				if base != "" && base != allowed {
					return "", false, false
				}
				base = allowed
			}
		}
		batch = batch || (contains(options.Modifiers, "Batch") && strings.EqualFold(word, "batch"))
		async = async || (contains(options.Modifiers, "Async") && strings.EqualFold(word, "async"))
	}
	return
}

func (c apiCheck) Analyze(ctx context.Context, pass *linter.Pass) {
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
		root := site{n: unwrap(doc.Root), file: file, ptr: "#"}
		files := graphFiles(ctx)
		files[file] = root.n
		g := graph{root: pass.Root, files: files}
		emit := func(s site, msg string) {
			pointer := s.ptr
			if s.file != file {
				pointer = "#"
				if source, ok := g.sources[s.file]; ok && source.file == file {
					pointer = source.ptr
				}
				msg += " (" + s.file + s.ptr + ")"
			}
			pass.Report(linter.Diagnostic{Path: doc.Path, Pointer: pointer, Line: doc.Line(pointer), RuleID: c.id, Message: msg, Severity: linter.SeverityError})
		}
		operations := g.operations(root)
		ids := map[string]site{}
		shapes := map[string]site{}
		pathDone := map[string]bool{}
		for _, op := range operations {
			if ctx.Err() != nil {
				return
			}
			if strings.HasPrefix(c.id, "portos.") && !op.requestPath {
				continue
			}
			base, batch, async := classify(op, c.options)
			collection := base == "Query" || base == "List"
			location := op.op
			id := location.child("operationId").value()
			requireBody := func(fn func(site)) {
				count := 0
				g.bodies(location, true, func(mt site) { count++; fn(mt.child("schema")) })
				if count == 0 {
					emit(location, "operation requires a structured request body")
				}
			}
			requireResponse := func(fn func(site)) {
				count := 0
				g.bodies(location, false, func(mt site) { count++; fn(mt.child("schema")) })
				if count == 0 {
					emit(location, "operation requires a structured success response")
				}
			}
			switch c.id {
			case "openapi.operation-id-unique":
				if id != "" {
					if first, ok := ids[id]; ok {
						emit(location.child("operationId"), "operationId "+id+" duplicates "+first.ptr)
					} else {
						ids[id] = location.child("operationId")
					}
				}
			case "openapi.operation-summary", "openapi.operation-description":
				field := "summary"
				if c.id == "openapi.operation-description" {
					field = "description"
				}
				if !location.child(field).nonblank() {
					emit(location, "provide a nonblank operation "+field)
				}
			case "openapi.parameter-description", "openapi.path-parameters", "openapi.parameters-unique":
				effective := map[string]site{}
				for _, list := range []site{op.item.child("parameters"), location.child("parameters")} {
					seen := map[string]bool{}
					for _, raw := range list.list() {
						p := g.resolve(raw)
						key := p.child("in").value() + ":" + p.child("name").value()
						if seen[key] && c.id == "openapi.parameters-unique" {
							emit(raw, "duplicate parameter "+key)
						}
						seen[key] = true
						effective[key] = p
					}
				}
				if c.id == "openapi.parameter-description" {
					for _, p := range effective {
						if !p.child("description").nonblank() {
							emit(p, "provide parameter description")
						}
					}
				}
				if c.id == "openapi.path-parameters" && op.requestPath {
					re := regexp.MustCompile(`\{([^{}]*)\}`)
					names := map[string]bool{}
					for _, m := range re.FindAllStringSubmatch(op.path, -1) {
						names[m[1]] = true
						if m[1] == "" {
							emit(op.item, "empty path placeholder")
						}
						if _, ok := effective["path:"+m[1]]; !ok {
							emit(location, "undefined path parameter "+m[1])
						}
					}
					for key, p := range effective {
						if strings.HasPrefix(key, "path:") {
							if !names[p.child("name").value()] {
								emit(p, "unused path parameter")
							}
							if p.child("required").value() != "true" {
								emit(p, "path parameter must be required")
							}
						}
					}
				}
			case "openapi.path-syntax":
				if !op.requestPath {
					continue
				}
				if pathDone[op.path] {
					continue
				}
				pathDone[op.path] = true
				if strings.ContainsAny(op.path, "?#") {
					emit(op.item, "path keys must not contain query strings or fragments")
				}
				shape := regexp.MustCompile(`\{[^{}]*\}`).ReplaceAllString(op.path, "{}")
				if first, ok := shapes[shape]; ok && first.ptr != op.item.ptr {
					emit(op.item, "identical templated path also at "+first.ptr)
				} else {
					shapes[shape] = op.item
				}
			case "openapi.operation-success-response":
				found := false
				for _, r := range location.child("responses").pairs() {
					for _, class := range c.options.Classes {
						found = found || strings.HasPrefix(r.key, class)
					}
				}
				if !found {
					emit(location.child("responses"), "document a success response")
				}
			case "openapi.security-references", "openapi.auth-required":
				security := location.child("security")
				if security.n == nil {
					security = root.child("security")
				}
				authenticated := len(security.list()) > 0
				for _, alternative := range security.list() {
					if len(alternative.pairs()) == 0 {
						authenticated = false
					}
					for _, entry := range alternative.pairs() {
						scheme := g.resolve(root.child("components").child("securitySchemes").child(entry.key))
						if scheme.n == nil && c.id == "openapi.security-references" {
							emit(security, "undefined security scheme "+entry.key)
						}
						for _, scope := range entry.value.list() {
							found := false
							for _, flow := range scheme.child("flows").pairs() {
								found = found || flow.value.child("scopes").child(scope.value()).n != nil
							}
							if !found && c.id == "openapi.security-references" {
								emit(security, "undefined OAuth scope "+scope.value())
							}
						}
					}
				}
				if c.id == "openapi.auth-required" && !authenticated && !contains(c.options.Public, id) {
					emit(location, "authentication required; add an explicit public-operations exception if intentional")
				}
			case "openapi.tags-defined":
				for _, tag := range location.child("tags").list() {
					found := false
					for _, global := range root.child("tags").list() {
						found = found || global.child("name").value() == tag.value()
					}
					if !found {
						emit(tag, "undefined operation tag "+tag.value())
					}
				}
			case "openapi.error-response":
				found := false
				for _, response := range location.child("responses").pairs() {
					if strings.HasPrefix(response.key, "4") || strings.HasPrefix(response.key, "5") || response.key == "default" {
						found = true
						if c.options.ErrorSchema != "" {
							for _, mt := range g.resolve(response.value).child("content").pairs() {
								if !g.named(mt.value.child("schema"), c.options.ErrorSchema) {
									emit(mt.value, "error response must reference "+c.options.ErrorSchema)
								}
							}
						}
					}
				}
				if !found {
					emit(location, "document an error response")
				}
			case "portos.operation-vocabulary":
				for _, word := range operationWords(id) {
					if (strings.EqualFold(word, "Batch") && !contains(c.options.Modifiers, "Batch")) || (strings.EqualFold(word, "Async") && !contains(c.options.Modifiers, "Async")) {
						emit(location, "modifier is not enabled: "+word)
					}
				}
				if base == "" {
					emit(location, "operationId must contain one of "+strings.Join(c.options.Operations, ", ")+"; Batch and Async are modifiers")
				}
				for _, word := range operationWords(id) {
					if strings.EqualFold(word, "create") || strings.EqualFold(word, "update") || strings.EqualFold(word, "search") || strings.EqualFold(word, "enumerate") {
						emit(location, "unsupported operation word "+word)
					}
				}
			case "portos.query-request":
				if base == "Query" {
					if op.method != "post" {
						emit(location, "Query must use POST")
					}
					requireBody(func(s site) {
						if !g.object(s) {
							emit(s, "Query request must be an object")
						}
					})
				}
			case "portos.async-response":
				if async {
					requireResponse(func(s site) {
						resolved := g.resolve(s)
						if !g.named(s, c.options.AsyncSchema) || !g.object(resolved) {
							emit(s, "Async success must reference the "+c.options.AsyncSchema+" object")
						}
						props := g.properties(resolved)
						if len(props) != 1 || props[c.options.ItemID].n == nil || g.resolve(props[c.options.ItemID]).child("type").value() != "string" {
							emit(s, "Async success must contain only "+c.options.ItemID)
						}
					})
				}
			case "portos.collection-response":
				if collection && !async {
					requireResponse(func(s site) {
						props := g.properties(s)
						results := g.resolve(props[c.options.Results])
						if !g.object(s) || results.child("type").value() != "array" || !g.object(results.child("items")) {
							emit(s, "Query/List response must be an object containing "+c.options.Results+" array of objects")
						}
						if props[c.options.Pagination].n == nil {
							emit(s, "Query/List response requires "+c.options.Pagination)
						}
					})
				}
			case "portos.pagination-response":
				if collection && !async {
					requireResponse(func(s site) {
						pc := g.properties(s)[c.options.Pagination]
						props := g.properties(pc)
						if !g.object(pc) || g.resolve(props[c.options.NextToken]).child("type").value() != "string" || g.resolve(props[c.options.MaxResults]).child("type").value() != "integer" {
							emit(s, "paginationContext requires string "+c.options.NextToken+" and integer "+c.options.MaxResults)
						}
					})
				}
			case "portos.pagination-request", "openapi.pagination":
				if collection || contains(c.options.CollectionOperations, id) {
					if base == "Query" {
						requireBody(func(s site) {
							props := g.properties(s)
							pc := props[c.options.Pagination]
							if pc.n != nil {
								props = g.properties(pc)
							}
							if props[c.options.NextToken].n == nil || props[c.options.MaxResults].n == nil {
								emit(s, "Query body must contain nextToken and maxResults pagination fields")
							}
						})
					} else {
						params := map[string]site{}
						for _, list := range []site{op.item.child("parameters"), location.child("parameters")} {
							for _, p := range list.list() {
								p = g.resolve(p)
								if p.child("in").value() == "query" {
									params[p.child("name").value()] = p
								}
							}
						}
						if params[c.options.NextToken].n == nil || params[c.options.MaxResults].n == nil {
							emit(location, "List requires nextToken and maxResults query parameters")
						}
					}
				}
			case "portos.batch-contract":
				if batch {
					requireBody(func(s site) {
						items := g.resolve(g.properties(s)[c.options.Items])
						if items.child("type").value() != "array" || !g.object(items.child("items")) || g.properties(items.child("items"))[c.options.ItemID].n == nil {
							emit(s, "Batch request requires items array of objects with ids")
						}
					})
					if !async {
						requireResponse(func(s site) {
							results := g.resolve(g.properties(s)[c.options.Results])
							if results.child("type").value() != "array" || !g.object(results.child("items")) || g.properties(results.child("items"))[c.options.ItemID].n == nil {
								emit(s, "Batch response requires results array of objects with ids")
							}
						})
					}
				}
			case "portos.query-graph":
				if base == "Query" {
					requireBody(func(s site) {
						query := g.properties(s)["query"]
						if !g.named(query, c.options.QuerySchema) {
							emit(s, "query must reference canonical "+c.options.QuerySchema+" schema")
						}
						q := g.properties(query)
						for _, key := range []string{"match", "lessThan", "greaterThan", "and", "or", "not", "patternMatch", "freeformMatch"} {
							if q[key].n == nil {
								emit(s, "canonical query graph is missing "+key)
							}
							switch key {
							case "and", "or":
								v := g.resolve(q[key])
								if v.child("type").value() != "array" || !g.named(v.child("items"), c.options.QuerySchema) {
									emit(s, "query "+key+" must contain recursive "+c.options.QuerySchema+" items")
								}
							case "not":
								if !g.named(q[key], c.options.QuerySchema) {
									emit(s, "query not must reference "+c.options.QuerySchema)
								}
							case "patternMatch", "freeformMatch":
								if g.resolve(q[key]).child("type").value() != "string" {
									emit(s, "query "+key+" must be a string")
								}
							case "match", "lessThan", "greaterThan":
								comp := g.properties(q[key])
								if !g.object(q[key]) || g.resolve(comp["key"]).child("type").value() != "string" || g.resolve(comp["value"]).child("type").value() != "string" {
									emit(s, "query comparator "+key+" must contain string key and value")
								}
							}

						}
					})
				}
			}
		}
		switch c.id {
		case "portos.path-description":
			for _, path := range root.child("paths").pairs() {
				item := g.resolve(path.value)
				if !item.child("description").nonblank() {
					emit(path.value, "every Path Item requires a description")
				}
			}
		case "openapi.tags-defined":
			names := map[string]bool{}
			for _, tag := range root.child("tags").list() {
				name := tag.child("name").value()
				if names[name] {
					emit(tag, "duplicate tag name "+name)
				}
				names[name] = true
			}
		case "openapi.info-description", "openapi.info-contact", "openapi.info-license":
			field := strings.TrimPrefix(c.id, "openapi.info-")
			if root.child("info").child(field).n == nil || (field == "description" && !root.child("info").child(field).nonblank()) {
				emit(root.child("info"), "provide API "+field)
			}
		case "openapi.server-variables", "openapi.server-policy":
			var walk func(site)
			walk = func(s site) {
				for _, pair := range s.pairs() {
					if pair.key == "servers" {
						for _, server := range pair.value.list() {
							raw := server.child("url").value()
							vars := server.child("variables")
							for _, m := range regexp.MustCompile(`\{([^{}]+)\}`).FindAllStringSubmatch(raw, -1) {
								v := vars.child(m[1])
								if c.id == "openapi.server-variables" && (v.n == nil || v.child("default").n == nil) {
									emit(server, "undefined server variable/default "+m[1])
								}
								if v.n != nil {
									if c.id == "openapi.server-variables" && v.child("enum").n != nil {
										found := false
										for _, value := range v.child("enum").list() {
											found = found || value.value() == v.child("default").value()
										}
										if !found {
											emit(v, "server default must be in enum")
										}
									}
									raw = strings.ReplaceAll(raw, m[0], v.child("default").value())
								}
							}
							if c.id == "openapi.server-policy" {
								u, e := url.Parse(raw)
								if e != nil || !contains(c.options.Protocols, u.Scheme) || (len(c.options.Hosts) > 0 && !contains(c.options.Hosts, u.Hostname())) {
									emit(server, "server does not match allowed protocols/hosts")
								}
							}
						}
					}
					walk(pair.value)
				}
				for _, item := range s.list() {
					walk(item)
				}
			}
			walk(root)
		}
		if c.id == "openapi.enum-values" || c.id == "portos.open-enums" || c.id == "portos.date-fields" || c.id == "portos.name-schema" || c.id == "portos.description-schema" || c.id == "openapi.ref-siblings" {
			visited := map[*yaml.Node]bool{}
			var walk func(site)
			walk = func(s site) {
				if s.n == nil || visited[s.n] {
					return
				}
				visited[s.n] = true
				if c.id == "openapi.ref-siblings" && s.child("$ref").nonblank() {
					isSchema := strings.Contains(s.ptr, "/schemas/") || strings.HasSuffix(s.ptr, "/schema")
					is31 := strings.HasPrefix(root.child("openapi").value(), "3.1")
					for _, p := range s.pairs() {
						if p.key != "$ref" && (!is31 || (!isSchema && p.key != "summary" && p.key != "description")) {
							emit(p.value, "reference sibling is not supported in this version/object")
						}
					}
				}
				enum := s.child("enum")
				if c.id == "portos.open-enums" && s.child("x-extensible-enum").n != nil {
					enum = s.child("x-extensible-enum")
				}
				if enum.n != nil {
					if c.id == "openapi.enum-values" {
						seen := map[string]bool{}
						for _, v := range enum.list() {
							var value any
							_ = v.n.Decode(&value)
							key := fmt.Sprintf("%#v", value)
							if seen[key] {
								emit(v, "duplicate enum value")
							}
							seen[key] = true
							if !enumTypeMatches(s, v) {
								emit(v, "enum value does not match declared type")
							}
						}
					}
					if c.id == "portos.open-enums" {
						if s.child("enum").n != nil {
							emit(s, "open enums must use x-extensible-enum instead of closed enum")
						}
						if len(enum.list()) == 0 {
							emit(s, "open enum values must not be empty")
						}
						names := s.child("x-enum-varnames").list()
						if len(names) != len(enum.list()) {
							emit(s, "x-enum-varnames must correspond to every enum value")
						}
						seen := map[string]bool{}
						for _, name := range names {
							if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name.value()) || seen[name.value()] {
								emit(name, "enum varnames must be unique identifiers")
							}
							seen[name.value()] = true
						}
					}
				}
				if c.id == "portos.date-fields" {
					for _, p := range s.child("properties").pairs() {
						key := strings.ToLower(p.key)
						isDate := contains(c.options.DateFields, p.key) || key == "date" || key == "timestamp" || strings.HasSuffix(p.key, "Date") || strings.HasSuffix(p.key, "Time") || strings.HasSuffix(p.key, "Timestamp") || strings.HasSuffix(key, "_date") || strings.HasSuffix(key, "_time") || strings.HasSuffix(key, "_timestamp") || strings.HasSuffix(p.key, "At")
						if isDate {
							resolved := g.resolve(p.value)
							if resolved.child("type").value() != "string" || resolved.child("format").value() != "date-time" {
								emit(p.value, "date fields must be RFC 3339 strings (format: date-time)")
							}
						}
					}
				}
				if c.id == "portos.name-schema" || c.id == "portos.description-schema" {
					field, name := "name", c.options.NameSchema
					if c.id == "portos.description-schema" {
						field, name = "description", c.options.DescriptionSchema
					}
					for _, p := range s.child("properties").pairs() {
						if p.key == field && (!g.named(p.value, name) || !g.object(p.value)) {
							emit(p.value, field+" must reference the "+name+" object schema")
						}
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
				for _, v := range s.list() {
					walk(v)
				}
			}
			walk(root)
		}
		if c.id == "openapi.unused-components" && root.child("paths").n != nil {
			reachable := map[string]bool{}
			visited := map[*yaml.Node]bool{}
			var visit func(site)
			visit = func(s site) {
				if s.n == nil || visited[s.n] {
					return
				}
				visited[s.n] = true
				resolved := g.resolve(s)
				if resolved.n != s.n {
					reachable[resolved.file+resolved.ptr] = true
					visit(resolved)
				}
				for _, p := range s.pairs() {
					if p.key != "example" && p.key != "examples" {
						visit(p.value)
					}
				}
				for _, v := range s.list() {
					visit(v)
				}
			}
			visit(root.child("paths"))
			visit(root.child("webhooks"))
			visit(root.child("security"))
			for _, section := range root.child("components").pairs() {
				for _, component := range section.value.pairs() {
					resolved := g.resolve(component.value)
					if !reachable[resolved.file+resolved.ptr] {
						if section.key == "securitySchemes" {
							used := false
							for _, op := range operations {
								sec := op.op.child("security")
								if sec.n == nil {
									sec = root.child("security")
								}
								for _, alternative := range sec.list() {
									used = used || alternative.child(component.key).n != nil
								}
							}
							if used {
								continue
							}
						}
						emit(component.value, "unused component "+component.key)
					}
				}
			}
		}
		if c.id == "openapi.spec-structure" {
			if e := c.validateStructure(root, emit); e != nil {
				pass.ReportError(e)
			}
		}
		if c.id == "openapi.examples-valid" {
			c.validateExamples(ctx, &g, root, emit, pass)
		}
		if g.err != nil {
			pass.ReportError(g.err)
		}
	}
}

func defaultOptions() apiOptions {
	return apiOptions{Operations: []string{"List", "Send", "Delete", "Query", "Get", "Modify"}, Modifiers: []string{"Batch", "Async"}, QuerySchema: "Query", NameSchema: "NameValue", DescriptionSchema: "DescriptionValue", AsyncSchema: "AsyncIdentifier", MaxResults: "maxResults", NextToken: "nextToken", Results: "results", Pagination: "paginationContext", Items: "items", ItemID: "id", Protocols: []string{"https"}, Classes: []string{"2", "3"}}
}

func enumTypeMatches(schema, value site) bool {
	types := schema.child("type").list()
	if len(types) == 0 && schema.child("type").nonblank() {
		types = []site{schema.child("type")}
	}
	if len(types) == 0 {
		return true
	}
	if value.n.Tag == "!!null" && schema.child("nullable").value() == "true" {
		return true
	}
	for _, typ := range types {
		switch typ.value() {
		case "string":
			if value.n.Tag == "!!str" {
				return true
			}
		case "boolean":
			if value.n.Tag == "!!bool" {
				return true
			}
		case "null":
			if value.n.Tag == "!!null" {
				return true
			}
		case "number":
			if value.n.Tag == "!!int" || value.n.Tag == "!!float" {
				return true
			}
		case "integer":
			if value.n.Tag == "!!int" {
				return true
			}
			if value.n.Tag == "!!float" {
				number, e := strconv.ParseFloat(value.value(), 64)
				if e == nil && math.Trunc(number) == number {
					return true
				}
			}
		case "object":
			if value.n.Kind == yaml.MappingNode {
				return true
			}
		case "array":
			if value.n.Kind == yaml.SequenceNode {
				return true
			}
		}
	}
	return false
}
