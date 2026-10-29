# 定长数组

Go 数组把元素包含在值里。Python 包装一个 Go 数组变量，按下标修改这个变量；向 Go
传参、从 Go 返回以及 `copy.copy()` 都复制整个数组。复制数组中的 slice、map、指针
等元素时，仍遵循这些元素自己的共享语义。

需要让 Go 修改原变量时，调用 `addr()` 取得指针。数组字段、数组或 slice 内的数组
元素，以及数组指针的 `value`，都是对应位置的视图；从 map 读取数组则得到副本。
视图持有 Go 指针，外层 Python 对象销毁后仍然有效。

`Array3Int()` 创建零值。`Array3Int(values=[1, 2, 3])` 复制可迭代对象，元素数量必须
恰好为三。下标支持负数。数组长度固定，使用 `copy_to_py_list()` 显式复制到 Python。
命名数组从对应未命名数组构造，`unname()` 也复制数组值。

```sh
python examples/build.py 13_array --test
MYPYPATH=examples/.build/13_array/python python -m mypy --strict examples/13_array/test.py
```
