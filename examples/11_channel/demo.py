"""演示 channel 的值与状态接收，以及用 Python 线程在后台接收、发送。"""

from pathlib import Path
import sys
import threading

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_channel as bridge


# 1. recv 对应 Go 的 v, ok := <-ch；零也是数据，Go 生产者关闭 channel 后返回 (0, False)
numbers: bridge.RecvChanInt = bridge.numbers(count=3)
received: list[int] = []
while True:
    value, ok = numbers.recv()
    if not ok:
        break
    received.append(value)
print("Go 发来的数据:", received)
print("关闭后再接收:", numbers.recv())


# 2. 后台接收：在 Python 线程里循环 recv。recv 等待时释放 GIL，主线程照常发送
def listen(ch: bridge.RecvChanInt, heard: list[int]) -> None:
    """持续接收直到 channel 被关闭；关闭就是让这个线程结束的信号。"""
    while True:
        value, ok = ch.recv()
        if not ok:
            return
        heard.append(value)


events = bridge.ChanInt(capacity=0)
heard: list[int] = []
listener = threading.Thread(
    target=listen,
    kwargs={"ch": events.recv_only(), "heard": heard},
)
listener.start()
for value in [7, 8, 9]:
    events.send(value=value)  # 无缓冲：每次发送都要等后台线程收走
events.close()
listener.join()
print("后台线程收到:", heard)


# 3. 后台发送：Python 线程往 channel 发送，同时主线程里的 Go 函数 Sum 在接收
def produce(ch: bridge.SendChanInt, values: list[int]) -> None:
    """发完全部数据后关闭 channel，让 Go 的 for range 结束。"""
    for value in values:
        ch.send(value=value)
    ch.close()


jobs = bridge.ChanInt(capacity=0)
producer = threading.Thread(
    target=produce,
    kwargs={"ch": jobs.send_only(), "values": [2, 3, 5]},
)
producer.start()
total: int = bridge.sum(ch=jobs.recv_only())
producer.join()
print("Go 收到并求和:", total)


# 4. 空字符串也是数据；关闭读空后同样返回空字符串，只能靠 ok 区分两者
text = bridge.ChanString(capacity=2)
text.send(value="hello")
text.send(value="")
text.close()
print("收到字符串:", text.recv())
print("收到空字符串:", text.recv())
print("关闭后:", text.recv())
