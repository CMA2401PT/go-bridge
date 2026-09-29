// Package abi defines the transport contract shared by Go and C++ emitters.
package abi

import (
	"fmt"
	"go/types"

	"go-bridge/internal/pyapi"
)

type Kind string

const (
	Signed   Kind = "signed"
	Unsigned Kind = "unsigned"
	Boolean  Kind = "boolean"
	Float    Kind = "float"
	String   Kind = "string"
)

// A String input borrows a UTF-8 buffer for the duration of a call. A String
// output owns C-allocated memory, including an empty string. Numeric values
// carry no ownership. Every result record also owns its panic_message buffer.
type Type struct {
	Python *pyapi.Type
	Kind   Kind
	C      string
	Go     string
	Bits   int
}

type Operation struct {
	Python  *pyapi.Function
	Symbol  string
	Result  string
	Params  []*Type
	Results []*Type
}

type Package struct {
	Python     *pyapi.Package
	Operations []*Operation
	IntBits    int
}

func Plan(p *pyapi.Package) (*Package, error) {
	result := &Package{Python: p, IntBits: int(p.Go.Sizes.Sizeof(types.Typ[types.Int]) * 8)}
	mapping := make(map[*pyapi.Type]*Type)
	for _, t := range p.Types {
		basic, ok := t.Go.Go.(*types.Basic)
		if !ok {
			return nil, fmt.Errorf("ABI for %s is not implemented", t.Go)
		}
		wire := &Type{Python: t, Go: basic.Name()}
		switch {
		case basic.Info()&types.IsBoolean != 0:
			wire.Kind, wire.C, wire.Bits = Boolean, "uint8_t", 8
		case basic.Info()&types.IsString != 0:
			wire.Kind, wire.C = String, "SBString"
		case basic.Info()&types.IsFloat != 0:
			wire.Kind, wire.Bits = Float, int(p.Go.Sizes.Sizeof(basic)*8)
			wire.C = "double"
			if wire.Bits == 32 {
				wire.C = "float"
			}
		case basic.Info()&types.IsInteger != 0:
			wire.Kind, wire.Bits = Signed, int(p.Go.Sizes.Sizeof(basic)*8)
			prefix := "int"
			if basic.Info()&types.IsUnsigned != 0 {
				wire.Kind, prefix = Unsigned, "uint"
			}
			wire.C = fmt.Sprintf("%s%d_t", prefix, wire.Bits)
			if basic.Kind() == types.Int || basic.Kind() == types.Uint || basic.Kind() == types.Uintptr {
				wire.C = prefix + "ptr_t"
			}
		default:
			return nil, fmt.Errorf("ABI for %s is not implemented", t.Go)
		}
		mapping[t] = wire
	}
	for i, f := range p.Functions {
		op := &Operation{Python: f, Symbol: fmt.Sprintf("SBCall%d", i), Result: fmt.Sprintf("SBResult%d", i)}
		for _, arg := range f.Params {
			op.Params = append(op.Params, mapping[arg.Type])
		}
		for _, r := range f.Results {
			op.Results = append(op.Results, mapping[r])
		}
		result.Operations = append(result.Operations, op)
	}
	return result, nil
}
