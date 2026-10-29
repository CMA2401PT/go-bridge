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

    def test_go_producer_and_callback_thread(self) -> None:
        values: list[int] = []
        caller = threading.get_ident()

        def received(value: int) -> None:
            self.assertEqual(threading.get_ident(), caller)
            values.append(value)

        ch = bridge.numbers(count=4)
        while ch.recv(callback=received):
            pass
        self.assertEqual(values, [0, 1, 2, 3])
        self.assertFalse(ch.recv(callback=received))
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
        values: list[int] = []
        worker = threading.Thread(target=lambda: ch.recv(callback=values.append))
        worker.start()
        ch.send(value=7)
        worker.join(timeout=5)
        self.assertEqual(values, [7])
        self.assertFalse(worker.is_alive())

    def test_buffered_alias_and_nil(self) -> None:
        ch = bridge.ChanInt(capacity=2)
        alias = bridge.echo(ch=ch)
        alias.send(value=0)
        alias.send(value=9)
        ch.close()
        values: list[int] = []
        while alias.recv(callback=values.append):
            pass
        self.assertEqual(values, [0, 9])
        self.assertTrue(bridge.echo(ch=bridge.ChanInt.nil()).is_nil)
        self.assertTrue(bridge.ChanInt.nil().recv_only().is_nil)
        self.assertTrue(bridge.ChanInt.nil().send_only().is_nil)

    def test_callback_exception_consumes_only_one_value(self) -> None:
        ch = bridge.ChanString(capacity=2)
        ch.send(value="中文\0🙂")
        ch.send(value="next")
        ch.close()

        def fail(value: str) -> None:
            self.assertEqual(value, "中文\0🙂")
            raise ValueError("callback failed")

        with self.assertRaisesRegex(ValueError, "callback failed"):
            ch.recv(callback=fail)
        values: list[str] = []
        self.assertTrue(bridge.echo_text(ch=ch).recv(callback=values.append))
        self.assertEqual(values, ["next"])
        self.assertFalse(ch.recv(callback=values.append))

    def test_reentrant_callback(self) -> None:
        ch = bridge.ChanInt(capacity=1)
        ch.send(value=1)

        def received(value: int) -> None:
            ch.send(value=value + 1)

        self.assertTrue(ch.recv(callback=received))
        values: list[int] = []
        self.assertTrue(ch.recv(callback=values.append))
        self.assertEqual(values, [2])
        ch.close()


if __name__ == "__main__":
    unittest.main()
