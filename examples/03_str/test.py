from pathlib import Path
import sys
import unittest


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_str as bridge


class StringTests(unittest.TestCase):
    def test_strings(self):
        for value in ["", "hello", "中文 🐍", "a\0b"]:
            self.assertEqual(bridge.recv_and_return(value), "Echo: " + value)
        with self.assertRaises(UnicodeEncodeError):
            bridge.recv_and_return("\ud800")
        with self.assertRaises(TypeError):
            bridge.recv_and_return(b"not str")


if __name__ == "__main__":
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(StringTests)
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful():
        raise SystemExit(1)
    print("03_str: OK")
