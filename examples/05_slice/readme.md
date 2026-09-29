# 05_slice：值元素与 Go 对象元素的 slice

本例研究 `[]byte` 以外的 slice 如何构造、传递和读取。按元素类型分成两类：

- `[]int`：元素适合按值复制，Python 用 int 序列构造。
- `[][]int`：元素本身是需要 Go 管理生命周期的 `[]int`，Python 用 `SliceInt` 构造。

slice 仍然复用 [04_bytes_and_holder](../04_bytes_and_holder/readme.md) 的 Ref 与
Pin / Get / UnPin，本文只记录新增的规则。

```sh
python examples/build.py 05_slice --test
go test -race ./examples/05_slice/expected/go
```

## Python API

```python
import go_slice as bridge

a = bridge.SliceInt([1, 2])
joined = bridge.concat_a(a, bridge.SliceInt(range(3, 5)))
assert joined.copy_to_py_list() == [1, 2, 3, 4]

outer = bridge.SliceSliceInt([a, bridge.SliceInt([])])
del a  # Go 中的 [][]int 仍引用它的元素
result = bridge.concat_b(outer, outer)
assert [s.copy_to_py_list() for s in result.copy_to_py_list()] == [[1, 2], []] * 2

values = bridge.SliceInt([10, 20, 30])
values[-1] = 31
middle = values[1:]          # Go 的 values[1:]，共享底层数组
middle[0] = 21
assert values.copy_to_py_list() == [10, 21, 31]
copied = values.clone()      # slices.Clone，新的底层数组
copied[0] = 0
assert values[0] == 10
```

类层次为 `Ref → Slice → SliceInt / SliceSliceInt`。`Slice` 在 `.pyi` 中是
`Generic[ElemType]`，声明所有公共操作（见下节），其中
`copy_to_py_list() -> list[ElemType]`、`s[i] -> ElemType`、`s[i:j]` 与
`clone() -> Self`；
运行时它是不可构造的 pybind11 基类，`__init__.py` 为它补上 `__class_getitem__`，
使 `bridge.Slice[int]` 在运行时注解中同样可用。每种 Go 元素类型对应一个具体
子类，名称由元素类型拼接（`[]int → SliceInt`，`[][]int → SliceSliceInt`）。
Go 函数参数不做隐式转换：`concat_a` 只接受 `SliceInt`，不接受 list。

## Slice 操作：保持 Go 语义

一个 Python 包装对象等价于一个持有 slice 头的 Go 变量。所有操作都照搬 Go 的
行为，只把 Python 的索引写法翻译成 Go 下标：

| Python | Go | 行为 |
| --- | --- | --- |
| `len(s)` | `len(s)` | 长度 |
| `s[i]` | `s[i]` | 支持负下标，越界 `IndexError`；`SliceSliceInt` 返回共享数组的新 `SliceInt` |
| `s[i] = v` | `s[i] = v` | 原地写入，共享底层数组的所有 Ref 可见；元素类型规则同构造 |
| `s[i:j]` | `s[i:j]` | 新 Ref，共享底层数组并保留剩余容量；类型与 `s` 相同 |
| `s.clone()` | `slices.Clone(s)` | 新底层数组；`[][]int` 为浅拷贝，元素数组仍共享 |
| `s.copy_to_py_list()` | — | 一次性复制为 Python list |

切片边界按 Python list 的规则规范化：负数从末尾计、越界截到 `[0, len]`、
`j < i` 得到空切片。因此 `s[i:j]` 永远不会越过 `len` 伸进剩余容量，也不暴露
Go 的三下标形式或 `cap()`。Go 没有步长，`s[::k]`（k ≠ 1）抛 `ValueError`。
不支持切片赋值 `s[i:j] = ...` 与 `del s[i]`，它们在 Go 中没有对应。

包装对象**不可迭代**：`Slice.__iter__ = None`，`iter(s)`、`for`、`in` 都抛
`TypeError`，`.pyi` 同样声明，mypy 会拒绝迭代。逐元素读取每次都是一次 cgo 调用
（嵌套 slice 还要 Pin 一次），实测 10 万个元素逐下标读取约为
`copy_to_py_list()` 的 30 倍耗时；需要遍历时先整体复制。不提供 `append`：Go 的
`append` 返回新 slice 头，与 `list.append` 的原地语义冲突，拼接由原函数完成。

## 两类元素的构造与读取

| 操作 | ABI | 行为 |
| --- | --- | --- |
| `SliceInt(values)` | `BrNewSliceInt(intptr_t 数组, 长度) → uintptr_t` | C++ 先逐项转换到临时数组，Go 复制到新 `[]int` 再 Pin |
| `SliceInt.copy_to_py_list()` | `BrCopySliceInt(句柄, 缓冲区, 长度)` | Go 按值填充 C++ 缓冲区，C++ 再建 list |
| `SliceSliceInt(values)` | `BrNewSliceSliceInt(uintptr_t 句柄数组, 长度) → uintptr_t` | Go 对每个句柄 `Get[[]int]`，组成新 `[][]int` 再 Pin |
| `SliceSliceInt.copy_to_py_list()` | `BrSliceSliceIntItems(句柄, 句柄缓冲区, 长度)` | Go 对每个元素分别 Pin，C++ 各包装为新的 `SliceInt` |
| `len()` | `Br<类型>Len(句柄) → size_t` | Go 读取长度 |

