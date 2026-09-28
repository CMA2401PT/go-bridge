package namedtype

import "fmt"

// 这里需要考验的是对基础非ref类型（值类型）的重命名
// python 侧不能简单将其映射为基础类型，因为其可能绑定方法
// 建议在py 和 go 之间直接使用值传递而非 ref
// 可以参考 examples/01_basic 和 examples/03_str 的思路
type MyNamedNonStructInt int

func (i MyNamedNonStructInt) EchoNonStructInt() string {
	return fmt.Sprintf("my non struct int: %v", i)
}

func FnEchoNonStructInt(i MyNamedNonStructInt) string {
	return fmt.Sprintf("my non struct int: %v", i)
}

// 这里需要考验的是对基础ref类型的重命名
// python 侧除了要以类似
// 在py 和 go 之间应当传递 ref 而非值
// 可以参考 examples/05_slice
type MyNamedNonStructSliceInt []int

func (si MyNamedNonStructSliceInt) EchoNonStructSliceInt() string {
	return fmt.Sprintf("my non struct slice int: %v", si)
}

func FnEchoNonStructSliceInt(si MyNamedNonStructSliceInt) string {
	return fmt.Sprintf("my non struct slice int: %v", si)
}

type MyNamedSubStruct struct {
	BasicMemberValue int
}

type MyNamedAnomySubStruct struct {
	BasicMemberValue2 int
}

type MyNamedStruct struct {
	MyNamedAnomySubStruct
	BasicMemberValue      int
	RefMemberValue        []int
	NamedBasicMemberValue MyNamedNonStructInt
	NamedRefMemberValue   MyNamedSubStruct
}

func (si MyNamedStruct) EchoNonPtr() string {
	return fmt.Sprintf("MyNamedStruct non Ptr: %v", si)
}

func (si *MyNamedStruct) EchoPtr() string {
	return fmt.Sprintf("MyNamedStruct non Ptr: %v", *si)
}

// 以下两个方法验证接收者语义：指针接收者修改调用方的值；值接收者只修改副本，
// 并按值返回一个新的 struct。
func (si *MyNamedStruct) IncBasicPtr() {
	si.BasicMemberValue++
}

func (si MyNamedStruct) IncBasicNonPtr() MyNamedStruct {
	si.BasicMemberValue++
	return si
}

func FnEchoNonPtr(si MyNamedStruct) string {
	return fmt.Sprintf("MyNamedStruct non Ptr: %v", si)
}

func PtrFnEchoPtr(si *MyNamedStruct) string {
	return fmt.Sprintf("MyNamedStruct non Ptr: %v", *si)
}
