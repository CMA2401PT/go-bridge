# 11_channel：Go channel 的收发

channel 是 Go 中用于传递值和同步执行的通道。Python 包装持有同一个 Go channel，
用 `send(value)` 发送，用 `recv()` 接收。等待收发时释放 GIL（Python 解释器锁），
其他 Python 线程可以继续运行。缓冲容量为 0 时，发送和接收必须相遇才能继续。

```python
import go_channel as bridge

ch = bridge.numbers(count=3)  # Go goroutine 发送 0、1、2，然后关闭
while True:
    value, ok = ch.recv()
    if not ok:
        break
    print(value)
```

构建后运行 [demo.py](demo.py)，可以直接看到本例的核心行为：

```sh
python examples/build.py 11_channel
python examples/11_channel/demo.py
```

```text
Go 发来的数据: [0, 1, 2]
关闭后再接收: (0, False)
后台线程收到: [7, 8, 9]
Go 收到并求和: 10
收到字符串: ('hello', True)
收到空字符串: ('', True)
关闭后: ('', False)
```

## 接收：`value, ok = ch.recv()`

`recv() -> tuple[T, bool]` 对应 Go 的 `v, ok := <-ch`。收到值时返回 `(值, True)`；
channel 已关闭且缓冲区读空时返回 `(零值, False)`。零值、空字符串和 nil 元素都可能是
对方真的发来的数据，所以判断 channel 是否结束只能看 `ok`，不能看值本身。

C++ 先释放 GIL，进入 Go 等待一个值；Go 返回后重新取得 GIL，再把结果转换成 Python 值。
阻塞中的调用没有超时或取消，Python 信号也不能中断它；要让等待结束，只能通过发送或
关闭 channel。

## 后台接收与发送：用 Python 线程

需要"后台监听"时，自己开一个 Python 线程循环调用 `recv()`：

```python
def listen(ch: bridge.RecvChanInt, heard: list[int]) -> None:
    while True:
        value, ok = ch.recv()
        if not ok:
            return
        heard.append(value)

threading.Thread(target=listen, kwargs={"ch": events.recv_only(), "heard": heard}).start()
```

`recv()` 等待期间释放了 GIL，所以后台线程阻塞在 Go 里时，主线程照常运行，可以继续
发送数据或做别的事。发送方向同理：后台线程循环 `send()`，主线程调用一个消费 channel 的
Go 函数（本例的 `sum`），两边在 channel 上配对，见 [demo.py](demo.py) 的第 2、3 步。

桥接层不提供"注册一个 Python 回调、由 Go 在后台调用"这样的监听接口。用自己的线程，
三个问题都留在调用者手里，答案也很直接：

- 什么时候停：关闭 channel，`recv()` 返回 `ok=False`，循环自然结束。
- 处理数据时出错怎么办：异常就发生在你自己的线程函数里，按普通 Python 代码处理。
- 在哪个线程处理数据：就是你开的那个线程。

如果由桥接层代为监听，这些问题都要另设一套协议来回答，比如取消对象、异常转交，以及
解释器退出时 Go 还在回调 Python 怎么办。而换来的只是少写几行线程代码。

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
接收结果由 C++ 在转换前接管，因此 Python 解码失败也会释放传输资源。

## 生成器需要知道什么

Go 模型需要 channel 元素类型和方向；类型闭包包含元素，并为双向 channel 补齐
两个方向类型。Python 模型决定类型名和方向允许的成员。ABI 分别规划
构造、收发、关闭和方向转换，接收结果由 ok 与元素值组成；关闭读空时元素值是 Go 零值。
ok 表示接收成功，不是 panic 标志。输出器沿用已有元素转换和句柄所有权协议。

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
方向转换、nil 传回、关闭后的零值与 ok=False、真实零值的 ok=True，以及字符串传输。每个测试结束后检查
句柄计数回到起点。阻塞测试应由外部测试运行器设置总超时，避免实现错误时无限等待。
