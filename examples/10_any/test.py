from pathlib import Path
from typing import Any, Callable, assert_type
import copy
import gc
import importlib
import math
import struct
import subprocess
import sys
import unittest


example_dir = Path(__file__).resolve().parent
package_dir = example_dir.parent / ".build" / example_dir.name / "python"
sys.path.insert(0, str(package_dir))

import go_any as bridge

_native = importlib.import_module("go_any._native")
BITS = struct.calcsize("P") * 8


class Holders(unittest.TestCase):
    def setUp(self) -> None:
        gc.collect()
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(int(_native._holder_count()), self.baseline)

    def holders(self) -> int:
        gc.collect()
        return int(_native._holder_count()) - self.baseline


class InterfaceTests(Holders):
    def test_interface_copy_preserves_type_and_lifetime(self) -> None:
        original: bridge.GoInterface = bridge.int_as_any(value=42)
        self.assertIsInstance(original, bridge.Ref)
        self.assertEqual(bridge.any_as_int(value=original), 42)
        copied = original.as_any()
        assert_type(copied, bridge.GoAny)
        self.assertIsNot(copied, original)
        del original
        gc.collect()
        self.assertEqual(bridge.any_as_int(value=copied), 42)
        self.assertIsNone(bridge.try_any_as_string(value=copied))

        nil: bridge.GoInterface = bridge.GoAny.nil()
        self.assertTrue(nil.as_any().is_nil)
        typed_nil: bridge.GoInterface = bridge.PtrInt.nil().as_any()
        copied_nil = typed_nil.as_any()
        self.assertFalse(copied_nil.is_nil)
        self.assertTrue(bridge.PtrInt.from_any(value=copied_nil).is_nil)


