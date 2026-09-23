from pathlib import Path
import sys


example_dir = Path(__file__).resolve().parent
sys.path.insert(0, str(example_dir.parent / ".build" / example_dir.name / "python"))

import go_basic_value as basic


assert basic.int_in_and_out(1) == 2
assert basic.bool_in_and_out(True) is False
assert basic.basic_go_spec_type(41) == 42

print("01_basic: OK")
