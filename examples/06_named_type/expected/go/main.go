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
	api "go-bridge/examples/06_named_type"
	"math"
	"runtime"
	"slices"
	"unsafe"
)

// Go string results use the SBString ABI of 03_str; C++ frees data.
func cString(value string) C.SBString {
	return C.SBString{data: C.CString(value), size: C.size_t(len(value))}
}

// ---------------------------------------------------------------------------
// []int, as in 05_slice.

// validBuffer checks an external buffer of size elements of type E.
func validBuffer[E any](data unsafe.Pointer, size C.size_t) bool {
	var element E
	limit := uint64(math.MaxInt) / uint64(unsafe.Sizeof(element))
	return uint64(size) <= limit && (data != nil || size == 0)
}

func inRange[E any](value []E, index C.size_t) bool {
	return uint64(index) < uint64(len(value))
}

func clone[E any](id C.uintptr_t) C.uintptr_t {
	return Pin(slices.Clone(Get[[]E](id)))
}

// reslice is Go's value[low:high]; it shares the backing array.
func reslice[E any](id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	value := Get[[]E](id)
	if low > high || uint64(high) > uint64(len(value)) {
		return 0
	}
	return Pin(value[low:high])
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

//export SBSliceIntAt
func SBSliceIntAt(id C.uintptr_t, index C.size_t, out unsafe.Pointer) C.int {
	value := Get[[]int](id)
	if !inRange(value, index) || out == nil {
		return 1
	}
	*(*int)(out) = value[index]
	return 0
}

//export SBSliceIntSet
func SBSliceIntSet(id C.uintptr_t, index C.size_t, item C.intptr_t) C.int {
	value := Get[[]int](id)
	if !inRange(value, index) {
		return 1
	}
	value[index] = int(item)
	return 0
}

//export SBSliceIntClone
func SBSliceIntClone(id C.uintptr_t) C.uintptr_t { return clone[int](id) }

//export SBSliceIntSlice
func SBSliceIntSlice(id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	return reslice[int](id, low, high)
}

// ---------------------------------------------------------------------------
// MyNamedNonStructInt: a named value type crosses the ABI as its underlying
// int (intptr_t). Only the Python class keeps the name.

//export SBMyNamedNonStructIntEchoNonStructInt
func SBMyNamedNonStructIntEchoNonStructInt(value C.intptr_t) C.SBString {
	return cString(api.MyNamedNonStructInt(value).EchoNonStructInt())
}

//export SBFnEchoNonStructInt
func SBFnEchoNonStructInt(i C.intptr_t) C.SBString {
	return cString(api.FnEchoNonStructInt(api.MyNamedNonStructInt(i)))
}

// ---------------------------------------------------------------------------
// MyNamedNonStructSliceInt: its handle has the []int layout, so len,
// indexing, reslice, clone and copy reuse the SliceInt exports. Get converts
// to the named type only where Go needs the name.

// SBNewMyNamedNonStructSliceInt is Go's MyNamedNonStructSliceInt(s): a new
// handle that shares the backing array.
//
//export SBNewMyNamedNonStructSliceInt
func SBNewMyNamedNonStructSliceInt(id C.uintptr_t) C.uintptr_t {
	return Pin(api.MyNamedNonStructSliceInt(Get[[]int](id)))
}

//export SBMyNamedNonStructSliceIntEchoNonStructSliceInt
func SBMyNamedNonStructSliceIntEchoNonStructSliceInt(id C.uintptr_t) C.SBString {
	return cString(Get[api.MyNamedNonStructSliceInt](id).EchoNonStructSliceInt())
}

// SBMyNamedNonStructSliceIntUnname is Go's []int(si): a new handle that shares
// the backing array.
//
//export SBMyNamedNonStructSliceIntUnname
func SBMyNamedNonStructSliceIntUnname(id C.uintptr_t) C.uintptr_t {
	return Pin([]int(Get[api.MyNamedNonStructSliceInt](id)))
}

//export SBFnEchoNonStructSliceInt
func SBFnEchoNonStructSliceInt(si C.uintptr_t) C.SBString {
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

//export SBNewMyNamedSubStruct
func SBNewMyNamedSubStruct(basicMemberValue C.intptr_t) C.uintptr_t {
	return Pin(&api.MyNamedSubStruct{BasicMemberValue: int(basicMemberValue)})
}

//export SBMyNamedSubStructGetBasicMemberValue
func SBMyNamedSubStructGetBasicMemberValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedSubStruct](id).BasicMemberValue)
}

