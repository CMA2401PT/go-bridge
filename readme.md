# go-bridge

go-bridge 读取一个 Go 包，生成一个 Python 原生扩展包，让 Python 可以直接调用这个
Go 包里的函数、类型和方法，并带有完整的 `.pyi` 类型声明。

它的工作方式是：先为 Go 包生成一层导出 C ABI 的 Go 适配代码，编译成动态库；
再生成调用这个动态库的 pybind11/C++ 扩展和 Python package。整个调用走 CPython
原生扩展，不经过 `ctypes`。

```text
你的 Go 包 ──go-bridge──> 生成目录 ──python build.py──> 可 import 的 Python 包
                          ├── go/main.go      Go 适配层（导出 C ABI）
                          ├── bindings.cpp    pybind11 扩展
                          └── go_xxx/         __init__.py / __init__.pyi
```

## 一个完整的例子

假设你有这样一个 Go 包：

```go
// person/person.go
package person

import "fmt"

type School struct {
	Name     string
	Duration int
}

type Person struct {
	Name    string
	Age     int
	Schools []School
}

func NewPerson(name string, age int, schools []School) *Person {
	return &Person{Name: name, Age: age, Schools: schools}
}

func (p *Person) Greet() string {
	return fmt.Sprintf("Hi, I am %s, %d years old", p.Name, p.Age)
}

func (p *Person) Birthday() { p.Age++ }

func ParseAge(text string) (int, error) {
	var age int
	if _, err := fmt.Sscanf(text, "%d", &age); err != nil {
		return 0, err
	}
	return age, nil
}
```

生成后，Python 里这样用：

```python
import go_person as person

alice = person.new_person(
    "alice",
    16,
    person.SliceSchool([person.School(name="Sunshine Primary", duration=6)]),
)
print(alice.greet())        # Hi, I am alice, 16 years old
alice.birthday()            # 调用 Go 的指针方法，修改的是同一个 Go 对象
alice.name = "Alice"        # 直接读写 Go struct 字段
print(alice.greet())        # Hi, I am Alice, 17 years old
print(alice.schools[0].name, len(alice.schools))  # Sunshine Primary 1

age, err = person.parse_age("forty-two")
if not err.is_nil:
    print("failed:", err.error())  # failed: expected integer
```

这段代码能通过 `mypy --strict`：IDE 能补全 `alice.greet`，也能知道
`parse_age` 返回 `tuple[int, error]`。

## 快速上手

### 环境要求

- Go 1.25.5 或更新，cgo 可用（需要 C/C++ 编译器）。
- Python 3，并安装 `pybind11` 和 `setuptools`：`pip install pybind11 setuptools`。
- macOS 或 Linux。实际验证过的环境是 macOS arm64、Go 1.27.1、CPython 3.13、
  pybind11 3.0.1；Linux 路径已写好但尚未实测，Windows 不支持。

### 第一步：编译 go-bridge

在本仓库根目录：

```sh
go build -o /tmp/go-bridge .
```

### 第二步：为你的 Go 包生成绑定

在你自己的 Go module 里（即上面 `person` 包所在的 module 根目录）：

```sh
/tmp/go-bridge -out ./bindings ./person
```

Python 包名默认是 `go_` 加上 Go 包名的 snake_case，这里就是 `go_person`。
可以用 `-name` 改名。也可以不进入 module 目录，用 `-dir` 指定从哪里加载 Go 包：

```sh
/tmp/go-bridge -dir /path/to/your/module -out ./bindings -name my_pkg example.com/demo/person
```

`-out` 必须是不存在或为空的目录，生成器不会覆盖已有文件。

### 第三步：构建并导入

```sh
python bindings/build.py
PYTHONPATH=bindings/.build/python python -c 'import go_person; print(go_person.parse_age("42")[0])'
```

`build.py` 会编译 Go 动态库和 pybind11 扩展，把可导入的包放到
`bindings/.build/python/go_person/`。构建和导入要用同一个 Python 解释器，
因为扩展是针对这个解释器编译的。

构建时仍然需要原来的 Go 源码：生成目录里的 `bridge.json` 记录了源码位置。
所以生成目录不是可以单独分发的 wheel，Go 源码改动后也要重新生成并重新构建。

## Go API 在 Python 里长什么样

下面这些规则决定了你写 Python 时的手感，大多数"为什么报错"都能在这里找到答案。

**命名改成 Python 风格。** 函数、方法、字段都转成 snake_case：`NewPerson` →
`new_person`，`Schools` → `schools`。类型名保持原样，组合类型按结构命名：
`[]School` → `SliceSchool`，`*Person` → `PtrPerson`，`map[string]int` → `MapStringInt`。
字段名与桥接层保留名（如 `value`）冲突时会加下划线。

**基础类型直接用 Python 值。** `bool`、各宽度整数、浮点数、`string` 直接传 Python
的 `bool`/`int`/`float`/`str`。整数超出 Go 类型范围抛 `OverflowError`，类型不对抛
`TypeError`。多返回值变成 tuple。

**其他 Go 值是 Python 包装对象，必须显式构造。** slice、struct、指针、map 等在
Python 里是持有 Go 对象的包装（`Ref`）。传参时要传对应的包装类型，不会自动把
Python list 转成 Go slice：

```python
person.new_person("bob", 3, person.SliceSchool([]))  # 正确
person.new_person("bob", 3, [])                      # TypeError
```

