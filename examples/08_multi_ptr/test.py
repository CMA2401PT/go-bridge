from pathlib import Path
import gc
import importlib
import sys
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_multi_ptr as bridge
# Test-only diagnostics; deliberately absent from the public stubs.
_native = importlib.import_module("go_multi_ptr._native")

INT_MAX = 2**63 - 1
INT_MIN = -(2**63)


class Holders(unittest.TestCase):
    def setUp(self) -> None:
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def holders(self) -> int:
        gc.collect()
        return int(_native._holder_count()) - self.baseline


class ClosureTests(unittest.TestCase):
    def test_every_dependent_type_exists(self) -> None:
        # G only names ***int, IntPtr and Box; the rest come from the closure.
        for name in ["PtrPtrPtrInt", "PtrPtrInt", "PtrInt", "IntPtr", "Box", "Inner",
                     "Counter", "PtrPtrBox"]:
            self.assertTrue(hasattr(bridge, name), name)
        # Built from basic values upwards, one level at a time.
        ppp = bridge.PtrPtrPtrInt(bridge.PtrPtrInt(bridge.PtrInt(1)))
        self.assertIs(type(ppp.value), bridge.PtrPtrInt)
        self.assertIs(type(ppp.value.value), bridge.PtrInt)
        self.assertEqual(ppp.value.value.value, 1)


