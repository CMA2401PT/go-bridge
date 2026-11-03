"""演示带类型的 nil、可处理 nil 的链表方法，以及指针字段的共享。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_nil as bridge

# 1. nil 仍是同类型的指针包装；用 is_nil 判断，指向零的指针并不是 nil
missing: bridge.PtrInt = bridge.maybe_int(ok=False)
zero = bridge.PtrInt(value=0)
print(f"nil 指针: {missing.is_nil} | 指向零: {zero.is_nil}")
print(f"传给 Go: {bridge.deref(p=missing)} {bridge.deref(p=zero)}")

# 2. Go 的 Len 和 Push 自行处理 nil 接收者，因此可以从 nil 空链表开始添加节点
empty: bridge.PtrList = bridge.PtrList.nil()
values: bridge.PtrList = empty.push(v=2).push(v=1)
print(f"空链表长度: {empty.len()} | 添加后: {values.len()}")

# 3. 查找返回原节点的指针，修改会反映到链表；没有找到仍返回 PtrList
found: bridge.PtrList = bridge.find(l=values, v=2)
# value_ 是 Go 的 Value 字段；value 已用于指针解引用
found.value_ = 20
not_found: bridge.PtrList = bridge.find(l=values, v=9)
print(f"链表第二项: {values.next.value_} | 未找到是 nil: {not_found.is_nil}")

# 4. 读取 next 复制指针；随后断开链表，不会改变已经取出的尾节点指针
tail: bridge.PtrList = values.next
values.next = bridge.PtrList.nil()
print(f"断开后长度: {values.len()} | 原尾节点: {tail.value_}")
