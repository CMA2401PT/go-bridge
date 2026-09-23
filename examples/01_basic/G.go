package basic_value

import "fmt"

func IntInAndOut(a int) int {
	fmt.Println("recv: ", a)
	return a + 1
}

func BoolInAndOut(b bool) bool {
	fmt.Println("recv: ", b)
	return !b
}

func BasicGoSpecType(u16 uint16) uint16 {
	fmt.Println("recv: ", u16)
	return u16 + 1
}
