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
	api "go-bridge/examples/07_ptr_basic"
	"runtime"
	"unsafe"
)

// Go string results use the BrString ABI of 03_str; C++ frees data.
func cString(value string) C.BrString {
	return C.BrString{data: C.CString(value), size: C.size_t(len(value))}
}

// newPtr is Go's p := new(T); *p = value. Every Python pointer constructor
// allocates a new variable; Python values have no Go address to take.
func newPtr[T any](value T) C.uintptr_t {
	p := new(T)
	*p = value
	return Pin(p)
}

// ---------------------------------------------------------------------------
// MyNamedNonStructInt, as in 06_named_type: passed by value as intptr_t.
// Only value-receiver methods belong to its method set; its ptr() is a
// Python-side call to BrNewPtrMyNamedNonStructInt.

//export BrMyNamedNonStructIntEchoNonStructInt
func BrMyNamedNonStructIntEchoNonStructInt(value C.intptr_t) C.BrString {
	return cString(api.MyNamedNonStructInt(value).EchoNonStructInt())
}

// ---------------------------------------------------------------------------
// *int. The handle's cell holds the pointer and never changes; Load and
// Store read and write the target variable *p.

//export BrNewPtrInt
func BrNewPtrInt(value C.intptr_t) C.uintptr_t { return newPtr(int(value)) }

//export BrPtrIntLoad
func BrPtrIntLoad(id C.uintptr_t) C.intptr_t { return C.intptr_t(*Get[*int](id)) }

//export BrPtrIntStore
func BrPtrIntStore(id C.uintptr_t, value C.intptr_t) { *Get[*int](id) = int(value) }

// ---------------------------------------------------------------------------
// *MyNamedNonStructInt. It is a type distinct from *int, so it has its own
// exports; Get always reads the exact type that was pinned.

//export BrNewPtrMyNamedNonStructInt
func BrNewPtrMyNamedNonStructInt(value C.intptr_t) C.uintptr_t {
	return newPtr(api.MyNamedNonStructInt(value))
}

//export BrPtrMyNamedNonStructIntLoad
func BrPtrMyNamedNonStructIntLoad(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(*Get[*api.MyNamedNonStructInt](id))
}

//export BrPtrMyNamedNonStructIntStore
func BrPtrMyNamedNonStructIntStore(id C.uintptr_t, value C.intptr_t) {
	*Get[*api.MyNamedNonStructInt](id) = api.MyNamedNonStructInt(value)
}

// The method set of *T includes the value-receiver methods of T: (*p).M().
//
//export BrPtrMyNamedNonStructIntEchoNonStructInt
func BrPtrMyNamedNonStructIntEchoNonStructInt(id C.uintptr_t) C.BrString {
	return cString(Get[*api.MyNamedNonStructInt](id).EchoNonStructInt())
}

//export BrPtrMyNamedNonStructIntPtrEchoNonStructInt
func BrPtrMyNamedNonStructIntPtrEchoNonStructInt(id C.uintptr_t) C.BrString {
	return cString(Get[*api.MyNamedNonStructInt](id).PtrEchoNonStructInt())
}

//export BrPtrMyNamedNonStructIntInc
func BrPtrMyNamedNonStructIntInc(id C.uintptr_t) { Get[*api.MyNamedNonStructInt](id).Inc() }

// ---------------------------------------------------------------------------
// []int, reduced to what this example needs from 05_slice: construction,
// length and a bulk copy. MyNamedNonStructSliceInt has the same layout and
// reuses these exports.

//export BrNewSliceInt
func BrNewSliceInt(data unsafe.Pointer, size C.size_t) C.uintptr_t {
	value := make([]int, int(size))
	copy(value, unsafe.Slice((*int)(data), int(size)))
	return Pin(value)
}

//export BrSliceIntLen
func BrSliceIntLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[]int](id)))
}

//export BrCopySliceInt
func BrCopySliceInt(id C.uintptr_t, data unsafe.Pointer) {
	value := Get[[]int](id)
	copy(unsafe.Slice((*int)(data), len(value)), value)
}

// Go's MyNamedNonStructSliceInt(s) and []int(si), as in 06.
//
//export BrNewMyNamedNonStructSliceInt
func BrNewMyNamedNonStructSliceInt(id C.uintptr_t) C.uintptr_t {
	return Pin(api.MyNamedNonStructSliceInt(Get[[]int](id)))
}

//export BrMyNamedNonStructSliceIntUnname
func BrMyNamedNonStructSliceIntUnname(id C.uintptr_t) C.uintptr_t {
	return Pin([]int(Get[api.MyNamedNonStructSliceInt](id)))
}

//export BrMyNamedNonStructSliceIntEchoNonStructSliceInt
func BrMyNamedNonStructSliceIntEchoNonStructSliceInt(id C.uintptr_t) C.BrString {
	return cString(Get[api.MyNamedNonStructSliceInt](id).EchoNonStructSliceInt())
}

