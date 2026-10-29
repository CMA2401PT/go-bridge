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
	api "go-bridge/examples/06_named_type"
	"runtime"
	"slices"
	"unsafe"
)

// Go string results use the BrString ABI of 03_str; C++ frees data.
func cString(value string) C.BrString {
	return C.BrString{data: C.CString(value), size: C.size_t(len(value))}
}

// ---------------------------------------------------------------------------
// []int, as in 05_slice.

func clone[E any](id C.uintptr_t) C.uintptr_t {
	return Pin(slices.Clone(Get[[]E](id)))
}

// reslice is Go's value[low:high]; it shares the backing array.
func reslice[E any](id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	value := Get[[]E](id)
	return Pin(value[low:high])
}

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

//export BrSliceIntAt
func BrSliceIntAt(id C.uintptr_t, index C.size_t) C.intptr_t {
	return C.intptr_t(Get[[]int](id)[index])
}

//export BrSliceIntSet
func BrSliceIntSet(id C.uintptr_t, index C.size_t, item C.intptr_t) {
	value := Get[[]int](id)
	value[index] = int(item)
}

//export BrSliceIntClone
func BrSliceIntClone(id C.uintptr_t) C.uintptr_t { return clone[int](id) }

//export BrSliceIntSlice
func BrSliceIntSlice(id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	return reslice[int](id, low, high)
}

// ---------------------------------------------------------------------------
// MyNamedNonStructInt: a named value type crosses the ABI as its underlying
// int (intptr_t). Only the Python class keeps the name.

//export BrMyNamedNonStructIntEchoNonStructInt
func BrMyNamedNonStructIntEchoNonStructInt(value C.intptr_t) C.BrString {
	return cString(api.MyNamedNonStructInt(value).EchoNonStructInt())
}

//export BrFnEchoNonStructInt
func BrFnEchoNonStructInt(i C.intptr_t) C.BrString {
	return cString(api.FnEchoNonStructInt(api.MyNamedNonStructInt(i)))
}

// ---------------------------------------------------------------------------
// MyNamedNonStructSliceInt: its handle has the []int layout, so len,
// indexing, reslice, clone and copy reuse the SliceInt exports. Get converts
// to the named type only where Go needs the name.

// BrNewMyNamedNonStructSliceInt is Go's MyNamedNonStructSliceInt(s): a new
// handle that shares the backing array.
//
//export BrNewMyNamedNonStructSliceInt
func BrNewMyNamedNonStructSliceInt(id C.uintptr_t) C.uintptr_t {
	return Pin(api.MyNamedNonStructSliceInt(Get[[]int](id)))
}

//export BrMyNamedNonStructSliceIntEchoNonStructSliceInt
func BrMyNamedNonStructSliceIntEchoNonStructSliceInt(id C.uintptr_t) C.BrString {
	return cString(Get[api.MyNamedNonStructSliceInt](id).EchoNonStructSliceInt())
}

// BrMyNamedNonStructSliceIntUnname is Go's []int(si): a new handle that shares
// the backing array.
//
//export BrMyNamedNonStructSliceIntUnname
func BrMyNamedNonStructSliceIntUnname(id C.uintptr_t) C.uintptr_t {
	return Pin([]int(Get[api.MyNamedNonStructSliceInt](id)))
}

//export BrFnEchoNonStructSliceInt
func BrFnEchoNonStructSliceInt(si C.uintptr_t) C.BrString {
	return cString(api.FnEchoNonStructSliceInt(Get[api.MyNamedNonStructSliceInt](si)))
}

// ---------------------------------------------------------------------------
// Structs. A struct wrapper pins a *T, never a T: each Ref owns its own cell
// even when several Refs reach the same struct or a field inside it (an
// embedded field at offset 0 shares its parent's address). The pointer keeps
// the struct, and for a field view the whole parent, reachable.
//
// T and *T have separate Python classes but the same handle, a pinned *T, so
// they share every export below; only the Python types differ.

// copyStruct is Go's v := *p, and also p := new(T); *p = v; slice fields
// still share their arrays.
func copyStruct[T any](id C.uintptr_t) C.uintptr_t {
	value := *Get[*T](id)
	return Pin(&value)
}

// aliasStruct pins the same *T under a new handle: T's addr() (Go's &v) and
// *T's value (Go's *p, a variable) both share the struct.
func aliasStruct[T any](id C.uintptr_t) C.uintptr_t {
	return Pin(Get[*T](id))
}

// storeStruct is Go's *p = v: the struct is copied in.
func storeStruct[T any](id, value C.uintptr_t) {
	*Get[*T](id) = *Get[*T](value)
}

// MyNamedSubStruct

