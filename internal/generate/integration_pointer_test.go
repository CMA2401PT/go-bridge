package generate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIntegrationPointers(t *testing.T) {
	python := integrationPython(t)
	t.Run("07_values", func(t *testing.T) {
		buildAndRun(t, python, "../../examples/07_ptr_basic", "go_ptr", `
import gc
import go_ptr as b
base=b._native._holder_count()
x=b.MyNamedNonStructInt(4)
p=x.ptr()
p.inc()
assert x==4 and p.value==5 and type(p.value) is b.MyNamedNonStructInt
assert p.echo_non_struct_int()=="my non struct int: 5"
assert p.ptr_echo_non_struct_int()=="my non struct int: 5"
p.value=b.MyNamedNonStructInt(10)
assert b.ptr_fn_echo_non_struct_int(p)=="my non struct int: 10"
a=b.PtrInt()
assert a.value==0
b.inc_int(a)
assert a.value==1 and b.ptr_fn_echo_int(a)=="*int: 1"
s=b.SliceInt([1,2])
sp=s.ptr()
old=sp.value
b.append_slice_int(sp,3)
assert len(s)==len(old)==2 and sp.value.copy_to_py_list()==[1,2,3]
named=b.MyNamedNonStructSliceInt(s)
np=named.ptr()
np.append(4)
assert np.value.unname().copy_to_py_list()==[1,2,4]
assert len(named)==2
for call in [lambda:b.inc_int(1), lambda:b.ptr_fn_echo_non_struct_int(a), lambda:b.fn_echo_non_struct_int(p), lambda:b.PtrInt(None)]:
    try:call()
    except TypeError:pass
    else:raise AssertionError("pointer mismatch accepted")
del p,a,s,sp,old,named,np,call
gc.collect()
assert b._native._holder_count()==base
`)
	})
	t.Run("08_levels", func(t *testing.T) {
		buildAndRun(t, python, "../../examples/08_multi_ptr", "go_multi_ptr", `
import gc
import go_multi_ptr as b
base=b._native._holder_count()
p=b.PtrInt(1)
pp=p.ptr()
ppp=pp.ptr()
b.inc_ppp(ppp)
assert p.value==2 and b.echo_ppp(ppp)=="***int: 2"
other=b.PtrInt(9)
b.retarget(pp,other)
assert pp.value.value==9 and p.value==2
pp.value.value=10
assert other.value==10
named=b.IntPtr(other)
b.inc_int_ptr(named)
assert other.value==11 and named.unname().value==11
assert b.IntPtr(b.PtrInt.nil()).is_nil
b.clear(pp)
assert pp.value.is_nil
nil=p.nil()
outer_nil=nil.ptr()
assert not outer_nil.is_nil and outer_nil.value.is_nil
box=b.Box(count=b.Counter(3),inner=b.Inner(n=7))
view=box.inner
bp=box.addr()
bpp=bp.ptr()
b.inc_box_pp(bpp)
assert box.count==4 and view.n==8
assert not hasattr(bpp,"inc")
del box,bp,bpp
b._native._collect_go()
assert view.n==8
del p,pp,ppp,other,named,nil,outer_nil,view
gc.collect()
assert b._native._holder_count()==base
`)
	})
}

func TestIntegrationNil(t *testing.T) {
	python := integrationPython(t)
	out := buildProject(t, python, "../../examples/09_nil", "go_nil")
	setup := "import sys\nsys.path.insert(0," + pythonLiteral(filepath.Join(out, ".build/python")) + ")\nimport go_nil as b\n"
	script := setup + `
import gc
base=b._native._holder_count()
p=b.PtrList.nil()
assert p.is_nil and p.len()==0
assert b._native._holder_count()==base
node=p.push(1).push(2)
assert node.len()==2 and node.head()==2 and node.first()==2
assert not b.find(node,1).is_nil and b.find(node,9).is_nil
v=b.List(value_=3,next=node)
assert v.len()==3 and v.head()==3
view=node.value
view.value_=20
assert node.value_==20
assert b.deref(b.PtrInt.nil())=="nil"
assert b.maybe_int(False).is_nil and b.maybe_int(True).value==42
for value in [p,node]:
    try:bool(value)
    except TypeError:pass
    else:raise AssertionError("pointer truth value accepted")
for call in [lambda:b.find(None,1),lambda:b.List(next=None),lambda:setattr(v,"next",None)]:
    try:call()
    except TypeError:pass
    else:raise AssertionError("None accepted as Go value")
# Go cycles contain Go pointers, never Python references or bridge handles.
node.next=node
assert node.next.value_==20
del value,call,p,node,v,view
gc.collect()
b._native._collect_go()
assert b._native._holder_count()==base
assert b.must_positive(2)==2
`
	if output, err := exec.Command(python, "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("nil behavior: %v\n%s", err, output)
	}
	for _, call := range []string{"b.must_positive(0)", "b.PtrInt.nil().value", "b.PtrList.nil().value", "b.PtrList.nil().value_", "b.PtrList.nil().head()", "b.PtrList.nil().first()", "setattr(b.PtrList.nil(), 'next', b.PtrList.nil())"} {
		t.Run(call, func(t *testing.T) {
			script := setup + "print('before fatal call',flush=True)\ntry:\n    " + call + "\nexcept BaseException:\n    print('caught in Python',flush=True)\nprint('returned',flush=True)\n"
			cmd := exec.Command(python, "-c", script)
			cmd.Env = append(os.Environ(), "GOTRACEBACK=single")
			output, err := cmd.CombinedOutput()
			if _, ok := err.(*exec.ExitError); !ok || !bytes.Contains(output, []byte("before fatal call")) || !bytes.Contains(output, []byte("panic:")) || bytes.Contains(output, []byte("caught in Python")) || bytes.Contains(output, []byte("returned")) {
				t.Fatalf("Go panic was not fatal: %v\n%s", err, output)
			}
		})
	}
}
