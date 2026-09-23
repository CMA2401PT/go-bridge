package bytesandholder

import "fmt"

// 本例研究 []byte 的暂存、pin 和 Python 包装对象的生命周期。
// 返回的切片保留在 Go 中，显式调用 copy_to_py_bytes 时才复制到 Python。
func RecvAndReturnBytes(inp []byte) []byte {
	fmt.Println("recv and echo: ", string(inp))
	return []byte("Echo: " + string(inp))
}
