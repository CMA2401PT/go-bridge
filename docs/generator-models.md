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
| Pointer | `Elem` | `**int` 指向 `*int` 节点 |
| Named，底层非 struct | `Name`、`PackagePath`、`Underlying`、方法集 | `type Count int` 指向 int 节点 |
| Named，底层 struct | `Name`、`PackagePath`、`Fields`、方法集 | `type Node struct { Next *Node }` |

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
匿名 struct、interface、map 都在这里被拒绝。named bool 的拒绝原因来自尚未确定的
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

`Fields` 当前只保存直接导出字段，`Index` 计入未导出字段的位置。提升字段展开还未
实现，不能从“提升方法已收集”推断“提升字段也已支持”。

## Python 模型：确定公开接口

[pyapi.Plan](../internal/pyapi/model.go) 将 Go 节点映射成 Python 节点。每个 Python 节点
仍引用原 Go 节点，所以 int8 与 int64 虽然 `Name` 都是 `int`，也不会被错误合并。

当前 `basicName` 只允许 Basic，映射到 bool/int/float/str。这是一道明确的后端能力
检查：只要 Go 模型包含未支持节点，整个规划失败。基础 alias 可以重新导出，但 named
类型即使底层是 int，目前也不会悄悄退化成 Python int。

`planFunction` 决定函数和参数名：`Snake` 处理连续缩写、大小写边界，`avoidKeyword`
给关键字加 `_`；无名和 `_` 参数按原位置变成 arg1、arg2。数字留在词内，如 Value2
变成 value2。一个 `namespace` 检查包级名字，另一个检查每个函数的参数名。冲突报出
双方来源；当前保留 _native、__all__。

函数、方法、字段未来都应使用这里的命名规划，但当前模型只有包级函数与基础 alias，
尚无成员名字空间、字段属性、构造参数或桥接保留成员的实现。

## ABI 模型：补回 Python 类型名无法表达的信息

[abi.Plan](../internal/abi/model.go) 调用 `wireType` 为每个 Python 类型确定 C 表示。
`Type` 保存 `Kind`、C 类型名、还原 Go 值时的类型名与位宽，并引用 Python 节点。
位宽来自加载目标包时的 `types.Sizes`，不依据 Python 类型名猜测。

每个 `Operation` 当前对应一个原 Go 函数，保存参数和返回类型、`SBCallN` 符号及
`SBResultN` 结果结构名（仅有返回值时）；无返回值时 Result 为空，对应 void ABI。
序号来自模型中已排序的函数顺序，不是文件中的声明顺序。
相同输入可重复生成；在前面插入函数会改变后续序号，因此没有跨版本二进制兼容承诺。

结果结构只包含原函数的返回值，不承载 panic。String 所有权由包注释、`Kind` 和输出器的共同约定表达，
尚无独立的“所有权字段”或一般操作体模型。添加句柄、批量传输或字段操作时，要先
补充这些协议概念；只新增一个 `Kind` 并不足以获得后端支持。
