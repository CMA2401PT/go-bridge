package main

/*
#include <stdint.h>
#include <stdlib.h>
typedef struct { char *data; size_t size; } BrString;
typedef struct { uint8_t ok; intptr_t value; } BrIntResult;
typedef struct { uint8_t ok; BrString value; } BrTextResult;
*/
import "C"

import (
	api "go-bridge/examples/11_channel"
	"unsafe"
)

// Channels use typed handles. Direction changes preserve the same Go channel.
//
//export BrIntNew
func BrIntNew(capacity C.intptr_t) C.uintptr_t { return bridgePin(make(chan int, int(capacity))) }

//export BrIntSendOnly
func BrIntSendOnly(h C.uintptr_t) C.uintptr_t {
	ch := bridgePointer[chan int](h)
	if ch == nil {
		return 0
	}
	return bridgePin((chan<- int)(ch))
}

//export BrIntRecvOnly
func BrIntRecvOnly(h C.uintptr_t) C.uintptr_t {
	ch := bridgePointer[chan int](h)
	if ch == nil {
		return 0
	}
	return bridgePin((<-chan int)(ch))
}

//export BrIntSend
func BrIntSend(h C.uintptr_t, value C.intptr_t) { bridgePointer[chan int](h) <- int(value) }

//export BrSendIntSend
func BrSendIntSend(h C.uintptr_t, value C.intptr_t) { bridgePointer[chan<- int](h) <- int(value) }

//export BrIntRecv
func BrIntRecv(h C.uintptr_t) (out C.BrIntResult) {
	value, ok := <-bridgePointer[chan int](h)
	if ok {
		out.ok = 1
		out.value = C.intptr_t(value)
	}
	return
}

//export BrRecvIntRecv
func BrRecvIntRecv(h C.uintptr_t) (out C.BrIntResult) {
	value, ok := <-bridgePointer[<-chan int](h)
	if ok {
		out.ok = 1
		out.value = C.intptr_t(value)
	}
	return
}

//export BrIntClose
func BrIntClose(h C.uintptr_t) { close(bridgePointer[chan int](h)) }

//export BrSendIntClose
func BrSendIntClose(h C.uintptr_t) { close(bridgePointer[chan<- int](h)) }

//export BrTextNew
func BrTextNew(capacity C.intptr_t) C.uintptr_t { return bridgePin(make(chan string, int(capacity))) }

//export BrTextSend
func BrTextSend(h C.uintptr_t, value C.BrString) {
	text := string(unsafe.Slice((*byte)(unsafe.Pointer(value.data)), int(value.size)))
	bridgePointer[chan string](h) <- text
}

//export BrTextRecv
func BrTextRecv(h C.uintptr_t) (out C.BrTextResult) {
	value, ok := <-bridgePointer[chan string](h)
	if ok {
		out.ok = 1
		out.value = C.BrString{data: C.CString(value), size: C.size_t(len(value))}
	}
	return
}

//export BrTextClose
func BrTextClose(h C.uintptr_t) { close(bridgePointer[chan string](h)) }

//export BrNumbers
func BrNumbers(count C.intptr_t) C.uintptr_t { return bridgePin(api.Numbers(int(count))) }

//export BrSum
func BrSum(h C.uintptr_t) C.intptr_t { return C.intptr_t(api.Sum(bridgePointer[<-chan int](h))) }

//export BrSend
func BrSend(h C.uintptr_t, value C.intptr_t) { api.Send(bridgePointer[chan<- int](h), int(value)) }

//export BrEcho
func BrEcho(h C.uintptr_t) C.uintptr_t {
	ch := api.Echo(bridgePointer[chan int](h))
	if ch == nil {
		return 0
	}
	return bridgePin(ch)
}

//export BrEchoText
func BrEchoText(h C.uintptr_t) C.uintptr_t {
	ch := api.EchoText(bridgePointer[chan string](h))
	if ch == nil {
		return 0
	}
	return bridgePin(ch)
}
func main() {}
