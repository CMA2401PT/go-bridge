# 生成器维护文档

这组文档讲解生成器的实现：模块分工、模型、运行时协议，以及怎样扩展和验证。

## 阅读顺序

| 文档 | 回答的问题 |
| --- | --- |
| [分工与设计](generator-architecture.md) | 流水线怎样分层，每个模块负责什么，为什么这样拆分？ |
| [模型与类型闭包](generator-models.md) | Go 类型、Python 接口和 ABI 分别记录什么，递归与方法集怎样处理？ |
| [一次生成与调用](generator-walkthrough.md) | 一个具体 Go 函数怎样变成 `.pyi`、C 导出和 Python 可调用函数？ |
| [对象、句柄与类型操作](generator-objects.md) | Ref 如何管理资源，named/struct/slice/pointer 怎样映射？ |
| [any 装箱与断言](generator-any.md) | GoAny 如何持有动态值，显式转换及 pointer/slice 组合怎样生成？ |
| [接口包装与转换](generator-interface.md) | interface 返回什么对象，静态转换和动态断言有什么区别？ |
| [channel 与回调](generator-channel.md) | channel 如何收发、限制方向，并在等待期间释放 GIL？ |
| [ABI、所有权与异常](generator-runtime.md) | 数据如何过边界，字符串由谁释放，panic 和转换错误怎样传播？ |
| [扩展、验证与排障](generator-maintenance.md) | 添加能力该改哪些地方，哪些测试能证明什么，失败时从哪里查？ |
| [01–15 验收](generator-acceptance.md) | 哪些行为已经实测，怎样复现，哪些环境仍未验证？ |
| [实施计划](generator-plan.md) | 已完成哪些阶段，剩下哪些工作？ |

第一次读代码，先看分工，再顺着“一次生成与调用”中的函数入口阅读。准备修改某条
规则时，先看模型和运行时约定，再用维护指南选择验证范围。

## 当前能力

生成器已贯通 examples 01–15：基础 bool、整数、浮点数、string、真正 alias、
函数和多返回值、字节和嵌套 slice，定长数组的值复制与视图，以及 map 的键比较、共享与批量读取。
Go panic 不由桥接层捕获，未恢复的 panic 终止整个 Python 进程。named 类型、struct、
字段/方法、普通/多级/命名指针与 typed nil 已通过 06–09 的原生测试。只有 alias 或没有公开声明的包也可构建导入，
导入会通过内部初始化入口等待 Go init 完成。
any/interface{} 映射为 GoAny，支持显式装箱／断言、any 字段、`*any`、`[]any`，
以及未知动态类型的持有和传回。一般 interface 支持 Go 方法调用、静态转换和动态断言，
共用 GoInterface 基类，error 是普通接口值。
Go 函数值支持持有、调用和传回 Go，包括命名类型和闭包状态。
channel 支持三种方向、命名类型与同步接收回调，及其与既有对象类型的组合。

“类型闭包中有这个节点”与“可以为这个类型生成可运行绑定”是两个不同的验收条件。
未实现能力的语义目标记在实施计划中，不能当作已经存在的代码。

## 文档与代码如何保持一致

- [仓库首页](../readme.md) 负责使用入口、当前能力和环境要求。
- 本目录的主题文档负责设计理由、模型约定和代码阅读路径。
- [examples](../examples/readme.md) 保留手写行为参考；部分样例的规则与生成器不同，见下文。
- 运行行为以当前实现与测试为证据。若文档和代码冲突，应查清设计意图并一起修正，
  不以“生成文件已经这样写了”作为保留错误的理由。

例如生成器中整数越界是 `OverflowError`，类型不符是 `TypeError`；部分手写样例在越界时
仍期待 `TypeError`。把样例断言移入生成器验收前，先对照生成器的规则。
样例与生成器均直接传播 Go panic，由 Go runtime 终止宿主进程。

代码链接指向源文件，正文用函数名定位，避免维护易失效的硬编码行号。
