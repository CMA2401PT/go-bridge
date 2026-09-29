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

	"go-bridge/internal/gomodel"
	"go-bridge/internal/pyapi"
)

// InitSymbol is the internal void(void) entry point called when the native
// module loads. Entering an exported Go function waits for Go initialization.
// It is separate from target API operations and has no Python export.
const InitSymbol = "SBInit"

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
	Python *pyapi.Type
	Cpp    string // internal C++ wrapper type name for handles
	Kind   Kind
	C      string // C type name, e.g. "int32_t" or "SBString"
	Go     string // Go type name used to convert the wire value back
	Bits   int    // wire width; zero for String
}

// Operation is one exported C function wrapping one Go function.
//
// Functions with results return a Result struct containing one field rN per
// Go result. Functions without results return void and have an empty Result.
type Operation struct {
	Python  *pyapi.Function
	Symbol  string // exported C function name
	Result  string // C name of the result struct; empty for void operations
	Params  []*Type
	Results []*Type
}

type Package struct {
	Python     *pyapi.Package
	Operations []*Operation
	Slices     []*Slice
	IntBits    int // width of Go int on the target; checked by the C++ side
}

// Plan assigns wire types and stable symbols. Symbols use declaration order,
// never user-visible names, so renaming rules cannot produce C collisions.
func Plan(p *pyapi.Package) (*Package, error) {
	result := &Package{Python: p, IntBits: int(p.Go.Sizes.Sizeof(types.Typ[types.Int]) * 8)}
	wire := make(map[*pyapi.Type]*Type)
	for _, t := range p.Types {
		w, err := wireType(t, p.Go.Sizes)
		if err != nil {
			return nil, err
		}
		wire[t] = w
	}
	for i, f := range p.Functions {
		op := &Operation{Python: f, Symbol: fmt.Sprintf("SBCall%d", i)}
		if len(f.Results) > 0 {
			op.Result = fmt.Sprintf("SBResult%d", i)
		}
		for _, arg := range f.Params {
			op.Params = append(op.Params, wire[arg.Type])
		}
		for _, r := range f.Results {
			op.Results = append(op.Results, wire[r])
		}
		result.Operations = append(result.Operations, op)
	}
	for _, t := range p.Types {
		if t.Elem == nil {
			continue
		}
		prefix := fmt.Sprintf("SBType%d", t.Go.ID)
		result.Slices = append(result.Slices, &Slice{Type: wire[t], Elem: wire[t.Elem],
			New: prefix + "New", Len: prefix + "Len", At: prefix + "At", Set: prefix + "Set",
			Reslice: prefix + "Reslice", Clone: prefix + "Clone", Copy: prefix + "Copy"})
	}
	return result, nil
}

func wireType(t *pyapi.Type, sizes types.Sizes) (*Type, error) {
	if t.Go.Kind == gomodel.Slice {
		return &Type{Python: t, Kind: Handle, C: "uintptr_t", Go: types.TypeString(t.Go.Go, nil), Cpp: fmt.Sprintf("Type%d", t.Go.ID)}, nil
	}
	basic, ok := t.Go.Go.(*types.Basic)
	if !ok {
		return nil, fmt.Errorf("ABI for %s is not implemented", t.Go)
	}
	w := &Type{Python: t, Go: basic.Name()}
	bits := int(sizes.Sizeof(basic) * 8)
	info := basic.Info()
	switch {
	case info&types.IsBoolean != 0:
		w.Kind, w.C, w.Bits = Boolean, "uint8_t", 8
	case info&types.IsString != 0:
		w.Kind, w.C = String, "SBString"
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
	ReleaseSymbol = "SBRelease"
	CountSymbol   = "SBHolderCount"
	CollectSymbol = "SBCollect"
)
