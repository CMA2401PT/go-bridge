package main

/*
#include <stddef.h>
#include <stdlib.h>

typedef struct {
    char *data;
    size_t size;
} SBString;
*/
import "C"

import (
	api "go-bridge/examples/03_str"
	"math"
)

func goString(value C.SBString) (string, bool) {
	if uint64(value.size) > math.MaxInt32 || (value.data == nil && value.size != 0) {
		return "", false
	}
	// GoStringN copies the full UTF-8 input, including embedded NUL bytes.
	return C.GoStringN(value.data, C.int(value.size)), true
}

func cString(value string) C.SBString {
	return C.SBString{data: C.CString(value), size: C.size_t(len(value))}
}

//export SBEchoString
func SBEchoString(input C.SBString) C.SBString {
	value, ok := goString(input)
	if !ok {
		return C.SBString{}
	}
	return cString(api.RecvAndReturn(value))
}

func main() {}
