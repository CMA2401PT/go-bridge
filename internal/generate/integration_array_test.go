package generate

import "testing"

func TestIntegrationArrayExample(t *testing.T) { runExample(t, "13_array", "go_array") }
func TestIntegrationArrayComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type A [2]int
func (v *A) Add() { v[0]++ }
type Box struct { Values A; Nested [2][2]int }
type Item struct { N int }
func Arrays(v [2]Item) [2]Item {return v}
func Slices(v []A) []A {return v}
func Maps(v map[A]A) map[A]A {return v}
func Channels(v chan A) chan A {return v}
func Any(v any) any {return v}
func Empty(v [0]int) [0]int {return v}
func Text(v [2]string) [2]string {return v}
func Broken() [2]string {return [2]string{"ok",string([]byte{255})}}
func Refs(v [2][]int) [2][]int {return v}
`)
	buildAndRun(t, python, source, "go_arrays", `
import copy,gc
import go_arrays as b
base=b._native._holder_count()
def check():
 a=b.A(b.Array2Int([1,2]));a.add();assert a[0]==2
 raw=a.unname();raw[0]=8;assert a[0]==2
 box=b.Box(values=a);a[0]=9;assert box.values[0]==2
 view=box.values;view[1]=7;assert box.values[1]==7
 ptr=view.addr();ptr.value[0]=6;assert box.values[0]==6
 ptr.value=a;assert view[0]==9
 nested=box.nested[0];nested[1]=5;assert box.nested[0][1]==5
 del box;gc.collect();nested[0]=3;assert nested[1]==5
 sl=b.SliceA([a]);sl[0][0]=4;assert a[0]==9
 arr=b.Array2Item([b.Item(n=1),b.Item(n=2)]);arr[0].n=5
 copied=b.arrays(arr);copied[0].n=7;assert arr[0].n==5
 listed=arr.copy_to_py_list();listed[0].n=8;assert arr[0].n==5
 m=b.MapAA([(a,a)]);a[0]=1;key=b.A(b.Array2Int([9,2]));assert m[key][0]==9
 m[key][0]=99;assert m[key][0]==9
 ch=b.ChanA(1);ch.send(a);a[0]=2;values=[];ch.recv(values.append);assert values[0][0]==1
 boxed=a.as_any();a[0]=3;assert b.A.from_any(boxed)[0]==2
 assert len(b.empty(b.Array0Int()))==0
 refs=b.Array2SliceInt([b.SliceInt([1]),b.SliceInt([2])]);copy.copy(refs)[0][0]=3;assert refs[0][0]==3
 before=b._native._holder_count()
 for _ in range(3):
  try:b.broken().copy_to_py_list()
  except UnicodeDecodeError:pass
  else:raise AssertionError("bad text accepted")
 assert b._native._holder_count()==before
check();gc.collect();b._native._collect_go();assert b._native._holder_count()==base
`)
}
