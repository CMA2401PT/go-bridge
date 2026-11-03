"""演示多级指针修改目标、改换指向，以及命名指针和 struct 指针的共享。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_multi_ptr as bridge

# 1. ptr() 增加一层指针；通过三级指针修改，原来的整数也会改变
number = bridge.PtrInt(value=1)
double: bridge.PtrPtrInt = number.ptr()
triple: bridge.PtrPtrPtrInt = double.ptr()
bridge.inc_ppp(p=triple)
print(f"原整数: {number.value} | 三级指针读到: {triple.value.value.value}")

# 2. 改换中间指针的目标后，外层能看到新目标，先前取出的指针仍指向旧目标
previous: bridge.PtrInt = double.value
replacement = bridge.PtrInt(value=10)
bridge.retarget(pp=double, p=replacement)
bridge.inc_ppp(p=triple)
print(f"旧目标: {previous.value} | 新目标: {replacement.value}")

# 3. IntPtr 是 Go 的命名指针类型；构造和 unname() 转换都共享原整数
named = bridge.IntPtr(p=number)
unnamed: bridge.PtrInt = named.unname()
bridge.inc_int_ptr(p=named)
print(f"原整数: {number.value} | 命名指针: {named.value} | 转回后: {unnamed.value}")

# 4. struct 先用 addr() 取得地址，再用 ptr() 构造二级指针，Go 修改的是原 struct
box = bridge.Box(count=bridge.Counter(value=1), inner=bridge.Inner(n=2))
box_pointer: bridge.PtrPtrBox = box.addr().ptr()
bridge.inc_box_pp(pp=box_pointer)
print(box.echo())
