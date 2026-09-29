"""Go multi-level and named pointers, with the types they need."""

from types import GenericAlias

from ._native import (
    Box,
    Counter,
    Inner,
    IntPtr,
    Ptr,
    PtrBox,
    PtrInner,
    PtrInt,
    PtrPtrBox,
    PtrPtrInt,
    PtrPtrPtrInt,
    Ref,
    clear,
    echo_ppp,
    inc_box_pp,
    inc_int_ptr,
    inc_ppp,
    retarget,
)

# Ptr is generic in __init__.pyi; allow the same subscript at runtime.
Ptr.__class_getitem__ = classmethod(GenericAlias)

__all__ = [
    "Box",
    "Counter",
    "Inner",
    "IntPtr",
    "Ptr",
    "PtrBox",
    "PtrInner",
    "PtrInt",
    "PtrPtrBox",
    "PtrPtrInt",
    "PtrPtrPtrInt",
    "Ref",
    "clear",
    "echo_ppp",
    "inc_box_pp",
    "inc_int_ptr",
    "inc_ppp",
    "retarget",
]
