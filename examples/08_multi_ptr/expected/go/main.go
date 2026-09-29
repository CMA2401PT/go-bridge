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
	api "go-bridge/examples/08_multi_ptr"
	"runtime"
)

// Go string results use the SBString ABI of 03_str; C++ frees data.
func cString(value string) C.SBString {
	return C.SBString{data: C.CString(value), size: C.size_t(len(value))}
}

// newPtr is Go's p := new(T); *p = value, as in 07_ptr_basic.
func newPtr[T any](value T) C.uintptr_t {
	p := new(T)
	*p = value
	return Pin(p)
}

// loadPointer reads *p when the target is itself a pointer: a new handle to
// the same variable, or 0 when Go stored nil there.
func loadPointer[E any, P ~*E](id C.uintptr_t) C.uintptr_t {
	target := *Get[*P](id)
	if target == nil {
		return 0
	}
	return Pin(target)
}

// ---------------------------------------------------------------------------
// *int, **int and ***int. Only ***int appears in G; the type closure adds
// the other two so that every .value has a class.

//export SBNewPtrInt
func SBNewPtrInt(value C.intptr_t) C.uintptr_t { return newPtr(int(value)) }

//export SBPtrIntLoad
func SBPtrIntLoad(id C.uintptr_t) C.intptr_t { return C.intptr_t(*Get[*int](id)) }

//export SBPtrIntStore
func SBPtrIntStore(id C.uintptr_t, value C.intptr_t) { *Get[*int](id) = int(value) }

// A pointer-to-pointer is built from a pointer: the new variable holds a
// copy of that pointer, so both refer to the same target.
//
//export SBNewPtrPtrInt
func SBNewPtrPtrInt(p C.uintptr_t) C.uintptr_t { return newPtr(Get[*int](p)) }

//export SBPtrPtrIntLoad
func SBPtrPtrIntLoad(id C.uintptr_t) C.uintptr_t { return loadPointer[int, *int](id) }

//export SBPtrPtrIntStore
func SBPtrPtrIntStore(id, p C.uintptr_t) { *Get[**int](id) = Get[*int](p) }

//export SBNewPtrPtrPtrInt
func SBNewPtrPtrPtrInt(pp C.uintptr_t) C.uintptr_t { return newPtr(Get[**int](pp)) }

//export SBPtrPtrPtrIntLoad
func SBPtrPtrPtrIntLoad(id C.uintptr_t) C.uintptr_t { return loadPointer[*int, **int](id) }

//export SBPtrPtrPtrIntStore
func SBPtrPtrPtrIntStore(id, pp C.uintptr_t) { *Get[***int](id) = Get[**int](pp) }

// ---------------------------------------------------------------------------
// IntPtr: a named pointer. Its handle pins an IntPtr value; conversions to
// and from *int share the variable.

//export SBNewIntPtr
func SBNewIntPtr(p C.uintptr_t) C.uintptr_t { return Pin(api.IntPtr(Get[*int](p))) }

//export SBIntPtrUnname
func SBIntPtrUnname(id C.uintptr_t) C.uintptr_t { return Pin((*int)(Get[api.IntPtr](id))) }

//export SBIntPtrLoad
func SBIntPtrLoad(id C.uintptr_t) C.intptr_t { return C.intptr_t(*Get[api.IntPtr](id)) }

//export SBIntPtrStore
func SBIntPtrStore(id C.uintptr_t, value C.intptr_t) { *Get[api.IntPtr](id) = int(value) }

// ---------------------------------------------------------------------------
// Structs, as in 06: T and *T have separate Python classes whose handles both
// pin a *T, so they share every export.

// copyStruct is Go's p := new(T); *p = v.
func copyStruct[T any](id C.uintptr_t) C.uintptr_t {
	value := *Get[*T](id)
	return Pin(&value)
}

// aliasStruct is Go's &v, and *p read as a variable: the struct is shared.
func aliasStruct[T any](id C.uintptr_t) C.uintptr_t { return Pin(Get[*T](id)) }

// storeStruct is Go's *p = v.
func storeStruct[T any](id, value C.uintptr_t) { *Get[*T](id) = *Get[*T](value) }

// ---------------------------------------------------------------------------
// Counter and Inner appear only as Box fields; the closure generates them.

