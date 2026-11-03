"""演示 Python str 与 Go string 的 UTF-8 转换、内嵌 NUL 保留和按值传递。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_str as bridge

# 1. 直接传入 str，Go 返回的 string 也会转换成 str
text: str = "hello"
echo: str = bridge.recv_and_return(value=text)
print("普通文本:", repr(echo), flush=True)

# 2. 中文和 emoji 通过 UTF-8 往返，无需调用者手动编码
unicode_echo: str = bridge.recv_and_return(value="中文 🐍")
print("Unicode:", repr(unicode_echo), flush=True)

# 3. 字符串按长度传递，NUL 不会截断；repr 让不可见字符也能看见
nul_echo: str = bridge.recv_and_return(value="a\0b")
print("内嵌 NUL:", repr(nul_echo), flush=True)

# 4. 返回值是独立的 Python 字符串，输入变量保留原文
print("按值传递:", repr(text), "->", repr(echo), flush=True)
