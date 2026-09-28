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
	api "go-bridge/examples/07_ptr_basic"
	"math"
	"runtime"
	"unsafe"
)

// Go string results use the SBString ABI of 03_str; C++ frees data.
func cString(value string) C.SBString {
	return C.SBString{data: C.CString(value), size: C.size_t(len(value))}
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
// Python-side call to SBNewPtrMyNamedNonStructInt.

//export SBMyNamedNonStructIntEchoNonStructInt
func SBMyNamedNonStructIntEchoNonStructInt(value C.intptr_t) C.SBString {
	return cString(api.MyNamedNonStructInt(value).EchoNonStructInt())
}

// ---------------------------------------------------------------------------
// *int. The handle's cell holds the pointer and never changes; Load and
// Store read and write the target variable *p.

//export SBNewPtrInt
func SBNewPtrInt(value C.intptr_t) C.uintptr_t { return newPtr(int(value)) }

//export SBPtrIntLoad
func SBPtrIntLoad(id C.uintptr_t) C.intptr_t { return C.intptr_t(*Get[*int](id)) }

//export SBPtrIntStore
func SBPtrIntStore(id C.uintptr_t, value C.intptr_t) { *Get[*int](id) = int(value) }

// ---------------------------------------------------------------------------
// *MyNamedNonStructInt. It is a type distinct from *int, so it has its own
// exports; Get always reads the exact type that was pinned.

//export SBNewPtrMyNamedNonStructInt
func SBNewPtrMyNamedNonStructInt(value C.intptr_t) C.uintptr_t {
	return newPtr(api.MyNamedNonStructInt(value))
}

//export SBPtrMyNamedNonStructIntLoad
func SBPtrMyNamedNonStructIntLoad(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(*Get[*api.MyNamedNonStructInt](id))
}

//export SBPtrMyNamedNonStructIntStore
func SBPtrMyNamedNonStructIntStore(id C.uintptr_t, value C.intptr_t) {
	*Get[*api.MyNamedNonStructInt](id) = api.MyNamedNonStructInt(value)
}

// The method set of *T includes the value-receiver methods of T: (*p).M().
//
//export SBPtrMyNamedNonStructIntEchoNonStructInt
func SBPtrMyNamedNonStructIntEchoNonStructInt(id C.uintptr_t) C.SBString {
	return cString(Get[*api.MyNamedNonStructInt](id).EchoNonStructInt())
}

//export SBPtrMyNamedNonStructIntPtrEchoNonStructInt
func SBPtrMyNamedNonStructIntPtrEchoNonStructInt(id C.uintptr_t) C.SBString {
	return cString(Get[*api.MyNamedNonStructInt](id).PtrEchoNonStructInt())
}

//export SBPtrMyNamedNonStructIntInc
func SBPtrMyNamedNonStructIntInc(id C.uintptr_t) { Get[*api.MyNamedNonStructInt](id).Inc() }

// ---------------------------------------------------------------------------
// []int, reduced to what this example needs from 05_slice: construction,
// length and a bulk copy. MyNamedNonStructSliceInt has the same layout and
// reuses these exports.

func validBuffer[E any](data unsafe.Pointer, size C.size_t) bool {
	var element E
	limit := uint64(math.MaxInt) / uint64(unsafe.Sizeof(element))
	return uint64(size) <= limit && (data != nil || size == 0)
}

//export SBNewSliceInt
func SBNewSliceInt(data unsafe.Pointer, size C.size_t) C.uintptr_t {
	if !validBuffer[int](data, size) {
		return 0
	}
	value := make([]int, int(size))
	copy(value, unsafe.Slice((*int)(data), int(size)))
	return Pin(value)
}

//export SBSliceIntLen
func SBSliceIntLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[]int](id)))
}

//export SBCopySliceInt
func SBCopySliceInt(id C.uintptr_t, data unsafe.Pointer, size C.size_t) C.int {
	value := Get[[]int](id)
	if uint64(size) != uint64(len(value)) || !validBuffer[int](data, size) {
		return 1
	}
	copy(unsafe.Slice((*int)(data), len(value)), value)
	return 0
}

// Go's MyNamedNonStructSliceInt(s) and []int(si), as in 06.
//
//export SBNewMyNamedNonStructSliceInt
func SBNewMyNamedNonStructSliceInt(id C.uintptr_t) C.uintptr_t {
	return Pin(api.MyNamedNonStructSliceInt(Get[[]int](id)))
}

//export SBMyNamedNonStructSliceIntUnname
func SBMyNamedNonStructSliceIntUnname(id C.uintptr_t) C.uintptr_t {
	return Pin([]int(Get[api.MyNamedNonStructSliceInt](id)))
}

//export SBMyNamedNonStructSliceIntEchoNonStructSliceInt
func SBMyNamedNonStructSliceIntEchoNonStructSliceInt(id C.uintptr_t) C.SBString {
	return cString(Get[api.MyNamedNonStructSliceInt](id).EchoNonStructSliceInt())
}

