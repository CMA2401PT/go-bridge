"""演示定长数组的零值与索引、按值复制，以及通过指针修改原数组。"""

from pathlib import Path
import copy
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_array as bridge

# 1. 数组长度属于类型；默认是三个零，也可以用恰好三个元素构造并使用负下标
zero = bridge.Array3Int()
numbers = bridge.Array3Int(values=[1, 2, 3])
print(f"默认数组: {zero.copy_to_py_list()} | 长度: {len(zero)}")
numbers[-1] = 8
print("修改末项:", numbers.copy_to_py_list())

# 2. 数组按值传给 Go，Increment 修改并返回副本，原数组保持不变
increased: bridge.Array3Int = bridge.increment(value=numbers)
print(f"原数组: {numbers.copy_to_py_list()} | Go 返回: {increased.copy_to_py_list()}")

# 3. copy.copy() 也复制整个数组，修改副本不会影响原数组
copied: bridge.Array3Int = copy.copy(numbers)
copied[1] = 20
print(f"原数组: {numbers.copy_to_py_list()} | Python 复制: {copied.copy_to_py_list()}")

# 4. 要让 Go 修改原变量，用 addr() 取得数组指针
bridge.mutate(value=numbers.addr())
print(f"指针修改后: {numbers.copy_to_py_list()} | 先前副本: {copied.copy_to_py_list()}")
