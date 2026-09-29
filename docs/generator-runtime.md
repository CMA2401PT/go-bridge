# C ABI、所有权与异常

[文档导航](README.md) · [调用实例](generator-walkthrough.md) · [验证与排障](generator-maintenance.md)

本页描述已经实现的基础值协议。Ref/holder、句柄、nil 指针和 slice 批量传输尚未进入
生成后端，不应根据手写 examples 推断这里已经支持这些路径。

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

所有操作返回自己的 SBResultN 结构。即使 Go 函数没有返回值，结构中仍然有 panicked
和 panic_message。零个结果映射成 None，一个结果直接返回，多个结果按顺序构造 tuple。

| 资源 | 分配/提供方 | 使用期限 | 释放方 |
| --- | --- | --- | --- |
| 输入 SBString.data | Python 的 UTF-8 缓存 | 同步调用期间借用 | Python 管理，Go/C++ 不释放 |
| `bridgeInput` 得到的 Go string | Go 复制输入 | 原包可以保留该 string | Go GC |
| 普通结果 SBString.data | `bridgeString` 调用 C.CString | 到结果转换结束或失败 | C++ owner 的 std::free；Go panic 清理可能提前释放并置零 |
| panic_message.data | `bridgePanic` → C.CString | 到异常消息转换结束或失败 | C++ owner 的 std::free |
| 数值结果 | 按值返回 | 随结果结构 | 无需释放 |

字符串长度是字节数，解码不依赖终止 NUL，所以内嵌 `\0` 不会截断字符串。空字符串
也遵循相同的分配/释放协议。SBString 只是内部传输结构，不是向任意外部 C 调用者开放
的安全缓冲区接口；当前适配层信任生成的 C++ 传入有效指针和长度。

[cppOwner](../internal/emit/bindings.go) 为每种结果生成一个 owner。它在任何 Python
返回对象创建前接管整个结果，析构时无条件释放 panic_message 和所有 String 结果。
例如结果 `(非法 UTF-8, 第二个字符串)` 解码第一个就失败，第二个也由同一 owner 释放。
这个约定不能改成“转换到哪个结果才开始拥有哪个结果”。

生成代码不主动释放 GIL，输入 Python 对象及 UTF-8 缓存在同步调用期间有效。Go 会
复制输入字符串后才调用原函数；原包不会因为保留 string 而持有 Python 内存。

## Go panic 怎样返回

完整逻辑由 `goOperation` 生成，`adapterRuntime` 提供 `bridgePanic`：

1. 命名结果 out 初始为零值，complete 初始为 false，并安装 defer。
2. 转换输入、调用原 Go 函数、逐个转换输出。
3. 正常完成后设置 complete 为 true，再返回。
4. 如果未完成就开始栈展开，defer 直接调用 recover，清理已转换的 String 结果并置零，
   写入 panicked 和 panic_message。

不能只用 `recover() != nil` 判断是否发生 panic：`GODEBUG=panicnil=1` 下 panic(nil)
的 recover 值可能为 nil，完成标志仍能识别它。异常标志也不能用消息长度代替，空字符串
本身就是合法 panic 值。

C++ 先检查 panicked。为保留 GoPanic 类型，panic 文本中的非法 UTF-8 使用替换字符
解码，再通过 `PyErr_SetObject` 设置异常。普通结果采用严格解码，非法 UTF-8 会产生
UnicodeDecodeError。GoPanic 是生成模块内创建的 RuntimeError 子类，目前只携带消息，
不保留原 panic 值类型或 Go 栈。

Go 清理时置零的字段仍会经过 C++ owner 析构，free(nullptr) 无操作。因此正常结果、
panic、Python 解码失败都遵循同一所有权入口。

这个恢复范围是适配函数内、当前调用 goroutine 的可恢复 panic。目标包 init 阶段、
其他 goroutine 的 panic、运行时 fatal error 不在此保证内。C 内存分配失败等终止进程
的行为也不能靠这层 GoPanic 转换解决。

## 错误发生在哪一层

| 条件 | 对外结果 | 原 Go 函数是否执行 |
| --- | --- | --- |
| bool 参数收到整数，string 参数收到 bytes 等 | TypeError | 否 |
| 整数超出目标范围，含负数传给无符号类型 | OverflowError | 否 |
| str 含无法编码的孤立代理字符 | UnicodeEncodeError | 否 |
| 巨大 Python int 无法转为 double | OverflowError | 否 |
| 原函数可恢复 panic | GoPanic | 已进入 |
| 普通结果含非法 UTF-8 | UnicodeDecodeError | 已正常返回 |

`1309094` 将整数范围错误从 TypeError 修正为 OverflowError；与类型错误的区分现在由
`integer_arg` / `integer_overflow` 实现，并由生成器集成测试验证。

## ABI 的稳定范围

生成 Go 动态库与 C++ 扩展必须配套重建。SBCallN 是当前操作顺序的内部名字，增删函数
可能使序号变化，没有独立动态库的版本协商或长期二进制兼容协议。C++ 使用 cgo 本次构建
产生的头文件，不另维护一份手写头文件。

下一阶段增加 Ref 时，必须明确新句柄什么时候归 C++ 所有、零值意味着什么、部分结果
失败怎样回收。现有 String 的规则只是已完成的一种资源协议，不能直接代替句柄协议。
