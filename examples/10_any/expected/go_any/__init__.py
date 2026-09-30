"""Explicit Go any values, boxing and dynamic type assertions."""

from types import GenericAlias
from . import _native
from ._native import (
    GoAny,
    Ref,
    Slice,
    Ptr,
    SliceInt,
    MySlice,
    Point,
    PtrInt,
    PtrPoint,
    PtrPtrInt,
    IntPtr,
    any_as_int,
    try_any_as_int,
    any_as_int8,
    try_any_as_int8,
    any_as_int16,
    try_any_as_int16,
    any_as_int32,
    try_any_as_int32,
    any_as_int64,
    try_any_as_int64,
    any_as_uint,
    try_any_as_uint,
    any_as_uint8,
    try_any_as_uint8,
    any_as_uint16,
    try_any_as_uint16,
    any_as_uint32,
    try_any_as_uint32,
    any_as_uint64,
    try_any_as_uint64,
    any_as_uintptr,
    try_any_as_uintptr,
    any_as_float32,
    try_any_as_float32,
    any_as_float64,
    try_any_as_float64,
    any_as_bool,
    try_any_as_bool,
    any_as_string,
    try_any_as_string,
    int_as_any,
    int8_as_any,
    int16_as_any,
    int32_as_any,
    int64_as_any,
    uint_as_any,
    uint8_as_any,
    uint16_as_any,
    uint32_as_any,
    uint64_as_any,
    uintptr_as_any,
    float32_as_any,
    float64_as_any,
    bool_as_any,
    string_as_any,
    echo,
    dynamic_kind,
    nil_any,
    nil_pointer,
    nil_slice,
    empty_slice,
    unknown,
    slice_is_nil,
    invalid_string,
    unknown_value,
    echo_my_slice,
    echo_int_ptr,
    echo_point,
    echo_double_pointer,
)

Slice.__class_getitem__ = classmethod(GenericAlias)
Ptr.__class_getitem__ = classmethod(GenericAlias)


class MyInt(int):
    __slots__ = ()

    def __new__(cls, value: int = 0) -> "MyInt":
        return int.__new__(cls, _native._check_int(value))

    def __init_subclass__(cls, **kwargs: object) -> None:
        raise TypeError("MyInt cannot be subclassed")

    def unname(self) -> int:
        if type(self) is not MyInt:
            raise TypeError("expected MyInt")
        return int(self)

    def as_any(self) -> GoAny:
        if type(self) is not MyInt:
            raise TypeError("expected MyInt")
        return _native._my_int_as_any(self)

    @classmethod
    def from_any(cls, value: GoAny) -> "MyInt":
        return cls(_native._any_as_my_int(value))

    @classmethod
    def try_from_any(cls, value: GoAny) -> "MyInt | None":
        result = _native._try_any_as_my_int(value)
        return None if result is None else cls(result)


def echo_my_int(value: MyInt) -> MyInt:
    if type(value) is not MyInt:
        raise TypeError("expected MyInt")
    return MyInt(_native._echo_my_int(value))


__all__ = [
    "GoAny",
    "Ref",
    "Slice",
    "Ptr",
    "SliceInt",
    "MySlice",
    "Point",
    "PtrInt",
    "PtrPoint",
    "PtrPtrInt",
    "IntPtr",
    "any_as_int",
    "try_any_as_int",
    "any_as_int8",
    "try_any_as_int8",
    "any_as_int16",
    "try_any_as_int16",
    "any_as_int32",
    "try_any_as_int32",
    "any_as_int64",
    "try_any_as_int64",
    "any_as_uint",
    "try_any_as_uint",
    "any_as_uint8",
    "try_any_as_uint8",
    "any_as_uint16",
    "try_any_as_uint16",
    "any_as_uint32",
    "try_any_as_uint32",
    "any_as_uint64",
    "try_any_as_uint64",
    "any_as_uintptr",
    "try_any_as_uintptr",
    "any_as_float32",
    "try_any_as_float32",
    "any_as_float64",
    "try_any_as_float64",
    "any_as_bool",
    "try_any_as_bool",
    "any_as_string",
    "try_any_as_string",
    "int_as_any",
    "int8_as_any",
    "int16_as_any",
    "int32_as_any",
    "int64_as_any",
    "uint_as_any",
    "uint8_as_any",
    "uint16_as_any",
    "uint32_as_any",
    "uint64_as_any",
    "uintptr_as_any",
    "float32_as_any",
    "float64_as_any",
    "bool_as_any",
    "string_as_any",
    "echo",
    "dynamic_kind",
    "nil_any",
    "nil_pointer",
    "nil_slice",
    "empty_slice",
    "unknown",
    "slice_is_nil",
    "invalid_string",
    "unknown_value",
    "echo_my_slice",
    "echo_int_ptr",
    "echo_point",
    "echo_double_pointer",
    "MyInt",
    "echo_my_int",
]