class MultiPtrTests(Holders):
    def test_levels_share_the_target(self) -> None:
        p = bridge.PtrInt(1)
        pp = p.ptr()  # Go's pp := new(*int); *pp = p
        ppp = pp.ptr()
        self.assertIs(type(pp), bridge.PtrPtrInt)
        self.assertIs(type(ppp), bridge.PtrPtrPtrInt)
        bridge.inc_ppp(ppp)  # ***p++
        self.assertEqual(p.value, 2)
        self.assertEqual(bridge.echo_ppp(ppp), "***int: 2")
        ppp.value.value.value = 5
        self.assertEqual(p.value, 5)
        self.assertEqual(bridge.PtrPtrInt.from_value(p).value.value, 5)
        self.assertIs(type(bridge.PtrPtrPtrInt.from_value(pp)), bridge.PtrPtrPtrInt)

    def test_value_is_a_new_wrapper(self) -> None:
        pp = bridge.PtrPtrInt(bridge.PtrInt(1))
        first = pp.value
        second = pp.value
        self.assertIsNot(first, second)  # Two Refs for the same Go variable.
        first.value = 2
        self.assertEqual(second.value, 2)
        self.assertEqual(self.holders(), 3)

    def test_retarget_changes_only_one_level(self) -> None:
        a = bridge.PtrInt(1)
        b = bridge.PtrInt(2)
        pp = a.ptr()
        ppp = pp.ptr()
        old = pp.value
        bridge.retarget(pp, b)  # *pp = b
        self.assertEqual(pp.value.value, 2)
        self.assertEqual(ppp.value.value.value, 2)  # ppp still points at pp.
        self.assertEqual(old.value, 1)  # A pointer read earlier is unchanged.
        bridge.inc_ppp(ppp)
        self.assertEqual((a.value, b.value), (1, 3))
        pp.value = a  # Python's *pp = a
        self.assertEqual(ppp.value.value.value, 1)
        ppp.value = bridge.PtrPtrInt(b)  # Replaces *ppp; pp is untouched.
        self.assertEqual(pp.value.value, 1)
        self.assertEqual(bridge.echo_ppp(ppp), "***int: 3")

    def test_copy_is_a_new_variable(self) -> None:
        p = bridge.PtrInt(1)
        pp = p.ptr()
        pp2 = p.ptr()
        bridge.retarget(pp, bridge.PtrInt(9))
        self.assertEqual(pp2.value.value, 1)  # Each ptr() allocates.

    def test_targets_outlive_their_wrappers(self) -> None:
        ppp = bridge.PtrInt(7).ptr().ptr()
        _native._collect_go()
        bridge.inc_ppp(ppp)
        self.assertEqual(ppp.value.value.value, 8)

    def test_nil_level(self) -> None:
        pp = bridge.PtrPtrInt(bridge.PtrInt(1))
        ppp = pp.ptr()
        bridge.clear(pp)  # Go stores nil; Python cannot build one.
        with self.assertRaises(ValueError):
            pp.value
        self.assertIs(type(ppp.value), bridge.PtrPtrInt)  # Only the cleared level.
        pp.value = bridge.PtrInt(4)
        self.assertEqual(bridge.echo_ppp(ppp), "***int: 4")

    def test_strict_levels(self) -> None:
        p = bridge.PtrInt(1)
        pp = p.ptr()
        with self.assertRaises(TypeError):
            bridge.PtrPtrInt()  # type: ignore[call-arg]
        for bad in [1, None, pp, bridge.IntPtr(p)]:
            with self.assertRaises(TypeError):
                bridge.PtrPtrInt(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                pp.value = bad  # type: ignore[assignment]
        for wrong in [p, pp.ptr().value.value, None]:
            with self.assertRaises(TypeError):
                bridge.inc_ppp(wrong)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.PtrPtrPtrInt(p)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.retarget(pp, 1)  # type: ignore[arg-type]
        # Go dereferences only once for methods: no methods on **T.
        self.assertFalse(hasattr(pp, "inc"))
        self.assertFalse(hasattr(bridge.PtrPtrPtrInt(pp), "ptr"))  # ****int never appears.
        del bad, wrong
        self.assertEqual(self.holders(), 2)


class NamedPtrTests(Holders):
    def test_conversions_share_the_variable(self) -> None:
        p = bridge.PtrInt(1)
        named = bridge.IntPtr(p)  # Go's IntPtr(p)
        self.assertIsInstance(named, bridge.Ptr)
        self.assertNotIsInstance(named, bridge.PtrInt)
        bridge.inc_int_ptr(named)
        self.assertEqual(p.value, 2)
        named.value = INT_MAX
        bridge.inc_int_ptr(named)  # Go int arithmetic wraps.
        self.assertEqual(p.value, INT_MIN)
        back = named.unname()  # Go's (*int)(named)
        self.assertIs(type(back), bridge.PtrInt)
        back.value = 3
        self.assertEqual(named.value, 3)
        del p, back
        _native._collect_go()
        self.assertEqual(named.value, 3)

    def test_strict(self) -> None:
        p = bridge.PtrInt(1)
        for bad in [1, None, p.ptr(), bridge.IntPtr(p)]:
            with self.assertRaises(TypeError):
                bridge.IntPtr(bad)  # type: ignore[arg-type]
        for wrong in [p, None]:
            with self.assertRaises(TypeError):
                bridge.inc_int_ptr(wrong)  # type: ignore[arg-type]
        named = bridge.IntPtr(p)
        with self.assertRaises(TypeError):
            bridge.retarget(p.ptr(), named)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            named.value = 1.5  # type: ignore[assignment]
        for name in ["from_value", "ptr"]:
            self.assertFalse(hasattr(named, name))


class StructPtrTests(Holders):
    def test_field_types_are_generated(self) -> None:
        box = bridge.Box(count=bridge.Counter(1), inner=bridge.Inner(n=2))
        self.assertIs(type(box.count), bridge.Counter)
        self.assertEqual(box.count.echo(), "counter: 1")
        self.assertIs(type(box.inner), bridge.Inner)
        self.assertEqual(box.echo(), "box: {1 {2}}")
        self.assertEqual(bridge.Box().echo(), "box: {0 {0}}")
        with self.assertRaises(TypeError):
            bridge.Box(count=1)  # type: ignore[arg-type]

    def test_pointer_to_struct_pointer(self) -> None:
        box = bridge.Box()
        pp = box.ptr()  # **Box holding this *Box: the struct is shared.
        self.assertIs(type(pp), bridge.PtrPtrBox)
        bridge.inc_box_pp(pp)
        self.assertEqual(box.echo(), "box: {1 {1}}")
        view = pp.value
        self.assertIs(type(view), bridge.Box)
        view.inner.n = 10  # Same struct through every level.
        self.assertEqual(box.inner.n, 10)
        other = bridge.Box(count=bridge.Counter(5))
        pp.value = other  # *pp = other: the pointer is copied, not the struct.
        bridge.inc_box_pp(pp)
        self.assertEqual(other.echo(), "box: {6 {1}}")
        self.assertEqual(box.echo(), "box: {1 {10}}")
        self.assertIs(type(bridge.PtrPtrBox.from_value(box)), bridge.PtrPtrBox)
        self.assertFalse(hasattr(pp, "inc"))  # Empty method set.
        for bad in [None, bridge.Inner(), bridge.PtrInt()]:
            with self.assertRaises(TypeError):
                bridge.PtrPtrBox(bad)  # type: ignore[arg-type]
            with self.assertRaises(TypeError):
                bridge.inc_box_pp(bad)  # type: ignore[arg-type]
        with self.assertRaises(TypeError):
            bridge.inc_box_pp(box)  # type: ignore[arg-type]


if __name__ == "__main__":
    suite = unittest.TestSuite()
    for case in [ClosureTests, MultiPtrTests, NamedPtrTests, StructPtrTests]:
        suite.addTests(unittest.defaultTestLoader.loadTestsFromTestCase(case))
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("08_multi_ptr: OK")
