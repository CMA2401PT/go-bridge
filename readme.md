# go-bridge

`go-bridge` 的长期目标是提供一个代码生成工具，让 Python 可以自然地调用 Go
代码编译得到的库。

本文使用以下几个简称：

- **G**：作为输入的 Go 代码。
- **C**：未来要实现的代码生成工具（不是 C 语言）。
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

## 当前阶段：先定义正确答案

**当前阶段不以设计或完善生成工具 C 为主。**

仓库现在首先要解决的问题是：对于一组有代表性的 Go 代码 G，理想的 Python
包装 P 究竟应该是什么样，才能正确、安全并且符合 Python 的使用习惯？

因此，`examples/` 是当前工作的中心。每个 example 都应先人工完成一条端到端
链路：

1. 编写一个范围明确、具有代表性的 Go API。
2. 手工设计 Go 到动态库 ABI 的导出适配层。
3. 手工编写理想的 pybind11/C++ 扩展、Python package 和 `.pyi`。
4. 编译并运行测试，验证行为、类型转换、错误处理和内存管理。
5. 在文档中记录设计选择、已知限制和可推广的映射规则。

这些人工验证过的案例就是未来生成器的“黄金样例”。等案例覆盖足够广、映射
规则趋于稳定后，再据此设计中间表示、解析器和代码生成器。

更详细的 example 规范和建议覆盖范围见 [`examples/readme.md`](examples/readme.md)。

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

未来生成器应尽量让 Python 用户以自然的方式使用它：

```python
import go_person as person

alice = person.NewPerson(
    "alice",
    16,
    [person.School(Name="PrimarySchool", Age=8, Duration=10)],
)
print(alice.Name)
alice.Name = "Alice"
alice.SayName()
```

这段代码表达的是长期 API 体验目标，并不表示其中涉及的 struct、slice、指针和
方法已经获得可靠支持。

## 给后续贡献者和 agent 的工作原则

- 在映射规则尚未由 example 验证前，不要把当前原型当作既定架构。
- 新增能力时，优先补充一个最小、可运行、带文档的 example。
- 清楚区分“原始 Go 代码”“Go/ABI 适配层”“pybind11 扩展”和“Python package”。
- 不要只验证正常返回；同时验证边界值、错误路径、资源释放和重复调用。
- 从多个黄金样例中提炼出稳定规则后，再将规则实现进生成器。

仓库中的 `gen/`、`generated/` 和 `target_go/` 是早期探索或实验材料，可以用于
理解问题，但当前不代表最终设计。
