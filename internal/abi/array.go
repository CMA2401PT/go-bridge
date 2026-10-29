package abi

import (
	"fmt"
	"go-bridge/internal/pyapi"
)

// Array operations keep the wrapper's address stable; Copy returns a value copy.
type Array struct {
	Type, Elem               *Type
	Length                   int64
	Zero, New, At, Set, Copy string
}

func planArray(t *pyapi.Type, wire map[*pyapi.Type]*Type) *Array {
	prefix := fmt.Sprintf("BrType%d", t.Go.ID)
	a := &Array{Type: wire[t], Elem: wire[t.Elem], Length: t.Length, At: prefix + "At", Set: prefix + "Set", Copy: prefix + "Items"}
	if t.Underlying == nil {
		a.Zero = prefix + "Zero"
		a.New = prefix + "New"
	}
	return a
}
