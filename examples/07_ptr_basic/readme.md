# 07_ptr_basic：值类型的指针

本例研究 Python 中不可寻址的 Go 值的指针：`*int`、`*MyNamedNonStructInt`、
`*[]int` 与 `*MyNamedNonStructSliceInt`。命名类型沿用
[06](../06_named_type/readme.md)，Ref 与 holder 沿用 [04](../04_bytes_and_holder/readme.md)。
slice 只保留本例需要的导出（构造、显式转换、`len`、`copy_to_py_list`），完整
操作见 [05](../05_slice/readme.md)。总体规则见 [`../types.md`](../types.md)。

```sh
python examples/build.py 07_ptr_basic --test
go test -race ./examples/07_ptr_basic/expected/go
```

## Python API

```python
import go_ptr_basic as bridge
Named = bridge.MyNamedNonStructInt

p = bridge.PtrInt(5)                    # Go: p := new(int); *p = 5
bridge.inc_int(p)                       # 修改经指针写回
assert p.value == 6                     # *p
bridge.inc_int(5)                       # TypeError：不隐式装箱

q = Named(1).ptr()                      # 值 -> 指针：分配新变量并复制
q.inc()                                 # 指针接收者
q.echo_non_struct_int()                 # *T 的方法集包含 T 的值接收者方法
n = q.value                             # 指针 -> 值：v := *p

s = bridge.SliceInt([1]).ptr()
bridge.append_slice_int(s, 2)           # *s = append(*s, 2)
assert s.value.copy_to_py_list() == [1, 2]
```

## 不可寻址的值

本例的基类型在 Python 中都**不可寻址**：

- 命名 int 在 Python 中是按值传递的 int 子类，没有 Go 存储。SliceInt 在 Python
  中是不可变的 slice 头，赋值只产生别名。二者都没有可被改写的单元，`*T` 必须是
  另一个持有 Go 变量的对象，取地址只能先复制（`ptr()`）。
- struct 包装对象本身是 Go 变量，可以直接取地址 `addr()`；它的指针类 `PtrT` 与
  本例规则相同，见 [06](../06_named_type/readme.md)。

## 值 ↔ 指针

| Go | C ABI | Python |
| --- | --- | --- |
| `p := new(T); *p = v` | `SBNewPtr<T>(v)` | `PtrT(v)`、`PtrT.from_value(v)`、`v.ptr()` |
| `v := *p` / `*p = v` | `SBPtr<T>Load` / `SBPtr<T>Store` | `p.value` 属性 |

- 三种构造是同一个操作：分配新变量，把值复制进去，相当于对副本取地址 `&v`，
  指针类型由 `v` 的类型决定，所以只接受自己的值类。`PtrInt` 与
  `PtrMyNamedNonStructInt` 的构造默认 Go 零值；slice 指针要求显式给出值。
- 值类有 `ptr()` 当且仅当它的指针类存在。它是方法而不是属性：每次调用都分配，
  `n.ptr().value = 1` 不会改变 `n`。Python 内建 `int` 无法挂方法，`*int` 只能从
  `PtrInt(v)` / `PtrInt.from_value(v)` 得到。
- Python 没有对已有值取地址的操作：Python 值没有 Go 存储，`&x` 只能是复制后分配。
- 句柄单元中的指针从不改变：Load / Store 只读写 `*p`，一个 Ptr 对象始终指向同一个
  Go 变量。Python 赋值只复制指针（别名）。
- `p.value` 返回副本（Go 的 `v := *p`）。对 slice，副本是 slice 头，与 `*p` 共享
  底层数组；经指针的 append 替换 `*p`，之前读出的头与原变量长度不变。
- 构造与 `*p = v` 都只接受基类型本身，读出时总是基类型对应的类。命名 slice 与
  `SliceInt` 之间需先显式转换（06），比 Go 的可赋值规则严格。`PtrInt` 接受命名
  int，是因为它是 int 子类，见 [`../types.md`](../types.md) 的已知差异。

## 指针类

