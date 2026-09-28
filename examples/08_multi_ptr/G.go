package multiptr

import "fmt"

// 多级指针：只出现 ***int，**int 与 *int 需要由类型闭包补齐。
func IncPPP(p ***int) {
	***p++
}

func EchoPPP(p ***int) string {
	return fmt.Sprintf("***int: %v", ***p)
}

// Retarget 修改的是中间一级指针，而不是最终的 int。
func Retarget(pp **int, p *int) {
	*pp = p
}

func Clear(pp **int) {
	*pp = nil
}

// 命名指针：底层是 *int，unname 与构造参数需要 *int。
type IntPtr *int

func IncIntPtr(p IntPtr) {
	*p++
}

// Counter 与 Inner 只作为字段出现，由类型闭包补齐。
type Counter int

func (c Counter) Echo() string {
	return fmt.Sprintf("counter: %v", int(c))
}

type Inner struct {
	N int
}

type Box struct {
	Count Counter
	Inner Inner
}

func (b Box) Echo() string {
	return fmt.Sprintf("box: %v", b)
}

func (b *Box) Inc() {
	b.Count++
	b.Inner.N++
}

// struct 的二级指针：**Box 的方法集为空，只能经 *pp 调用。
func IncBoxPP(pp **Box) {
	(*pp).Inc()
}