class ScalarTests(Holders):
    def test_every_scalar_retains_its_go_type(self) -> None:
        cases: list[tuple[str, Any]] = [
            ("int", 0), ("int8", -128), ("int16", -32768),
            ("int32", -(2**31)), ("int64", -(2**63)),
            ("uint", 2**BITS - 1), ("uint8", 255), ("uint16", 65535),
            ("uint32", 2**32 - 1), ("uint64", 2**64 - 1),
            ("uintptr", 2**BITS - 1), ("float32", 1.25),
            ("float64", 1.25), ("bool", False), ("string", "中文\x00🙂"),
        ]
        for name, value in cases:
            with self.subTest(name=name):
                box: Callable[[Any], bridge.GoAny] = getattr(bridge, name + "_as_any")
                unbox: Callable[[bridge.GoAny], Any] = getattr(bridge, "any_as_" + name)
                attempt: Callable[[bridge.GoAny], Any] = getattr(bridge, "try_any_as_" + name)
                boxed = box(value)
                self.assertIs(type(boxed), bridge.GoAny)
                self.assertIsInstance(boxed, bridge.Ref)
                self.assertFalse(boxed.is_nil)
                self.assertEqual(bridge.dynamic_kind(boxed), name)
                self.assertEqual(unbox(boxed), value)
                self.assertEqual(attempt(boxed), value)
                self.assertIs(type(unbox(boxed)), type(value))
                returned = bridge.echo(boxed)
                self.assertIsNot(returned, boxed)
                self.assertEqual(unbox(returned), value)
                # Equal representations cannot substitute for Go type identity.
                for other, _ in cases:
                    if other == name:
                        continue
                    other_attempt = getattr(bridge, "try_any_as_" + other)
                    other_unbox = getattr(bridge, "any_as_" + other)
                    self.assertIsNone(other_attempt(boxed))
                    with self.assertRaises(TypeError):
                        other_unbox(boxed)

    def test_zero_values_are_successes(self) -> None:
        self.assertEqual(bridge.try_any_as_int(bridge.int_as_any(0)), 0)
        self.assertIs(bridge.try_any_as_bool(bridge.bool_as_any(False)), False)
        self.assertEqual(bridge.try_any_as_string(bridge.string_as_any("")), "")
        self.assertEqual(bridge.try_any_as_float64(bridge.float64_as_any(0.0)), 0.0)
        self.assertIsNone(bridge.try_any_as_int(bridge.bool_as_any(False)))

    def test_integer_boundaries(self) -> None:
        types = [("int", BITS, True), ("uint", BITS, False), ("uintptr", BITS, False)]
        types += [(f"int{bits}", bits, True) for bits in [8, 16, 32, 64]]
        types += [(f"uint{bits}", bits, False) for bits in [8, 16, 32, 64]]
        for name, bits, signed in types:
            box = getattr(bridge, name + "_as_any")
            unbox = getattr(bridge, "any_as_" + name)
            low = -(2 ** (bits - 1)) if signed else 0
            high = 2 ** (bits - 1) - 1 if signed else 2**bits - 1
            for good in [low, high]:
                self.assertEqual(unbox(box(good)), good)
            for bad in [low - 1, high + 1, 2**200]:
                with self.subTest(name=name, bad=bad), self.assertRaises(OverflowError):
                    box(bad)
            for wrong in [1.5, "1", None, object()]:
                with self.assertRaises(TypeError):
                    box(wrong)

    def test_basic_input_rules_are_preserved(self) -> None:
        # Existing integer input rules accept int subclasses, including bool.
        self.assertEqual(bridge.any_as_int(bridge.int_as_any(True)), 1)
        self.assertEqual(bridge.dynamic_kind(bridge.int_as_any(bridge.MyInt(2))), "int")
        self.assertEqual(bridge.any_as_float64(bridge.float64_as_any(3)), 3.0)
        for bad in [0, 1, None, "true"]:
            with self.assertRaises(TypeError):
                bridge.bool_as_any(bad)  # type: ignore[arg-type]
        for bad_string in [b"hello", 1, None]:
            with self.assertRaises(TypeError):
                bridge.string_as_any(bad_string)  # type: ignore[arg-type]
        for bad_float in ["1.0", None, object()]:
            with self.assertRaises(TypeError):
                bridge.float64_as_any(bad_float)  # type: ignore[arg-type]
        with self.assertRaises(UnicodeEncodeError):
            bridge.string_as_any("\ud800")

    def test_float32_rounding_and_special_values(self) -> None:
        expected = struct.unpack("f", struct.pack("f", 1.1))[0]
        self.assertEqual(bridge.any_as_float32(bridge.float32_as_any(1.1)), expected)
        self.assertEqual(bridge.any_as_float64(bridge.float64_as_any(1.1)), 1.1)
        self.assertTrue(math.isnan(bridge.any_as_float64(bridge.float64_as_any(float("nan")))))
        self.assertEqual(bridge.any_as_float32(bridge.float32_as_any(float("inf"))), float("inf"))
        negative_zero = bridge.any_as_float64(bridge.float64_as_any(-0.0))
        self.assertEqual(math.copysign(1.0, negative_zero), -1.0)

    def test_named_scalar_identity(self) -> None:
        value = bridge.MyInt(12)
        boxed = value.as_any()
        self.assertEqual(bridge.dynamic_kind(boxed), "anyvalue.MyInt")
        self.assertIs(type(bridge.MyInt.from_any(boxed)), bridge.MyInt)
        self.assertEqual(bridge.MyInt.try_from_any(boxed), value)
        self.assertIsNone(bridge.try_any_as_int(boxed))
        with self.assertRaises(TypeError):
            bridge.any_as_int(boxed)
        plain = bridge.int_as_any(12)
        self.assertIsNone(bridge.MyInt.try_from_any(plain))
        with self.assertRaises(TypeError):
            bridge.MyInt.from_any(plain)
        self.assertEqual(bridge.MyInt.from_any(bridge.echo(boxed)).unname(), 12)
        self.assertIs(type(bridge.echo_my_int(value)), bridge.MyInt)
        with self.assertRaises(TypeError):
            bridge.echo_my_int(12)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.MyInt.as_any(12)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.MyInt.unname(12)  # type: ignore[arg-type]

    def test_static_narrowing(self) -> None:
        value = bridge.try_any_as_int(bridge.int_as_any(4))
        assert_type(value, int | None)
        if value is not None:
            assert_type(value, int)
            self.assertEqual(value + 1, 5)
        named = bridge.MyInt.try_from_any(bridge.MyInt(4).as_any())
        assert_type(named, bridge.MyInt | None)
        if named is not None:
            assert_type(named, bridge.MyInt)
            self.assertEqual(named.unname(), 4)


