package main

import (
	api "go-bridge/examples/10_any"
	"runtime/cgo"
	"testing"
)

func TestNilInterfaceAndTypedNil(t *testing.T) {
	baseline := bridgeRoots.Load()
	if bridgePinAny(nil) != 0 || bridgeAny(0) != nil {
		t.Fatal("nil interface must use handle 0")
	}
	BrRelease(0)
	if bridgeRoots.Load() != baseline {
		t.Fatal("nil interface created ownership")
	}
	var pointer *int
	handle := bridgePinAny(pointer)
	defer BrRelease(handle)
	if handle == 0 || bridgeAny(handle) == nil {
		t.Fatal("typed nil pointer lost its dynamic type")
	}
	result := BrTryAnyAsPtrInt(handle)
	if result.ok != 1 || result.value != 0 {
		t.Fatal("typed nil pointer assertion must succeed with handle 0")
	}
	if wrong := BrTryAnyAsPtrPoint(handle); wrong.ok != 0 || wrong.value != 0 {
		t.Fatal("wrong pointer type asserted successfully")
	}
}

func TestNamedDynamicTypesSurviveNormalizedCells(t *testing.T) {
	slice := bridgePin([]int{1})
	defer BrRelease(slice)
	boxedSlice := BrMySliceAsAny(slice)
	defer BrRelease(boxedSlice)
	if _, ok := bridgeAny(boxedSlice).(api.MySlice); !ok {
		t.Fatal("named slice was boxed as []int")
	}
	if result := BrTryAnyAsSliceInt(boxedSlice); result.ok != 0 {
		BrRelease(result.value)
		t.Fatal("named slice matched its underlying type")
	}
	restored := BrTryAnyAsMySlice(boxedSlice)
	defer BrRelease(restored.value)
	if restored.ok != 1 {
		t.Fatal("named slice assertion failed")
	}
	if _, ok := cgo.Handle(restored.value).Value().([]int); !ok {
		t.Fatal("named slice output was not normalized into its Cell")
	}
	bridgeGet[[]int](restored.value)[0] = 9
	if bridgeGet[[]int](slice)[0] != 9 {
		t.Fatal("slice assertion copied the backing array")
	}

	pointer := BrNewPtrInt(1)
	defer BrRelease(pointer)
	boxedPointer := BrIntPtrAsAny(pointer)
	defer BrRelease(boxedPointer)
	if _, ok := bridgeAny(boxedPointer).(api.IntPtr); !ok {
		t.Fatal("named pointer was boxed as *int")
	}
	if result := BrTryAnyAsPtrInt(boxedPointer); result.ok != 0 {
		BrRelease(result.value)
		t.Fatal("named pointer matched its underlying type")
	}
}

func TestStructAssertionsCopyValues(t *testing.T) {
	point := BrNewPoint(1, 0, 0)
	defer BrRelease(point)
	boxed := BrPointAsAny(point)
	defer BrRelease(boxed)
	BrPointSetX(point, 2)
	value, ok := bridgeAny(boxed).(api.Point)
	if !ok || value.X != 1 {
		t.Fatal("struct boxing did not snapshot the value")
	}
	first := BrTryAnyAsPoint(boxed)
	defer BrRelease(first.value)
	second := BrTryAnyAsPoint(boxed)
	defer BrRelease(second.value)
	if first.ok != 1 || second.ok != 1 || first.value == second.value {
		t.Fatal("struct assertions must create independent variables and handles")
	}
	BrPointSetX(first.value, 3)
	if BrPointX(second.value) != 1 || BrPointX(point) != 2 {
		t.Fatal("struct assertion exposed a shared variable")
	}
}

func TestUnknownValuesAndOwnership(t *testing.T) {
	baseline := bridgeRoots.Load()
	handle := BrUnknown()
	copy := BrEcho(handle)
	if handle == copy || bridgeRoots.Load() != baseline+2 {
		t.Fatal("each result must own a new handle")
	}
	BrRelease(handle)
	BrCollect()
	if BrUnknownValue(copy) != 42 {
		t.Fatal("opaque dynamic value did not survive source release")
	}
	for range 100 {
		if result := BrTryAnyAsPoint(copy); result.ok != 0 || result.value != 0 {
			t.Fatal("failed assertion allocated a value")
		}
	}
	BrRelease(copy)
	if bridgeRoots.Load() != baseline {
		t.Fatal("handle leak")
	}
}

func TestAnyHandleStoresDynamicValueDirectly(t *testing.T) {
	handle := BrIntAsAny(1)
	defer BrRelease(handle)
	if value, ok := cgo.Handle(handle).Value().(int); !ok || value != 1 {
		t.Fatal("any handle must store the original dynamic value directly")
	}
	if result := BrTryAnyAsInt(handle); result.ok != 1 || result.value != 1 {
		t.Fatal("int assertion failed")
	}
	if result := BrTryAnyAsInt64(handle); result.ok != 0 {
		t.Fatal("int and int64 must remain different dynamic types")
	}
}
