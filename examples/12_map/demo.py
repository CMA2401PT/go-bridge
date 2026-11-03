"""演示 Go map 的重复键、共享与 clone、删除，以及 nil 与空 map 的区别。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_map as bridge

# 1. 用键值对构造 map，重复键保留后值；批量读取顺序不保证，这里排序便于阅读
scores = bridge.MapStringInt(items=[("a", 1), ("a", 2), ("b", 3)])
print("重复键保留后值:", sorted(scores.copy_to_py_items()))

# 2. Go 返回的 map 共享内容，clone 则创建独立的浅复制
shared: bridge.MapStringInt = bridge.echo(value=scores)
cloned: bridge.MapStringInt = scores.clone()
bridge.put(value=shared, key="a", item=5)
print(f"原 map: {scores['a']} | 返回的 map: {shared['a']} | clone: {cloned['a']}")

# 3. 删除和清空也作用于共享的 map，clone 的内容保持不变
shared.delete(key="a")
print("原 map 还有 a:", "a" in scores)
scores.clear()
print(f"清空后共享 map 长度: {len(shared)} | clone: {sorted(cloned.copy_to_py_items())}")

# 4. nil 和空 map 长度都为零，但只有空 map 可以写入；用 is_nil 区分
nil_map: bridge.MapStringInt = bridge.nil()
empty_map = bridge.MapStringInt()
print(
    f"nil / 空 map 的 is_nil: {nil_map.is_nil} {empty_map.is_nil}"
    f" | 长度: {len(nil_map)} {len(empty_map)}"
)
empty_map["a"] = 1
print("空 map 写入后:", sorted(empty_map.copy_to_py_items()))
