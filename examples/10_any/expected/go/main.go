package main

/*
#include <stdint.h>
#include <stdlib.h>
typedef struct { char *data; size_t size; } BrString;
typedef struct { uintptr_t value; uint8_t ok; } BrHandleResult;
typedef struct { intptr_t value; uint8_t ok; } BrIntResult;
typedef struct { int8_t value; uint8_t ok; } BrInt8Result;
typedef struct { int16_t value; uint8_t ok; } BrInt16Result;
typedef struct { int32_t value; uint8_t ok; } BrInt32Result;
typedef struct { int64_t value; uint8_t ok; } BrInt64Result;
typedef struct { uintptr_t value; uint8_t ok; } BrUintResult;
typedef struct { uint8_t value; uint8_t ok; } BrUint8Result;
typedef struct { uint16_t value; uint8_t ok; } BrUint16Result;
typedef struct { uint32_t value; uint8_t ok; } BrUint32Result;
typedef struct { uint64_t value; uint8_t ok; } BrUint64Result;
typedef struct { uintptr_t value; uint8_t ok; } BrUintptrResult;
typedef struct { float value; uint8_t ok; } BrFloat32Result;
typedef struct { double value; uint8_t ok; } BrFloat64Result;
typedef struct { uint8_t value; uint8_t ok; } BrBoolResult;
typedef struct { BrString value; uint8_t ok; } BrStringResult;
*/
import "C"

import (
	api "go-bridge/examples/10_any"
	"unsafe"
)

func inputString(value C.BrString) string {
	return string(unsafe.Slice((*byte)(unsafe.Pointer(value.data)), int(value.size)))
}

func outputString(value string) C.BrString {
	// Even an empty successful string owns a C allocation. No Go pointers
	// cross the ABI, and the C++ result owner frees it after decoding.
	data := C.malloc(C.size_t(len(value)) + 1)
	copy(unsafe.Slice((*byte)(data), len(value)), value)
	return C.BrString{data: (*C.char)(data), size: C.size_t(len(value))}
}

//export BrIntAsAny
func BrIntAsAny(value C.intptr_t) C.uintptr_t {
	return bridgePinAny(int(value))
}

//export BrTryAnyAsInt
func BrTryAnyAsInt(handle C.uintptr_t) C.BrIntResult {
	value, ok := bridgeAny(handle).(int)
	if !ok {
		return C.BrIntResult{}
	}
	return C.BrIntResult{value: C.intptr_t(value), ok: 1}
}

//export BrInt8AsAny
func BrInt8AsAny(value C.int8_t) C.uintptr_t {
	return bridgePinAny(int8(value))
}

//export BrTryAnyAsInt8
func BrTryAnyAsInt8(handle C.uintptr_t) C.BrInt8Result {
	value, ok := bridgeAny(handle).(int8)
	if !ok {
		return C.BrInt8Result{}
	}
	return C.BrInt8Result{value: C.int8_t(value), ok: 1}
}

//export BrInt16AsAny
func BrInt16AsAny(value C.int16_t) C.uintptr_t {
	return bridgePinAny(int16(value))
}

//export BrTryAnyAsInt16
func BrTryAnyAsInt16(handle C.uintptr_t) C.BrInt16Result {
	value, ok := bridgeAny(handle).(int16)
	if !ok {
		return C.BrInt16Result{}
	}
	return C.BrInt16Result{value: C.int16_t(value), ok: 1}
}

//export BrInt32AsAny
func BrInt32AsAny(value C.int32_t) C.uintptr_t {
	return bridgePinAny(int32(value))
}

//export BrTryAnyAsInt32
func BrTryAnyAsInt32(handle C.uintptr_t) C.BrInt32Result {
	value, ok := bridgeAny(handle).(int32)
	if !ok {
		return C.BrInt32Result{}
	}
	return C.BrInt32Result{value: C.int32_t(value), ok: 1}
}

//export BrInt64AsAny
func BrInt64AsAny(value C.int64_t) C.uintptr_t {
	return bridgePinAny(int64(value))
}

//export BrTryAnyAsInt64
func BrTryAnyAsInt64(handle C.uintptr_t) C.BrInt64Result {
	value, ok := bridgeAny(handle).(int64)
	if !ok {
		return C.BrInt64Result{}
	}
	return C.BrInt64Result{value: C.int64_t(value), ok: 1}
}

//export BrUintAsAny
func BrUintAsAny(value C.uintptr_t) C.uintptr_t {
	return bridgePinAny(uint(value))
}

//export BrTryAnyAsUint
func BrTryAnyAsUint(handle C.uintptr_t) C.BrUintResult {
	value, ok := bridgeAny(handle).(uint)
	if !ok {
		return C.BrUintResult{}
	}
	return C.BrUintResult{value: C.uintptr_t(value), ok: 1}
}