这是为了保留 Go 的共享语义：多个 Python 包装可以指向同一个 Go slice 或 map，
一边修改另一边可见，和在 Go 里一样。需要 Python 原生数据时，用
`copy_to_py_list()` 之类的方法显式复制出来。

**struct 是一个 Go 变量，可以取地址。** `person.Person(name="bob", age=3)` 创建一个
Go struct 变量，`.addr()` 得到 `PtrPerson`。Go 中指针接收者的方法在两者上都能调用。

**nil 是带类型的值。** 指针、map、channel、函数值和 interface 的 nil 用
`PtrPerson.nil()` 这样的类方法构造，用 `.is_nil` 判断。这些包装不能直接放进
`if`，因为"是 nil"和"是零值"在 Go 里是两回事。

**error 是普通返回值，不是异常。** Go 的 `error` 和其他 interface 一样返回一个包装，
用 `err.is_nil` 判断是否出错、`err.error()` 取消息。这和 Go 本身的写法一致。

**Go panic 会直接终止 Python 进程。** 桥接层不恢复 panic，`try/except` 抓不到。
Go 代码里的 nil 指针解引用、越界访问都属于这一类，调用前需要像在 Go 里一样自己保证前提。

**any 需要显式装箱和断言。** `any`/`interface{}` 映射为 `GoAny`，用
`int_as_any(1)` 装箱，用 `any_as_int(v)` 或 `try_any_as_int(v)` 取回。

**耗时调用会释放 GIL。** channel 收发和原 Go 函数调用期间释放 GIL，其他 Python
线程可以继续运行。

## 支持哪些 Go 类型

每一项都有一个最小样例，写明了 Python 侧的 API、复制与共享语义和限制：

| Go 类型 | Python 侧 | 样例 |
| --- | --- | --- |
| bool、整数、浮点数、string 及其 alias | Python 原生值 | [01](examples/01_basic/readme.md)、[03](examples/03_str/readme.md) |
| 多返回值 | tuple | [02](examples/02_multi_return/readme.md) |
| `[]byte`、`[]T`、嵌套 slice | `SliceT`，共享底层数组 | [04](examples/04_bytes_and_holder/readme.md)、[05](examples/05_slice/readme.md) |
| 命名类型、struct、方法、提升字段 | 同名类 | [06](examples/06_named_type/readme.md) |
| 指针、多级指针、命名指针 | `PtrT` | [07](examples/07_ptr_basic/readme.md)、[08](examples/08_multi_ptr/readme.md) |
| typed nil | `.nil()` / `.is_nil` | [09](examples/09_nil/readme.md) |
| `any` / `interface{}` | `GoAny` | [10](examples/10_any/readme.md) |
| channel（三种方向） | `ChanT`，用 Python 回调接收 | [11](examples/11_channel/readme.md) |
| map | `MapKV`，由 Go 比较键 | [12](examples/12_map/readme.md) |
| 定长数组 | `ArrayNT`，值语义 | [13](examples/13_array/readme.md) |
| 函数值、闭包 | 可调用对象 | [14](examples/14_func/readme.md) |
| 一般 interface、error | 接口包装，可调用方法与断言 | [15](examples/15_interface/readme.md) |

完整的映射规则见 [类型映射总览](examples/types.md)。

以下 Go 特性暂不支持：泛型、可变参数函数、导出变量和常量、命名 bool 类型、
匿名 struct，以及用 Python 对象实现 Go interface 或作为 Go 回调函数传入。
如果这些类型只在包内部使用，不影响生成；一旦出现在导出 API 里，生成器会报告
具体是哪个声明、字段或参数，然后停止，而不是悄悄跳过。

## 先跑一下仓库自带的样例

不想自己写 Go 包，可以先看 `examples/` 里的样例。每个样例目录都有一个 `demo.py`，
构建后直接运行就能看到这个 Go 特性在 Python 里怎么用：

```sh
python examples/build.py 15_interface
python examples/15_interface/demo.py
```

也可以用生成器为任何一个样例目录生成绑定：

```sh
go run . -out generated/basic ./examples/01_basic
python generated/basic/build.py
PYTHONPATH=generated/basic/.build/python python -c 'import go_basic_value as b; print(b.int_in_and_out(1))'
```

`generated/` 不提交到仓库，可以随意删除重建。

## 参与开发

仓库结构：

- `main.go`：CLI 入口。
- `internal/`：生成器实现，按"读取 Go 类型 → 设计 Python API → 设计 ABI → 输出代码"分层。
- `examples/`：每种 Go 特性的手写参考实现和测试，是生成器行为的标准答案。
  `expected/` 下是人工编写的理想产物，不是生成器输出。
- `docs/`：生成器的设计和维护文档，从 [文档导航](docs/README.md) 开始读。

运行测试：

```sh
go test ./...
go vet ./...
# 原生集成测试：真正生成、编译并在 Python 里调用 examples 01–15
GO_BRIDGE_PYTHON=python go test ./internal/generate -run TestIntegration -v
# 解释器装了 mypy 时，同时检查生成的 .pyi
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/generate -run TestIntegration -v
```

不设置 `GO_BRIDGE_PYTHON` 时，常规 `go test` 会跳过需要原生编译的集成测试。
哪些行为已经实测、哪些环境尚未验证，见 [验收记录](docs/generator-acceptance.md)。

新增一种 Go 特性的支持时，先在 `examples/` 里写一个最小、可运行、带文档的样例，
确定 Python 侧该是什么样子，再实现到生成器里。具体流程见
[examples/readme.md](examples/readme.md) 和 [扩展、验证与排障](docs/generator-maintenance.md)。
