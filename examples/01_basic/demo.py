"""演示从 Python 向 Go 传递整数、布尔值和固定宽度整数，并读取返回值。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_basic_value as bridge

# 1. Go 的 int 在 Python 中直接用 int，函数名采用 snake_case
next_number: int = bridge.int_in_and_out(value=1)
print("int: 1 ->", next_number, flush=True)

# 2. Go 的 bool 对应 Python bool，本例的 Go 函数将真假反转
inverted: bool = bridge.bool_in_and_out(value=True)
print("bool: True ->", inverted, flush=True)

# 3. Go 的 uint16 也用 Python int 传入，但可接受的范围仍由 Go 类型决定
next_uint16: int = bridge.basic_go_spec_type(value=41)
print("uint16: 41 ->", next_uint16, flush=True)
