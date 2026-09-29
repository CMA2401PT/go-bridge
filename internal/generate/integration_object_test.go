package generate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIntegrationNamedAndStruct(t *testing.T) {
	python := integrationPython(t)
	buildAndRun(t, python, "../../examples/06_named_type", "go_named", `
import copy,gc
import go_named as b
base=b._native._holder_count()
n=b.MyNamedNonStructInt(5)
assert isinstance(n,int) and n.unname()==5
assert n.echo_non_struct_int() == "my non struct int: 5"
assert b.fn_echo_non_struct_int(n) == "my non struct int: 5"
try: b.fn_echo_non_struct_int(5)
except TypeError: pass
else: raise AssertionError("plain integer accepted")
try: type("Sub",(b.MyNamedNonStructInt,),{})
except TypeError: pass
else: raise AssertionError("named scalar subclass accepted")
s=b.SliceInt([1,2])
named=b.MyNamedNonStructSliceInt(values=s)
assert not isinstance(named,b.SliceInt)
assert named.echo_non_struct_slice_int() == "my non struct slice int: [1 2]"
assert type(named[:]) is b.MyNamedNonStructSliceInt
named[0]=9
assert s[0]==9 and named.unname()[0]==9
v=b.MyNamedStruct(basic_member_value=1,named_basic_member_value=n,ref_member_value=s,named_ref_member_value=b.MyNamedSubStruct(basic_member_value=6))
assert v.basic_member_value2 == 0
v.basic_member_value2=8
assert v.my_named_anomy_sub_struct.basic_member_value2==8
view=v.named_ref_member_value
view.basic_member_value=60
assert v.named_ref_member_value.basic_member_value==60
v.named_ref_member_value=b.MyNamedSubStruct(basic_member_value=7)
assert view.basic_member_value==7
v.inc_basic_ptr()
w=v.inc_basic_non_ptr()
assert v.basic_member_value==2 and w.basic_member_value==3
c=copy.copy(v)
c.basic_member_value=8
assert v.basic_member_value==2
c.ref_member_value[1]=20
assert v.ref_member_value[1]==20
p=v.addr()
q=b.PtrMyNamedStruct(v)
p.basic_member_value=40
assert v.basic_member_value==40 and q.basic_member_value==2
q.value=v
assert q.basic_member_value==40
p.value.basic_member_value=42
assert v.basic_member_value==42
assert p.echo_ptr()==v.echo_ptr()
for action in [lambda:b.fn_echo_non_ptr(p),lambda:b.ptr_fn_echo_ptr(v),lambda:b.MyNamedStruct(basic_member_value="bad"),lambda:b.MyNamedStruct(unknown=1),lambda:b.MyNamedStruct(ref_member_value=None)]:
    try: action()
    except TypeError: pass
    else: raise AssertionError("invalid struct input accepted")
del action,v,p
b._native._collect_go()
assert view.basic_member_value==7
del view,w,c,q,s,named
gc.collect()
assert b._native._holder_count()==base
`)
}

func TestIntegrationObjectComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `import "example.test/api/dependency"
func Classmethod() int {return 1}
func Property() int {return 2}
type Alias = dependency.Counter
type Label string
func (v Label) Join(other Label) Label {return v+other}
type Rate float32
func EchoRate(v Rate) Rate {return v}
type Inner struct { N int }
type Outer struct { dependency.Data; Title Label; Ratio Rate; Items []dependency.Counter; Self int; Cls int }
type DataRef *dependency.Data
func Make() Outer {return Outer{Data:dependency.Make()}}
func Echo(v Alias) Alias {return v}
func Rows() []Inner {return []Inner{{N:1},{N:2}}}
func Bad() (Outer,Label,Outer) {return Make(),Label("\xff"),Make()}
`)
	dep := filepath.Join(source, "dependency")
	if err := os.Mkdir(dep, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dep, "dep.go"), `package dependency
type Counter int
type hidden struct { Number Counter }
func (h hidden) Read() Counter {return h.Number}
type Data struct { hidden }
func Make() Data {return Data{hidden{3}}}
`)
	buildAndRun(t, python, source, "go_objects", `
import gc
import go_objects as b
base=b._native._holder_count()
assert b.classmethod()==1 and b.property()==2
assert b.Outer(self=1,cls=2).self==1 and b.Outer(self=1,cls=2).cls==2
assert b.Alias is b.Counter
assert b.echo(b.Counter(5))==5
text=b.Label("中文\0")
assert isinstance(text,str) and type(text.join_(b.Label("尾"))) is b.Label
assert text.join_(b.Label("尾")).unname()=="中文\0尾"
assert type(b.echo_rate(b.Rate(1.5))) is b.Rate
value=b.make()
assert value.number==3 and value.read()==3
value.number=b.Counter(9)
assert value.data.number==9
value.title=text
value.ratio=b.Rate(2.5)
value.items=b.SliceCounter([b.Counter(1),b.Counter(2)])
assert type(value.items[0]) is b.Counter
assert value.items.copy_to_py_list()==[1,2]
ref=b.DataRef(value.data.addr())
ref.number=b.Counter(12)
assert value.number==12
ref.value.number=b.Counter(13)
assert value.number==13 and ref.value.read()==13
assert not hasattr(ref,"read")
rows=b.rows()
rows[0].n=4
assert rows[0].n==4
copies=rows.copy_to_py_list()
copies[0].n=8
assert rows[0].n==4
view=rows[1]
del rows
b._native._collect_go()
assert view.n==2
before=b._native._holder_count()
for _ in range(30):
    try:b.bad()
    except UnicodeDecodeError:pass
    else:raise AssertionError("invalid named string accepted")
gc.collect()
assert b._native._holder_count()==before
del value,ref,copies,view
gc.collect()
assert b._native._holder_count()==base
`)
}

func TestIntegrationOnlyNamedScalars(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `type Count int; type Text string; type Ratio float64`)
	buildAndRun(t, python, source, "named_only", `
import named_only as b
assert b.Count(3).unname()==3
assert b.Text("x").unname()=="x"
assert b.Ratio(1.5).unname()==1.5
assert not hasattr(b,"Ref")
`)
}
