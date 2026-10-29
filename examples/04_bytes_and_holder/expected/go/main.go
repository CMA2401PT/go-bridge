package main

/*
#include <stdint.h>
#include <stddef.h>
*/
import "C"

import (
	api "go-bridge/examples/04_bytes_and_holder"
	"runtime"
	"unsafe"
)

//export BrNewBytes
func BrNewBytes(data unsafe.Pointer, size C.size_t) C.uintptr_t {
	return Pin(append([]byte(nil), unsafe.Slice((*byte)(data), int(size))...))
}

//export BrEchoBytes
func BrEchoBytes(id C.uintptr_t) C.uintptr_t {
	value := Get[[]byte](id)
	return Pin(api.RecvAndReturnBytes(value))
}

//export BrBytesLen
func BrBytesLen(id C.uintptr_t) C.size_t {
	return C.size_t(len(Get[[]byte](id)))
}

//export BrCopyBytes
func BrCopyBytes(id C.uintptr_t, data unsafe.Pointer) {
	value := Get[[]byte](id)
	copy(unsafe.Slice((*byte)(data), len(value)), value)
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
