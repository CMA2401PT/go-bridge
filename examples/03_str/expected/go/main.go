package main

/*
#include <stddef.h>
#include <stdlib.h>

typedef struct {
    char *data;
    size_t size;
} BrString;
*/
import "C"

import (
	api "go-bridge/examples/03_str"
	"unsafe"
)

// Copy the borrowed UTF-8 bytes, including embedded NULs.
func goString(value C.BrString) string {
	return string(unsafe.Slice((*byte)(unsafe.Pointer(value.data)), int(value.size)))
}

func cString(value string) C.BrString {
	return C.BrString{data: C.CString(value), size: C.size_t(len(value))}
}

//export BrEchoString
func BrEchoString(input C.BrString) C.BrString {
	value := goString(input)
	return cString(api.RecvAndReturn(value))
}

func main() {}
