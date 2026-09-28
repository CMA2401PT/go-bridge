from pathlib import Path
import gc
import importlib
import sys
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_ptr_basic as bridge
# Test-only diagnostics; deliberately absent from the public stubs.
_native = importlib.import_module("go_ptr_basic._native")

INT_MIN = -(2**63)
INT_MAX = 2**63 - 1
Named = bridge.MyNamedNonStructInt


def named_slice(values: list[int]) -> bridge.MyNamedNonStructSliceInt:
    return bridge.MyNamedNonStructSliceInt(bridge.SliceInt(values))


class Holders(unittest.TestCase):
    def setUp(self) -> None:
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def holders(self) -> int:
        gc.collect()
        return int(_native._holder_count()) - self.baseline


class PtrIntTests(Holders):
    def test_new_and_value(self) -> None:
        self.assertEqual(bridge.PtrInt().value, 0)  # Go's new(int)
        p = bridge.PtrInt(5)
        self.assertIsInstance(p, bridge.Ptr)
        self.assertIsInstance(p, bridge.Ref)
        self.assertIs(type(p.value), int)
        p.value = INT_MIN
        self.assertEqual(p.value, INT_MIN)
        # A named int is a Python int subclass and is stored as plain int, as
        # for any Go int argument since 06 (Go would need int(n)).
        p.value = Named(3)
        self.assertIs(type(p.value), int)
        for bad in [INT_MAX + 1, 1.5, "1", None]:
            with self.assertRaises(TypeError):
                p.value = bad  # type: ignore[assignment]
            with self.assertRaises(TypeError):
                bridge.PtrInt(bad)  # type: ignore[arg-type]
        self.assertEqual(p.value, 3)

    def test_each_constructor_allocates(self) -> None:
        x = 5
        p = bridge.PtrInt(x)
        q = bridge.PtrInt(x)
        p.value = 6
        self.assertEqual(q.value, 5)
        self.assertEqual(x, 5)  # A Python value has no Go address to take.
        alias = p
        alias.value = 7  # Python aliases are copies of the pointer.
        self.assertEqual(p.value, 7)

    def test_functions_write_through(self) -> None:
        p = bridge.PtrInt(INT_MAX)
        self.assertEqual(bridge.ptr_fn_echo_int(p), f"*int: {INT_MAX}")
        bridge.inc_int(p)  # Go int arithmetic wraps.
        self.assertEqual(p.value, INT_MIN)
        self.assertEqual(bridge.fn_echo_int(p.value), f"int: {INT_MIN}")

    def test_rejected_arguments(self) -> None:
        # No implicit boxing: a temporary pointer would lose Go's writes.
        for bad in [5, None, bridge.PtrMyNamedNonStructInt(Named(5))]:
            with self.assertRaises(TypeError):
                bridge.ptr_fn_echo_int(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                bridge.inc_int(bad)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.fn_echo_int(bridge.PtrInt(5))  # type: ignore[arg-type]
        del bad
        self.assertEqual(self.holders(), 0)


class PtrNamedIntTests(Holders):
    def test_new_and_value(self) -> None:
        zero = bridge.PtrMyNamedNonStructInt()
        self.assertEqual(zero.value, 0)
        self.assertIs(type(zero.value), Named)
        p = bridge.PtrMyNamedNonStructInt(Named(5))
        self.assertIsInstance(p, bridge.Ptr)
        self.assertNotIsInstance(p, bridge.PtrInt)  # *int and *Named are distinct types.
        p.value = Named(INT_MAX)
        self.assertEqual(p.value, INT_MAX)
        self.assertIs(type(p.value), Named)
        for bad in [5, True, None, 1.5]:
            with self.assertRaises(TypeError):
                p.value = bad  # type: ignore[assignment]
            with self.assertRaises(TypeError):
                bridge.PtrMyNamedNonStructInt(bad)  # type: ignore[arg-type]
        self.assertEqual(self.holders(), 2)  # A failed constructor pins nothing.

    def test_method_set(self) -> None:
        p = bridge.PtrMyNamedNonStructInt(Named(INT_MAX))
        # *T has the value-receiver methods of T as well as its own.
        self.assertEqual(p.echo_non_struct_int(), f"my non struct int: {INT_MAX}")
        self.assertEqual(p.ptr_echo_non_struct_int(), f"my non struct int: {INT_MAX}")
        self.assertIsNone(p.inc())  # type: ignore[func-returns-value]
        self.assertEqual(p.value, INT_MIN)
        # A value is not addressable: no pointer-receiver methods on it.
        self.assertFalse(hasattr(Named(1), "inc"))
        self.assertFalse(hasattr(Named(1), "ptr_echo_non_struct_int"))

    def test_value_is_a_copy(self) -> None:
        p = bridge.PtrMyNamedNonStructInt(Named(1))
        value = p.value
        p.inc()
        self.assertEqual(value, 1)  # Go's v := *p
        self.assertEqual(p.value, 2)

    def test_ptr_and_from_value(self) -> None:
        n = Named(1)
        p = n.ptr()  # Go's p := new(T); *p = n
        self.assertIs(type(p), bridge.PtrMyNamedNonStructInt)
        q = n.ptr()
        p.inc()
        self.assertEqual(p.value, 2)
        self.assertEqual(q.value, 1)  # Every call allocates a new variable.
        self.assertEqual(n, 1)
        r = bridge.PtrMyNamedNonStructInt.from_value(Named(3))
        self.assertIs(type(r), bridge.PtrMyNamedNonStructInt)
        self.assertEqual(r.value, 3)
        with self.assertRaises(TypeError):
            bridge.PtrMyNamedNonStructInt.from_value(3)  # type: ignore[arg-type]
        # No conversion between distinct pointer types.
        self.assertFalse(hasattr(p, "unname"))
        s = bridge.PtrInt.from_value(4)
        self.assertIs(type(s), bridge.PtrInt)
        self.assertEqual(s.value, 4)
        self.assertEqual(self.holders(), 4)

    def test_functions(self) -> None:
        p = bridge.PtrMyNamedNonStructInt(Named(-7))
        self.assertEqual(bridge.ptr_fn_echo_non_struct_int(p), "my non struct int: -7")
        self.assertEqual(bridge.fn_echo_non_struct_int(p.value), "my non struct int: -7")
        for bad in [Named(5), 5, None, bridge.PtrInt(5)]:
            with self.assertRaises(TypeError):
                bridge.ptr_fn_echo_non_struct_int(bad)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.fn_echo_non_struct_int(p)  # type: ignore[arg-type]

    def test_public_surface(self) -> None:
        p = bridge.PtrInt()
        with self.assertRaises(TypeError):
            bridge.Ptr()  # type: ignore[call-arg]
        for name in ["handle", "close", "__enter__", "__dict__"]:
            self.assertFalse(hasattr(p, name))
        with self.assertRaises(AttributeError):
            p.extra = 1  # type: ignore[attr-defined]
        self.assertIs(bridge.Ptr[int].__origin__, bridge.Ptr)  # type: ignore[attr-defined]


class PtrSliceTests(Holders):
    def test_ptr_holds_a_header(self) -> None:
        numbers = bridge.SliceInt([1, 2])
        p = numbers.ptr()  # Go's p := new([]int); *p = numbers
        self.assertIs(type(p), bridge.PtrSliceInt)
        self.assertIs(type(bridge.PtrSliceInt.from_value(numbers)), bridge.PtrSliceInt)
        self.assertIs(type(p.value), bridge.SliceInt)
        self.assertEqual(p.value.copy_to_py_list(), [1, 2])
        with self.assertRaises(TypeError):
            bridge.PtrSliceInt([1])  # type: ignore[arg-type]
        # A named slice has its own pointer type.
        with self.assertRaises(TypeError):
            bridge.PtrSliceInt(named_slice([1]))  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.PtrSliceInt()  # type: ignore[call-arg]

    def test_append_replaces_only_the_pointed_header(self) -> None:
        numbers = bridge.SliceInt([1])
        p = bridge.PtrSliceInt(numbers)
        before = p.value
        bridge.append_slice_int(p, 2)
        self.assertEqual(p.value.copy_to_py_list(), [1, 2])
        # Headers read earlier, and the original variable, keep their length.
        self.assertEqual(before.copy_to_py_list(), [1])
        self.assertEqual(numbers.copy_to_py_list(), [1])
        self.assertEqual(bridge.ptr_fn_echo_slice_int(p), "*slice int: &[1 2]")
        self.assertEqual(bridge.fn_echo_slice_int(p.value), "slice int: [1 2]")
        p.value = bridge.SliceInt([7])  # *p = s
        self.assertEqual(bridge.ptr_fn_echo_slice_int(p), "*slice int: &[7]")
        for bad in [[7], named_slice([7])]:
            with self.assertRaises(TypeError):
                p.value = bad  # type: ignore[assignment]
        with self.assertRaises(TypeError):
            p.value = None  # type: ignore[assignment]

    def test_named_slice_pointer(self) -> None:
        # Construction only pairs a pointer with its own value class.
        for bad in [bridge.SliceInt([1]), None]:
            with self.assertRaises(TypeError):
                bridge.PtrMyNamedNonStructSliceInt(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                bridge.PtrMyNamedNonStructSliceInt.from_value(bad)  # type: ignore[arg-type]
        p = bridge.PtrMyNamedNonStructSliceInt(named_slice([1]))
        self.assertIs(type(p.value), bridge.MyNamedNonStructSliceInt)
        self.assertNotIsInstance(p, bridge.PtrSliceInt)
        named = named_slice([1, 2])
        q = named.ptr()
        self.assertIs(type(q), bridge.PtrMyNamedNonStructSliceInt)
        q.append(3)
        self.assertEqual(q.value.copy_to_py_list(), [1, 2, 3])
        self.assertEqual(len(named), 2)
        self.assertEqual(q.echo_non_struct_slice_int(), "my non struct slice int: [1 2 3]")
        self.assertEqual(q.ptr_echo_non_struct_slice_int(), "my non struct slice int: &[1 2 3]")
        self.assertEqual(named.echo_non_struct_slice_int(), "my non struct slice int: [1 2]")
        self.assertFalse(hasattr(named, "append"))  # Pointer receiver only.
        self.assertEqual(bridge.ptr_fn_echo_non_struct_slice_int(q),
                         "my non struct slice int: &[1 2 3]")
        self.assertEqual(bridge.fn_echo_non_struct_slice_int(named_slice([4])),
                         "my non struct slice int: [4]")
        with self.assertRaises(TypeError):
            bridge.fn_echo_non_struct_slice_int(bridge.SliceInt([4]))  # type: ignore[arg-type]
        self.assertIs(type(bridge.PtrMyNamedNonStructSliceInt.from_value(named)),
                      bridge.PtrMyNamedNonStructSliceInt)
        with self.assertRaises(TypeError):  # Stricter than Go: convert first.
            q.value = bridge.SliceInt([9])  # type: ignore[assignment]
        q.value = bridge.MyNamedNonStructSliceInt(bridge.SliceInt([9]))
        self.assertIs(type(q.value), bridge.MyNamedNonStructSliceInt)
        self.assertEqual(q.value.copy_to_py_list(), [9])

    def test_rejected_arguments(self) -> None:
        plain = bridge.SliceInt([1]).ptr()
        named = named_slice([1]).ptr()
        for bad in [named, bridge.SliceInt([1]), None]:
            with self.assertRaises(TypeError):
                bridge.ptr_fn_echo_slice_int(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                bridge.append_slice_int(bad, 1)  # type: ignore[arg-type]
        for wrong in [plain, named_slice([1]), None]:
            with self.assertRaises(TypeError):
                bridge.ptr_fn_echo_non_struct_slice_int(wrong)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.append_slice_int(plain, 1.5)  # type: ignore[arg-type]
        del bad, wrong
        self.assertEqual(self.holders(), 2)

    def test_variable_outlives_its_source(self) -> None:
        numbers = bridge.SliceInt([1])
        p = numbers.ptr()
        del numbers
        _native._collect_go()
        bridge.append_slice_int(p, 2)
        self.assertEqual(p.value.copy_to_py_list(), [1, 2])
        with self.assertRaises(TypeError):
            for _ in p.value:  # type: ignore[misc]
                pass

if __name__ == "__main__":
    suite = unittest.TestSuite()
    for case in [PtrIntTests, PtrNamedIntTests, PtrSliceTests]:
        suite.addTests(unittest.defaultTestLoader.loadTestsFromTestCase(case))
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("07_ptr_basic: OK")
