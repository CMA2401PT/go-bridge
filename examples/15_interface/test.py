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
_native = importlib.import_module("go_interface._native")
class InterfaceTests(unittest.TestCase):
    def setUp(self) -> None:
        gc.collect()
        self.baseline = int(_native._holder_count())
    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)
    def test_hidden_implementation(self) -> None:
        value = bridge.new(start=10)
        shared = bridge.Counter(value=bridge.echo(value=value))
        assert_type(shared, bridge.Counter)
        self.assertIsInstance(shared, bridge.GoInterface)
        self.assertEqual(value.add(arg1=2), 12)
        del value
        gc.collect()
        self.assertEqual(shared.add(arg1=3), 15)
    def test_nil(self) -> None:
        self.assertTrue(bridge.nil().is_nil)
        self.assertTrue(bridge.echo(value=bridge.Counter.nil()).is_nil)
        value = bridge.typed_nil()
        self.assertFalse(value.is_nil)
        self.assertEqual(value.add(arg1=1), -1)
    def test_interface_conversions(self) -> None:
        value = bridge.new(start=1)
        boxed = value.as_any()
        self.assertIsInstance(boxed, bridge.GoAny)
        self.assertEqual(bridge.Counter.from_any(value=boxed).add(arg1=2), 3)
        self.assertEqual(bridge.Counter.from_any(value=value).add(arg1=1), 4)
        self.assertIsNone(bridge.error.try_from_any(value=value))
        with self.assertRaises(TypeError):
            bridge.error.from_any(value=value)
        nil = bridge.Counter(value=bridge.Counter.nil())
        self.assertTrue(nil.is_nil)
        self.assertTrue(nil.as_any().is_nil)
        self.assertIsNone(bridge.Counter.try_from_any(value=nil))
        typed = bridge.Counter.from_any(value=bridge.typed_nil())
        self.assertFalse(typed.is_nil)
        self.assertEqual(typed.add(arg1=1), -1)
        self.assertTrue(bridge.GoAny.nil().as_any().is_nil)
    def test_error_is_an_ordinary_value(self) -> None:
        value, failure = bridge.result(fail=False)
        self.assertEqual(value, 7)
        self.assertTrue(failure.is_nil)
        value, failure = bridge.result(fail=True)
        self.assertEqual(value, 0)
        self.assertFalse(failure.is_nil)
        self.assertEqual(failure.error(), "ordinary error")
        self.assertEqual(bridge.error.from_any(value=failure).error(), "ordinary error")
if __name__ == "__main__":
    unittest.main()
