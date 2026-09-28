package namedtype

import "fmt"

type MyNamedNonStructInt int

func (i MyNamedNonStructInt) EchoNonStructInt() string {
	return fmt.Sprintf("my non struct int: %v", i)
}

func FnEchoNonStructInt(i MyNamedNonStructInt) string {
	return fmt.Sprintf("my non struct int: %v", i)
}

func (i *MyNamedNonStructInt) PtrEchoNonStructInt() string {
	return fmt.Sprintf("my non struct int: %v", *i)
}

func PtrFnEchoNonStructInt(i *MyNamedNonStructInt) string {
	return fmt.Sprintf("my non struct int: %v", *i)
}

func (i *MyNamedNonStructInt) Inc() {
	*i++
}

func FnEchoInt(i int) string {
	return fmt.Sprintf("int: %v", i)
}

func PtrFnEchoInt(i *int) string {
	return fmt.Sprintf("*int: %v", *i)
}

func IncInt(i *int) {
	*i++
}

type MyNamedNonStructSliceInt []int

func (si MyNamedNonStructSliceInt) EchoNonStructSliceInt() string {
	return fmt.Sprintf("my non struct slice int: %v", si)
}

func FnEchoNonStructSliceInt(si MyNamedNonStructSliceInt) string {
	return fmt.Sprintf("my non struct slice int: %v", si)
}

func (si *MyNamedNonStructSliceInt) PtrEchoNonStructSliceInt() string {
	return fmt.Sprintf("my non struct slice int: %v", si)
}

func PtrFnEchoNonStructSliceInt(si *MyNamedNonStructSliceInt) string {
	return fmt.Sprintf("my non struct slice int: %v", si)
}

func FnEchoSliceInt(si []int) string {
	return fmt.Sprintf("slice int: %v", si)
}

func PtrFnEchoSliceInt(si *[]int) string {
	return fmt.Sprintf("*slice int: %v", si)
}

func (si *MyNamedNonStructSliceInt) Append(v int) {
	*si = append(*si, v)
}

func AppendSliceInt(si *[]int, v int) {
	*si = append(*si, v)
}
