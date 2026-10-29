// Package gomodel describes the exported Go API without Python or ABI policy.
//
// The model is a graph of Go facts: every type reachable from the target
// package's exported declarations, with its fields, methods and dependencies.
// Later stages decide how (and whether) each node is presented to Python.
package gomodel

import (
	"go/types"

	"golang.org/x/tools/go/types/typeutil"
)

// Kind classifies a Type node by its Go type constructor.
type Kind string

const (
	Basic     Kind = "basic"
	Named     Kind = "named"
	Slice     Kind = "slice"
	Channel   Kind = "channel"
	Pointer   Kind = "pointer"
	Interface Kind = "interface"
)

// Type is one node of the type closure. Go is canonical (aliases have been
// removed) and determines identity: two nodes never hold identical Go types.
//
// A node is registered before its dependencies are visited, so edges may form
// cycles (for example a struct holding a pointer to itself).
type Type struct {
	ID          int    // position in Package.Types, stable for a given input
	Origin      string // first dependency path that introduced this node
	Go          types.Type
	Kind        Kind
	Name        string // Basic and Named only
	PackagePath string // Named only; empty for predeclared types

	Underlying *Type         // Named non-struct only
	Elem       *Type         // Slice, Pointer or Channel
	Direction  types.ChanDir // Channel only
	Fields     []Field       // Named struct only: direct exported fields
	Selectable []Selection   // Named struct only: visible direct and promoted fields

	// Method sets of T and *T, including promoted methods (Named only).
	ValueMethods   []*Function
	PointerMethods []*Function
}

func (t *Type) String() string { return types.TypeString(t.Go, nil) }

// Field is a direct exported field, eligible for keyed construction. Every
// Field also occurs in Selectable with Direct=true.
type Field struct {
	Name     string
	Type     *Type
	Embedded bool
	Index    int // actual Go field index, counting unexported fields
}

// Parameter is a function parameter or result.
type Parameter struct {
	Name string // may be empty or "_"
	Type *Type
}

// Function is a package-level function or a method selected from a method set.
//
// Receiver is the declared receiver; SelectedReceiver is the type whose method
// set was queried (nil for package-level functions). They differ for methods
// promoted through embedding, and for value methods selected from *T. Querying
// *T's method set does not by itself put *T in the type closure, and a
// promoted method may be declared on an unexported embedded type.
type Function struct {
	Name             string
	Receiver         types.Type
	SelectedReceiver types.Type
	PointerReceiver  bool // the declared receiver is a pointer
	Params           []Parameter
	Results          []Parameter
}

// Declaration is an exported type name. Aliases share the node of the type
// they denote instead of inventing a new identity.
type Declaration struct {
	Name  string
	Type  *Type
	Alias bool
}

// Package is the analyzed target package.
type Package struct {
	Path         string
	Name         string
	Dir          string // source directory; set by Load only
	Types        []*Type
	Declarations []Declaration
	Functions    []*Function
	Sizes        types.Sizes

	index typeutil.Map // types.Type -> *Type, keyed by types.Identical
}

// Find returns the node for t, or nil if t is not in the closure.
func (p *Package) Find(t types.Type) *Type {
	node, _ := p.index.At(types.Unalias(t)).(*Type)
	return node
}

// Selection is an exported field visible through a Go selector on a struct.
// Path is diagnostic metadata; emitters select by Name so hidden embedding
// paths never have to be named outside the target package.
type Selection struct {
	Name     string
	Type     *Type
	Path     []int
	Indirect bool
	Direct   bool
}
