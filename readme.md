# go-bridge

`go-bridge` 的长期目标是提供一个代码生成工具，让 Python 可以自然地调用 Go
代码编译得到的库。

本文使用以下几个简称：

- **G**：作为输入的 Go 代码。
- **C**：代码生成工具（不是 C 语言）。
- **P**：C 根据 G 生成的 pybind11/C++ 扩展、Python package 和 `.pyi` 类型声明。

目标流程可以概括为：

```text
Go 代码 G ────────────────> 编译后的动态库
    │                            │
    └──── 输入生成工具 C ───> 包装代码 P
                                 │
                                 └──── Python 用户调用 Go API
```

实际落地时，还需要生成一层 Go 导出适配代码，将原始 Go API 转换为稳定的
C ABI；pybind11 扩展再调用该 ABI，并向 Python 暴露 CPython 原生扩展接口。
项目的目标实现是 pybind11 路线，而不是基于 `ctypes` 的运行时调用。生成结果
还必须包含 `.pyi`，让 IDE 和静态类型检查器获得明确的类型信息。

## 当前阶段：根据样例实现生成器

生成器已覆盖 examples 01–15，并完成原生编译、行为与类型声明验收。完整路线、阶段状态与支持范围见
[`docs/generator-plan.md`](docs/generator-plan.md)，模块边界见
[`docs/generator-architecture.md`](docs/generator-architecture.md)。每个可验证阶段单独提交。

维护代码从[文档导航](docs/README.md)开始：按分工、模型、完整调用实例、运行时协议、
扩展与排障逐步阅读。文档对应当前实现，并明确标出尚未落地的后续设计。

examples 先行研究的问题是：对于一组有代表性的 Go 代码 G，理想的 Python
包装 P 究竟应该是什么样，才能正确、安全并且符合 Python 的使用习惯？

`examples/` 是行为参考。每个 example 都应先人工完成一条端到端
链路：

1. 编写一个范围明确、具有代表性的 Go API。
2. 手工设计 Go 到动态库 ABI 的导出适配层。
3. 手工编写理想的 pybind11/C++ 扩展、Python package 和 `.pyi`。
4. 编译并运行测试，验证行为、类型转换、错误处理和内存管理。
5. 在文档中记录设计选择、已知限制和可推广的映射规则。

这些人工验证过的案例是生成器的“黄金样例”。生成产物不要求与手写代码逐字一致；
nil 表示参考 09，所有样例与生成器均不恢复 Go panic。具体能力以生成器已通过的
端到端测试为准，不能把计划中的能力当作已经实现。

更详细的 example 规范和建议覆盖范围见 [`examples/readme.md`](examples/readme.md)。

## 当前可用的生成器

目前后端支持基础 bool、所有整数宽度（含 int/uint/uintptr）、float32/float64、
string，以及由这些类型组成的函数参数和任意数量返回值。这些基础类型的 Go alias
保留为 Python 类型别名。已支持 Ref、字节/一般/嵌套 slice 的构造、索引、共享切片、
clone 与批量复制。生成调用不捕获 Go panic：未恢复的 panic（含 `panic(nil)`）
终止整个 Python 进程，不能用 Python `try/except` 捕获。详见[运行时契约](docs/generator-runtime.md)。

已支持命名标量与 slice、struct 构造/字段/提升字段/方法、普通/多级/命名指针，以及
同类包装的 typed nil。any/interface{} 使用 GoAny 显式装箱和断言，支持 any 字段、
`*any` 与 `[]any`；未知动态类型可持有和传回。生成器原生测试覆盖 examples 01–15，
行为细节见[对象协议](docs/generator-objects.md)和[any 协议](docs/generator-any.md)。
channel 支持三种方向、命名类型和同步 Python 接收回调，收发及原 Go API 调用释放 GIL。
用法见 [11_channel](examples/11_channel/readme.md)。map 支持 Go 键比较、共享、浅复制、
nil 和批量键值对读取，见 [12_map](examples/12_map/readme.md)。定长数组保留 Go 值语义，
支持复制、变量视图与取址，见 [13_array](examples/13_array/readme.md)。Go 函数值可持有、
调用和传回，闭包保持共享，见 [14_func](examples/14_func/readme.md)。一般 interface
支持 Go 实现的方法调用、显式转换和动态断言，error 保持普通接口值，见
[15_interface](examples/15_interface/readme.md)。
泛型、可变参数、导出变量/常量及 named bool 仍明确拒绝。

从仓库根目录运行：

```sh
go run . -out generated/basic ./examples/01_basic
python generated/basic/build.py
PYTHONPATH=generated/basic/.build/python python -c 'import go_basic_value as b; print(b.int_in_and_out(1))'
```

