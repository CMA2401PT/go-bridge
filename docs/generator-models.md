# 模型、类型身份与闭包

[文档导航](README.md) · [分工](generator-architecture.md) · [扩展指南](generator-maintenance.md)

## Go 模型：保留类型身份

[model.go](../internal/gomodel/model.go) 的 `Package` 保存目标包的路径、名字、源码目录、
`types.Sizes`，以及类型节点、导出类型声明和包级函数。`Dir` 由 `Load` 填写；直接调用
`Analyze` 的单元测试没有构建目录，因此这样的模型只能用于分析/渲染测试。

`Type` 的字段随 `Kind` 有明确用途：

| Kind | 有效的结构字段 | 示例 |
| --- | --- | --- |
| Basic | `Name`、`Go` | int、string |
| Slice | `Elem` | `[]int` 指向 int 节点 |
| Signature | `Call` | 参数、返回类型；参数名按位置规范化 |
| Array | `Elem`、`Length` | `[N]T` 包含元素类型与固定长度 |
| Map | `Key`、`Elem` | `map[K]V` 递归收集键和值类型 |
| Channel | `Elem`、`Direction` | `chan T`、`chan<- T`、`<-chan T` |
| Pointer | `Elem` | `**int` 指向 `*int` 节点 |
| Interface | `Go`、公开方法集 | any、匿名方法接口 |
| Named，底层非 struct/interface | `Name`、`PackagePath`、`Underlying`、方法集 | `type Count int` 指向 int 节点 |
| Named，底层 interface | `Name`、`PackagePath`、方法集 | 保留原接口与私有方法的身份 |
| Named，底层 struct | `Name`、`PackagePath`、`Fields`、`Selectable`、方法集 | `type Node struct { Next *Node }` |

这里没有独立的匿名 Struct 节点：匿名 struct 目前被拒绝，命名 struct 的直接导出字段
就记录在 Named 节点中。需要检查其底层结构时，可以读取 `Type.Go.Underlying()`。

每个节点保留 `go/types.Type`。`types.Unalias` 先消除真正的 alias，私有 `typeutil.Map`
索引按照 Go 类型身份查找节点；`Package.Find` 也走该索引。`Package.Types` 保留确定的
遍历顺序，`Type.ID` 是其中的位置。索引与列表必须由构造过程一起维护，不能仅向列表
追加类型后期待 `Find` 能找到它。

以下三种情况不同：

```go
type Count int          // 新的 named 类型
type OtherCount int     // 另一个 named 类型
type CountAlias = Count // 同一个类型的另一个名字
```

前两者各有节点，但 `Underlying` 都指向 int。`CountAlias` 在 `Declarations` 中保留
导出名字，指向 Count 的同一节点；不会为了 alias 再生成一种类型身份。同名但来自
不同包的 named 类型也有不同节点，不能用短名字作索引。

## Load 与 Analyze 的分界

[load.go](../internal/gomodel/load.go) 中的 `Load` 负责调用 `go/packages`，让 Go 工具
选择多文件、依赖、构建标签和平台文件；要求结果恰好一个包，并拒绝 package main。
依赖包错误也会收集并报告。它随后把包的类型信息与目标平台尺寸交给 `Analyze`。

`Analyze` 不再解析 AST，也不遍历依赖包的全部声明。它先加入 `supportedBasics` 中的
基础类型，再按排序后的包作用域名字访问导出类型和函数。当前遇到导出变量、常量会
报错，即使生成器支持它们的值类型；内部变量和常量不作为根。

闭包通过递归访问建立，没有另一个显式待处理队列：

```text
visit(t)
  → 去掉 alias
  → 已登记：返回已有节点
  → 未登记：先加入 Types 和 index
  → 根据 Kind 递归收集 Elem / Underlying / 字段 / 方法签名
```

先登记、再展开使 `Node → *Node → Node` 能结束。停止条件是“不再发现新类型”，不是
“最后所有路径都通向基础类型”。分析失败返回错误，不向下游交付部分模型。

