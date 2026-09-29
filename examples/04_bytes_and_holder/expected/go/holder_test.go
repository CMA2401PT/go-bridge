package main

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
)

func TestGenericPinAndGC(t *testing.T) {
	type record struct {
		Name  string
		Data  []byte
		Child *int
	}
	child := 42
	id := Pin(record{Name: "record", Data: []byte{0, 255}, Child: &child})
	runtime.GC()
	value := Get[record](id)
	if value.Name != "record" || !bytes.Equal(value.Data, []byte{0, 255}) || *value.Child != 42 {
		t.Fatal("pinned value did not preserve its Go references")
	}
	UnPin(id)
	// A Go reference obtained through Get has an ordinary Go lifetime.
	runtime.GC()
	if *value.Child != 42 || value.Data[1] != 255 {
		t.Fatal("UnPin invalidated another Go reference")
	}
	if holderCount() != 0 {
		t.Fatal("pin leaked")
	}
}

func TestNilValue(t *testing.T) {
	id := Pin[any](nil)
	if Get[any](id) != nil {
		t.Fatal("nil value changed")
	}
	UnPin(id)
	UnPin(id) // No intervening allocation; this is a missing entry, not a new Ref.
	UnPin(0)
	if holderCount() != 0 {
		t.Fatal("pin leaked")
	}
}

func TestConcurrentIndependentRefs(t *testing.T) {
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 100; j++ {
				id := Pin([]byte{42})
				if Get[[]byte](id)[0] != 42 {
					t.Error("value changed")
				}
				UnPin(id)
			}
		}()
	}
	group.Wait()
	if holderCount() != 0 {
		t.Fatal("pin leaked")
	}
}

func TestInvalidInputBuffer(t *testing.T) {
	if BrNewBytes(nil, 1) != 0 {
		t.Fatal("invalid external input accepted")
	}
	id := BrNewBytes(nil, 0)
	defer BrUnPin(id)
	if id == 0 || BrBytesLen(id) != 0 || BrCopyBytes(id, nil, 0) != 0 {
		t.Fatal("empty slice rejected")
	}
	if BrCopyBytes(id, nil, 1) == 0 {
		t.Fatal("invalid copy buffer accepted")
	}
}
