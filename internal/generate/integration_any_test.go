package generate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationAnyExample(t *testing.T) {
	python := integrationPython(t)
	out := buildProject(t, python, "../../examples/10_any", "go_any")
	source, err := os.ReadFile("../../examples/10_any/test.py")
	if err != nil {
		t.Fatal(err)
	}
	packageDir := filepath.Join(out, ".build/python")
	pathSetup := `package_dir = example_dir.parent / ".build" / example_dir.name / "python"`
	if strings.Count(string(source), pathSetup) != 1 {
		t.Fatal("any example package path setup changed")
	}
	script := strings.Replace(string(source), pathSetup, "package_dir = Path("+pythonLiteral(packageDir)+")", 1)
	check := filepath.Join(out, "check.py")
	write(t, check, script)
	if output, err := exec.Command(python, check).CombinedOutput(); err != nil {
		t.Fatalf("any example: %v\n%s", err, output)
	}
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		cmd := exec.Command(python, "-m", "mypy", "--strict", "--cache-dir", filepath.Join(out, ".mypy_cache"), check)
		cmd.Env = append(os.Environ(), "MYPYPATH="+out)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("any consumer type check: %v\n%s", err, output)
		}
	}
}

func TestIntegrationAnyComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type Value = interface{}
type Values []any
type AnyPtr *any
type Box struct { Data any; Values []any }
type Count int
type Text string
type Ratio float32
func (v Count) AsAny() int { return int(v) }
func (v Count) FromAny() int { return int(v) }
func (v Count) TryFromAny() int { return int(v) }
func Echo(value Value) any { return value }
func EchoPointer(value *any) *any { return value }
func EchoSlice(value []any) []any { return value }
func SliceNil() any { return []any(nil) }
func SliceIsNil(value any) bool { return value.([]any) == nil }
func Many() (any, []any, *any, string) {
    value := any(Count(3))
    return value, []any{nil, value, (*any)(nil)}, &value, string([]byte{255})
}
func Hidden() any { return struct{ x int }{7} }
func InvalidText() any { return Text(string([]byte{255})) }
func ReadHidden(value any) int { return value.(struct{ x int }).x }
func Cycle() any { value := make([]any, 1); value[0] = value; return value }
`)
	out := buildProject(t, python, source, "go_composed_any")
	setup := "import sys\nsys.path.insert(0," + pythonLiteral(filepath.Join(out, ".build/python")) + ")\nimport go_composed_any as bridge\n"
	script := setup + `
import gc
from typing import assert_type
base = bridge._native._holder_count()
assert bridge.Value is bridge.GoAny
boxed = bridge.int_as_any(7)
nil_any = bridge.GoAny.nil()
assert nil_any.is_nil and not boxed.is_nil
assert not hasattr(bridge.GoAny, "as_any")
assert bridge.Count(3).as_any_() == 3
assert bridge.Count(3).from_any_() == 3
assert bridge.Count(3).try_from_any_() == 3
named = bridge.Count(3).as_any()
assert type(bridge.Count.from_any(named)) is bridge.Count
assert bridge.try_any_as_int(named) is None
text = bridge.Text("中文\0").as_any()
ratio = bridge.Ratio(1.25).as_any()
assert type(bridge.Text.from_any(text)) is bridge.Text and bridge.Text.from_any(text) == "中文\0"
assert type(bridge.Ratio.from_any(ratio)) is bridge.Ratio and bridge.Ratio.from_any(ratio) == 1.25
assert bridge.try_any_as_string(text) is None and bridge.try_any_as_float32(ratio) is None
for call in [bridge.Text.from_any, bridge.Text.try_from_any]:
    for _ in range(100):
        try: call(bridge.invalid_text())
        except UnicodeDecodeError: pass
        else: raise AssertionError("invalid named string decoded")
