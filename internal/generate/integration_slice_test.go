package generate

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIntegrationSlices(t *testing.T) {
	python := integrationPython(t)
	t.Run("04_bytes", func(t *testing.T) {
		buildAndRun(t, python, "../../examples/04_bytes_and_holder", "go_bytes", `
import gc
import go_bytes as b
base = b._native._holder_count()
for data in [b"", b"hello\0\xff"]:
    source = b.SliceBytes(data)
    assert isinstance(source, b.Ref)
    result = b.recv_and_return_bytes(source)
    del source
    b._native._collect_go()
    assert result.copy_to_py_bytes() == b"Echo: " + data
    assert result.copy_to_py_list() == list(b"Echo: " + data)
    del result
for ctor in [b.Ref, b.Slice]:
    try: ctor()
    except TypeError: pass
    else: raise AssertionError("abstract base constructed")
gc.collect()
assert b._native._holder_count() == base
`)
	})
	t.Run("05_nested", func(t *testing.T) {
		buildAndRun(t, python, "../../examples/05_slice", "go_slice", `
import gc
import go_slice as b
base = b._native._holder_count()
a = b.SliceInt([1,2,3,4,0])
s = a[:4]
first = b.concat_a(s, b.SliceInt([5]))
second = b.concat_a(s, b.SliceInt([6]))
assert first.copy_to_py_list() == [1,2,3,4,6]
s[-1] = 9
assert a[3] == 9
clone = s.clone()
clone[0] = 70
assert a[0] == 1
assert s[99:0].copy_to_py_list() == []
assert s[-100:100].copy_to_py_list() == [1,2,3,9]
for action, error in [(lambda: s[100], IndexError), (lambda: s[1<<100], IndexError), (lambda: s[::2], ValueError), (lambda: iter(s), TypeError), (lambda: reversed(s), TypeError), (lambda: b.concat_a([],s), TypeError), (lambda: b.SliceInt([1<<100]), OverflowError)]:
    try: action()
    except error: pass
    else: raise AssertionError(action)
outer = b.SliceSliceInt(b.SliceInt([i]) for i in range(20))
b._native._collect_go()
items = outer.copy_to_py_list()
assert [x.copy_to_py_list() for x in items] == [[i] for i in range(20)]
shallow = outer.clone()
shallow[0][0] = 88
assert outer[0][0] == 88
assert b.Slice[int] is b.Slice
try: type("Sub", (b.SliceInt,), {})
except TypeError: pass
else: raise AssertionError("concrete wrapper subclass accepted")

before = b._native._holder_count()
def broken():
    yield b.SliceInt([1])
    raise ValueError("iterator failed")
try: b.SliceSliceInt(broken())
except ValueError: pass
else: raise AssertionError("iterator error swallowed")
gc.collect()
assert b._native._holder_count() == before
del a,s,first,second,clone,outer,items,shallow,action
# The lambdas above are no longer retained.
gc.collect()
b._native._collect_go()
assert b._native._holder_count() == base
`)
	})
}

func TestIntegrationSliceStringOwnership(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `func Echo(s []string) []string { return s }
func Bad() []string { return []string{"\xff", "second"} }
func Both() ([][]int, string, [][]int) { return [][]int{{1}}, "\xff", [][]int{{2}} }`)
	buildAndRun(t, python, source, "slice_contract", `
import gc
import slice_contract as b
base=b._native._holder_count()
s=b.SliceString((v for v in ["", "中文\0tail"]))
assert b.echo(s).copy_to_py_list() == ["", "中文\0tail"]
for _ in range(50):
    bad=b.bad()
    for call in [bad.copy_to_py_list, lambda: bad[0], b.both]:
        try: call()
        except UnicodeDecodeError: pass
        else: raise AssertionError("bad UTF-8 accepted")
    del call, bad
del s
gc.collect()
assert b._native._holder_count() == base
`)
}

func TestIntegrationIndependentModules(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `func Echo(v []int) []int { return v }`)
	first := buildProject(t, python, source, "first_bridge")
	second := buildProject(t, python, source, "second_bridge")
	script := "import sys\nsys.path[:0] = [" + pythonLiteral(filepath.Join(first, ".build/python")) + ", " + pythonLiteral(filepath.Join(second, ".build/python")) + "]\n" + `
import first_bridge as a
import second_bridge as b
x=a.SliceInt([1])
y=b.SliceInt([2])
assert a.echo(x)[0] == 1 and b.echo(y)[0] == 2
for call in [lambda: a.echo(y), lambda: a.SliceInt.clone(y)]:
    try: call()
    except TypeError: pass
    else: raise AssertionError("foreign Go handle accepted")
`
	if output, err := exec.Command(python, "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("independent modules: %v\n%s", err, output)
	}
}
