# Go 对象、句柄与类型操作

[文档导航](README.md) · [运行时协议](generator-runtime.md)

## 句柄与所有权

生成器使用标准库 `runtime/cgo.Handle` 保存 Go 对象的可达根，C++ 只持有不透明整数。
不使用历史样例的地址句柄、Pinner 或整数到 Go 指针的转换。每个新 Ref 都有独立句柄；
同一底层数组可以由多个句柄共同保持可达。Go 对象内部只保存普通 Go 引用，不保存句柄。

- 输入句柄借用，Python 对象在同步调用期间保持存活。
- 返回句柄归 C++ 所有，Ref 不可复制、可移动，析构调用 SBRelease。
- 结果结构和批量缓冲区在任何 Python 对象转换前由 RAII 整体接管。
- 句柄转交给 Ref 时原槽位置零；剩余槽位由 owner 释放，避免重复释放与部分结果泄漏。
- `SBRelease(0)` 用于空所有权槽；其他无效句柄或类型不匹配是内部错误，直接 panic。
- `_native._holder_count` 与 `_collect_go` 仅用于生命周期测试，不是公开 API。

## slice 的操作分工

`pyapi.Type` 决定具体 Python 类名及元素类型；`abi.Slice` 保存 New/Len/At/Set/
Reslice/Clone/Copy 的符号、slice 和元素传输类型。Go/C++ 分别消费同一份计划：

- `slice_go.go` 实现具体 Go 类型的操作与元素转换。
- `slice_cpp.go` 实现 Python 构造、索引规范化、批量转换与所有权。
- `objects.cpp.tmpl` 提供通用 Ref、缓冲区 owner 和索引处理。
- `holder.go` 生成通用 Go 句柄运行时，不理解 slice 的业务操作。

使用类型专属计划表达内建操作，避免让包级函数 Operation 承担构造、字段和缓冲区等
不同协议。输出器不得自行发明公开名字或传输符号；支持更多对象类型时扩展相应计划。

具体 slice 继承不可构造的 `Slice[Elem]`，后者继承 Ref。`[]byte`/`[]uint8` 的类名
是 SliceBytes，从 bytes 复制；其他 slice 从 Iterable[Elem] 复制。Go 函数只接受对应
包装类，不隐式接受 list。整数元素的越界行为与标量参数一致，抛 OverflowError。

索引支持负数，越界在 C++ 抛 IndexError。切片按 Python 边界裁剪，步长只接受 1；
结果共享底层数组并保留剩余容量。clone 浅复制数组，嵌套 slice 的元素数组仍共享。
具体包装类不可被 Python 子类化；包装对象不可迭代（含 reversed），整体读取使用 copy_to_py_list；字节切片另提供 copy_to_py_bytes。

构造先完成全部 Python 输入转换，并保留迭代产生的临时包装对象，之后一次进入 Go。
批量输出缓冲区先清零，Go 写入各个元素；C++ 转换任何元素失败时释放整批尚未转交
的资源。Go 不保存外部缓冲区地址。nil slice 与空 slice 不区分，都持有非零句柄。

Go 适配层不 recover；非法内部句柄、Go 操作 panic 都直接终止进程。Python 输入检查
和 C++ 资源释放仍保留，不将 Go panic 包装成可恢复异常。

## 类型补全规则

Go 模型只描述语言事实。Python 规划为 struct 的 addr() 补 *T；其他类型仅在闭包
已存在 *T 时提供 ptr()，不递归创造无限指针链。推广字段的可见性属于 Go 事实，
由 gomodel 用 Go 选择器规则收集；Python 层只负责命名及冲突检查。

具体包装类在 C++ 中使用模块专属命名空间，避免多个生成模块的类型注册和 caster 混淆。
模块可以同时导入，但不能互传包装对象。调用前统一检查包装对象的具体类型及其 C++
holder 是否构造；直接 __new__ 得到的未初始化对象抛 TypeError，不读取未构造内存。

## 命名类型与 Cell

