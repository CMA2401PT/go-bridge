// Package gomodel describes the exported Go API without Python or ABI policy.
package gomodel

import "go/types"

type Kind string

const (
	Basic   Kind = "basic"
	Named   Kind = "named"
	Slice   Kind = "slice"
	Pointer Kind = "pointer"
)

// Type is registered before its dependencies are visited. Edges may form cycles.
// Go is canonical (aliases have been removed), and determines type identity.
type Type struct {
	ID             int
	Go             types.Type
	Kind           Kind
	Name           string
	PackagePath    string
	Underlying     *Type   // named non-struct only
	Elem           *Type   // slice or pointer
	Fields         []Field // direct exported fields of a named struct
	ValueMethods   []*Function
	PointerMethods []*Function
}

func (t *Type) IsStruct() bool {
	_, ok := t.Go.Underlying().(*types.Struct)
	return ok
}

func (t *Type) String() string { return types.TypeString(t.Go, nil) }

type Field struct {
	Name     string
	Type     *Type
	Embedded bool
	Index    int // actual Go field index, including unexported fields
}

type Parameter struct {
	Name string // may be empty or "_"
	Type *Type
}

// Receiver is the declared receiver; SelectedReceiver is the type whose method
// set was queried. Querying *T's method set does not itself put *T in the type
// closure. Promoted methods may be declared on an unexported embedded type.
type Function struct {
	Name             string
	Receiver         types.Type
	SelectedReceiver types.Type
	PointerReceiver  bool
	Params           []Parameter
	Results          []Parameter
}

// Declaration preserves exported alias names without inventing type identities.
type Declaration struct {
	Name  string
	Type  *Type
	Alias bool
}

type Package struct {
	Path         string
	Name         string
	Dir          string
	Types        []*Type
	Declarations []Declaration
	Functions    []*Function
	Sizes        types.Sizes
}

func (p *Package) Find(t types.Type) *Type {
	t = types.Unalias(t)
	for _, node := range p.Types {
		if types.Identical(node.Go, t) {
			return node
		}
	}
	return nil
}
