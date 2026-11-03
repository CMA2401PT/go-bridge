"""演示 Go 多返回值如何变成 Python tuple，以及如何按声明顺序解包。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_multi_return as bridge

# 1. 一次 Go 调用的多个返回值，合在一个普通 tuple 中
pair: tuple[int, int] = bridge.return_two(value=10)
print("两个返回值:", pair, "| 类型:", type(pair).__name__)

# 2. 按 Go 声明顺序解包；Go 的返回值名不会变成 Python 属性
first: int
second: int
first, second = pair
print("按顺序解包: first =", first, "| second =", second)

# 3. 返回四项时规则相同，每个位置都有自己的类型声明
four: tuple[int, int, int, int] = bridge.return_four(value=-2)
print("四个返回值:", four)
