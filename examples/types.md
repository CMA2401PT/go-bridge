# 类型映射总览

本文汇总 01–09 已确认的 Go → Python 类型规则，是各案例 readme 的索引与上位约定。
具体 ABI、测试与限制见对应案例。

## Go 类型分类

| 分类 | 例子 | 说明 |
| --- | --- | --- |
| 基础值类型 | `int`、`string`、`bool` | 按值跨 ABI |
| 引用类型 | `[]int`、`[]byte` | 共享底层数据，由 Go 管理 |
| 指针类型 | `*int`、`*[]int`、`*MyInt`、`**int` | 指向一个 Go 变量 |
| 命名类型 | `type MyInt int`、`type MySlice []int`、`type MyStruct struct{...}` | 可以绑定方法，与底层类型是不同类型 |
| 匿名 struct | `func F() struct{ X int }` | 不支持 |

`type MyStruct struct{...}` 是底层类型为 struct 的命名类型，不属于匿名 struct。

## 总原则

1. **ABI 表示只由底层类型决定。** 名字只存在于 Python 类和 Go 适配层。
2. **不同的 Go 类型不隐式混用。** 命名类型与其底层类型、不同指针类型之间只能显式
   转换；Python 侧不做 Go 允许的 `[]int` → 命名 slice 隐式赋值，也不把值装箱成
   临时指针。
3. **除基础值类型及以其为底层的命名类型外，Python 对象都是 `Ref`。** Ref 持有 Go
   句柄，保证 Go GC 不回收被引用的数据，析构时释放（04）。
4. **值与变量。** Python 的 int 子类与 slice 包装对象是 Go 的**值**，不是可寻址的
   Go 变量；struct 包装对象的句柄持有 `*T`，本身就是一个类型为 `T` 的 **Go 变量**。
   这决定了方法集和取地址方式：不可寻址的值用 `ptr()` 复制后取地址，struct 变量
   用 `addr()` 直接取地址。无论哪种，`*T` 都是独立的指针类。

5. **类型闭包。** 生成器从 G 中出现的类型出发，递归生成构造和使用它们所需的
   全部类型，直到不再发现新类型。依赖是图，允许 `List → *List → List` 这样的环：
   - 指针类型 `*T`：生成 `T` 对应的类（`.value` 的类型）；
   - 命名 struct `T`：总是生成 `PtrT`（`addr()` 的类型）；
   - 以非 struct 为底层的命名类型：生成底层类型对应的类（`unname()` 与构造函数
     参数的类型）；
   - slice：生成元素类型；
   - struct：生成每个导出字段的类型（构造函数参数与字段的类型）。

   因此出现 `***int` 就会生成 `PtrPtrPtrInt`、`PtrPtrInt`、`PtrInt`；出现
   `type IntPtr *int` 就会生成 `PtrInt`。任何 `.value`、`unname()`、字段和构造
   参数的类型都一定存在，且总能从基础值类型逐层构造出来（08）。

6. **nil 是带类型的零值包装。** handle 0 表示指针类型的零值 nil，只有指针类与
   命名指针可以是 nil；struct 包装是变量，永远非 nil。Python 的 `None` 不表示 nil，一律拒绝；nil
   的类型与 Go 一样是静态的，所以 nil 接收者也能调用方法（09）。

## 基础值类型

直接是 Python 内建类型，按值复制（01、03）。Go `int` 为 `intptr_t`，float、None
抛 `TypeError`，越界抛 `OverflowError`。

## 引用类型：slice

- `Slice[Elem]` 泛型基类，继承 Ref；每种元素类型一个具体类，如 `SliceInt`（05）。
- 构造函数参数与浅转换结果的类型一致：`SliceInt(Iterable[int])`、
  `copy_to_py_list() -> list[int]`。元素是命名类型时同样只做浅转换，`[]MyInt` 对应
  `list[MyInt]`。
- 提供 `__len__`、`__getitem__`、`__setitem__`、reslice、`clone` 等，保持 Go 的
  共享与 append 语义。刻意不支持迭代：逐元素跨 cgo 太慢，应先 `copy_to_py_list()`。
- slice 包装对象是一个不可变的 slice 头，Python 赋值只产生别名，对应 Go 的
  `a := s`。

## 命名类型

### 底层为基础值类型：内建类型的子类

`type MyInt int` → `class MyInt(int)`，不是 Ref，按值传递（06）。

这里由命名 int 验证。不能直接将此规则推广到命名 bool，因为 Python bool 不能被
继承；它的映射尚未定义，首版生成器明确拒绝命名 bool。