//export BrUint8AsAny
func BrUint8AsAny(value C.uint8_t) C.uintptr_t {
	return bridgePinAny(uint8(value))
}

//export BrTryAnyAsUint8
func BrTryAnyAsUint8(handle C.uintptr_t) C.BrUint8Result {
	value, ok := bridgeAny(handle).(uint8)
	if !ok {
		return C.BrUint8Result{}
	}
	return C.BrUint8Result{value: C.uint8_t(value), ok: 1}
}

//export BrUint16AsAny
func BrUint16AsAny(value C.uint16_t) C.uintptr_t {
	return bridgePinAny(uint16(value))
}

//export BrTryAnyAsUint16
func BrTryAnyAsUint16(handle C.uintptr_t) C.BrUint16Result {
	value, ok := bridgeAny(handle).(uint16)
	if !ok {
		return C.BrUint16Result{}
	}
	return C.BrUint16Result{value: C.uint16_t(value), ok: 1}
}

//export BrUint32AsAny
func BrUint32AsAny(value C.uint32_t) C.uintptr_t {
	return bridgePinAny(uint32(value))
}

//export BrTryAnyAsUint32
func BrTryAnyAsUint32(handle C.uintptr_t) C.BrUint32Result {
	value, ok := bridgeAny(handle).(uint32)
	if !ok {
		return C.BrUint32Result{}
	}
	return C.BrUint32Result{value: C.uint32_t(value), ok: 1}
}

//export BrUint64AsAny
func BrUint64AsAny(value C.uint64_t) C.uintptr_t {
	return bridgePinAny(uint64(value))
}

//export BrTryAnyAsUint64
func BrTryAnyAsUint64(handle C.uintptr_t) C.BrUint64Result {
	value, ok := bridgeAny(handle).(uint64)
	if !ok {
		return C.BrUint64Result{}
	}
	return C.BrUint64Result{value: C.uint64_t(value), ok: 1}
}

//export BrUintptrAsAny
func BrUintptrAsAny(value C.uintptr_t) C.uintptr_t {
	return bridgePinAny(uintptr(value))
}

//export BrTryAnyAsUintptr
func BrTryAnyAsUintptr(handle C.uintptr_t) C.BrUintptrResult {
	value, ok := bridgeAny(handle).(uintptr)
	if !ok {
		return C.BrUintptrResult{}
	}
	return C.BrUintptrResult{value: C.uintptr_t(value), ok: 1}
}

//export BrFloat32AsAny
func BrFloat32AsAny(value C.float) C.uintptr_t {
	return bridgePinAny(float32(value))
}

//export BrTryAnyAsFloat32
func BrTryAnyAsFloat32(handle C.uintptr_t) C.BrFloat32Result {
	value, ok := bridgeAny(handle).(float32)
	if !ok {
		return C.BrFloat32Result{}
	}
	return C.BrFloat32Result{value: C.float(value), ok: 1}
}

//export BrFloat64AsAny
func BrFloat64AsAny(value C.double) C.uintptr_t {
	return bridgePinAny(float64(value))
}

//export BrTryAnyAsFloat64
func BrTryAnyAsFloat64(handle C.uintptr_t) C.BrFloat64Result {
	value, ok := bridgeAny(handle).(float64)
	if !ok {
		return C.BrFloat64Result{}
	}
	return C.BrFloat64Result{value: C.double(value), ok: 1}
}

//export BrBoolAsAny
func BrBoolAsAny(value C.uint8_t) C.uintptr_t {
	return bridgePinAny(value != 0)
}

//export BrTryAnyAsBool
func BrTryAnyAsBool(handle C.uintptr_t) C.BrBoolResult {
	value, ok := bridgeAny(handle).(bool)
	if !ok {
		return C.BrBoolResult{}
	}
	var result C.uint8_t
	if value {
		result = 1
	}
	return C.BrBoolResult{value: result, ok: 1}
}

//export BrStringAsAny
func BrStringAsAny(value C.BrString) C.uintptr_t {
	return bridgePinAny(inputString(value))
}

//export BrTryAnyAsString
func BrTryAnyAsString(handle C.uintptr_t) C.BrStringResult {
	value, ok := bridgeAny(handle).(string)
	if !ok {
		return C.BrStringResult{}
	}
	return C.BrStringResult{value: outputString(value), ok: 1}
}

//export BrMyIntAsAny
func BrMyIntAsAny(value C.intptr_t) C.uintptr_t { return bridgePinAny(api.MyInt(value)) }