`named`、`fields`、`methods`、`function`、`tuple` 各负责一类依赖。它们传递 `where`
字符串，把失败上下文组织为“包声明 → 字段 / 方法 → 参数 / 结果”。新增诊断应继续
沿用这条路径，而不是只报告一个缺少来源的类型名。

前端同时执行支持域校验，并非无条件接纳所有合法 Go 类型。例如 named bool、泛型、
匿名 struct、类型约束接口和含私有方法的匿名接口都在这里被拒绝。空 interface 通过完成后的
`types.Interface.Empty()` 识别；any 与 interface{} 共用节点，不分析运行时动态值。
named bool 的拒绝原因来自尚未确定的
Python 表示；将来解决该映射时，也必须移除此处的限制。

## 方法声明与方法集查询

`methods(owner, selected, where)` 查询真实的 Go 方法集，`selected` 分别是 `T` 和
`*T`。`Function` 中两个接收者字段不能合并：

| 字段 | 含义 |
| --- | --- |
| `Receiver` | 方法原始声明中的接收者 |
| `SelectedReceiver` | 本次查询方法集所用的类型 |
| `PointerReceiver` | 原始声明是否为指针接收者 |

例如 `func (c Counter) Read() int` 既能从 Counter 方法集读到，也能从 `*Counter`
方法集读到。第二次查询的 `SelectedReceiver` 是 `*Counter`，`Receiver` 仍是 Counter。
单纯查询这个集合不会将 `*Counter` 加入闭包；真正声明在 owner 上的指针接收者才会
引入对应依赖。因此不能把方法列表当成“所有这些包装类都应生成”的依据。

提升方法还可能声明在未导出的嵌入类型上。通过导出 owner 可以调用这些方法，不需要
为隐藏接收者生成包装类，所以原始接收者保留为 `types.Type`，不要求一定对应公开节点。
参数、结果中直接暴露未导出 named 类型仍会被拒绝。

`Fields` 保存直接导出字段，供关键字构造使用，`Index` 计入未导出字段的位置。
`Selectable` 保存最终可见的直接/提升字段：`selections` 遍历嵌入类型（包括未导出的
嵌入类型）收集候选名字，再调用 `types.LookupFieldOrMethod` 决定遮蔽与歧义。选择
路径只作诊断；Direct 表示直接字段，Indirect 表示选择路径经过指针解引用。
生成代码使用公开字段名访问，不写出包外无法访问的私有嵌入路径。

`Package.Ensure` 允许后续阶段用同一类型身份索引加入依赖；它只收集 Go 事实。
`pyapi.complete` 定义桥接补全政策：命名 struct 自动加入 *T，双向 channel 加入两个方向类型，不扩展无限指针链。
补全会原地扩展传入的模型，重复补全不会改变模型；Type.Origin 保存首次引入节点的
依赖路径，包含 API 来源或 bridge addr 来源。ptr() 的存在取决于补全后的闭包中是否
存在 *T，不区分其来源。这一模型准备工作本身不代表新类型后端已通过验收。

`Ensure` 扩展失败后，调用者必须整体丢弃该 Package，因为部分依赖可能尚未展开完成。
模型不维护错误状态，也不支持在失败后继续扩展。Analyze 失败时不返回 Package。

## Python 模型：确定公开接口

[pyapi.Plan](../internal/pyapi/model.go) 将 Go 节点映射成 Python 节点。每个 Python 节点
仍引用原 Go 节点，所以 int8 与 int64 虽然 `Name` 都是 `int`，也不会被错误合并。

`className` 保留 named 类型名字，并为无名 slice/pointer 递归生成名字。
`planClasses` 记录 Shape、Elem、Underlying、Pointer、可见字段、构造字段和选定的方法集。
Shape 区分内建值、命名标量、slice、struct、pointer、channel 和 any；ByteSlice 决定字节专用传输接口。
这些信息集中规划，输出器不自行决定类型关系。alias 重新导出同一个 Python 类型。
命名标量是独立的 int/float/str 子类，不能当成无名类型丢弃其身份。

