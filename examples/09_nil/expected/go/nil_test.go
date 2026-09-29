package main

import (
	api "go-bridge/examples/09_nil"
	"testing"
	"unsafe"
)

// Test files cannot write C.BrString; _Ctype_BrString is cgo's internal name
// for it, visible within the package but undocumented, acceptable in a test.
// BrString is {char *data; size_t size}; the C copy is left unfreed.
func panicOf[S any](call func(*S)) string {
	var out S
	call(&out)
	data := *(*unsafe.Pointer)(unsafe.Pointer(&out))
	if data == nil {
		return ""
	}
	size := *(*uintptr)(unsafe.Add(unsafe.Pointer(&out), unsafe.Sizeof(data)))
	return unsafe.String((*byte)(data), size)
}

func TestZeroHandleIsNil(t *testing.T) {
	if Get[*api.List](0) != nil || Get[*int](0) != nil {
		t.Fatal("handle 0 must read as nil")
	}
	if PinPointer[int](nil) != 0 {
		t.Fatal("a nil result must not be pinned")
	}
	UnPin(0) // Releasing nil is a no-op.
}

func TestPanicIsRecovered(t *testing.T) {
	baseline := holderCount()
	if got := panicOf(func(p *_Ctype_BrString) { BrMustPositive(-1, p) }); got != "not positive: -1" {
		t.Fatalf("panic = %q", got)
	}
	if panicOf(func(p *_Ctype_BrString) { BrListFirst(0, p) }) == "" {
		t.Fatal("nil dereference inside Go was not recovered")
	}
	if got := panicOf(func(p *_Ctype_BrString) {
		if BrListLen(0, p) != 0 {
			t.Error("nil receiver must run the method")
		}
	}); got != "" {
		t.Fatalf("unexpected panic %q", got)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}
