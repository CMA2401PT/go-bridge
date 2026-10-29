package generate

import "testing"

func TestIntegrationFuncExample(t *testing.T) { runExample(t, "14_func", "go_func") }
func TestIntegrationFuncComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type Transform func(int) (string, int)
func (t Transform) Apply(n int) (string,int) {return t(n)}
type Recursive func() Recursive
type Box struct { Call Transform }
func Make() Transform { return func(n int)(string,int){return "text",n+1} }
func Recurse() Recursive {return Recurse}
func Pointer(v *Transform) *Transform {return v}
func Items(v []Transform) []Transform {return v}
func Maps(v map[string]Transform) map[string]Transform {return v}
func Array(v [1]Transform) [1]Transform {return v}
func Channel(v chan Transform) chan Transform {return v}
func Any(v any) any {return v}
func Identity(v int) int {return v}
func Factory() func() func(int) int {return func()func(int)int{return Identity}}
func Void() func() {return func(){}}
func Invalid() func() (string,[]int) {return func()(string,[]int){return string([]byte{255}),[]int{1}}}
func Signal(done chan int) func(){return func(){done<-1}}
`)
	buildAndRun(t, python, source, "go_funcs", `
import gc,subprocess,sys,threading
import go_funcs as b
base=b._native._holder_count()
def check():
 f=b.make();assert f(3)==("text",4) and f.apply(5)==("text",6)
 raw=f.unname();assert raw(arg1=7)==("text",8)
 assert b.Transform(raw)(9)==("text",10)
 assert b.Box(call=f).call(1)==("text",2)
 assert b.PtrTransform(f).value(2)==("text",3)
 assert b.SliceTransform([f])[0](3)==("text",4)
 assert b.MapStringTransform([("f",f)])["f"](4)==("text",5)
 assert b.Array1Transform([f])[0](5)==("text",6)
 ch=b.ChanTransform(1);ch.send(f);values=[];ch.recv(values.append);assert values[0](6)==("text",7)
 assert b.Transform.from_any(f.as_any())(7)==("text",8)
 assert b.FuncIntToStringAndInt.try_from_any(f.as_any()) is None
 nil=b.Transform.nil();assert b.Transform.from_any(nil.as_any()).is_nil and not nil.as_any().is_nil
 assert nil.unname().is_nil
 assert b.recurse()()() is not None
 assert b.factory()()(arg1=4)==4
 assert b.void()() is None
 done=b.ChanInt();signal=b.signal(done);thread=threading.Thread(target=signal);thread.start()
 received=[];assert done.recv(received.append);thread.join(timeout=3);assert not thread.is_alive() and received==[1]
 before=b._native._holder_count()
 for _ in range(3):
  try:b.invalid()()
  except UnicodeDecodeError:pass
  else:raise AssertionError("bad text accepted")
 assert b._native._holder_count()==before
check();gc.collect();b._native._collect_go();assert b._native._holder_count()==base
result=subprocess.run([sys.executable,"-c",f"import sys;sys.path.insert(0,{sys.path[0]!r});import go_funcs as b;b.Transform.nil()(1)"],capture_output=True)
assert result.returncode!=0 and b"panic:" in result.stderr
`)
}
