package linter

import (
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

type locatedContractRule struct{}

func (locatedContractRule) Name() string                                 { return "locations" }
func (locatedContractRule) VisitSchema(string, *base.Schema) []Violation { return nil }
func (locatedContractRule) VisitPath(path string, _ *v3.PathItem) []Violation {
	return []Violation{{RuleName: "locations", Path: path, Message: "path"}}
}
func (locatedContractRule) VisitOperation(path, method string, _ *v3.Operation) []Violation {
	return []Violation{{RuleName: "locations", Path: path, Message: method}, {RuleName: "locations", Pointer: "#/custom/location", Message: "preserved"}}
}

func TestEngineCallbackWebhookAndReusedPathLocations(t *testing.T) {
	callbackItem := &v3.PathItem{Post: &v3.Operation{}}
	callbacks := orderedmap.New[string, *v3.Callback]()
	expressions := orderedmap.New[string, *v3.PathItem]()
	expressions.Set("{$request.body#/callback}", callbackItem)
	callbacks.Set("on/event", &v3.Callback{Expression: expressions})
	callbacks.Set("empty", &v3.Callback{})
	shared := &v3.PathItem{Get: &v3.Operation{Callbacks: callbacks}}
	paths := orderedmap.New[string, *v3.PathItem]()
	paths.Set("/first", shared)
	paths.Set("/second", shared)
	paths.Set("/empty", nil)
	hooks := orderedmap.New[string, *v3.PathItem]()
	hooks.Set("changed", &v3.PathItem{Put: &v3.Operation{}})
	// A callback graph can reuse the enclosing item; active recursion stops that cycle.
	expressions.Set("cycle", shared)
	e := &Engine{}
	e.RegisterRule(locatedContractRule{})
	findings := e.Run(&v3.Document{Paths: &v3.Paths{PathItems: paths}, Webhooks: hooks})
	if len(findings) != 15 {
		t.Fatalf("expected five visited path items with two operation findings: %#v", findings)
	}
	pointers := map[string]bool{}
	for _, f := range findings {
		pointers[f.Pointer] = true
	}
	for _, pointer := range []string{"#/paths/~1first/get", "#/paths/~1second/get", "#/paths/~1first/get/callbacks/on~1event/{$request.body#~1callback}/post", "#/webhooks/changed/put", "#/custom/location"} {
		if !pointers[pointer] {
			t.Fatalf("missing location %s: %#v", pointer, findings)
		}
	}
	for _, f := range findings {
		if strings.Contains(f.Pointer, "cycle") {
			t.Fatal("recursive callback revisited")
		}
	}
}
