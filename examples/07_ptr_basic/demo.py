"""演示显式构造 Go 指针、经指针写回，以及标量副本和 slice 头的区别。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_ptr_basic as bridge

# 1. PtrInt 分配一个新的 Go 变量，Go 经指针修改它，Python 原整数保持不变
original: int = 5
pointer = bridge.PtrInt(value=original)
bridge.inc_int(i=pointer)
print("经指针递增: Python 原值 =", original, "| Go 变量 =", pointer.value)

# 2. Python 赋值产生指针别名，.value 则读写指向的 Go 变量
alias: bridge.PtrInt = pointer
alias.value = 10
print("别名写回:", pointer.value)

# 3. 命名值的 ptr() 复制出新变量；指针方法可修改它，之前读出的值是副本
named = bridge.MyNamedNonStructInt(value=1)
named_pointer: bridge.PtrMyNamedNonStructInt = named.ptr()
snapshot: bridge.MyNamedNonStructInt = named_pointer.value
named_pointer.inc()
print(
    "命名指针: 原值 =",
    named,
    "| 读取副本 =",
    snapshot,
    "| 当前值 =",
    named_pointer.value,
)
print("经指针调用值方法:", named_pointer.echo_non_struct_int())

# 4. slice 指针持有 slice 头；append 替换该头，原切片和已读出的头长度不变
numbers = bridge.SliceInt(values=[1])
slice_pointer: bridge.PtrSliceInt = numbers.ptr()
before: bridge.SliceInt = slice_pointer.value
bridge.append_slice_int(si=slice_pointer, v=2)
print(
    "append 后: 原切片 =",
    numbers.copy_to_py_list(),
    "| 旧切片头 =",
    before.copy_to_py_list(),
)
print("指针中的新切片:", slice_pointer.value.copy_to_py_list())
