package main

/*
#include <stdint.h>
#include <stddef.h>
#include <stdlib.h>
typedef struct {char *data;size_t size;} BrString;
typedef struct {BrString value;uintptr_t error;} BrFeedResult;
typedef struct {uint8_t ok;uintptr_t value;} BrAssertion;
*/
import "C"

import api "go-bridge/examples/15_interface"

// nil 接口用零句柄表示；装着 nil 指针的接口仍有动态类型，必须保留。
func pin(value any) C.uintptr_t {
	if value == nil {
		return 0
	}
	return bridgePin(value)
}

// 返回的字符串由 C++ 释放，Go 不把自己的字符串内存交给 Python。
func cString(value string) C.BrString {
	return C.BrString{data: C.CString(value), size: C.size_t(len(value))}
}

// Python 的结构体包装持有 *T；断言得到的 T 要放进独立变量，保持值复制语义。
func assertStruct[T any](handle C.uintptr_t) (out C.BrAssertion) {
	value, ok := bridgePointer[any](handle).(T)
	if ok {
		out.ok = 1
		out.value = bridgePin(&value)
	}
	return
}

// 断言成功与指针是否为 nil 是两件事，ok 不能由句柄是否为零推断。
func assertPointer[T any](handle C.uintptr_t) (out C.BrAssertion) {
	value, ok := bridgePointer[any](handle).(*T)
	if ok {
		out.ok = 1
		if value != nil {
			out.value = bridgePin(value)
		}
	}
	return
}

//export BrNewDog
func BrNewDog(name C.BrString) C.uintptr_t {
	return bridgePin(&api.Dog{Name: C.GoStringN(name.data, C.int(name.size))})
}

//export BrNewCat
func BrNewCat(name C.BrString) C.uintptr_t {
	return bridgePin(&api.Cat{Name: C.GoStringN(name.data, C.int(name.size))})
}

//export BrDogName
func BrDogName(handle C.uintptr_t) C.BrString {
	return cString(bridgeGet[*api.Dog](handle).Name)
}

//export BrSetDogName
func BrSetDogName(handle C.uintptr_t, name C.BrString) {
	bridgeGet[*api.Dog](handle).Name = C.GoStringN(name.data, C.int(name.size))
}

//export BrCatName
func BrCatName(handle C.uintptr_t) C.BrString {
	return cString(bridgeGet[*api.Cat](handle).Name)
}

//export BrSetCatName
func BrSetCatName(handle C.uintptr_t, name C.BrString) {
	bridgeGet[*api.Cat](handle).Name = C.GoStringN(name.data, C.int(name.size))
}

// addr() 为同一变量建立新句柄，让指针与原包装各自管理生命周期。
//
//export BrDogAddr
func BrDogAddr(handle C.uintptr_t) C.uintptr_t {
	return bridgePin(bridgeGet[*api.Dog](handle))
}

//export BrCatAddr
func BrCatAddr(handle C.uintptr_t) C.uintptr_t {
	return bridgePin(bridgeGet[*api.Cat](handle))
}

// Dog 作为接口值时复制；*Dog、*Cat 作为接口值时共享原变量。
//
//export BrBoxDog
func BrBoxDog(handle C.uintptr_t) C.uintptr_t {
	return pin(*bridgeGet[*api.Dog](handle))
}

//export BrBoxPtrDog
func BrBoxPtrDog(handle C.uintptr_t) C.uintptr_t {
	return pin(bridgePointer[*api.Dog](handle))
}

//export BrBoxPtrCat
func BrBoxPtrCat(handle C.uintptr_t) C.uintptr_t {
	return pin(bridgePointer[*api.Cat](handle))
}

//export BrBox
func BrBox(handle C.uintptr_t) C.uintptr_t { return pin(bridgePointer[any](handle)) }

//export BrAssertAnimal
func BrAssertAnimal(handle C.uintptr_t) (out C.BrAssertion) {
	value, ok := bridgePointer[any](handle).(api.Animal)
	if ok {
		out.ok = 1
		out.value = pin(value)
	}
	return
}

//export BrAssertDog
func BrAssertDog(handle C.uintptr_t) C.BrAssertion { return assertStruct[api.Dog](handle) }

//export BrAssertCat
func BrAssertCat(handle C.uintptr_t) C.BrAssertion { return assertStruct[api.Cat](handle) }

//export BrAssertPtrDog
func BrAssertPtrDog(handle C.uintptr_t) C.BrAssertion { return assertPointer[api.Dog](handle) }

//export BrAssertPtrCat
func BrAssertPtrCat(handle C.uintptr_t) C.BrAssertion { return assertPointer[api.Cat](handle) }

//export BrAssertError
func BrAssertError(handle C.uintptr_t) (out C.BrAssertion) {
	value, ok := bridgePointer[any](handle).(error)
	if ok {
		out.ok = 1
		out.value = pin(value)
	}
	return
}

//export BrAdopt
func BrAdopt(kind, name C.BrString) C.uintptr_t {
	return pin(api.Adopt(
		C.GoStringN(kind.data, C.int(kind.size)),
		C.GoStringN(name.data, C.int(name.size)),
	))
}

//export BrAnimalKind
func BrAnimalKind(handle C.uintptr_t) C.BrString {
	return cString(bridgePointer[api.Animal](handle).Kind())
}

//export BrAnimalSpeak
func BrAnimalSpeak(handle C.uintptr_t, times C.intptr_t) C.BrString {
	return cString(bridgePointer[api.Animal](handle).Speak(int(times)))
}

//export BrIntroduce
func BrIntroduce(handle C.uintptr_t) C.BrString {
	return cString(api.Introduce(bridgePointer[api.Animal](handle)))
}

//export BrNobody
func BrNobody() C.uintptr_t { return pin(api.Nobody()) }

//export BrStray
func BrStray() C.uintptr_t { return pin(api.Stray()) }

// error 与 Animal 一样跨 ABI 传递，保留为普通接口包装。
//
//export BrFeed
func BrFeed(handle C.uintptr_t, food C.BrString) C.BrFeedResult {
	value, err := api.Feed(
		bridgePointer[api.Animal](handle),
		C.GoStringN(food.data, C.int(food.size)),
	)
	return C.BrFeedResult{value: cString(value), error: pin(err)}
}

//export BrError
func BrError(handle C.uintptr_t) C.BrString {
	return cString(bridgePointer[error](handle).Error())
}

func main() {}
