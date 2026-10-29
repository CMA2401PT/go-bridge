// Package abi defines the C transport contract shared by the Go and C++
// emitters, so both sides of the dynamic library boundary agree on one
// signature per operation.
//
// Ownership: a String input borrows a UTF-8 buffer for the duration of a call.
// A String output owns C-allocated memory (freed with free), including an
// empty string. Numeric values carry no ownership. Go panics are not recovered
// by the bridge and have no ABI representation.
package abi

import (
	"fmt"
	"go/types"

	"go-bridge/internal/pyapi"
)

// InitSymbol is the internal void(void) entry point called when the native
// module loads. Entering an exported Go function waits for Go initialization.
// It is separate from target API operations and has no Python export.
const InitSymbol = "BrInit"

// Kind selects the conversion used on both sides of the boundary.
type Kind string

const (
	Signed   Kind = "signed"
	Unsigned Kind = "unsigned"
	Boolean  Kind = "boolean"
	Float    Kind = "float"
	String   Kind = "string"
	Handle   Kind = "handle"
)

// Type is the wire representation of one Python-facing type.
type Type struct {
	Python      *pyapi.Type
	Cpp         string
	Kind        Kind
	C, Go, Cell string
	Bits        int
}

// Operation is one exported C function wrapping one Go function.
//
// Functions with results return a Result struct containing one field rN per
// Go result. Functions without results return void and have an empty Result.
type Operation struct {
	Python   *pyapi.Function
	Receiver *Type
	Invoke   bool   // receiver is a function value, called directly
	Symbol   string // exported C function name
	Result   string // C name of the result struct; empty for void operations
	Params   []*Type
	Results  []*Type
}

type Package struct {
	Python               *pyapi.Package
	Operations           []*Operation
	Slices               []*Slice
	Channels             []*Channel
	Maps                 []*Map
	Arrays               []*Array
	InterfaceConversions []*InterfaceConversion
	Types                []*Type
	Classes              []*Class
	Imports              []Import
	IntBits              int   // width of Go int on the target; checked by the C++ side
	Any                  *Type // GoAny, or nil when the closure has no empty interface
	AnyConversions       []*AnyConversion
}

// Plan assigns wire types and stable symbols. Symbols use declaration order,
// never user-visible names, so renaming rules cannot produce C collisions.
func Plan(p *pyapi.Package) (*Package, error) {
	result := &Package{Python: p, IntBits: int(p.Go.Sizes.Sizeof(types.Typ[types.Int]) * 8)}
	qualifier := planImports(result)
	wire := make(map[*pyapi.Type]*Type)
	for _, t := range p.Types {
		w, err := wireType(t, p.Go.Sizes, qualifier)
		if err != nil {
			return nil, err
		}
		result.Types = append(result.Types, w)
		wire[t] = w
	}
	result.Any = wire[p.Any]
	for _, c := range p.AnyConversions {
		prefix := fmt.Sprintf("BrType%d", c.Type.Go.ID)
		result.AnyConversions = append(result.AnyConversions, &AnyConversion{
			Python: c, Type: wire[c.Type], Box: prefix + "AsAny", Assert: prefix + "AssertAny", Result: prefix + "AnyResult",
		})
	}
	for _, f := range p.Functions {
		result.Operations = append(result.Operations, planCall(f, nil, wire, len(result.Operations)))
	}
	for _, t := range p.Types {
		w := wire[t]
		for _, source := range t.Implementors {
			result.InterfaceConversions = append(result.InterfaceConversions, &InterfaceConversion{Target: w, Source: wire[source], Symbol: fmt.Sprintf("BrType%dFromType%d", t.Go.ID, source.Go.ID)})
		}
		if t.Shape == pyapi.ArrayObject {
			result.Arrays = append(result.Arrays, planArray(t, wire))
		}
		if t.Shape == pyapi.MapObject {
			result.Maps = append(result.Maps, planMap(t, wire))
		}
		if t.Shape == pyapi.ChannelObject {
			result.Channels = append(result.Channels, planChannel(t, wire))
		}
		if t.Shape == pyapi.SliceObject {
			prefix := fmt.Sprintf("BrType%d", t.Go.ID)
			sl := &Slice{Type: w, Elem: wire[t.Elem], Len: prefix + "Len", At: prefix + "At", Set: prefix + "Set", Reslice: prefix + "Reslice", Clone: prefix + "Clone", Copy: prefix + "Copy"}
			if t.Underlying == nil {
				sl.New = prefix + "New"
			}
			result.Slices = append(result.Slices, sl)
		}
		if t.Shape != pyapi.Builtin {
			c := planClass(t, wire)
			if t.Call != nil {
				op := planCall(t.Call, w, wire, len(result.Operations))
				op.Invoke = true
				result.Operations = append(result.Operations, op)
			}
			for _, f := range t.Methods {
				op := planCall(f, w, wire, len(result.Operations))
				c.Methods = append(c.Methods, op)
				result.Operations = append(result.Operations, op)
			}
			result.Classes = append(result.Classes, c)
		}
	}
	return result, nil
}

