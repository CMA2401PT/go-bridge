"""演示用 SliceBytes 持有 Go 字节切片，以及显式复制和对象生命周期。"""

from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_bytes_and_holder as bridge

# 1. bytes 先复制成 Go 切片；Ref 基类负责在包装对象存活时持有 Go 值
source = bridge.SliceBytes(value=b"hello")
print(
    "Go 切片: 长度 =",
    len(source),
    "| 是 Ref:",
    isinstance(source, bridge.Ref),
    flush=True,
)

# 2. 返回值仍保存在 Go 中；释放输入包装后，返回包装仍可读取
result: bridge.SliceBytes = bridge.recv_and_return_bytes(value=source)
del source
print("释放输入后: 返回长度 =", len(result), flush=True)

# 3. 显式复制得到独立的 Python bytes，释放 Go 包装后仍可使用
copied: bytes = result.copy_to_py_bytes()
del result
print("释放返回包装后:", copied, flush=True)

# 4. 字节切片保存原始二进制，NUL 和非 UTF-8 字节都不会丢失
binary = bridge.SliceBytes(value=b"a\0b\xff")
print("二进制:", binary.copy_to_py_bytes(), "| 长度 =", len(binary), flush=True)
