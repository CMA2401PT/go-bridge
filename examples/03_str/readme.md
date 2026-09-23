# 03_str：字符串

本例只研究 Go `string` 与 Python `str` 的转换：统一 ABI 类型、UTF-8 编码、
按值复制和 C 内存释放。字节切片及 holder 的研究见 [04_bytes_and_holder](../04_bytes_and_holder/readme.md)。

```sh
python examples/build.py 03_str --test
```

## 输入与 Python API

`G.go` 定义 `func RecvAndReturn(inp string) string`。Python 接口和 `.pyi` 为：

```python
import go_str

assert go_str.recv_and_return("你好\0world") == "Echo: 你好\0world"
# def recv_and_return(value: str) -> str: ...
```

## string：C.GoStringN 与 C.CString

Python str 先由 C++ 转为 UTF-8，并包装为一个 `SBString`（指针与长度）。
Go 导出函数的参数和返回值都是 `C.SBString`，所以一个 Go `string` 在函数
签名中始终占一个参数或返回位置。输入结构体及其 UTF-8 缓冲区只在调用期间
借用 Python 内存；Go 包裹层用 `C.GoStringN` 拷贝为 Go string。带长度的版本
保留内嵌 `\0`，而 `C.GoString` 按 NUL 终止。

Go 返回时，`SBString` 结构体按值返回，只有字符数据由 `C.CString` 分配在 C 内存。
C++ 包裹层按保存的长度严格解码为 Python str，然后释放字符数据。
转换和释放逻辑分别封装在 Go/C++ 两侧；string 不进入 holder。

输入含孤立代理字符时，UTF-8 编码失败并抛出 `UnicodeEncodeError`。如果 Go
返回的字节不是有效 UTF-8，解码会抛出 `UnicodeDecodeError`。string 路径是
按值复制，没有所有权转移。

## 类型映射与验证

| Go API | C ABI | Python API |
| --- | --- | --- |
| `string` 参数 | 按值传入 `SBString` | `str` |
| `string` 返回值 | 按值返回 `SBString` | `str` |

`SBString` 包含字符地址和字节长度；每个字符串始终对应一个 ABI 参数或返回值。
Go 侧的 `goString` / `cString` 与 C++ 侧的 `c_string` / `py_string` 分别封装转换，
函数绑定只负责连接这些转换与原始 API 调用。

`test.py` 检查空字符串、普通文本、Unicode、内嵌 NUL、孤立代理字符和类型拒绝。
当前输入长度受 `C.GoStringN` 的 `C.int` 范围限制，超长输入返回错误。一般 Go
panic 和 `(T, error)` 的转换留给专门案例。
