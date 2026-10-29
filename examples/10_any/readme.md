# 10_any：any 装箱与类型断言

本例定义 Go `any`／`interface{}` 如何映射为 Python API：显式装箱、按动态类型
断言、nil interface 与带类型 nil、复制和共享，以及未知动态类型的持有与传回。
这是手写行为样例，生成器已从生成产物验证本例协议，并另外验证 `*any`、`[]any` 组合。
总体类型规则见 [`../types.md`](../types.md)。

本例使用当前生成器的 `runtime/cgo.Handle`，不使用历史案例的地址句柄与 Pinner。
Go panic 不恢复，未恢复的 panic 终止整个 Python 进程。

```sh
python examples/build.py 10_any --test
GOEXPERIMENT=cgocheck2 python examples/build.py 10_any --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/10_any/expected/go
MYPYPATH=examples/.build/10_any/python mypy examples/10_any/test.py --strict
```

## Python API

`GoAny(Ref)` 持有一个不可直接修改的 Go interface 值。它与 `typing.Any` 无关；
`Ref` 只负责句柄所有权，不提供通用装箱，也不公开原始句柄。`any` 参数只接受
`GoAny`，返回值始终为 `GoAny`，不根据动态类型自动转换为 Python 值。

```python
import go_any as bridge

a = bridge.int_as_any(42)
assert bridge.any_as_int(a) == 42
assert bridge.try_any_as_string(a) is None
assert bridge.any_as_int(bridge.echo(a)) == 42

named = bridge.MyInt(42).as_any()
assert type(bridge.MyInt.from_any(named)) is bridge.MyInt
assert bridge.try_any_as_int(named) is None  # MyInt 与 int 是不同 Go 类型。

s = bridge.SliceInt([1, 2])
restored = bridge.SliceInt.from_any(s.as_any())
restored[0] = 3
assert s.copy_to_py_list() == [3, 2]
```

基础值使用模块函数，每个 **Go 类型** 一组：

```python
def int_as_any(value: int) -> GoAny: ...
def any_as_int(value: GoAny) -> int: ...
def try_any_as_int(value: GoAny) -> int | None: ...
```

覆盖 `int`、`uint`、`uintptr`、所有 8/16/32/64 位整数、`float32`、`float64`、
`bool`、`string`。例如 `int64_as_any()` 与 `int_as_any()` 都接受 Python int，
但产生不同的 Go 动态类型；`string_as_any()` 接受 Python str。Go `byte` 与
`rune` 是别名，分别使用 `uint8` 与 `int32` 的转换，不产生新的动态类型。

装箱输入沿用当前生成器的基础值检查：整数越界抛 `OverflowError`，类型错误抛
`TypeError`；整数接受 int 子类（包括 bool），bool 只接受真正的 bool，float
接受 Python float 或 int，string 只接受 str。因此 `int_as_any(MyInt(1))`
显式得到动态类型 int；保留命名类型则写 `MyInt(1).as_any()`。

其他已映射具体类型提供：

```python
def as_any(self) -> GoAny: ...
@classmethod
def from_any(cls, value: GoAny) -> Self: ...
@classmethod
def try_from_any(cls, value: GoAny) -> Self | None: ...
```

本例验证 `MyInt`、`SliceInt`、`MySlice`、`Point`、`PtrInt`、`PtrPoint`、
`PtrPtrInt` 与命名指针 `IntPtr`。转换实现属于各具体类型；不能让通用 Ref
根据 holder 的内部表示推测 Go 类型。GoAny 已经是 any 包装，不再给它提供这组三个
转换方法；它可直接作为 any 参数传入。

## 类型断言和失败边界

取出执行 Go 的类型断言，不执行 Go 类型转换：动态类型必须与目标具体 Go 类型
相同。`MyInt` 不能断言成 int，`MySlice` 不能断言成 `[]int`，`IntPtr` 不能断言
成 `*int`；`Point` 与 `*Point`、`*int` 与 `**int` 同样不同。

- `any_as_*()`／`T.from_any()`：不匹配时抛 `TypeError`。
- `try_any_as_*()`／`T.try_from_any()`：不匹配时返回 `None`，失败不创建句柄。
- 匹配时返回转换后的值；`0`、`False`、空字符串和带类型 nil 都是成功结果。
- 两种形式都拒绝不属于 GoAny 的参数，包括 Python `None`。
- `try` 只处理类型断言失败。成功断言为 string 后的 UTF-8 解码错误仍抛
  `UnicodeDecodeError`；它不捕获 Python 转换异常，也不恢复 Go panic。

Python 应使用 `result is None` 判断断言失败。指针不能用 bool 判断，标量和 slice
也可能是合法的假值。`.pyi` 使用 `T | None`，判断后可以收窄到 T。

适配层始终用 `value, ok := a.(T)`，通过 ABI 的 ok 字段传出结果。普通形式在 C++
将不匹配变成 TypeError，不触发 Go panic，也不引入 recover。Go API 自己的单值
断言失败仍会 panic，例如错误调用 `unknown_value(int_as_any(1))`。

## 值、变量与共享

| 来源 Go 类型 | 装箱 | 成功取出 |
| --- | --- | --- |
| 基础／命名标量 | 复制值，保留原 Go 类型 | Python 基础值／对应命名类 |
| slice／命名 slice | 复制 slice 头，共享数组 | 新包装，共享原数组 |
| struct T | 复制 T 的值 | 新的可寻址 T 变量，复制 interface 中的值 |
| 指针／命名指针 | 复制指针值，共享目标 | 新指针包装，共享目标 |

struct 包装的 Cell 保存 `*T`，但 `v.as_any()` 必须读取 `*cell`，装入动态类型 T。
`v.addr().as_any()` 才装入动态类型 `*T`。不能直接将 struct Cell 装入 any。

