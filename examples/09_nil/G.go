package nilptr

import "fmt"

// 链表：nil 接收者是合法的空链表。
type List struct {
	Value int
	Next  *List // 指针字段，可以为 nil
}

func (l *List) Len() int {
	if l == nil {
		return 0
	}
	return 1 + l.Next.Len()
}

func (l *List) Push(v int) *List {
	return &List{Value: v, Next: l}
}

// 值接收者：nil 接收者会在调用前 panic（Go 需要 *l）。
func (l List) Head() int {
	return l.Value
}

// 未处理 nil 的指针接收者：Go 内部解引用 nil 会 panic。
func (l *List) First() int {
	return l.Value
}

// 返回 nil 指针。
func Find(l *List, v int) *List {
	for ; l != nil; l = l.Next {
		if l.Value == v {
			return l
		}
	}
	return nil
}

// nil 参数与 nil 指向的 int。
func Deref(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}

func MaybeInt(ok bool) *int {
	if !ok {
		return nil
	}
	v := 42
	return &v
}

// 显式 panic。
func MustPositive(v int) int {
	if v <= 0 {
		panic(fmt.Sprintf("not positive: %d", v))
	}
	return v
}