- 构造：`MyInt(v: int = 0)`，即 Go 的 `MyInt(v)`；`unname() -> int`，即 `int(i)`。
- 参数为 `MyInt` 时只接受 `MyInt`，普通 int 被拒绝。
- 只有值接收者方法；有 `ptr() -> PtrMyInt`（见指针类型）。
- **已知差异**：作为 int 子类，它能传给普通 `int` 参数（Python 子类型规则），且
  `n + 1` 返回普通 int。

### 底层为 slice：独立的 Slice 子类

`type MySlice []int` → `class MySlice(Slice[int])`，与 `SliceInt` 没有继承关系
（06）。

- 构造：`MySlice(s: SliceInt)`，即 Go 的 `MySlice(s)`，共享底层数组；
  `unname() -> SliceInt`，即 `[]int(s)`，同样共享。从 Python list 构造写成
  `MySlice(SliceInt([...]))`。
- 其他 slice 操作与 `SliceInt` 相同，reslice、`clone` 保持名字；ABI 导出也与
  `SliceInt` 共用（布局相同）。
- `SliceInt` 与 `MySlice` 互不接受，需显式转换（比 Go 严格，见总原则 2）。
- 只有值接收者方法；有 `ptr() -> PtrMySlice`。

### 底层为 struct：一个 Go 变量

`type MyStruct struct{...}` → `class MyStruct(Ref)`，Go 侧 Pin 的是它的地址
`*MyStruct`（06）。

- 构造：只接受关键字参数，每个导出字段一个，省略即 Go 零值，对应 keyed
  composite literal。嵌入字段按类型名的 snake_case 作为参数，要先构造嵌入 struct
  再传入，不能直接用其内部字段构造。struct 参数复制进来，slice 参数共享数组。
- 字段：每个导出字段一对 getter/setter；嵌入字段本身和它的提升字段都有。读取
  struct 字段返回视图（`&s.F` 所指的变量），赋值复制值（`s.F = v`）。
- **同时有值接收者与指针接收者方法**：包装对象就是一个可寻址的 Go 变量，对应 Go
  中对变量 `v` 调用 `v.PtrMethod()`。值接收者作用在副本上。
- `MyStruct` 参数传入时复制；`copy.copy(v)` 即 `c := v`。
- `addr() -> PtrMyStruct` 即 `&v`，与 `v` 共享同一个变量。`*MyStruct` 由独立的
  `PtrMyStruct` 表示，见指针类型。
- 没有 `unname`：它的底层类型是匿名 struct，不支持。
- 永远非 nil，没有 `nil()` 与 `is_nil`。

## 指针类型

每个出现的指针类型都有自己的指针类，命名 struct `T` 总会生成 `PtrT`（规则 5）。

`Ptr[Target]` 泛型基类，继承 Ref；每种指针类型一个具体类，如 `PtrInt`、
`PtrSliceInt`、`PtrMyInt`、`PtrMySlice`、`PtrMyStruct`（07、06）。Go 侧 Pin 的是
`*T`，句柄中的指针永远不变，一个 Ptr 对象始终指向同一个 Go 变量。

- **值 → 指针**：`PtrT(v)`、`PtrT.from_value(v)` 是同一个操作，等价于
  `p := new(T); *p = v`，也就是对 `v` 的**副本**取地址。不可寻址的值（标量、slice
  及其命名类型、指针）另有 `v.ptr()`，也是复制：Python 值不是 Go 变量，没有地址
  可取；对 slice，复制的是 slice 头，底层数组仍共享。每次调用都分配，所以
  `ptr()` 是方法而不是属性。Python 内建 int 无法挂方法，`*int` 只能用
  `PtrInt(v)` 或 `PtrInt.from_value(v)`。
- **变量 → 指针**：struct 包装是变量，`v.addr()` 即 `&v`，不复制，与 `v` 共享。
  struct 没有 `ptr()`，`addr()` 与 `ptr()` 语义相反，不共用名字。
- **指针 → 值**：`p.value` 读 `*p`，`p.value = v` 写 `*p`。类型只接受基类型本身：
  基类型是基础值类型时用 Python 内建类型，否则是对应的 Python 类。读取时，不可
  寻址的基类型返回副本；struct 返回 `*p` 这个变量本身（视图，与 struct 字段相同），
  写入仍复制。经指针的修改（如 append）替换 `*p`，之前读出的 slice 不变。
- **方法**：指针类有基类型的全部方法，指针接收者 `p.M()` 与值接收者
  `(*p).M()`。struct 指针类还有基类型的全部字段 `p.F`。
