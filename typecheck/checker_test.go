// Copyright (c) 2025 Visvasity LLC

package typecheck

import (
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestChecker(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.LoadTypes | packages.NeedTypesInfo | packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, "github.com/visvasity/slabgen/input")
	if err != nil {
		t.Fatal(err)
	}
	scope := pkgs[0].Types.Scope()
	object := scope.Lookup("TestGenericBlock")
	if object == nil {
		t.Fatalf("TestGenericBlock not found")
	}
	named, ok := object.Type().(*types.Named)
	if !ok {
		t.Fatalf("TestGenericBlock is not a named type")
	}
	// t.Logf("type=%s", types.ObjectString(object, nil))

	checker := New()
	if err := checker.Check(named.Obj()); err != nil {
		t.Fatal(err)
	}
	// js, _ := json.MarshalIndent(checker.StructDataMap(), "", "  ")
	// t.Logf("%s", js)
}
