# 类型映射总览

本文汇总 01–15 已确认的 Go → Python 类型规则，是各案例 readme 的索引与上位约定。
具体 ABI、测试与限制见对应案例。本文描述手写样例；当前生成器的契约见
[实施计划](../docs/generator-plan.md)与[运行时协议](../docs/generator-runtime.md)。
样例与生成器均不恢复 Go panic；未恢复的 panic（包括 nil 解引用）会终止整个
Python 进程。
10 使用当前句柄和 panic 协议验证 any 映射；生成器已支持本例及 `*any`、`[]any` 组合。

## Go 类型分类

| 分类 | 例子 | 说明 |
| --- | --- | --- |
| 基础值类型 | `int`、`string`、`bool` | 按值跨 ABI |
| 引用类型 | `[]int`、`[]byte`、`map[string]int` | 共享底层数据，由 Go 管理 |
| 数组 | `[3]int` | 固定长度的值，包装持有可寻址变量 |
| 函数值 | `func(int) int` | 可调用的 Go 函数包装，保留闭包 |
| 指针类型 | `*int`、`*[]int`、`*MyInt`、`**int` | 指向一个 Go 变量 |
| 命名类型 | `type MyInt int`、`type MySlice []int`、`type MyStruct struct{...}` | 可以绑定方法，与底层类型是不同类型 |
| 空 interface | `any`、`interface{}` | 保存动态类型和值，用 GoAny 持有，显式装箱和断言 |
| 一般 interface | `io.Reader`、`error` | 返回声明的接口包装，调用 Go 实现的方法 |
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
   Go 变量；struct 和数组包装对象的句柄持有 `*T`，本身就是一个类型为 `T` 的 **Go 变量**。
   这决定了方法集和取地址方式：不可寻址的值用 `ptr()` 复制后取地址，struct 和数组变量
   用 `addr()` 直接取地址。无论哪种，`*T` 都是独立的指针类。

5. **类型闭包。** 生成器从 G 中出现的类型出发，递归生成构造和使用它们所需的
   全部类型，直到不再发现新类型。依赖是图，允许 `List → *List → List` 这样的环：
   - 指针类型 `*T`：生成 `T` 对应的类（`.value` 的类型）；
   - struct 或数组 `T`：总是生成对应指针类（`addr()` 的类型）；
   - 以非 struct/interface 为底层的命名类型：生成底层类型对应的类（`unname()` 与构造函数
     参数的类型）；
   - slice、数组、channel：生成元素类型；map 生成键和值类型；
   - 函数值与 interface：生成公开调用签名涉及的类型；
   - struct：生成每个导出字段的类型（构造函数参数与字段的类型）。

   因此出现 `***int` 就会生成 `PtrPtrPtrInt`、`PtrPtrInt`、`PtrInt`；出现
   `type IntPtr *int` 就会生成 `PtrInt`。任何 `.value`、`unname()`、字段和构造
   参数的类型都一定存在（08）。对象入口可以是公开构造器或 Go 函数返回值；
   接口的隐藏实现不需要额外的 Python 构造器。

6. **nil 是带类型的零值包装。** handle 0 表示指针、channel、map、函数值或 interface 的零值 nil；
   struct 和数组包装是变量，永远非 nil。Python 的 `None`
   不表示 Go nil，作为 Go 值输入一律拒绝；nil 的类型与 Go 一样是静态的，所以
   nil 接收者也能调用方法（09）。
   GoAny 中带类型的 nil 不等于 nil interface（10）；try 断言的 None 结果只表示失败。

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

## map：Go 键比较与共享

`map[K]V` 映射为 `MapKV`，从键值对 iterable 复制构造。查找、包含、写入与删除均由
Go 判定键相等。map 包装共享同一张表，clone 浅复制。map 中的 struct 和数组元素
不可寻址，读取返回副本。批量读取使用 copy_to_py_items 返回键值对列表，避免 Python
包装对象的身份比较改变 Go 键语义。nil 可读，写入 nil 由 Go panic。
见 [12_map](12_map/readme.md)。

## 数组：定长值与变量视图

`[N]T` 映射为 `ArrayNT`。无参构造得到零值，iterable 构造要求恰好 N 个元素。
数组包装表示一个 Go 变量，支持下标、copy.copy、addr 和 copy_to_py_list。
传参、返回、赋值、命名转换复制数组值；字段、slice 或数组元素、指针解引用保留
变量视图。数组内引用类型的共享关系由元素自身决定。见 [13_array](13_array/readme.md)。

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
- **解引用 nil**：`.value`、struct 指针的字段读写与值接收者方法需要目标；
  解引用 nil 会触发 Go panic 并终止进程。`ptr()` 与 `from_value(q)` 复制的是指针
  本身，nil 也可以。
- **nil 接收者**：指针接收者方法原样交给 Go，Go 方法可以自行处理 nil。
- **panic**：适配层直接调用 Go，不传输 panic 状态或消息。未恢复的 panic 终止
  整个 Python 进程，Python 不捕获它。
- slice 包装的 Python API 暂不区分 nil 与空 slice，没有 `is_nil`；内部保留 Go 的
  nil 状态，any 装箱与取出也不会丢失它（10）。

## any：显式装箱与类型断言

