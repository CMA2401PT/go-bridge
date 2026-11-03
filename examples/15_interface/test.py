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
        hachimi = bridge.adopt(kind="dog", name="Hachimi")
        assert_type(hachimi, bridge.Animal)
        self.assertIsInstance(hachimi, bridge.GoInterface)
        self.assertEqual(hachimi.kind(), "dog")
        self.assertEqual(hachimi.speak(times=2), "Hachimi: woof woof")

    # 类型断言检查接口内部的动态类型，Dog、Cat 与 *Cat 彼此不同。
    def test_type_assertions_recover_the_concrete_dynamic_type(self) -> None:
        hachimi = bridge.adopt(kind="dog", name="Hachimi")
        self.assertEqual(bridge.Animal.from_any(value=hachimi).kind(), "dog")
        self.assertEqual(bridge.Dog.from_any(value=hachimi).name, "Hachimi")
        self.assertEqual(
            bridge.Dog.from_any(value=hachimi.as_any()).name,
            "Hachimi",
        )
        self.assertIsNone(bridge.PtrCat.try_from_any(value=hachimi))

        bruce = bridge.adopt(kind="cat", name="Bruce")
        self.assertEqual(bridge.PtrCat.from_any(value=bruce).name, "Bruce")
        self.assertIsNone(bridge.Cat.try_from_any(value=bruce))
        with self.assertRaises(TypeError):
            bridge.Cat.from_any(value=bruce)
        with self.assertRaises(TypeError):
            bridge.Dog.from_any(value=bruce)

    # Go 按方法集判断可赋值性；Python 必须先显式构造接口包装。
    def test_interface_arguments_require_explicit_compatible_wrappers(self) -> None:
        self.assertEqual(
            bridge.introduce(animal=bridge.Animal(value=bridge.Dog(name="Hachimi"))),
            "this is a dog. Hachimi: woof",
        )
        cat = bridge.Cat(name="Bruce")
        self.assertEqual(
            bridge.introduce(animal=bridge.Animal(value=cat.addr())),
            "this is a cat. Bruce: meow",
        )
        with self.assertRaises(TypeError):
            bridge.Animal(value=cat)  # type: ignore[arg-type]

        dog = bridge.Dog(name="Hachimi")
        with self.assertRaises(TypeError):
            bridge.introduce(animal=dog)  # type: ignore[arg-type]
        self.assertEqual(
            bridge.introduce(animal=bridge.Animal(value=dog.addr())),
            "this is a dog. Hachimi: woof",
        )

    # 值装入接口后独立于原变量，指针装入接口后仍指向原变量。
    def test_interface_values_copy_structs_and_share_pointers(self) -> None:
        dog = bridge.Dog(name="Hachimi")
        hachimi = bridge.Animal(value=dog)
        dog.name = "Hachimi II"
        self.assertEqual(hachimi.speak(times=1), "Hachimi: woof")

        cat = bridge.Cat(name="Bruce")
        bruce = bridge.Animal(value=cat.addr())
        cat.name = "Bruce II"
        self.assertEqual(bruce.speak(times=1), "Bruce II: meow")
        del cat
        gc.collect()
        self.assertEqual(bruce.speak(times=1), "Bruce II: meow")

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
        bruce = bridge.adopt(kind="cat", name="Bruce")
        value, err = bridge.feed(animal=bruce, food="fish")
        assert_type(err, bridge.error)
        self.assertEqual(value, "cat ate fish")
        self.assertTrue(err.is_nil)

        value, err = bridge.feed(animal=bruce, food="bone")
        self.assertEqual(value, "")
        self.assertFalse(err.is_nil)
        self.assertEqual(err.error(), "Bruce only eats fish")


if __name__ == "__main__":
    unittest.main()
