package main

import (
	api "go-bridge/examples/08_multi_ptr"
	"runtime"
	"testing"
	"unsafe"
)

// Test files cannot name C types, so handle slices are built by inference.
func list[H any](items ...H) []H { return items }

func TestLevelsShareTargets(t *testing.T) {
	baseline := holderCount()
	p := SBNewPtrInt(1)
	pp := SBNewPtrPtrInt(p)
	ppp := SBNewPtrPtrPtrInt(pp)
	if *Get[**int](pp) != Get[*int](p) || *Get[***int](ppp) != Get[**int](pp) {
		t.Fatal("a pointer level does not hold a copy of the given pointer")
	}
	loaded := SBPtrPtrPtrIntLoad(ppp)
	if loaded == pp || Get[**int](loaded) != Get[**int](pp) {
		t.Fatal("Load must pin a new handle to the same variable")
	}
	SBUnPin(p)
	SBUnPin(pp)
	runtime.GC()
	SBIncPPP(ppp)
	if ***Get[***int](ppp) != 2 {
		t.Fatal("inner variables lost after their Refs were released")
	}
	SBClear(loaded)
	if SBPtrPtrIntLoad(loaded) != 0 {
		t.Fatal("a nil level must load as 0")
	}
	for _, handle := range list(ppp, loaded) {
		SBUnPin(handle)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestNamedPointerAndStructPointer(t *testing.T) {
	baseline := holderCount()
	p := SBNewPtrInt(1)
	named := SBNewIntPtr(p)
	back := SBIntPtrUnname(named)
	if unsafe.Pointer(Get[api.IntPtr](named)) != unsafe.Pointer(Get[*int](back)) {
		t.Fatal("named pointer conversions must share the variable")
	}
	box := SBNewBox(1, 0)
	pp := SBNewPtrPtrBox(box)
	if *Get[**api.Box](pp) != Get[*api.Box](box) {
		t.Fatal("**Box must hold the same *Box")
	}
	SBIncBoxPP(pp)
	if Get[*api.Box](box).Count != 2 {
		t.Fatal("struct not shared through **Box")
	}
	for _, handle := range list(p, named, back, box, pp) {
		SBUnPin(handle)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}