`any`／`interface{}` → `GoAny(GoInterface)`（10 的显式转换规则，15 的公共基类）。参数只接受 GoAny，返回值统一为 GoAny；
GoAny 保存一个不可直接修改的 interface 值，与 Python `typing.Any` 无关。

- 基础值使用模块函数，如 `int_as_any(v)`、`any_as_int(a)`、
  `try_any_as_int(a)`。按 Go 类型区分整数宽度、float32/float64、bool、string。
- 其他已映射具体类型，包括命名标量，提供 `v.as_any()`、`T.from_any(a)`、
  `T.try_from_any(a)`。GoAny 直接作为 any 参数传入，并继承 as_any 复制包装。
  动态断言接受 GoInterface，可直接传入其他接口包装。
- 装箱保留准确的 Go 动态类型，取出执行具体类型断言，不执行类型转换。
  `MyInt` 与 int、命名 slice 与底层 slice、T 与 `*T` 均不互认。
- 普通断言不匹配抛 TypeError；try 不匹配返回 None，成功返回 `T`，标注为
  `T | None`。None 只表示断言失败；类型匹配后的转换异常仍抛出。
- struct 装箱复制 T，成功取出再复制为独立的可寻址变量；不能将 Cell 中的 `*T`
  当成 T 装箱，也不能提供 interface 内部 struct 的可修改视图。slice 复制头并
  共享数组，指针复制指针值并共享目标；struct 的复制是浅复制。
- `GoAny.nil()`／`is_nil` 判断 nil interface，句柄为 0；装有带类型 nil 的
  GoAny 非 nil、持有非零句柄。成功取出的 nil 指针返回对应指针包装，不能返回 None。
  GoAny 的 bool 判断被拒绝，应使用 is_nil；try 结果使用 `is None` 判断失败。
- any 句柄直接保存原动态值，不增加 anyCell 包装。命名 slice／指针在装箱前恢复
  名字，取出后才按其既有 Cell 归一化。非 nil GoAny 和非 nil 对象结果各自拥有独立句柄。
- any 不扩展静态类型闭包。Go 返回的未知动态类型可以持有、传回；只有已映射的
  类型提供 Python 侧取出操作。生成器支持 `*any`、`[]any` 的既有指针／slice 操作。

具体失败协议、ABI、字段和生命周期见 [10_any](10_any/readme.md)。

## 一般 interface：Go 实现的方法集

接口返回声明的包装类型。具体包装类继承 GoInterface，提供接口公开方法；实现可以
是不在类型闭包内的隐藏 Go 类型。`I(value)` 按 Go 静态可赋值规则显式转换，
`I.from_any(value)` 和 `try_from_any(value)` 按动态类型断言。nil interface 保留接口
包装，不自动变成 None。error 同样是普通接口，Error 方法映射为 error()。
详见 [15_interface](15_interface/readme.md)。

## 函数值：调用 Go 闭包

Go 函数值成为可调用包装对象，例如 `FuncIntToInt`。调用释放 GIL，参数统一按位置
命名为 arg1、arg2；多返回值使用 tuple。包装和传回 Go 的函数值共享闭包状态。
命名函数类型可通过底层函数包装构造，unname 保留同一闭包。Python callable 不作为
Go 函数传入。见 [14_func](14_func/readme.md)。

## channel：同步回调接收

`chan T`、`chan<- T`、`<-chan T` 分别映射为 ChanT、SendChanT、RecvChanT。
包装持有同一个 Go channel；nil 用 handle 0 表示，释放包装不关闭 channel。
`recv(callback) -> bool` 接收一个值后同步调用 Python 函数，关闭且读空后返回 False。
阻塞收发与原 Go API 调用释放 GIL，回调在调用线程持有 GIL 时执行。双向 channel
通过 send_only/recv_only 显式转换方向；单向类只提供方向允许的操作。
具体语义、ABI、线程与关闭责任见 [11_channel](11_channel/readme.md)。

## 方法集

| Python 类 | 值接收者 | 指针接收者 |
| --- | --- | --- |
| 命名标量、slice、map、channel、函数值 | 有 | 无，需先 `ptr()` |
| 指针类 | 有，`(*p).M()` | 有 |
| struct、命名数组（变量） | 有，作用在副本上 | 有 |

命名 struct 是变量，对应 Go 中可寻址的 `v.PtrM()`；它的指针类 `PtrT` 与其他指针类
一样有全部方法。不可寻址的值不挂指针方法：临时装箱调用会静默丢失修改，生成器也无法判断方法
是否只读。

## 生命周期与边界

- 一个持有关系对应一个 Ref（04–09 用 Pin，10–15 用 cgo.Handle）；视图、读出的
  slice、`p.value` 都创建新的持有关系。Go 值中从不保存句柄，没有跨边界引用环。
  被指向的数据由 Go 可达性管理。
- nil 与 panic 见上节。06–08 的 Ref 早于 09；生成器的 nil 表示参考 09，
  样例和生成器均不恢复 Go panic。
- struct `T` 与 `PtrT` 的句柄都持有 `*T`，共用字段与方法导出；只是 Python 类型不同。
- 匿名 struct 不支持：生成器遇到导出签名或字段依赖此类型时明确报错。
- 未覆盖：指针相等、标量字段取地址 `&s.X`、Python 实现 Go callback/interface。
