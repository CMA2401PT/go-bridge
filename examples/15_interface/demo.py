"""从 Python 使用 Go interface 的完整流程，构建后直接运行即可看到每一步的结果。"""
# type: ignore
from pathlib import Path
import sys

example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_interface as bridge

# 1. Go 返回接口：Python 只拿到 Animal，看不到里面是 Dog 还是 *Cat
hachimi = bridge.adopt(kind="cat", name="Hachimi")
bruce = bridge.adopt(kind="dog", name="Bruce")
print(hachimi.kind(), "|", hachimi.speak(times=2))
print(bruce.kind(), "|", bruce.speak(times=2))

# 2. 类型断言取回具体类型；Hachimi 的动态类型是 *Cat，所以断言 PtrCat
cat = bridge.PtrCat.from_any(value=hachimi)
cat.name = "Hachimi the Great"
print("renamed through *Cat:", hachimi.speak(times=1))
print("is bruce a *Cat?", bridge.PtrCat.try_from_any(value=bruce) is not None)

# 3. 具体对象传给接口参数，必须显式构造 Animal
puppy = bridge.Dog(name="Bruce Jr.")
print(bridge.introduce(animal=bridge.Animal(value=puppy)))
kitten = bridge.Cat(name="Hachimi Jr.")
print(bridge.introduce(animal=bridge.Animal(value=kitten.addr())))

# 4. error 是普通返回值，用 is_nil 判断
for food in ["fish", "bone"]:
    result, err = bridge.feed(animal=hachimi, food=food)
    if err.is_nil:
        print("fed:", result)
    else:
        print("failed:", err.error())
