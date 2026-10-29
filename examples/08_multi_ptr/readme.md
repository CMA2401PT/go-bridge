# 08_multi_ptr：多级指针、命名指针与类型闭包

本例研究 `***int`、`**Box` 这样的多级指针，底层是指针的命名类型
`type IntPtr *int`，以及生成器必须补齐的依赖类型（类型闭包）。指针的基本规则
沿用 [07](../07_ptr_basic/readme.md)，struct 沿用 [06](../06_named_type/readme.md)，
总体规则见 [`../types.md`](../types.md)。

```sh
python examples/build.py 08_multi_ptr --test
go test -race ./examples/08_multi_ptr/expected/go
```

## Python API

```python
import go_multi_ptr as bridge

p = bridge.PtrInt(1)
ppp = p.ptr().ptr()                 # Go: pp := &p; ppp := &pp（各自复制指针）
bridge.inc_ppp(ppp)                 # ***p++
assert p.value == 2
assert ppp.value.value.value == 2   # 每级 .value 都是指向同一变量的新对象

n = bridge.IntPtr(p)                # Go: IntPtr(p)，共享变量
bridge.inc_int_ptr(n)
assert n.unname().value == 3        # Go: (*int)(n)

box = bridge.Box()                  # Counter 与 Inner 由闭包生成
bridge.inc_box_pp(box.addr().ptr()) # &box 再取地址：**Box 指向同一个 struct
```

## 类型闭包

G 只直接出现 `***int`、`**int`（`Retarget`）、`IntPtr`、`Box`、`**Box`。生成器
从这些类型出发递归补齐，直到基础值类型：

| 出现的类型 | 需要 | 原因 |
| --- | --- | --- |
| `***int` | `**int` → `*int` → `int` | 每级 `.value` 与构造参数 |
| `IntPtr`（底层 `*int`） | `*int` | `unname()` 与构造参数 |
| `Box` | `Counter`、`Inner`、`*Box` | 字段与构造参数；`addr()` |
| `Inner`（闭包生成的 struct） | `*Inner` | `addr()` |
| `**Box` | `*Box` → `Box` | `.value` 与构造参数 |

本例中 `*int` 即使 G 中没有出现也会生成；而 `****int` 没有出现，所以
`PtrPtrPtrInt` 没有 `ptr()`。`ptr()` 只在对应的指针类存在时才有。

## 多级指针

| Go | Python |
| --- | --- |
| `pp := new(*int); *pp = p` | `PtrPtrInt(p)`、`PtrPtrInt.from_value(p)`、`p.ptr()` |
| `q := *pp` | `pp.value`，新的 `PtrInt`，与 `q` 指向同一变量 |
| `*pp = q` | `pp.value = q` |
| `**pp = 1` | `pp.value.value = 1` |

- 构造复制的是给定的**指针**，新变量与它指向同一个目标，与 07 中“复制值”是同一
  条规则（被复制的值恰好是指针）。每次都分配新变量：两个 `p.ptr()` 互不影响。
- `.value` 每次返回新的包装对象（新 Pin），它们是同一 Go 变量的别名。
- 修改某一级只影响那一级：`retarget(pp, b)` 改写 `*pp`，之前读出的 `pp.value`
  仍指向旧目标，而 `ppp` 看到的是新目标（它指向 `pp` 本身）。
- 指向指针的指针类没有零值构造：Go 的零值是 nil，Python 构造的指针永远非 nil。
- Go 只为方法调用自动解引用一次，`**T` 的方法集为空，所以这些类不挂 Go 方法。
- 每级严格匹配：`PtrPtrInt` 只接受 `PtrInt`，`IntPtr` 这类命名指针要先 `unname()`。

### nil

Python 不能构造 nil，但 Go 代码可以写入：`clear(pp)` 执行 `*pp = nil`。读取该级
`.value` 抛 `ValueError`，其他级不受影响；再次赋值即可恢复。
这是本例在引入 nil 包装前的限制。正式 nil 映射见 [09_nil](../09_nil/readme.md)：
返回同类型的 nil 包装，使用 `is_nil` 判断，类型标注仍为 `T`。

## 命名指针 `type IntPtr *int`

- Python 类 `IntPtr(Ptr[int])`，继承 Ref，与 `PtrInt` 互不继承、互不接受。
- 构造函数 `IntPtr(p: PtrInt)` 与 `unname() -> PtrInt` 都是转换，共享变量。
- 有 `.value`：Go 允许 `*p`。没有 `from_value`（`new(IntPtr)` 得到的是 `*IntPtr`，
  另一种类型）、`ptr()`（`*IntPtr` 未出现）和方法（Go 不允许为指针类型定义方法）。
- Go 侧 Pin 的是 `IntPtr` 值本身，与 07 中 slice 句柄 Pin 的是 slice 头一致。

## `**Box`

`*Box` 是 `PtrBox`（06），所以 `**Box` 是 `PtrPtrBox(Ptr[PtrBox])`，与 `**int` 同一
条规则，没有例外：

- `box.addr()` 即 `&box`；`p.ptr()` 是新的 `**Box` 变量，其中是 `p` 的副本，struct
  本身共享。`Box` 没有 `ptr()`，要先 `addr()`。
- `pp.value` 返回同一个 struct 的新 `PtrBox` 包装；`pp.value = other.addr()` 复制指针。
- `Box`、`PtrBox` 与 `PtrPtrBox` 互不接受：`inc_box_pp(box)` 与 `inc_box_pp(p)` 都抛
  `TypeError`。

Box 构造时省略 count 或 inner 使用对应 Go 零值；显式传值必须是 Counter 或 Inner，
None 不表示省略。

## 所有权与边界

- 每个包装对象是一个 Pin，指向的每级变量由 Go 可达性管理：释放中间级的 Ref 后，
  外层指针仍使整条链可达。
- 未覆盖：命名指针的指针（`*IntPtr`）、指向命名类型的多级指针、指针相等、
  nil 的正式映射、struct 的指针字段。

## 验证

Python 测试覆盖：闭包生成的类都存在并可逐层构造、各级共享目标、`.value` 为新包装、
修改只影响一级、每次构造分配新变量、中间 Ref 释放后仍可用、nil 级、每级严格类型、
`**T` 无方法、命名指针转换与严格类型、闭包生成的字段类型与 struct 指针类、
`**Box` 共享 struct 与复制指针。Go 测试覆盖各级指针复制、Load 的新句柄、nil 级、
命名指针共享与 `**Box`。

```sh
GOEXPERIMENT=cgocheck2 python examples/build.py 08_multi_ptr --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/08_multi_ptr/expected/go
MYPYPATH=examples/.build/08_multi_ptr/python mypy examples/08_multi_ptr/test.py --strict
```

## 对未来生成器的要求

生成器需要在 `go/types` 上从 G 的全部签名、字段与方法出发计算类型闭包：遇到
指针取基类型、遇到非 struct 的命名类型取底层类型、遇到 slice 取元素、遇到
struct 取导出字段类型并补上 `*T`，直到基础值类型；类名按 Go 类型拼写。还需要知道哪些指针类型
出现在闭包中，以决定值类是否有 `ptr()`；以及方法集只包含一层解引用。
