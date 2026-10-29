package main

import (
	api "go-bridge/examples/09_nil"
	"testing"
)

func TestZeroHandleIsNil(t *testing.T) {
	if Get[*api.List](0) != nil || Get[*int](0) != nil {
		t.Fatal("handle 0 must read as nil")
	}
	if PinPointer[int](nil) != 0 {
		t.Fatal("a nil result must not be pinned")
	}
	UnPin(0) // Releasing nil is a no-op.
}

func TestNilReceiver(t *testing.T) {
	if BrListLen(0) != 0 {
		t.Fatal("nil receiver must run the method")
	}
}
