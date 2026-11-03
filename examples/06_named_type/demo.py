"""演示 Go 命名类型的显式转换，以及 struct 字段视图和两种接收者的修改语义。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_named_type as bridge

# 1. 命名 int 自带 Go 方法；Python 加法返回普通 int，传回命名参数前需恢复类型
number = bridge.MyNamedNonStructInt(value=5)
print("命名 int:", number.echo_non_struct_int())
incremented: int = number + 1
renamed = bridge.MyNamedNonStructInt(value=incremented)
print(
    "加法结果:",
    incremented,
    type(incremented).__name__,
    "| 恢复命名:",
    bridge.fn_echo_non_struct_int(i=renamed),
)

# 2. 命名 slice 需要显式转换，两个方向的转换都共享底层数组
plain = bridge.SliceInt(values=[1, 2])
named = bridge.MyNamedNonStructSliceInt(values=plain)
named[0] = 10
back: bridge.SliceInt = named.unname()
back[1] = 20
print(
    "转换后共享:",
    plain.copy_to_py_list(),
    "| 命名切片:",
    named.copy_to_py_list(),
)

# 3. 构造 struct 时复制子 struct；读出的子字段是视图，修改会写回父对象
sub = bridge.MyNamedSubStruct(basic_member_value=6)
record = bridge.MyNamedStruct(named_ref_member_value=sub)
sub.basic_member_value = 60
print(
    "构造复制: 源对象 =",
    sub.basic_member_value,
    "| 子字段 =",
    record.named_ref_member_value.basic_member_value,
)
record.named_ref_member_value.basic_member_value = 7
print(
    "字段视图写回:",
    record.named_ref_member_value.basic_member_value,
    "| 省略字段的零值:",
    record.basic_member_value,
)

# 4. 指针接收者修改原 struct，值接收者只修改副本并返回
record.inc_basic_ptr()
updated: bridge.MyNamedStruct = record.inc_basic_non_ptr()
print("接收者: 原对象 =", record.basic_member_value, "| 返回副本 =", updated.basic_member_value)
