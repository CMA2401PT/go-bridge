"""演示 Go slice 的构造、共享底层数组、clone 隔离和嵌套切片浅拷贝。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_slice as bridge

# 1. 先把 Python 序列复制到 Go；需要遍历时整体复制为 list
numbers = bridge.SliceInt(values=[10, 20, 30])
print("Go 切片:", numbers.copy_to_py_list())

# 2. 切片共享底层数组，因此修改局部切片也会改变原切片
tail: bridge.SliceInt = numbers[1:]
tail[0] = 21
print("修改共享切片:", numbers.copy_to_py_list())

# 3. clone 复制底层数组，修改副本不会改变原切片
independent: bridge.SliceInt = numbers.clone()
independent[0] = 0
print(
    "clone: 原切片 =",
    numbers.copy_to_py_list(),
    "| 副本 =",
    independent.copy_to_py_list(),
)

# 4. 拼接遵循 Go append：容量足够时复用数组，后一次拼接会覆盖共享位置
storage = bridge.SliceInt(values=[1, 2, 0])
base: bridge.SliceInt = storage[:2]
first: bridge.SliceInt = bridge.concat_a(
    a=base,
    b=bridge.SliceInt(values=[3]),
)
print("首次拼接:", first.copy_to_py_list())
second: bridge.SliceInt = bridge.concat_a(
    a=base,
    b=bridge.SliceInt(values=[4]),
)
print(
    "再次拼接: 首次结果 =",
    first.copy_to_py_list(),
    "| 本次结果 =",
    second.copy_to_py_list(),
)

# 5. 嵌套 slice 的 clone 是浅拷贝，内层数组仍共享；外层会持有内层数据
inner = bridge.SliceInt(values=[7, 8])
outer = bridge.SliceSliceInt(values=[inner])
shallow: bridge.SliceSliceInt = outer.clone()
del inner
shallow[0][0] = 70
print("嵌套浅拷贝:", [row.copy_to_py_list() for row in outer.copy_to_py_list()])
