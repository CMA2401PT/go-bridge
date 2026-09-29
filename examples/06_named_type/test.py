from pathlib import Path
import copy
import gc
import importlib
import sys
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_named_type as bridge
# Test-only diagnostics; deliberately absent from the public stubs.
_native = importlib.import_module("go_named_type._native")

INT_MIN = -(2**63)
INT_MAX = 2**63 - 1


def full_struct() -> bridge.MyNamedStruct:
    return bridge.MyNamedStruct(
        my_named_anomy_sub_struct=bridge.MyNamedAnomySubStruct(basic_member_value_2=2),
        basic_member_value=1,
        ref_member_value=bridge.SliceInt([3, 4]),
        named_basic_member_value=bridge.MyNamedNonStructInt(5),
        named_ref_member_value=bridge.MyNamedSubStruct(basic_member_value=6),
    )


def echo(value: str) -> str:
    return "MyNamedStruct non Ptr: " + value


class NamedIntTests(unittest.TestCase):
    def test_is_an_int(self) -> None:
        value = bridge.MyNamedNonStructInt(41)
        self.assertIsInstance(value, int)
        self.assertEqual(value, 41)
        self.assertEqual(hash(value), hash(41))
        self.assertEqual(bridge.MyNamedNonStructInt(), 0)  # Go zero value
        # int operations return plain int, as a Go constant expression would
        # need an explicit conversion back to the named type.
        self.assertIs(type(value + 1), int)
        self.assertIs(type(value.unname()), int)
        self.assertEqual(value.unname(), 41)
        with self.assertRaises(AttributeError):
            value.extra = 1  # type: ignore[attr-defined]

    def test_methods_and_functions(self) -> None:
        for number in [0, -7, INT_MIN, INT_MAX]:
            value = bridge.MyNamedNonStructInt(number)
            expected = f"my non struct int: {number}"
            self.assertEqual(value.echo_non_struct_int(), expected)
            self.assertEqual(bridge.fn_echo_non_struct_int(value), expected)

    def test_range_and_types(self) -> None:
        self.assertEqual(bridge.MyNamedNonStructInt(True), 1)
        for bad in [INT_MAX + 1, INT_MIN - 1, 1.5, "1", None]:
            with self.assertRaises(TypeError):
                bridge.MyNamedNonStructInt(bad)  # type: ignore[arg-type]
        # The name is part of the Go type: a plain int is not accepted.
        for bad in [5, True, None]:
            with self.assertRaises(TypeError):
                bridge.fn_echo_non_struct_int(bad)  # type: ignore[arg-type]


class Holders(unittest.TestCase):
    def setUp(self) -> None:
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def holders(self) -> int:
        gc.collect()
        return int(_native._holder_count()) - self.baseline


class NamedSliceTests(Holders):
    def test_is_a_distinct_slice(self) -> None:
        values = bridge.MyNamedNonStructSliceInt(bridge.SliceInt([1, 2, 3]))
        self.assertIsInstance(values, bridge.Slice)
        self.assertNotIsInstance(values, bridge.SliceInt)
        self.assertEqual(len(values), 3)
        self.assertEqual(values.copy_to_py_list(), [1, 2, 3])
        values[-1] = 30
        self.assertEqual(values[2], 30)
        with self.assertRaises(TypeError):
            for _ in values:  # type: ignore[misc]
                pass

    def test_conversions_share_the_array(self) -> None:
        plain = bridge.SliceInt([1, 2])
        values = bridge.MyNamedNonStructSliceInt(plain)  # Go's MyNamed(s)
        values[0] = 10
        self.assertEqual(plain[0], 10)
        back = values.unname()  # Go's []int(si)
        self.assertIs(type(back), bridge.SliceInt)
        back[1] = 20
        self.assertEqual(values[1], 20)
        del plain, values
        _native._collect_go()
        self.assertEqual(back.copy_to_py_list(), [10, 20])
        # Construction is a conversion, not a copy from Python values.
        for bad in [[1], range(2), None, bridge.MyNamedNonStructSliceInt(bridge.SliceInt([]))]:
            with self.assertRaises(TypeError):
                bridge.MyNamedNonStructSliceInt(bad)  # type: ignore[arg-type]

    def test_operations_keep_the_name(self) -> None:
        values = bridge.MyNamedNonStructSliceInt(bridge.SliceInt([1, 2, 3]))
        tail = values[1:]
        self.assertIs(type(tail), bridge.MyNamedNonStructSliceInt)
        self.assertIs(type(values.clone()), bridge.MyNamedNonStructSliceInt)
        tail[0] = 20  # Reslices share the backing array, as in Go.
        self.assertEqual(values[1], 20)
        with self.assertRaises(ValueError):
            values[::2]

    def test_methods_and_functions(self) -> None:
        values = bridge.MyNamedNonStructSliceInt(bridge.SliceInt([1, -2]))
        self.assertEqual(values.echo_non_struct_slice_int(), "my non struct slice int: [1 -2]")
        self.assertEqual(bridge.fn_echo_non_struct_slice_int(values), "my non struct slice int: [1 -2]")
        self.assertFalse(hasattr(bridge.SliceInt([]), "echo_non_struct_slice_int"))
        # Stricter than Go, which assigns []int to the named type implicitly.
        for bad in [bridge.SliceInt([]), [1], None]:
            with self.assertRaises(TypeError):
                bridge.fn_echo_non_struct_slice_int(bad)  # type: ignore[arg-type]


