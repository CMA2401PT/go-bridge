from pathlib import Path
import gc
import sys
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_bytes_and_holder as bridge
from go_bytes_and_holder import _native


class HolderTests(unittest.TestCase):
    def setUp(self):
        self.baseline = _native._holder_count()

    def tearDown(self):
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def test_bytes_and_go_gc(self):
        for value in [b"", b"hello", b"a\0b\xff", bytes(range(256))]:
            source = bridge.SliceBytes(value)
            self.assertEqual(len(source), len(value))
            self.assertEqual(source.copy_to_py_bytes(), value)
            result = bridge.recv_and_return_bytes(source)
            self.assertEqual(_native._holder_count(), self.baseline + 2)
            del source
            gc.collect()
            self.assertEqual(_native._holder_count(), self.baseline + 1)
            _native._collect_go()
            self.assertEqual(len(result), len(value) + 6)
            copied = result.copy_to_py_bytes()
            self.assertEqual(copied, b"Echo: " + value)
            del result
            gc.collect()
            self.assertEqual(_native._holder_count(), self.baseline)
            self.assertEqual(copied, b"Echo: " + value)

    def test_destructor_and_alias(self):
        for _ in range(20):
            source = bridge.SliceBytes(b"retained")
            alias = source
            del source
            gc.collect()
            self.assertEqual(_native._holder_count(), self.baseline + 1)
            self.assertEqual(alias.copy_to_py_bytes(), b"retained")
            del alias
            gc.collect()
            self.assertEqual(_native._holder_count(), self.baseline)

    def test_minimal_public_surface(self):
        source = bridge.SliceBytes(b"x")
        self.assertIsInstance(source, bridge.Ref)
        self.assertTrue(issubclass(bridge.SliceBytes, bridge.Ref))
        with self.assertRaises(TypeError):
            bridge.Ref()
        for name in ["index", "closed", "close", "__enter__", "__exit__"]:
            self.assertFalse(hasattr(source, name))
        del source

    def test_ref_lifetime_on_exception(self):
        def use_ref():
            reference: bridge.Ref = bridge.SliceBytes(b"temporary")
            self.assertIsInstance(reference, bridge.Ref)
            raise RuntimeError("body")

        with self.assertRaisesRegex(RuntimeError, "body"):
            use_ref()
        gc.collect()
        self.assertEqual(_native._holder_count(), self.baseline)

    def test_rejected_types(self):
        for value in ["text", bytearray(b"data"), memoryview(b"data"), None]:
            with self.assertRaises(TypeError):
                bridge.SliceBytes(value)
        with self.assertRaises(TypeError):
            bridge.recv_and_return_bytes(b"create a SliceBytes first")


if __name__ == "__main__":
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(HolderTests)
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("04_bytes_and_holder: OK")
