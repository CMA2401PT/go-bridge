"""演示 Go 函数值的调用、传回 Go、闭包状态共享，以及 nil 函数判断。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_func as bridge

# 1. Go 返回的函数包装可直接调用；函数类型不保留形参名，因此使用 arg1
counter: bridge.FuncIntToInt = bridge.counter(start=10)
print("Python 调用加 2:", counter(arg1=2))

# 2. 把函数包装传回 Go，Apply 调用的仍是同一个闭包，继续累加原来的状态
print("传回 Go 加 3:", bridge.apply(value=counter, delta=3))

# 3. Echo 返回的包装共享闭包；删除原包装后，返回的包装仍可调用
shared: bridge.FuncIntToInt = bridge.echo(value=counter)
print("返回的包装减 1:", shared(arg1=-1))
print("原包装再加 1:", counter(arg1=1))
del counter
print("删除原包装后加 2:", shared(arg1=2))

# 4. nil 也是同类型的函数包装，用 is_nil 判断；调用 nil 函数会使 Go panic
nil_function: bridge.FuncIntToInt = bridge.nil()
returned_nil: bridge.FuncIntToInt = bridge.echo(value=nil_function)
print(f"普通函数是 nil: {shared.is_nil} | 返回的 nil 函数: {returned_nil.is_nil}")