//export BrNewMyNamedSubStruct
func BrNewMyNamedSubStruct(basicMemberValue C.intptr_t) C.uintptr_t {
	return Pin(&api.MyNamedSubStruct{BasicMemberValue: int(basicMemberValue)})
}

//export BrMyNamedSubStructGetBasicMemberValue
func BrMyNamedSubStructGetBasicMemberValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedSubStruct](id).BasicMemberValue)
}

//export BrMyNamedSubStructSetBasicMemberValue
func BrMyNamedSubStructSetBasicMemberValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedSubStruct](id).BasicMemberValue = int(value)
}

//export BrMyNamedSubStructCopy
func BrMyNamedSubStructCopy(id C.uintptr_t) C.uintptr_t {
	return copyStruct[api.MyNamedSubStruct](id)
}

//export BrMyNamedSubStructAlias
func BrMyNamedSubStructAlias(id C.uintptr_t) C.uintptr_t {
	return aliasStruct[api.MyNamedSubStruct](id)
}

//export BrMyNamedSubStructStore
func BrMyNamedSubStructStore(id, value C.uintptr_t) {
	storeStruct[api.MyNamedSubStruct](id, value)
}

// MyNamedAnomySubStruct

//export BrNewMyNamedAnomySubStruct
func BrNewMyNamedAnomySubStruct(basicMemberValue2 C.intptr_t) C.uintptr_t {
	return Pin(&api.MyNamedAnomySubStruct{BasicMemberValue2: int(basicMemberValue2)})
}

//export BrMyNamedAnomySubStructGetBasicMemberValue2
func BrMyNamedAnomySubStructGetBasicMemberValue2(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedAnomySubStruct](id).BasicMemberValue2)
}

//export BrMyNamedAnomySubStructSetBasicMemberValue2
func BrMyNamedAnomySubStructSetBasicMemberValue2(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedAnomySubStruct](id).BasicMemberValue2 = int(value)
}

//export BrMyNamedAnomySubStructCopy
func BrMyNamedAnomySubStructCopy(id C.uintptr_t) C.uintptr_t {
	return copyStruct[api.MyNamedAnomySubStruct](id)
}

//export BrMyNamedAnomySubStructAlias
func BrMyNamedAnomySubStructAlias(id C.uintptr_t) C.uintptr_t {
	return aliasStruct[api.MyNamedAnomySubStruct](id)
}

//export BrMyNamedAnomySubStructStore
func BrMyNamedAnomySubStructStore(id, value C.uintptr_t) {
	storeStruct[api.MyNamedAnomySubStruct](id, value)
}

// MyNamedStruct

// BrNewMyNamedStruct is a keyed composite literal: struct arguments are copied
// into the new value, the slice argument shares its backing array. A 0 handle
// is an omitted field and keeps its zero value (a zero struct or a nil slice).
//
//export BrNewMyNamedStruct
func BrNewMyNamedStruct(
	myNamedAnomySubStruct C.uintptr_t,
	basicMemberValue C.intptr_t,
	refMemberValue C.uintptr_t,
	namedBasicMemberValue C.intptr_t,
	namedRefMemberValue C.uintptr_t,
) C.uintptr_t {
	value := &api.MyNamedStruct{
		BasicMemberValue:      int(basicMemberValue),
		NamedBasicMemberValue: api.MyNamedNonStructInt(namedBasicMemberValue),
	}
	if myNamedAnomySubStruct != 0 {
		value.MyNamedAnomySubStruct = *Get[*api.MyNamedAnomySubStruct](myNamedAnomySubStruct)
	}
	if refMemberValue != 0 {
		value.RefMemberValue = Get[[]int](refMemberValue)
	}
	if namedRefMemberValue != 0 {
		value.NamedRefMemberValue = *Get[*api.MyNamedSubStruct](namedRefMemberValue)
	}
	return Pin(value)
}

// A struct-typed field is read as a view, Go's &s.Field; writing it copies the
// value in, Go's s.Field = v.

//export BrMyNamedStructGetMyNamedAnomySubStruct
func BrMyNamedStructGetMyNamedAnomySubStruct(id C.uintptr_t) C.uintptr_t {
	return Pin(&Get[*api.MyNamedStruct](id).MyNamedAnomySubStruct)
}

//export BrMyNamedStructSetMyNamedAnomySubStruct
func BrMyNamedStructSetMyNamedAnomySubStruct(id C.uintptr_t, value C.uintptr_t) {
	Get[*api.MyNamedStruct](id).MyNamedAnomySubStruct = *Get[*api.MyNamedAnomySubStruct](value)
}

