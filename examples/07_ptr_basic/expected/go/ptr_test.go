package main

import (
	api "go-bridge/examples/07_ptr_basic"
	"runtime"
	"slices"
	"testing"
)

func TestPtrWritesThrough(t *testing.T) {
	baseline := holderCount()
	id := BrNewPtrInt(1)
	BrIncInt(id)
	BrPtrIntStore(id, BrPtrIntLoad(id)+1)
	if *Get[*int](id) != 3 {
		t.Fatal("writes do not reach the pointed-to variable")
	}
	BrUnPin(id)
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestPtrSliceHeader(t *testing.T) {
	baseline := holderCount()
	numbers := Pin([]int{1})
	initial := Pin(api.MyNamedNonStructSliceInt{0})
	p := BrNewPtrMyNamedNonStructSliceInt(initial)
	BrUnPin(initial)
	// Python only passes named handles, but the ABI reads any []int layout.
	BrPtrMyNamedNonStructSliceIntStore(p, numbers)
	before := BrPtrMyNamedNonStructSliceIntLoad(p)
	BrPtrMyNamedNonStructSliceIntAppend(p, 2)
	after := BrPtrMyNamedNonStructSliceIntLoad(p)
	if len(Get[[]int](before)) != 1 || !slices.Equal(Get[[]int](after), []int{1, 2}) {
		t.Fatal("append through the pointer must replace only *p")
	}
	Get[[]int](numbers)[0] = 10 // The first header still shares the array.
	if Get[api.MyNamedNonStructSliceInt](before)[0] != 10 {
		t.Fatal("loaded header does not share the backing array")
	}
	BrUnPin(p)
	runtime.GC()
	for _, handle := range list(numbers, before, after) {
		BrUnPin(handle)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

// Test files cannot name C types, so handle slices are built by inference.
func list[H any](items ...H) []H { return items }
