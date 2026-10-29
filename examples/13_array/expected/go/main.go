package main

/*
#include <stdint.h>
#include <stddef.h>
*/
import "C"
import (
	api "go-bridge/examples/13_array"
	"unsafe"
)

func pin(value [3]int) C.uintptr_t { return bridgePin(&value) }

//export BrZero
func BrZero() C.uintptr_t { return pin([3]int{}) }

//export BrNew
func BrNew(data *C.intptr_t) C.uintptr_t {
	var value [3]int
	for i, v := range unsafe.Slice(data, 3) {
		value[i] = int(v)
	}
	return pin(value)
}

//export BrAt
func BrAt(handle C.uintptr_t, index C.size_t) C.intptr_t {
	return C.intptr_t(bridgeGet[*[3]int](handle)[int(index)])
}

//export BrSet
func BrSet(handle C.uintptr_t, index C.size_t, value C.intptr_t) {
	bridgeGet[*[3]int](handle)[int(index)] = int(value)
}

//export BrCopy
func BrCopy(handle C.uintptr_t) C.uintptr_t { return pin(*bridgeGet[*[3]int](handle)) }

//export BrAddress
func BrAddress(handle C.uintptr_t) C.uintptr_t { return bridgePin(bridgeGet[*[3]int](handle)) }

//export BrItems
func BrItems(handle C.uintptr_t, data *C.intptr_t) {
	for i, v := range bridgeGet[*[3]int](handle) {
		unsafe.Slice(data, 3)[i] = C.intptr_t(v)
	}
}

//export BrEcho
func BrEcho(handle C.uintptr_t) C.uintptr_t { return pin(api.Echo(*bridgeGet[*[3]int](handle))) }

//export BrIncrement
func BrIncrement(handle C.uintptr_t) C.uintptr_t {
	return pin(api.Increment(*bridgeGet[*[3]int](handle)))
}

//export BrMutate
func BrMutate(handle C.uintptr_t) { api.Mutate(bridgePointer[*[3]int](handle)) }
func main()                       {}
