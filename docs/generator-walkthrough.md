# 追踪一次生成与调用

[文档导航](README.md) · [分工](generator-architecture.md) · [运行时协议](generator-runtime.md)

本文用一个含 alias、不同参数类型和两个返回值的函数贯穿整条链路。它与
[TestSignaturesAgreeAcrossFiles](../internal/emit/emit_test.go) 使用同样的签名。

## 输入与复现

目标 Go 包的内容是：

```go
package sample

type Text = string

func Pair(n int, s Text, f float32) (Text, bool) {
    return s, n > 0
}
```

在仓库根目录、同一 shell 中执行以下命令，可在临时目录生成真实产物。使用安装了
pybind11 和 setuptools 的 Python 解释器：

```sh
bridge_demo=$(mktemp -d)
mkdir "$bridge_demo/source"
cat > "$bridge_demo/source/go.mod" <<'EOF'
module example.test/walkthrough

go 1.25.5
EOF
cat > "$bridge_demo/source/api.go" <<'EOF'
package sample

type Text = string

func Pair(n int, s Text, f float32) (Text, bool) {
    return s, n > 0
}
EOF
go run . -out "$bridge_demo/generated" -name go_pair "$bridge_demo/source"
python "$bridge_demo/generated/build.py"
PYTHONPATH="$bridge_demo/generated/.build/python" python -c 'import go_pair; assert go_pair.pair(n=2, s="你好\0", f=1.5) == ("你好\0", True)'
```

产物是调试对象，规则修改仍应回到生成器源文件。修改生成的 `bindings.cpp` 不会改变
下一次生成结果，也不会进入生成器回归测试。

## 第一步：Go 模型

从 [Run](../internal/generate/generate.go) 跟到
[Load → Analyze](../internal/gomodel/load.go)：

1. `Load` 在目标目录加载包，得到 import path、类型信息和目标尺寸。
2. `Analyze` 预先加入基础类型，再读取导出声明。
3. Pair 记录为包级 Function，参数是 int、string、float32，结果是 string、bool。
4. Text 记录为 `Declaration{Alias: true}`，指向已有 string 节点。

这里还没有 pair 这个 Python 名字，也没有 C 的 intptr_t。Text 是真正 alias，Go
签名中的 Text 已归一化为 string；`type Text string` 则会产生 Named 节点，并在当前
Python 规划阶段被拒绝。

## 第二步：Python 接口

[pyapi.Plan](../internal/pyapi/model.go) 使用 `basicName`、`planFunction` 得到：

| 内容 | 结果 |
| --- | --- |
| 包名 | CLI 明确指定的 go_pair |
| 函数名 | Pair → pair |
| 参数名 | n、s、f，来自原签名 |
| 参数类型 | int、str、float，各自仍引用原 Go 节点 |
| 返回类型 | str、bool，按原返回顺序保存 |
| 类型 alias | Text → Python str |

[stubSource](../internal/emit/python.go) 稍后会将这些信息渲染为：

```python
import builtins as _builtins

Text = _builtins.str
def pair(n: _builtins.int, s: _builtins.str, f: _builtins.float) -> _builtins.tuple[_builtins.str, _builtins.bool]: ...
```

限定 `_builtins` 使将来同包出现 `Int`、`Tuple` 等函数时，类型标注仍指向正确的内建
类型。Text 保留为公开 alias，函数签名直接使用归一化后的 str。

## 第三步：ABI 操作

[abi.Plan](../internal/abi/model.go) 为这个唯一的函数分配 BrCall0/BrResult0。相关 C
声明的含义如下；实际头文件由后续 cgo 构建产生：

```c
typedef struct { char *data; size_t size; } BrString;
typedef struct {
    BrString r0;
    uint8_t r1;
} BrResult0;

BrResult0 BrCall0(intptr_t a0, BrString a1, float a2);
```

Go int 的具体范围由目标尺寸决定；bool 通过 uint8_t 传输。返回的第一个字符串拥有
C 缓冲区，第二个 bool 没有资源所有权。结果不携带 panic 信息。

## 第四步：生成两端代码

[goOperation](../internal/emit/adapter.go) 生成的导出签名和正常调用核心是：

```go
func BrCall0(a0 C.intptr_t, a1 C.BrString, a2 C.float) (out C.BrResult0)
// 函数体内：
r0, r1 := api.Pair(int(a0), bridgeInput(a1), float32(a2))
out.r0 = bridgeString(string(r0))
if r1 {
    out.r1 = 1
}
```

上面只摘出相关语句；完整函数还有 `//export` 和末尾的 return，不安装 defer/recover。

[cppFunction](../internal/emit/bindings.go) 注册 Python 名字 pair，先调用
`integer_arg<intptr_t>`、`string_arg`、`float_arg<float>` 转换三个输入，然后调用
`BrCall0`。正常返回的结果立即放入 `OwnedBrResult0`，再构造两个元素的 tuple。
函数退出或转换中途抛出 Python 异常时，owner 析构都释放结果字符串。

`pythonSource` 导入 `_native` 并重新导出 pair，定义 Text alias 与 `__all__`。
即使包中只有 alias、没有函数，也会加载 `_native`。C++ 模块初始化调用内部 BrInit，
等待目标 Go 包初始化完成；该入口不占用 BrCallN 序号，也不公开到 Python。
`Files` 汇总上述产物并格式化 Go 源码；`publish` 最后写入输出目录。

## 编译后的调用

运行 `build.py` 才会得到动态库、cgo 头文件和扩展，运行生成器本身不会执行这些编译。
`go_pair.pair(2, "你好\0", 1.5)` 的实际路径是：

```text
Python 参数
  → C++ 类型/范围检查，借用字符串的 UTF-8 缓冲区
  → BrCall0，Go 复制字符串后调用原始 Pair
  → Go 分配 C 结果字符串，返回 BrResult0
  → C++ 严格 UTF-8 解码并构造 tuple
  → owner 释放 C 缓冲区，Python 持有自己的返回对象
```

把 n 换成超出 Go int 范围的整数，会在 C++ 层抛 OverflowError，Pair 尚未执行。
目标函数未自行恢复的 panic 会终止整个 Python 进程。普通结果中包含非法 UTF-8 则在结果转换阶段
抛 UnicodeDecodeError；无论在哪个结果上失败，整个结果结构中的字符串仍会被释放。
