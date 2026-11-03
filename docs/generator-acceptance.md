# 01–15 生成器验收

验收对象是当前生成器产出的 Go、C++ 和 Python package，不编译手写 `expected/`。
验收环境为 macOS arm64、Go 1.27.1、CPython 3.13、pybind11 3.0.1。Linux 构建路径
存在，但尚未在 Linux 上验收；free-threaded Python、子解释器与 wheel 分发不在范围内。

## 覆盖与证据

| 范围 | 原生验证内容 | 测试位置（internal/generate） |
| --- | --- | --- |
| 01–03 | 基础数值、各宽度越界、字符串/NUL/UTF-8、多返回值 | generate_test.go |
| 04–05 | bytes、一般/嵌套 slice、索引、切片共享、clone、批量复制 | integration_slice_test.go |
| 06 | named 标量/slice、struct 零值/字段/提升字段、方法、视图与复制 | integration_object_test.go |
| 07 | 基础值和 slice 的指针、named 值的指针方法、slice header 更新 | integration_pointer_test.go |
| 08 | 多级指针、重定向、named pointer 转换与解引用 | integration_pointer_test.go |
| 09 | typed nil、nil 接收者方法、Go 环、致命 panic 与 nil 解引用 | integration_pointer_test.go |
| 10 | 全部基础转换、命名类型断言、复制／共享、typed nil、未知值、失败清理 | integration_any_test.go |
| 11 / channel 组合 | 三种方向、阻塞/GIL、回调线程/异常/重入、命名与递归、字段/指针/slice/any、关闭 panic | integration_channel_test.go |
| 12 / map 组合 | Go 键比较、struct 复制、引用共享、命名/递归、nil 写入 panic、批量清理 | integration_map_test.go |
| 13 / array 组合 | 定长/零长构造、值复制、变量视图、取址、命名数组与容器组合 | integration_array_test.go |
| 14 / func 组合 | 闭包共享、命名/递归、多返回值、GIL、nil 调用 panic 和结果清理 | integration_func_test.go |
| 15 / interface 组合 | Go 实现方法、静态转换、动态断言、普通 error、nil/typed nil、私有方法与容器组合 | integration_interface_test.go |
| any 组合 | any 字段、`*any`、`[]any`、命名容器、类型收窄、alias-only 包、并发句柄释放 | integration_any_test.go |
| 组合 | 外部包类型、隐藏嵌入、命名字符串方法、struct slice、转换失败清理 | integration_object_test.go |
| 资源 | 多模块同时导入、拒绝异模块对象、GC、失败路径句柄计数 | integration_slice_test.go |
| 初始化 | 无导出函数/仅 alias 包、import 等待 Go init、init panic | generate_test.go |
| 类型与并发 | 消费者 mypy、递归模型确定性、holder 并发/根计数 | acceptance_test.go |

所有原生扩展构建均启用 `GOEXPERIMENT=cgocheck2` 和 `-gcflags=all=-d=checkptr=2`，
实际执行字符串与 slice 缓冲区等跨 C 边界路径。holder 的纯 Go 测试以 `-race` 验证
并发与根计数；它虽也开启 checkptr/cgocheck2，但不经过 C 边界，不能替代原生测试。
句柄计数检查外部根释放，不能据此声称测量过整个进程的所有 C++/Python 分配或
任意 Go 目标包的并发安全。

字段视图测试在删除父持有关系并 GC 后读取数据；这属于行为证据，未用 weak 指针
独立观测对象回收。对象保活还依赖 runtime/cgo.Handle 的根语义，不能只凭旧内存
仍可读取推断对象尚未被回收。

## 复现

```sh
go test ./...
go vet ./...
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/... -count=1
```

该 Python 必须安装 pybind11、setuptools 和 mypy。未设置 `GO_BRIDGE_PYTHON` 时，
测试会跳过原生检查；未设置 `GO_BRIDGE_MYPY=1` 时会跳过声明与消费者类型检查。
不能把仅运行普通 `go test` 的结果当作完整验收。

## 语义边界

桥接层不恢复 Go panic；显式 panic、nil 解引用、内部句柄断言失败均可终止进程。
致命行为在独立 Python 子进程验证，断言执行前标记、Go panic 输出、非零退出，
并确认调用没有正常返回。子进程直接执行致命调用，不尝试捕获 panic。目标 Go 代码自己
写的 recover 仍有效。Python 输入转换错误和 C++ 资源清理遵循
[运行时契约](generator-runtime.md)。

与部分手写样例的差异包括：整数越界为 OverflowError；struct
构造省略字段得到 Go 零值，显式 None 不替代零值；保留成员名加下划线；具体包装类
禁止 Python 子类化。slice 不可隐式迭代，复制到 Python 要显式调用批量方法。

尚未支持的类型/API 由分析或规划阶段明确拒绝：匿名 struct、complex、unsafe.Pointer、泛型、可变参数、
导出变量/常量和 named bool。
命名冲突也明确报错，不悄悄丢弃字段或方法。完整对象规则见
[对象协议](generator-objects.md)，各阶段职责见[分工与设计](generator-architecture.md)。
any 的支持边界与转换协议见 [any 装箱与断言](generator-any.md)。
