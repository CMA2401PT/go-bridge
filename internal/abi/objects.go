package abi

import (
	"fmt"
	"go/types"
	"sort"

	"go-bridge/internal/pyapi"
)

// Class holds the ABI operations for one concrete Python wrapper. Operations
// absent from that class have an empty symbol, so emitters do not infer policy.
type Class struct {
	Type, Underlying, Elem, Pointer                    *Type
	New, Copy, Address, Load, Store, Unname, ToPointer string
	Fields                                             []*Field
	Constructor                                        []*Field
	Methods                                            []*Operation
}

// Field plans the getter and setter for one visible Go field.
type Field struct {
	Python   *pyapi.Field
	Type     *Type
	Get, Set string
}

// Import pairs a Go import path with its generated local qualifier.
type Import struct{ Alias, Path string }

func planClass(t *pyapi.Type, wire map[*pyapi.Type]*Type) *Class {
	c := &Class{Type: wire[t], Underlying: wire[t.Underlying], Elem: wire[t.Elem], Pointer: wire[t.Pointer]}
	prefix := fmt.Sprintf("SBType%d", t.Go.ID)
	if t.Shape == pyapi.StructObject {
		c.New = prefix + "New"
		c.Copy = prefix + "Copy"
		c.Address = prefix + "Address"
	}
	if t.Shape == pyapi.PointerObject {
		c.Load = prefix + "Load"
		c.Store = prefix + "Store"
		c.New = prefix + "New"
	}
	if t.Underlying != nil && t.Shape != pyapi.NamedValue {
		c.New = prefix + "Convert"
		c.Unname = prefix + "Unname"
	}
	if t.Pointer != nil && t.Shape != pyapi.StructObject {
		c.ToPointer = prefix + "Pointer"
	}
	for i, f := range t.Fields {
		field := &Field{Python: f, Type: wire[f.Type], Get: fmt.Sprintf("%sField%dGet", prefix, i), Set: fmt.Sprintf("%sField%dSet", prefix, i)}
		c.Fields = append(c.Fields, field)
		for _, cf := range t.Constructor {
			if cf == f {
				c.Constructor = append(c.Constructor, field)
			}
		}
	}
	return c
}

// planImports names dependency packages deterministically. The target is
// always linked for init, even if only its aliases are exported.
func planImports(p *Package) types.Qualifier {
	// Only imports referenced by generated Go operations need a qualifier.
	// A standalone named scalar is created entirely in C++, so its package
	// still initializes through the target import without an unused Go alias.
	paths := map[string]bool{}
	mark := func(pkg *types.Package) string { paths[pkg.Path()] = true; return pkg.Name() }
	use := func(t *pyapi.Type) {
		types.TypeString(t.Go.Go, mark)
		if t.Underlying != nil {
			types.TypeString(t.Go.Go.Underlying(), mark)
		}
	}
	function := func(f *pyapi.Function) {
		for _, a := range f.Params {
			use(a.Type)
		}
		for _, r := range f.Results {
			use(r)
		}
	}
	for _, f := range p.Python.Functions {
		function(f)
	}
	for _, t := range p.Python.Types {
		if t.Shape != pyapi.Builtin && (t.Shape != pyapi.NamedValue || len(t.Methods) > 0 || t.Pointer != nil) {
			use(t)
		}
		for _, f := range t.Fields {
			use(f.Type)
		}
		for _, m := range t.Methods {
			function(m)
		}
	}
	targetUsed := len(p.Python.Functions) > 0 || paths[p.Python.Go.Path]
	aliases := map[string]string{p.Python.Go.Path: "api"}
	targetAlias := "_"
	if targetUsed {
		targetAlias = "api"
	}
	p.Imports = append(p.Imports, Import{targetAlias, p.Python.Go.Path})
	delete(paths, p.Python.Go.Path)
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	for i, path := range sorted {
		alias := fmt.Sprintf("dep%d", i)
		aliases[path] = alias
		p.Imports = append(p.Imports, Import{alias, path})
	}
	return func(pkg *types.Package) string {
		if alias := aliases[pkg.Path()]; alias != "" {
			return alias
		}
		return pkg.Name()
	}
}

func (p *Package) HasRefs() bool {
	for _, t := range p.Types {
		if t.Kind == Handle {
			return true
		}
	}
	return false
}
