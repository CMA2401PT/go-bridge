# 扩展、验证与排障

[文档导航](README.md) · [模块分工](generator-architecture.md) · [阶段计划](generator-plan.md)

## 改动落点

先判断改动属于哪条规则，再定位函数，避免把新能力全部堆进某个输出器。

| 要改变的行为 | 首先查看 | 同时验证 |
| --- | --- | --- |
| 包定位、多文件/构建条件 | [gomodel.Load](../internal/gomodel/load.go) | 跨文件与构建选择测试 |
| 新的类型依赖、方法/字段信息 | `visit`、`named`、`fields`、`methods`、`tuple` | 闭包、类型身份、递归、接收者与选择类型 |
| Python 类型或名字 | [pyapi.Plan/basicName/planFunction](../internal/pyapi/model.go) | alias、缩写/关键字、冲突和公开签名 |
| C 表示、位宽或结果协议 | [abi.Plan/wireType](../internal/abi/model.go) | 两端签名、运行时转换、结果释放 |
| Go 初始化、无函数包导入 | `abi.InitSymbol`、`goSource`、`cppSource`、`pythonSource` | 头文件生成、实际加载动态库、导入等待 Go init 完成 |
| Go 导出操作 | [goOperation/goInput/goOutput](../internal/emit/adapter.go) | 生成的 Go 编译与目标 API 行为 |
| C++ 调用及结果接管 | [cppFunction/cppOwner](../internal/emit/bindings.go) | 正常与失败路径，所有结果的所有权 |
| 公共 `.pyi` / Python 导出 | [stubSource/pythonSource](../internal/emit/python.go) | 类型声明、`__all__`、内建名字被遮蔽的情况 |
| 通用参数转换与异常类型 | [C++ 运行时模板](../internal/emit/templates/runtime.cpp.tmpl) | 实际生成扩展的异常行为 |
| 文件清单/构建配置 | [emit.Files](../internal/emit/emit.go) 与 [build.py 模板](../internal/emit/templates/build.py) | 独立 module、本地 replace、实际编译导入 |
| 输出目录发布 | [checkOutput/publish](../internal/generate/generate.go) | 缺失/空/非空目录、符号链接、失败不发布 |

源码中简短的包注释负责描述本模块契约；本组文档负责跨模块关系和理由；测试负责证明
关键行为。三者不必逐行重复，但改变契约时应一起更新。

## 添加一种类型的顺序

以已实现的 slice 扩展过程为例，不能仅在类型映射处放行后直接拼接几段 C++。完整工作是：

1. 明确用户接口：构造输入、元素类型、是否复制、reslice/clone 的共享规则及可用操作。
2. 检查 Go 模型信息是否足够。slice 的 Elem 已存在，额外桥接类型依赖应在接口规划
   阶段补充，而不是无条件给所有 Go 类型添加指针。
3. 扩展 Python 模型，表达类、成员、构造与返回类型，统一完成命名和冲突检查。
4. 扩展 ABI 操作与所有权描述。Operation 包装原函数/方法，Slice/Class/Field 计划
   分别描述内建操作，Cell 明确句柄内保存的 Go 类型；两端使用同一份协议。
5. 分别扩展 Go 导出、C++ 包装和 Python 声明，最后用同一语义案例贯通测试。

某个类别尚未贯通时应保持明确拒绝，而不是生成缺少成员或不能编译的部分绑定。
named bool 的接口仍未定义；不要为完成代码而随意决定其 Python 表示。

## 需要守住的现有约定

- Go 类型身份使用 go/types，不用字符串名字或底层类型代替。
- 模型中查询 `*T` 方法集不自动产生 `*T` 类型依赖；struct `addr()` 的补型由
  pyapi.complete 明确执行。接收者与方法集选择类型保持分离。
- pyapi 集中决定公开名字。输出器打印已有名字，不另外加下划线或编号掩盖冲突。
- Go 和 C++ 消费同一个 ABI 描述。新增 Kind 时要检查 `goInput`、`goOutput`、
  `cppInput`、`cppOutput`：当前 default 分支按数值处理，新资源类型不能误落进去。
- 一个 C++ owner 在 Python 返回值转换前接管整个 ABI 结果，失败路径也释放未转换结果。
- 字符串严格解码；整数越界是 OverflowError，类型不符是 TypeError。
- Go 适配层不恢复 panic；未恢复的 Go panic 终止整个 Python 进程，不转换为 Python 异常。
  这些行为属于公开接口，不是任意可换的实现细节。
- 相同输入的输出应可重复，但模型 ID 和 ABI 操作序号不是跨版本稳定标识。

`53cfc4b` 的拆分方式应延续：一个函数完成一层或一个操作，类型映射用独立 helper，
按输出语言分文件，模板只承载共用运行时代码。helper 的边界应解释得清楚，而不是
仅为减少行数拆出大量没有独立含义的小片段。

## 测试分别证明什么

| 测试文件 | 关注的行为与关键测试 |
| --- | --- |
| [gomodel/model_test.go](../internal/gomodel/model_test.go) | 闭包与 alias、跨包同名类型；`TestMethodSetQueryDoesNotInventTypes`；01–14 的前端分析；独立 module 与构建条件 |
| [pyapi/model_test.go](../internal/pyapi/model_test.go) | Snake、参数名、alias、名字冲突与后端拒绝边界 |
| [gomodel/selection_test.go](../internal/gomodel/selection_test.go) | 提升字段的 Go 可见性、歧义、隐藏嵌入、指针路径和错误上下文 |
| [pyapi/classes_test.go](../internal/pyapi/classes_test.go) | 字段/方法/生成类名冲突、桥接与 Python 内建基类保留名 |
| [pyapi/any_test.go](../internal/pyapi/any_test.go)、[abi/any_test.go](../internal/abi/any_test.go) | any 转换的启用条件、类型身份、名字冲突、独立操作与 import 规划 |
| [emit/emit_test.go](../internal/emit/emit_test.go) | `TestSignaturesAgreeAcrossFiles` 对照 Go/C++/stub 关键片段；无函数包仍链接原包、加载扩展；void 不带结果和 panic 协议 |
| [generate/generate_test.go](../internal/generate/generate_test.go) | 生成确定性、输出保护；01–03 实际编译调用；独立 module、本地 replace、标量范围、字符串转换错误；独立 Python 子进程验证 panic 终止进程 |

