"""Go named types: a named int, a named slice and structs."""

from types import GenericAlias

from ._native import (
    MyNamedAnomySubStruct,
    MyNamedNonStructInt,
    MyNamedNonStructSliceInt,
    MyNamedStruct,
    MyNamedSubStruct,
    Ptr,
    PtrMyNamedAnomySubStruct,
    PtrMyNamedStruct,
    PtrMyNamedSubStruct,
    Ref,
    Slice,
    SliceInt,
    fn_echo_non_ptr,
    fn_echo_non_struct_int,
    fn_echo_non_struct_slice_int,
    ptr_fn_echo_ptr,
)

# Slice and Ptr are generic in __init__.pyi; allow the same subscript at runtime.
Slice.__class_getitem__ = classmethod(GenericAlias)
Ptr.__class_getitem__ = classmethod(GenericAlias)

__all__ = [
    "MyNamedAnomySubStruct",
    "MyNamedNonStructInt",
    "MyNamedNonStructSliceInt",
    "MyNamedStruct",
    "MyNamedSubStruct",
    "Ptr",
    "PtrMyNamedAnomySubStruct",
    "PtrMyNamedStruct",
    "PtrMyNamedSubStruct",
    "Ref",
    "Slice",
    "SliceInt",
    "fn_echo_non_ptr",
    "fn_echo_non_struct_int",
    "fn_echo_non_struct_slice_int",
    "ptr_fn_echo_ptr",
]
