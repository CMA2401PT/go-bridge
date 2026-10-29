package gomodel

import (
	"go/types"
	"sort"

	"golang.org/x/tools/go/types/typeutil"
)

// Ensure adds a type and its Go dependencies using the same identity index as
// Analyze. After an error the caller must discard the package. Callers
// choose bridge policy; this method only records Go facts.
func (p *Package) Ensure(t types.Type, where string) (*Type, error) {
	return (&builder{model: p}).visit(t, where)
}

// selections collects visible fields, including those promoted through hidden
// embedded types. LookupFieldOrMethod decides shadowing, ambiguity and methods
// which hide a field. Generated access uses Name, never an unexported path.
func (b *builder) selections(n *Type, t *types.Named, where string) error {
	names := map[string]bool{}
	var seen typeutil.Map
	var walk func(types.Type)
	walk = func(t types.Type) {
		t = types.Unalias(t)
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}
		if seen.At(t) != nil {
			return
		}
		seen.Set(t, true)
		structure, ok := t.Underlying().(*types.Struct)
		if !ok {
			return
		}
		for i := 0; i < structure.NumFields(); i++ {
			f := structure.Field(i)
			if f.Exported() {
				names[f.Name()] = true
			}
			if f.Embedded() {
				walk(f.Type())
			}
		}
	}
	walk(t)
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		obj, path, indirect := types.LookupFieldOrMethod(t, true, nil, name)
		field, ok := obj.(*types.Var)
		if !ok || !field.IsField() {
			continue
		}
		node, err := b.visit(field.Type(), where+" -> selected field "+name)
		if err != nil {
			return err
		}
		n.Selectable = append(n.Selectable, Selection{Name: name, Type: node, Path: path, Indirect: indirect, Direct: len(path) == 1})
	}
	return nil
}