//export SBCounterEcho
func SBCounterEcho(value C.intptr_t) C.SBString { return cString(api.Counter(value).Echo()) }

//export SBNewInner
func SBNewInner(n C.intptr_t) C.uintptr_t { return Pin(&api.Inner{N: int(n)}) }

//export SBInnerGetN
func SBInnerGetN(id C.uintptr_t) C.intptr_t { return C.intptr_t(Get[*api.Inner](id).N) }

//export SBInnerSetN
func SBInnerSetN(id C.uintptr_t, n C.intptr_t) { Get[*api.Inner](id).N = int(n) }

//export SBInnerCopy
func SBInnerCopy(id C.uintptr_t) C.uintptr_t { return copyStruct[api.Inner](id) }

//export SBInnerAlias
func SBInnerAlias(id C.uintptr_t) C.uintptr_t { return aliasStruct[api.Inner](id) }

//export SBInnerStore
func SBInnerStore(id, value C.uintptr_t) { storeStruct[api.Inner](id, value) }

// ---------------------------------------------------------------------------
// Box and *Box, as in 06_named_type: both handles pin a *Box.

// A 0 inner handle keeps the zero value.
//
//export SBNewBox
func SBNewBox(count C.intptr_t, inner C.uintptr_t) C.uintptr_t {
	value := &api.Box{Count: api.Counter(count)}
	if inner != 0 {
		value.Inner = *Get[*api.Inner](inner)
	}
	return Pin(value)
}

//export SBBoxGetCount
func SBBoxGetCount(id C.uintptr_t) C.intptr_t { return C.intptr_t(Get[*api.Box](id).Count) }

//export SBBoxSetCount
func SBBoxSetCount(id C.uintptr_t, count C.intptr_t) {
	Get[*api.Box](id).Count = api.Counter(count)
}

//export SBBoxGetInner
func SBBoxGetInner(id C.uintptr_t) C.uintptr_t { return Pin(&Get[*api.Box](id).Inner) }

//export SBBoxSetInner
func SBBoxSetInner(id, inner C.uintptr_t) { Get[*api.Box](id).Inner = *Get[*api.Inner](inner) }

//export SBBoxEcho
func SBBoxEcho(id C.uintptr_t) C.SBString { return cString(Get[*api.Box](id).Echo()) }

//export SBBoxInc
func SBBoxInc(id C.uintptr_t) { Get[*api.Box](id).Inc() }

//export SBBoxCopy
func SBBoxCopy(id C.uintptr_t) C.uintptr_t { return copyStruct[api.Box](id) }

//export SBBoxAlias
func SBBoxAlias(id C.uintptr_t) C.uintptr_t { return aliasStruct[api.Box](id) }

//export SBBoxStore
func SBBoxStore(id, value C.uintptr_t) { storeStruct[api.Box](id, value) }

// **Box. Its target is a *Box, a PtrBox handle; storing one copies the
// pointer, not the struct. **Box has an empty method set in Go.
//
//export SBNewPtrPtrBox
func SBNewPtrPtrBox(box C.uintptr_t) C.uintptr_t { return newPtr(Get[*api.Box](box)) }

//export SBPtrPtrBoxLoad
func SBPtrPtrBoxLoad(id C.uintptr_t) C.uintptr_t { return loadPointer[api.Box, *api.Box](id) }

//export SBPtrPtrBoxStore
func SBPtrPtrBoxStore(id, box C.uintptr_t) { *Get[**api.Box](id) = Get[*api.Box](box) }

// ---------------------------------------------------------------------------
// Functions.

//export SBIncPPP
func SBIncPPP(p C.uintptr_t) { api.IncPPP(Get[***int](p)) }

//export SBEchoPPP
func SBEchoPPP(p C.uintptr_t) C.SBString { return cString(api.EchoPPP(Get[***int](p))) }

//export SBRetarget
func SBRetarget(pp, p C.uintptr_t) { api.Retarget(Get[**int](pp), Get[*int](p)) }

//export SBClear
func SBClear(pp C.uintptr_t) { api.Clear(Get[**int](pp)) }

//export SBIncIntPtr
func SBIncIntPtr(p C.uintptr_t) { api.IncIntPtr(Get[api.IntPtr](p)) }

//export SBIncBoxPP
func SBIncBoxPP(pp C.uintptr_t) { api.IncBoxPP(Get[**api.Box](pp)) }

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
