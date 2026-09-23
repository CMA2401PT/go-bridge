# 02_multi_return：多返回值

本例研究普通 Go 多返回值如何映射到 Python 的固定长度 tuple。通用目录和构建
约定见 [examples 文档](../readme.md)。

在仓库根目录运行：

```sh
python examples/build.py 02_multi_return --test
```

成功时最后输出 `02_multi_return: OK`。

## 输入与 Python API

`G.go` 保持普通 Go 代码，定义两个带命名返回值的函数：

```go
func ReturnTwo(i int) (i1 int, i2 int)
func ReturnFour(i int) (i1 int, i2 int, i3 int, i4 int)
```

Python 使用 snake_case 函数名，返回值顺序与 Go 声明一致，可以直接解包：

```python
import go_multi_return as multi

first, second = multi.return_two(10)  # 11, 12
assert multi.return_four(-2) == (-1, 0, 1, 2)
```

Go 返回值名 `i1` 等不成为 Python 属性或字典键。本例采用普通 tuple，命名与
匿名返回值应遵循相同的位置映射规则。对应 `.pyi` 明确表达返回数量和逐项类型：

```python
def return_two(value: int) -> tuple[int, int]: ...
def return_four(value: int) -> tuple[int, int, int, int]: ...
```

## ABI 方案

C 函数只返回一个值，因此 `expected/go/main.go` 在 C preamble 中显式定义
`MultiReturnTwoResult` 和 `MultiReturnFourResult` 两个结构体，分别包含 2 个和
4 个 `intptr_t` 字段。导出函数返回对应的 C 结构体，C++ 从生成的 `libbridge.h`
获得相同声明。

| Go | C ABI | Python |
| --- | --- | --- |
| 输入 `int` | `intptr_t` | `int` |
| 返回 `(int, int)` | `MultiReturnTwoResult`，按值返回 | `tuple[int, int]` |
| 返回 `(int, int, int, int)` | `MultiReturnFourResult`，按值返回 | `tuple[int, int, int, int]` |

每个 Go 返回值按声明顺序复制到结构体字段；`bindings.cpp` 调用导出函数一次，
再使用 `py::make_tuple` 将字段转换成 Python tuple。结构体布局和按值返回的
调用约定由同一目标平台的 cgo/C/C++ 工具链处理，不手工假设偏移或打包方式。

本例选择显式 C 结构体，使 ABI 字段和类型可直接审阅；未来生成器需生成这些
声明和相应的 Go/C++ 转换。当前不使用输出指针参数，也不依赖 cgo 为 Go 多返回
导出函数自动命名的返回结构体。此选择可随后续案例调整。

## 所有权与边界

结构体仅包含标量，按值跨越 ABI，不携带 Go 指针，也不需要跨边界的释放函数。
Python tuple 和整数对象由 Python 管理，与 Go 调用完成后的内存无关。

输入 `int` 继续映射为随目标架构宽度变化的 `intptr_t`，使用 pybind11 的范围
检查和 `noconvert()` 参数策略。返回值的加法在 Go 中执行，仍遵循 Go 整数语义，
不会变成 Python 任意精度运算。

本例不涉及 error、panic 转换、不同类型的混合返回值或返回对象的生命周期。
特别是 `(T, error)` 是否转换为 Python 异常，需要另一个案例单独定义。

## 验证与生成器所需信息

`test.py` 保持为小型冒烟测试，检查两个函数的 tuple 类型、长度和顺序、逐项
整数类型，以及解包和关键字参数调用。使用的值包含正数、负数和零；尚未覆盖
整数极值、溢出和错误输入。

未来生成器需读取包路径、函数名、参数类型、返回值数量与逐项类型，并保留返回
顺序。返回值名称不是生成位置字段或 Python tuple 所必需的信息。所有公开函数
都需同步生成固定长度的 `.pyi` 返回声明。