class StructTests(Holders):
    def test_constructor_and_fields(self) -> None:
        value = full_struct()
        self.assertEqual(self.holders(), 1)  # Arguments were copied or shared into Go.
        self.assertEqual(value.basic_member_value, 1)
        self.assertEqual(value.basic_member_value_2, 2)
        self.assertEqual(value.my_named_anomy_sub_struct.basic_member_value_2, 2)
        self.assertEqual(value.ref_member_value.copy_to_py_list(), [3, 4])
        self.assertEqual(value.named_basic_member_value, 5)
        self.assertIs(type(value.named_basic_member_value), bridge.MyNamedNonStructInt)
        self.assertEqual(value.named_ref_member_value.basic_member_value, 6)
        self.assertEqual(value.echo_non_ptr(), echo("{{2} 1 [3 4] 5 {6}}"))

    def test_zero_values(self) -> None:
        self.assertEqual(bridge.MyNamedStruct().echo_ptr(), echo("{{0} 0 [] 0 {0}}"))
        self.assertEqual(bridge.MyNamedSubStruct().basic_member_value, 0)
        self.assertEqual(bridge.MyNamedAnomySubStruct().basic_member_value_2, 0)
        self.assertEqual(len(bridge.MyNamedStruct().ref_member_value), 0)
        partial = bridge.MyNamedStruct(basic_member_value=INT_MAX)
        self.assertEqual(partial.echo_ptr(), echo(f"{{{{0}} {INT_MAX} [] 0 {{0}}}}"))

    def test_constructor_copies_structs_and_shares_slices(self) -> None:
        sub = bridge.MyNamedSubStruct(basic_member_value=6)
        numbers = bridge.SliceInt([3])
        value = bridge.MyNamedStruct(ref_member_value=numbers, named_ref_member_value=sub)
        sub.basic_member_value = 60
        numbers[0] = 30
        self.assertEqual(value.named_ref_member_value.basic_member_value, 6)
        self.assertEqual(value.ref_member_value[0], 30)
        del sub, numbers
        _native._collect_go()
        self.assertEqual(value.echo_ptr(), echo("{{0} 0 [30] 0 {6}}"))

    def test_scalar_field_assignment(self) -> None:
        value = full_struct()
        value.basic_member_value = INT_MIN
        value.basic_member_value_2 = 20  # Promoted field writes the embedded struct.
        value.named_basic_member_value = bridge.MyNamedNonStructInt(50)
        self.assertEqual(value.my_named_anomy_sub_struct.basic_member_value_2, 20)
        self.assertEqual(value.echo_ptr(), echo(f"{{{{20}} {INT_MIN} [3 4] 50 {{6}}}}"))
        for bad in [1.5, None, INT_MAX + 1]:
            with self.assertRaises(TypeError):
                value.basic_member_value = bad  # type: ignore[assignment]
        with self.assertRaises(TypeError):
            value.named_basic_member_value = 50  # type: ignore[assignment]

    def test_struct_field_is_a_view(self) -> None:
        value = full_struct()
        sub = value.named_ref_member_value
        embedded = value.my_named_anomy_sub_struct
        sub.basic_member_value = 60  # Go's p := &s.Field; p.X = ...
        embedded.basic_member_value_2 = 20
        self.assertEqual(value.basic_member_value_2, 20)
        self.assertEqual(value.echo_ptr(), echo("{{20} 1 [3 4] 5 {60}}"))
        value.basic_member_value_2 = 21
        self.assertEqual(embedded.basic_member_value_2, 21)
        # A view keeps the whole parent reachable in Go.
        del value
        _native._collect_go()
        self.assertEqual(sub.basic_member_value, 60)
        self.assertEqual(embedded.basic_member_value_2, 21)

    def test_struct_field_assignment_copies(self) -> None:
        value = full_struct()
        view = value.named_ref_member_value
        replacement = bridge.MyNamedSubStruct(basic_member_value=7)
        value.named_ref_member_value = replacement  # Go's s.Field = v
        replacement.basic_member_value = 70
        self.assertEqual(value.named_ref_member_value.basic_member_value, 7)
        self.assertEqual(view.basic_member_value, 7)  # The old view sees the new value.
        value.my_named_anomy_sub_struct = bridge.MyNamedAnomySubStruct(basic_member_value_2=8)
        self.assertEqual(value.basic_member_value_2, 8)
        for bad in [None, bridge.MyNamedAnomySubStruct()]:
            with self.assertRaises(TypeError):
                value.named_ref_member_value = bad  # type: ignore[assignment]

    def test_slice_field_shares_array(self) -> None:
        value = full_struct()
        numbers = value.ref_member_value
        numbers[0] = 30
        self.assertEqual(value.ref_member_value[0], 30)
        replacement = bridge.SliceInt([9])
        value.ref_member_value = replacement
        replacement[0] = 90
        self.assertIs(type(value.ref_member_value), bridge.SliceInt)
        self.assertEqual(value.echo_ptr(), echo("{{2} 1 [90] 5 {6}}"))
        self.assertEqual(numbers.copy_to_py_list(), [30, 4])
        named = bridge.MyNamedNonStructSliceInt(bridge.SliceInt([1]))
        for bad in [[1], named]:  # A named slice needs unname() first.
            with self.assertRaises(TypeError):
                value.ref_member_value = bad  # type: ignore[assignment]
        value.ref_member_value = named.unname()
        self.assertEqual(value.ref_member_value.copy_to_py_list(), [1])

    def test_copy_is_shallow(self) -> None:
        value = full_struct()
        copied = copy.copy(value)
        self.assertIs(type(copied), bridge.MyNamedStruct)
        copied.basic_member_value = 10
        copied.named_ref_member_value.basic_member_value = 60
        self.assertEqual(value.echo_ptr(), echo("{{2} 1 [3 4] 5 {6}}"))
        copied.ref_member_value[0] = 30  # Like Go's v := *p, slice arrays stay shared.
        self.assertEqual(value.ref_member_value[0], 30)
        sub = bridge.MyNamedSubStruct(basic_member_value=1)
        sub_copy = copy.copy(sub)
        sub_copy.basic_member_value = 2
        self.assertEqual(sub.basic_member_value, 1)
        self.assertEqual(copy.copy(bridge.MyNamedAnomySubStruct(basic_member_value_2=3))
                         .basic_member_value_2, 3)

    def test_aliases_share_one_struct(self) -> None:
        value = full_struct()
        alias = value
        alias.basic_member_value = 100
        self.assertEqual(value.basic_member_value, 100)

    def test_receivers(self) -> None:
        value = full_struct()
        self.assertEqual(value.echo_ptr(), value.echo_non_ptr())
        self.assertIsNone(value.inc_basic_ptr())  # type: ignore[func-returns-value]
        self.assertEqual(value.basic_member_value, 2)
        result = value.inc_basic_non_ptr()
        self.assertIsNot(result, value)
        self.assertEqual(value.basic_member_value, 2)
        self.assertEqual(result.basic_member_value, 3)
        result.named_ref_member_value.basic_member_value = 60
        self.assertEqual(value.named_ref_member_value.basic_member_value, 6)
        largest = bridge.MyNamedStruct(basic_member_value=INT_MAX)
        largest.inc_basic_ptr()  # Go int arithmetic wraps.
        self.assertEqual(largest.basic_member_value, INT_MIN)

    def test_functions(self) -> None:
        value = full_struct()
        expected = echo("{{2} 1 [3 4] 5 {6}}")
        self.assertEqual(bridge.fn_echo_non_ptr(value), expected)
        self.assertEqual(bridge.ptr_fn_echo_ptr(value.addr()), expected)
        for bad in [None, bridge.MyNamedSubStruct(), {"basic_member_value": 1}]:
            with self.assertRaises(TypeError):
                bridge.fn_echo_non_ptr(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                bridge.ptr_fn_echo_ptr(bad)  # type: ignore[arg-type]
        # T and *T are distinct types: no implicit &v or *p.
        with self.assertRaises(TypeError):
            bridge.ptr_fn_echo_ptr(value)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.fn_echo_non_ptr(value.addr())  # type: ignore[arg-type]

    def test_rejected_constructor_arguments(self) -> None:
        with self.assertRaises(TypeError):
            bridge.MyNamedSubStruct(1)  # type: ignore[misc]
        with self.assertRaises(TypeError):
            bridge.MyNamedSubStruct(basic_member_value=1.5)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.MyNamedStruct(named_basic_member_value=5)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.MyNamedStruct(ref_member_value=[1])  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.MyNamedStruct(named_ref_member_value=bridge.MyNamedAnomySubStruct())  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.MyNamedStruct(unknown=1)  # type: ignore[call-arg]
        self.assertEqual(self.holders(), 0)  # A failed constructor pins nothing.

    def test_public_surface(self) -> None:
        value = bridge.MyNamedStruct()
        for cls in [bridge.MyNamedStruct, bridge.MyNamedSubStruct, bridge.MyNamedAnomySubStruct]:
            self.assertTrue(issubclass(cls, bridge.Ref))
            self.assertFalse(issubclass(cls, bridge.Ptr))
        with self.assertRaises(TypeError):
            bridge.Ref()  # type: ignore[call-arg]
        for name in ["handle", "close", "__enter__", "__dict__"]:
            self.assertFalse(hasattr(value, name))
        with self.assertRaises(AttributeError):
            value.extra = 1  # type: ignore[attr-defined]


class StructPtrTests(Holders):
    def test_addr_shares_the_variable(self) -> None:
        value = full_struct()
        p = value.addr()  # Go's p := &v
        self.assertIs(type(p), bridge.PtrMyNamedStruct)
        self.assertIsInstance(p, bridge.Ptr)
        self.assertNotIsInstance(p, bridge.MyNamedStruct)
        p.basic_member_value = 10  # Go's p.X = 10
        self.assertEqual(value.basic_member_value, 10)
        p.inc_basic_ptr()
        self.assertEqual(value.basic_member_value, 11)
        value.named_ref_member_value.basic_member_value = 60
        self.assertEqual(p.named_ref_member_value.basic_member_value, 60)
        self.assertEqual(p.echo_non_ptr(), value.echo_ptr())
        # Each addr() is a new Ref to the same variable.
        self.assertIsNot(value.addr(), p)
        del value
        _native._collect_go()
        self.assertEqual(p.basic_member_value, 11)

    def test_from_value_copies(self) -> None:
        value = bridge.MyNamedStruct(basic_member_value=1)
        for p in [bridge.PtrMyNamedStruct(value), bridge.PtrMyNamedStruct.from_value(value)]:
            p.basic_member_value = 2  # Go's p := new(T); *p = v
            self.assertEqual(value.basic_member_value, 1)
        sub = bridge.PtrMyNamedSubStruct(bridge.MyNamedSubStruct(basic_member_value=3))
        self.assertEqual(sub.basic_member_value, 3)
        anomy = bridge.PtrMyNamedAnomySubStruct.from_value(bridge.MyNamedAnomySubStruct())
        self.assertEqual(anomy.basic_member_value_2, 0)

    def test_value_is_the_variable(self) -> None:
        value = full_struct()
        p = value.addr()
        target = p.value  # Go's *p is addressable: a view, not a copy.
        self.assertIs(type(target), bridge.MyNamedStruct)
        target.basic_member_value = 7
        self.assertEqual(value.basic_member_value, 7)
        p.value = bridge.MyNamedStruct(basic_member_value=8)  # Go's *p = v copies in.
        self.assertEqual(value.basic_member_value, 8)
        self.assertEqual(target.basic_member_value, 8)
        self.assertEqual(value.echo_ptr(), echo("{{0} 8 [] 0 {0}}"))

    def test_value_receiver_through_pointer(self) -> None:
        p = full_struct().addr()
        result = p.inc_basic_non_ptr()  # (*p).M(): a copy.
        self.assertIs(type(result), bridge.MyNamedStruct)
        self.assertEqual((p.basic_member_value, result.basic_member_value), (1, 2))

    def test_strict(self) -> None:
        value = full_struct()
        p = value.addr()
        sub = bridge.MyNamedSubStruct()
        for bad in [None, p, sub, sub.addr()]:
            with self.assertRaises(TypeError):
                bridge.PtrMyNamedStruct(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                p.value = bad  # type: ignore[assignment]
        with self.assertRaises(TypeError):
            bridge.PtrMyNamedStruct()  # type: ignore[call-arg]
        # Fields keep their own types: a pointer is not a struct value.
        with self.assertRaises(TypeError):
            value.named_ref_member_value = sub.addr()  # type: ignore[assignment]
        with self.assertRaises(TypeError):
            bridge.MyNamedStruct(named_ref_member_value=sub.addr())  # type: ignore[arg-type]
        for name in ["addr", "__copy__"]:
            self.assertFalse(hasattr(p, name))
        self.assertFalse(hasattr(value, "value"))


if __name__ == "__main__":
    suite = unittest.TestSuite()
    for case in [NamedIntTests, NamedSliceTests, StructTests, StructPtrTests]:
        suite.addTests(unittest.defaultTestLoader.loadTestsFromTestCase(case))
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("06_named_type: OK")
