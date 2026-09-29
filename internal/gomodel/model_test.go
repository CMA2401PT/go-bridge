package gomodel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func analyzeSource(t *testing.T, source string) (*Package, error) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "api.go", "package api\n"+source, 0)
	if err != nil {
		t.Fatal(err)
	}
	sizes := types.SizesFor("gc", "amd64")
	pkg, err := (&types.Config{Sizes: sizes}).Check("example.test/api", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(pkg, sizes)
}

func named(p *Package, name string) *Type {
	for _, n := range p.Types {
		if n.Kind == Named && n.Name == name {
			return n
		}
	}
	return nil
}

func TestClosureAndIdentity(t *testing.T) {
	p, err := analyzeSource(t, `
type Count int
type Alias = Count
type Count2 int
type Counts []Count
type Link struct { Value Count; Next *Link; hidden map[string]int }
type IntPtr *int
func Deep(p ***int, s Counts, n IntPtr) **Link { return nil }
`)
	if err != nil {
		t.Fatal(err)
	}
	count, link := named(p, "Count"), named(p, "Link")
	if count == named(p, "Count2") {
		t.Fatal("distinct named types merged")
	}
	if len(link.Fields) != 2 || link.Fields[1].Type.Elem != link {
		t.Fatal("recursive edge lost")
	}
	if named(p, "Counts").Underlying.Elem != count {
		t.Fatal("slice element lost its name")
	}
	if named(p, "IntPtr").Underlying.Kind != Pointer {
		t.Fatal("named pointer lost its underlying type")
	}
	var alias *Type
	for _, d := range p.Declarations {
		if d.Name == "Alias" && d.Alias {
			alias = d.Type
		}
	}
	if alias != count {
		t.Fatal("alias creates a new type identity")
	}
	for _, k := range []types.BasicKind{types.Bool, types.Int, types.Uint8, types.String, types.Float64} {
		if p.Find(types.Typ[k]) == nil {
			t.Fatalf("missing baseline type %s", types.Typ[k])
		}
	}
	if p.Find(types.NewPointer(types.NewPointer(types.NewPointer(types.Typ[types.Int])))) == nil {
		t.Fatal("multi-pointer dependency not collected")
	}
}

func TestMethodDependenciesAndSets(t *testing.T) {
	p, err := analyzeSource(t, `
type Count int
type Child struct { V int }
type Parent struct { Child }
func (c Count) Read(s []string) *Child { return nil }
func (c *Count) Add(n int) {}
func (c Child) Value() Count { return 0 }
func (c *Child) Set(v Count) {}
`)
	if err != nil {
		t.Fatal(err)
	}
	c := named(p, "Count")
	if len(c.ValueMethods) != 1 || len(c.PointerMethods) != 2 {
		t.Fatal("incorrect method sets")
	}
	if c.ValueMethods[0].Params[0].Type.Kind != Slice {
		t.Fatal("method signature dependency missing")
	}
	if _, ok := c.PointerMethods[0].Receiver.(*types.Pointer); !ok || !c.PointerMethods[0].PointerReceiver {
		t.Fatal("pointer receiver lost")
	}
	parent := named(p, "Parent")
	if len(parent.ValueMethods) != 1 || len(parent.PointerMethods) != 2 {
		t.Fatal("promoted methods missing")
	}
	if !parent.Fields[0].Embedded {
		t.Fatal("embedded field lost")
	}
	// **Count is not created by recursively adding a pointer to each type.
	if p.Find(types.NewPointer(types.NewPointer(c.Go))) != nil {
		t.Fatal("unbounded pointer expansion")
	}
}

