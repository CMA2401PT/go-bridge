# 调用 Go interface 的实现

interface 包装持有 Go 返回的动态值，Python 根据接口声明调用方法。实现可以是未导出
类型，也可以来自其他包；生成器只需要知道公开接口。Python 对象不实现 Go interface。

Go `error` 是声明 `Error() string` 的普通接口。Python 类保留名字 `error`，方法为
`error()`，函数的 `(T, error)` 返回 Python tuple。桥接不会替调用者判断错误、抛异常
或捕获 Go panic。

`nil()` 创建 nil interface。`is_nil` 只判断 interface 自身是否为 nil；包含 typed nil
指针的 interface 不为 nil，其方法照常交给 Go 实现。nil interface 的方法调用由 Go
产生致命 panic。

生成器为一般接口补齐 `GoAny` 转换入口。`Counter.from_any(value.as_any())` 使用 Go
类型断言检查动态值是否实现 Counter；`try_from_any()` 在不满足接口时返回 None。
从接口 `as_any()` 保留具体动态类型，仍可断言回已支持的具体类型。命名接口还可以
从对应未命名接口显式构造，`unname()` 保留同一个动态值。

```python
import go_interface
counter = go_interface.new(start=10)
assert counter.add(arg1=2) == 12
value, err = go_interface.result(fail=True)
assert err.error() == "ordinary error"
```

```sh
python examples/build.py 15_interface --test
MYPYPATH=examples/.build/15_interface/python python -m mypy --strict examples/15_interface/test.py
```