//export BrTryAnyAsMyInt
func BrTryAnyAsMyInt(handle C.uintptr_t) C.BrIntResult {
	value, ok := bridgeAny(handle).(api.MyInt)
	if !ok {
		return C.BrIntResult{}
	}
	return C.BrIntResult{value: C.intptr_t(value), ok: 1}
}

//export BrSliceIntAsAny
func BrSliceIntAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(bridgeGet[[]int](handle))
}

//export BrTryAnyAsSliceInt
func BrTryAnyAsSliceInt(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).([]int)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePin(value), ok: 1}
}

//export BrMySliceAsAny
func BrMySliceAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(api.MySlice(bridgeGet[[]int](handle)))
}

//export BrTryAnyAsMySlice
func BrTryAnyAsMySlice(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).(api.MySlice)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePin([]int(value)), ok: 1}
}

//export BrPointAsAny
func BrPointAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(*bridgeGet[*api.Point](handle))
}

//export BrTryAnyAsPoint
func BrTryAnyAsPoint(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).(api.Point)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePin(&value), ok: 1}
}

//export BrPtrIntAsAny
func BrPtrIntAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(bridgePointer[*int](handle))
}

//export BrTryAnyAsPtrInt
func BrTryAnyAsPtrInt(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).(*int)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePinPointer(value), ok: 1}
}

//export BrPtrPointAsAny
func BrPtrPointAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(bridgePointer[*api.Point](handle))
}

//export BrTryAnyAsPtrPoint
func BrTryAnyAsPtrPoint(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).(*api.Point)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePinPointer(value), ok: 1}
}

//export BrPtrPtrIntAsAny
func BrPtrPtrIntAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(bridgePointer[**int](handle))
}

//export BrTryAnyAsPtrPtrInt
func BrTryAnyAsPtrPtrInt(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).(**int)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePinPointer(value), ok: 1}
}

//export BrIntPtrAsAny
func BrIntPtrAsAny(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(api.IntPtr(bridgePointer[*int](handle)))
}

//export BrTryAnyAsIntPtr
func BrTryAnyAsIntPtr(handle C.uintptr_t) C.BrHandleResult {
	value, ok := bridgeAny(handle).(api.IntPtr)
	if !ok {
		return C.BrHandleResult{}
	}
	return C.BrHandleResult{value: bridgePinPointer((*int)(value)), ok: 1}
}

//export BrCopySlice
func BrCopySlice(handle C.uintptr_t) C.uintptr_t { return bridgePin(bridgeGet[[]int](handle)) }

//export BrNewSlice
func BrNewSlice(data *C.intptr_t, size C.size_t) C.uintptr_t {
	values := make([]int, int(size))
	for i, value := range unsafe.Slice(data, int(size)) {
		values[i] = int(value)
	}
	return bridgePin(values)
}

//export BrSliceLen
func BrSliceLen(handle C.uintptr_t) C.size_t { return C.size_t(len(bridgeGet[[]int](handle))) }

//export BrSliceGet
func BrSliceGet(handle C.uintptr_t, index C.size_t) C.intptr_t {
	return C.intptr_t(bridgeGet[[]int](handle)[int(index)])
}

//export BrSliceSet
func BrSliceSet(handle C.uintptr_t, index C.size_t, value C.intptr_t) {
	bridgeGet[[]int](handle)[int(index)] = int(value)
}

//export BrSliceCopyToPy
func BrSliceCopyToPy(handle C.uintptr_t, output *C.intptr_t) {
	values := bridgeGet[[]int](handle)
	target := unsafe.Slice(output, len(values))
	for i, value := range values {
		target[i] = C.intptr_t(value)
	}
}

//export BrNewPoint
func BrNewPoint(x C.intptr_t, values C.uintptr_t, data C.uintptr_t) C.uintptr_t {
	value := api.Point{X: int(x), Data: bridgeAny(data)}
	if values != 0 {
		value.Values = bridgeGet[[]int](values)
	}
	return bridgePin(&value)
}

//export BrPointCopy
func BrPointCopy(handle C.uintptr_t) C.uintptr_t {
	value := *bridgeGet[*api.Point](handle)
	return bridgePin(&value)
}

//export BrPointView
func BrPointView(handle C.uintptr_t) C.uintptr_t {
	value := bridgePointer[*api.Point](handle)
	_ = *value // nil dereference must panic, rather than create a nil struct.
	return bridgePin(value)
}

//export BrPointX
func BrPointX(handle C.uintptr_t) C.intptr_t { return C.intptr_t(bridgePointer[*api.Point](handle).X) }

//export BrPointSetX
func BrPointSetX(handle C.uintptr_t, x C.intptr_t) { bridgePointer[*api.Point](handle).X = int(x) }

