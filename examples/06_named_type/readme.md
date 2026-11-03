# 06_named_type：命名类型与 struct

本例研究 Go 命名类型如何映射到 Python。按底层类型分三类：

- `type MyNamedNonStructInt int`：底层是值类型，按值跨 ABI。
- `type MyNamedNonStructSliceInt []int`：底层是 Go 管理的 slice，按句柄跨 ABI。
- `struct`：含标量、slice、命名类型、嵌套 struct 与嵌入字段，按句柄跨 ABI；
  以及它的指针类型 `*MyNamedStruct`。

Ref、Pin / Get / UnPin 沿用 [04](../04_bytes_and_holder/readme.md)，slice 沿用
[05](../05_slice/readme.md)，string 返回值沿用 [03](../03_str/readme.md)。本文只记录新增规则。

```sh
python examples/build.py 06_named_type --test
go test -race ./examples/06_named_type/expected/go
```

构建后运行 [demo.py](demo.py)，可以直接看到本例的核心行为：

```sh
python examples/build.py 06_named_type
python examples/06_named_type/demo.py
```

```text
命名 int: my non struct int: 5
加法结果: 6 int | 恢复命名: my non struct int: 6
转换后共享: [10, 20] | 命名切片: [10, 20]
构造复制: 源对象 = 60 | 子字段 = 6
字段视图写回: 7 | 省略字段的零值: 0
接收者: 原对象 = 1 | 返回副本 = 2
```

## Python API

```python
import copy
import go_named_type as bridge

n = bridge.MyNamedNonStructInt(5)       # int 的子类
assert n.echo_non_struct_int() == "my non struct int: 5"
bridge.fn_echo_non_struct_int(n)        # 5 本身会被拒绝

s = bridge.MyNamedNonStructSliceInt(bridge.SliceInt([1, 2]))  # Go 的 MyNamed(s)
assert type(s[1:]) is bridge.MyNamedNonStructSliceInt
bridge.fn_echo_non_struct_slice_int(s)  # SliceInt 需先显式转换

v = bridge.MyNamedStruct(
    basic_member_value=1,
    named_ref_member_value=bridge.MyNamedSubStruct(basic_member_value=6),
)                                       # 省略的字段为 Go 零值
v.named_ref_member_value.basic_member_value = 60   # Go 的 v.F.X = 60
v.basic_member_value_2 = 2              # 嵌入字段的提升字段
v.inc_basic_ptr()                       # 指针接收者：修改 v
w = v.inc_basic_non_ptr()               # 值接收者：修改副本并返回
c = copy.copy(v)                        # Go 的 c := v，浅拷贝
p = v.addr()                            # Go 的 p := &v，PtrMyNamedStruct
bridge.ptr_fn_echo_ptr(p)               # 传 v 本身会被拒绝
q = bridge.PtrMyNamedStruct(v)          # Go 的 q := new(T); *q = v，复制
```

总原则：**命名类型的 ABI 表示只由底层类型决定，名字只存在于 Python 类和 Go
适配层。** 命名类型之间的可赋值关系遵循 Go 的规则。

## 命名值类型：int 子类

| Go | C ABI | Python |
| --- | --- | --- |
| `MyNamedNonStructInt` 参数/返回值/字段 | `intptr_t`，同 `int` | `MyNamedNonStructInt(int)` |
| 方法 `EchoNonStructInt` | `BrMyNamedNonStructIntEchoNonStructInt(intptr_t) → BrString` | 实例方法 |
| `int(i)` | 无 | `unname() -> int` |

值类型没有需要 Go 管理的生命周期，不进入 holder，也不继承 Ref。Python 类是
真正的 `int` 子类，可以直接参与比较、哈希、格式化和 int 运算；方法调用时把
自身的整数值传给 Go，Go 适配层做 `api.MyNamedNonStructInt(value)` 再调用方法。

pybind11 的 `py::class_` 不能继承内建 `int`，因此这个类由 `_native` 在导入时用
`type(name, (int,), ...)` 创建，方法是绑定到该类的 `cpp_function`。`__new__` 与
Go int 参数使用同样的范围和类型检查，默认值 0 是 Go 零值；`__slots__ = ()` 使它
和 int 一样没有实例 `__dict__`。

以 `MyNamedNonStructInt` 为参数时只接受该类实例，普通 int 抛 `TypeError`。这
对应 Go 中 int 类型的变量不能直接赋给命名类型；Go 允许无类型常量 `5` 隐式
转换，Python 无法区分字面量与变量，本例选择严格一侧。

**已知差异**：Python int 运算返回普通 `int`（`n + 1`），而 Go 中 `n + 1` 仍是
`MyNamedNonStructInt`。需要保留名字时要显式构造。逐个重写算术运算符可以
消除差异，但会显著增加生成代码，暂不采用。

## 命名 slice：独立的 Slice 类

`MyNamedNonStructSliceInt` 与 `[]int` 底层类型相同，内存布局一致。因此它复用
`SliceInt` 的导出函数（Len / At / Set / Slice / Clone / Copy），C++ 中两者共用一个
模板实现；但它在 Python 中是 `Slice[int]` 的独立子类，**不继承 `SliceInt`**：

