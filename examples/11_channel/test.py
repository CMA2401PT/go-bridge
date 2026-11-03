from pathlib import Path
import gc
import importlib
import sys
import threading
import unittest

example_dir = Path(__file__).resolve().parent
package_dir = example_dir.parent / ".build" / example_dir.name / "python"
sys.path.insert(0, str(package_dir))

import go_channel as bridge

_native = importlib.import_module("go_channel._native")


class ChannelTests(unittest.TestCase):
    def setUp(self) -> None:
        gc.collect()
        self.baseline = int(_native._holder_count())

    def tearDown(self) -> None:
        gc.collect()
        self.assertEqual(int(_native._holder_count()), self.baseline)

    def test_go_producer(self) -> None:
        values: list[int] = []
        ch = bridge.numbers(count=4)
        while True:
            value, ok = ch.recv()
            if not ok:
                break
            values.append(value)
        self.assertEqual(values, [0, 1, 2, 3])
        self.assertEqual(ch.recv(), (0, False))
        self.assertFalse(hasattr(ch, "send"))
        self.assertFalse(hasattr(ch, "close"))

    def test_unbuffered_python_threads_and_go_consumer(self) -> None:
        ch = bridge.ChanInt()

        def produce() -> None:
            sender = ch.send_only()
            for value in [2, 3, 5]:
                bridge.send(ch=sender, value=value)
            sender.close()

        worker = threading.Thread(target=produce)
        worker.start()
        self.assertEqual(bridge.sum(ch=ch.recv_only()), 10)
        worker.join(timeout=5)
        self.assertFalse(worker.is_alive())

    def test_python_receive_releases_gil(self) -> None:
        ch = bridge.ChanInt()
        results: list[tuple[int, bool]] = []

        def receive() -> None:
            results.append(ch.recv())

        worker = threading.Thread(target=receive)
        worker.start()
        ch.send(value=7)
        worker.join(timeout=5)
        self.assertEqual(results, [(7, True)])
        self.assertFalse(worker.is_alive())

    def test_buffered_alias_and_nil(self) -> None:
        ch = bridge.ChanInt(capacity=2)
        alias = bridge.echo(ch=ch)
        alias.send(value=0)
        alias.send(value=9)
        ch.close()
        self.assertEqual(alias.recv(), (0, True))
        self.assertEqual(alias.recv(), (9, True))
        self.assertEqual(alias.recv(), (0, False))
        self.assertEqual(alias.recv(), (0, False))
        self.assertTrue(bridge.echo(ch=bridge.ChanInt.nil()).is_nil)
        self.assertTrue(bridge.ChanInt.nil().recv_only().is_nil)
        self.assertTrue(bridge.ChanInt.nil().send_only().is_nil)

    def test_string_transfer_and_closed_zero(self) -> None:
        ch = bridge.ChanString(capacity=2)
        ch.send(value="中文\0🙂")
        ch.send(value="")
        ch.close()
        self.assertEqual(bridge.echo_text(ch=ch).recv(), ("中文\0🙂", True))
        self.assertEqual(ch.recv(), ("", True))
        self.assertEqual(ch.recv(), ("", False))
        self.assertEqual(ch.recv(), ("", False))


if __name__ == "__main__":
    unittest.main()
