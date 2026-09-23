package main

/*
#include <stdint.h>

typedef struct {
    intptr_t value1;
    intptr_t value2;
} MultiReturnTwoResult;

typedef struct {
    intptr_t value1;
    intptr_t value2;
    intptr_t value3;
    intptr_t value4;
} MultiReturnFourResult;
*/
import "C"

import multireturn "go-bridge/examples/02_multi_return"

//export MultiReturnTwo
func MultiReturnTwo(value C.intptr_t) C.MultiReturnTwoResult {
	i1, i2 := multireturn.ReturnTwo(int(value))
	return C.MultiReturnTwoResult{value1: C.intptr_t(i1), value2: C.intptr_t(i2)}
}

//export MultiReturnFour
func MultiReturnFour(value C.intptr_t) C.MultiReturnFourResult {
	i1, i2, i3, i4 := multireturn.ReturnFour(int(value))
	return C.MultiReturnFourResult{
		value1: C.intptr_t(i1),
		value2: C.intptr_t(i2),
		value3: C.intptr_t(i3),
		value4: C.intptr_t(i4),
	}
}

func main() {}
