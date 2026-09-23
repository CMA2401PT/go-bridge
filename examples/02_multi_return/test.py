from pathlib import Path
import sys


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_multi_return as multi


two = multi.return_two(10)
assert isinstance(two, tuple)
assert two == (11, 12)
first, second = two
assert first == 11 and second == 12

four = multi.return_four(value=-2)
assert isinstance(four, tuple)
assert four == (-1, 0, 1, 2)
assert all(type(value) is int for value in two + four)

print("02_multi_return: OK")
