package rulepack

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"sync"
)

// Official OpenAPI schemas are vendored with their dated $id. Neither
// compilation nor linting fetches schemas or references over the network.
//
//go:embed schemas/*.json
var structureSchemas embed.FS
var structureOnce sync.Once
var structures map[string]*jsonschema.Schema
var structureError error

func structuralSchema(version string) (*jsonschema.Schema, error) {
	structureOnce.Do(func() {
		structures = map[string]*jsonschema.Schema{}
		for _, version := range []string{"3.0", "3.1"} {
			data, e := structureSchemas.ReadFile("schemas/" + version + ".json")
			if e != nil {
				structureError = e
				return
			}
			var resource any
			if e = json.Unmarshal(data, &resource); e != nil {
				structureError = e
				return
			}
			compiler := jsonschema.NewCompiler()
			compiler.UseLoader(offlineSchemas{})
			uri := "urn:lint:openapi:" + version
			if e = compiler.AddResource(uri, resource); e != nil {
				structureError = e
				return
			}
			schema, e := compiler.Compile(uri)
			if e != nil {
				structureError = e
				return
			}
			structures[version] = schema
		}
	})
	if structureError != nil {
		return nil, structureError
	}
	schema, ok := structures[version]
	if !ok {
		return nil, fmt.Errorf("unsupported OpenAPI version %s", version)
	}
	return schema, nil
}
func (c apiCheck) validateStructure(root site, emit func(site, string)) error {
	version := "3.0"
	if strings.HasPrefix(root.child("openapi").value(), "3.1") {
		version = "3.1"
	}
	schema, e := structuralSchema(version)
	if e != nil {
		return e
	}
	value, e := jsonValue(root.n)
	if e != nil {
		return e
	}
	e = schema.Validate(value)
	if e == nil {
		return nil
	}
	var invalid *jsonschema.ValidationError
	if !errors.As(e, &invalid) {
		return e
	}
	var report func(*jsonschema.ValidationError)
	report = func(v *jsonschema.ValidationError) {
		if len(v.Causes) > 0 {
			for _, cause := range v.Causes {
				report(cause)
			}
			return
		}
		s := root
		for _, segment := range v.InstanceLocation {
			s = s.child(segment)
		}
		emit(s, "OpenAPI structure: "+v.Error())
	}
	report(invalid)
	return nil
}
