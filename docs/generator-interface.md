# 接口包装与转换

[文档导航](README.md) · [any 装箱与断言](generator-any.md)

Go interface 保存一个动态类型及其值。Python 侧用声明的接口包装持有它，方法调用
直接进入 Go 接口的方法。返回对象的 Python 类型由 Go 签名确定，不随动态实现变化。
这使隐藏实现和外部包实现也能通过公开接口使用。

## 公开接口

所有接口包装及 GoAny 继承不可直接构造的 GoInterface。它提供 is_nil、as_any，并禁止
隐式真假判断。具体接口类不可继承；嵌入接口展开为方法集，不对应 Python 继承关系。
Python Protocol 仅检查结构，不能使普通 Python 对象成为 Go 实现，因此这里使用具体包装类。

转换与断言回答不同的问题：

| 操作 | 判断依据 | nil interface |
| --- | --- | --- |
| `Reader(value)` | 来源静态 Go 类型可赋值给 Reader | 来源接口可赋值时保留 nil |
| `Reader.from_any(value)` | 实际动态类型实现 Reader | 断言失败，抛 TypeError |
| `Reader.try_from_any(value)` | 同一动态断言 | 断言失败，返回 None |
| `value.as_any()` | 保存原动态值为 GoAny | 返回 GoAny.nil() |

构造只接受已映射的 Go 包装类型，`.pyi` 使用这些来源类型的 union。参数不会隐式做
构造转换。普通 Python 标量可能对应多个 Go 类型，需要先用 int_as_any 等函数显式选择。
断言接受 GoInterface，可直接从一个接口断言到另一个接口或具体类型，不必经 as_any 中转。

接口转换复制 Go 值：struct 和数组被复制，指针共享目标，容器及函数保留各自的共享规则。
接口的动态类型始终是具体类型。一个包含 nil 指针的接口不为 nil，取出指针后才得到
零句柄指针包装。所有方法调用释放 GIL，Go panic 不由桥接捕获。

error 没有特殊协议。`(T, error)` 返回 `(T包装或基础值, error包装)`，其 Error 方法
映射为 error()。调用者通过 is_nil 和普通方法读取这个值。

## 模型与生成

命名接口保留原 Go 类型和方法集，不展开为匿名底层接口。这样未导出方法仍属于原包，
可以支持带私有方法的公开接口。匿名接口只有全部方法导出时才能在适配包中重新表达。
类型约束接口不是运行时值，分析阶段拒绝。

`gomodel` 从公开方法签名扩展类型闭包。`pyapi.planInterfaces` 使用 types.AssignableTo
计算闭包内的构造来源，不把全部实现者加入闭包。`abi.InterfaceConversion` 分别规划
静态赋值操作；动态断言继续使用 AnyConversion。方法调用复用普通 Operation。

`interface_go.go` 生成 Go 的静态赋值。`interface_cpp.go` 注册公共基类与构造分派。
`interface_python.go` 写入来源类型 union。C++ 内部基类叫 InterfaceRef，避免和 cgo
头文件自带的 GoInterface 类型冲突；公开 Python 名字仍为 GoInterface。

输出句柄直接保存动态值，nil 为零。每个非零输出都有独立所有权，由已有 Ref 管理。
参数句柄借用；转换只在同步调用内读取它，返回包装不依赖来源 Python 对象存活。

## 使用与验证

可运行示例及命令见 [15_interface](../examples/15_interface/readme.md)。
`integration_interface_test.go` 从生成产物验证隐藏实现、结构体值复制、指针共享、
静态转换与动态断言的区别、命名标量、外部包接口、递归及带私有方法的接口和容器组合。
消费者 mypy strict 检查有效调用与预期不合法的来源类型；nil 方法 panic 在子进程中验证。