- 不同的指针类型互不继承、互不转换。`*T` 参数只接受对应指针类（含它的 nil）；
  普通值、`None` 与其他指针类都被拒绝。struct 值与它的指针类同样互不接受：
  `*T` 参数写 `f(v.addr())`，`T` 参数写 `f(p.value)`。
- 类名按 Go 类型拼写，每个 `*` 对应一个 `Ptr`：`**int` → `PtrPtrInt`，
  `*MyStruct` → `PtrMyStruct`，`**MyStruct` → `PtrPtrMyStruct`，没有例外。
- **多级指针**（08）：目标本身是指针时，`.value` 返回指向同一个变量的新指针对象；
  `from_value(q)` 与 `q.ptr()` 复制的是指针 `q` 本身，新变量与 `q` 指向同一个
  目标。`p.value = q` 替换 `*p`，不影响 `q` 原来所在的变量。
- 目标是指针的指针类没有无参构造：它的零值是 nil，用 `nil()` 得到。
- **方法集只有一层**：Go 只为 `*T` 自动解引用一次，`**T` 的方法集为空，所以
  `PtrPtrMyStruct`、`PtrPtrInt` 不挂任何 Go 方法。
- **命名指针** `type IntPtr *int`：底层不是 struct 的命名类型。构造函数
  `IntPtr(p: PtrInt)` 与 `unname() -> PtrInt` 都是转换，指向同一个变量；有
  `.value`（Go 允许 `*p`），没有 `from_value` 和方法（Go 不允许为它定义方法）。
  与 `PtrInt` 互不接受。
- **名字冲突**：struct 指针类挂着字段，Go 字段 `Value` 会与 `.value` 冲突。此时
  字段在 `T` 与 `PtrT` 上都加下划线后缀，写作 `value_`，构造参数同名（09）。

## nil

规则 6 的细节（09）：

- **表示**：所有指针类与命名指针都可以是 nil，即 handle 为 0 的同类对象；struct
  包装类是变量，不能是 nil。nil 不占 Pin，不计入 holder。
- **构造与判断**：类方法 `T.nil()` 即 `var p *T`；只读属性 `is_nil` 即 `p == nil`。
  `__bool__` 抛 `TypeError`，提示使用 `is_nil`，避免 `if p:` 对 nil 也为真。
- **出现位置**：参数、返回值、`.value`（目标是指针时）与指针字段都可以是 nil，
  类型标注仍是 `T`，不写 `T | None`。与 Go 一样，类型系统不区分可空与非空。
- **解引用 nil**：`.value`、struct 指针的字段读写与值接收者方法需要目标，抛
  `GoPanic`；Go 在同样的位置也会 panic。`ptr()` 与 `from_value(q)` 复制的是指针
  本身，nil 也可以。
- **nil 接收者**：指针接收者方法原样交给 Go，Go 方法可以自行处理 nil。
- **panic**：每个导出函数由适配层 `recover`，把 panic 消息返回给 C++，抛
  `GoPanic(RuntimeError)`，不会让进程崩溃。已 Pin 的参数不受影响。
- slice 的 nil 与空 slice 在 Go 中行为基本一致，暂不区分，也没有 `is_nil`。

## 方法集

| Python 类 | 值接收者 | 指针接收者 |
| --- | --- | --- |
| 命名值类型（int 子类）、命名 slice | 有 | 无，需先 `ptr()` |
| 指针类 | 有，`(*p).M()` | 有 |
| 命名 struct（变量） | 有，作用在副本上 | 有 |

命名 struct 是变量，对应 Go 中可寻址的 `v.PtrM()`；它的指针类 `PtrT` 与其他指针类
一样有全部方法。不可寻址的值不挂指针方法：临时装箱调用会静默丢失修改，生成器也无法判断方法
是否只读。

## 生命周期与边界

- 一个 Pin 对应一个 Ref；视图、读出的 slice、`p.value` 都是新的 Pin。Go 值中从不
  保存句柄，没有跨边界引用环。被指向的数据由 Go 可达性管理。
- nil 与 panic 见上节。06–08 的 Ref 早于 09，仍不接受 nil、不 recover；08 读取
  nil 级时抛 `ValueError` 是临时行为，生成器应统一按 09 处理。
- `T` 与 `PtrT` 的句柄都是 Pin 的 `*T`，共用字段与方法导出；只是 Python 类型不同。
- 匿名 struct 不支持：出现在参数、返回值中的 API 不生成；命名 struct 中类型为
  匿名 struct 的字段跳过，由生成器给出警告。
- 未覆盖：指针相等、标量字段取地址 `&s.X`、map、interface、error。