| Go | 句柄中 Pin 的值 | Python |
| --- | --- | --- |
| `*int` | `*int` | `PtrInt(Ptr[int])` |
| `*MyNamedNonStructInt` | `*MyNamedNonStructInt` | `PtrMyNamedNonStructInt(Ptr[MyNamedNonStructInt])` |
| `*[]int` | `*[]int` | `PtrSliceInt(Ptr[SliceInt])` |
| `*MyNamedNonStructSliceInt` | `*MyNamedNonStructSliceInt` | `PtrMyNamedNonStructSliceInt(Ptr[MyNamedNonStructSliceInt])` |

指针类继承 `Ref`，由 Ref 析构 UnPin；泛型基类 `Ptr[T]` 与 `Slice[Elem]` 对应。
不同指针类型互不继承，也不提供相互转换：`*int` 与 `*MyNamedNonStructInt` 在 Go 中
不可互相赋值，而 Python 侧没有 Go 意义上的命名指针，`unname` 之类的转换没有对象。

`*MyNamedNonStructInt` 的底层类型是它自己而不是 `*int`，不满足 06 的 `Get` 约定
（底层类型相同），所以每个指针类型生成自己的 New / Load / Store / 方法导出，`Get`
始终按 Pin 时的类型读取，`holder.go` 与 06 相同。

## 方法与函数

| Go | 适配层 | Python |
| --- | --- | --- |
| 值接收者，值调用 | `T(v).M()` / `Get[T](id).M()` | 值类的方法 |
| 值接收者，经指针 | `p.M()`，即 `(*p).M()` | 指针类的方法 |
| 指针接收者 `func (p *T) M()` | `p.M()` | 只在指针类上 |
| 参数 `*T` | 直接传 `p` | 只接受对应指针类 |

方法集严格照 Go：值类只有值接收者方法。Go 的 `x.PtrM()` 可用是因为 `x` 是可寻址
变量，而 Python 值不是 Go 变量；临时装箱调用会静默丢失 `Inc`、`Append` 这类修改，
生成器也无法判断方法是否只读。

指针参数只接受对应的指针类：普通值、`None`、其他指针类都抛 `TypeError`，理由同上。

`G.go` 为此做了调整：

- 增加 `Inc`、`IncInt`、`Append`、`AppendSliceInt`，因为 echo 无法区分传指针与传
  副本。
- `*int` 与 `*MyNamedNonStructInt` 的 echo 打印 `*i`：`%v` 打印这类指针得到的是
  随运行变化的地址，无法断言。`*[]int` 的 `%v` 是 `&[1 2]`，保持不变。

## 所有权与边界

- 每个 Ptr 是一个 Pin；`p.value` 读出的 slice 是新的 Pin。被指向的变量与 slice
  底层数组由 Go 可达性管理，释放构造时使用的值不影响指针。
- 07 的指针永远非 nil：只能由构造得到，`.value` 不会解引用 nil。nil 参数与返回值
  留给可空值案例。
- 未覆盖：指针相等（`p == q`）、标量字段取地址 `&s.X`、struct 的指针字段、
  `**T`、指针返回值。

## 验证

Python 测试覆盖：零值构造、读写与范围检查、三种构造方式、每次构造分配新变量、
别名、经指针修改（含 Go int 回绕）、命名指针的严格类型、方法集、`p.value` 为副本、
slice 指针 append 只替换 `*p`、构造与 `*p = s` 只接受自己的值类、参数
拒绝、构造失败不泄漏句柄以及变量在源值释放后存活。Go 测试覆盖写回与 slice 头
语义。

```sh
GOEXPERIMENT=cgocheck2 python examples/build.py 07_ptr_basic --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/07_ptr_basic/expected/go
MYPYPATH=examples/.build/07_ptr_basic/python mypy examples/07_ptr_basic/test.py --strict
```

## 对未来生成器的要求

生成器需要判断每个指针的基类型在 Python 中是否可寻址（struct 可寻址，标量、
slice 及其命名类型不可寻址），为指针类型生成 `Ptr<T>` 类及 New / Load / Store
导出，在不可寻址且可挂方法的值类上生成 `ptr()`，在 struct 上生成 `addr()`；需要方法集（值类只挂
值接收者方法，指针类挂全部方法），以及每个位置恰好接受的 Python 类。
