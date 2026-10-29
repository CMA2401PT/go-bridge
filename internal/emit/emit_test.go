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
		"func BrCall0(a0 C.intptr_t, a1 C.BrString, a2 C.float) (out C.BrResult0)",
		"r0, r1 := api.Pair(int(a0), bridgeInput(a1), float32(a2))",
		"out.r0 = bridgeString(string(r0))")
	contains(t, "bindings.cpp", out["bindings.cpp"],
		"static_assert(sizeof(intptr_t) * 8 == 64",
		"std::free(data.r0.data);",
		`py::arg("n"), py::arg("s"), py::arg("f")`)
	contains(t, "go_api/__init__.pyi", out["go_api/__init__.pyi"],
		"Text = _builtins.str",
		"def pair(n: _builtins.int, s: _builtins.str, f: _builtins.float) -> _builtins.tuple[_builtins.str, _builtins.bool]: ...")
	contains(t, "go_api/__init__.py", out["go_api/__init__.py"], `__all__ = ["pair","Text"]`)
}

func TestPackageWithoutFunctionsInitializesTarget(t *testing.T) {
	out := files(t, `type Text = string`)
	contains(t, "go/main.go", out["go/main.go"], `_ "example.test/api"`, "//export BrInit", "func BrInit() {}")
	contains(t, "bindings.cpp", out["bindings.cpp"], "BrInit();")
	contains(t, "go_api/__init__.py", out["go_api/__init__.py"], "from . import _native")
}

func TestVoidHasNoResultOrPanicProtocol(t *testing.T) {
	out := files(t, `func Fail() { panic("failure") }`)
	contains(t, "go/main.go", out["go/main.go"], "func BrCall0()", "api.Fail()")
	contains(t, "bindings.cpp", out["bindings.cpp"], "BrCall0();", "return py::none();")
	for _, path := range []string{"go/main.go", "bindings.cpp", "go_api/__init__.py", "go_api/__init__.pyi"} {
		for _, removed := range []string{"BrResult", "defer", "recover", "panicked", "panic_message", "GoPanic"} {
			if strings.Contains(out[path], removed) {
				t.Errorf("%s contains %q", path, removed)
			}
		}
	}
}

func TestEmptyPackageExports(t *testing.T) {
	out := files(t, "")
	contains(t, "go_api/__init__.py", out["go_api/__init__.py"], "from . import _native", "__all__ = []")
}

func TestAnyEmission(t *testing.T) {
	out := files(t, `
type Value = any
type Count int
type Values []int
type IntPtr *int
type Point struct { Data any }
func Echo(value any, pointer *any, values []any) interface{} { return value }
`)
	contains(t, "go/any.go", out["go/any.go"], "return cgo.Handle(handle).Value()", "return bridgePin(value)")
	contains(t, "go/main.go", out["go/main.go"], "bridgeAny(a0)", "bridgePinAny(r0)", ".(api.Count)", ".(api.Values)", ".(api.IntPtr)", "bridgePinAny(api.Values(", "bridgePinAny(api.IntPtr(", "out.ok = 1")
	contains(t, "bindings.cpp", out["bindings.cpp"], `"int_as_any"`, `"any_as_int"`, `"try_any_as_int"`, `"as_any"`, `"from_any"`, `"try_from_any"`, "owner.values[0] = result.value", "if (!result.ok)", "py::weakref(classType")
	contains(t, "go_api/__init__.pyi", out["go_api/__init__.pyi"], "class GoAny(GoInterface):", "def int_as_any(value: _builtins.int) -> GoAny:", "def any_as_string(value: GoInterface) -> _builtins.str:", "def try_any_as_string(value: GoInterface) -> _builtins.str | None:", "def as_any(self) -> GoAny:", "def from_any(cls, value: GoInterface) -> _typing.Self:", "def try_from_any(cls, value: GoInterface) -> _typing.Self | None:", "class PtrGoAny(Ptr[\"GoAny\"]):", "class SliceGoAny(Slice[\"GoAny\"]):", "Value = GoAny")
	contains(t, "go_api/__init__.py", out["go_api/__init__.py"], "from ._native import int_as_any as int_as_any", "from ._native import GoAny as GoAny")
	for path, source := range out {
		if strings.HasSuffix(path, ".go") && (strings.Contains(source, "anyCell") || strings.Contains(source, "recover(")) {
			t.Errorf("%s introduces an any Cell or panic recovery", path)
		}
	}
}
