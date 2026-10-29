from pathlib import Path
import gc
import importlib
import sys
import unittest
example_dir = Path(__file__).resolve().parent
package_dir = example_dir.parent / ".build" / example_dir.name / "python"
sys.path.insert(0, str(package_dir))
import go_func as bridge
_native = importlib.import_module("go_func._native")
class FunctionTests(unittest.TestCase):
    def setUp(self) -> None:
        gc.collect()
        self.baseline = int(_native._holder_count())
    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)
    def test_shared_closure(self) -> None:
        value = bridge.counter(start=10)
        shared = bridge.echo(value=value)
        self.assertEqual(value(arg1=2), 12)
        self.assertEqual(bridge.apply(value=shared, delta=3), 15)
        del value
        gc.collect()
        self.assertEqual(shared(-1), 14)
    def test_nil(self) -> None:
        self.assertTrue(bridge.nil().is_nil)
        self.assertTrue(bridge.echo(value=bridge.FuncIntToInt.nil()).is_nil)
        self.assertFalse(bridge.counter(start=0).is_nil)
if __name__ == "__main__":
    unittest.main()
