# channel 与 GIL

Go channel 通过 Ref 包装在 Python 中共享。send(value) 发送一个值；recv() 对应 Go 的
`v, ok := <-ch`，返回 `(值, ok)`。关闭且缓冲区读空后返回 `(零值, False)`。零值和
nil 元素都可能是正常数据，所以是否结束只由 ok 表达。

等待 channel 时必须让其他 Python 线程继续运行，否则无缓冲收发可能相互等待 GIL。
C++ 在进入 Go 收发前释放 GIL，Go 返回后重新获取 GIL，再转换值。
原 Go 函数和方法同样可能阻塞，所以它们的 ABI 调用也释放 GIL。Go 从不调用 Python；
后台收发由调用者用自己的 Python 线程循环 recv/send 完成。

## Python 类型与 Go 语义

`chan T`、`chan<- T`、`<-chan T` 分别命名为 ChanT、SendChanT、RecvChanT。
三者是独立的 Ref 子类。双向包装以 capacity=0 构造，或通过 send_only/recv_only
取得方向受限的包装。只收类型提供 recv，只发类型提供 send/close。
命名 channel 从对应底层包装显式构造，unname 返回底层包装，两者共享 channel。
参数仍要求准确类型，不隐式缩窄方向。

nil() 返回同类的零值包装，is_nil 判断其是否为 nil。nil channel 收发永久阻塞；
关闭 nil、重复关闭、向关闭的 channel 发送或使用负容量构造会触发 Go panic。
桥接层不恢复 panic。close 的责任属于应用，包装析构只释放句柄。
没有后台订阅、超时或取消；应用通过自己的线程及发送/关闭协议控制等待。
Python 信号不能保证中断一个正在等待的 Go 调用。

channel 元素遵循已有转换：struct 复制，slice、指针、channel 共享引用，any 保留
动态类型。Go channel 支持并发收发；共享的 struct/slice 等数据仍须由调用者同步。
释放 GIL 不会自动使任意 Go API 成为线程安全的 API。

## 各层怎样实现

为了按方向限制可用操作，[Go 模型](../internal/gomodel/model.go)记录 Channel、Elem
和 Direction。named 节点仍指向底层节点。类型闭包递归收集元素，允许递归 channel。
[Python 补全](../internal/pyapi/completion.go)为双向 channel 补入两个方向类型；
[类规划](../internal/pyapi/classes.go)确定名字、CanSend/CanRecv 和方向转换的目标类型。
channel 成员名参加统一的保留名处理，Go 的 Send 方法因此成为 send_。

跨 ABI 需要同时保留 channel 和接收结果的所有权。
[abi.Channel](../internal/abi/channel.go)规划独立的构造、发送、接收、关闭和转换符号。
channel 值使用 uintptr_t 句柄，nil 是 0。非 nil holder 保存准确的底层 Go channel
类型，命名类型在调用或 any 装箱前恢复名字。方向转换生成独立句柄，引用同一 channel。
接收返回 `{ok, value}`；ok 为 False 时 value 是 Go 零值，和普通元素一样转换、释放。

[Go 输出](../internal/emit/channel_go.go)执行 make、收发、close 和方向转换，并复用
通用 goInput/goOutput。[C++ 输出](../internal/emit/channel_cpp.go)先建立结果 owner，
再释放 GIL 等待，重新取得 GIL 后转换元素并返回 tuple。字符串和句柄沿用既有
释放协议，解码失败不会泄漏传输资源。
[声明输出](../internal/emit/channel_python.go)提供 `tuple[T, bool]` 与准确的方向成员。

## 使用与验证

完整可运行用法见 [11_channel](../examples/11_channel/readme.md)。生成并构建：

```sh
go run . -out generated/channel ./examples/11_channel
python generated/channel/build.py
PYTHONPATH=generated/channel/.build/python python - <<'PYTHON'
import go_channel as bridge
values = []
ch = bridge.numbers(count=3)
while True:
    value, ok = ch.recv()
    if not ok:
        break
    values.append(value)
print(values)
PYTHON
```

输出目录须不存在或为空。原生验收从输入 Go 包重新生成，运行例子 11 的 Python 测试，
并另测命名/递归 channel、各类元素、字段/指针/slice/any 组合和关闭 panic。

```sh
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/generate -run TestIntegrationChannel -count=1 -v
```