//export SBMyNamedSubStructSetBasicMemberValue
func SBMyNamedSubStructSetBasicMemberValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedSubStruct](id).BasicMemberValue = int(value)
}

//export SBMyNamedSubStructCopy
func SBMyNamedSubStructCopy(id C.uintptr_t) C.uintptr_t {
	return copyStruct[api.MyNamedSubStruct](id)
}

//export SBMyNamedSubStructAlias
func SBMyNamedSubStructAlias(id C.uintptr_t) C.uintptr_t {
	return aliasStruct[api.MyNamedSubStruct](id)
}

//export SBMyNamedSubStructStore
func SBMyNamedSubStructStore(id, value C.uintptr_t) {
	storeStruct[api.MyNamedSubStruct](id, value)
}

// MyNamedAnomySubStruct

//export SBNewMyNamedAnomySubStruct
func SBNewMyNamedAnomySubStruct(basicMemberValue2 C.intptr_t) C.uintptr_t {
	return Pin(&api.MyNamedAnomySubStruct{BasicMemberValue2: int(basicMemberValue2)})
}

//export SBMyNamedAnomySubStructGetBasicMemberValue2
func SBMyNamedAnomySubStructGetBasicMemberValue2(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedAnomySubStruct](id).BasicMemberValue2)
}

//export SBMyNamedAnomySubStructSetBasicMemberValue2
func SBMyNamedAnomySubStructSetBasicMemberValue2(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedAnomySubStruct](id).BasicMemberValue2 = int(value)
}

//export SBMyNamedAnomySubStructCopy
func SBMyNamedAnomySubStructCopy(id C.uintptr_t) C.uintptr_t {
	return copyStruct[api.MyNamedAnomySubStruct](id)
}

//export SBMyNamedAnomySubStructAlias
func SBMyNamedAnomySubStructAlias(id C.uintptr_t) C.uintptr_t {
	return aliasStruct[api.MyNamedAnomySubStruct](id)
}

//export SBMyNamedAnomySubStructStore
func SBMyNamedAnomySubStructStore(id, value C.uintptr_t) {
	storeStruct[api.MyNamedAnomySubStruct](id, value)
}

// MyNamedStruct

// SBNewMyNamedStruct is a keyed composite literal: struct arguments are copied
// into the new value, the slice argument shares its backing array. A 0 handle
// is an omitted field and keeps its zero value (a zero struct or a nil slice).
//
//export SBNewMyNamedStruct
func SBNewMyNamedStruct(
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

//export SBMyNamedStructGetMyNamedAnomySubStruct
func SBMyNamedStructGetMyNamedAnomySubStruct(id C.uintptr_t) C.uintptr_t {
	return Pin(&Get[*api.MyNamedStruct](id).MyNamedAnomySubStruct)
}

//export SBMyNamedStructSetMyNamedAnomySubStruct
func SBMyNamedStructSetMyNamedAnomySubStruct(id C.uintptr_t, value C.uintptr_t) {
	Get[*api.MyNamedStruct](id).MyNamedAnomySubStruct = *Get[*api.MyNamedAnomySubStruct](value)
}

// BasicMemberValue2 is promoted from the embedded MyNamedAnomySubStruct.
//
//export SBMyNamedStructGetBasicMemberValue2
func SBMyNamedStructGetBasicMemberValue2(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedStruct](id).BasicMemberValue2)
}

//export SBMyNamedStructSetBasicMemberValue2
func SBMyNamedStructSetBasicMemberValue2(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedStruct](id).BasicMemberValue2 = int(value)
}

//export SBMyNamedStructGetBasicMemberValue
func SBMyNamedStructGetBasicMemberValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedStruct](id).BasicMemberValue)
}

//export SBMyNamedStructSetBasicMemberValue
func SBMyNamedStructSetBasicMemberValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedStruct](id).BasicMemberValue = int(value)
}