Python 的命名标量是 int/float/str 的独立子类，按基础值跨 ABI；参数要求对应类型。
命名 slice 与无名 slice 是独立类，显式构造和 unname 共享数组。命名指针显式转换与
unname 共享目标。所有具体类禁止 Python 子类化，内建数值运算仍可能返回普通内建值。

| 类型 | holder 中的 Cell | C ABI |
| --- | --- | --- |
| 基础/命名标量 | 不使用 holder | 数值或 SBString |
| 无名/命名 slice | 剥去最外层名字的 []E，保留元素身份 | 拥有的整数句柄 |
| struct T | *T | 拥有的整数句柄 |
| 普通/多级指针 | 原指针类型 | 整数句柄；nil 为 0 |
| 命名指针 | 剥去最外层名字的指针类型 | 整数句柄；nil 为 0 |

`goInput` 从 Cell 还原原 API 类型，`goOutput` 归一化成 Cell 后保存。类型断言必须与
保存的 Cell 完全一致。命名类型与其底层类型各有独立操作计划，不依赖内存强转。
`*MySlice` 的 Cell 仍是 *MySlice，不剥去指针目标的名字。

## struct、视图与方法

struct 关键字构造只接受直接导出字段，省略时保留 Go 零值；显式 None 不表示零值。
所有输入先在 C++ 校验，再一次进入 Go 构造。未导出字段不可传参，但其提升出来的
公开字段可以通过 Go 选择器访问。经过 nil 嵌入指针的访问会直接 panic。

struct 字段读取和 `slice[i]` 的 struct 元素读取返回变量视图，写入会影响原变量。
字段赋值、普通函数参数/返回值、copy.copy 和 copy_to_py_list 中的 struct 元素按值复制；
slice/pointer 字段继续共享其引用。通过嵌入指针读出的视图指向该指针的目标，目标由 Go
可达性保持存活，父 Python 包装释放后视图仍有效。

`T.addr()` 共享 T 的变量；`PtrT(T)` 和 from_value 复制 T 到新变量。`PtrT.value` 是
目标变量的视图，赋值则复制值。T 与 PtrT 均具有 Go 的值/指针接收者方法；命名标量和
slice 只有值接收者方法，其指针类才有指针接收者方法。多级指针及命名指针没有这些方法。

## 指针与 nil

普通指针构造及 from_value 分配存放输入副本的新变量。value 读写只解引用一层，
多级指针的 value 赋值可以重定向中间层；先前读出的指针值不随重定向改变。
ptr() 复制当前值（包括 nil 指针）到新变量；struct 使用语义不同的 addr()。

nil() 创建同一具体指针类的零句柄，is_nil 直接检查句柄；不接受 None，不允许 bool(p)。
nil 返回值不创建 Go 持有关系。nil 接收者原样传给 Go，方法可以合法处理 nil；真正
解引用 nil（含 value、字段、值接收者方法）直接终止整个 Python 进程。

当前未映射 Go 指针相等或 struct 值相等。Ref 的 Python 身份不等于 Go 目标地址，
不要用两个包装对象的 == 判断它们是否指向同一 Go 对象；nil 使用 is_nil 判断。
除 struct 的 copy.copy 外，deepcopy、pickle 和标量字段取地址也不在已实现协议内。

## 输出器入口

- `objects_go.go`：构造、复制、显式转换、指针 load/store、字段视图及赋值。
- `objects_cpp.go`：先注册全部类，再绑定构造、成员与属性，支持递归类型；所有 Ref
  接收者使用 checked_ref 校验，所有资源结果在 Python 转换前接管。
- `objects_python.go`：类继承、构造、字段、方法与泛型声明；类型名字来自 pyapi。
- `abi/objects.go`：Class/Field 操作及依赖包 import 计划；不生成任何源码。

桥接成员名加下划线避让，并在最终名字上检查冲突。例如 Go Value 字段成为 value_，
命名 string 的 Join 方法成为 join_，保留 Python str.join。方法参数 self 变成 self_。