func TestMethodSetQueryDoesNotInventTypes(t *testing.T) {
	p, err := analyzeSource(t, `
type Counter int
func (c Counter) Read() int { return int(c) }
type hidden struct{}
func (hidden) Public() int { return 1 }
type Owner struct{ hidden }
`)
	if err != nil {
		t.Fatal(err)
	}
	c := named(p, "Counter")
	if p.Find(types.NewPointer(c.Go)) != nil {
		t.Fatal("querying a method set invented a pointer dependency")
	}
	if len(c.PointerMethods) != 1 || !types.Identical(c.PointerMethods[0].Receiver, c.Go) {
		t.Fatal("declared receiver lost")
	}
	if !types.Identical(c.PointerMethods[0].SelectedReceiver, types.NewPointer(c.Go)) {
		t.Fatal("selected receiver lost")
	}
	owner := named(p, "Owner")
	if len(owner.ValueMethods) != 1 || named(p, "hidden") != nil {
		t.Fatal("promoted method requires a wrapper for a hidden receiver")
	}
}

func TestSameNameInDifferentPackages(t *testing.T) {
	api := types.NewPackage("example.test/api", "api")
	var params []*types.Var
	for _, path := range []string{"example.test/a", "example.test/b"} {
		pkg := types.NewPackage(path, "same")
		obj := types.NewTypeName(token.NoPos, pkg, "Value", nil)
		typ := types.NewNamed(obj, types.Typ[types.Int], nil)
		params = append(params, types.NewVar(token.NoPos, api, "", typ))
	}
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(params...), nil, false)
	api.Scope().Insert(types.NewFunc(token.NoPos, api, "Use", sig))
	p, err := Analyze(api, types.SizesFor("gc", "amd64"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := p.Functions[0].Params[0].Type, p.Functions[0].Params[1].Type
	if a == b || a.Name != b.Name || a.PackagePath == b.PackagePath {
		t.Fatal("type identity does not preserve the declaring package")
	}
}

func TestUnsupportedHasContext(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`func Bad(v any) {}`, "parameter 1 (v): unsupported type interface{}"},
		{`type Box struct { Data map[string]int }`, "field Data: unsupported type map[string]int"},
		{`func Bad() error { return nil }`, "result 1"},
		{`func Bad(v chan int) {}`, "unsupported type chan int"},
		{`func Bad(v [2]int) {}`, "unsupported type [2]int"},
		{`func Bad(v func()) {}`, "unsupported type func()"},
		{`func Bad(v struct{X int}) {}`, "unsupported type struct"},
		{`type private int; func Bad() private { return 0 }`, "non-exported named type"},
		{`type Flag bool`, "named bool"},
		{`type Box[T any] struct{ V T }`, "generic type"},
		{`func Bad[T any](v T) {}`, "generic function"},
		{`func Bad(v ...int) {}`, "variadic function"},
		{`var Global int`, "package variables/constants"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			p, err := analyzeSource(t, tc.source)
			if err == nil || p != nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("model=%v err=%v; want %q", p, err, tc.want)
			}
		})
	}
}

func TestLoadExamples(t *testing.T) {
	paths, err := filepath.Glob("../../examples/0*")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 9 {
		t.Fatalf("expected 9 examples, got %d", len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			p, err := Load("", path)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Functions) == 0 {
				t.Fatal("no exported functions")
			}
		})
	}
}

func TestLoadModuleAndBuildSelection(t *testing.T) {
	dir := t.TempDir()
	for file, source := range map[string]string{
		"go.mod":     "module example.test/standalone\n\ngo 1.25.0\n",
		"a.go":       "package standalone\nfunc Echo(v Value) Value { return v }\n",
		"b.go":       "package standalone\ntype Value int\n",
		"ignored.go": "//go:build never_bridge_test\n\npackage standalone\nfunc Unsupported(v chan int) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Path != "example.test/standalone" || len(p.Functions) != 1 || p.Functions[0].Params[0].Type != named(p, "Value") {
		t.Fatalf("incorrect package: %+v", p)
	}
	if _, err := Load(dir, "does.not.exist/module"); err == nil {
		t.Fatal("missing package accepted")
	}
}