构建用的 Python 需安装 `pybind11`、`setuptools`；导入时使用同一解释器。
当前实际验证环境为 macOS arm64、Go 1.27.1、CPython 3.13、pybind11 3.0.1。
构建脚本也提供 Linux 路径，尚未在本轮执行 Linux 验证。仓库要求 Go 1.25.5 或更新。

CLI 接受一个包目录或 import path：

```sh
go build -o /tmp/go-bridge .
/tmp/go-bridge -out /tmp/my-bindings -name my_python_package /path/to/go/package
/tmp/go-bridge -dir /path/to/go/module -out /tmp/my-bindings-other example.org/module/pkg
```

`-name` 默认为 `go_<包名的 snake_case>`。输出目录必须不存在或为空，生成器不覆盖
已有项目。生成与构建分离；输出里的 `bridge.json` 记录原始源码目录，构建仍依赖
原 Go module/workspace。输出不是可独立分发的 wheel，也不承诺搬迁源码后仍能构建。

```text
generated/basic/
├── go/main.go             # Go C ABI 适配层
├── bindings.cpp           # pybind11 绑定
├── go_basic_value/        # __init__.py、__init__.pyi、py.typed
├── bridge.json            # 构建上下文
├── build.py
└── .build/python/         # 构建后可导入的 package
```

验证命令：

```sh
go test ./...
go vet ./...
GO_BRIDGE_PYTHON=python go test ./internal/generate -run TestIntegration -v
# 解释器还安装了 mypy 时，同时检查生成的 .pyi：
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/generate -run TestIntegration -v
```

集成测试从生成产物编译和调用，不依赖 `expected/`；包括 01–15 与独立临时 module、跨包类型、组合资源与 panic 子进程。
消费者类型检查、holder race、原生 cgocheck2/checkptr 及未验证范围见[验收记录](docs/generator-acceptance.md)。
未设置 `GO_BRIDGE_PYTHON` 时，常规 Go 测试会明确跳过原生编译集成测试。

## 每个 example 要回答的问题

一个完整案例应区分三层接口：

```text
原始 Go API
    ↓
动态库边界上的 ABI
    ↓
Python 用户最终看到的 API
```

它至少需要回答：

- Go 类型在 ABI 层使用什么稳定表示？
- Python 侧暴露什么类型和调用方式？
- `.pyi` 应如何准确表达参数、返回值、重载和可空性？
- 输入和返回值是否发生复制，内存由谁分配、由谁释放？
- Go 对象跨调用存活时，句柄和生命周期如何管理？
- `error`、`panic` 和空值如何映射为 Python 行为？
- 哪些 Go 语义不能直接跨越 ABI，需要适配、限制或明确拒绝？
- 方案在 Linux、macOS 和 Windows 上有哪些差异？

能够“调用成功”只是最低要求；接口语义、所有权和异常路径同样属于案例的正确性。

## 建议的探索顺序

examples 可以由简单到复杂逐步覆盖：

1. 固定宽度数值和布尔值。
2. 字符串与字节序列。
3. slice、数组和多返回值。
4. `error`、`panic` 与 Python 异常。
5. struct、命名类型和枚举式常量。
6. 指针、可空值以及对象句柄。
7. 方法、构造函数和对象生命周期。
8. callback、goroutine、线程与 GIL。
9. map、interface、泛型等难以直接跨 ABI 的类型。

这里的顺序是研究路线，不代表这些能力已经实现。

## 最终期望的 Python 体验

例如输入的 Go 包含：

```go
package person

import "fmt"

type School struct {
	Name     string
	Age      int
	Duration int
}

type Person struct {
	Name    string
	Age     int
	Schools []School
}

func (p Person) SayName() {
	fmt.Println("I am ", p.Name)
}

func NewPerson(name string, age int, schools []School) *Person {
	return &Person{Name: name, Age: age, Schools: schools}
}
```

当前接口按如下方式使用（Go 字段 Value 等桥接保留名会加下划线）：

```python
import go_person as person

alice = person.new_person(
    "alice",
    16,
    person.SliceSchool([person.School(name="PrimarySchool", age=8, duration=10)]),
)
print(alice.name)
alice.name = "Alice"
alice.say_name()
```

struct、slice、指针和方法已有生成器端到端测试；这段代码用于说明它们的组合方式。

## 给后续贡献者和 agent 的工作原则

- 在映射规则尚未由 example 验证前，不要把当前原型当作既定架构。
- 新增能力时，优先补充一个最小、可运行、带文档的 example。
- 清楚区分“原始 Go 代码”“Go/ABI 适配层”“pybind11 扩展”和“Python package”。
- 不要只验证正常返回；同时验证边界值、错误路径、资源释放和重复调用。
- 从多个黄金样例中提炼出稳定规则后，再将规则实现进生成器。

旧 `gen/` 和硬编码生成入口已删除，新实现位于 `internal/`。`generated/` 用于本地
产物，不提交；`examples/expected/` 继续作为手写行为参考维护。
