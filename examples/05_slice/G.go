package slice

// 这里研究 非 []byte/[]uint8 的 slice 的构造
// 构造基本分为两种情况，
// 一种是，内部数据类型是 int/bool 等适合值传递的数据类型
// 另一种是，内部数据类型是 []int 这种需要由 Go 侧管理生命周期的类型，比如 ([][]int)
func ConcatA(a []int, b []int) []int {
	return append(a, b...)
}

func ConcatB(a [][]int, b [][]int) [][]int {
	return append(a, b...)
}
