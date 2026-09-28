from pathlib import Path
from collections.abc import Iterator
import gc
import importlib
import sys
import typing
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_slice as bridge
# Test-only diagnostics; deliberately absent from the public stubs.
_native = importlib.import_module("go_slice._native")

INT_MIN = -(2**63)
INT_MAX = 2**63 - 1


def nested(value: bridge.SliceSliceInt) -> list[list[int]]:
    return [element.copy_to_py_list() for element in value.copy_to_py_list()]


class SliceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def holders(self) -> int:
        gc.collect()
        return int(_native._holder_count()) - self.baseline

    def test_slice_int_round_trip(self) -> None:
        for values in [[], [0], [1, -2, 3], [INT_MIN, INT_MAX], list(range(1000))]:
            source = bridge.SliceInt(values)
            self.assertEqual(len(source), len(values))
            self.assertEqual(source.copy_to_py_list(), values)
            del source
        self.assertEqual(bridge.SliceInt(x * x for x in range(4)).copy_to_py_list(), [0, 1, 4, 9])
        self.assertEqual(bridge.SliceInt((True, 5)).copy_to_py_list(), [1, 5])

    def test_concat_a(self) -> None:
        a = bridge.SliceInt([1, 2])
        b = bridge.SliceInt([3])
        result = bridge.concat_a(a, b)
        self.assertEqual(self.holders(), 3)
        del a, b
        _native._collect_go()
        self.assertEqual(self.holders(), 1)
        self.assertEqual(result.copy_to_py_list(), [1, 2, 3])
        empty = bridge.SliceInt([])
        self.assertEqual(bridge.concat_a(empty, empty).copy_to_py_list(), [])
        self.assertEqual(bridge.concat_a(result, empty).copy_to_py_list(), [1, 2, 3])

    def test_append_keeps_go_semantics(self) -> None:
        # A reslice of an exact-capacity slice has spare capacity, as in Go.
        storage = bridge.SliceInt([1, 2, 3, 4, 0])
        base = storage[:4]
        first = bridge.concat_a(base, bridge.SliceInt([5]))
        second = bridge.concat_a(base, bridge.SliceInt([6]))
        # Both appends write storage[4]; Go reports first == [1, 2, 3, 4, 6] too.
        self.assertEqual(first.copy_to_py_list(), [1, 2, 3, 4, 6])
        self.assertEqual(second.copy_to_py_list(), [1, 2, 3, 4, 6])
        self.assertEqual(storage.copy_to_py_list(), [1, 2, 3, 4, 6])
        # clone() is the Go way to avoid that sharing.
        independent = bridge.concat_a(base.clone(), bridge.SliceInt([7]))
        self.assertEqual(independent.copy_to_py_list(), [1, 2, 3, 4, 7])
        self.assertEqual(storage.copy_to_py_list(), [1, 2, 3, 4, 6])

    def test_slice_int_index_and_assignment(self) -> None:
        values = bridge.SliceInt([10, 20, 30])
        self.assertEqual([values[0], values[2], values[-1], values[-3]], [10, 30, 30, 10])
        values[1] = INT_MIN
        values[-1] = True
        self.assertEqual(values.copy_to_py_list(), [10, INT_MIN, 1])
        for index in [3, -4, INT_MAX]:
            with self.assertRaises(IndexError):
                values[index]
            with self.assertRaises(IndexError):
                values[index] = 0
        for bad in [1.5, "1", None, INT_MAX + 1]:
            with self.assertRaises(TypeError):
                values[0] = bad  # type: ignore[assignment]
        with self.assertRaises(TypeError):
            values["0"]  # type: ignore[call-overload]
        self.assertEqual(values.copy_to_py_list(), [10, INT_MIN, 1])

    def test_reslice_shares_array(self) -> None:
        values = bridge.SliceInt(range(6))
        middle = values[1:4]
        self.assertIs(type(middle), bridge.SliceInt)
        self.assertEqual(middle.copy_to_py_list(), [1, 2, 3])
        middle[0] = 100
        self.assertEqual(values[1], 100)
        # Bounds are clamped like a list slice and never reach spare capacity.
        self.assertEqual(values[-2:].copy_to_py_list(), [4, 5])
        self.assertEqual(values[4:100].copy_to_py_list(), [4, 5])
        self.assertEqual(len(values[5:2]), 0)
        self.assertEqual(len(values[:0]), 0)
        self.assertEqual(values[:].copy_to_py_list(), [0, 100, 2, 3, 4, 5])
        for step in [2, -1, 0]:
            with self.assertRaises(ValueError):
                values[::step]
        del values
        _native._collect_go()
        # The reslice keeps the shared array reachable on its own.
        self.assertEqual(middle.copy_to_py_list(), [100, 2, 3])

    def test_clone_is_independent(self) -> None:
        values = bridge.SliceInt([1, 2, 3])
        copied = values.clone()
        self.assertIs(type(copied), bridge.SliceInt)
        copied[0] = 9
        values[2] = 8
        self.assertEqual(values.copy_to_py_list(), [1, 2, 8])
        self.assertEqual(copied.copy_to_py_list(), [9, 2, 3])
        self.assertEqual(bridge.SliceInt([]).clone().copy_to_py_list(), [])

    def test_not_iterable(self) -> None:
        values = bridge.SliceInt([1])
        outer = bridge.SliceSliceInt([values])
        for value in [values, outer]:
            with self.assertRaises(TypeError):
                iter(value)  # type: ignore[call-overload]
            with self.assertRaises(TypeError):
                1 in value  # type: ignore[operator]

    def test_slice_slice_int_holds_elements(self) -> None:
        inner = [bridge.SliceInt([1, 2]), bridge.SliceInt([]), bridge.SliceInt([3])]
        outer = bridge.SliceSliceInt(inner)
        self.assertEqual(len(outer), 3)
        self.assertEqual(self.holders(), 4)
        del inner
        _native._collect_go()
        self.assertEqual(self.holders(), 1)
        elements = outer.copy_to_py_list()
        self.assertEqual(self.holders(), 4)
        self.assertTrue(all(type(element) is bridge.SliceInt for element in elements))
        self.assertEqual([element.copy_to_py_list() for element in elements], [[1, 2], [], [3]])
        del outer
        _native._collect_go()
        self.assertEqual(elements[0].copy_to_py_list(), [1, 2])
        del elements

    def test_slice_slice_int_from_temporaries(self) -> None:
        outer = bridge.SliceSliceInt(bridge.SliceInt([i] * i) for i in range(50))
        self.assertEqual(self.holders(), 1)
        _native._collect_go()
        self.assertEqual(nested(outer), [[i] * i for i in range(50)])
        self.assertEqual(nested(bridge.SliceSliceInt([])), [])

    def test_same_element_twice(self) -> None:
        inner = bridge.SliceInt([7])
        outer = bridge.SliceSliceInt([inner, inner])
        self.assertEqual(nested(outer), [[7], [7]])

    def test_slice_slice_int_elements(self) -> None:
        a = bridge.SliceInt([1, 2])
        outer = bridge.SliceSliceInt([a, bridge.SliceInt([3])])
        element = outer[0]
        self.assertIs(type(element), bridge.SliceInt)
        self.assertEqual(self.holders(), 3)  # a, outer and element
        # An element read shares its array with the original SliceInt.
        element[0] = 100
        self.assertEqual(a[0], 100)
        self.assertEqual(outer[-1].copy_to_py_list(), [3])
        # Assignment stores the element header; the outer slice keeps it alive.
        outer[1] = bridge.SliceInt([7, 8])
        _native._collect_go()
        self.assertEqual(nested(outer), [[100, 2], [7, 8]])
        for index in [2, -3]:
            with self.assertRaises(IndexError):
                outer[index]
            with self.assertRaises(IndexError):
                outer[index] = a
        for bad in [[1], None, outer]:
            with self.assertRaises(TypeError):
                outer[0] = bad  # type: ignore[assignment]
        self.assertEqual(nested(outer), [[100, 2], [7, 8]])

    def test_slice_slice_int_reslice_and_clone(self) -> None:
        outer = bridge.SliceSliceInt(bridge.SliceInt([i]) for i in range(4))
        tail = outer[2:]
        self.assertIs(type(tail), bridge.SliceSliceInt)
        self.assertEqual(nested(tail), [[2], [3]])
        tail[0] = bridge.SliceInt([20])
        self.assertEqual(outer[2].copy_to_py_list(), [20])
        # clone() is shallow like slices.Clone: element arrays stay shared.
        copied = outer.clone()
        copied[0] = bridge.SliceInt([-1])
        self.assertEqual(outer[0].copy_to_py_list(), [0])
        copied[1][0] = 10
        self.assertEqual(outer[1].copy_to_py_list(), [10])
        with self.assertRaises(ValueError):
            outer[::2]

    def test_concat_b(self) -> None:
        a = bridge.SliceSliceInt([bridge.SliceInt([1]), bridge.SliceInt([2, 3])])
        b = bridge.SliceSliceInt([bridge.SliceInt([])])
        result = bridge.concat_b(a, b)
        del a, b
        _native._collect_go()
        self.assertEqual(self.holders(), 1)
        self.assertEqual(nested(result), [[1], [2, 3], []])
        again = bridge.concat_b(result, result)
        self.assertEqual(nested(again), [[1], [2, 3], [], [1], [2, 3], []])

    def test_class_hierarchy(self) -> None:
        values = bridge.SliceInt([1])
        self.assertIsInstance(values, bridge.Slice)
        self.assertIsInstance(values, bridge.Ref)
        self.assertTrue(issubclass(bridge.SliceSliceInt, bridge.Slice))
        self.assertIs(typing.get_origin(bridge.Slice[int]), bridge.Slice)
        for cls in [bridge.Ref, bridge.Slice]:
            with self.assertRaises(TypeError):
                cls()
        for name in ["index", "close", "__enter__", "append", "cap"]:
            self.assertFalse(hasattr(values, name))

    def test_rejected_elements(self) -> None:
        for values in [[1.0], ["1"], [None], [INT_MAX + 1], [INT_MIN - 1], [[1]]]:
            with self.assertRaises(TypeError):
                bridge.SliceInt(values)  # type: ignore[arg-type]
        for bad in [None, 5, bridge.SliceInt([1])]:
            with self.assertRaises(TypeError):
                bridge.SliceInt(bad)  # type: ignore[arg-type]
        for elements in [[[1]], [1], [bridge.SliceSliceInt([])], [None]]:
            with self.assertRaises(TypeError):
                bridge.SliceSliceInt(elements)  # type: ignore[arg-type]

    def test_rejected_arguments(self) -> None:
        values = bridge.SliceInt([1])
        outer = bridge.SliceSliceInt([values])
        with self.assertRaises(TypeError):
            bridge.concat_a([1], values)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.concat_a(values, outer)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.concat_b(outer, values)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.concat_a(None, values)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.concat_b(outer, None)  # type: ignore[arg-type]

    def test_iterable_failure_releases(self) -> None:
        def failing() -> Iterator[bridge.SliceInt]:
            yield bridge.SliceInt([1])
            yield bridge.SliceInt([2])
            raise RuntimeError("iteration")

        with self.assertRaisesRegex(RuntimeError, "iteration"):
            bridge.SliceSliceInt(failing())
        with self.assertRaisesRegex(TypeError, "SliceInt"):
            bridge.SliceSliceInt([bridge.SliceInt([1]), "x"])  # type: ignore[list-item]


if __name__ == "__main__":
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(SliceTests)
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("05_slice: OK")
