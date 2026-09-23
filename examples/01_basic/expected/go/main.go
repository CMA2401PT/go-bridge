package main

/*
#include <stdint.h>
*/
import "C"

import basic "go-bridge/examples/01_basic"

//export BasicIntInAndOut
func BasicIntInAndOut(value C.intptr_t) C.intptr_t {
	return C.intptr_t(basic.IntInAndOut(int(value)))
}

//export BasicBoolInAndOut
func BasicBoolInAndOut(value C.uint8_t) C.uint8_t {
	result := basic.BoolInAndOut(value != 0)
	if result {
		return 1
	}
	return 0
}

//export BasicGoSpecType
func BasicGoSpecType(value C.uint16_t) C.uint16_t {
	return C.uint16_t(basic.BasicGoSpecType(uint16(value)))
}

func main() {}