del call
values = bridge.SliceGoAny([boxed, nil_any, named, bridge.PtrGoAny.nil().as_any()])
assert_type(values[0], bridge.GoAny)
assert bridge.any_as_int(values[0]) == 7 and values[1].is_nil
assert bridge.PtrGoAny.from_any(values[3]).is_nil
assert bridge.SliceGoAny.try_from_any(boxed) is None
restored = bridge.SliceGoAny.from_any(values.as_any())
snapshot = restored[0]
restored[0] = bridge.string_as_any("changed")
assert bridge.any_as_int(snapshot) == 7
assert bridge.any_as_string(values[0]) == "changed"
shared = values[:]
cloned = values.clone()
shared[1] = boxed
assert bridge.any_as_int(values[1]) == 7 and cloned[1].is_nil
copied = values.copy_to_py_list()
assert len(copied) == 4 and type(copied[0]) is bridge.GoAny
named_slice = bridge.Values(values)
assert bridge.SliceGoAny.try_from_any(named_slice.as_any()) is None
assert bridge.Values.from_any(named_slice.as_any()).unname()[2].is_nil is False
nil_slice = bridge.SliceGoAny.from_any(bridge.slice_nil())
assert len(nil_slice) == 0 and bridge.slice_is_nil(nil_slice.as_any())
pointer = bridge.PtrGoAny.from_value(nil_any)
assert not pointer.is_nil and pointer.value.is_nil
assert bridge.PtrGoAny.from_any(pointer.as_any()).value.is_nil
pointer.value = boxed
old = pointer.value
bridge.echo_pointer(pointer).value = named
assert bridge.any_as_int(old) == 7 and bridge.Count.from_any(pointer.value) == 3
created = nil_any.ptr()
assert not created.is_nil and created.value.is_nil
named_pointer = bridge.AnyPtr(pointer)
assert bridge.PtrGoAny.try_from_any(named_pointer.as_any()) is None
assert bridge.Count.from_any(bridge.AnyPtr.from_any(named_pointer.as_any()).value) == 3
box = bridge.Box()
assert box.data.is_nil
box.data = boxed
field = box.data
box.data = named
assert bridge.any_as_int(field) == 7 and bridge.Count.from_any(box.data) == 3
assert bridge.read_hidden(bridge.echo(bridge.hidden())) == 7
cycle = bridge.SliceGoAny.from_any(bridge.cycle())
assert len(bridge.SliceGoAny.from_any(cycle[0])) == 1
for bad in [None, 1, bridge.Count(1), pointer, values]:
    for call in [lambda: bridge.echo(bad), lambda: bridge.SliceGoAny([bad]), lambda: bridge.PtrGoAny.from_value(bad)]:
        try: call()
        except TypeError: pass
        else: raise AssertionError("unboxed value accepted")
del bad, call
before = bridge._native._holder_count()
for _ in range(100):
    try: bridge.many()
    except UnicodeDecodeError: pass
    else: raise AssertionError("invalid string decoded")
gc.collect()
assert bridge._native._holder_count() == before
del boxed, nil_any, named, values, restored, snapshot, shared, cloned, copied, named_slice, nil_slice
del pointer, old, created, named_pointer, box, field, cycle
del text, ratio
gc.collect()
bridge._native._collect_go()
assert bridge._native._holder_count() == base
`
	if output, err := exec.Command(python, "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("any composition: %v\n%s", err, output)
	}
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		consumer := filepath.Join(out, "consumer.py")
		write(t, consumer, `from typing import assert_type
import go_composed_any as bridge
boxed = bridge.int_as_any(3)
assert_type(boxed, bridge.GoAny)
assert_type(bridge.try_any_as_int(boxed), int | None)
count = bridge.Count.try_from_any(boxed)
assert_type(count, bridge.Count | None)
if count is not None:
    assert_type(count, bridge.Count)
pointer = bridge.PtrGoAny.try_from_any(boxed)
assert_type(pointer, bridge.PtrGoAny | None)
if pointer is not None:
    assert_type(pointer.value, bridge.GoAny)
