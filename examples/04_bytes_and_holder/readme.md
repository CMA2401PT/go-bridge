# 04_bytes_and_holder：Ref 与 Go 对象生命周期

本例研究如何让 Python 持有生命周期由 Go 管理的对象。`[]byte` 是第一个具体类型；
**Ref 是通用的生命周期基类，后续 Go 对象的包装类型也应复用它，不能将它退化成
bytes 专用逻辑。**

```sh
python examples/build.py 04_bytes_and_holder --test
go test -race ./examples/04_bytes_and_holder/expected/go
```

## Python API 与 Ref 的职责

```python
import go_bytes_and_holder as bridge

source = bridge.SliceBytes(b"hello\0\xff")
assert isinstance(source, bridge.Ref)
result = bridge.recv_and_return_bytes(source)
del source
assert len(result) == 13
copied = result.copy_to_py_bytes()
del result
assert copied == b"Echo: hello\0\xff"
```

`Ref` 只保存一个内部句柄。它的析构函数通知 Go：Python 已不再需要这份持有关系。
它不释放 Go 对象内存、不理解对象的具体类型，也不提供字节切片操作。`SliceBytes`
继承 `Ref`，只添加从 Python bytes 构造、`len()` 和 `copy_to_py_bytes()`。
以后增加 struct 等包装类型时，同样继承 Ref，复用析构通知。

Ref 在 Python 中可用于类型标注和 isinstance，但不能由用户直接构造；绑定层
只有在获得有效的 Go 句柄后才构造它。句柄不作为 Python 属性公开，也不提供
close、closed、上下文管理、复制所有权或 pickle 接口。

Python 变量赋值产生的是同一个包装对象的别名。最后一个引用消失后，C++ 的
Ref 析构调用通用 `BrUnPin`。C++ 禁止复制 Ref，允许移动；移动后旧对象不再通知
释放。这也保护了返回包装对象时的所有权，避免两次释放同一个句柄。

## Go 的 Pin / Get / UnPin

`expected/go/holder.go` 保留地址句柄方案：

- `Pin[T](value)`：在 Go 堆上保存 T，pin 住它，用地址作为 C.uintptr_t 句柄；全局
  `holders sync.Map` 保存 `C.uintptr_t 地址 → *runtime.Pinner`。
- `Get[T](address)`：按指定 T 直接读取该地址上的 Go 值，不查询 holders。
- `UnPin(address)`：从 holders 移除 Pinner，调用 Unpin，结束这份持有关系。

这里使用 Go 的泛型 T，而不是为 []byte 单独建表。保存 `*runtime.Pinner` 避免
复制 Pinner，直接 pin `&value`。Pin 的返回类型及 Get / UnPin 的句柄参数统一为
`C.uintptr_t`，导出包裹层直接传递句柄。当前案例不研究零大小类型的地址身份。

对于 []byte，pin 的是存放切片值的位置，底层数组继续通过 Go 引用保持可达。
对于包含 string、slice 或指针字段的结构体，也是保存整个 Go 值。嵌套对象仍由
Go GC 管理，本例不递归 pin，也不让 C++ 读取它们。UnPin 只解除这份 pin；若 Go
还有其他引用，对象仍然存活，并不立即被回收。

`uintptr` 是整数，不能在 Go 中一般性地转换回 unsafe.Pointer；参见
[unsafe.Pointer 的转换规则](https://pkg.go.dev/unsafe#Pointer)。本例在一个 C
辅助函数中将地址转换为 void*，Go 接收该指针后转换为 *T 并读取。辅助函数不
解引用或保留指针。Pin 和 holders 保证这次取值发生时，该地址仍然有效；没有
关闭 checkptr 或 cgo 检查。这个转换会产生一次 cgo 调用，不宣称比查表更快。

[Pinner 的约束](https://pkg.go.dev/runtime#Pinner) 仍然适用：Pinner 本身必须保持
可达直到外部不再需要该对象。本例只把整数句柄长期保存在 C++ 中，T 的内存和
它内部的 Go 指针全部由 Go 读取。

## 句柄使用约定

这是一套绑定内部的地址协议。Get[T] 必须使用由 Pin[T] 创建、尚未 UnPin 的
句柄，且 T 必须准确匹配。它不做类型断言或失效句柄检查。不能把未知地址、过期
地址或错误类型交给 Get。释放后的地址可能被 Go 分配器复用，因此调用者不得
保存并再次使用旧句柄。UnPin 必须且只能对每个有效句柄调用一次。

当前每次 Pin 对应一个 Ref。若未来要为同一个对象产生多个独立包装对象，应分别
Pin 或另行定义共享持有策略，不能直接复制原句柄的释放责任。对于 pointer 类型，
Pin 保存的是这个指针值，其引用的原对象由 Go 保持可达。

Python 绑定保持 GIL，传入对象在整个调用期间存活，保证 Get 不与同一个 Ref 的
析构并发。sync.Map 只保护注册和移除；原始 Go 对象的并发读写仍需对象自身同步。
本例尚不支持 free-threaded Python 或跨解释器共享。

## 字节切片调用链

| 操作 | ABI | 行为 |
| --- | --- | --- |
| 构造 SliceBytes | BrNewBytes(外部地址, 长度) → uintptr_t | 复制字节到 Go，再 Pin |
| 调用 Go 函数 | BrEchoBytes(uintptr_t) → uintptr_t | Get[[]byte]、调用原函数、Pin 返回值 |
| 查询长度 | BrBytesLen(uintptr_t) → size_t | Go 读取切片长度 |
| 显式复制 | BrCopyBytes(句柄, Python 缓冲区) | Go 同步填充新的 Python bytes |
| 通知释放 | BrUnPin(uintptr_t) | 通用 UnPin，与 bytes 类型无关 |

每个原始 Go []byte 参数/返回值对应一个句柄。构造和复制是专用传输辅助函数，
借用外部缓冲区；构造传入长度，复制的长度由 Go slice 决定。C++ Ref 内部只有句柄，不保存 Go 数组地址或长度，也不再
使用 BrBuffer。字符和任意二进制数据都可保留，包括空值和内嵌 NUL。

C++ 按 slice 长度分配复制缓冲区，内部 ABI 信任这一约定。复制时只借用尚未发布
的 Python 缓冲区，Go 不保存这个地址。返回值在显式复制前都保存在 Go 中；原始函数内部
可能分配和复制，不能据此宣称整个操作零拷贝。nil 切片与空切片在 Python API 中
均表现为长度 0。

## 验证

Python 测试覆盖 Ref 继承、空值与二进制数据、强制 Go GC 后读取、输入先析构、
复制结果独立存活、别名和异常退出时的释放、参数类型及公开接口范围。
Go 测试覆盖通用结构体及其引用、nil 和独立句柄的并发操作。`_native._holder_count` 与 `_collect_go` 仅供测试，不是公共接口。

检查地址转换可以运行：

```sh
GOEXPERIMENT=cgocheck2 python examples/build.py 04_bytes_and_holder --test
GOEXPERIMENT=cgocheck2 go test -race -gcflags=all=-d=checkptr=2 ./examples/04_bytes_and_holder/expected/go
go vet ./examples/04_bytes_and_holder/expected/go
```

未恢复的 Go panic 直接终止 Python 进程。error 映射和将 Go 数据地址直接交给外部
长期读取留待后续案例。