//export BrPointValues
func BrPointValues(handle C.uintptr_t) C.uintptr_t {
	return bridgePin(bridgePointer[*api.Point](handle).Values)
}

//export BrPointSetValues
func BrPointSetValues(handle, values C.uintptr_t) {
	bridgePointer[*api.Point](handle).Values = bridgeGet[[]int](values)
}

//export BrPointData
func BrPointData(handle C.uintptr_t) C.uintptr_t {
	return bridgePinAny(bridgePointer[*api.Point](handle).Data)
}

//export BrPointSetData
func BrPointSetData(handle, data C.uintptr_t) {
	bridgePointer[*api.Point](handle).Data = bridgeAny(data)
}

//export BrCopyPointer
func BrCopyPointer(handle C.uintptr_t) C.uintptr_t {
	return bridgePinPointer(bridgePointer[*int](handle))
}

//export BrNewPtrInt
func BrNewPtrInt(value C.intptr_t) C.uintptr_t { v := int(value); return bridgePin(&v) }

//export BrPtrIntLoad
func BrPtrIntLoad(handle C.uintptr_t) C.intptr_t { return C.intptr_t(*bridgePointer[*int](handle)) }

//export BrPtrIntStore
func BrPtrIntStore(handle C.uintptr_t, value C.intptr_t) { *bridgePointer[*int](handle) = int(value) }

//export BrNewPtrPtrInt
func BrNewPtrPtrInt(handle C.uintptr_t) C.uintptr_t {
	value := bridgePointer[*int](handle)
	return bridgePin(&value)
}

//export BrPtrPtrIntLoad
func BrPtrPtrIntLoad(handle C.uintptr_t) C.uintptr_t {
	return bridgePinPointer(*bridgePointer[**int](handle))
}

//export BrPtrPtrIntStore
func BrPtrPtrIntStore(handle, value C.uintptr_t) {
	*bridgePointer[**int](handle) = bridgePointer[*int](value)
}

//export BrPtrPointStore
func BrPtrPointStore(handle, value C.uintptr_t) {
	*bridgePointer[*api.Point](handle) = *bridgeGet[*api.Point](value)
}

//export BrEcho
func BrEcho(handle C.uintptr_t) C.uintptr_t { return bridgePinAny(api.Echo(bridgeAny(handle))) }

//export BrDynamicKind
func BrDynamicKind(handle C.uintptr_t) C.BrString {
	return outputString(api.DynamicKind(bridgeAny(handle)))
}

//export BrNilAny
func BrNilAny() C.uintptr_t { return bridgePinAny(api.NilAny()) }

//export BrNilPointer
func BrNilPointer() C.uintptr_t { return bridgePinAny(api.NilPointer()) }

//export BrNilSlice
func BrNilSlice() C.uintptr_t { return bridgePinAny(api.NilSlice()) }

//export BrEmptySlice
func BrEmptySlice() C.uintptr_t { return bridgePinAny(api.EmptySlice()) }

//export BrUnknown
func BrUnknown() C.uintptr_t { return bridgePinAny(api.Unknown()) }

//export BrSliceIsNil
func BrSliceIsNil(handle C.uintptr_t) C.uint8_t {
	if api.SliceIsNil(bridgeAny(handle)) {
		return 1
	}
	return 0
}

//export BrInvalidString
func BrInvalidString() C.uintptr_t { return bridgePinAny(api.InvalidString()) }

//export BrUnknownValue
func BrUnknownValue(handle C.uintptr_t) C.intptr_t {
	return C.intptr_t(api.UnknownValue(bridgeAny(handle)))
}

//export BrEchoMyInt
func BrEchoMyInt(value C.intptr_t) C.intptr_t { return C.intptr_t(api.EchoMyInt(api.MyInt(value))) }

//export BrEchoMySlice
func BrEchoMySlice(handle C.uintptr_t) C.uintptr_t {
	return bridgePin([]int(api.EchoMySlice(api.MySlice(bridgeGet[[]int](handle)))))
}

//export BrEchoIntPtr
func BrEchoIntPtr(handle C.uintptr_t) C.uintptr_t {
	return bridgePinPointer((*int)(api.EchoIntPtr(api.IntPtr(bridgePointer[*int](handle)))))
}

//export BrEchoPoint
func BrEchoPoint(handle C.uintptr_t) C.uintptr_t {
	value := api.EchoPoint(*bridgeGet[*api.Point](handle))
	return bridgePin(&value)
}

//export BrEchoDoublePointer
func BrEchoDoublePointer(handle C.uintptr_t) C.uintptr_t {
	return bridgePinPointer(api.EchoDoublePointer(bridgePointer[**int](handle)))
}

func main() {}
