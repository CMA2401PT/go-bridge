package main

/*
#include <stdint.h>
#include <stddef.h>
#include <stdlib.h>

typedef struct {
    char *data;
    size_t size;
} SBString;
*/
import "C"

import (
	"fmt"
	api "go-bridge/examples/09_nil"
	"runtime"
)

// Go string results use the SBString ABI of 03_str; C++ frees data.
func cString(value string) C.SBString {
	return C.SBString{data: C.CString(value), size: C.size_t(len(value))}
}

// Every export that runs G code takes a panic out-parameter as its last
// argument. guard recovers a panic, including a nil dereference, and hands
// its message to C++, which raises GoPanic; the function then returns its
// zero value, which C++ ignores. A panic must never unwind into C.
func guard(panicked *C.SBString) {
	if r := recover(); r != nil {
		*panicked = cString(fmt.Sprint(r))
	}
}

// ---------------------------------------------------------------------------
// *int. Handle 0 is nil: Get returns a nil *int, and Go's own dereference
// panics where Go would.

//export SBNewPtrInt
func SBNewPtrInt(value C.intptr_t) C.uintptr_t {
	p := new(int)
	*p = int(value)
	return Pin(p)
}

//export SBPtrIntLoad
func SBPtrIntLoad(id C.uintptr_t, panicked *C.SBString) C.intptr_t {
	defer guard(panicked)
	return C.intptr_t(*Get[*int](id))
}

//export SBPtrIntStore
func SBPtrIntStore(id C.uintptr_t, value C.intptr_t, panicked *C.SBString) {
	defer guard(panicked)
	*Get[*int](id) = int(value)
}

// ---------------------------------------------------------------------------
// List and *List, as in 06: both wrappers pin a *List and share the field and
// method exports. A List is a variable and never nil; handle 0 is a nil
// *List, which only PtrList can hold.

// A 0 next handle is a nil Next, which is also the zero value.
//
//export SBNewList
func SBNewList(value C.intptr_t, next C.uintptr_t) C.uintptr_t {
	return Pin(&api.List{Value: int(value), Next: Get[*api.List](next)})
}

//export SBListGetValue
func SBListGetValue(id C.uintptr_t, panicked *C.SBString) C.intptr_t {
	defer guard(panicked)
	return C.intptr_t(Get[*api.List](id).Value)
}

//export SBListSetValue
func SBListSetValue(id C.uintptr_t, value C.intptr_t, panicked *C.SBString) {
	defer guard(panicked)
	Get[*api.List](id).Value = int(value)
}

// A pointer field is read as the pointer it holds, not as a view of the
// field: the list it points to is shared, and nil is handle 0.
//
//export SBListGetNext
func SBListGetNext(id C.uintptr_t, panicked *C.SBString) C.uintptr_t {
	defer guard(panicked)
	return PinPointer(Get[*api.List](id).Next)
}

// Storing a pointer field copies the pointer, so the lists are shared.
//
//export SBListSetNext
func SBListSetNext(id, next C.uintptr_t, panicked *C.SBString) {
	defer guard(panicked)
	Get[*api.List](id).Next = Get[*api.List](next)
}

// Go's p := new(List); *p = v. The source is a List, never nil.
//
//export SBListCopy
func SBListCopy(id C.uintptr_t) C.uintptr_t {
	value := *Get[*api.List](id)
	return Pin(&value)
}

// Go's &v: shares the variable.
//
//export SBListAddr
func SBListAddr(id C.uintptr_t) C.uintptr_t { return Pin(Get[*api.List](id)) }

// Go's *p, read as the variable it names; dereferencing nil panics.
//
//export SBPtrListLoad
func SBPtrListLoad(id C.uintptr_t, panicked *C.SBString) C.uintptr_t {
	defer guard(panicked)
	p := Get[*api.List](id)
	_ = *p
	return Pin(p)
}

// Go's *p = v: the struct is copied in.
//
//export SBPtrListStore
func SBPtrListStore(id, value C.uintptr_t, panicked *C.SBString) {
	defer guard(panicked)
	*Get[*api.List](id) = *Get[*api.List](value)
}

// Pointer receivers run on a nil *List; the method decides what nil means.
//
//export SBListLen
func SBListLen(id C.uintptr_t, panicked *C.SBString) C.intptr_t {
	defer guard(panicked)
	return C.intptr_t(Get[*api.List](id).Len())
}

//export SBListPush
func SBListPush(id C.uintptr_t, v C.intptr_t, panicked *C.SBString) C.uintptr_t {
	defer guard(panicked)
	return PinPointer(Get[*api.List](id).Push(int(v)))
}

//export SBListFirst
func SBListFirst(id C.uintptr_t, panicked *C.SBString) C.intptr_t {
	defer guard(panicked)
	return C.intptr_t(Get[*api.List](id).First())
}

// A value receiver needs *l first, which panics on nil as in Go.
//
//export SBListHead
func SBListHead(id C.uintptr_t, panicked *C.SBString) C.intptr_t {
	defer guard(panicked)
	return C.intptr_t((*Get[*api.List](id)).Head())
}

// ---------------------------------------------------------------------------
// Functions.

//export SBFind
func SBFind(l C.uintptr_t, v C.intptr_t, panicked *C.SBString) C.uintptr_t {
	defer guard(panicked)
	return PinPointer(api.Find(Get[*api.List](l), int(v)))
}

//export SBDeref
func SBDeref(p C.uintptr_t, panicked *C.SBString) C.SBString {
	defer guard(panicked)
	return cString(api.Deref(Get[*int](p)))
}

// Go bool crosses as uint8_t, as in 01_basic.
//
//export SBMaybeInt
func SBMaybeInt(ok C.uint8_t, panicked *C.SBString) C.uintptr_t {
	defer guard(panicked)
	return PinPointer(api.MaybeInt(ok != 0))
}

//export SBMustPositive
func SBMustPositive(v C.intptr_t, panicked *C.SBString) C.intptr_t {
	defer guard(panicked)
	return C.intptr_t(api.MustPositive(int(v)))
}

// ---------------------------------------------------------------------------
// Lifecycle.

//export SBUnPin
func SBUnPin(id C.uintptr_t) { UnPin(id) }

// Internal test diagnostics.
//
//export SBHolderCount
func SBHolderCount() C.size_t { return C.size_t(holderCount()) }

//export SBCollect
func SBCollect() { runtime.GC() }

func main() {}
