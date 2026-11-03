package generate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runExample applies the handwritten behavior tests to freshly generated code.
func runExample(t *testing.T, example, name string) {
	t.Helper()
	python := integrationPython(t)
	out := buildProject(t, python, "../../examples/"+example, name)
	source, err := os.ReadFile("../../examples/" + example + "/test.py")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.Replace(string(source), `package_dir = example_dir.parent / ".build" / example_dir.name / "python"`, "package_dir = Path("+pythonLiteral(filepath.Join(out, ".build/python"))+")", 1)
	check := filepath.Join(out, "check.py")
	write(t, check, script)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, python, check).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", example, err, output)
	}
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		cmd := exec.Command(python, "-m", "mypy", "--strict", check)
		cmd.Env = append(os.Environ(), "MYPYPATH="+out)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s types: %v\n%s", example, err, output)
		}
	}
}
func TestIntegrationMapExample(t *testing.T) { runExample(t, "12_map", "go_map") }
func TestIntegrationMapComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type Key struct { N int }
type Value struct { N int; Items []int }
type Table map[Key]Value
type Recursive map[string]Recursive
func (t Table) Clear() int { return len(t) }
func Echo(t Table) Table { return t }
func Box(t any) any { return t }
func Ptr(t *Table) *Table { return t }
func Slice(t []Table) []Table { return t }
func Channels(t chan Table) chan Table { return t }
func Keys(t map[*Key]int) map[*Key]int { return t }
func AnyKeys(t map[any]int) map[any]int { return t }
func Strings(t map[string]string) map[string]string { return t }
func Broken() map[string]string { return map[string]string{"a":string([]byte{255}),"b":"ok"} }
`)
	buildAndRun(t, python, source, "go_maps", `
import gc
import subprocess
import sys
import go_maps as b
base=b._native._holder_count()
def check():
 k=b.Key(n=1);v=b.Value(n=2,items=b.SliceInt([3]))
 m=b.Table(b.MapKeyValue([(k,v)]));k.n=7;v.n=8
 key=b.Key(n=1)
 assert key in m and m[key].n==2
 got=m[key];got.n=9;got.items[0]=4
 assert m[key].n==2 and v.items[0]==4
 clone=m.clone();b.echo(m)[key]=v
 assert clone[key].n==2 and m[key].n==8
 assert m.clear_()==1
 p=b.PtrTable(m);p.value[key]=b.Value(n=11)
 assert m[key].n==11 and b.Table.from_any(m.as_any())[key].n==11
 assert b.MapKeyValue.try_from_any(m.as_any()) is None
 assert b.Table.nil().unname().is_nil
 assert not b.Table.nil().as_any().is_nil
 assert b.Table.from_any(b.Table.nil().as_any()).is_nil
 entries=m.copy_to_py_items();assert len(entries)==1 and entries[0][0].n==1
 entries[0][0].n=10;assert key in m
 sl=b.SliceTable([m]);sl[0].delete(key);assert len(m)==0
 ch=b.ChanTable(capacity=1);ch.send(value=m);item,ok=ch.recv();assert ok;item[key]=v;assert len(m)==1
 ch.send(value=b.Table.nil());ch.close();item,ok=ch.recv();assert ok and item.is_nil
 item,ok=ch.recv();assert not ok and type(item) is b.Table and item.is_nil
 pointers=b.MapPtrKeyInt([(k.addr(),4)])
 assert pointers[k.addr()]==4 and k.addr() in pointers
 dynamic=b.MapGoAnyInt([(key.as_any(),7)])
 assert dynamic[b.Key(n=1).as_any()]==7
 rec=b.Recursive(b.MapStringRecursive());rec["self"]=rec
 assert "self" in rec["self"];rec.delete("self")
 before=b._native._holder_count()
 for _ in range(3):
  try:b.broken().copy_to_py_items()
  except UnicodeDecodeError:pass
  else:raise AssertionError("bad text accepted")
 assert b._native._holder_count()==before
check();gc.collect();b._native._collect_go();assert b._native._holder_count()==base
result=subprocess.run([sys.executable,"-c",f"import sys;sys.path.insert(0,{sys.path[0]!r});import go_maps as b;b.MapStringString.nil()['x']='y'"],capture_output=True)
assert result.returncode!=0 and b"panic:" in result.stderr
`)
}
