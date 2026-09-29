package pyapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"go-bridge/internal/gomodel"
)

func TestCompletionOnlyAddsStructAddresses(t *testing.T) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "api.go", `package api
    type Value int
    type Inner struct { N int }; type Box struct { Value Value; Inner Inner }
    func F(s []int, p ***int) {}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	sizes := types.SizesFor("gc", "amd64")
	pkg, err := (&types.Config{Sizes: sizes}).Check("test/api", fs, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gomodel.Analyze(pkg, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if err := complete(g); err != nil {
		t.Fatal(err)
	}
	length := len(g.Types)
	if err := complete(g); err != nil || len(g.Types) != length {
		t.Fatal("completion is not idempotent", err)
	}
	for _, tc := range []struct {
		typ  types.Type
		want bool
	}{
		{types.NewPointer(pkg.Scope().Lookup("Box").Type()), true},
		{types.NewPointer(pkg.Scope().Lookup("Inner").Type()), true},
		{types.NewPointer(pkg.Scope().Lookup("Value").Type()), false},
		{types.NewPointer(types.NewSlice(types.Typ[types.Int])), false},
		{types.NewPointer(types.NewPointer(types.NewPointer(types.NewPointer(types.Typ[types.Int])))), false},
	} {
		if (g.Find(tc.typ) != nil) != tc.want {
			t.Errorf("unexpected completion of %s", tc.typ)
		}
	}
}
