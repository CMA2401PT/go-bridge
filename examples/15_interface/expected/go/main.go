package main

/*
#include <stdint.h>
#include <stddef.h>
#include <stdlib.h>
typedef struct {char *data;size_t size;} BrString;
typedef struct {intptr_t value;uintptr_t error;} BrResult;
*/
import "C"
import api "go-bridge/examples/15_interface"

func pin(value any) C.uintptr_t {
	if value == nil {
		return 0
	}
	return bridgePin(value)
}

//export BrNew
func BrNew(start C.intptr_t) C.uintptr_t { return pin(api.New(int(start))) }

//export BrAdd
func BrAdd(handle C.uintptr_t, delta C.intptr_t) C.intptr_t {
	return C.intptr_t(bridgePointer[api.Counter](handle).Add(int(delta)))
}

//export BrEcho
func BrEcho(handle C.uintptr_t) C.uintptr_t { return pin(api.Echo(bridgePointer[api.Counter](handle))) }

//export BrNil
func BrNil() C.uintptr_t { return pin(api.Nil()) }

//export BrTypedNil
func BrTypedNil() C.uintptr_t { return pin(api.TypedNil()) }

//export BrResultValue
func BrResultValue(fail C.uint8_t) C.BrResult {
	v, e := api.Result(fail != 0)
	return C.BrResult{value: C.intptr_t(v), error: pin(e)}
}

//export BrError
func BrError(handle C.uintptr_t) C.BrString {
	v := bridgePointer[error](handle).Error()
	return C.BrString{data: C.CString(v), size: C.size_t(len(v))}
}
func main() {}
