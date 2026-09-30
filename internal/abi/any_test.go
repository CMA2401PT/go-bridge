package abi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"go-bridge/internal/gomodel"
	"go-bridge/internal/pyapi"
)

func TestAnyWireAndConversionPlan(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "api.go", "package api\ntype Value = any\ntype Count int\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	sizes := types.SizesFor("gc", "amd64")
	pkg, err := (&types.Config{Sizes: sizes}).Check("example.test/api", fileSet, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	goModel, err := gomodel.Analyze(pkg, sizes)
	if err != nil {
		t.Fatal(err)
	}
	python, err := pyapi.Plan(goModel, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(python)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Any.Kind != Handle || plan.Any.C != "uintptr_t" || len(plan.Operations) != 0 || len(plan.AnyConversions) != len(python.AnyConversions) {
		t.Fatal("any conversions changed the business operation model")
	}
	if plan.Imports[0].Alias != "api" {
		t.Fatal("standalone named scalar assertions do not import their Go type")
	}
	symbols := map[string]bool{}
	for _, conversion := range plan.AnyConversions {
		if conversion.Type == plan.Any || conversion.Python.Type != conversion.Type.Python {
			t.Fatal("incorrect conversion type identity")
		}
		for _, symbol := range []string{conversion.Box, conversion.Assert, conversion.Result} {
			if symbol == "" || symbols[symbol] {
				t.Fatal("conversion symbols are empty or duplicated")
			}
			symbols[symbol] = true
		}
	}
}
