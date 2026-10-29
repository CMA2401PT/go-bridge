# 调用 Go interface 的实现

Go 返回接口时，Python 得到对应接口的包装对象。包装持有 Go 的动态值，并暴露接口
声明的方法；即使实现是未导出的 Go 类型，也可以调用。接口包装和 `GoAny` 都继承
`GoInterface`，后者继承负责句柄所有权的 `Ref`。具体包装类不可继承，不使用
Python `Protocol`，因为 Python 对象实现同名方法并不能成为 Go interface 的实现者。

需要把已知的 Go 包装对象传给接口参数时，使用 `Counter(value)` 显式转换。生成器
通过 Go 的可赋值性规则决定允许的来源，并写入 `.pyi`。值接收者和指针接收者有不同
方法集，因此 `T` 能调用某个指针方法，不代表 Go 的 `T` 值就实现了对应接口。
普通函数的接口参数只接受对应接口包装，返回类型也始终是声明的接口包装。

需要检查接口内的动态值时，使用 `Counter.from_any(value)`。它接受任何
`GoInterface` 包装，包括其他接口和 `GoAny`；不匹配时抛 TypeError。
`try_from_any()` 不匹配时返回 None。`as_any()` 保留原动态类型，因此仍可断言回
已支持的具体 Go 类型。基础 Python 值先显式装箱，例如 `int_as_any(1)`，以选择 Go 类型。

`nil()` 创建 nil interface。兼容接口之间的构造转换保留 nil，而对 nil interface
做类型断言会失败。`is_nil` 只判断 interface 自身；包含 typed nil 指针的 interface
不为 nil。方法原样交给 Go 执行，未恢复的 panic 终止 Python 进程。

Go `error` 是声明 `Error() string` 的普通接口。Python 类保留名字 `error`，方法为
`error()`，函数的 `(T, error)` 返回 Python tuple，不自动抛异常。

命名接口直接使用原 Go 类型，保留未导出方法的包身份。未导出方法参与 Go 的可赋值性
和断言检查，Python 仅调用导出方法。匿名接口按方法签名生成类名；如果包含未导出
方法，适配包无法表达相同类型，生成器会明确报错。接口嵌入展开为方法集，不建立
Python 接口继承树；接口之间用显式转换或断言。

```python
import go_interface
counter = go_interface.new(start=10)
shared = go_interface.Counter(value=counter)
assert shared.add(arg1=2) == 12
assert go_interface.Counter.from_any(value=counter.as_any()).add(arg1=1) == 13
value, err = go_interface.result(fail=True)
assert err.error() == "ordinary error"
```

手写例子验证隐藏实现、接口共享、构造转换、动态断言和普通 error 返回值。生成器的
组合测试还覆盖结构体与命名标量实现者、指针方法集、嵌入接口、跨包接口、带私有方法
的接口，以及字段、指针、slice、map、数组、channel、函数值中的接口。

```sh
python examples/build.py 15_interface --test
MYPYPATH=examples/.build/15_interface/python python -m mypy --strict examples/15_interface/test.py
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/generate -run TestIntegrationInterface -count=1
```
