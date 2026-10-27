package rulepack

import "gopkg.in/yaml.v3"

// properties retains guarantees common to every alternative. A shared property's
// full alternatives travel with it, so later checks cannot silently choose one.
func (g *graph) properties(schema site) map[string]site {
	active := map[*yaml.Node]bool{}
	var walk func(site) map[string]site
	walk = func(s site) map[string]site {
		out := map[string]site{}
		s = g.resolve(s)
		if s.n == nil || active[s.n] {
			return out
		}
		active[s.n] = true
		defer delete(active, s.n)
		for _, p := range s.child("properties").pairs() {
			out[p.key] = p.value
		}
		for _, parent := range s.child("allOf").list() {
			for key, value := range walk(parent) {
				if previous, ok := out[key]; ok {
					value = g.composeProperty("allOf", []site{previous, value})
				}
				out[key] = value
			}
		}
		for _, operator := range []string{"oneOf", "anyOf"} {
			branches := s.child(operator).list()
			if len(branches) == 0 {
				continue
			}
			common := walk(branches[0])
			for _, branch := range branches[1:] {
				properties := walk(branch)
				for key, previous := range common {
					if value, ok := properties[key]; ok {
						common[key] = g.composeProperty("anyOf", []site{previous, value})
					} else {
						delete(common, key)
					}
				}
			}
			for key, value := range common {
				if previous, ok := out[key]; ok {
					value = g.composeProperty("allOf", []site{previous, value})
				}
				out[key] = value
			}
		}
		return out
	}
	return walk(schema)
}

func (g *graph) composeProperty(operator string, schemas []site) site {
	first := g.resolve(schemas[0])
	other := g.resolve(schemas[1])
	if first.n == other.n {
		return schemas[0]
	}
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, schema := range schemas {
		sequence.Content = append(sequence.Content, g.resolve(schema).n)
	}
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: operator}, sequence}}
	// Expose a type only when every alternative agrees. Preserve the alternatives
	// for constraints (enums, required fields, nested objects), rather than copying
	// the first alternative's constraints into an alleged common schema.
	if typ := g.schemaType(first); typ != "" && typ == g.schemaType(other) {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "type"}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: typ})
		if typ == "array" && first.child("items").n != nil && other.child("items").n != nil {
			items := g.composeProperty(operator, []site{first.child("items"), other.child("items")})
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "items"}, items.n)
		}
	}
	return site{n: node, file: first.file, ptr: first.ptr}
}

func (g *graph) object(schema site) bool {
	active := map[*yaml.Node]bool{}
	var walk func(site) bool
	walk = func(s site) bool {
		s = g.resolve(s)
		if s.n == nil || active[s.n] {
			return false
		}
		active[s.n] = true
		defer delete(active, s.n)
		object := s.child("type").value() == "object" || s.child("type").n == nil && len(s.child("properties").pairs()) > 0
		for _, parent := range s.child("allOf").list() {
			object = walk(parent) || object
		}
		for _, operator := range []string{"oneOf", "anyOf"} {
			if s.child(operator).n == nil {
				continue
			}
			branches := s.child(operator).list()
			if len(branches) == 0 {
				return false
			}
			for _, branch := range branches {
				if !walk(branch) {
					return false
				}
			}
			object = true
		}
		return object
	}
	return walk(schema)
}

func requiredProperties(g *graph, schema site) map[string]bool {
	active := map[*yaml.Node]bool{}
	var walk func(site) map[string]bool
	walk = func(s site) map[string]bool {
		required := map[string]bool{}
		s = g.resolve(s)
		if s.n == nil || active[s.n] {
			return required
		}
		active[s.n] = true
		defer delete(active, s.n)
		for _, property := range s.child("required").list() {
			required[property.value()] = true
		}
		for _, parent := range s.child("allOf").list() {
			for key := range walk(parent) {
				required[key] = true
			}
		}
		for _, operator := range []string{"oneOf", "anyOf"} {
			branches := s.child(operator).list()
			if len(branches) == 0 {
				continue
			}
			common := walk(branches[0])
			for _, branch := range branches[1:] {
				other := walk(branch)
				for key := range common {
					if !other[key] {
						delete(common, key)
					}
				}
			}
			for key := range common {
				required[key] = true
			}
		}
		return required
	}
	return walk(schema)
}