func planCall(f *pyapi.Function, receiver *Type, wire map[*pyapi.Type]*Type, index int) *Operation {
	op := &Operation{Python: f, Receiver: receiver, Symbol: fmt.Sprintf("BrCall%d", index)}
	if len(f.Results) > 0 {
		op.Result = fmt.Sprintf("BrResult%d", index)
	}
	if receiver != nil {
		op.Params = append(op.Params, receiver)
	}
	for _, arg := range f.Params {
		op.Params = append(op.Params, wire[arg.Type])
	}
	for _, r := range f.Results {
		op.Results = append(op.Results, wire[r])
	}
	return op
}

func wireType(t *pyapi.Type, sizes types.Sizes, qualifier types.Qualifier) (*Type, error) {
	w := &Type{Python: t, Go: types.TypeString(t.Go.Go, qualifier), Cpp: fmt.Sprintf("Type%d", t.Go.ID)}
	if t.Shape != pyapi.Builtin && t.Shape != pyapi.NamedValue {
		w.Kind, w.C = Handle, "uintptr_t"
		cell := t.Go.Go
		if t.Underlying != nil {
			cell = t.Go.Go.Underlying()
		}
		if t.Variable() {
			cell = types.NewPointer(t.Go.Go)
		}
		w.Cell = types.TypeString(cell, qualifier)
		return w, nil
	}
	basic, ok := t.Go.Go.Underlying().(*types.Basic)
	if !ok {
		return nil, fmt.Errorf("nonbasic scalar %s", t.Go)
	}

	bits := int(sizes.Sizeof(basic) * 8)
	info := basic.Info()
	switch {
	case info&types.IsBoolean != 0:
		w.Kind, w.C, w.Bits = Boolean, "uint8_t", 8
	case info&types.IsString != 0:
		w.Kind, w.C = String, "BrString"
	case info&types.IsFloat != 0:
		w.Kind, w.C, w.Bits = Float, "double", bits
		if bits == 32 {
			w.C = "float"
		}
	case info&types.IsInteger != 0:
		w.Kind, w.Bits = Signed, bits
		prefix := "int"
		if info&types.IsUnsigned != 0 {
			w.Kind, prefix = Unsigned, "uint"
		}
		// Word-sized Go integers use intptr_t/uintptr_t rather than a fixed
		// width, so the C declaration matches whatever target cgo builds for.
		switch basic.Kind() {
		case types.Int, types.Uint, types.Uintptr:
			w.C = prefix + "ptr_t"
		default:
			w.C = fmt.Sprintf("%s%d_t", prefix, bits)
		}
	default:
		return nil, fmt.Errorf("ABI for %s is not implemented", t.Go)
	}
	return w, nil
}

// Slice plans the typed bulk and element operations of one concrete slice.
// New borrows an element buffer; Copy writes owned elements into caller memory.
// At returns one owned element, and Reslice/Clone/New return owned handles.
type Slice struct {
	Type, Elem                              *Type
	New, Len, At, Set, Reslice, Clone, Copy string
}

const (
	ReleaseSymbol = "BrRelease"
	CountSymbol   = "BrHolderCount"
	CollectSymbol = "BrCollect"
)
