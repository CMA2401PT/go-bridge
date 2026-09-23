"""High-performance Python bindings for the 01_basic Go example."""

from ._native import basic_go_spec_type, bool_in_and_out, int_in_and_out

__all__ = ["basic_go_spec_type", "bool_in_and_out", "int_in_and_out"]
