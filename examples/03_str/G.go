package strvalue

import "fmt"

// 本例研究 Go string 与 Python str 的按值复制，以及统一的 BrString ABI 表示。
func RecvAndReturn(inp string) string {
	fmt.Println("recv and echo: ", inp)
	return "Echo: " + inp
}
