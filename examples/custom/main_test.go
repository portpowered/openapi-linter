package main

import (
	"context"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	public "github.com/portpowered/openapi-linter"
	"testing"
)

func TestCustomerTitleExample(t *testing.T) {
	check := titleCheck{Title: "Expected"}
	if check.ID() != "customer.title" {
		t.Fatal(check.ID())
	}
	pass := public.NewPass([]*public.Document{{}, {Path: "matching.yaml", Model: &v3.Document{Info: &base.Info{Title: "Expected"}}}, {Path: "missing.yaml", Model: &v3.Document{}}, {Path: "wrong.yaml", Model: &v3.Document{Info: &base.Info{Title: "Other"}}}})
	check.Analyze(context.Background(), pass)
	got := pass.Diagnostics()
	if len(got) != 2 || got[0].Path != "missing.yaml" || got[1].Pointer != "#/info/title" {
		t.Fatalf("%+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pass = public.NewPass([]*public.Document{{Model: &v3.Document{}}})
	check.Analyze(ctx, pass)
	if len(pass.Diagnostics()) != 0 {
		t.Fatal(pass.Diagnostics())
	}
}