// A slice field is read and written as a slice header; arrays stay shared.
//
//export SBMyNamedStructGetRefMemberValue
func SBMyNamedStructGetRefMemberValue(id C.uintptr_t) C.uintptr_t {
	return Pin(Get[*api.MyNamedStruct](id).RefMemberValue)
}

//export SBMyNamedStructSetRefMemberValue
func SBMyNamedStructSetRefMemberValue(id C.uintptr_t, value C.uintptr_t) {
	Get[*api.MyNamedStruct](id).RefMemberValue = Get[[]int](value)
}

//export SBMyNamedStructGetNamedBasicMemberValue
func SBMyNamedStructGetNamedBasicMemberValue(id C.uintptr_t) C.intptr_t {
	return C.intptr_t(Get[*api.MyNamedStruct](id).NamedBasicMemberValue)
}

//export SBMyNamedStructSetNamedBasicMemberValue
func SBMyNamedStructSetNamedBasicMemberValue(id C.uintptr_t, value C.intptr_t) {
	Get[*api.MyNamedStruct](id).NamedBasicMemberValue = api.MyNamedNonStructInt(value)
}

//export SBMyNamedStructGetNamedRefMemberValue
func SBMyNamedStructGetNamedRefMemberValue(id C.uintptr_t) C.uintptr_t {
	return Pin(&Get[*api.MyNamedStruct](id).NamedRefMemberValue)
}

//export SBMyNamedStructSetNamedRefMemberValue
func SBMyNamedStructSetNamedRefMemberValue(id C.uintptr_t, value C.uintptr_t) {
	Get[*api.MyNamedStruct](id).NamedRefMemberValue = *Get[*api.MyNamedSubStruct](value)
}

//export SBMyNamedStructCopy
func SBMyNamedStructCopy(id C.uintptr_t) C.uintptr_t {
	return copyStruct[api.MyNamedStruct](id)
}

//export SBMyNamedStructAlias
func SBMyNamedStructAlias(id C.uintptr_t) C.uintptr_t {
	return aliasStruct[api.MyNamedStruct](id)
}

//export SBMyNamedStructStore
func SBMyNamedStructStore(id, value C.uintptr_t) {
	storeStruct[api.MyNamedStruct](id, value)
}

// Methods: a value receiver gets a copy of *p, a pointer receiver gets p, so
// its writes are visible through every Ref to the same struct. MyNamedStruct
// and PtrMyNamedStruct both call these.

//export SBMyNamedStructEchoNonPtr
func SBMyNamedStructEchoNonPtr(id C.uintptr_t) C.SBString {
	return cString(Get[*api.MyNamedStruct](id).EchoNonPtr())
}

//export SBMyNamedStructEchoPtr
func SBMyNamedStructEchoPtr(id C.uintptr_t) C.SBString {
	return cString(Get[*api.MyNamedStruct](id).EchoPtr())
}

//export SBMyNamedStructIncBasicPtr
func SBMyNamedStructIncBasicPtr(id C.uintptr_t) {
	Get[*api.MyNamedStruct](id).IncBasicPtr()
}

//export SBMyNamedStructIncBasicNonPtr
func SBMyNamedStructIncBasicNonPtr(id C.uintptr_t) C.uintptr_t {
	result := Get[*api.MyNamedStruct](id).IncBasicNonPtr()
	return Pin(&result)
}

// Functions: a T parameter receives a copy, a *T parameter the pointer.

//export SBFnEchoNonPtr
func SBFnEchoNonPtr(si C.uintptr_t) C.SBString {
	return cString(api.FnEchoNonPtr(*Get[*api.MyNamedStruct](si)))
}

//export SBPtrFnEchoPtr
func SBPtrFnEchoPtr(si C.uintptr_t) C.SBString {
	return cString(api.PtrFnEchoPtr(Get[*api.MyNamedStruct](si)))
}

//export SBUnPin
func SBUnPin(id C.uintptr_t) { UnPin(id) }

// Internal test diagnostics.
//
//export SBHolderCount
func SBHolderCount() C.size_t { return C.size_t(holderCount()) }

//export SBCollect
func SBCollect() { runtime.GC() }

func main() {}
