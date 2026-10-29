from pathlib import Path
import gc
import importlib
import sys
import unittest
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
        shared = bridge.echo(value=value)
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
    def test_error_is_an_ordinary_value(self) -> None:
        value, failure = bridge.result(fail=False)
        self.assertEqual(value, 7)
        self.assertTrue(failure.is_nil)
        value, failure = bridge.result(fail=True)
        self.assertEqual(value, 0)
        self.assertFalse(failure.is_nil)
        self.assertEqual(failure.error(), "ordinary error")
if __name__ == "__main__":
    unittest.main()
