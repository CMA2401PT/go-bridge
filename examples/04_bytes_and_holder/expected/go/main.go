package main

/*
#include <stdint.h>
#include <stddef.h>
*/
import "C"

import (
	api "go-bridge/examples/04_bytes_and_holder"
	"math"
	"runtime"
	"unsafe"
)

//export SBNewBytes
func SBNewBytes(data unsafe.Pointer, size C.size_t) C.uintptr_t {
	if uint64(size) > math.MaxInt32 || (data == nil && size != 0) {
		return 0
	}
	return Pin(C.GoBytes(data, C.int(size)))
}

//export SBEchoBytes
func SBEchoBytes(id C.uintptr_t) C.uintptr_t {
	value := Get[[]byte](id)
	return Pin(api.RecvAndReturnBytes(value))
}

//export SBBytesLen
func SBBytesLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[]byte](id)))
}

//export SBCopyBytes
func SBCopyBytes(id C.uintptr_t, data unsafe.Pointer, size C.size_t) C.int {
	value := Get[[]byte](id)
	if uint64(size) != uint64(len(value)) || (data == nil && size != 0) {
		return 1
	}
	copy(unsafe.Slice((*byte)(data), len(value)), value)
	return 0
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
