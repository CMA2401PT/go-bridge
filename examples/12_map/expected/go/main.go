package main

/*
#include <stdint.h>
#include <stdlib.h>
typedef struct { char *data; size_t size; } BrString;
typedef struct { uint8_t ok; intptr_t value; } BrLookup;
*/
import "C"
import (
	api "go-bridge/examples/12_map"
	"unsafe"
)

func text(value C.BrString) string {
	return string(unsafe.Slice((*byte)(unsafe.Pointer(value.data)), int(value.size)))
}
func pin(value map[string]int) C.uintptr_t {
	if value == nil {
		return 0
	}
	return bridgePin(value)
}

//export BrNew
func BrNew(keys *C.BrString, values *C.intptr_t, size C.size_t) C.uintptr_t {
	result := make(map[string]int, int(size))
	input := unsafe.Slice(values, int(size))
	for i, k := range unsafe.Slice(keys, int(size)) {
		result[text(k)] = int(input[i])
	}
	return pin(result)
}

//export BrGet
func BrGet(h C.uintptr_t, key C.BrString) (out C.BrLookup) {
	value, ok := bridgePointer[map[string]int](h)[text(key)]
	if ok {
		out.ok = 1
		out.value = C.intptr_t(value)
	}
	return
}

//export BrSet
func BrSet(h C.uintptr_t, key C.BrString, value C.intptr_t) {
	bridgePointer[map[string]int](h)[text(key)] = int(value)
}

//export BrDelete
func BrDelete(h C.uintptr_t, key C.BrString) { delete(bridgePointer[map[string]int](h), text(key)) }

//export BrClear
func BrClear(h C.uintptr_t) { clear(bridgePointer[map[string]int](h)) }

//export BrLen
func BrLen(h C.uintptr_t) C.size_t { return C.size_t(len(bridgePointer[map[string]int](h))) }

//export BrClone
func BrClone(h C.uintptr_t) C.uintptr_t {
	value := bridgePointer[map[string]int](h)
	if value == nil {
		return 0
	}
	result := make(map[string]int, len(value))
	for k, v := range value {
		result[k] = v
	}
	return pin(result)
}

//export BrItems
func BrItems(h C.uintptr_t, keys *C.BrString, values *C.intptr_t) {
	value := bridgePointer[map[string]int](h)
	ks, vs := unsafe.Slice(keys, len(value)), unsafe.Slice(values, len(value))
	i := 0
	for k, v := range value {
		ks[i] = C.BrString{data: C.CString(k), size: C.size_t(len(k))}
		vs[i] = C.intptr_t(v)
		i++
	}
}

//export BrEcho
func BrEcho(h C.uintptr_t) C.uintptr_t { return pin(api.Echo(bridgePointer[map[string]int](h))) }

//export BrPut
func BrPut(h C.uintptr_t, key C.BrString, value C.intptr_t) {
	api.Put(bridgePointer[map[string]int](h), text(key), int(value))
}

//export BrNil
func BrNil() C.uintptr_t { return pin(api.Nil()) }
func main()              {}
