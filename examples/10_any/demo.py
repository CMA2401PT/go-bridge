"""演示 any 的显式装箱、动态类型断言、复制与共享，以及带类型 nil。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_any as bridge

# 1. 基础值要显式装进 GoAny；断言要求 Go 类型相同，MyInt 不能当成 int 取出
boxed_int: bridge.GoAny = bridge.int_as_any(value=42)
returned: bridge.GoAny = bridge.echo(value=boxed_int)
print("int 装箱再返回:", bridge.any_as_int(value=returned))
boxed_named: bridge.GoAny = bridge.MyInt(value=42).as_any()
restored_named: bridge.MyInt = bridge.MyInt.from_any(value=boxed_named)
print(
    f"MyInt 取出: {restored_named.unname()}"
    f" | 按 int 断言: {bridge.try_any_as_int(value=boxed_named)}"
)

# 2. struct 装箱复制值，指针装箱共享目标；取出时分别断言 Point 和 PtrPoint
point = bridge.Point(x=1)
boxed_point: bridge.GoAny = point.as_any()
point.x = 9
copied_point: bridge.Point = bridge.Point.from_any(value=boxed_point)
print(f"原 struct: {point.x} | 装箱副本: {copied_point.x}")
boxed_pointer: bridge.GoAny = point.addr().as_any()
shared_point: bridge.PtrPoint = bridge.PtrPoint.from_any(value=boxed_pointer)
shared_point.x = 10
print("通过指针修改后:", point.x)

# 3. slice 装箱只复制 slice 头，取出后仍共享底层数组
numbers = bridge.SliceInt(values=[1, 2])
restored_slice: bridge.SliceInt = bridge.SliceInt.from_any(value=numbers.as_any())
restored_slice[0] = 8
print("原 slice:", numbers.copy_to_py_list())

# 4. nil 接口没有动态类型；装着 nil 指针的接口有 *int 类型，因此接口本身非 nil
nil_interface: bridge.GoAny = bridge.GoAny.nil()
typed_nil: bridge.GoAny = bridge.PtrInt.nil().as_any()
restored_nil: bridge.PtrInt = bridge.PtrInt.from_any(value=typed_nil)
print(f"nil 接口: {nil_interface.is_nil} | 装着 nil 指针的接口: {typed_nil.is_nil}")
print(
    f"从 nil 接口断言: {bridge.PtrInt.try_from_any(value=nil_interface)}"
    f" | 取出的指针是 nil: {restored_nil.is_nil}"
)