| Go | ABI | Python |
| --- | --- | --- |
| `MyNamedNonStructSliceInt(s)` | `BrNewMyNamedNonStructSliceInt`，新句柄共享底层数组 | 构造函数，只接受 `SliceInt` |
| `[]int(si)` | `BrMyNamedNonStructSliceIntUnname`，新句柄共享底层数组 | `unname() -> SliceInt` |
| `si.EchoNonStructSliceInt()` | `Get[MyNamedNonStructSliceInt](id)` 后调用 | 实例方法 |
| `si[i:j]`、`slices.Clone(si)` | 复用 SliceInt 导出，C++ 包装为命名类 | 结果仍为命名类型 |

构造函数就是 Go 的类型转换，不从 Python iterable 复制；从 list 构造写成
`MyNamedNonStructSliceInt(SliceInt([...]))`。两个方向的转换都共享底层数组，没有
复制成本。

`Get[T]` 的约定因此放宽为：读取类型与 Pin 时的类型必须**底层类型相同**，见
`holder.go`。同一个 `[]int` 句柄可以按 `[]int` 或命名类型读取。

命名 slice 与 `SliceInt` 互不接受，参数与字段都需要显式转换。这与命名 int 一致，
也**比 Go 严格**：Go 允许在 `[]int` 与命名 slice 之间直接赋值（二者之一是未命名
类型）。选择严格是因为继承无法正确表达这种关系：Go 的命名 slice 并不是 `[]int`
的子类型，到了指针层面（07）`*[]int` 与 `*MyNamedNonStructSliceInt` 也不可互换；
而显式转换共享数组，代价只是一次调用。总体规则见 [`../types.md`](../types.md)。

## struct：句柄持有 `*T`

struct 包装对象继承 Ref，是一个类型为 `T` 的 Go 变量。Go 侧 **Pin 的是 `*T` 而不是
`T`**：

- 字段视图和指针接收者都需要稳定地址。视图是 `Pin(&s.Field)`，嵌入字段位于
  偏移 0，与父 struct 地址相同；若直接以内部地址为句柄，会与父对象的 holders
  表项冲突。Pin 一个指针时，每个 Ref 都拥有自己的存放单元，句柄互不相同。
- 视图中的指针指向父 struct 内部，Go GC 会让整个父对象保持可达，所以释放父
  Ref 后，视图仍然有效。
- Go 不移动堆对象，C/C++ 也从不解引用；Pinner 只固定存放指针的单元，被指向
  的 struct 由普通 Go 可达性管理，与 05 中 slice 的底层数组一致。

### 构造：关键字参数的复合字面量

构造函数只接受关键字参数，所有字段都有默认值，对应 Go 的 keyed composite
literal `MyNamedStruct{BasicMemberValue: 1}`：省略的字段就是 Go 零值（零 struct、
nil slice、0）。这样字段顺序变化不会静默改变调用含义，大 struct 也不必逐个填写。
参数按 ABI 表示传入：标量传值，struct 与 slice 传句柄，省略的句柄参数为 0，
Go 侧保持零值。显式传入的 struct 和 slice 必须是对应包装，None 不表示省略。
所有参数先在 C++ 中转换，任一失败都不会调用 Go 或创建句柄。

构造时 struct 参数被**复制**进新值（Go 中把 struct 值放进字面量同样是复制），
slice 参数**共享**底层数组。

### 字段访问

| 字段类型 | 读 | 写 |
| --- | --- | --- |
| 标量 `int` | 按值 | 按值 |
| 命名值类型 | 返回 `MyNamedNonStructInt` | 只接受 `MyNamedNonStructInt` |
| slice | 新 `SliceInt` 句柄，共享底层数组 | 存入 slice 头，共享底层数组 |
| struct（含嵌入字段） | 视图 `&s.Field`，修改写回父对象 | 复制值：`s.Field = v` |

读取 struct 字段返回视图而不是副本，是为了让 `v.sub.x = 1` 与 Go 的 `v.Sub.X = 1`
含义一致；若返回副本，这行代码会静默丢失修改。赋值一个 struct 字段则复制，
之后修改原对象不影响字段，而旧视图看到的是新的字段值。

嵌入字段 `MyNamedAnomySubStruct` 以类型名的 snake_case 暴露为属性
`my_named_anomy_sub_struct`，其字段同时作为提升字段 `basic_member_value_2`
直接出现在外层类上。字段名采用 snake_case，`BasicMemberValue2` 为
`basic_member_value_2`。

### 方法、函数与复制

| Go | 适配层 | Python |
| --- | --- | --- |
| 值接收者 `func (s T) M()` | `(*p).M()`，接收者是副本 | 修改不影响原对象 |
| 指针接收者 `func (s *T) M()` | `p.M()` | 修改对所有别名与视图可见 |
| 参数 `T` | `*p` 复制后传入 | 只接受 `T` |
| 参数 `*T` | 直接传 `p` | 只接受 `PtrT`，见下节 |
| `c := v` | `Br<T>Copy` | `copy.copy(v)`，浅拷贝，slice 字段仍共享数组 |

