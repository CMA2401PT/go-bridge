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
	api "go-bridge/examples/08_multi_ptr"
	"runtime"
)

// Go string results use the BrString ABI of 03_str; C++ frees data.
func cString(value string) C.BrString {
	return C.BrString{data: C.CString(value), size: C.size_t(len(value))}
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

//export BrNewPtrInt
func BrNewPtrInt(value C.intptr_t) C.uintptr_t { return newPtr(int(value)) }

//export BrPtrIntLoad
func BrPtrIntLoad(id C.uintptr_t) C.intptr_t { return C.intptr_t(*Get[*int](id)) }

//export BrPtrIntStore
func BrPtrIntStore(id C.uintptr_t, value C.intptr_t) { *Get[*int](id) = int(value) }

// A pointer-to-pointer is built from a pointer: the new variable holds a
// copy of that pointer, so both refer to the same target.
//
//export BrNewPtrPtrInt
func BrNewPtrPtrInt(p C.uintptr_t) C.uintptr_t { return newPtr(Get[*int](p)) }

//export BrPtrPtrIntLoad
func BrPtrPtrIntLoad(id C.uintptr_t) C.uintptr_t { return loadPointer[int, *int](id) }

//export BrPtrPtrIntStore
func BrPtrPtrIntStore(id, p C.uintptr_t) { *Get[**int](id) = Get[*int](p) }

//export BrNewPtrPtrPtrInt
func BrNewPtrPtrPtrInt(pp C.uintptr_t) C.uintptr_t { return newPtr(Get[**int](pp)) }

//export BrPtrPtrPtrIntLoad
func BrPtrPtrPtrIntLoad(id C.uintptr_t) C.uintptr_t { return loadPointer[*int, **int](id) }

//export BrPtrPtrPtrIntStore
func BrPtrPtrPtrIntStore(id, pp C.uintptr_t) { *Get[***int](id) = Get[**int](pp) }

// ---------------------------------------------------------------------------
// IntPtr: a named pointer. Its handle pins an IntPtr value; conversions to
// and from *int share the variable.

//export BrNewIntPtr
func BrNewIntPtr(p C.uintptr_t) C.uintptr_t { return Pin(api.IntPtr(Get[*int](p))) }

//export BrIntPtrUnname
func BrIntPtrUnname(id C.uintptr_t) C.uintptr_t { return Pin((*int)(Get[api.IntPtr](id))) }

//export BrIntPtrLoad
func BrIntPtrLoad(id C.uintptr_t) C.intptr_t { return C.intptr_t(*Get[api.IntPtr](id)) }

//export BrIntPtrStore
func BrIntPtrStore(id C.uintptr_t, value C.intptr_t) { *Get[api.IntPtr](id) = int(value) }

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

//export BrCounterEcho
func BrCounterEcho(value C.intptr_t) C.BrString { return cString(api.Counter(value).Echo()) }

//export BrNewInner
func BrNewInner(n C.intptr_t) C.uintptr_t { return Pin(&api.Inner{N: int(n)}) }

//export BrInnerGetN
func BrInnerGetN(id C.uintptr_t) C.intptr_t { return C.intptr_t(Get[*api.Inner](id).N) }

//export BrInnerSetN
func BrInnerSetN(id C.uintptr_t, n C.intptr_t) { Get[*api.Inner](id).N = int(n) }

//export BrInnerCopy
func BrInnerCopy(id C.uintptr_t) C.uintptr_t { return copyStruct[api.Inner](id) }

//export BrInnerAlias
func BrInnerAlias(id C.uintptr_t) C.uintptr_t { return aliasStruct[api.Inner](id) }

//export BrInnerStore
func BrInnerStore(id, value C.uintptr_t) { storeStruct[api.Inner](id, value) }

// ---------------------------------------------------------------------------
// Box and *Box, as in 06_named_type: both handles pin a *Box.

// A 0 inner handle keeps the zero value.
//
//export BrNewBox
func BrNewBox(count C.intptr_t, inner C.uintptr_t) C.uintptr_t {
	value := &api.Box{Count: api.Counter(count)}
	if inner != 0 {
		value.Inner = *Get[*api.Inner](inner)
	}
	return Pin(value)
}

//export BrBoxGetCount
func BrBoxGetCount(id C.uintptr_t) C.intptr_t { return C.intptr_t(Get[*api.Box](id).Count) }

//export BrBoxSetCount
func BrBoxSetCount(id C.uintptr_t, count C.intptr_t) {
	Get[*api.Box](id).Count = api.Counter(count)
}

//export BrBoxGetInner
func BrBoxGetInner(id C.uintptr_t) C.uintptr_t { return Pin(&Get[*api.Box](id).Inner) }

//export BrBoxSetInner
func BrBoxSetInner(id, inner C.uintptr_t) { Get[*api.Box](id).Inner = *Get[*api.Inner](inner) }

//export BrBoxEcho
func BrBoxEcho(id C.uintptr_t) C.BrString { return cString(Get[*api.Box](id).Echo()) }

//export BrBoxInc
func BrBoxInc(id C.uintptr_t) { Get[*api.Box](id).Inc() }

//export BrBoxCopy
func BrBoxCopy(id C.uintptr_t) C.uintptr_t { return copyStruct[api.Box](id) }

//export BrBoxAlias
func BrBoxAlias(id C.uintptr_t) C.uintptr_t { return aliasStruct[api.Box](id) }

//export BrBoxStore
func BrBoxStore(id, value C.uintptr_t) { storeStruct[api.Box](id, value) }

// **Box. Its target is a *Box, a PtrBox handle; storing one copies the
// pointer, not the struct. **Box has an empty method set in Go.
//
//export BrNewPtrPtrBox
func BrNewPtrPtrBox(box C.uintptr_t) C.uintptr_t { return newPtr(Get[*api.Box](box)) }

//export BrPtrPtrBoxLoad
func BrPtrPtrBoxLoad(id C.uintptr_t) C.uintptr_t { return loadPointer[api.Box, *api.Box](id) }

//export BrPtrPtrBoxStore
func BrPtrPtrBoxStore(id, box C.uintptr_t) { *Get[**api.Box](id) = Get[*api.Box](box) }

// ---------------------------------------------------------------------------
// Functions.

//export BrIncPPP
func BrIncPPP(p C.uintptr_t) { api.IncPPP(Get[***int](p)) }

//export BrEchoPPP
func BrEchoPPP(p C.uintptr_t) C.BrString { return cString(api.EchoPPP(Get[***int](p))) }

//export BrRetarget
func BrRetarget(pp, p C.uintptr_t) { api.Retarget(Get[**int](pp), Get[*int](p)) }

//export BrClear
func BrClear(pp C.uintptr_t) { api.Clear(Get[**int](pp)) }

//export BrIncIntPtr
func BrIncIntPtr(p C.uintptr_t) { api.IncIntPtr(Get[api.IntPtr](p)) }

//export BrIncBoxPP
func BrIncBoxPP(pp C.uintptr_t) { api.IncBoxPP(Get[**api.Box](pp)) }

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
