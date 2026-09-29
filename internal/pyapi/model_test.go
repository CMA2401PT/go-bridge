package pyapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"go-bridge/internal/gomodel"
)

func planSource(t *testing.T, source string) (*Package, error) {
	t.Helper()
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "api.go", "package api\n"+source, 0)
	if err != nil {
		t.Fatal(err)
	}
	sizes := types.SizesFor("gc", "amd64")
	g, err := (&types.Config{Sizes: sizes}).Check("example.test/api", fs, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := gomodel.Analyze(g, sizes)
	if err != nil {
		t.Fatal(err)
	}
	return Plan(m, "")
}

func TestSnake(t *testing.T) {
	for input, want := range map[string]string{
		"HTTPServer": "http_server", "URL": "url", "MyURLValue": "my_url_value",
		"Uint16": "uint16", "Value2": "value2", "BasicMemberValue2": "basic_member_value2",
		"From": "from_", "already_snake": "already_snake", "IntInAndOut": "int_in_and_out",
	} {
		if got := Snake(input); got != want {
			t.Errorf("Snake(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSignaturesAndAliases(t *testing.T) {
	p, err := planSource(t, `type Text = string
func HTTPValue(from string, _ int) (bool, Text) { return true, from }`)
	if err != nil {
		t.Fatal(err)
	}
	f := p.Functions[0]
	if p.Name != "go_api" || f.Name != "http_value" || f.Params[0].Name != "from_" || f.Params[1].Name != "arg2" {
		t.Fatalf("wrong names: %+v %+v", p, f)
	}
	if f.Results[0].Name != "bool" || f.Results[1].Name != "str" || p.Aliases[0].Type != f.Results[1] {
		t.Fatal("alias and return types disagree")
	}
}

func TestConflictsAndBackendBoundary(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`func URLValue() {}; func UrlValue() {}`, "collides"},
		{`func F(from int, from_ int) {}`, "parameter"},
		{`type SliceInt []int`, "collides"},
	} {
		p, err := planSource(t, tc.source)
		if err == nil || p != nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v, want %s", tc.source, err, tc.want)
		}
	}
	p, err := planSource(t, "func F() {}")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape", "a.b", "a-b", "class", "9name"} {
		if _, err := Plan(p.Go, name); err == nil {
			t.Errorf("accepted package name %q", name)
		}
	}
}

func TestGoPanicIsNotReserved(t *testing.T) {
	p, err := planSource(t, `type GoPanic = string`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Aliases) != 1 || p.Aliases[0].Name != "GoPanic" {
		t.Fatalf("alias not preserved: %+v", p.Aliases)
	}
}