// BasicMemberValue2 is promoted from the embedded MyNamedAnomySubStruct.
//
//export BrMyNamedStructGetBasicMemberValue2
func BrMyNamedStructGetBasicMemberValue2(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedStruct](id).BasicMemberValue2)
}

//export BrMyNamedStructSetBasicMemberValue2
func BrMyNamedStructSetBasicMemberValue2(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedStruct](id).BasicMemberValue2 = int(value)
}

//export BrMyNamedStructGetBasicMemberValue
func BrMyNamedStructGetBasicMemberValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedStruct](id).BasicMemberValue)
}

//export BrMyNamedStructSetBasicMemberValue
func BrMyNamedStructSetBasicMemberValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedStruct](id).BasicMemberValue = int(value)
}

// A slice field is read and written as a slice header; arrays stay shared.
//
//export BrMyNamedStructGetRefMemberValue
func BrMyNamedStructGetRefMemberValue(id C.uintptr_t) C.uintptr_t {
	return Pin(Get[*api.MyNamedStruct](id).RefMemberValue)
}

//export BrMyNamedStructSetRefMemberValue
func BrMyNamedStructSetRefMemberValue(id C.uintptr_t, value C.uintptr_t) {
	Get[*api.MyNamedStruct](id).RefMemberValue = Get[[]int](value)
}

//export BrMyNamedStructGetNamedBasicMemberValue
func BrMyNamedStructGetNamedBasicMemberValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedStruct](id).NamedBasicMemberValue)
}

//export BrMyNamedStructSetNamedBasicMemberValue
func BrMyNamedStructSetNamedBasicMemberValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedStruct](id).NamedBasicMemberValue = api.MyNamedNonStructInt(value)
}

//export BrMyNamedStructGetNamedRefMemberValue
func BrMyNamedStructGetNamedRefMemberValue(id C.uintptr_t) C.uintptr_t {
	return Pin(&Get[*api.MyNamedStruct](id).NamedRefMemberValue)
}

//export BrMyNamedStructSetNamedRefMemberValue
func BrMyNamedStructSetNamedRefMemberValue(id C.uintptr_t, value C.uintptr_t) {
	Get[*api.MyNamedStruct](id).NamedRefMemberValue = *Get[*api.MyNamedSubStruct](value)
}

//export BrMyNamedStructCopy
func BrMyNamedStructCopy(id C.uintptr_t) C.uintptr_t {
	return copyStruct[api.MyNamedStruct](id)
}

//export BrMyNamedStructAlias
func BrMyNamedStructAlias(id C.uintptr_t) C.uintptr_t {
	return aliasStruct[api.MyNamedStruct](id)
}

//export BrMyNamedStructStore
func BrMyNamedStructStore(id, value C.uintptr_t) {
	storeStruct[api.MyNamedStruct](id, value)
}

// Methods: a value receiver gets a copy of *p, a pointer receiver gets p, so
// its writes are visible through every Ref to the same struct. MyNamedStruct
// and PtrMyNamedStruct both call these.

//export BrMyNamedStructEchoNonPtr
func BrMyNamedStructEchoNonPtr(id C.uintptr_t) C.BrString {
	return cString(Get[*api.MyNamedStruct](id).EchoNonPtr())
}

//export BrMyNamedStructEchoPtr
func BrMyNamedStructEchoPtr(id C.uintptr_t) C.BrString {
	return cString(Get[*api.MyNamedStruct](id).EchoPtr())
}

//export BrMyNamedStructIncBasicPtr
func BrMyNamedStructIncBasicPtr(id C.uintptr_t) {
	Get[*api.MyNamedStruct](id).IncBasicPtr()
}

//export BrMyNamedStructIncBasicNonPtr
func BrMyNamedStructIncBasicNonPtr(id C.uintptr_t) C.uintptr_t {
	result := Get[*api.MyNamedStruct](id).IncBasicNonPtr()
	return Pin(&result)
}

// Functions: a T parameter receives a copy, a *T parameter the pointer.

//export BrFnEchoNonPtr
func BrFnEchoNonPtr(si C.uintptr_t) C.BrString {
	return cString(api.FnEchoNonPtr(*Get[*api.MyNamedStruct](si)))
}

//export BrPtrFnEchoPtr
func BrPtrFnEchoPtr(si C.uintptr_t) C.BrString {
	return cString(api.PtrFnEchoPtr(Get[*api.MyNamedStruct](si)))
}

//export BrUnPin
func BrUnPin(id C.uintptr_t) { UnPin(id) }

// Internal test diagnostics.
//
//export BrHolderCount
func BrHolderCount() C.size_t { return C.size_t(holderCount()) }

//export BrCollect
func BrCollect() { runtime.GC() }

func main() {}
