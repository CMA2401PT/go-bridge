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

func TestIntegrationChannelExample(t *testing.T) {
	python := integrationPython(t)
	out := buildProject(t, python, "../../examples/11_channel", "go_channel")
	source, err := os.ReadFile("../../examples/11_channel/test.py")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.Replace(string(source), `package_dir = example_dir.parent / ".build" / example_dir.name / "python"`, "package_dir = Path("+pythonLiteral(filepath.Join(out, ".build/python"))+")", 1)
	check := filepath.Join(out, "check.py")
	write(t, check, script)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, python, check).CombinedOutput(); err != nil {
		t.Fatalf("channel example: %v\n%s", err, output)
	}
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		cmd := exec.Command(python, "-m", "mypy", "--strict", check)
		cmd.Env = append(os.Environ(), "MYPYPATH="+out)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("channel consumer types: %v\n%s", err, output)
		}
	}
}

func TestIntegrationChannelComposition(t *testing.T) {
	python := integrationPython(t)
	source := module(t, `
type Count int
type Stream chan Count
type Reader <-chan Count
type Writer chan<- Count
type Box struct { N int; Stream Stream }
type Recursive chan Recursive
func (s Stream) Read() Count { return <-s }
func (s Stream) Send() int { return 42 }
func (s *Stream) Replace(value Stream) { *s = value }
func Echo(ch chan int) chan int { return ch }
func EchoAny(value any) any { return value }
func EchoPointer(value *chan int) *chan int { return value }
func EchoSlice(value []chan int) []chan int { return value }
func EchoStruct(value chan Box) chan Box { return value }
func EchoStructPointer(value chan *Box) chan *Box { return value }
func EchoValues(value chan []int) chan []int { return value }
func EchoInterface(value chan any) chan any { return value }
func EchoText(value chan string) chan string { return value }
func EchoBool(value chan bool) chan bool { return value }
func EchoFloat(value chan float32) chan float32 { return value }
func InvalidText() <-chan string {
    ch := make(chan string, 1); ch <- string([]byte{255}); close(ch); return ch
}
`)
	out := buildProject(t, python, source, "go_channel_composition")
	script := `
import gc
import threading
import go_channel_composition as b

def check() -> None:
    ch = b.ChanInt(capacity=2)
    p = b.PtrChanInt(ch)
    b.echo_pointer(value=p).value.send(value=1)
    assert b.echo_slice(value=b.SliceChanInt([ch]))[0].recv() == (1, True)
    nil = b.ChanInt.nil()
    assert b.ChanInt.from_any(nil.as_any()).is_nil
    assert not nil.as_any().is_nil
    assert b.RecvChanInt.try_from_any(ch.as_any()) is None
    b.ChanInt.from_any(ch.as_any()).send(value=3)
    assert ch.recv() == (3, True)
    stream = b.Stream(b.ChanCount(capacity=1))
    assert stream.send_() == 42
    stream.send(value=b.Count(8))
    count, ok = b.Reader(stream.recv_only()).recv()
    assert ok and type(count) is b.Count and count == 8
    b.Writer(stream.send_only()).send(value=b.Count(9))
    assert stream.unname().recv() == (9, True)
    assert b.Stream.try_from_any(stream.unname().as_any()) is None
    assert b.Stream.from_any(stream.as_any()).recv_only().is_nil is False
    assert b.Reader.nil().unname().is_nil
    pointer = stream.ptr()
    replacement = b.Stream(b.ChanCount(capacity=1))
    pointer.replace(value=replacement)
    pointer.value.send(value=b.Count(11))
    assert replacement.read() == 11
    # A Go method blocked on receive must allow a Python sender to run.
    worker = threading.Thread(target=lambda: replacement.send(value=b.Count(12)))
    worker.start()
    assert replacement.read() == 12
    worker.join()
    box = b.Box(n=5, stream=stream)
    assert not box.stream.is_nil and b.Box().stream.is_nil
    objects = b.ChanBox(capacity=1)
    objects.send(value=box)
    box.n = 6
    received_box, ok = b.echo_struct(value=objects).recv()
    assert ok and received_box.n == 5  # Send and receive copy struct values.
    objects.close()
    zero_box, ok = objects.recv()
    assert not ok and type(zero_box) is b.Box
    assert zero_box.n == 0 and zero_box.stream.is_nil
    pointers = b.ChanPtrBox(capacity=1)
    pointers.send(value=box.addr())
    ref, ok = pointers.recv()
    assert ok
    ref.n = 7
    assert box.n == 7
    pointers.send(value=b.PtrBox.nil())
    pointers.close()
    ref, ok = pointers.recv()
    assert ok and ref.is_nil
    ref, ok = pointers.recv()
    assert not ok and type(ref) is b.PtrBox and ref.is_nil
    slices = b.ChanSliceInt(capacity=1)
    original = b.SliceInt([1])
    slices.send(value=original)
    received_slice, ok = slices.recv()
    assert ok
    received_slice[0] = 2
    assert original[0] == 2
    slices.close()
    zero_slice, ok = slices.recv()
    assert not ok and type(zero_slice) is b.SliceInt and len(zero_slice) == 0
    interfaces = b.ChanGoAny(capacity=1)
    interfaces.send(value=b.GoAny.nil())
    any_value, ok = interfaces.recv()
    assert ok and any_value.is_nil
    interfaces.close()
    any_value, ok = interfaces.recv()
    assert not ok and type(any_value) is b.GoAny and any_value.is_nil
    recursive = b.Recursive(b.ChanRecursive(capacity=1))
    recursive.send(value=recursive)
    rec, ok = recursive.recv()
    assert ok
    rec.send(value=b.Recursive.nil())
    rec.close()
    rec, ok = recursive.recv()
    assert ok and rec.is_nil
    rec, ok = recursive.recv()
    assert not ok and type(rec) is b.Recursive and rec.is_nil
    flags = b.ChanBool(capacity=1)
    flags.send(value=False)
    assert flags.recv() == (False, True)
    flags.close()
    assert flags.recv() == (False, False)
    floats = b.ChanFloat32(capacity=1)
    floats.send(value=1.25)
    assert floats.recv() == (1.25, True)
    floats.send(value=0.0)
    floats.close()
    assert floats.recv() == (0.0, True)
    assert floats.recv() == (0.0, False)
    stream.send(value=b.Count(0))
    stream.close()
    count, ok = stream.recv()
    assert ok and type(count) is b.Count and count == 0
    count, ok = stream.recv()
    assert not ok and type(count) is b.Count and count == 0
    invalid = b.invalid_text()
    try:
        invalid.recv()
    except UnicodeDecodeError:
        pass
    else:
        raise AssertionError("invalid UTF-8 accepted")
    assert invalid.recv() == ("", False)
    # Wrong directions and elements fail before receiving.
    for call in [lambda: b.echo(ch=ch.recv_only()), lambda: ch.send(value="bad")]:
        try:
            call()
        except TypeError:
            pass
        else:
            raise AssertionError("wrong input accepted")

base = b._native._holder_count()
check()
gc.collect()
b._native._collect_go()
assert b._native._holder_count() == base
`
	check := filepath.Join(out, "check.py")
	write(t, check, "import sys\nsys.path.insert(0,"+pythonLiteral(filepath.Join(out, ".build/python"))+")\n"+script)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, python, check).CombinedOutput(); err != nil {
		t.Fatalf("channel composition: %v\n%s", err, output)
	}
	setup := "import sys\nsys.path.insert(0," + pythonLiteral(filepath.Join(out, ".build/python")) + ")\nimport go_channel_composition as b\n"
	for _, call := range []string{
		"ch = b.ChanInt(1); ch.close(); ch.send(1)",
		"ch = b.ChanInt(); ch.close(); ch.close()",
		"b.ChanInt.nil().close()",
		"b.ChanInt(-1)",
	} {
		cmd := exec.CommandContext(ctx, python, "-c", setup+"print('before fatal call', flush=True)\n"+call)
		cmd.Env = append(os.Environ(), "GOTRACEBACK=single")
		output, err := cmd.CombinedOutput()
		if _, ok := err.(*exec.ExitError); !ok || !strings.Contains(string(output), "before fatal call") || !strings.Contains(string(output), "panic:") {
			t.Fatalf("channel panic was not fatal: %v\n%s", err, output)
		}
	}
	if os.Getenv("GO_BRIDGE_MYPY") == "1" {
		consumer := filepath.Join(out, "consumer.py")
		write(t, consumer, `from typing import assert_type
import go_channel_composition as b
c = b.ChanInt(capacity=1)
assert_type(c.recv(), tuple[int, bool])
assert_type(c.recv_only().recv(), tuple[int, bool])
assert_type(b.Stream(b.ChanCount()).recv(), tuple[b.Count, bool])
assert_type(b.ChanBox().recv(), tuple[b.Box, bool])
assert_type(b.ChanPtrBox().recv(), tuple[b.PtrBox, bool])
assert_type(b.ChanString().recv(), tuple[str, bool])
assert_type(c.send_only(), b.SendChanInt)
assert_type(c.recv_only(), b.RecvChanInt)
assert_type(b.Stream(b.ChanCount()).unname(), b.ChanCount)
assert_type(b.Box().stream, b.Stream)
c.recv_only().send(1)  # type: ignore[attr-defined]
c.send_only().recv()  # type: ignore[attr-defined]
b.echo(ch=c.recv_only())  # type: ignore[arg-type]
`)
		cmd := exec.Command(python, "-m", "mypy", "--strict", consumer)
		cmd.Dir = out
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("channel consumer types: %v\n%s", err, output)
		}
	}
}
