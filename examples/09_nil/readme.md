# 09_nil：nil 与 panic

本例研究 nil 指针与 Go panic 如何映射到 Python：带类型的 nil、nil 接收者、nil
参数与返回值、指针字段，以及把 panic（含解引用 nil）转换为 Python 异常。struct
沿用 [06](../06_named_type/readme.md)，指针沿用 [07](../07_ptr_basic/readme.md)，总体规则见
[`../types.md`](../types.md)。

```sh
python examples/build.py 09_nil --test
go test -race ./examples/09_nil/expected/go
```

## Python API

```python
import go_nil as bridge

empty = bridge.List.nil()           # Go: var l *List
assert empty.is_nil                 # l == nil
assert empty.len() == 0             # nil 接收者：Go 方法自行处理 nil
values = empty.push(2).push(1)
assert bridge.find(values, 9).is_nil   # 返回 nil
assert values.next.next.is_nil         # 指针字段为 nil

try:
    empty.value                     # 解引用 nil
except bridge.GoPanic:              # RuntimeError 的子类
    pass
bridge.must_positive(-1)            # GoPanic("not positive: -1")，进程不崩溃
```

## nil：handle 0 的同类包装

**handle 0 表示该 Go 类型的零值**，对指针、命名指针与 struct 包装（`*T`）就是
nil。它是一个普通的同类对象，只是不持有 Pin。

| Go | ABI | Python |
| --- | --- | --- |
| `var p *T` | handle 0 | `T.nil()` |
| `p == nil` | `id == 0` | `p.is_nil`（只读） |
| nil 参数 | 传 0，`Get[*T](0)` 返回 nil | 传 nil 包装 |
| nil 返回值、指针字段 | `PinPointer` 把 nil 转为 0，不 Pin | `is_nil` 为真的同类对象 |

为什么不用 `None`：Go 的 nil 有静态类型，`var l *List; l.Len()` 是合法且常见的
写法（链表、树的空节点）。`None` 丢失了类型，无法调用方法；带类型的 nil 包装则
和 Go 一样可以调用，也不需要在每个签名里写 `T | None`。`None` 一律拒绝，一种值
只有一种表示。

代价与对策：

- 类型检查器无法区分可空与非空指针，这与 Go 相同；解引用 nil 在运行时抛 `GoPanic`。
- Python 对象默认为真，`if p:` 对 nil 也成立。因此 `__bool__` 抛 `TypeError`，
  提示使用 `is_nil`。
- nil 不占 Pin，`Ref` 析构对 0 本来就不调用 `SBUnPin`；只有会分配的构造函数才要求
  Go 返回非 0 的句柄。
- `PinPointer[E any](*E)` 以类型参数约束为指针再判断 nil，避免 `any(p) == nil`
  这类 interface 比较。

## 方法与字段

| 操作 | nil 时 | 原因 |
| --- | --- | --- |
| 指针接收者 `func (l *List) Len()` | 交给 Go 执行 | Go 允许 nil 接收者 |
| 指针接收者且内部解引用 `First()` | Go panic → `GoPanic` | 与 Go 相同 |
| 值接收者 `func (l List) Head()` | `(*l).Head()` panic → `GoPanic` | Go 需要先解引用 |
| 字段读写、`PtrInt.value` | panic → `GoPanic` | 解引用 nil |

指针字段 `Next *List` 读出的是字段中保存的**指针**，而不是 06 中 struct 字段的
视图 `&s.Field`：读写都共享被指向的链表，nil 读出为 `is_nil` 的 `List`，零值
就是 nil。赋值 `head.next = x` 复制指针。构造函数中省略的 `next` 默认为 nil。

链表形成环（`head.next = head`）时，环只存在于 Go 堆中；Python 包装对象各自只
持有一个 Pin，不产生跨边界引用环。

## panic

每个会运行 G 代码的导出函数多一个 `SBString* panicked` 出参，并以
`defer guard(panicked)` 开头：

- `guard` 调用 `recover`，把 panic 值 `fmt.Sprint` 后写入出参，函数返回零值。
  panic 绝不会穿过 C 帧展开，否则会终止进程。
- C++ 的 `go_call` 检查出参：非空则释放 Go 结果（字符串结果需要 free）、解码消息
  并抛 `GoPanic`。零值句柄不持有 Pin，丢弃即可。
- `GoPanic` 继承 `RuntimeError`，由 `py::exception` 注册在模块中。解引用 nil 与
  显式 panic 使用同一个类型，与 Go 一致。
- 已 Pin 的参数不受 panic 影响；重复 panic 不泄漏句柄（测试覆盖 100 次）。

构造、Unpin 与诊断函数不运行 G 代码，没有出参。`Go 运行时 fatal error`（如并发
map 写、栈溢出）无法 recover，仍会终止进程。

## Go bool

`maybe_int(ok: bool)` 的 bool 按 [01](../01_basic/readme.md) 以 `uint8_t` 跨 ABI：cgo 导出
头文件中的 `C.bool` 是 C 的 `_Bool`，C++ 无法编译。Python 侧只接受真正的 `bool`。

## 与 06–08 的关系

06–08 的 Ref 早于本例：句柄不能为 0、`None` 被拒绝、没有 recover。08 读取 nil 级
时抛 `ValueError` 是临时行为。生成器应统一使用本例的规则；各案例暂不回改，以免
把 panic 出参引入所有旧 ABI。

## 验证

Python 测试覆盖：nil 是同类对象且不 Pin、指向 0 的指针不是 nil、`is_nil` 只读、
`bool()` 被拒绝、解引用 nil 与写入 nil 抛 `GoPanic`、nil 参数与返回值、nil 接收者
正常执行、nil 接收者内部 panic、值接收者与字段访问 panic、指针字段共享与 nil
零值、环、`find` 返回 nil、节点在父包装释放后可达、显式 panic 的消息、panic 后
继续调用以及重复 panic 不泄漏。Go 测试覆盖 handle 0 读为 nil、nil 结果不 Pin、
panic 被 recover 以及 nil 接收者。

```sh
GOEXPERIMENT=cgocheck2 python examples/build.py 09_nil --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/09_nil/expected/go
MYPYPATH=examples/.build/09_nil/python mypy examples/09_nil/test.py --strict
```

## 对未来生成器的要求

生成器需要知道每个位置的类型是否为指针（决定是否允许 nil 与使用 `PinPointer`），
每个方法的接收者是值还是指针（值接收者先解引用），以及哪些导出会运行 G 代码
（需要 panic 出参）。未覆盖：slice、map、interface 的 nil，error 返回值，
panic 值的类型（目前只保留字符串），以及 goroutine 中的 panic。
