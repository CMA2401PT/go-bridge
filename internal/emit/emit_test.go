package emit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"go-bridge/internal/abi"
	"go-bridge/internal/gomodel"
	"go-bridge/internal/pyapi"
)

func files(t *testing.T, source string) map[string]string {
	t.Helper()
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "api.go", "package api\n"+source, 0)
	if err != nil {
		t.Fatal(err)
	}
	sizes := types.SizesFor("gc", "arm64")
	pkg, err := (&types.Config{Sizes: sizes}).Check("example.test/api", fs, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gomodel.Analyze(pkg, sizes)
	if err != nil {
		t.Fatal(err)
	}
	p, err := pyapi.Plan(g, "")
	if err != nil {
		t.Fatal(err)
	}
	a, err := abi.Plan(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Files(a)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string)
	for path, data := range out {
		result[path] = string(data)
	}
	return result
}

func contains(t *testing.T, file, text string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("%s lacks %q:\n%s", file, w, text)
		}
	}
}

func TestSignaturesAgreeAcrossFiles(t *testing.T) {
	out := files(t, `type Text = string
func Pair(n int, s Text, f float32) (Text, bool) { return s, n > 0 }`)
	contains(t, "go/main.go", out["go/main.go"],
		"func SBCall0(a0 C.intptr_t, a1 C.SBString, a2 C.float) (out C.SBResult0)",
		"r0, r1 := api.Pair(int(a0), bridgeInput(a1), float32(a2))",
		"C.free(unsafe.Pointer(out.r0.data))")
	contains(t, "bindings.cpp", out["bindings.cpp"],
		"static_assert(sizeof(intptr_t) * 8 == 64",
		"std::free(data.r0.data);",
		`py::arg("n"), py::arg("s"), py::arg("f")`)
	contains(t, "go_api/__init__.pyi", out["go_api/__init__.pyi"],
		"Text = _builtins.str",
		"def pair(n: _builtins.int, s: _builtins.str, f: _builtins.float) -> _builtins.tuple[_builtins.str, _builtins.bool]: ...")
	contains(t, "go_api/__init__.py", out["go_api/__init__.py"], `__all__ = ["GoPanic","pair","Text"]`)
}

func TestPackageWithoutFunctionsOnlyLinksTarget(t *testing.T) {
	out := files(t, `type Text = string`)
	contains(t, "go/main.go", out["go/main.go"], `_ "example.test/api"`)
}
