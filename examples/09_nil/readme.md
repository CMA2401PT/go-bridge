# 09_nil：带类型的 nil 与 Go panic

本例把 Go nil 指针表示为同类 Python 包装：PtrInt.nil() 仍是 PtrInt，句柄为 0，
不分配 holder。nil 接收者的方法原样交给 Go 执行，由方法决定如何处理 nil。
需要解引用 nil 的操作遵循 Go 语义；未恢复的 panic 直接终止整个 Python 进程。

struct 沿用 [06](../06_named_type/readme.md)，指针沿用
[07](../07_ptr_basic/readme.md)。struct 包装代表一个 Go 变量，永远非 nil；
它的指针包装可以是 nil。总体规则见 [类型映射](../types.md)。

构建后运行 [demo.py](demo.py)，可以直接看到本例的核心行为：

```sh
python examples/build.py 09_nil
python examples/09_nil/demo.py
```

```text
nil 指针: True | 指向零: False
传给 Go: nil 0
空链表长度: 0 | 添加后: 2
链表第二项: 20 | 未找到是 nil: True
断开后长度: 1 | 原尾节点: 20
```

## Python API

```python
import go_nil as bridge

empty = bridge.PtrList.nil()
assert empty.is_nil
assert empty.len() == 0  # Go 方法自行处理 nil 接收者
values = empty.push(2).push(1)
assert bridge.find(values, 9).is_nil
assert values.next.next.is_nil
node = bridge.List(value_=3)
assert node.addr().value.value_ == 3
```

Go nil 有静态类型，因此不用 Python None 表示它。None 作为 Go 参数被拒绝。
nil 包装和非 nil 包装有相同的方法，类型注解也相同。is_nil 是只读属性；bool()
抛 TypeError，避免 Python 默认真值掩盖 nil。

## 句柄和共享

| 操作 | Go / ABI 行为 | Python 行为 |
| --- | --- | --- |
| 构造 nil | 句柄 0，不 Pin | PtrT.nil() |
| 判断 nil | 句柄是否为 0 | p.is_nil |
| 传入 nil | Get[*T](0) 返回 nil | 参数仍是 PtrT |
| 返回 nil | PinPointer 将 nil 表示为 0 | 同类 nil 包装 |
| 取得 struct 地址 | Pin(&v) | v.addr() 与 v 共享变量 |
| 读取指针字段 | 复制字段保存的指针 | 新包装共享被指向对象 |

每个非 nil 包装拥有独立的 Pin，析构时释放一次。Get 和 UnPin 的非零句柄必须仍然
有效，内部 ABI 不处理未知句柄或重复释放。nil 不占 Pin，释放 0 没有操作。

Next *List 是指针字段。head.next 复制的是指针，不是字段地址；赋值 head.next
只替换该字段。构造时省略 next 得到 nil。Value 字段与指针的 value 属性冲突，
因此在 List 与 PtrList 上统一命名为 value_。
链表形成的环只存在于 Go 堆中，不产生跨 Python/Go 的引用环。

## 方法与 panic

| 操作 | nil 接收者的行为 |
| --- | --- |
| (*List).Len / Push | 方法可以自行处理 nil，正常返回 |
| (*List).First | 方法内部解引用，Go panic |
| List.Head | 调用值接收者前解引用，Go panic |
| 指针的 value、struct 指针字段读写 | 解引用，Go panic |

适配层直接执行 Go 调用，没有 recover，没有 panic 状态或消息出参。C++ 不把
Go panic 转换为 Python 异常。Python try/except 无法捕获它，也不能依赖 finally
在进程终止时执行。目标 Go 代码自己写的恢复逻辑仍由 Go 管理。

必须正常返回的业务失败应由 Go API 自己表达；panic 的行为与数据转换错误分开。
Python 参数类型错误仍是 TypeError，字符串编解码错误仍是 Python 异常。

## 验证

在仓库根目录运行：

```sh
python examples/build.py 09_nil --test
go test -race ./examples/09_nil/expected/go
MYPYPATH=examples/.build/09_nil/python mypy examples/09_nil/test.py --strict
```

正常行为测试覆盖 nil 参数与返回值、nil 接收者、字段与视图共享、Go 环和句柄释放。
每个致命调用单独放进 Python 子进程，检查非零退出与 Go panic 输出。子进程直接
执行调用，不尝试捕获 panic。Go 测试覆盖零句柄、nil 不 Pin 和正常 nil 接收者。

生成器需要区分指针与 struct 变量，并按方法集选择接收者。panic 不需要额外的
ABI 规划。生成器的完整规则见 [运行时契约](../../docs/generator-runtime.md)。
