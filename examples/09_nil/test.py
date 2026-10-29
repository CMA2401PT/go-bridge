from pathlib import Path
import gc
import importlib
import sys
import subprocess
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_nil as bridge
# Test-only diagnostics; deliberately absent from the public stubs.
_native = importlib.import_module("go_nil._native")


class Holders(unittest.TestCase):
    def setUp(self) -> None:
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def holders(self) -> int:
        gc.collect()
        return int(_native._holder_count()) - self.baseline


def build(*values: int) -> bridge.PtrList:
    head = bridge.PtrList.nil()
    for value in reversed(values):
        head = head.push(value)
    return head


class NilPtrTests(Holders):
    def test_nil_is_a_typed_wrapper(self) -> None:
        nil = bridge.PtrInt.nil()  # Go's var p *int
        self.assertIs(type(nil), bridge.PtrInt)
        self.assertTrue(nil.is_nil)
        self.assertFalse(bridge.PtrInt(0).is_nil)  # A pointer to 0 is not nil.
        self.assertEqual(self.holders(), 0)  # nil pins nothing.
        with self.assertRaises(AttributeError):
            nil.is_nil = False  # type: ignore[misc]

    def test_no_truth_value(self) -> None:
        for value in [bridge.PtrInt.nil(), bridge.PtrInt(1)]:
            with self.assertRaises(TypeError):
                bool(value)
            with self.assertRaises(TypeError):
                if value:
                    pass

    def test_nil_argument_and_result(self) -> None:
        self.assertEqual(bridge.deref(bridge.PtrInt.nil()), "nil")
        self.assertEqual(bridge.deref(bridge.PtrInt(7)), "7")
        self.assertTrue(bridge.maybe_int(False).is_nil)
        present = bridge.maybe_int(True)
        self.assertEqual(present.value, 42)
        self.assertEqual(self.holders(), 1)
        for bad in [None, 7]:
            with self.assertRaises(TypeError):
                bridge.deref(bad)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.maybe_int(1)  # type: ignore[arg-type]


class NilListTests(Holders):
    def test_nil_receiver(self) -> None:
        empty = bridge.PtrList.nil()
        self.assertEqual(empty.len(), 0)  # (*List)(nil).Len() is valid Go.
        one = empty.push(1)
        self.assertIs(type(one), bridge.PtrList)
        self.assertFalse(one.is_nil)
        self.assertEqual(one.len(), 1)
        self.assertEqual(build(1, 2, 3).len(), 3)

    def test_value_is_never_nil(self) -> None:
        value = bridge.List(value_=1)
        self.assertFalse(hasattr(bridge.List, "nil"))
        self.assertFalse(hasattr(value, "is_nil"))
        self.assertEqual((value.len(), value.first(), value.head()), (1, 1, 1))
        p = value.addr()  # Go's &v
        self.assertFalse(p.is_nil)
        p.value_ = 2
        self.assertEqual(value.value_, 2)
        target = p.value  # *p is the same variable.
        self.assertIs(type(target), bridge.List)
        target.value_ = 3
        self.assertEqual(value.value_, 3)
        p.value = bridge.List(value_=4)  # *p = v copies in.
        self.assertEqual(value.value_, 4)
        copied = bridge.PtrList(value)  # new(List) holding a copy
        copied.value_ = 5
        self.assertEqual(value.value_, 4)
        for bad in [None, p, bridge.PtrList.nil()]:
            with self.assertRaises(TypeError):
                bridge.PtrList(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                p.value = bad  # type: ignore[assignment]

    def test_pointer_field(self) -> None:
        tail = bridge.List(value_=2)
        self.assertTrue(tail.next.is_nil)  # The zero value of a pointer field.
        head = bridge.List(value_=1, next=tail.addr())
        self.assertEqual(head.len(), 2)
        head.next.value_ = 20  # The field holds a pointer: tail is shared.
        self.assertEqual(tail.value_, 20)
        head.next = bridge.PtrList.nil()
        self.assertEqual(head.len(), 1)
        head.next = head.addr()  # A cycle lives in Go; no Python reference cycle.
        self.assertIs(type(head.next.next), bridge.PtrList)
        head.next = bridge.PtrList.nil()
        for bad in [None, bridge.PtrInt.nil(), tail]:
            with self.assertRaises(TypeError):
                head.next = bad  # type: ignore[assignment]
            with self.assertRaises(TypeError):
                bridge.List(next=bad)  # type: ignore[arg-type]

    def test_find_returns_nil(self) -> None:
        values = build(1, 2, 3)
        found = bridge.find(values, 2)
        self.assertEqual(found.value_, 2)
        self.assertEqual(found.len(), 2)  # Shares the rest of the list.
        self.assertTrue(bridge.find(values, 9).is_nil)
        self.assertTrue(bridge.find(bridge.PtrList.nil(), 1).is_nil)
        for bad in [None, bridge.List()]:
            with self.assertRaises(TypeError):
                bridge.find(bad, 1)  # type: ignore[arg-type]

    def test_nodes_stay_reachable(self) -> None:
        values = build(1, 2)
        rest = values.next
        del values
        _native._collect_go()
        self.assertEqual(rest.value_, 2)
        self.assertEqual(self.holders(), 1)


class PanicTests(unittest.TestCase):
    def test_go_panic_terminates_process(self) -> None:
        setup = (
            "import sys\n"
            f"sys.path.insert(0, {str(example_dir.parent / '.build' / example_dir.name / 'python')!r})\n"
            "import go_nil as bridge\n"
        )
        for call in [
            "bridge.must_positive(-3)",
            "bridge.PtrInt.nil().value",
            "bridge.PtrInt.nil().value = 1",
            "bridge.PtrList.nil().first()",
            "bridge.PtrList.nil().head()",
            "bridge.PtrList.nil().value_",
            "bridge.PtrList.nil().value",
            "bridge.PtrList.nil().next = bridge.PtrList.nil()",
            "bridge.PtrList.nil().value = bridge.List()",
        ]:
            with self.subTest(call=call):
                result = subprocess.run(
                    args=[sys.executable, "-c", setup + call],
                    capture_output=True,
                    text=True,
                    timeout=10,
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("panic:", result.stderr)


if __name__ == "__main__":
    suite = unittest.TestSuite()
    for case in [NilPtrTests, NilListTests, PanicTests]:
        suite.addTests(unittest.defaultTestLoader.loadTestsFromTestCase(case))
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("09_nil: OK")