**值元素**：Python int 与 Go int 之间只复制值，ABI 元素类型沿用 01_basic 的
`intptr_t`。构造接受任意 `Iterable[int]`（list、tuple、range、生成器等），每个元素
按 `noconvert` 规则检查：float、str、None 以及超出 Go int 范围的整数抛出
`TypeError`；`bool` 作为 int 的子类被接受。转换全部成功后才调用 Go，失败时不创建
任何句柄。

**Go 对象元素**：`[][]int` 的元素不复制内容，只跨 ABI 传递句柄数组。Go 读取的
元素值是 slice 头，与原 `SliceInt` 共享底层数组；新 `[][]int` 作为普通 Go 引用
保持这些数组可达。因此 Python 释放元素 Ref 后，外层 slice 仍然有效；同一个
`SliceInt` 可以出现多次。构造期间 C++ 持有迭代得到的每个元素对象，直到 Go 读取
完句柄，所以生成器产生的临时 `SliceInt` 也是安全的。元素不是 `SliceInt`（包括
嵌套 list 或 `SliceSliceInt`）时抛出 `TypeError`。

读取嵌套 slice 时，每个元素得到一个独立 Pin 与独立 Ref，遵守 04 中“一个 Pin 对应
一个 Ref”的规则。C++ 在创建任何 Python 对象之前先接管所有新句柄，中途失败时
已接管的句柄由析构释放。

这两种策略可以推广为：元素是可按值表示的标量时传元素数组；元素本身需要 Go
管理时传句柄数组，并由容器自己的 Go 引用维持元素存活，而不是递归 pin。

## append 与共享底层数组

`append` 在容量足够时原地写入底层数组。适配层把参数原样交给原函数，不做
截断，因此 Python 看到的就是 Go 的行为：

```python
storage = bridge.SliceInt([1, 2, 3, 4, 0])
base = storage[:4]                                   # len 4, cap 5
first = bridge.concat_a(base, bridge.SliceInt([5]))
second = bridge.concat_a(base, bridge.SliceInt([6]))
assert first.copy_to_py_list() == [1, 2, 3, 4, 6]   # 与 Go 相同
fresh = bridge.concat_a(base.clone(), bridge.SliceInt([7]))  # Go 的惯用规避方式
```

曾考虑过读取参数后执行 `slices.Clip`，强制 `append` 重新分配。它只屏蔽了写入
剩余容量这一种别名；元素赋值、重新切片仍然共享数组，得到的既不是值语义也
不是 Go 语义，还会改变依赖容量的 Go 函数的行为。故不采用。

这种共享只涉及数据可见性，与生命周期无关。多个 Ref 指向同一数组是安全的：
`UnPin` 只解除本 Ref 的 pin，数组由 Go GC 按可达性管理；Go 值中从不保存句柄，
C++ 包装对象也不持有 Python 对象，因此不存在跨边界的引用环，不需要引用计数。

## 所有权与边界

- 构造时 Python 数据被复制到 Go；读取时 Go 数据被复制到新的 Python list。
  C++ 临时缓冲区只在调用期间借给 Go，Go 不保存其地址。
- 所有 slice 的内存都由 Go GC 管理，Ref 析构只调用通用的 `BrUnPin`。
- nil 与空 slice 在 Python 中都表现为长度 0；从 Python 构造的空 slice 是非 nil 的。
- 外部缓冲区为空指针但长度非零、长度超出 Go int 能表达的元素数、或句柄为 0 时，
  Go 返回 0 或错误码，C++ 抛出 `ValueError`。
- 当前假设 Go int 与 `intptr_t` 等宽，即 01_basic 的平台假设。

## 验证

Python 测试覆盖极值与生成器构造、强制 Go GC 后读取、元素 Ref 先释放、临时元素、
重复元素、append 共享剩余容量、下标读写与越界、切片边界与步长拒绝、切片与
clone 的共享关系、不可迭代、类层次与泛型下标、元素和参数类型拒绝，以及迭代
中途抛出异常时的释放。每个测试结束时检查 holder 数量回到基线。`test.py` 可通过
`mypy --strict` 检查。Go 测试覆盖输入复制、append 语义、元素读写、重新切片、浅拷贝、元素存活和非法缓冲区与下标。

```sh
GOEXPERIMENT=cgocheck2 python examples/build.py 05_slice --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/05_slice/expected/go
MYPYPATH=examples/.build/05_slice/python mypy examples/05_slice/test.py --strict
```

生成器需要知道每个 slice 参数与返回值的完整元素类型，递归判断元素是按值传递
还是按句柄传递，并为每种具体 slice 类型生成一个 `Slice[Elem]` 子类及对应的
New / Len / Copy / At / Set / Slice / Clone 导出函数；Go 侧的 clone 与 reslice 已是
泛型辅助函数，只有元素的 ABI 表示（值或句柄）随元素类型变化。数组 `[N]T`、其他标量元素、struct 元素以及 Python
侧的原地修改尚未覆盖。
