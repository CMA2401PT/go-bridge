# Examples

`examples/` 用于研究和定义 **Go API 应当如何映射为可用的 Python API**。
这里的案例是代码生成器的行为参考与黄金测试。先人工验证应该生成什么，再实现生成器。
生成器的当前能力见[仓库首页](../readme.md)。
生成产物不要求逐字匹配 expected；部分案例的规则与生成器不同，nil 表示以 09 为准。
样例与生成器均不捕获 Go panic，未恢复的 panic 终止整个 Python 进程。

## 目录约定

```text
examples/
├── readme.md
├── types.md                      # 类型映射总览
├── build.py                      # 共用构建入口
├── .build/                       # 构建产物，不作为黄金代码
│   └── 01_basic/
│       ├── native/               # Go 动态库和 C header
│       ├── python/               # 可导入的完整 Python package
│       └── temp/                 # C++ 编译临时文件
└── 01_basic/
    ├── readme.md                 # 本例范围、映射规则和限制
    ├── G.go                      # 输入：普通 Go API
    ├── test.py                   # 验证：Python 用户看到的行为
    ├── demo.py                   # 可直接运行的使用例子
    └── expected/                 # 手写的预期生成结果
        ├── go/
        │   └── main.go           # package main，导出 C ABI
        ├── bindings.cpp          # pybind11 扩展
        └── go_basic_value/
            ├── __init__.py       # Python 公共 API
            └── __init__.pyi      # 公共 API 类型声明
```

每个案例只维护输入、期望输出、测试、使用例子和本例文档。`expected/` 中的代码由人工
编写，不是生成器产物；`.build/` 中的内容可以通过构建重新得到。

Go 适配层单独放在 `expected/go/`，因为它是独立的 `package main`。
`expected/` 下应有且只有一个包含 `__init__.py` 的 Python package；构建工具
从目录识别包名，不需要额外配置。包可以包含其他 Python 文件。

## 构建与测试

要求 Go、cgo、可用的 C/C++ 编译器，以及安装了 `pybind11` 和 `setuptools` 的
Python。当前支持 macOS 和 Linux。以下命令在仓库根目录运行：

```sh
python examples/build.py 01_basic --test
```

去掉 `--test` 则只构建。需要指定 Python 环境时，直接用对应解释器执行：

```sh
/path/to/python examples/build.py 01_basic --test
```

构建和测试始终使用同一个解释器。脚本自身使用绝对路径定位源码，也可以从其他
工作目录调用。Go 适配层编译为 `libbridge.dylib` 或 `libbridge.so`，对应头文件
名为 `libbridge.h`，C++ 绑定应包含该头文件并定义 `_native` 模块。

工具编译、复制手写文件，并为组装后的 package 添加空的 `py.typed`。动态库与
扩展位于同一 package 内，通过相对 loader path 加载。每次构建都会重新组装该
案例的 Python package，避免残留已删除的 API 或旧扩展；不要手工编辑 `.build/`。

单独运行测试或做静态检查：

```sh
python examples/01_basic/test.py
MYPYPATH=examples/.build/01_basic/python mypy examples/01_basic/test.py --strict
```

单独测试需使用构建时的解释器；mypy 是可选工具。各案例的 `test.py` 将自己的
`.build/<案例名>/python` 加入导入路径。其他使用者可将该目录加入 `PYTHONPATH`。

## 新增案例

1. 参照 `01_basic` 创建新目录，编写聚焦单一特性的 `G.go`。
2. 修改 `expected/go/main.go` 的 Go import 和 C ABI 适配代码。
3. 编写 `expected/bindings.cpp`、Python package 和 `.pyi`；包名按案例选择。
4. 编写 `test.py`、`demo.py` 和本例 `readme.md`；readme 中附上 demo 的运行命令和输出。
5. 运行 `python examples/build.py <案例目录名> --test`。

不需要复制构建脚本、setup.py、py.typed 或构建产物。Python 调用通过编译后的
pybind11 扩展完成，不使用 ctypes。公共工具只负责构建，不推导或生成绑定语义。

## 每个案例应说明什么

- 研究哪些 Go 声明和类型，以及 Python 用户最终看到的 API。
- Go、C ABI、Python 三层类型映射，以及 `.pyi` 如何表达接口。
- 数据复制、内存所有权、对象或句柄的创建和释放。
- error、panic、空值及其他异常情况的行为。
- 生成器需要获得哪些 Go AST 或类型信息。
- 已确认规则、暂定选择、平台假设和未覆盖的边界。

通用环境要求与构建约定统一维护在本文；案例文档只记录自身的设计。

## 验证范围与研究顺序

已确认的类型映射规则汇总在 [`types.md`](types.md)，各案例 readme 记录细节与推导。

案例应能从干净环境构建并通过 Python 测试，公开 API 与 `.pyi` 保持一致。
涉及资源管理和失败路径时，应验证释放、重复调用和异常行为。`01_basic` 作为
最小目录模板只保留三个正常调用，不代表完整的标量边界测试规范。

- [`01_basic`](01_basic/readme.md)：基础标量参数和返回值，也是新案例的目录模板。
- [`02_multi_return`](02_multi_return/readme.md)：多返回值，通过 C 结构体映射为固定长度 Python tuple。
- [`03_str`](03_str/readme.md)：字符串转换、按值 BrString ABI 与内存释放。
- [`04_bytes_and_holder`](04_bytes_and_holder/readme.md)：通用 Ref、地址句柄与 Go 生命周期，以字节切片为例。
- [`05_slice`](05_slice/readme.md)：值元素与 Go 对象元素的 slice，保持 Go 的共享与 append 语义。
- [`06_named_type`](06_named_type/readme.md)：命名值类型、命名 slice 与 struct，ABI 由底层类型决定。
- [`07_ptr_basic`](07_ptr_basic/readme.md)：Python 中不可寻址的值类型（标量、slice 及其命名类型）的指针，值与指针的转换和方法集。
- [`08_multi_ptr`](08_multi_ptr/readme.md)：多级指针、命名指针，以及生成器必须补齐的依赖类型（类型闭包）。
- [`09_nil`](09_nil/readme.md)：带类型的 nil、nil 接收者，以及未恢复的 Go panic 直接终止进程。
- [`10_any`](10_any/readme.md)：显式 any 装箱、准确动态类型断言、try 的 `T | None`、复制与共享、nil interface 与带类型 nil，以及未知动态类型的持有。
- [`11_channel`](11_channel/readme.md)：通过同步 Python 回调接收 channel，验证方向、阻塞收发、GIL 和关闭语义。
- [`12_map`](12_map/readme.md)：Go map 的键比较、共享、nil、浅复制和批量读取。
- [`13_array`](13_array/readme.md)：定长数组的值复制、变量视图和取址。
- [`14_func`](14_func/readme.md)：持有、调用和传回 Go 函数值，保留闭包状态。
- [`15_interface`](15_interface/readme.md)：GoInterface 公共基类、Go 接口方法调用、静态转换与动态断言，error 是普通接口值。

编号代表研究顺序。新案例如果暴露出已有规则的问题，先修正文档和黄金样例，再改
生成器。公共构建约定随真实案例需要调整，不预先引入复杂的配置机制。
