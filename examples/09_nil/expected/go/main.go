package main

/*
#include <stdint.h>
#include <stddef.h>
#include <stdlib.h>

typedef struct {
    char *data;
    size_t size;
} BrString;
*/
import "C"

import (
	api "go-bridge/examples/09_nil"
	"runtime"
)

// Go string results use the BrString ABI of 03_str; C++ frees data.
func cString(value string) C.BrString {
	return C.BrString{data: C.CString(value), size: C.size_t(len(value))}
}

// ---------------------------------------------------------------------------
// *int. Handle 0 is nil: Get returns a nil *int, and Go's own dereference
// panics where Go would.

//export BrNewPtrInt
func BrNewPtrInt(value C.intptr_t) C.uintptr_t {
	p := new(int)
	*p = int(value)
	return Pin(p)
}

//export BrPtrIntLoad
func BrPtrIntLoad(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(*Get[*int](id))
}

//export BrPtrIntStore
func BrPtrIntStore(id C.uintptr_t, value C.intptr_t) {
	*Get[*int](id) = int(value)
}

// ---------------------------------------------------------------------------
// List and *List, as in 06: both wrappers pin a *List and share the field and
// method exports. A List is a variable and never nil; handle 0 is a nil
// *List, which only PtrList can hold.

// A 0 next handle is a nil Next, which is also the zero value.
//
//export BrNewList
func BrNewList(value C.intptr_t, next C.uintptr_t) C.uintptr_t {
	return Pin(&api.List{Value: int(value), Next: Get[*api.List](next)})
}

//export BrListGetValue
func BrListGetValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.List](id).Value)
}

//export BrListSetValue
func BrListSetValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.List](id).Value = int(value)
}

// A pointer field is read as the pointer it holds, not as a view of the
// field: the list it points to is shared, and nil is handle 0.
//
//export BrListGetNext
func BrListGetNext(id C.uintptr_t) C.uintptr_t {
	return PinPointer(Get[*api.List](id).Next)
}

// Storing a pointer field copies the pointer, so the lists are shared.
//
//export BrListSetNext
func BrListSetNext(id, next C.uintptr_t) {
	Get[*api.List](id).Next = Get[*api.List](next)
}

// Go's p := new(List); *p = v. The source is a List, never nil.
//
//export BrListCopy
func BrListCopy(id C.uintptr_t) C.uintptr_t {
	value := *Get[*api.List](id)
	return Pin(&value)
}

// Go's &v: shares the variable.
//
//export BrListAddr
func BrListAddr(id C.uintptr_t) C.uintptr_t { return Pin(Get[*api.List](id)) }

// Go's *p, read as the variable it names; dereferencing nil panics.
//
//export BrPtrListLoad
func BrPtrListLoad(id C.uintptr_t) C.uintptr_t {
	p := Get[*api.List](id)
	_ = *p
	return Pin(p)
}

// Go's *p = v: the struct is copied in.
//
//export BrPtrListStore
func BrPtrListStore(id, value C.uintptr_t) {
	*Get[*api.List](id) = *Get[*api.List](value)
}

// Pointer receivers run on a nil *List; the method decides what nil means.
//
//export BrListLen
func BrListLen(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.List](id).Len())
}

//export BrListPush
func BrListPush(id C.uintptr_t, v C.intptr_t) C.uintptr_t {
	return PinPointer(Get[*api.List](id).Push(int(v)))
}

//export BrListFirst
func BrListFirst(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.List](id).First())
}

// A value receiver needs *l first, which panics on nil as in Go.
//
//export BrListHead
func BrListHead(id C.uintptr_t) C.intptr_t {
	return C.intptr_t((*Get[*api.List](id)).Head())
}

// ---------------------------------------------------------------------------
// Functions.

//export BrFind
func BrFind(l C.uintptr_t, v C.intptr_t) C.uintptr_t {
	return PinPointer(api.Find(Get[*api.List](l), int(v)))
}

//export BrDeref
func BrDeref(p C.uintptr_t) C.BrString {
	return cString(api.Deref(Get[*int](p)))
}

// Go bool crosses as uint8_t, as in 01_basic.
//
//export BrMaybeInt
func BrMaybeInt(ok C.uint8_t) C.uintptr_t {
	return PinPointer(api.MaybeInt(ok != 0))
}

//export BrMustPositive
func BrMustPositive(v C.intptr_t) C.intptr_t {
	return C.intptr_t(api.MustPositive(int(v)))
}

// ---------------------------------------------------------------------------
// Lifecycle.

//export BrUnPin
func BrUnPin(id C.uintptr_t) { UnPin(id) }

// Internal test diagnostics.
//
//export BrHolderCount
func BrHolderCount() C.size_t { return C.size_t(holderCount()) }

//export BrCollect
func BrCollect() { runtime.GC() }

func main() {}