class ObjectTests(Holders):
    def test_slices_share_arrays_and_retain_names(self) -> None:
        source = bridge.SliceInt([1, 2])
        boxed = source.as_any()
        first = bridge.SliceInt.from_any(boxed)
        second = bridge.SliceInt.from_any(boxed)
        self.assertIsNot(first, second)
        first[0] = 8
        self.assertEqual(source.copy_to_py_list(), [8, 2])
        self.assertEqual(second.copy_to_py_list(), [8, 2])
        named = bridge.MySlice(source)
        named_any = named.as_any()
        self.assertEqual(bridge.dynamic_kind(named_any), "anyvalue.MySlice")
        self.assertIsNone(bridge.SliceInt.try_from_any(named_any))
        self.assertIsNone(bridge.MySlice.try_from_any(boxed))
        recovered = bridge.MySlice.try_from_any(named_any)
        assert_type(recovered, bridge.MySlice | None)
        assert recovered is not None
        recovered[1] = 9
        self.assertEqual(source.copy_to_py_list(), [8, 9])
        self.assertIs(type(bridge.echo_my_slice(named)), bridge.MySlice)
        self.assertEqual(recovered.unname().copy_to_py_list(), [8, 9])
        del source, named, first, second, recovered
        _native._collect_go()
        self.assertEqual(bridge.SliceInt.from_any(boxed).copy_to_py_list(), [8, 9])

    def test_struct_boxing_and_assertion_copy_values(self) -> None:
        source = bridge.Point(x=1, values=bridge.SliceInt([2]), data=bridge.int_as_any(3))
        boxed = source.as_any()
        self.assertEqual(bridge.dynamic_kind(boxed), "anyvalue.Point")
        self.assertIsNone(bridge.PtrPoint.try_from_any(boxed))
        source.x = 10
        source.values[0] = 20
        source.data = bridge.string_as_any("replacement")
        first = bridge.Point.from_any(boxed)
        self.assertEqual(first.x, 1)
        self.assertEqual(first.values.copy_to_py_list(), [20])
        self.assertEqual(bridge.any_as_int(first.data), 3)
        first.x = 30
        first.values[0] = 40
        first.data = bridge.bool_as_any(True)
        second = bridge.Point.from_any(boxed)
        self.assertIsNot(first, second)
        self.assertEqual(second.x, 1)
        self.assertEqual(second.values.copy_to_py_list(), [40])
        self.assertEqual(bridge.any_as_int(second.data), 3)
        copied = copy.copy(second)
        copied.x = 99
        self.assertEqual(second.x, 1)
        self.assertEqual(bridge.echo_point(second).x, 1)
        self.assertFalse(hasattr(source, "nil"))

    def test_pointer_boxing_preserves_shared_target(self) -> None:
        source = bridge.Point(x=1)
        boxed = source.addr().as_any()
        self.assertEqual(bridge.dynamic_kind(boxed), "*anyvalue.Point")
        self.assertIsNone(bridge.Point.try_from_any(boxed))
        restored = bridge.PtrPoint.from_any(boxed)
        restored.x = 5
        self.assertEqual(source.x, 5)
        source.x = 6
        self.assertEqual(restored.value.x, 6)
        restored.value = bridge.Point(x=7)
        self.assertEqual(source.x, 7)
        del source, restored
        _native._collect_go()
        self.assertEqual(bridge.PtrPoint.from_any(boxed).x, 7)

    def test_named_pointer_and_multilevel_pointer(self) -> None:
        pointer = bridge.PtrInt(1)
        named = bridge.IntPtr(pointer)
        named_any = named.as_any()
        self.assertEqual(bridge.dynamic_kind(named_any), "anyvalue.IntPtr")
        self.assertIsNone(bridge.PtrInt.try_from_any(named_any))
        self.assertIsNone(bridge.IntPtr.try_from_any(pointer.as_any()))
        restored = bridge.IntPtr.from_any(named_any)
        restored.value = 2
        self.assertEqual(pointer.value, 2)
        self.assertEqual(restored.unname().value, 2)
        self.assertIs(type(bridge.echo_int_ptr(named)), bridge.IntPtr)
        double = pointer.ptr()
        boxed = double.as_any()
        self.assertEqual(bridge.dynamic_kind(boxed), "**int")
        self.assertIsNone(bridge.PtrInt.try_from_any(boxed))
        restored_double = bridge.PtrPtrInt.from_any(boxed)
        restored_double.value = bridge.PtrInt(9)
        self.assertEqual(double.value.value, 9)
        self.assertEqual(pointer.value, 2)
        self.assertEqual(bridge.echo_double_pointer(double).value.value, 9)

    def test_any_fields_copy_interface_values(self) -> None:
        point = bridge.Point()
        self.assertTrue(point.data.is_nil)
        self.assertEqual(point.values.copy_to_py_list(), [])
        point.data = bridge.int_as_any(2)
        previous = point.data
        point.data = bridge.string_as_any("next")
        self.assertEqual(bridge.any_as_int(previous), 2)
        self.assertEqual(bridge.any_as_string(point.data), "next")
        point.data = point.addr().as_any()  # A cycle entirely within Go.
        del previous
        _native._collect_go()
        self.assertEqual(bridge.PtrPoint.from_any(point.data).x, 0)

    def test_all_object_assertions_reject_wrong_dynamic_types(self) -> None:
        classes: list[Any] = [bridge.SliceInt, bridge.MySlice, bridge.Point,
                              bridge.PtrInt, bridge.PtrPoint, bridge.PtrPtrInt, bridge.IntPtr]
        boxed = bridge.int_as_any(1)
        for cls in classes:
            self.assertIsNone(cls.try_from_any(boxed))
            with self.assertRaises(TypeError):
                cls.from_any(boxed)


