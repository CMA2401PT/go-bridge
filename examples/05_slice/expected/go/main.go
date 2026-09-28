package main

/*
#include <stdint.h>
#include <stddef.h>
*/
import "C"

import (
	api "go-bridge/examples/05_slice"
	"math"
	"runtime"
	"slices"
	"unsafe"
)

// validBuffer checks an external buffer of size elements of type E.
func validBuffer[E any](data unsafe.Pointer, size C.size_t) bool {
	var element E
	limit := uint64(math.MaxInt) / uint64(unsafe.Sizeof(element))
	return uint64(size) <= limit && (data != nil || size == 0)
}

// Operations shared by every slice wrapper. Each keeps Go semantics: element
// writes, reslices and appends are visible through every slice that shares the
// backing array. Indices are already normalized and bounds-checked by C++;
// Go checks them again so a bad call returns an error instead of panicking.

func inRange[E any](value []E, index C.size_t) bool {
	return uint64(index) < uint64(len(value))
}

func clone[E any](id C.uintptr_t) C.uintptr_t {
	return Pin(slices.Clone(Get[[]E](id)))
}

// reslice is Go's value[low:high]. It shares the backing array and keeps the
// remaining capacity, so a later append may overwrite value[high:].
func reslice[E any](id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	value := Get[[]E](id)
	if low > high || uint64(high) > uint64(len(value)) {
		return 0
	}
	return Pin(value[low:high])
}

// []int: elements are copied by value; Go int is intptr_t on the C side.

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

// [][]int: elements are Go-owned []int values, exchanged as SliceInt handles.

//export SBNewSliceSliceInt
func SBNewSliceSliceInt(ids unsafe.Pointer, size C.size_t) C.uintptr_t {
	if !validBuffer[C.uintptr_t](ids, size) {
		return 0
	}
	handles := unsafe.Slice((*C.uintptr_t)(ids), int(size))
	value := make([][]int, len(handles))
	for i, handle := range handles {
		if handle == 0 {
			return 0
		}
		// Shares the element's backing array; the new slice keeps it reachable.
		value[i] = Get[[]int](handle)
	}
	return Pin(value)
}

//export SBSliceSliceIntLen
func SBSliceSliceIntLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[][]int](id)))
}

// SBSliceSliceIntItems pins every element as a new SliceInt handle. The caller
// owns all written handles and must release each one.
//
//export SBSliceSliceIntItems
func SBSliceSliceIntItems(id C.uintptr_t, ids unsafe.Pointer, size C.size_t) C.int {
	value := Get[[][]int](id)
	if uint64(size) != uint64(len(value)) || !validBuffer[C.uintptr_t](ids, size) {
		return 1
	}
	handles := unsafe.Slice((*C.uintptr_t)(ids), len(value))
	for i, element := range value {
		handles[i] = Pin(element)
	}
	return 0
}

// SBSliceSliceIntAt pins the element header as a new SliceInt handle that
// shares the element's backing array. It returns 0 when index is out of range.
//
//export SBSliceSliceIntAt
func SBSliceSliceIntAt(id C.uintptr_t, index C.size_t) C.uintptr_t {
	value := Get[[][]int](id)
	if !inRange(value, index) {
		return 0
	}
	return Pin(value[index])
}

// SBSliceSliceIntSet stores the header of an existing SliceInt; the outer
// backing array then keeps that element's array reachable.
//
//export SBSliceSliceIntSet
func SBSliceSliceIntSet(id C.uintptr_t, index C.size_t, item C.uintptr_t) C.int {
	value := Get[[][]int](id)
	if !inRange(value, index) || item == 0 {
		return 1
	}
	value[index] = Get[[]int](item)
	return 0
}

// Clone is shallow like slices.Clone: elements still share their arrays.
//
//export SBSliceSliceIntClone
func SBSliceSliceIntClone(id C.uintptr_t) C.uintptr_t { return clone[[]int](id) }

//export SBSliceSliceIntSlice
func SBSliceSliceIntSlice(id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	return reslice[[]int](id, low, high)
}

//export SBConcatA
func SBConcatA(a C.uintptr_t, b C.uintptr_t) C.uintptr_t {
	return Pin(api.ConcatA(Get[[]int](a), Get[[]int](b)))
}

//export SBConcatB
func SBConcatB(a C.uintptr_t, b C.uintptr_t) C.uintptr_t {
	return Pin(api.ConcatB(Get[[][]int](a), Get[[][]int](b)))
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
