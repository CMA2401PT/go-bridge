from pathlib import Path
import copy
import gc
import importlib
import sys
import unittest
example_dir = Path(__file__).resolve().parent
package_dir = example_dir.parent / ".build" / example_dir.name / "python"
sys.path.insert(0, str(package_dir))
import go_array as bridge
_native = importlib.import_module("go_array._native")
class ArrayTests(unittest.TestCase):
    def setUp(self) -> None:
        gc.collect()
        self.baseline = int(_native._holder_count())
    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)
    def test_values_and_address(self) -> None:
        value = bridge.Array3Int(values=[1, 2, 3])
        copied = copy.copy(value)
        echoed = bridge.echo(value=value)
        increased = bridge.increment(value=value)
        bridge.mutate(value=value.addr())
        self.assertEqual(value.copy_to_py_list(), [2, 2, 3])
        self.assertEqual(copied[0], 1)
        self.assertEqual(echoed[0], 1)
        self.assertEqual(increased[0], 2)
        value[-1] = 8
        self.assertEqual(value[2], 8)
        self.assertEqual(len(value), 3)
    def test_zero_and_length(self) -> None:
        self.assertEqual(bridge.Array3Int().copy_to_py_list(), [0, 0, 0])
        with self.assertRaises(ValueError):
            bridge.Array3Int(values=[1, 2])
        with self.assertRaises(IndexError):
            _ = bridge.Array3Int()[3]
if __name__ == "__main__":
    unittest.main()
