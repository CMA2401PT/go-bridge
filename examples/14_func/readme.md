# Go 函数值

Go 函数值可以包含闭包状态。Python 包装持有这个函数值，调用包装对象就调用 Go
函数；把包装对象传回 Go 也使用同一个函数及闭包。删除一个包装对象不影响其他
仍然持有函数的 Go 或 Python 对象。

`func(int) int` 映射为 `FuncIntToInt`。参数按位置命名为 `arg1`、`arg2`，因为 Go
函数类型的身份不包含参数名字。多返回值遵循普通 Go 函数的 tuple 规则。
函数值的调用释放 GIL，闭包的并发访问由调用者协调。

`nil()` 和 `is_nil` 表示 Go nil 函数。调用 nil 函数仍由 Go 产生致命 panic。
本例验证未命名函数值。生成器中的命名函数类型使用自己的类名，通过底层函数包装
构造，`unname()` 保留同一闭包。
Python callable 不能直接传给 Go；本协议描述 Go 函数值的持有与调用。

```python
import go_func
counter = go_func.counter(start=10)
assert counter(arg1=2) == 12
assert go_func.apply(value=counter, delta=3) == 15
```

```sh
python examples/build.py 14_func --test
MYPYPATH=examples/.build/14_func/python python -m mypy --strict examples/14_func/test.py
```
