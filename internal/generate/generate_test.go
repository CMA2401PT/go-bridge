package generate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func module(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.test/api\n\ngo 1.25.0\n")
	write(t, filepath.Join(dir, "api.go"), "package api\n"+source)
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPublicationAndDeterminism(t *testing.T) {
	source := module(t, "func Echo(s string) (int, string) { return len(s), s }")
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	for _, out := range []string{first, second} {
		if err := Run(Config{Target: source, Output: out, Name: "go_echo"}); err != nil {
			t.Fatal(err)
		}
	}
	err := filepath.WalkDir(first, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(first, path)
		if err != nil {
			return err
		}
		a, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(second, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(a, b) {
			t.Errorf("nondeterministic file %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(first, "user.txt"), "do not overwrite")
	if err := Run(Config{Target: source, Output: first}); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatal("nonempty output accepted", err)
	}
	data, err := os.ReadFile(filepath.Join(first, "user.txt"))
	if err != nil || string(data) != "do not overwrite" {
		t.Fatal("existing file changed")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(second, link); err != nil {
		t.Fatal(err)
	}
	if err := Run(Config{Target: source, Output: link}); err == nil {
		t.Fatal("output symlink followed")
	}
}

func TestFailureDoesNotPublish(t *testing.T) {
	source := module(t, "func Unsupported(v complex128) {}")
	out := filepath.Join(t.TempDir(), "absent")
	if err := Run(Config{Target: source, Output: out}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failure left an output directory")
	}
}

func TestEmptyOutputDirectory(t *testing.T) {
	source := module(t, "func F() {}")
	out := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(out, 0755); err != nil {
		t.Fatal(err)
	}
	if err := Run(Config{Target: source, Output: out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "bindings.cpp")); err != nil {
		t.Fatal(err)
	}
}

func integrationPython(t *testing.T) string {
	t.Helper()
	python := os.Getenv("GO_BRIDGE_PYTHON")
	if python == "" {
		t.Skip("set GO_BRIDGE_PYTHON to an interpreter with pybind11 and setuptools")
	}
	resolved, err := exec.LookPath(python)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func buildProject(t *testing.T, python, target, name string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "generated")
	if err := Run(Config{Target: target, Name: name, Output: out}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, filepath.Join(out, "build.py"))
	// Exercise actual C-buffer paths with checkptr. This test-only final
	// -gcflags overrides any earlier -gcflags in the caller's GOFLAGS.
	cmd.Env = append(os.Environ(), "GOEXPERIMENT=cgocheck2", "GOFLAGS="+os.Getenv("GOFLAGS")+" -gcflags=all=-d=checkptr=2")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		cmd := exec.Command(python, "-m", "mypy", "--strict", "--cache-dir", filepath.Join(out, ".mypy_cache"), filepath.Join(out, name, "__init__.pyi"))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("stub type check: %v\n%s", err, output)
		}
	}
	return out
}

func buildAndRun(t *testing.T, python, target, name, script string) string {
	t.Helper()
	out := buildProject(t, python, target, name)
	check := filepath.Join(out, "check.py")
	write(t, check, "import sys\nsys.path.insert(0, "+pythonLiteral(filepath.Join(out, ".build/python"))+")\n"+script)
	cmd := exec.Command(python, check)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python behavior: %v\n%s", err, output)
	}
	return out
}

func pythonLiteral(s string) string {
	// Paths are encoded using a Python-compatible quoted string.
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}

func TestIntegrationPanicTerminatesProcess(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
func PanicText() (int, string) { panic("fatal marker") }
func PanicEmpty() { panic("") }
func PanicNil() int { panic(nil) }
func Divide(n int) int { return 1 / n }
func NilDereference() int { var p *int; return *p }
func Recovered() (result int) {
	defer func() { if recover() != nil { result = 7 } }()
	panic("handled by target")
}
`)
	out := buildProject(t, python, source, "panic_contract")
	setup := "import sys\nsys.path.insert(0, " + pythonLiteral(filepath.Join(out, ".build/python")) + ")\nimport panic_contract as b\n"
	for _, tc := range []struct{ name, call, debug, message string }{
		{"text", "b.panic_text()", "panicnil=0", "panic: fatal marker"},
		{"empty", "b.panic_empty()", "panicnil=0", "panic:"},
		{"nil", "b.panic_nil()", "panicnil=0", "panic:"},
		{"legacy_nil", "b.panic_nil()", "panicnil=1", "panic:"},
		{"divide", "b.divide(0)", "panicnil=0", "integer divide by zero"},
		{"dereference", "b.nil_dereference()", "panicnil=0", "invalid memory address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Run fatal calls only in child processes. A catchable Python error or
			// a normal return must not be mistaken for the expected process exit.
			script := setup + `assert not hasattr(b, "GoPanic")
assert b.recovered() == 7
print("before call", flush=True)
` + tc.call + `
print("returned to Python", flush=True)
sys.exit(78)
`
			cmd := exec.Command(python, "-c", script)
			cmd.Env = append(os.Environ(), "GOTRACEBACK=single", "GODEBUG="+tc.debug)
			output, err := cmd.CombinedOutput()
			if _, ok := err.(*exec.ExitError); !ok {
				t.Fatalf("expected child process failure, got %v\n%s", err, output)
			}
			for _, want := range []string{"before call", "panic:", tc.message} {
				if !bytes.Contains(output, []byte(want)) {
					t.Errorf("missing %q:\n%s", want, output)
				}
			}
			if bytes.Contains(output, []byte("returned to Python")) {
				t.Errorf("panic did not terminate process:\n%s", output)
			}
		})
	}
}

func TestIntegrationWithoutFunctions(t *testing.T) {
	python := integrationPython(t)
	for _, tc := range []struct{ name, declaration, check string }{
		{"alias", "type Text = string", `assert b.Text is str; assert b.__all__ == ["Text"]`},
		{"empty", "", `assert b.__all__ == []`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "initialized")
			t.Setenv("GO_BRIDGE_INIT_MARKER", marker)
			source := module(t, `import ("os"; "time")
`+tc.declaration+`
func init() {
	// Import must wait for Go initialization, not just start it asynchronously.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(os.Getenv("GO_BRIDGE_INIT_MARKER"), []byte("ready"), 0600); err != nil {
		panic(err)
	}
}
`)
			buildAndRun(t, python, source, "init_contract", `
import os
from pathlib import Path
import init_contract as b
assert Path(os.environ["GO_BRIDGE_INIT_MARKER"]).read_text() == "ready"
assert not hasattr(b, "BrInit")
assert not hasattr(b._native, "BrInit")
`+tc.check)
		})
	}
}

func TestIntegrationPanicDuringImport(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `type Text = string
func init() { panic("target init") }`)
	out := buildProject(t, python, source, "init_panic")
	script := "import sys\nsys.path.insert(0, " + pythonLiteral(filepath.Join(out, ".build/python")) + ")\n" + `
print("before import", flush=True)
import init_panic
print("after import", flush=True)
`
	cmd := exec.Command(python, "-c", script)
	cmd.Env = append(os.Environ(), "GOTRACEBACK=single")
	output, err := cmd.CombinedOutput()
	if _, ok := err.(*exec.ExitError); !ok ||
		!bytes.Contains(output, []byte("before import")) ||
		!bytes.Contains(output, []byte("panic: target init")) ||
		bytes.Contains(output, []byte("after import")) {
		t.Fatalf("expected fatal Go initialization during import: %v\n%s", err, output)
	}
}

func TestIntegrationExamples(t *testing.T) {
	python := integrationPython(t)
	for _, tc := range []struct{ example, name, script string }{
		{"01_basic", "go_basic", `import go_basic as b
assert b.int_in_and_out(a=1) == 2
assert b.bool_in_and_out(b=True) is False
assert b.basic_go_spec_type(u16=65535) == 0
for v, error in [(-1, OverflowError), (65536, OverflowError), (1.2, TypeError), (None, TypeError)]:
    try: b.basic_go_spec_type(v)
    except error: pass
    else: raise AssertionError(v)
`},
		{"02_multi_return", "go_multi", `import go_multi as b
assert b.return_two(i=1) == (2, 3)
assert b.return_four(10) == (11, 12, 13, 14)
`},
		{"03_str", "go_string", `import go_string as b
for s in ["", "你好\0world", "plain"]:
    assert b.recv_and_return(inp=s) == "Echo: " + s
try: b.recv_and_return("\ud800")
except UnicodeEncodeError: pass
else: raise AssertionError("surrogate accepted")
for v in [b"bytes", None, 1]:
    try: b.recv_and_return(v)
    except TypeError: pass
    else: raise AssertionError(v)
`},
	} {
		t.Run(tc.example, func(t *testing.T) { buildAndRun(t, python, "../../examples/"+tc.example, tc.name, tc.script) })
	}
}

func TestIntegrationScalarContract(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type Text = string
func String(s Text) Text { return s }
func Int(i int) int { return i }
func Int8(v int8) int8 { return v }
func Int16(v int16) int16 { return v }
func Int32(v int32) int32 { return v }
func Int64(v int64) int64 { return v }
func Uint(v uint) uint { return v }
func Uint8(v uint8) uint8 { return v }
func Uint16(v uint16) uint16 { return v }
func Uint32(v uint32) uint32 { return v }
func Uint64(v uint64) uint64 { return v }
func Uintptr(v uintptr) uintptr { return v }
func Float32(v float32) float32 { return v }
func Float64(v float64) float64 { return v }
func Bool(v bool) bool { return v }
func Void() {}
func Many(from string, _ int) (string, int, string, bool, float64) { return from, 2, "tail", true, 1.5 }
func BadUTF8() (string, string) { return "\xff", "allocated second result" }
func Tuple() (int, string) { return 1, "ok" }
var saved string
func Save(s string) { saved = s }
func Load() string { return saved }
`)
	// The generated project lives outside this module. Its build must still
	// resolve this module's local replacement rather than the generator's go.mod.
	dep := filepath.Join(source, "dependency")
	if err := os.Mkdir(dep, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dep, "go.mod"), "module example.test/dependency\n\ngo 1.25.0\n")
	write(t, filepath.Join(dep, "value.go"), "package dependency\nfunc Value() int { return 73 }\n")
	write(t, filepath.Join(source, "go.mod"), "module example.test/api\n\ngo 1.25.0\n\nrequire example.test/dependency v0.0.0\nreplace example.test/dependency => ./dependency\n")
	write(t, filepath.Join(source, "dep.go"), "package api\nimport \"example.test/dependency\"\nfunc Dependency() int { return dependency.Value() }\n")
	buildAndRun(t, python, source, "scalar_contract", `
import ast
import gc
import struct
from pathlib import Path
import scalar_contract as b

assert b.Text is str
assert b.dependency() == 73
assert b.string("中文\0text") == "中文\0text"
assert b.void() is None
assert b.many(from_="a\0b", arg2=1) == ("a\0b", 2, "tail", True, 1.5)
assert b.tuple() == (1, "ok")
word_bits = struct.calcsize("P") * 8
for name, bits, signed in [
    ("int", word_bits, True), ("int8", 8, True), ("int16", 16, True),
    ("int32", 32, True), ("int64", 64, True), ("uint", word_bits, False),
    ("uint8", 8, False), ("uint16", 16, False), ("uint32", 32, False),
    ("uint64", 64, False), ("uintptr", word_bits, False)]:
    f = getattr(b, name)
    low = -(1 << (bits - 1)) if signed else 0
    high = (1 << (bits - int(signed))) - 1
    for v in [low, 0, high, True]: assert f(v) == v
    for v, error in [(low - 1, OverflowError), (high + 1, OverflowError), (1 << 200, OverflowError),
                     (1.5, TypeError), (None, TypeError), ("1", TypeError)]:
        try: f(v)
        except error: pass
        else: raise AssertionError((name, v))
assert b.bool(True) is True
for v in [0, 1, None]:
    try: b.bool(v)
    except TypeError: pass
    else: raise AssertionError(v)
assert b.float64(2) == 2.0
assert b.float32(1.5) == 1.5
assert b.float64(float("inf")) == float("inf")
for _ in range(100):
    try: b.bad_utf8()
    except UnicodeDecodeError: pass
    else: raise AssertionError("invalid UTF-8 accepted")
assert b.int(42) == 42
b.save("".join(["owned", " by Go", "\0data"]))
gc.collect()
assert b.load() == "owned by Go\0data"
stub = Path(b.__file__).with_suffix(".pyi").read_text()
ast.parse(stub)
# Evaluate annotations as well: public functions called int/bool/tuple must
# not shadow builtin types in stubs. A callable object is not a type annotation.
namespace = {}
exec(compile(stub, "generated.pyi", "exec"), namespace)
import typing
assert typing.get_type_hints(namespace["int"], namespace)["return"] is int
assert typing.get_type_hints(namespace["tuple"], namespace)["return"] == tuple[int, str]
`)
}
