# 阶段审查记录

用户要求每完成一部分，通过 byobu 中的 Claude 审查、讨论、修正后再提交。
审查对象是当前生成器与行为测试，不要求复制手写 expected 的实现。

## M3 前的架构讨论

Claude 建议用不透明 ID 代替地址句柄，并明确整体结果接管、批量缓冲区清零、转交后清空
原槽位。实现采用标准库 runtime/cgo.Handle，避免维护另一份 ID 表；所有权规则采纳。

类型操作采用各自独立的 ABI 计划（Slice，后续 Structure/Pointer），不把构造、缓冲区、
字段与函数统一塞进万能 Operation。共用的值转换与所有权仍由 ABI Type 描述。

类型闭包保留既定规则：只为 struct 自动补 *T；其他类型仅在已有 *T 时提供 ptr()。
推广字段属于 Go 事实，后续放到 gomodel，使用 Go 选择器规则处理未导出嵌入、遮蔽和歧义。

## M3：Ref 与 slice（审查通过）

Claude 用真实扩展复现了直接调用 `SliceInt.__new__` 后传参会访问未构造 C++ 内存的问题。
修正为统一的 checked_ref<T>：确认具体 Python 类型及对应 C++ holder 已构造之后才 cast，
函数参数、slice 接收者和批量元素均经过此入口。这里检查 Python 输入有效性，不恢复 Go panic。
已增加对应回归，正常 Go panic 的子进程测试仍保留。

同时加入模块专属 C++ 命名空间，验证两个生成库可在同一个 Python 进程中使用，跨模块
包装对象被拒绝，不交换 Go 句柄。04/05、字符串和嵌套元素、GC、批量错误释放、mypy 与
cgocheck2 的原生测试通过。

Claude 最终审查确认无阻塞。另采纳两项建议：具体类禁止 Python 子类化，确保 Self
声明与返回类型一致；Slice 禁用 reversed，避免它绕过不可迭代约定。
