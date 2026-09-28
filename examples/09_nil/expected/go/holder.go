package main

/*
#include <stdint.h>

// C restores the pointer representation; Go never round-trips a saved uintptr
// through unsafe.Pointer. Only Go dereferences the pinned value.
static inline void *ref_pointer(uintptr_t address) {
    return (void *)address;
}
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

// The table manages pin lifetimes only. Get does not look values up here.
var holders sync.Map // C.uintptr_t -> *runtime.Pinner

func Pin[T any](value T) C.uintptr_t {
	pinner := new(runtime.Pinner)
	pinner.Pin(&value)
	address := C.uintptr_t(uintptr(unsafe.Pointer(&value)))
	holders.Store(address, pinner)
	return address
}

func UnPin(address C.uintptr_t) {
	if entry, ok := holders.LoadAndDelete(address); ok {
		entry.(*runtime.Pinner).Unpin()
	}
}

// Handle 0 is the zero value of T and is never pinned; for a pointer T it is
// nil. Otherwise the caller must own a live Ref created by Pin[P], where P and
// T have identical underlying types (e.g. []int and a named slice of int), so
// both share one memory layout. Get returns a Go value; it never exposes that
// value's pointers to C/Python.
func Get[T any](address C.uintptr_t) T {
	if address == 0 {
		var zero T
		return zero
	}
	return *(*T)(C.ref_pointer(address))
}

// PinPointer is Pin for a pointer result: nil becomes handle 0 and pins
// nothing. The type parameter keeps the nil check on the typed pointer.
func PinPointer[E any](value *E) C.uintptr_t {
	if value == nil {
		return 0
	}
	return Pin(value)
}

func holderCount() int {
	count := 0
	holders.Range(func(_, _ any) bool { count++; return true })
	return count
}
