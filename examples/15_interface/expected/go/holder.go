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

// 句柄让 Go 值在 Python 包装存活期间保持可达，计数用于验证释放责任。
func bridgePin[T any](value T) C.uintptr_t {
	handle := cgo.NewHandle(value)
	bridgeRoots.Add(1)
	return C.uintptr_t(handle)
}

func bridgeGet[T any](handle C.uintptr_t) T {
	return cgo.Handle(handle).Value().(T)
}

// 零句柄表示目标类型的零值，让 nil 接口与 nil 指针共用 ABI 表示。
func bridgePointer[T any](handle C.uintptr_t) T {
	if handle == 0 {
		var zero T
		return zero
	}
	return bridgeGet[T](handle)
}

// 移动后的 C++ 包装也会析构，因此零句柄无需释放。
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
