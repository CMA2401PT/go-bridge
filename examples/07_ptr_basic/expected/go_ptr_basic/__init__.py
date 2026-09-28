"""Go pointers to value types: int, a named int, []int and a named slice."""

from types import GenericAlias

from ._native import (
    MyNamedNonStructInt,
    MyNamedNonStructSliceInt,
    Ptr,
    PtrInt,
    PtrMyNamedNonStructInt,
    PtrMyNamedNonStructSliceInt,
    PtrSliceInt,
    Ref,
    Slice,
    SliceInt,
    append_slice_int,
    fn_echo_int,
    fn_echo_non_struct_int,
    fn_echo_non_struct_slice_int,
    fn_echo_slice_int,
    inc_int,
    ptr_fn_echo_int,
    ptr_fn_echo_non_struct_int,
    ptr_fn_echo_non_struct_slice_int,
    ptr_fn_echo_slice_int,
)

# Ptr and Slice are generic in __init__.pyi; allow the same subscript at runtime.
Ptr.__class_getitem__ = classmethod(GenericAlias)
Slice.__class_getitem__ = classmethod(GenericAlias)

__all__ = [
    "MyNamedNonStructInt",
    "MyNamedNonStructSliceInt",
    "Ptr",
    "PtrInt",
    "PtrMyNamedNonStructInt",
    "PtrMyNamedNonStructSliceInt",
    "PtrSliceInt",
    "Ref",
    "Slice",
    "SliceInt",
    "append_slice_int",
    "fn_echo_int",
    "fn_echo_non_struct_int",
    "fn_echo_non_struct_slice_int",
    "fn_echo_slice_int",
    "inc_int",
    "ptr_fn_echo_int",
    "ptr_fn_echo_non_struct_int",
    "ptr_fn_echo_non_struct_slice_int",
    "ptr_fn_echo_slice_int",
]
