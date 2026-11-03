# 11_channel：通过 Python 回调接收 Go channel

channel 是 Go 中用于传递值和同步执行的通道。Python 包装持有同一个 Go channel，
发送调用 `send(value)`；接收调用 `recv(callback)`，拿到一个值后在调用线程执行
Python 函数。等待收发时释放 GIL（Python 解释器锁），使其他 Python 线程可以完成
配对操作。缓冲容量为 0 时，发送和接收必须相遇才能继续。

```python
import go_channel as bridge

values: list[int] = []
ch = bridge.numbers(count=3)  # Go goroutine 发送 0、1、2，然后关闭
while ch.recv(callback=values.append):
    pass
assert values == [0, 1, 2]
```

构建后运行 [demo.py](demo.py)，可以直接看到本例的核心行为：

```sh
python examples/build.py 11_channel
python examples/11_channel/demo.py
```

```text
Go 发来的数据: [0, 1, 2]
关闭后再接收: False | 数据: [0, 1, 2]
关闭后读完缓冲区: [0, 9]
Go 收到并求和: 10
```

## 接收与回调

`recv(callback: Callable[[T], None]) -> bool` 每次接收一个值。成功时调用一次回调并
返回 True；channel 已关闭且缓冲区已读空时返回 False，不调用回调。零值也是正常数据。
回调返回值被忽略。回调抛出的 Python 异常原样向上传播，已经收到的值不会退回 channel。

回调在调用 recv 的 Python 线程中同步执行，可以再次调用桥接 API。C++ 先释放 GIL，
进入 Go 等待一个值；Go 返回后重新取得 GIL，转换结果并调用回调。Go 不保存 Python
函数，也不调用 Python。这里不引入后台监听、额外线程或取消对象；需要后台接收时，
调用者可以使用自己的 Python 线程。阻塞中的调用没有超时或取消，不能依靠 Python
信号中断等待；应用需要通过发送或关闭 channel 让等待结束。

## 类型、方向和生命周期

| Go 类型 | Python 类型 | 操作 |
| --- | --- | --- |
| `chan int` | `ChanInt` | 构造、send、recv、close、send_only、recv_only |
| `chan<- int` | `SendChanInt` | send、close |
| `<-chan int` | `RecvChanInt` | recv |
| `chan string` | `ChanString` | 构造、send、recv、close；验证字符串传输 |

`ChanInt(capacity=0)` 对应 `make(chan int, capacity)`。`send_only()` 和 `recv_only()`
创建方向受限的包装，与原 channel 共享队列和关闭状态。Go 参数接受对应的准确包装
类型；传给 `<-chan int` 参数前显式调用 recv_only。命名 channel 的一般规则与命名
slice 一致：从底层包装显式构造，unname 返回底层包装，均共享 channel。
手写样例聚焦 int 的方向转换和 string 的资源传输；生成器还需将这些规则用于命名
channel、其他已支持元素和字段、指针、slice、any 中的 channel。

所有 channel 包装都有 nil() 和 is_nil。nil 使用 handle 0，不分配 holder；nil 上
收发永久阻塞。关闭 nil、重复关闭、向已关闭的 channel 发送都会触发 Go panic，
未恢复的 panic 直接终止 Python 进程。Python 不捕获 Go panic。谁负责关闭 channel
由应用协议决定；释放包装只删除持有关系，不关闭 channel。

非 nil channel 使用 cgo.Handle 保存准确的 Go 类型，ABI 只传整数句柄。每个返回包装
拥有一个独立句柄，Go channel 本身由 Go GC 管理。收发沿用元素类型的复制与共享规则：
基础值按值传输，string 跨边界复制，struct 按值复制，slice 和指针保持其共享语义。
接收结果由 C++ 在转换前接管，因此 Python 解码或回调异常也会释放传输资源。

## 生成器需要知道什么

Go 模型需要 channel 元素类型和方向；类型闭包包含元素，并为双向 channel 补齐
两个方向类型。Python 模型决定类型名、方向允许的成员及回调注解。ABI 分别规划
构造、收发、关闭和方向转换，接收结果由 ok 与元素值组成。ok 表示接收成功，不是
panic 标志。输出器沿用已有元素转换和句柄所有权协议。

原 Go 函数和方法也可能阻塞在 channel 上，所以它们的 C ABI 调用同样释放 GIL。
Python 入参在调用期间保持存活，字符串在 Go 侧复制后使用。GIL 不保护 Go 对象：
channel 自身可并发收发，共享 struct/slice 的并发访问由调用者按 Go 规则同步。

## 验证

在仓库根目录运行：

```sh
python examples/build.py 11_channel --test
MYPYPATH=examples/.build/11_channel/python python -m mypy examples/11_channel/test.py --strict
```

测试覆盖 Go 生产者、Python 线程间无缓冲收发、阻塞的 Go 消费者、缓冲区读空、
方向转换、nil 传回、回调线程身份、重入和异常后的下一次接收。每个测试结束后检查
句柄计数回到起点。阻塞测试应由外部测试运行器设置总超时，避免实现错误时无限等待。
