# C ABI、所有权与异常

[文档导航](README.md) · [调用实例](generator-walkthrough.md) · [验证与排障](generator-maintenance.md)

本页描述基础值与错误边界。Ref/句柄及 slice 批量传输已实现，详见
[对象与句柄协议](generator-objects.md)；指针与 struct 后端仍在后续阶段。

## 模块加载与 Go 初始化

每个生成库都导出内部入口 `void SBInit(void)`，名称由 `abi.InitSymbol` 统一定义。
`__init__.py` 总是导入 `_native`，C++ 在模块初始化时调用 SBInit。函数体为空；进入
cgo 导出的 Go 函数会等待 Go runtime 和包初始化完成。它不重复运行目标包的 init，
不向 Python 公开，也不增加每次业务调用的步骤。

这个入口同时保证没有导出函数的包也能生成 cgo 头文件，并让扩展对 Go 库有实际符号
依赖。只有 alias 或没有公开声明的包同样遵循这条初始化路径，不需要伪造业务操作。
Go init 中未恢复的 panic 会在 Python 导入过程中终止进程，仍不转换为 Python 异常。

## 值如何跨边界

类型选择在 [abi.wireType](../internal/abi/model.go)，C++ 输入转换在
[runtime.cpp.tmpl](../internal/emit/templates/runtime.cpp.tmpl)，Go 转换由
[goInput/goOutput](../internal/emit/adapter.go) 输出。

| Go 类型 | C 表示 | Python 输入/输出 |
| --- | --- | --- |
| bool | uint8_t | 输入必须是 bool；结果变成 bool |
| int8…int64 | int8_t…int64_t | int，按对应位宽检查 |
| uint8…uint64 | uint8_t…uint64_t | int，按无符号范围检查 |
| int / uint / uintptr | intptr_t / uintptr_t / uintptr_t | int，使用目标平台字宽 |
| float32 / float64 | float / double | 输入接受 float 或 int；输出 float |
| string | SBString，包含 char* 和 size_t | 输入必须是 str；输出严格 UTF-8 解码 |

整数检查用 `PyLong_Check`，因此 bool 也可传给整数参数；整数越界抛 OverflowError，
类型不符抛 TypeError。整数参数不接受 float、字符串或仅提供 `__index__` 的其他类型。
浮点数经过 C float/double 转换，float32 存在精度收窄；未额外限制 NaN 或无穷大。

`cppSource` 生成的 `static_assert` 检查本次分析得到的 Go int 位宽和 C++ intptr_t
是否一致。它不是整套跨平台 ABI 兼容检查，当前仍按本机构建和使用。

## 结果结构是一份所有权单位

有返回值的操作返回自己的 SBResultN 结构，其中只有 r0、r1 等结果字段；无返回值的
操作使用 void ABI，不生成结果结构。Python 侧零个结果映射成 None，一个结果直接返回，
多个结果按顺序构造 tuple。ABI 不传输 panic 状态或消息。

| 资源 | 分配/提供方 | 使用期限 | 释放方 |
| --- | --- | --- | --- |
| 输入 SBString.data | Python 的 UTF-8 缓存 | 同步调用期间借用 | Python 管理，Go/C++ 不释放 |
| `bridgeInput` 得到的 Go string | Go 复制输入 | 原包可以保留该 string | Go GC |
| 普通结果 SBString.data | `bridgeString` 调用 C.CString | 到结果转换结束或失败 | C++ owner 的 std::free |
| 数值结果 | 按值返回 | 随结果结构 | 无需释放 |

字符串长度是字节数，解码不依赖终止 NUL，所以内嵌 `\0` 不会截断字符串。空字符串
也遵循相同的分配/释放协议。SBString 只是内部传输结构，不是向任意外部 C 调用者开放
的安全缓冲区接口；当前适配层信任生成的 C++ 传入有效指针和长度。

[cppOwner](../internal/emit/bindings.go) 为每种结果结构生成一个 owner。它在任何 Python
返回对象创建前接管整个结果，析构时释放所有 String 结果。
例如结果 `(非法 UTF-8, 第二个字符串)` 解码第一个就失败，第二个也由同一 owner 释放。
这个约定不能改成“转换到哪个结果才开始拥有哪个结果”。

生成代码不主动释放 GIL，输入 Python 对象及 UTF-8 缓存在同步调用期间有效。Go 会
复制输入字符串后才调用原函数；原包不会因为保留 string 而持有 Python 内存。

## Go panic 直接终止宿主进程

生成的 Go 适配函数不安装 defer/recover，直接转换参数、调用原函数、转换并返回结果。
如果 panic 未被目标 Go 代码自身恢复，Go runtime 会终止整个宿主 Python 进程；这包括
显式 panic、panic(nil)、整数除零、nil 解引用，以及初始化或其他 goroutine 中未恢复
的 panic。Python 的 try/except（包括 BaseException）不能接住它。

生成模块不再提供 GoPanic，ABI 不再有异常标志或消息，C++ 也不检查 Go panic。
panic 不作为 C++ 异常跨语言展开，不能依赖 C++ 析构或 Python finally 在进程终止时执行。
前述 owner 的释放保证适用于正常返回和 C++/Python 结果转换失败，不涵盖进程终止。
目标 Go 代码自己写的 defer/recover 仍遵循 Go 原有语义。

移除了每次调用的恢复代码、panic 结果字段和 C++ 分支；实际性能收益尚未做基准测试，
不能仅凭删除 defer 推断提升幅度。整数入参范围检查与 UTF-8 检查仍由 C++ 执行。

## 错误发生在哪一层

| 条件 | 对外结果 | 原 Go 函数是否执行 |
| --- | --- | --- |
| bool 参数收到整数，string 参数收到 bytes 等 | TypeError | 否 |
| 整数超出目标范围，含负数传给无符号类型 | OverflowError | 否 |
| str 含无法编码的孤立代理字符 | UnicodeEncodeError | 否 |
| 巨大 Python int 无法转为 double | OverflowError | 否 |
| 原函数 panic 且未自行恢复 | 整个 Python 进程终止 | 已进入 |
| 普通结果含非法 UTF-8 | UnicodeDecodeError | 已正常返回 |

Go 内部整数运算溢出遵循 Go 语义，不会因上述输入检查而自动变成 OverflowError；
例如 uint16 的 65535 加 1 得到 0。整数除零会 panic，并按上面的进程终止契约处理。

`1309094` 将整数范围错误从 TypeError 修正为 OverflowError；与类型错误的区分现在由
`integer_arg` / `integer_overflow` 实现，并由生成器集成测试验证。

## ABI 的稳定范围

生成 Go 动态库与 C++ 扩展必须配套重建。SBCallN 是当前操作顺序的内部名字，增删函数
可能使序号变化，没有独立动态库的版本协商或长期二进制兼容协议。C++ 使用 cgo 本次构建
产生的头文件，不另维护一份手写头文件。

句柄结果同样由整个结果 owner 接管，转交给 Ref 时槽位置零，析构只释放剩余所有权。
字符串槽位继续由 owner 释放。批量输出遵循相同规则。