// ---------------------------------------------------------------------------
// *[]int. *p is a slice header: Load pins a copy of the header (sharing the
// backing array, like Go's s := *p) and Store replaces it (*p = s). A
// header change made through the pointer, e.g. append, is therefore visible
// through p.value but not in a SliceInt loaded earlier.

//export SBNewPtrSliceInt
func SBNewPtrSliceInt(value C.uintptr_t) C.uintptr_t { return newPtr(Get[[]int](value)) }

//export SBPtrSliceIntLoad
func SBPtrSliceIntLoad(id C.uintptr_t) C.uintptr_t { return Pin(*Get[*[]int](id)) }

//export SBPtrSliceIntStore
func SBPtrSliceIntStore(id, value C.uintptr_t) { *Get[*[]int](id) = Get[[]int](value) }

// ---------------------------------------------------------------------------
// *MyNamedNonStructSliceInt: a type distinct from *[]int, with its own exports.

//export SBNewPtrMyNamedNonStructSliceInt
func SBNewPtrMyNamedNonStructSliceInt(value C.uintptr_t) C.uintptr_t {
	return newPtr(Get[api.MyNamedNonStructSliceInt](value))
}

//export SBPtrMyNamedNonStructSliceIntLoad
func SBPtrMyNamedNonStructSliceIntLoad(id C.uintptr_t) C.uintptr_t {
	return Pin(*Get[*api.MyNamedNonStructSliceInt](id))
}

//export SBPtrMyNamedNonStructSliceIntStore
func SBPtrMyNamedNonStructSliceIntStore(id, value C.uintptr_t) {
	*Get[*api.MyNamedNonStructSliceInt](id) = Get[api.MyNamedNonStructSliceInt](value)
}

//export SBPtrMyNamedNonStructSliceIntEchoNonStructSliceInt
func SBPtrMyNamedNonStructSliceIntEchoNonStructSliceInt(id C.uintptr_t) C.SBString {
	return cString(Get[*api.MyNamedNonStructSliceInt](id).EchoNonStructSliceInt())
}

//export SBPtrMyNamedNonStructSliceIntPtrEchoNonStructSliceInt
func SBPtrMyNamedNonStructSliceIntPtrEchoNonStructSliceInt(id C.uintptr_t) C.SBString {
	return cString(Get[*api.MyNamedNonStructSliceInt](id).PtrEchoNonStructSliceInt())
}

//export SBPtrMyNamedNonStructSliceIntAppend
func SBPtrMyNamedNonStructSliceIntAppend(id C.uintptr_t, v C.intptr_t) {
	Get[*api.MyNamedNonStructSliceInt](id).Append(int(v))
}

// ---------------------------------------------------------------------------
// Functions.

//export SBFnEchoNonStructInt
func SBFnEchoNonStructInt(i C.intptr_t) C.SBString {
	return cString(api.FnEchoNonStructInt(api.MyNamedNonStructInt(i)))
}

//export SBPtrFnEchoNonStructInt
func SBPtrFnEchoNonStructInt(i C.uintptr_t) C.SBString {
	return cString(api.PtrFnEchoNonStructInt(Get[*api.MyNamedNonStructInt](i)))
}

//export SBFnEchoInt
func SBFnEchoInt(i C.intptr_t) C.SBString { return cString(api.FnEchoInt(int(i))) }

//export SBPtrFnEchoInt
func SBPtrFnEchoInt(i C.uintptr_t) C.SBString {
	return cString(api.PtrFnEchoInt(Get[*int](i)))
}

//export SBIncInt
func SBIncInt(i C.uintptr_t) { api.IncInt(Get[*int](i)) }

//export SBFnEchoNonStructSliceInt
func SBFnEchoNonStructSliceInt(si C.uintptr_t) C.SBString {
	return cString(api.FnEchoNonStructSliceInt(Get[api.MyNamedNonStructSliceInt](si)))
}

//export SBPtrFnEchoNonStructSliceInt
func SBPtrFnEchoNonStructSliceInt(si C.uintptr_t) C.SBString {
	return cString(api.PtrFnEchoNonStructSliceInt(Get[*api.MyNamedNonStructSliceInt](si)))
}

//export SBFnEchoSliceInt
func SBFnEchoSliceInt(si C.uintptr_t) C.SBString {
	return cString(api.FnEchoSliceInt(Get[[]int](si)))
}

//export SBPtrFnEchoSliceInt
func SBPtrFnEchoSliceInt(si C.uintptr_t) C.SBString {
	return cString(api.PtrFnEchoSliceInt(Get[*[]int](si)))
}

//export SBAppendSliceInt
func SBAppendSliceInt(si C.uintptr_t, v C.intptr_t) { api.AppendSliceInt(Get[*[]int](si), int(v)) }

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
