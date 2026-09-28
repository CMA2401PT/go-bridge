"""Go-owned slices of values and of Go-owned slices."""

from types import GenericAlias

from ._native import Ref, Slice, SliceInt, SliceSliceInt, concat_a, concat_b

# Slice is generic in __init__.pyi; allow the same subscript at runtime.
Slice.__class_getitem__ = classmethod(GenericAlias)

__all__ = ["Ref", "Slice", "SliceInt", "SliceSliceInt", "concat_a", "concat_b"]