// schemaType returns a type only when all alternatives establish the same type.
func (g *graph) schemaType(schema site) string {
	active := map[*yaml.Node]bool{}
	var walk func(site) string
	walk = func(s site) string {
		s = g.resolve(s)
		if s.n == nil || active[s.n] {
			return ""
		}
		active[s.n] = true
		defer delete(active, s.n)
		if s.child("nullable").value() == "true" {
			return ""
		}
		typ := s.child("type").value()
		for _, parent := range s.child("allOf").list() {
			inherited := walk(parent)
			if typ != "" && inherited != "" && typ != inherited {
				return ""
			}
			if inherited != "" {
				typ = inherited
			}
		}
		for _, operator := range []string{"oneOf", "anyOf"} {
			if s.child(operator).n == nil {
				continue
			}
			branches := s.child(operator).list()
			if len(branches) == 0 {
				return ""
			}
			common := walk(branches[0])
			if common == "" {
				return ""
			}
			for _, branch := range branches[1:] {
				if walk(branch) != common {
					return ""
				}
			}
			if typ != "" && typ != common {
				return ""
			}
			typ = common
		}
		return typ
	}
	return walk(schema)
}

func batchItemID(g *graph, item site, field string) bool {
	typ := g.schemaType(g.properties(item)[field])
	return g.object(item) && (typ == "string" || typ == "integer")
}

func flatBatchErrorShape(g *graph, schema site, idField string) bool {
	if !g.object(schema) {
		return false
	}
	properties, required := g.properties(schema), requiredProperties(g, schema)
	for _, field := range []string{idField, "code", "family", "message"} {
		if !required[field] {
			return false
		}
	}
	return stringSchema(g, properties["code"], false, false) && stringSchema(g, properties["family"], true, false) && stringSchema(g, properties["message"], false, true)
}

// stringSchema checks every composed property constraint; annotations copied from
// one branch cannot hide a numeric enum or a fixed message in another branch.
func stringSchema(g *graph, schema site, requireEnum, freeText bool) bool {
	if g.schemaType(schema) != "string" {
		return false
	}
	active := map[*yaml.Node]bool{}
	var walk func(site) (bool, bool)
	walk = func(s site) (valid, enumeration bool) {
		s = g.resolve(s)
		if s.n == nil || active[s.n] {
			return false, false
		}
		active[s.n] = true
		defer delete(active, s.n)
		if freeText && (s.child("enum").n != nil || s.child("const").n != nil) {
			return false, false
		}
		if s.child("enum").n != nil {
			values := s.child("enum").list()
			if len(values) == 0 {
				return false, false
			}
			for _, value := range values {
				if value.n.Tag != "!!str" {
					return false, false
				}
			}
			enumeration = true
		}
		for _, parent := range s.child("allOf").list() {
			valid, inherited := walk(parent)
			if !valid {
				return false, false
			}
			enumeration = enumeration || inherited
		}
		for _, operator := range []string{"oneOf", "anyOf"} {
			if s.child(operator).n == nil {
				continue
			}
			branches := s.child(operator).list()
			if len(branches) == 0 {
				return false, false
			}
			allEnumerated := true
			for _, branch := range branches {
				valid, inherited := walk(branch)
				if !valid {
					return false, false
				}
				allEnumerated = allEnumerated && inherited
			}
			enumeration = enumeration || allEnumerated
		}
		return true, enumeration
	}
	valid, enumerated := walk(schema)
	return valid && (!requireEnum || enumerated)
}

// emptyBatchArray proves every alternative permits an empty outcome list.
func emptyBatchArray(g *graph, schema site) bool {
	if g.schemaType(schema) != "array" {
		return false
	}
	active := map[*yaml.Node]bool{}
	var walk func(site) bool
	walk = func(s site) bool {
		s = g.resolve(s)
		if s.n == nil || active[s.n] {
			return false
		}
		active[s.n] = true
		defer delete(active, s.n)
		if s.child("minItems").n != nil && s.child("minItems").value() != "0" {
			return false
		}
		for _, operator := range []string{"allOf", "oneOf", "anyOf"} {
			if s.child(operator).n == nil {
				continue
			}
			branches := s.child(operator).list()
			if len(branches) == 0 {
				return false
			}
			for _, branch := range branches {
				if !walk(branch) {
					return false
				}
			}
		}
		return true
	}
	return walk(schema)
}
