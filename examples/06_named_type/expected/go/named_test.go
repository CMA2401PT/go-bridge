package main

import (
	api "go-bridge/examples/06_named_type"
	"runtime"
	"slices"
	"testing"
	"unsafe"
)

// Test files cannot name C types, so handle slices are built by inference.
func list[H any](items ...H) []H { return items }

func TestNamedSliceSharesLayout(t *testing.T) {
	baseline := holderCount()
	id := Pin([]int{1, 2})
	named := Get[api.MyNamedNonStructSliceInt](id)
	named[0] = 10
	plain := SBMyNamedNonStructSliceIntUnname(id)
	SBUnPin(id)
	runtime.GC()
	if !slices.Equal(Get[[]int](plain), []int{10, 2}) {
		t.Fatal("unname does not share the backing array")
	}
	SBUnPin(plain)
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestStructFieldViewKeepsParent(t *testing.T) {
	baseline := holderCount()
	parent := SBNewMyNamedStruct(0, 1, 0, 2, 0)
	if got := *Get[*api.MyNamedStruct](parent); got.RefMemberValue != nil || got.BasicMemberValue != 1 {
		t.Fatal("omitted fields are not zero values")
	}
	embedded := SBMyNamedStructGetMyNamedAnomySubStruct(parent)
	sub := SBMyNamedStructGetNamedRefMemberValue(parent)
	// The embedded field is at offset 0: the view and its parent share an
	// address, yet each Ref owns its own pin.
	if unsafe.Pointer(Get[*api.MyNamedAnomySubStruct](embedded)) !=
		unsafe.Pointer(Get[*api.MyNamedStruct](parent)) {
		t.Fatal("expected the embedded field at the parent's address")
	}
	SBUnPin(parent)
	runtime.GC()
	SBMyNamedSubStructSetBasicMemberValue(sub, 7)
	SBMyNamedAnomySubStructSetBasicMemberValue2(embedded, 8)
	if SBMyNamedSubStructGetBasicMemberValue(sub) != 7 ||
		SBMyNamedAnomySubStructGetBasicMemberValue2(embedded) != 8 {
		t.Fatal("field view lost after its parent Ref was released")
	}
	SBUnPin(embedded)
	SBUnPin(sub)
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestReceiversAndCopies(t *testing.T) {
	baseline := holderCount()
	numbers := Pin([]int{3})
	id := SBNewMyNamedStruct(0, 1, numbers, 0, 0)
	SBUnPin(numbers)
	SBMyNamedStructIncBasicPtr(id)
	result := SBMyNamedStructIncBasicNonPtr(id)
	copied := SBMyNamedStructCopy(id)
	Get[*api.MyNamedStruct](copied).BasicMemberValue = 100
	Get[*api.MyNamedStruct](copied).RefMemberValue[0] = 30
	value := Get[*api.MyNamedStruct](id)
	if value.BasicMemberValue != 2 || Get[*api.MyNamedStruct](result).BasicMemberValue != 3 {
		t.Fatal("wrong receiver semantics")
	}
	if value.RefMemberValue[0] != 30 {
		t.Fatal("struct copy is not shallow")
	}
	for _, handle := range list(id, result, copied) {
		SBUnPin(handle)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}
