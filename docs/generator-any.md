# any 装箱与类型断言

[文档导航](README.md) · [模型](generator-models.md) · [运行时](generator-runtime.md)

## 范围与公开接口

any 和 interface{} 映射为同一个 GoAny 类型，真正 alias 重新导出该类型。
只有静态类型闭包中存在空 interface 时，才生成 GoAny 和转换能力。
命名 interface、带方法 interface、error 仍在 Go 分析阶段拒绝。

基础类型每个 Go 类型生成三个模块函数：

```python
def int_as_any(value: int) -> GoAny: ...
def any_as_int(value: GoAny) -> int: ...
def try_any_as_int(value: GoAny) -> int | None: ...
```

bool、各宽度整数、uintptr、float32/float64、string 均提供相同形式。
byte/rune 的 alias 使用 uint8/int32，不创造新的动态类型。其他已映射具体类型
提供 as_any、from_any、try_from_any；包括命名标量、slice、struct 和 pointer。
GoAny 自身不提供这组三个转换。any 参数只接收已初始化的 GoAny，不自动装箱。

转换执行具体 Go 类型断言，不执行 Go 类型转换。MyInt 与 int、命名 slice 与
底层 slice、命名 pointer 与底层 pointer、T 与 *T 都不能相互断言。
普通断言不匹配抛 TypeError，try 不匹配返回 None；零值和 typed nil 都可以是
成功结果。try 不隐藏成功断言后的 Python 转换错误，Go panic 不捕获、不恢复。

转换函数名称参加包级冲突检查，冲突直接报错。as_any、from_any、try_from_any
属于桥接保留成员；同名 Go 字段／方法按现有规则增加下划线。

## 值与句柄

GoAny.nil() 使用句柄 0，is_nil 为真；没有公开构造函数，bool 判断抛 TypeError。
非 nil interface 直接交给 cgo.NewHandle 保存，读取直接使用 Handle.Value()。
没有 anyCell，也不在 Go 侧额外验证 any 句柄类别；Python 入口负责包装类型和
初始化状态检查。any 的运行时输出到 go/any.go，通用 holder 继续负责生命周期。

装有 typed nil 的 interface 本身非 nil，因此使用非零句柄。断言成功取出 nil
pointer 时，pointer 的 ABI 值为 0；独立 ok 字段区分成功与断言失败。
nil slice 同样保留动态类型和 nil 状态，slice 包装仍不提供 is_nil。

装箱前通过既有 goInput 恢复原 Go 类型；断言成功后通过 goOutput 转为既有 ABI：

| 类型 | 装箱与取出语义 |
| --- | --- |
| 基础／命名标量 | 复制值，保留命名类型身份 |
| slice／命名 slice | 复制头，共享数组；装箱恢复名字，取出后归一化 Cell |
| struct T | 装箱复制 T，取出创建独立的可寻址变量；复制为浅复制 |
| pointer／命名 pointer | 复制指针，共享目标；装箱恢复名字 |

struct 的内部 Cell 是 *T，不能直接装箱该 Cell；必须装箱解引用后的 T。
any 字段 getter 复制当次 interface 值，setter 复制传入值。替换原字段不会改变
之前读出的 GoAny；内部 pointer 和 slice 的共享关系仍保持。

输入句柄借用。每次非 nil 装箱、any 返回和非 nil 对象断言结果都有独立的拥有
句柄；来源释放后结果仍有效。失败断言不创建资源。C++ 在解码或构造 Python
结果之前接管返回资源；解码错误、多返回值后续转换失败等路径沿用既有 RAII。

## 模型与输出路径

- gomodel 的 Interface 节点仅表示空 interface，不追踪其可能的动态类型。
- pyapi.AnyConversion 规划具体类型的公开转换名字，Package.Any 标识 GoAny。
- abi.AnyConversion 规划 Box、Assert 和结果结构，不伪装成业务 Operation。
- any 沿用 Handle 的 uintptr_t ABI，不增加新的传输 Kind。
- emit/any_go.go 输出运行时和具体类型断言；emit/any_cpp.go 输出 Python 转换。

每个具体类型仅有两个 Go 导出操作：Box(T) 返回 any 句柄；Assert(GoAny) 返回
{ok, value}。普通和 try 形式共用 Assert，C++ 决定失败时抛 TypeError 或返回 None。
命名标量取出的 Python 值仍是命名类；转换闭包用弱引用访问类，避免类持有自身。

any 可以持有并传回闭包外的动态类型，包括私有类型、map 和匿名 struct。
这不表示生成器支持这些类型作为静态 API，也不会生成它们的 Python 操作接口。

## pointer 与 slice 组合

静态闭包中的 *any 和 []any 分别生成 PtrGoAny 和 SliceGoAny，复用既有类型操作。
不自动补充 *any；闭包确实包含它时，GoAny 才提供 ptr()，结果是保存 interface
值副本的新变量，不是修改 GoAny 内部值的视图。

PtrGoAny.nil() 是 nil pointer；PtrGoAny.from_value(GoAny.nil()) 是非 nil pointer，
指向一个值为 nil interface 的变量。load/store 使用 GoAny，nil 解引用照常 panic。
SliceGoAny 构造、赋值只接受 GoAny；索引、批量复制得到独立包装，slice clone
复制 interface 元素但保留其中引用类型的共享关系。命名容器使用既有转换规则。

## 验证

integration_any_test.go 从生成产物运行例子 10 的行为测试，不编译 expected。
独立 module 验证 any 字段、命名 scalar／容器、*any、[]any、nil、Go 环、私有动态
类型、转换异常清理、alias-only import 与 panic 子进程；同时检查声明和消费者的
mypy strict 类型收窄。生成的 any／holder 运行时另以 race/checkptr/cgocheck2 验证。
