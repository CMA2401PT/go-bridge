# 01_basic：基础标量

本例既是其他案例的目录模板，也验证基础标量函数如何从 Go 映射为带类型声明的
Python 扩展。通用目录约定、环境要求和构建说明见 [examples 文档](../readme.md)。

在仓库根目录执行：

```sh
python examples/build.py 01_basic --test
```

成功时最后输出 `01_basic: OK`。

## 输入与期望输出

`G.go` 是唯一原始输入，保持普通 Go package，不包含绑定细节：

```go
func IntInAndOut(a int) int
func BoolInAndOut(b bool) bool
func BasicGoSpecType(u16 uint16) uint16
```

`expected/` 保存手写的预期生成结果：

- `go/main.go`：调用原始 Go API，导出 C ABI。
- `bindings.cpp`：包含构建生成的 `libbridge.h`，通过 pybind11 暴露 `_native` 模块。
- `go_basic_value/__init__.py`：重导出公共函数。
- `go_basic_value/__init__.pyi`：声明 Python 公共接口。

`test.py` 验证最终用户行为；构建产物统一位于 `examples/.build/01_basic/`。

## 类型和命名映射

| Go API | C ABI | pybind11/C++ | Python API 与 `.pyi` |
| --- | --- | --- | --- |
| `int` | `intptr_t` | `intptr_t` | `int -> int` |
| `bool` | `uint8_t`，输出为 0/1 | `bool` | `bool -> bool` |
| `uint16` | `uint16_t` | `uint16_t` | `int -> int` |

`intptr_t` 表达 Go `int` 随目标架构变化的宽度。pybind11 的整数 caster 检查
目标 C++ 整数类型的范围；参数使用 `py::arg(...).noconvert()` 限制隐式转换。
Go 适配层将非零的布尔 ABI 输入视为 true，C++ 调用侧只传入 0 或 1。

Go 导出符号带 `Basic` 前缀，例如 `BasicIntInAndOut`，以降低符号冲突风险。
Python 暴露 snake_case 名称：

```python
import go_basic_value as basic

assert basic.int_in_and_out(1) == 2
assert basic.bool_in_and_out(True) is False
assert basic.basic_go_spec_type(41) == 42
```

`.pyi` 声明如下，构建时添加 `py.typed` 标记：

```python
def int_in_and_out(value: int) -> int: ...
def bool_in_and_out(value: bool) -> bool: ...
def basic_go_spec_type(value: int) -> int: ...
```

Python 的 `int` 声明不表达底层整数范围，实际可接受范围由绑定层限制。

## 内存、错误与测试范围

本例在 ABI 边界只按值传递标量，没有需要跨边界管理的内存或对象，因此不需要
释放函数。类型不匹配或超出 C++ 目标整数范围的参数由 pybind11 拒绝。

Go 函数没有返回 error，本例也不处理 panic。字符串、容器、对象生命周期、并发
和 GIL 策略见其他案例。

`test.py` 对三个函数各做一次正常调用，验证扩展和 Go 动态库可以加载、导出符号
可以链接、参数与返回值能完成往返。不覆盖整数边界、溢出语义、失败路径、压力
测试或性能基准。

## 生成器需要的信息

生成器需要获取原始包的 import 路径、导出函数名、参数及返回类型，产生对应的
Go ABI 转换、C++ 绑定和 Python 类型声明。导出符号前缀、snake_case 命名及整数
映射是本例采用的方案。
