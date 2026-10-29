from pathlib import Path
import gc
import importlib
import sys
import unittest
example_dir = Path(__file__).resolve().parent
package_dir = example_dir.parent / ".build" / example_dir.name / "python"
sys.path.insert(0, str(package_dir))
import go_map as bridge
_native = importlib.import_module("go_map._native")

class MapTests(unittest.TestCase):
    def setUp(self) -> None:
        gc.collect()
        self.baseline = int(_native._holder_count())
    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)
    def test_shared_and_cloned(self) -> None:
        value = bridge.MapStringInt(items=[("中文\0", 1), ("x", 0), ("x", 2)])
        shared = bridge.echo(value=value)
        clone = value.clone()
        bridge.put(value=shared, key="x", item=3)
        self.assertEqual(value["x"], 3)
        self.assertEqual(clone["x"], 2)
        self.assertEqual(dict(value.copy_to_py_items()), {"中文\0": 1, "x": 3})
        shared.delete(key="x")
        shared.delete(key="missing")
        self.assertFalse("x" in value)
        with self.assertRaises(KeyError):
            _ = value["x"]
        value.clear()
        self.assertEqual(len(shared), 0)
    def test_nil_and_empty(self) -> None:
        value = bridge.nil()
        self.assertTrue(value.is_nil)
        self.assertTrue(value.clone().is_nil)
        self.assertEqual(value.copy_to_py_items(), [])
        self.assertFalse(bridge.MapStringInt().is_nil)
        value.delete(key="missing")
        value.clear()
        with self.assertRaises(KeyError):
            _ = value["missing"]
    def test_input_copies(self) -> None:
        source = {"a": 1}
        value = bridge.MapStringInt(items=source.items())
        source["a"] = 2
        self.assertEqual(value["a"], 1)
        value["a"] = 3
        self.assertEqual(source["a"], 2)

if __name__ == "__main__":
    unittest.main()