class NilTests(Holders):
    def test_nil_interface_has_no_handle(self) -> None:
        nil = bridge.GoAny.nil()
        self.assertTrue(nil.is_nil)
        self.assertTrue(bridge.nil_any().is_nil)
        self.assertTrue(bridge.echo(nil).is_nil)
        self.assertEqual(bridge.dynamic_kind(nil), "<nil>")
        self.assertEqual(self.holders(), 0)
        self.assertIsNone(bridge.try_any_as_int(nil))
        self.assertIsNone(bridge.PtrInt.try_from_any(nil))
        with self.assertRaises(TypeError):
            bridge.any_as_int(nil)
        with self.assertRaises(TypeError):
            bridge.PtrInt.from_any(nil)
        with self.assertRaises(TypeError):
            bool(nil)

    def test_typed_nil_pointer_is_a_successful_assertion(self) -> None:
        boxed = bridge.PtrInt.nil().as_any()
        self.assertFalse(boxed.is_nil)
        self.assertEqual(self.holders(), 1)
        self.assertEqual(bridge.dynamic_kind(boxed), "*int")
        restored = bridge.PtrInt.try_from_any(boxed)
        assert_type(restored, bridge.PtrInt | None)
        assert restored is not None
        self.assertTrue(restored.is_nil)
        self.assertEqual(self.holders(), 1)
        self.assertFalse(bridge.nil_pointer().is_nil)
        self.assertTrue(bridge.PtrInt.from_any(bridge.nil_pointer()).is_nil)
        self.assertIsNone(bridge.PtrPoint.try_from_any(boxed))
        with self.assertRaises(TypeError):
            bool(restored)

    def test_every_pointer_type_keeps_typed_nil(self) -> None:
        classes: list[Any] = [bridge.PtrInt, bridge.PtrPoint, bridge.PtrPtrInt, bridge.IntPtr]
        for cls in classes:
            boxed = cls.nil().as_any()
            self.assertFalse(boxed.is_nil)
            restored = cls.try_from_any(boxed)
            self.assertIsNotNone(restored)
            assert restored is not None
            self.assertTrue(restored.is_nil)
            self.assertTrue(cls.from_any(bridge.echo(boxed)).is_nil)
        nonnil_double = bridge.PtrPtrInt(bridge.PtrInt.nil())
        self.assertFalse(nonnil_double.as_any().is_nil)
        self.assertFalse(bridge.PtrPtrInt.from_any(nonnil_double.as_any()).is_nil)
        self.assertTrue(bridge.PtrPtrInt.from_any(nonnil_double.as_any()).value.is_nil)

    def test_nil_and_empty_slices_both_assert_successfully(self) -> None:
        for boxed, is_nil in [(bridge.nil_slice(), True), (bridge.empty_slice(), False)]:
            self.assertFalse(boxed.is_nil)
            self.assertEqual(bridge.dynamic_kind(boxed), "[]int")
            restored = bridge.SliceInt.try_from_any(boxed)
            assert restored is not None
            self.assertEqual(restored.copy_to_py_list(), [])
            self.assertFalse(hasattr(restored, "is_nil"))
            self.assertFalse(restored.as_any().is_nil)
            self.assertIs(bridge.slice_is_nil(boxed), is_nil)
            self.assertIs(bridge.slice_is_nil(restored.as_any()), is_nil)


