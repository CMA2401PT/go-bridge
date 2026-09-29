# 生成器模块边界

## 数据流

```text
Go 包
  ↓ load（go/packages）
Go 模型（类型身份、字段、函数、方法、依赖）
  ↓ Python 接口规划（映射、命名、桥接依赖）
Python 接口模型 ───────────────────→ .pyi / __init__.py
  ↓ ABI 规划
ABI 描述 ─────────────────────────→ Go 适配层
  ↓ 与 Python 接口模型一起
C++ 包装类 / pybind11 注册
```

包加载、模型构造和输出文件写入分开；生成阶段先在内存里完成全部文件，再创建输出。
分析/验证失败不会留下半个 package。生成只接收一个目标包，不把依赖包的所有声明
也当作根 API；依赖中的类型只沿公开签名和字段进入闭包。

## 目录与责任

- `internal/gomodel`：包加载和 Go 事实模型，诊断与确定的类型遍历顺序。
  `model.go` 定义节点，`load.go` 负责加载与闭包构造（按类型、字段、方法集、签名拆分）。
- `internal/pyapi`：Python 类型/API 规划与命名规则；所有用户可见名字及冲突诊断集中于此。
- `internal/abi`：传输类型、导出操作、结果/异常所有权描述；包注释记录所有权约定。
- `internal/emit`：消费已验证模型，按产物拆分：`adapter.go`（Go cgo 适配层）、
  `bindings.go`（pybind11 C++）、`python.go`（`__init__.py`/`.pyi`）、`emit.go`（文件清单）。
  `templates/` 存放原样嵌入的 C++ 运行时与 `build.py`。
- `internal/generate`：连接流水线、输出目录约束；不推导类型语义。
- `main.go`：CLI 参数解析与错误展示。

通用运行时通过受版本控制的模板随产物生成，不依赖生成器程序运行。
模板承载协议实现，Go/C++ 类型专属操作由模型生成。避免模板内部再分析 Go AST。

## 类型身份和闭包

Go 类型在包加载时由 go/types 提供。alias 归一化后通过 types.Identical 判断身份；
不能用 Python 类名、Go 短名或底层类型代替身份。类型节点先登记，再展开依赖，
因此递归 struct/指针不会无限递归。收集导出方法时同时记录值与指针方法集；不能
通过简单删除 `*` 推导多级指针的方法集。方法声明的接收者与查询方法集的类型分开
记录：查询 `*T` 的方法集不意味着源码真的使用了 `*T`。提升自非导出嵌入类型的
公开方法可以通过公开 owner 调用，无须为那个隐藏的接收者生成包装类型。

Go 模型记录直接导出字段；Python 模型还需展开提升成员，构造只接收直接字段，
提升成员由 Go selector 的可见性和歧义规则决定。未导出字段保留在原 Go 值内部，
不成为 Python API。字段展开属于后续 struct 阶段，前端已用真实方法集收集提升方法。

同包导出类型是根，即便没有函数使用也要分析。非导出类型若出现在公开 API 中，
首版报告不支持；暂不通过导出 alias 追溯这种类型的可见名字，也不使用 unsafe 或反射绕开 Go 可见性。
真正的 alias 不新增包装类型，导出的 alias 名可以重新导出同一 Python 类型。

## 命名

Python 类型保留 Go 类型名；函数、方法、字段和参数转 snake_case，连续缩写作为
一个单词（HTTPServer → http_server），数字保留在原词内。Python 关键字加 `_`。
`value` 等桥接保留成员也按统一规则避让，struct 与其指针使用相同字段名。

两个不同声明经过规则转换后冲突时，报告两个来源；不靠遍历顺序自动编号。
生成器内部的 ABI 符号可以使用稳定序号，与用户可见名字分离。

## 所有权与异常

C ABI 只传基础值、显式长度的缓冲区和不透明句柄。Go 数据不由 C++ 直接解引用。
每个结果必须规定谁负责释放；多返回值在 Python 转换失败时也释放所有已分配结果。
所有调用 G 或会解引用/索引 Go 对象的导出操作都应拦截可恢复 panic；fatal runtime
error 和其他 goroutine 的 panic 不承诺可恢复。panic 的表示与普通 string 结果独立。

GIL 默认保持；同步调用中的参数由 Python/C++ 持有到调用结束。Go 包内部的并发
仍由目标包自行负责，不通过 holder 锁伪装成对象并发安全。

## 输出与构建

输出目录是一个独立生成项目，包含 `go/`、`bindings.cpp`、Python package 和
`build.py`。生成与编译分离；构建入口使用执行它的 Python 解释器。
首版拒绝覆盖非空目录，避免覆盖用户文件和残留旧 API。输出记录目标 Go 包的构建
上下文；构建需要原始源码及其依赖可用，不复制目标包。

实际已实现的布局和命令随阶段落地记录在仓库首页。旧 `gen/` 不提供兼容层，替换
时删除旧接口和硬编码入口。

## M2 已实现的基础值 ABI

每个函数有一个稳定序号的导出符号，参数使用明确宽度整数、float/double 或
`SBString {char *data; size_t size;}`。返回值统一是该操作的结果结构，包含
`panicked`、`panic_message` 及各个结果字段；无返回值函数仍有异常字段。
`int`/`uint`/`uintptr` 使用指针宽度类型，C++ 静态检查生成时的 Go int 宽度。

输入 string 借用 Python UTF-8 缓冲区，Go 复制后才交给目标包；结果字符串由 Go
适配层分配 C 内存，C++ RAII 在正常、panic 和 Python 解码失败路径都释放。多返回
值的所有字符串在第一次 Python 转换前即归入同一个所有者。普通字符串严格解码
UTF-8；panic 文本中的非法 UTF-8 用替换字符呈现，仍抛 GoPanic。

panic 标志与消息长度独立，因此 `panic("")` 不会遗漏。Go 适配层用完成标志区分
正常返回与展开过程，使 `GODEBUG=panicnil=1` 下的 `panic(nil)` 也不会被吞掉。
Go 异常只能恢复当前调用 goroutine 中可 recover 的 panic。

Python bool 参数只接受 bool；整数参数接受 int（含 bool 子类关系）并检查目标
Go 位宽，越界抛 TypeError。浮点参数接受 Python float/int，float32 按 Go 的宽度
舍入；大整数转换为 Python double 失败会抛 OverflowError。字符串只接受 str。

生成的 `.pyi` 使用 `_builtins.int` 等限定名字，避免公开函数 `Int` → `int` 遮蔽
基础类型。函数参数采用原始 Go 参数名经 snake_case 转换的结果；无名或 `_` 参数
使用 arg1、arg2 等位置名，因此不保证与手写样例使用的参数名相同。