// ---------------------------------------------------------------------------
// *[]int. *p is a slice header: Load pins a copy of the header (sharing the
// backing array, like Go's s := *p) and Store replaces it (*p = s). A
// header change made through the pointer, e.g. append, is therefore visible
// through p.value but not in a SliceInt loaded earlier.

//export BrNewPtrSliceInt
func BrNewPtrSliceInt(value C.uintptr_t) C.uintptr_t { return newPtr(Get[[]int](value)) }

//export BrPtrSliceIntLoad
func BrPtrSliceIntLoad(id C.uintptr_t) C.uintptr_t { return Pin(*Get[*[]int](id)) }

//export BrPtrSliceIntStore
func BrPtrSliceIntStore(id, value C.uintptr_t) { *Get[*[]int](id) = Get[[]int](value) }

// ---------------------------------------------------------------------------
// *MyNamedNonStructSliceInt: a type distinct from *[]int, with its own exports.

//export BrNewPtrMyNamedNonStructSliceInt
func BrNewPtrMyNamedNonStructSliceInt(value C.uintptr_t) C.uintptr_t {
	return newPtr(Get[api.MyNamedNonStructSliceInt](value))
}

//export BrPtrMyNamedNonStructSliceIntLoad
func BrPtrMyNamedNonStructSliceIntLoad(id C.uintptr_t) C.uintptr_t {
	return Pin(*Get[*api.MyNamedNonStructSliceInt](id))
}

//export BrPtrMyNamedNonStructSliceIntStore
func BrPtrMyNamedNonStructSliceIntStore(id, value C.uintptr_t) {
	*Get[*api.MyNamedNonStructSliceInt](id) = Get[api.MyNamedNonStructSliceInt](value)
}

//export BrPtrMyNamedNonStructSliceIntEchoNonStructSliceInt
func BrPtrMyNamedNonStructSliceIntEchoNonStructSliceInt(id C.uintptr_t) C.BrString {
	return cString(Get[*api.MyNamedNonStructSliceInt](id).EchoNonStructSliceInt())
}

//export BrPtrMyNamedNonStructSliceIntPtrEchoNonStructSliceInt
func BrPtrMyNamedNonStructSliceIntPtrEchoNonStructSliceInt(id C.uintptr_t) C.BrString {
	return cString(Get[*api.MyNamedNonStructSliceInt](id).PtrEchoNonStructSliceInt())
}

//export BrPtrMyNamedNonStructSliceIntAppend
func BrPtrMyNamedNonStructSliceIntAppend(id C.uintptr_t, v C.intptr_t) {
	Get[*api.MyNamedNonStructSliceInt](id).Append(int(v))
}

// ---------------------------------------------------------------------------
// Functions.

//export BrFnEchoNonStructInt
func BrFnEchoNonStructInt(i C.intptr_t) C.BrString {
	return cString(api.FnEchoNonStructInt(api.MyNamedNonStructInt(i)))
}

//export BrPtrFnEchoNonStructInt
func BrPtrFnEchoNonStructInt(i C.uintptr_t) C.BrString {
	return cString(api.PtrFnEchoNonStructInt(Get[*api.MyNamedNonStructInt](i)))
}

//export BrFnEchoInt
func BrFnEchoInt(i C.intptr_t) C.BrString { return cString(api.FnEchoInt(int(i))) }

//export BrPtrFnEchoInt
func BrPtrFnEchoInt(i C.uintptr_t) C.BrString {
	return cString(api.PtrFnEchoInt(Get[*int](i)))
}

//export BrIncInt
func BrIncInt(i C.uintptr_t) { api.IncInt(Get[*int](i)) }

//export BrFnEchoNonStructSliceInt
func BrFnEchoNonStructSliceInt(si C.uintptr_t) C.BrString {
	return cString(api.FnEchoNonStructSliceInt(Get[api.MyNamedNonStructSliceInt](si)))
}

//export BrPtrFnEchoNonStructSliceInt
func BrPtrFnEchoNonStructSliceInt(si C.uintptr_t) C.BrString {
	return cString(api.PtrFnEchoNonStructSliceInt(Get[*api.MyNamedNonStructSliceInt](si)))
}

//export BrFnEchoSliceInt
func BrFnEchoSliceInt(si C.uintptr_t) C.BrString {
	return cString(api.FnEchoSliceInt(Get[[]int](si)))
}

//export BrPtrFnEchoSliceInt
func BrPtrFnEchoSliceInt(si C.uintptr_t) C.BrString {
	return cString(api.PtrFnEchoSliceInt(Get[*[]int](si)))
}

//export BrAppendSliceInt
func BrAppendSliceInt(si C.uintptr_t, v C.intptr_t) { api.AppendSliceInt(Get[*[]int](si), int(v)) }

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
