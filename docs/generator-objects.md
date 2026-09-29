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

## 后续类型补全规则

Go 模型只描述语言事实。Python 规划为 struct 的 addr() 补 *T；其他类型仅在闭包
已存在 *T 时提供 ptr()，不递归创造无限指针链。推广字段的可见性属于 Go 事实，后续
由 gomodel 用 Go 选择器规则收集；Python 层只负责命名及冲突检查。

具体包装类在 C++ 中使用模块专属命名空间，避免多个生成模块的类型注册和 caster 混淆。
模块可以同时导入，但不能互传包装对象。调用前统一检查包装对象的具体类型及其 C++
holder 是否构造；直接 __new__ 得到的未初始化对象抛 TypeError，不读取未构造内存。