assert_type(bridge.GoAny.nil().ptr(), bridge.PtrGoAny)
assert_type(bridge.SliceGoAny([boxed])[0], bridge.GoAny)
assert_type(bridge.SliceGoAny([boxed]).copy_to_py_list(), list[bridge.GoAny])
assert_type(bridge.Values.from_any(boxed), bridge.Values)
assert_type(bridge.AnyPtr.from_any(boxed), bridge.AnyPtr)
assert_type(bridge.Box().data, bridge.GoAny)
bridge.echo(None)  # type: ignore[arg-type]
bridge.SliceGoAny([1])  # type: ignore[list-item]
bridge.PtrGoAny.from_value(None)  # type: ignore[arg-type]
bridge.Count.from_any(3)  # type: ignore[arg-type]
`)
		cmd := exec.Command(python, "-m", "mypy", "--strict", "--cache-dir", filepath.Join(out, ".mypy_cache"), consumer)
		cmd.Dir = out
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("any composition consumer types: %v\n%s", err, output)
		}
	}
	for _, call := range []string{"bridge.PtrGoAny.nil().value", "bridge.read_hidden(bridge.GoAny.nil())"} {
		cmd := exec.Command(python, "-c", setup+"print('before fatal call', flush=True)\n"+call+"\n")
		cmd.Env = append(os.Environ(), "GOTRACEBACK=single")
		output, err := cmd.CombinedOutput()
		if _, ok := err.(*exec.ExitError); !ok || !bytes.Contains(output, []byte("before fatal call")) || !bytes.Contains(output, []byte("panic:")) {
			t.Fatalf("Go panic was not fatal: %v\n%s", err, output)
		}
	}
}

func TestIntegrationAnyAliasOnly(t *testing.T) {
	python := integrationPython(t)
	source := module(t, "type Value = any\ntype Count int\n")
	buildAndRun(t, python, source, "go_any_alias", `
import go_any_alias as bridge
assert bridge.Value is bridge.GoAny
assert bridge.any_as_int(bridge.int_as_any(2)) == 2
assert type(bridge.Count.from_any(bridge.Count(3).as_any())) is bridge.Count
`)
}

func TestIntegrationAnyHolderChecks(t *testing.T) {
	integrationPython(t)
	source := module(t, "type Value = any\ntype Count int\n")
	out := filepath.Join(t.TempDir(), "generated")
	if err := Run(Config{Target: source, Output: out}); err != nil {
		t.Fatal(err)
	}
	check := filepath.Join(out, "go", "any_test.go")
	write(t, check, `package main
import (
    api "example.test/api"
    "sync"
    "testing"
)
func TestAnyRoots(t *testing.T) {
    if bridgePinAny(nil) != 0 || bridgeAny(0) != nil { t.Fatal("nil interface changed") }
    var workers sync.WaitGroup
    for worker := 0; worker < 8; worker++ {
        workers.Add(1)
        go func() {
            defer workers.Done()
            for index := 0; index < 100; index++ {
                handle := bridgePinAny(api.Count(index))
                copied := bridgePinAny(bridgeAny(handle))
                BrRelease(handle)
                value, ok := bridgeAny(copied).(api.Count)
                if !ok || int(value) != index { t.Error("named dynamic value changed") }
                BrRelease(copied)
                pointer := bridgePinAny((*api.Count)(nil))
                if pointer == 0 { t.Error("typed nil lost its dynamic type") }
                restored, ok := bridgeAny(pointer).(*api.Count)
                if !ok || restored != nil { t.Error("typed nil changed") }
                BrRelease(pointer)
            }
        }()
    }
    workers.Wait()
    BrRelease(0)
    if BrHolderCount() != 0 { t.Fatal("leaked roots") }
}
`)
	cmd := exec.Command("go", "test", "-race", "-gcflags=all=-d=checkptr=2", filepath.Join(out, "go", "main.go"), filepath.Join(out, "go", "holder.go"), filepath.Join(out, "go", "any.go"), check)
	cmd.Dir = source
	cmd.Env = append(os.Environ(), "GOEXPERIMENT=cgocheck2")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("any holder race/checkptr/cgocheck2: %v\n%s", err, output)
	}
}

func TestIntegrationAnyDependencyAliases(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `import "example.test/api/dependency"
type Value = any
type Count = dependency.Count
type Text = dependency.Text
`)
	dependency := filepath.Join(source, "dependency")
	if err := os.Mkdir(dependency, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dependency, "types.go"), "package dependency\ntype Count int\ntype Text string\n")
	buildAndRun(t, python, source, "go_any_dependency", `
import go_any_dependency as bridge
assert bridge.Value is bridge.GoAny
assert bridge.Count.from_any(bridge.Count(5).as_any()) == 5
assert type(bridge.Text.from_any(bridge.Text("hello").as_any())) is bridge.Text
`)
}
