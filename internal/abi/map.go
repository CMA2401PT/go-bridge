package abi

import (
	"fmt"
	"go-bridge/internal/pyapi"
)

// Map plans Go-keyed operations. Bulk output owns both key and value buffers.
type Map struct {
	Type, Key, Elem                                              *Type
	New, Len, Get, Set, Has, Delete, Clear, Clone, Items, Result string
}

func planMap(t *pyapi.Type, wire map[*pyapi.Type]*Type) *Map {
	p := fmt.Sprintf("BrType%d", t.Go.ID)
	m := &Map{Type: wire[t], Key: wire[t.Key], Elem: wire[t.Elem], Len: p + "Len", Get: p + "Get", Set: p + "Set", Has: p + "Has", Delete: p + "Delete", Clear: p + "Clear", Clone: p + "Clone", Items: p + "Items", Result: p + "Lookup"}
	if t.Underlying == nil {
		m.New = p + "New"
	}
	return m
}
