// Package abi defines the C transport contract shared by the Go and C++
// emitters, so both sides of the dynamic library boundary agree on one
// signature per operation.
//
// Ownership: a String input borrows a UTF-8 buffer for the duration of a call.
// A String output owns C-allocated memory (freed with free), including an
// empty string. Numeric values carry no ownership. Every result record also
// owns its panic_message buffer, which is set only when panicked is nonzero.
package abi

import (
	"fmt"
	"go/types"

	"go-bridge/internal/pyapi"
)

// Kind selects the conversion used on both sides of the boundary.
type Kind string

const (
	Signed   Kind = "signed"
	Unsigned Kind = "unsigned"
	Boolean  Kind = "boolean"
	Float    Kind = "float"
	String   Kind = "string"
)

// Type is the wire representation of one Python-facing type.
type Type struct {
	Python *pyapi.Type
	Kind   Kind
	C      string // C type name, e.g. "int32_t" or "SBString"
	Go     string // Go type name used to convert the wire value back
	Bits   int    // wire width; zero for String
}

// Operation is one exported C function wrapping one Go function.
//
// It is declared as `Result Symbol(params...)`, where Result is a struct with
// `uint8_t panicked; SBString panic_message;` followed by one field rN per
// Go result.
type Operation struct {
	Python  *pyapi.Function
	Symbol  string // exported C function name
	Result  string // C name of the result struct
	Params  []*Type
	Results []*Type
}

type Package struct {
	Python     *pyapi.Package
	Operations []*Operation
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
		op := &Operation{Python: f, Symbol: fmt.Sprintf("SBCall%d", i), Result: fmt.Sprintf("SBResult%d", i)}
		for _, arg := range f.Params {
			op.Params = append(op.Params, wire[arg.Type])
		}
		for _, r := range f.Results {
			op.Results = append(op.Results, wire[r])
		}
		result.Operations = append(result.Operations, op)
	}
	return result, nil
}

func wireType(t *pyapi.Type, sizes types.Sizes) (*Type, error) {
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
