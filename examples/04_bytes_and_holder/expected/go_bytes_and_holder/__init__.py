"""References to Go-managed objects and Go byte slices."""

from ._native import Ref, SliceBytes, recv_and_return_bytes

__all__ = ["Ref", "SliceBytes", "recv_and_return_bytes"]
