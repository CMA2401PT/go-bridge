package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIntegrationInterfaceExample(t *testing.T) { runExample(t, "15_interface", "go_interface") }
func TestIntegrationInterfaceComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
import "fmt"
type Count int
func(v Count) Read()int {return int(v)}
func(v Count) String()string {return "count"}
func External() fmt.Stringer {return Count(4)}
type Reader interface { Read() int }
type Writer interface { Write(int) }
type ReadWriter interface {Reader;Writer}
type Value struct {N int}
func (v Value) Read() int {return v.N}
func (v *Value) Write(n int) {v.N=n}
type Empty interface{}
type Recursive interface { Next() Recursive }
type node struct {}
func (node) Next() Recursive {return node{}}
func Recurse() Recursive {return node{}}
type Box struct { R Reader }
func Pass(v Reader) Reader {return v}
func Anonymous(v interface {Read() int}) interface {Read() int} {return v}
func Pointer(v *Reader) *Reader {return v}
func Slice(v []Reader) []Reader {return v}
func Map(v map[Reader]Reader) map[Reader]Reader {return v}
func Array(v [1]Reader) [1]Reader {return v}
func Channel(v chan Reader) chan Reader {return v}
func Function(v Reader) func() Reader {return func()Reader{return v}}
// The private method stays in the Go interface constraint; only public methods are exposed.
type Sealed interface { hidden(); Read() int }
type sealed struct {}
func (sealed) hidden() {}
func (sealed) Read() int {return 42}
func Hidden() Sealed {return sealed{}}
`)
	out := buildAndRun(t, python, source, "go_interfaces", `
import gc,subprocess,sys
import go_interfaces as b
base=b._native._holder_count()
def check():
 v=b.Value(n=7)
 reader=b.Reader(v);v.n=8;assert reader.read()==7
 assert b.Writer.try_from_any(v.as_any()) is None
 rw=b.ReadWriter(v.addr());rw.write(9);assert rw.read()==9 and v.n==9
 assert b.Value.try_from_any(rw) is None
 assert b.PtrValue.from_any(rw).value.n==9
 raw=b.InterfaceReadToInt(reader);assert b.anonymous(raw).read()==7 and b.Reader.from_any(raw.as_any()).read()==7
 assert b.pass_(reader).read()==7
 writer=b.Writer(rw)
 assert b.Reader.from_any(writer).read()==9
 assert b.Reader(rw).read()==9
 assert b.Reader(b.Count(3)).read()==3
 assert b.Stringer(b.Count(2)).string()=="count"
 assert b.Count.from_any(b.external())==4
 assert b.Reader(b.hidden()).read()==42
 for call in [lambda:b.Reader(writer),lambda:b.Writer(v),lambda:b.pass_(v)]:
  try:call()
  except TypeError:pass
  else:raise AssertionError("incompatible static interface accepted")
 assert b.Box(r=reader).r.read()==7
 p=b.PtrReader(reader);assert p.value.read()==7;p.value=b.Reader.nil();assert p.value.is_nil
 assert b.SliceReader([reader])[0].read()==7
 m=b.MapReaderReader([(reader,reader)]);assert m[b.Reader.from_any(b.Value(n=7).as_any())].read()==7
 assert b.Array1Reader([reader])[0].read()==7
 ch=b.ChanReader(1);ch.send(reader);values=[];ch.recv(values.append);assert values[0].read()==7
 assert b.function(reader)().read()==7
 nil=b.Reader.nil();assert nil.as_any().is_nil
 assert b.Reader(b.ReadWriter.nil()).is_nil
 assert b.Reader.try_from_any(nil) is None
 typed=b.Reader.from_any(b.PtrValue.nil().as_any());assert not typed.is_nil
 assert b.PtrValue.from_any(typed).is_nil
 empty=b.Empty(reader);assert b.Reader.from_any(empty).read()==7
 assert b.Empty(nil).is_nil
 assert b.any_as_int(b.Empty(b.int_as_any(5)))==5
 assert b.Empty.nil().is_nil
 assert b.recurse().next().next() is not None
 assert b.hidden().read()==42
 assert b.Sealed.try_from_any(v.as_any()) is None
check();gc.collect();b._native._collect_go();assert b._native._holder_count()==base
result=subprocess.run([sys.executable,"-c",f"import sys;sys.path.insert(0,{sys.path[0]!r});import go_interfaces as b;b.Reader.nil().read()"],capture_output=True)
assert result.returncode!=0 and b"panic:" in result.stderr
`)
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		consumer := filepath.Join(out, "consumer.py")
		write(t, consumer, `from typing import assert_type
import go_interfaces as b
v=b.Value(n=1)
r=b.Reader(v)
rw=b.ReadWriter(v.addr())
w=b.Writer(rw)
assert_type(b.pass_(r),b.Reader)
assert_type(b.Reader(rw),b.Reader)
assert_type(b.Reader.from_any(w),b.Reader)
assert_type(b.PtrValue.from_any(rw),b.PtrValue)
assert_type(b.Count.from_any(b.external()),b.Count)
assert_type(b.Reader(b.Count(2)),b.Reader)
assert_type(b.Reader(b.hidden()),b.Reader)
assert_type(r.as_any(),b.GoAny)
assert_type(b.Empty(r),b.Empty)
assert_type(b.any_as_int(b.Empty(b.int_as_any(2))),int)
maybe=b.Reader.try_from_any(w)
assert_type(maybe,b.Reader | None)
if maybe is not None:
    assert_type(maybe.read(),int)
b.Reader(w)  # type: ignore[arg-type]
b.Writer(v)  # type: ignore[arg-type]
b.pass_(v)  # type: ignore[arg-type]
b.Reader.from_any(v)  # type: ignore[arg-type]
`)
		cmd := exec.Command(python, "-m", "mypy", "--strict", consumer)
		cmd.Dir = out
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("interface consumer: %v\n%s", err, output)
		}
	}

}
