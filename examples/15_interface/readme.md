# 15_interface：Go interface

Go 没有继承，只有"实现了接口"这一种关系，而且这个关系从来不写在类型声明上。
本例的 `Dog` 和 `Cat` 都实现了 `Animal`，但 Go 代码里没有任何一处写着
"Dog 实现 Animal"，编译器在需要的时候按方法集临时判断。

Python 表达"具有某种能力"的常规办法是继承或 `Protocol`，两者都要求把关系写在类上。
如果照搬到这里，生成器就得分析出每个类型实现了哪些接口，再让 Python 类逐一继承，
一个类的声明会列出一长串接口，难以阅读，也和 Go 代码对不上。

所以 Python 侧不建立任何继承关系，而是模仿 Go 在运行时的做法：

- Go 函数返回接口时，Python 得到一个**接口包装**，例如 `Animal`。它持有 Go 的接口值
  （动态类型加上值），可以直接调用接口声明的方法。
- 要取回接口里的具体类型，用 `Dog.from_any(animal)`，相当于 Go 的类型断言 `animal.(Dog)`。
- 要把具体对象传给接口参数，用 `Animal(dog)` 显式构造接口值，相当于 Go 里的隐式赋值，
  只是在 Python 里要写出来。

## 本例的 Go API

```go
type Animal interface {
	Kind() string
	Speak(times int) string
}

type Dog struct{ Name string }          // 值接收者实现：Dog 和 *Dog 都是 Animal
type Cat struct{ Name string }          // 指针接收者实现：只有 *Cat 是 Animal

func Adopt(kind string, name string) Animal
func Introduce(animal Animal) string
func Feed(animal Animal, food string) (string, error)
```

完整代码见 [G.go](G.go)。想先看整体效果，构建后直接运行 [demo.py](demo.py)，它按
"领养 → 断言 → 传参 → 喂食"走一遍完整流程：

```sh
python examples/build.py 15_interface
python examples/15_interface/demo.py
```

```text
cat | Hachimi: meow meow
dog | Bruce: woof woof
renamed through *Cat: Hachimi the Great: meow
is bruce a *Cat? False
this is a dog. Bruce Jr.: woof
this is a cat. Hachimi Jr.: meow
fed: cat ate fish
failed: Hachimi the Great only eats fish
```

下面逐个拆开讲，片段都来自 [test.py](test.py)。

## 返回接口：只看得到接口的方法

```python
import go_interface as bridge

bruce = bridge.adopt(kind="dog", name="Bruce")  # 类型是 Animal
bruce.kind()                                    # "dog"
bruce.speak(times=2)                            # "Bruce: woof woof"
```

`adopt` 的返回类型固定是 `Animal`，不会因为里面装的是 Dog 就变成 `Dog`。
这和 Go 一样：调用者只知道拿到了一个 Animal。

## 类型断言：取回具体类型

```python
dog = bridge.Dog.from_any(value=bruce)              # 断言成功，得到 Dog
dog.name                                            # "Bruce"
bridge.PtrCat.try_from_any(value=bruce)             # 不是 *Cat，返回 None

hachimi = bridge.adopt(kind="cat", name="Hachimi")
bridge.PtrCat.from_any(value=hachimi).name          # "Hachimi"
bridge.Cat.try_from_any(value=hachimi)              # None：动态类型是 *Cat，不是 Cat
bridge.Dog.from_any(value=hachimi)                  # TypeError
```

`from_any` 对应 Go 的 `v := x.(T)`，失败抛 `TypeError`；`try_from_any` 对应
`v, ok := x.(T)`，失败返回 `None`。断言看的是接口里的**动态类型**，Go 里 `Cat` 和 `*Cat`
是不同的类型，所以 Python 里 `Cat` 和 `PtrCat` 也要分别断言。

每个 Go 类型的包装和每个接口包装都有这两个方法，参数可以是任何接口包装，包括 `GoAny`。
所以也能从一个接口断言到另一个接口，例如 `Animal.from_any(value=some_any)`。

## 传入接口参数：必须显式构造

```python
dog = bridge.Dog(name="Bruce")
bridge.introduce(animal=bridge.Animal(value=dog))           # 正确
bridge.introduce(animal=dog)                                # TypeError，mypy 也会报错

cat = bridge.Cat(name="Hachimi")
bridge.introduce(animal=bridge.Animal(value=cat.addr()))    # 正确：*Cat 是 Animal
bridge.Animal(value=cat)                                    # TypeError：Cat 值不是 Animal
```

