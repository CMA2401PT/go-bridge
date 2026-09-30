package main

/*
#include <stdint.h>
*/
import "C"

import (
	"runtime"
	"runtime/cgo"
	"sync/atomic"
)

var bridgeRoots atomic.Int64

func bridgePin[T any](value T) C.uintptr_t {
	handle := cgo.NewHandle(value)
	bridgeRoots.Add(1)
	return C.uintptr_t(handle)
}

func bridgeGet[T any](handle C.uintptr_t) T {
	return cgo.Handle(handle).Value().(T)
}

func bridgePointer[T any](handle C.uintptr_t) T {
	if handle == 0 {
		var zero T
		return zero
	}
	return bridgeGet[T](handle)
}

func bridgePinPointer[T any](value *T) C.uintptr_t {
	if value == nil {
		return 0
	}
	return bridgePin(value)
}

func bridgeAny(handle C.uintptr_t) any {
	if handle == 0 {
		return nil
	}
	return cgo.Handle(handle).Value()
}

func bridgePinAny(value any) C.uintptr_t {
	if value == nil {
		return 0
	}
	return bridgePin(value)
}

//export BrRelease
func BrRelease(handle C.uintptr_t) {
	if handle == 0 {
		return
	}
	cgo.Handle(handle).Delete()
	bridgeRoots.Add(-1)
}

//export BrHolderCount
func BrHolderCount() C.int64_t { return C.int64_t(bridgeRoots.Load()) }

//export BrCollect
func BrCollect() { runtime.GC() }