class BoundaryTests(Holders):
    def test_unknown_dynamic_type_can_be_held_and_passed_back(self) -> None:
        boxed = bridge.unknown()
        self.assertEqual(bridge.dynamic_kind(boxed), "map[string]int")
        self.assertIsNone(bridge.try_any_as_int(boxed))
        self.assertIsNone(bridge.Point.try_from_any(boxed))
        returned = bridge.echo(boxed)
        del boxed
        _native._collect_go()
        self.assertEqual(bridge.unknown_value(returned), 42)

    def test_python_values_require_explicit_boxing(self) -> None:
        for bad in [1, None, bridge.MyInt(1), bridge.PtrInt(1), bridge.SliceInt([1])]:
            for call in [bridge.echo, bridge.any_as_int, bridge.try_any_as_int,
                         bridge.MyInt.try_from_any, bridge.Point.try_from_any]:
                with self.assertRaises(TypeError):
                    call(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                bridge.Point(data=bad)  # type: ignore[arg-type]
        point = bridge.Point()
        with self.assertRaises(TypeError):
            point.data = None  # type: ignore[assignment]
        with self.assertRaises(TypeError):
            bridge.Point(values=None)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.Point(1)  # type: ignore[misc]
        with self.assertRaises(TypeError):
            bridge.Point(unknown=1)  # type: ignore[call-arg]
        with self.assertRaises(TypeError):
            bridge.GoAny()  # type: ignore[call-arg]
        with self.assertRaises(TypeError):
            bridge.Ref(123)  # type: ignore[arg-type]

    def test_failed_assertions_allocate_no_handles(self) -> None:
        boxed = bridge.int_as_any(1)
        self.assertEqual(self.holders(), 1)
        for _ in range(100):
            self.assertIsNone(bridge.Point.try_from_any(boxed))
            self.assertIsNone(bridge.try_any_as_string(boxed))
            self.assertIsNone(bridge.MyInt.try_from_any(boxed))
            with self.assertRaises(TypeError):
                bridge.Point.from_any(boxed)
            with self.assertRaises(TypeError):
                bridge.any_as_string(boxed)
        self.assertEqual(self.holders(), 1)

    def test_try_does_not_hide_decoding_errors(self) -> None:
        boxed = bridge.invalid_string()
        self.assertEqual(bridge.dynamic_kind(boxed), "string")
        for _ in range(100):
            with self.assertRaises(UnicodeDecodeError):
                bridge.any_as_string(boxed)
            with self.assertRaises(UnicodeDecodeError):
                bridge.try_any_as_string(boxed)
        self.assertEqual(self.holders(), 1)

    def test_boxes_and_recovered_objects_own_independent_handles(self) -> None:
        source = bridge.SliceInt([7])
        first = source.as_any()
        second = source.as_any()
        self.assertEqual(self.holders(), 3)
        restored = bridge.SliceInt.from_any(first)
        self.assertEqual(self.holders(), 4)
        del source, first, second
        _native._collect_go()
        self.assertEqual(self.holders(), 1)
        self.assertEqual(restored.copy_to_py_list(), [7])
        del restored
        self.assertEqual(self.holders(), 0)

    def test_cyclic_go_values_do_not_keep_python_handles_alive(self) -> None:
        point = bridge.Point(x=7)
        point.data = point.addr().as_any()
        boxed = point.as_any()
        self.assertEqual(self.holders(), 2)
        del point
        _native._collect_go()
        recovered = bridge.Point.from_any(boxed)
        self.assertEqual(bridge.PtrPoint.from_any(recovered.data).x, 7)
        del recovered, boxed
        _native._collect_go()
        self.assertEqual(self.holders(), 0)

    def test_go_panics_remain_process_fatal(self) -> None:
        for expression in ["bridge.PtrInt.nil().value",
                           "bridge.unknown_value(bridge.int_as_any(1))"]:
            script = (
                "import resource, sys\n"
                "resource.setrlimit(resource.RLIMIT_CORE, (0, 0))\n"
                f"sys.path.insert(0, {str(package_dir)!r})\n"
                "import go_any as bridge\n"
                + expression + "\n"
            )
            result = subprocess.run([sys.executable, "-c", script], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("panic:", result.stderr)


if __name__ == "__main__":
    unittest.main()