`Animal(...)` 只接受在 Go 里能赋值给 Animal 的类型，生成器用 Go 的方法集规则算出这个
列表，写进 `.pyi`：

```python
class Animal(GoInterface):
    def __new__(cls, value: Animal | Dog | PtrDog | PtrCat) -> Self: ...
```

注意列表里没有 `Cat`。Python 的 `Cat` 包装的是一个可寻址的 Go 变量，Go 允许在这样的
变量上调用指针方法，所以生成的 `Cat` 也有 `kind()`、`speak()`。但 `Cat` 值的方法集
不包含指针方法，它不是 Animal。
这也是参数类型不用 `Protocol` 的原因：`Protocol` 只比较方法名和签名，会把 `Cat` 当成
Animal 放行，到运行时才失败。普通 Python 对象凑齐同名方法也能骗过 `Protocol`，
但它并不是 Go 的实现。

## 装进接口：复制还是共享

接口值和 Go 一样，装进去的是值本身：

```python
dog = bridge.Dog(name="Bruce")
bruce = bridge.Animal(value=dog)
dog.name = "Bruce II"
bruce.speak(times=1)    # "Bruce: woof"，接口里是一份复制

cat = bridge.Cat(name="Hachimi")
hachimi = bridge.Animal(value=cat.addr())
cat.name = "Hachimi II"
hachimi.speak(times=1)      # "Hachimi II: meow"，接口里是指向 cat 的指针
```

struct 和数组装进接口时被复制；指针装进去后仍指向原变量；slice、map 等保持各自的
共享规则。

## nil 接口与装着 nil 指针的接口

```python
bridge.nobody().is_nil      # True：Go 返回 nil，接口没有动态类型
stray = bridge.stray()      # Go 返回 (*Cat)(nil)
stray.is_nil                # False：接口里有动态类型 *Cat，只是指针为 nil
stray.speak(times=1)        # "(no cat)"，Cat 的方法自己处理了 nil 接收者
```

这和 Go 里著名的"非 nil 的 nil 接口"一致。`Animal.nil()` 构造 nil 接口；对 nil 接口
做断言总是失败。接口包装不能直接用在 `if` 里，要用 `is_nil`，避免把"是 nil"和
"是零值"混为一谈。

## error 是普通接口

```python
value, err = bridge.feed(animal=hachimi, food="bone")
if not err.is_nil:
    err.error()   # "Hachimi only eats fish"
```

Go 的 `error` 就是声明了 `Error() string` 的接口，Python 里同样是接口包装，类名保留
`error`，方法是 `error()`。`(T, error)` 返回 tuple，不会自动抛成 Python 异常。

## 其他规则

- 所有接口包装和 `GoAny` 都继承 `GoInterface`，它提供 `is_nil` 和 `as_any()`。
  具体的接口类不能被 Python 继承。
- 接口嵌入展开为方法集，不建立 Python 继承树；接口之间也用显式构造或断言转换。
- 命名接口保留原 Go 类型。接口里的未导出方法照样参与 Go 的可赋值性和断言检查，
  Python 只能调用导出方法。匿名接口按方法签名生成类名；如果它包含未导出方法，
  生成器无法在适配包里写出同一个类型，会报错。
- 方法调用释放 GIL；Go panic 不被捕获，会终止 Python 进程。
- 只能持有和调用 Go 实现的接口，Python 对象不能作为 Go 接口的实现传进去。

协议细节和生成器实现见 [接口包装与转换](../../docs/generator-interface.md)。

## 运行

```sh
python examples/build.py 15_interface --test
python examples/15_interface/demo.py
MYPYPATH=examples/.build/15_interface/python python -m mypy --strict examples/15_interface/test.py examples/15_interface/demo.py
GO_BRIDGE_PYTHON=python GO_BRIDGE_MYPY=1 go test ./internal/generate -run TestIntegrationInterface -count=1
```

第三条命令让生成器从同一份 G.go 生成绑定，再跑同一个 test.py。生成器的组合测试还覆盖
命名标量实现者、嵌入接口、跨包接口、带未导出方法的接口，以及字段、指针、slice、map、
数组、channel、函数值里的接口。
