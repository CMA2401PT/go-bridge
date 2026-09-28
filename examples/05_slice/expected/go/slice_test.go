package main

import (
	"runtime"
	"slices"
	"testing"
	"unsafe"
)

// Test files cannot name C types, so handle slices are built by inference.
func list[H any](items ...H) []H { return items }

func must[H comparable](t *testing.T, id H) H {
	t.Helper()
	var zero H
	if id == zero {
		t.Fatal("valid input rejected")
	}
	return id
}

func TestSliceIntCopiesInput(t *testing.T) {
	input := []int{1, -2, 3}
	id := must(t, SBNewSliceInt(unsafe.Pointer(&input[0]), 3))
	defer SBUnPin(id)
	input[0] = 100
	runtime.GC()
	if !slices.Equal(Get[[]int](id), []int{1, -2, 3}) {
		t.Fatal("Go slice aliases the external buffer")
	}
}

func TestAppendKeepsGoSemantics(t *testing.T) {
	// Len 4 with spare capacity, like a reslice or a ConcatA result.
	base := Pin(append(make([]int, 0, 8), 1, 2, 3, 4))
	defer SBUnPin(base)
	extra := []int{5, 6}
	five := must(t, SBNewSliceInt(unsafe.Pointer(&extra[0]), 1))
	six := must(t, SBNewSliceInt(unsafe.Pointer(&extra[1]), 1))
	defer SBUnPin(five)
	defer SBUnPin(six)

	first := SBConcatA(base, five)
	defer SBUnPin(first)
	second := SBConcatA(base, six)
	defer SBUnPin(second)
	// Arguments are passed unchanged, so both appends write the same element.
	if !slices.Equal(Get[[]int](first), []int{1, 2, 3, 4, 6}) {
		t.Fatal("append did not reuse spare capacity")
	}
	if !slices.Equal(Get[[]int](second), []int{1, 2, 3, 4, 6}) {
		t.Fatal("wrong concatenation")
	}
}

func TestElementsResliceAndClone(t *testing.T) {
	input := []int{1, 2, 3, 4}
	id := must(t, SBNewSliceInt(unsafe.Pointer(&input[0]), 4))
	defer SBUnPin(id)
	if SBSliceIntSet(id, 1, 20) != 0 || SBSliceIntSet(id, 4, 0) == 0 {
		t.Fatal("wrong bounds for set")
	}
	var value int
	if SBSliceIntAt(id, 1, unsafe.Pointer(&value)) != 0 || value != 20 {
		t.Fatal("wrong element")
	}
	if SBSliceIntAt(id, 4, unsafe.Pointer(&value)) == 0 || SBSliceIntAt(id, 0, nil) == 0 {
		t.Fatal("invalid element read accepted")
	}

	middle := must(t, SBSliceIntSlice(id, 1, 3))
	defer SBUnPin(middle)
	copied := must(t, SBSliceIntClone(id))
	defer SBUnPin(copied)
	SBSliceIntSet(middle, 0, 200)
	if !slices.Equal(Get[[]int](id), []int{1, 200, 3, 4}) {
		t.Fatal("reslice does not share the backing array")
	}
	if !slices.Equal(Get[[]int](copied), []int{1, 20, 3, 4}) {
		t.Fatal("clone shares the backing array")
	}
	if cap(Get[[]int](middle)) != 3 {
		t.Fatal("reslice does not keep Go capacity")
	}
	if SBSliceIntSlice(id, 3, 2) != 0 || SBSliceIntSlice(id, 0, 5) != 0 {
		t.Fatal("invalid reslice bounds accepted")
	}
}

func TestNestedSharesElements(t *testing.T) {
	input := []int{7, 8}
	inner := must(t, SBNewSliceInt(unsafe.Pointer(&input[0]), 2))
	handles := list(inner, inner)
	outer := must(t, SBNewSliceSliceInt(unsafe.Pointer(&handles[0]), 2))
	defer SBUnPin(outer)
	SBUnPin(inner)
	runtime.GC()

	items := list(outer, outer) // Placeholders, overwritten below.
	if SBSliceSliceIntItems(outer, unsafe.Pointer(&items[0]), 2) != 0 {
		t.Fatal("items rejected")
	}
	for _, item := range items {
		if !slices.Equal(Get[[]int](item), []int{7, 8}) {
			t.Fatal("element lost after its Ref was released")
		}
		SBUnPin(item)
	}
}

func TestInvalidBuffers(t *testing.T) {
	if SBNewSliceInt(nil, 1) != 0 || SBNewSliceSliceInt(nil, 1) != 0 {
		t.Fatal("invalid external input accepted")
	}
	zero := list(SBNewSliceInt(nil, 1))
	if SBNewSliceSliceInt(unsafe.Pointer(&zero[0]), 1) != 0 {
		t.Fatal("zero handle accepted")
	}
	empty := SBNewSliceInt(nil, 0)
	defer SBUnPin(empty)
	if SBSliceIntLen(empty) != 0 || SBCopySliceInt(empty, nil, 0) != 0 {
		t.Fatal("empty slice rejected")
	}
	if SBCopySliceInt(empty, nil, 1) == 0 {
		t.Fatal("invalid copy buffer accepted")
	}
	nested := SBNewSliceSliceInt(nil, 0)
	defer SBUnPin(nested)
	if SBSliceSliceIntLen(nested) != 0 || SBSliceSliceIntItems(nested, nil, 1) == 0 {
		t.Fatal("invalid items buffer accepted")
	}
	if holderCount() != 2 {
		t.Fatal("pin leaked")
	}
}

func TestNestedElementsAndClone(t *testing.T) {
	baseline := holderCount()
	input := []int{1, 2}
	inner := must(t, SBNewSliceInt(unsafe.Pointer(&input[0]), 2))
	handles := list(inner, inner)
	outer := must(t, SBNewSliceSliceInt(unsafe.Pointer(&handles[0]), 2))
	replacement := Pin([]int{9})
	if SBSliceSliceIntSet(outer, 1, replacement) != 0 || SBSliceSliceIntSet(outer, 2, inner) == 0 ||
		SBSliceSliceIntSet(outer, 0, 0) == 0 {
		t.Fatal("wrong bounds for nested set")
	}
	SBUnPin(replacement)
	if SBSliceSliceIntAt(outer, 2) != 0 {
		t.Fatal("out-of-range element accepted")
	}

	copied := must(t, SBSliceSliceIntClone(outer))
	tail := must(t, SBSliceSliceIntSlice(outer, 1, 2))
	element := must(t, SBSliceSliceIntAt(copied, 0))
	SBSliceIntSet(element, 0, 100) // Shallow clone: element arrays stay shared.
	SBUnPin(inner)
	SBUnPin(outer)
	runtime.GC()
	if !slices.Equal(Get[[]int](element), []int{100, 2}) || input[0] != 1 {
		t.Fatal("element does not share its array")
	}
	if got := Get[[][]int](copied); len(got) != 2 || got[0][0] != 100 || got[1][0] != 9 {
		t.Fatal("wrong shallow clone")
	}
	if got := Get[[][]int](tail); len(got) != 1 || got[0][0] != 9 {
		t.Fatal("wrong nested reslice")
	}
	for _, id := range list(copied, tail, element) {
		SBUnPin(id)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}
