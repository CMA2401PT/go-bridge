"""用同一组用户行为验证手写参考实现和生成器产物。"""

from pathlib import Path
import gc
import importlib
import sys
import unittest
from typing import assert_type

example_dir = Path(__file__).resolve().parent
package_dir = example_dir.parent / ".build" / example_dir.name / "python"
sys.path.insert(0, str(package_dir))

import go_interface as bridge

_native = importlib.import_module(name="go_interface._native")


class InterfaceTests(unittest.TestCase):
    # 每个场景结束后，所有包装拥有的 Go 句柄都应释放。
    def setUp(self) -> None:
        gc.collect()
        self.baseline: int = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    # 返回接口时，Python 只需要知道接口承诺的方法。
    def test_returned_interface_exposes_animal_methods(self) -> None:
        bruce = bridge.adopt(kind="dog", name="Bruce")
        assert_type(bruce, bridge.Animal)
        self.assertIsInstance(bruce, bridge.GoInterface)
        self.assertEqual(bruce.kind(), "dog")
        self.assertEqual(bruce.speak(times=2), "Bruce: woof woof")

    # 类型断言检查接口内部的动态类型，Dog、Cat 与 *Cat 彼此不同。
    def test_type_assertions_recover_the_concrete_dynamic_type(self) -> None:
        bruce = bridge.adopt(kind="dog", name="Bruce")
        self.assertEqual(bridge.Animal.from_any(value=bruce).kind(), "dog")
        self.assertEqual(bridge.Dog.from_any(value=bruce).name, "Bruce")
        self.assertEqual(
            bridge.Dog.from_any(value=bruce.as_any()).name,
            "Bruce",
        )
        self.assertIsNone(bridge.PtrCat.try_from_any(value=bruce))

        hachimi = bridge.adopt(kind="cat", name="Hachimi")
        self.assertEqual(bridge.PtrCat.from_any(value=hachimi).name, "Hachimi")
        self.assertIsNone(bridge.Cat.try_from_any(value=hachimi))
        with self.assertRaises(TypeError):
            bridge.Cat.from_any(value=hachimi)
        with self.assertRaises(TypeError):
            bridge.Dog.from_any(value=hachimi)

    # Go 按方法集判断可赋值性；Python 必须先显式构造接口包装。
    def test_interface_arguments_require_explicit_compatible_wrappers(self) -> None:
        self.assertEqual(
            bridge.introduce(animal=bridge.Animal(value=bridge.Dog(name="Bruce"))),
            "this is a dog. Bruce: woof",
        )
        cat = bridge.Cat(name="Hachimi")
        self.assertEqual(
            bridge.introduce(animal=bridge.Animal(value=cat.addr())),
            "this is a cat. Hachimi: meow",
        )
        with self.assertRaises(TypeError):
            bridge.Animal(value=cat)  # type: ignore[arg-type]

        dog = bridge.Dog(name="Bruce")
        with self.assertRaises(TypeError):
            bridge.introduce(animal=dog)  # type: ignore[arg-type]
        self.assertEqual(
            bridge.introduce(animal=bridge.Animal(value=dog.addr())),
            "this is a dog. Bruce: woof",
        )

    # 值装入接口后独立于原变量，指针装入接口后仍指向原变量。
    def test_interface_values_copy_structs_and_share_pointers(self) -> None:
        dog = bridge.Dog(name="Bruce")
        bruce = bridge.Animal(value=dog)
        dog.name = "Bruce II"
        self.assertEqual(bruce.speak(times=1), "Bruce: woof")

        cat = bridge.Cat(name="Hachimi")
        hachimi = bridge.Animal(value=cat.addr())
        cat.name = "Hachimi II"
        self.assertEqual(hachimi.speak(times=1), "Hachimi II: meow")
        del cat
        gc.collect()
        self.assertEqual(hachimi.speak(times=1), "Hachimi II: meow")

    # nil 接口没有动态类型；含 nil 指针的接口仍能调用接收者允许的方法。
    def test_nil_interface_differs_from_an_interface_holding_a_nil_pointer(self) -> None:
        nobody = bridge.nobody()
        self.assertTrue(nobody.is_nil)
        self.assertTrue(bridge.Animal.nil().is_nil)
        self.assertIsNone(bridge.Animal.try_from_any(value=nobody))
        self.assertIsNone(bridge.Dog.try_from_any(value=nobody))
        self.assertIsNone(bridge.PtrCat.try_from_any(value=nobody))

        stray = bridge.stray()
        self.assertFalse(stray.is_nil)
        self.assertEqual(stray.speak(times=1), "(no cat)")
        self.assertTrue(bridge.PtrCat.from_any(value=stray).is_nil)

    # error 留在返回值里，由调用者检查，不会被自动抛成 Python 异常。
    def test_feed_returns_error_as_an_ordinary_interface_value(self) -> None:
        hachimi = bridge.adopt(kind="cat", name="Hachimi")
        value, err = bridge.feed(animal=hachimi, food="fish")
        assert_type(err, bridge.error)
        self.assertEqual(value, "cat ate fish")
        self.assertTrue(err.is_nil)

        value, err = bridge.feed(animal=hachimi, food="bone")
        self.assertEqual(value, "")
        self.assertFalse(err.is_nil)
        self.assertEqual(err.error(), "Hachimi only eats fish")


if __name__ == "__main__":
    unittest.main()
