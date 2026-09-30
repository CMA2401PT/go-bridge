package anyvalue

import "fmt"

type MyInt int
type MySlice []int
type IntPtr *int

type Point struct {
	X      int
	Values []int
	Data   any
}

func Echo(value interface{}) any   { return value }
func DynamicKind(value any) string { return fmt.Sprintf("%T", value) }
func NilAny() any                  { return nil }
func NilPointer() any              { return (*int)(nil) }
func NilSlice() any                { return []int(nil) }
func EmptySlice() any              { return []int{} }
func SliceIsNil(value any) bool    { return value.([]int) == nil }
func InvalidString() any           { return string([]byte{0xff}) }
func Unknown() any                 { return map[string]int{"answer": 42} }
func UnknownValue(value any) int   { return value.(map[string]int)["answer"] }

// These signatures put representative concrete types in the static closure.
func EchoMyInt(value MyInt) MyInt         { return value }
func EchoMySlice(value MySlice) MySlice   { return value }
func EchoIntPtr(value IntPtr) IntPtr      { return value }
func EchoPoint(value Point) Point         { return value }
func EchoDoublePointer(value **int) **int { return value }
