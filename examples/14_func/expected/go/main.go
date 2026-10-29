package main

/*
#include <stdint.h>
*/
import "C"
import api "go-bridge/examples/14_func"

func pin(value func(int) int) C.uintptr_t {
	if value == nil {
		return 0
	}
	return bridgePin(value)
}

//export BrCounter
func BrCounter(start C.intptr_t) C.uintptr_t { return pin(api.Counter(int(start))) }

//export BrCall
func BrCall(handle C.uintptr_t, delta C.intptr_t) C.intptr_t {
	return C.intptr_t(bridgePointer[func(int) int](handle)(int(delta)))
}

//export BrApply
func BrApply(handle C.uintptr_t, delta C.intptr_t) C.intptr_t {
	return C.intptr_t(api.Apply(bridgePointer[func(int) int](handle), int(delta)))
}

//export BrEcho
func BrEcho(handle C.uintptr_t) C.uintptr_t {
	return pin(api.Echo(bridgePointer[func(int) int](handle)))
}

//export BrNil
func BrNil() C.uintptr_t { return pin(api.Nil()) }
func main()              {}
