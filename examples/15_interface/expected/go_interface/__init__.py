"""用显式接口包装和类型断言保留 Go 的接口语义。"""

from ._native import (
    Ref,
    Ptr,
    GoInterface,
    GoAny,
    Animal,
    Dog,
    PtrDog,
    Cat,
    PtrCat,
    error,
    adopt,
    introduce,
    nobody,
    stray,
    feed,
)
