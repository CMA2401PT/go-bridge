package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossPackageNameCollision(t *testing.T) {
	source := module(t, `import other "example.test/api/dependency"
type Item int
func Echo(v other.Item) Item {return Item(v)}
`)
	dep := filepath.Join(source, "dependency")
	if err := os.Mkdir(dep, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dep, "dep.go"), "package api\ntype Item int\n")
	out := filepath.Join(t.TempDir(), "generated")
	err := Run(Config{Target: source, Output: out})
	if err == nil || !strings.Contains(err.Error(), "collides") || !strings.Contains(err.Error(), "example.test/api.Item") || !strings.Contains(err.Error(), "example.test/api/dependency.Item") {
		t.Fatalf("missing both type origins: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("name collision published output")
	}
}

func TestIntegrationConsumerTypes(t *testing.T) {
	python := integrationPython(t)
	if os.Getenv("GO_BRIDGE_MYPY") != "1" {
		t.Skip("set GO_BRIDGE_MYPY=1 to check Python consumers")
	}
	source := module(t, `
type Count int
type Counts []Count
func Classmethod() int {return 1}
func Property() int {return 2}
type Node struct { Value Count; Next *Node }
func Echo(n Node, rows []Node, counts Counts) (*Node, []Node, Counts) { return &n, rows, counts }
`)
	out := buildProject(t, python, source, "consumer_bridge")
	consumer := filepath.Join(out, "consumer.py")
	write(t, consumer, `from typing import assert_type
import consumer_bridge as b
n = b.Node(value_=b.Count(2))
n.next = b.PtrNode.nil()
assert_type(n.addr(), b.PtrNode)
assert_type(b.PtrNode.from_value(n).value, b.Node)
assert_type(n.next.is_nil, bool)
rows = b.SliceNode([n])
counts = b.Counts(values=b.SliceCount([b.Count(1)]))
assert_type(rows[0], b.Node)
assert_type(rows[:], b.SliceNode)
assert_type(rows.copy_to_py_list(), list[b.Node])
assert_type(counts.unname(), b.SliceCount)
assert_type(b.echo(n, rows, counts), tuple[b.PtrNode, b.SliceNode, b.Counts])
# Each ignore must suppress an actual error under --strict.
b.Node(value_=1)  # type: ignore[arg-type]
b.Node(next=None)  # type: ignore[arg-type]
b.echo(n, [n], counts)  # type: ignore[arg-type]
b.Counts(b.SliceCount([1]))  # type: ignore[list-item]
b.Counts([b.Count(1)])  # type: ignore[arg-type]
`)
	cmd := exec.Command(python, "-m", "mypy", "--strict", "--cache-dir", filepath.Join(out, ".mypy_cache"), consumer)
	cmd.Dir = out
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("consumer types: %v\n%s", err, output)
	}
}

func TestIntegrationHolderChecks(t *testing.T) {
	integrationPython(t) // Use the same opt-in as other native checks.
	source := module(t, "type Node struct { Value int; Next *Node }; func Echo(n *Node) *Node {return n}")
	out := filepath.Join(t.TempDir(), "generated")
	if err := Run(Config{Target: source, Output: out}); err != nil {
		t.Fatal(err)
	}
	check := filepath.Join(out, "go", "holder_test.go")
	write(t, check, `package main
import ("runtime"; "sync"; "testing")
type item struct { value int; next *item }
func TestRoots(t *testing.T) {
 var workers sync.WaitGroup
 for i:=0;i<8;i++ { workers.Add(1); go func() {
  defer workers.Done()
  for j:=0;j<100;j++ {
   value:=&item{value:j}; value.next=value
   handle:=bridgePin(value)
   copied:=bridgeStruct(*value)
   if bridgeGet[*item](copied).next!=value {t.Error("not a shallow copy")}
   SBRelease(copied)
   view:=bridgePin(&value.value)
   SBRelease(handle)
   value=nil // Only the field-view handle roots the original allocation now.
   if j%25==0 {runtime.GC()}
   if *bridgeGet[*int](view)!=j {t.Error("lost rooted field")}
   SBRelease(view)
  }
 }() }
 workers.Wait()
 SBRelease(0)
 if bridgePointer[*item](0)!=nil {t.Fatal("nil pointer changed")}
 if SBHolderCount()!=0 {t.Fatal("leaked roots")}
}

`)
	cmd := exec.Command("go", "test", "-race", "-gcflags=all=-d=checkptr=2", filepath.Join(out, "go", "main.go"), filepath.Join(out, "go", "holder.go"), check)
	cmd.Dir = source
	cmd.Env = append(os.Environ(), "GOEXPERIMENT=cgocheck2")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("holder race/checkptr/cgocheck2: %v\n%s", err, output)
	}
}

func TestObjectDeterminism(t *testing.T) {
	source := module(t, `type Count int; type Counts []Count; type Node struct { Value Count; Next *Node; Children []Node }; func (n *Node) Set(v Count) {n.Value=v}`)
	first, second := filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")
	for _, out := range []string{first, second} {
		if err := Run(Config{Target: source, Output: out, Name: "objects"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := filepath.WalkDir(first, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
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
		if string(a) != string(b) {
			t.Errorf("nondeterministic %s", rel)
		}
		if strings.HasSuffix(path, ".go") && (strings.Contains(string(a), "recover(") || strings.Contains(string(a), "defer ")) {
			t.Errorf("generated adapter contains panic recovery or deferred cleanup: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationRecursiveObjects(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type Forest []Forest
type Link *Link
type Node struct { Name string; Next *Node; Children []Node }
func Same(n *Node) *Node {return n}
`)
	buildAndRun(t, python, source, "recursive_bridge", `
import gc
import recursive_bridge as b
base=b._native._holder_count()
leaf=b.Forest(b.SliceForest([]))
tree=b.Forest(b.SliceForest([leaf]))
assert len(tree[0])==0
assert type(tree[:]) is b.Forest
nil=b.Link.nil()
outer=b.PtrLink(nil)
link=b.Link(outer)
assert not link.is_nil and link.value.is_nil
assert link.unname().value.is_nil
node=b.Node(name="root")
node.next=node.addr()
node.children=b.SliceNode([b.Node(name="child")])
view=b.same(node.addr()).value
del node
b._native._collect_go()
assert view.next.name=="root" and view.children[0].name=="child"
view.next=b.PtrNode.nil()
del leaf,tree,nil,outer,link,view
gc.collect()
b._native._collect_go()
assert b._native._holder_count()==base
`)
}