对象原生测试分在 `integration_slice_test.go`、`integration_object_test.go` 和
`integration_pointer_test.go`，覆盖 04–09、句柄计数、GC、视图/复制/共享、多级重定向、
Go 环、nil 接收者和 nil 解引用的致命子进程。每个生成包的声明文件也执行 mypy strict。
`integration_channel_test.go` 验证例子 11、方向限制、阻塞/GIL、回调行为、对象组合与关闭 panic。
`integration_any_test.go` 验证例子 10、any 字段、pointer/slice 组合、命名类型、
未知动态值、失败清理、消费者类型收窄与 any holder 的并发释放。

`emit` 的文本断言能发现两端签名或导出名偏离，但不能替代编译。`gomodel` 能分析
01–14，也不能替代对应后端的端到端验收。反复制造解码失败能检查调用是否继续工作，
但不是独立的内存泄漏测量；资源类扩展仍需生命周期观测和相应测试。

在仓库根目录运行：

```sh
# 普通测试；没有显式 Python 配置时会跳过原生集成测试
go test ./...
go vet ./...

# 重新执行真实编译/导入验证，Python 需安装 pybind11、setuptools
GO_BRIDGE_PYTHON=python go test ./internal/generate -run TestIntegration -count=1 -v

# 该 Python 还安装了 mypy 时，同时验证生成的 .pyi
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/generate -run TestIntegration -count=1 -v
```

`buildProject` 给 Go 构建启用 cgocheck2 与 checkptr=2；`buildAndRun` 执行正常行为与 Python 异常测试。
`TestIntegrationPanicTerminatesProcess` 将各个致命调用放在独立 Python 子进程中，验证
panic、除零和 nil 解引用无法被 Python 捕获；panic(nil) 覆盖默认与 GODEBUG=panicnil=1
模式，并确认目标 Go 代码自身的 recover 仍有效。mypy 检查生成的 `.pyi`；脚本另执行
声明文件并检查部分注解，防止 int/tuple 等公开函数遮蔽内建类型。
`acceptance_test.go` 还以真实消费者代码验证 mypy 推断与错误拒绝，用递归对象检查
生成确定性；句柄测试用 race 验证并发创建/释放、浅复制和最终根计数，GC 后检查
字段视图仍可读取。纯 Go 句柄测试虽额外开启 checkptr/cgocheck2，但实际的缓冲区与
C 边界覆盖来自原生扩展测试。它不代表任意目标 Go 包都不存在数据竞争。

`TestIntegrationWithoutFunctions` 实际构建只有 alias 和没有公开声明的包，使用延迟
初始化写入文件来检查 import 返回前 Go init 已完成；`TestIntegrationPanicDuringImport`
在独立 Python 子进程中直接触发 init panic 并验证进程终止。

按变更范围选择测试：纯命名改动先运行 pyapi/emit；改变传输或运行时语义必须跑原生
集成测试；文档改动核对链接、关键符号和可执行命令。通过后只有出现新改动或疑点才
扩大或重复测试。

## 从失败位置定位

| 现象 | 起点 |
| --- | --- |
| Go 包加载失败、多包匹配、package main | `gomodel.Load`，检查 target、`-dir` 与 Go 环境 |
| “unsupported type” 并带字段/参数路径 | `gomodel.visit/named/tuple`，确认是支持域限制还是漏收集 |
| 无 Python 映射或成员命名冲突 | `pyapi.className/planClasses`，核对形态、方法集与保留名 |
| Python 名字冲突 | `namespace.claim` 及产生两个名字的声明 |
| 生成的 Go 格式化失败 | `emit.Files` 与对应 `adapter.go` 输出分支 |
| 构建 Go 动态库失败 | 生成的 `go/main.go`、原包依赖；确认原源码目录仍存在 |
| C++ 找不到头文件/符号或编译类型不符 | cgo 头文件、`abi.Operation`、`bindings.go` 及 `.build/native` |
| 导入错误、加载到旧扩展 | 当前解释器、`PYTHONPATH`、`.build/python` 中的库和扩展 |
| 错误异常类型或数值范围 | `runtime.cpp.tmpl` 及对应 ABI 位宽 |
| Go panic 导致 Python 进程退出 | 这是当前契约；按 Go runtime 输出的栈定位原函数 |
| 多返回值转换失败时资源未释放 | `cppOwner` 是否在任何结果转换前接管全部结果 |

`-dir` 只控制包解析的起点，`-out` 相对生成器进程当前目录解析。`bridge.json` 保存源码
目录而不快照整个 Go 构建环境；生成与构建之间修改源码、构建标签或平台设置时，应
重新生成并配套构建。可用[调用实例](generator-walkthrough.md)中的临时项目复现原生问题。

当前构建脚本面向 macOS/Linux、启用 GIL 的 CPython。macOS arm64 已实测；Linux
代码路径存在，但不应把本机测试结果表述为 Linux 已验收。
