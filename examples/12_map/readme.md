# 12_map：共享的 Go map

map 包装持有同一个 Go map。参数、返回、赋值和命名转换共享其内容；clone 创建浅复制。
Python 通过下标读写，缺键读取抛 KeyError，in 在 Go 中检查键。delete(key) 与 Go delete
相同，缺键时没有操作。clear 清空同一个 map。

构造接收键值对 iterable，重复的 Go 键由后面的值覆盖。copy_to_py_items 一次性复制
为键值对列表，顺序不保证。使用列表保留 Go 的全部键语义：struct 或指针包装在 Python
中按对象身份比较，不等价于 Go 的值或指针比较，因此不自动生成 dict。
标量键需要 Python dict 时，可以显式调用 dict(value.copy_to_py_items())。

```python
import go_map as bridge
value = bridge.MapStringInt(items=[("a", 1)])
alias = bridge.echo(value=value)
alias["a"] = 2
assert value["a"] == 2
```

nil() 与 is_nil 保留 Go nil map；nil 可读、可删除、长度为零，写入会触发 Go panic。
未恢复的 panic 终止进程。Python bool 使用 len；空 map 与 nil map 都是假值。
释放包装只删除 holder。逐项隐式迭代被禁用，批量读取使用 copy_to_py_items。

ABI 用整数句柄传递 map，零表示 nil。构造借用键和值数组；查询返回 ok 与拥有的值；
批量输出在转换前接管全部键和值。struct 元素按值复制，slice、指针、map 等共享引用。
Go 判断键相等；any 键若包含不可比较的动态值，会按 Go 规则 panic。
调用者负责同步并发读写，Python 的 GIL 不保护正在其他 Go 调用中使用的 map。

在仓库根目录运行：

```sh
python examples/build.py 12_map --test
MYPYPATH=examples/.build/12_map/python python -m mypy examples/12_map/test.py --strict
```

手写样例验证 string→int 的复制、共享、重复键、缺键、nil 和句柄释放。生成器
递归收集键和值类型，为命名 map、字段、指针、slice、channel 和 any 组合复用该协议。
