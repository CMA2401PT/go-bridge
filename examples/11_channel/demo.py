"""演示 channel 的回调接收、共享方向包装，以及无缓冲 channel 的线程配对。"""

from pathlib import Path
import sys
import threading

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_channel as bridge

# 1. recv 每次同步调用一次回调；零也是数据，Go 生产者关闭通道后返回 False
numbers: bridge.RecvChanInt = bridge.numbers(count=3)
received: list[int] = []
while numbers.recv(callback=received.append):
    pass
print("Go 发来的数据:", received)
print(f"关闭后再接收: {numbers.recv(callback=received.append)} | 数据: {received}")

# 2. 只发与只收包装共享通道；容量为二可先发送两项，关闭后仍能读完缓冲区
buffered = bridge.ChanInt(capacity=2)
sender: bridge.SendChanInt = buffered.send_only()
receiver: bridge.RecvChanInt = buffered.recv_only()
sender.send(value=0)
sender.send(value=9)
sender.close()
buffered_values: list[int] = []
while receiver.recv(callback=buffered_values.append):
    pass
print("关闭后读完缓冲区:", buffered_values)

# 3. 无缓冲收发必须配对；等待时释放 Python 解释器锁（GIL），让另一个线程能发送
unbuffered = bridge.ChanInt(capacity=0)


def send_numbers() -> None:
    """发送求和所需的数据，并关闭通道，让 Go 的 Sum 能结束等待。"""
    sum_sender: bridge.SendChanInt = unbuffered.send_only()
    for value in [2, 3, 5]:
        bridge.send(ch=sum_sender, value=value)
    sum_sender.close()


worker = threading.Thread(target=send_numbers)
worker.start()
total: int = bridge.sum(ch=unbuffered.recv_only())
worker.join()
print("Go 收到并求和:", total)