`planFunction` 决定函数和参数名：`Snake` 处理连续缩写、大小写边界，`avoidKeyword`
给关键字加 `_`；无名和 `_` 参数按原位置变成 arg1、arg2。数字留在词内，如 Value2
变成 value2。一个 `namespace` 检查包级名字，另一个检查每个函数的参数名。冲突报出
双方来源；当前保留 _native、__all__、Ref、Slice、Ptr。any 节点占用 GoAny 名字；
`planAny` 规划全部具体类型的转换，基础转换函数参加同一包级名字冲突检查。

`memberName` 统一字段/方法名字：桥接保留成员加 `_`，例如 Value → value_；命名标量
还避开其 Python 内建基类成员，例如 Go string 方法 Join → join_，保留 str.join。
重命名后仍检查字段、方法之间的冲突。T 与 *T 使用相同拼写；方法参数 self 变成 self_。
不同 Go 类型生成相同类名（含不同包中的同名 named）时报错，不自动编号掩盖冲突。

## ABI 模型：补回 Python 类型名无法表达的信息

[abi.Plan](../internal/abi/model.go) 调用 `wireType` 为每个 Python 类型确定 C 表示。
`Type` 保存 `Kind`、C 类型名、还原 Go 值时的类型名与位宽，并引用 Python 节点。
位宽来自加载目标包时的 `types.Sizes`，不依据 Python 类型名猜测。

`abi.InitSymbol` 固定为 BrInit，表示模块加载时调用的内部 void(void) 初始化入口。
它不来自目标包的公开声明，不属于 Python 函数，也不占用业务操作的序号。

每个 `Operation` 对应一个原 Go 函数或选定的方法；方法额外带 Receiver，保存参数和返回类型、`BrCallN` 符号及
`BrResultN` 结果结构名（仅有返回值时）；无返回值时 Result 为空，对应 void ABI。
序号先分配给包级函数，再按类型闭包顺序分配给方法，不依据源文件声明顺序。
相同输入可重复生成；在前面插入函数会改变后续序号，因此没有跨版本二进制兼容承诺。

结果结构只包含原函数的返回值，不承载 panic。String/Handle 所有权由包注释、`Kind` 和输出器的共同约定表达，
参数按传输类型借用，结果按传输类型拥有资源；Type.Cell 明确 Go holder 中保存的类型，
Type.Go 明确调用原 API 时的类型。新资源类型须先定义其所有权协议再扩展输出器。

`abi.Slice` 单独描述具体 slice 的元素类型和批量/索引操作符号，Go/C++ 输出器共享
这份计划。它与包级函数 Operation 分工不同，不将缓冲区操作伪装成目标包函数。

`abi.Channel` 独立规划构造、收发、关闭和方向转换。接收返回 ok 与元素值，
关闭读空时元素值是 Go 零值。详见 [channel 协议](generator-channel.md)。

`pyapi.AnyConversion` 保存转换类型及公开名字，`abi.AnyConversion` 保存装箱、
断言符号和带 ok 字段的结果结构。普通和 try 断言共用一个 ABI 操作，转换不计入
业务 Operation，也不要求对应对象 Class。转换引用的命名类型参与 `planImports`，
即使目标包没有导出函数也不能漏掉命名标量所需的 import。

`abi.Class` 记录具体类型的构造、复制、取地址、load/store、显式转换和字段操作；不存在
的操作用空符号表示。`abi.Field` 绑定已规划的公开选择器与传输类型。`planImports` 为
实际被 Go 适配代码引用的目标包分配 api，未引用时 blank import 仍触发初始化；
依赖包按路径排序分配 depN，供 types.TypeString 正确渲染 Go 类型。