`G.go` 为此增加了 `IncBasicPtr` 与 `IncBasicNonPtr`，因为原有的 echo 方法不修改
接收者，无法区分两种语义。值接收者返回的 `MyNamedStruct` 被 Pin 为新的 `*T`。

Python 变量赋值只产生别名，两个名字指向同一个 Go 变量，而不是复制 struct。没有提供
`__deepcopy__`、`__eq__` 或 `__repr__`：Go 没有通用深拷贝，struct 的 `==` 与
`%v` 语义没有映射。

## struct 的指针类

`*T` 由独立的 `PtrT(Ptr[T])` 表示，每个命名 struct 都会生成（类型闭包，见
[`../types.md`](../types.md)）。它和 `T` 的句柄都是 Pin 的 `*T`，所以共用字段与方法
导出，只是 Python 类型不同：

| Go | ABI | Python |
| --- | --- | --- |
| `p := &v` | `Br<T>Alias`，新句柄，同一个 `*T` | `v.addr() -> PtrT`，共享 |
| `p := new(T); *p = v` | `Br<T>Copy` | `PtrT(v)`、`PtrT.from_value(v)`，复制 |
| `*p`（可寻址） | `Br<T>Alias` | `p.value -> T`，视图，与读 struct 字段相同 |
| `*p = v` | `Br<T>Store` | `p.value = v`，复制 |
| `p.F`、`p.M()` | 与 `T` 相同 | `PtrT` 有 `T` 的全部字段与方法 |

- 为什么分开：Go 中 `T` 与 `*T` 是不同类型，赋给 `any`、type switch、实现接口时都
  不一样（10）。合并成一个类就无法表达“这个 `any` 装的是 `T` 还是 `*T`”。
- `T` 与 `PtrT` 互不接受：`*T` 参数写 `f(v.addr())`，`T` 参数写 `f(p.value)`，与 Go
  的 `F(&v)`、`F(*p)` 一样显式。字段与构造参数同样严格。
- 取地址叫 `addr()` 而不是 `ptr()`：07 的 `ptr()` 对**副本**取地址，而 struct 包装
  本身是变量，`addr()` 共享它。同名但语义相反容易误用。
- `PtrT` 没有 `addr()`、`__copy__`：它是指针值，复制指针就是 Python 赋值。
- 06 的 `PtrT` 永远非 nil；nil 见 [09](../09_nil/readme.md)。

## 所有权与边界

- 命名值类型只按值复制；其余包装对象析构时只调用通用 `BrUnPin`。
- struct 视图、slice 字段读取与值接收者结果都是新的 Pin，遵守“一个 Pin 对应
  一个 Ref”；Go 值中从不保存句柄，不存在跨边界引用环。
- 本例的 `PtrT` 永远非 nil；nil 指针的返回和参数、指针字段见 09。
- 共享数据的并发读写与 05 相同，由 Go 对象自身负责同步；绑定始终持有 GIL。

## 验证

Python 测试覆盖：命名 int 的 int 行为、极值、类型拒绝和方法；命名 slice 与
SliceInt 互不继承、双向转换共享数组、名字保持、参数与字段拒绝未转换的 slice；struct 的零值
构造、复制与共享规则、标量/命名/slice/struct 字段读写、嵌入与提升字段、视图在
父对象释放后存活、别名、浅拷贝、两种接收者、Go int 回绕、参数拒绝和构造失败
不泄漏句柄；struct 指针类的 `addr()` 共享、构造复制、`.value` 视图与写入复制、
经指针调用两种接收者，以及 `T` 与 `PtrT` 互不接受。每个 Ref 测试结束时检查 holder
数量回到基线。Go 测试覆盖命名 slice 布局共享、零值字段、偏移 0 的视图、父对象
释放后的视图、接收者和浅拷贝，以及 Alias / Copy / Store 的共享与复制。

```sh
GOEXPERIMENT=cgocheck2 python examples/build.py 06_named_type --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/06_named_type/expected/go
MYPYPATH=examples/.build/06_named_type/python mypy examples/06_named_type/test.py --strict
```

## 对未来生成器的要求

生成器需要从 `go/types` 获得每个命名类型的底层类型，并据此选择：底层为标量
时生成 Python 内建类型的子类并按值传递；底层为 slice 时生成对应 `Slice[Elem]`
类的子类并复用其导出；struct 生成持有 `*T` 的 Ref 子类和共用导出的 `PtrT`。还需要知道方法集及
每个方法的接收者是值还是指针、struct 字段顺序、导出性与嵌入关系（用于提升
字段，以及同名字段的遮蔽规则），参数只接受恰好对应的 Python 类，
命名类型与底层类型之间生成显式转换。未覆盖：其他底层类型（string、bool、float、map）、提升方法、嵌入指针、
字段名冲突、未导出字段、指针字段、struct 元素的 slice 以及 iota 常量。