```python
v = bridge.Point(x=1, values=bridge.SliceInt([2]))
a = v.as_any()
v.x = 9
assert bridge.Point.from_any(a).x == 1

first = bridge.Point.from_any(a)
first.x = 3
assert bridge.Point.from_any(a).x == 1  # 每次取出都复制。
first.values[0] = 4
assert v.values.copy_to_py_list() == [4]  # struct 的复制是浅复制。

p = v.addr().as_any()
bridge.PtrPoint.from_any(p).x = 5
assert v.x == 5
```

`Point.Data any` 的 getter 返回 interface 值的新包装，setter 复制传入的 Go
interface 值。之后替换字段不会改变先前读出的 GoAny；字段中的指针和 slice 继续
保持 Go 的共享语义。构造时省略 data 使用 nil interface，显式 None 被拒绝。

## nil interface 与带类型 nil

`GoAny.nil()` 表示 `var a any`。它的句柄为 0、`is_nil` 为真、不占 holder；没有
无参或接收任意 Python 对象的 GoAny 构造函数。`bool(a)` 抛 TypeError。

```python
a = bridge.GoAny.nil()
assert a.is_nil
assert bridge.PtrInt.try_from_any(a) is None

b = bridge.PtrInt.nil().as_any()
assert not b.is_nil
p = bridge.PtrInt.try_from_any(b)
assert p is not None
assert p.is_nil
```

b 保存动态类型 `*int`，虽然动态值为 nil，interface 本身仍非 nil，因此 b 的
句柄非零。成功取出后指针包装的句柄为 0，ABI 必须用独立的 ok 字段区分成功和
失败，不能依赖结果句柄是否为 0。

nil slice 也具有动态类型。`nil_slice()` 与 `empty_slice()` 都返回非 nil 的 GoAny，
并能成功断言成 SliceInt。SliceInt 本身沿用既有 API，不提供 nil 判断；但
装箱、取出仍保留 Go 中底层 slice 是否为 nil 的状态。本例的 Go 函数
`slice_is_nil()` 用于验证这个状态，不代表为所有 slice 增加 is_nil。

## ABI、Cell 与生命周期

| Go 值 | Cell | ABI |
| --- | --- | --- |
| nil interface | 无 Cell | 句柄 0 |
| 非 nil interface（含带类型 nil） | 直接保存原 any 值 | 拥有的非零整数句柄 |
| 标量断言结果 | 不使用 Cell | 标量／BrString，加 ok 字段 |
| 对象断言结果 | 对应类型既有 Cell | 拥有的句柄，加 ok 字段 |

非 nil any 直接交给 `cgo.NewHandle` 保存，读取时使用 `cgo.Handle.Value()`，不增加
anyCell 包装。装箱前恢复原 Go 类型，取出后再转为既有 Cell：例如 MySlice 的
Cell 是 `[]int`，装箱须先还原 MySlice；IntPtr 的 Cell 是 `*int`，装箱须先还原
IntPtr。any 内部不能剥去命名类型身份。

Python 入口检查参数是否为 GoAny；调用者通过公开接口构造对象。Go 侧不额外检查
any 句柄的类别。

输入句柄借用；每次成功装箱、非 nil any 返回、非 nil 对象取出均创建独立句柄。
GoAny 不借用来源 Ref 的句柄，来源释放后仍有效；取出的对象也能在 GoAny 释放后
继续使用。Go 值内部只保存 Go 引用，不保存 Python 对象或句柄，环交给 Go GC。

C++ 使用不可复制、可移动的 Ref 接管句柄，析构调用 BrRelease。创建 Python
结果前，临时 Ref 已持有对象句柄，string 则由 C 缓冲区 owner 接管；转换异常不会
遗失所有权。无效内部句柄直接 panic，Go panic 不恢复。

## 静态闭包与范围

any 不提供动态类型枚举，也不扩展类型闭包。其他具体类型仍从 Go 的静态声明、
参数、结果和字段等依赖中得到；只能为已经生成的具体类型提供对象转换方法。
基础值转换函数按本例列出的 Go 基础类型提供。

Go 返回的 any 可以保存闭包外的动态类型，包括未导出的类型或尚未映射的 map。
本例 `unknown()` 返回装有 `map[string]int` 的 GoAny；Python 能持有、传给 echo
或 unknown_value，不能因此构造或操作 Python map 包装。

生成器分别规划 any 的 Go 类型分类、Python GoAny 表示、any 句柄读写与 ABI
断言操作；命名和冲突规划包含 GoAny、各基础值转换函数以及 as_any、from_any、
try_from_any。对象适配层负责具体类型恢复和归一化，holder 只管理生命周期。
生成器实现与组合类型规则见 [any 协议](../../docs/generator-any.md)。

本例不覆盖带方法的 interface、命名 interface、error、`*any`、`[]any`、跨生成
模块的包装互传、动态方法调用、动态类型反射 API 或 interface 相等比较。
代表性包装只保留验证 any 所需的构造、字段、索引等操作，不重复完整 slice API。

## 验证

Python 测试覆盖全部基础类型及整数边界、动态类型不互认、命名类型身份、struct
双向复制、slice／指针共享、多级和命名指针、nil interface、各类带类型 nil、
nil slice 状态、any 字段快照、未知动态类型、显式装箱要求、
失败无分配、源／结果独立生命周期、Go 环、解码异常以及子进程中的 Go panic。
静态检查验证 try 结果的类型收窄。Go 测试直接检查句柄保存的动态值、命名类型恢复和
归一化、struct 副本、nil 的 ok 字段及句柄释放。
