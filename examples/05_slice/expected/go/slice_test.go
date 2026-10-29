package main

import (
	"runtime"
	"slices"
	"testing"
	"unsafe"
)

// Test files cannot name C types, so handle slices are built by inference.
func list[H any](items ...H) []H { return items }

func TestSliceIntCopiesInput(t *testing.T) {
	input := []int{1, -2, 3}
	id := BrNewSliceInt(unsafe.Pointer(&input[0]), 3)
	defer BrUnPin(id)
	input[0] = 100
	runtime.GC()
	if !slices.Equal(Get[[]int](id), []int{1, -2, 3}) {
		t.Fatal("Go slice aliases the external buffer")
	}
}

func TestAppendKeepsGoSemantics(t *testing.T) {
	// Len 4 with spare capacity, like a reslice or a ConcatA result.
	base := Pin(append(make([]int, 0, 8), 1, 2, 3, 4))
	defer BrUnPin(base)
	extra := []int{5, 6}
	five := BrNewSliceInt(unsafe.Pointer(&extra[0]), 1)
	six := BrNewSliceInt(unsafe.Pointer(&extra[1]), 1)
	defer BrUnPin(five)
	defer BrUnPin(six)

	first := BrConcatA(base, five)
	defer BrUnPin(first)
	second := BrConcatA(base, six)
	defer BrUnPin(second)
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
	id := BrNewSliceInt(unsafe.Pointer(&input[0]), 4)
	defer BrUnPin(id)
	BrSliceIntSet(id, 1, 20)
	if BrSliceIntAt(id, 1) != 20 {
		t.Fatal("wrong element")
	}

	middle := BrSliceIntSlice(id, 1, 3)
	defer BrUnPin(middle)
	copied := BrSliceIntClone(id)
	defer BrUnPin(copied)
	BrSliceIntSet(middle, 0, 200)
	if !slices.Equal(Get[[]int](id), []int{1, 200, 3, 4}) {
		t.Fatal("reslice does not share the backing array")
	}
	if !slices.Equal(Get[[]int](copied), []int{1, 20, 3, 4}) {
		t.Fatal("clone shares the backing array")
	}
	if cap(Get[[]int](middle)) != 3 {
		t.Fatal("reslice does not keep Go capacity")
	}
}

func TestNestedSharesElements(t *testing.T) {
	input := []int{7, 8}
	inner := BrNewSliceInt(unsafe.Pointer(&input[0]), 2)
	handles := list(inner, inner)
	outer := BrNewSliceSliceInt(unsafe.Pointer(&handles[0]), 2)
	defer BrUnPin(outer)
	BrUnPin(inner)
	runtime.GC()

	items := list(outer, outer) // Placeholders, overwritten below.
	BrSliceSliceIntItems(outer, unsafe.Pointer(&items[0]))
	for _, item := range items {
		if !slices.Equal(Get[[]int](item), []int{7, 8}) {
			t.Fatal("element lost after its Ref was released")
		}
		BrUnPin(item)
	}
}

func TestNestedElementsAndClone(t *testing.T) {
	baseline := holderCount()
	input := []int{1, 2}
	inner := BrNewSliceInt(unsafe.Pointer(&input[0]), 2)
	handles := list(inner, inner)
	outer := BrNewSliceSliceInt(unsafe.Pointer(&handles[0]), 2)
	replacement := Pin([]int{9})
	BrSliceSliceIntSet(outer, 1, replacement)
	BrUnPin(replacement)

	copied := BrSliceSliceIntClone(outer)
	tail := BrSliceSliceIntSlice(outer, 1, 2)
	element := BrSliceSliceIntAt(copied, 0)
	BrSliceIntSet(element, 0, 100) // Shallow clone: element arrays stay shared.
	BrUnPin(inner)
	BrUnPin(outer)
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
		BrUnPin(id)
	}
	if holderCount() != baseline {
		t.Fatal("pin leaked")
	}
}
