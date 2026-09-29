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

//export BrNewSliceInt
func BrNewSliceInt(data unsafe.Pointer, size C.size_t) C.uintptr_t {
	if !validBuffer[int](data, size) {
		return 0
	}
	value := make([]int, int(size))
	copy(value, unsafe.Slice((*int)(data), int(size)))
	return Pin(value)
}

//export BrSliceIntLen
func BrSliceIntLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[]int](id)))
}

//export BrCopySliceInt
func BrCopySliceInt(id C.uintptr_t, data unsafe.Pointer, size C.size_t) C.int {
	value := Get[[]int](id)
	if uint64(size) != uint64(len(value)) || !validBuffer[int](data, size) {
		return 1
	}
	copy(unsafe.Slice((*int)(data), len(value)), value)
	return 0
}

//export BrSliceIntAt
func BrSliceIntAt(id C.uintptr_t, index C.size_t, out unsafe.Pointer) C.int {
	value := Get[[]int](id)
	if !inRange(value, index) || out == nil {
		return 1
	}
	*(*int)(out) = value[index]
	return 0
}

//export BrSliceIntSet
func BrSliceIntSet(id C.uintptr_t, index C.size_t, item C.intptr_t) C.int {
	value := Get[[]int](id)
	if !inRange(value, index) {
		return 1
	}
	value[index] = int(item)
	return 0
}

//export BrSliceIntClone
func BrSliceIntClone(id C.uintptr_t) C.uintptr_t { return clone[int](id) }

//export BrSliceIntSlice
func BrSliceIntSlice(id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	return reslice[int](id, low, high)
}

// [][]int: elements are Go-owned []int values, exchanged as SliceInt handles.

//export BrNewSliceSliceInt
func BrNewSliceSliceInt(ids unsafe.Pointer, size C.size_t) C.uintptr_t {
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

//export BrSliceSliceIntLen
func BrSliceSliceIntLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[][]int](id)))
}

// BrSliceSliceIntItems pins every element as a new SliceInt handle. The caller
// owns all written handles and must release each one.
//
//export BrSliceSliceIntItems
func BrSliceSliceIntItems(id C.uintptr_t, ids unsafe.Pointer, size C.size_t) C.int {
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

// BrSliceSliceIntAt pins the element header as a new SliceInt handle that
// shares the element's backing array. It returns 0 when index is out of range.
//
//export BrSliceSliceIntAt
func BrSliceSliceIntAt(id C.uintptr_t, index C.size_t) C.uintptr_t {
	value := Get[[][]int](id)
	if !inRange(value, index) {
		return 0
	}
	return Pin(value[index])
}

// BrSliceSliceIntSet stores the header of an existing SliceInt; the outer
// backing array then keeps that element's array reachable.
//
//export BrSliceSliceIntSet
func BrSliceSliceIntSet(id C.uintptr_t, index C.size_t, item C.uintptr_t) C.int {
	value := Get[[][]int](id)
	if !inRange(value, index) || item == 0 {
		return 1
	}
	value[index] = Get[[]int](item)
	return 0
}

// Clone is shallow like slices.Clone: elements still share their arrays.
//
//export BrSliceSliceIntClone
func BrSliceSliceIntClone(id C.uintptr_t) C.uintptr_t { return clone[[]int](id) }

//export BrSliceSliceIntSlice
func BrSliceSliceIntSlice(id C.uintptr_t, low, high C.size_t) C.uintptr_t {
	return reslice[[]int](id, low, high)
}

//export BrConcatA
func BrConcatA(a C.uintptr_t, b C.uintptr_t) C.uintptr_t {
	return Pin(api.ConcatA(Get[[]int](a), Get[[]int](b)))
}

//export BrConcatB
func BrConcatB(a C.uintptr_t, b C.uintptr_t) C.uintptr_t {
	return Pin(api.ConcatB(Get[[][]int](a), Get[[][]int](b)))
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
