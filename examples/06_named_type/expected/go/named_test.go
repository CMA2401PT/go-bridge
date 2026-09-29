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
	plain := BrMyNamedNonStructSliceIntUnname(id)
	BrUnPin(id)
	runtime.GC()
	if !slices.Equal(Get[[]int](plain), []int{10, 2}) {
		t.Fatal("unname does not share the backing array")
	}
	BrUnPin(plain)
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestStructFieldViewKeepsParent(t *testing.T) {
	baseline := holderCount()
	parent := BrNewMyNamedStruct(0, 1, 0, 2, 0)
	if got := *Get[*api.MyNamedStruct](parent); got.RefMemberValue != nil || got.BasicMemberValue != 1 {
		t.Fatal("omitted fields are not zero values")
	}
	embedded := BrMyNamedStructGetMyNamedAnomySubStruct(parent)
	sub := BrMyNamedStructGetNamedRefMemberValue(parent)
	// The embedded field is at offset 0: the view and its parent share an
	// address, yet each Ref owns its own pin.
	if unsafe.Pointer(Get[*api.MyNamedAnomySubStruct](embedded)) !=
		unsafe.Pointer(Get[*api.MyNamedStruct](parent)) {
		t.Fatal("expected the embedded field at the parent's address")
	}
	BrUnPin(parent)
	runtime.GC()
	BrMyNamedSubStructSetBasicMemberValue(sub, 7)
	BrMyNamedAnomySubStructSetBasicMemberValue2(embedded, 8)
	if BrMyNamedSubStructGetBasicMemberValue(sub) != 7 ||
		BrMyNamedAnomySubStructGetBasicMemberValue2(embedded) != 8 {
		t.Fatal("field view lost after its parent Ref was released")
	}
	BrUnPin(embedded)
	BrUnPin(sub)
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestReceiversAndCopies(t *testing.T) {
	baseline := holderCount()
	numbers := Pin([]int{3})
	id := BrNewMyNamedStruct(0, 1, numbers, 0, 0)
	BrUnPin(numbers)
	BrMyNamedStructIncBasicPtr(id)
	result := BrMyNamedStructIncBasicNonPtr(id)
	copied := BrMyNamedStructCopy(id)
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
		BrUnPin(handle)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}

func TestStructPointerSharesOrCopies(t *testing.T) {
	baseline := holderCount()
	id := BrNewMyNamedStruct(0, 1, 0, 0, 0)
	alias := BrMyNamedStructAlias(id)
	copied := BrMyNamedStructCopy(id)
	if alias == id || Get[*api.MyNamedStruct](alias) != Get[*api.MyNamedStruct](id) {
		t.Fatal("addr must pin a new handle to the same struct")
	}
	if Get[*api.MyNamedStruct](copied) == Get[*api.MyNamedStruct](id) {
		t.Fatal("from_value must allocate a new struct")
	}
	Get[*api.MyNamedStruct](copied).BasicMemberValue = 5
	BrMyNamedStructStore(alias, copied)
	if Get[*api.MyNamedStruct](id).BasicMemberValue != 5 ||
		Get[*api.MyNamedStruct](alias) == Get[*api.MyNamedStruct](copied) {
		t.Fatal("*p = v must copy the struct in")
	}
	for _, handle := range list(id, alias, copied) {
		BrUnPin(handle)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}
