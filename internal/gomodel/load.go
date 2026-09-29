package gomodel

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Load accepts one package pattern or directory, interpreted relative to dir.
// The Go tool selects source files, dependencies and build constraints.
func Load(dir, target string) (*Package, error) {
	if target == "" {
		return nil, fmt.Errorf("a Go package is required")
	}
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		dir, target = path, "."
	}
	loaded, err := packages.Load(&packages.Config{
		Dir: dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesSizes,
	}, target)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", target, err)
	}
	var failures []string
	packages.Visit(loaded, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			failures = append(failures, e.Error())
		}
	})
	if len(failures) != 0 {
		return nil, fmt.Errorf("load %s:\n%s", target, strings.Join(failures, "\n"))
	}
	if len(loaded) != 1 {
		return nil, fmt.Errorf("expected one Go package, got %d for %q", len(loaded), target)
	}
	p := loaded[0]
	if p.Name == "main" {
		return nil, fmt.Errorf("%s: package main cannot be imported by an adapter", p.PkgPath)
	}
	if len(p.GoFiles) == 0 {
		return nil, fmt.Errorf("%s: no Go files", p.PkgPath)
	}
	model, err := Analyze(p.Types, p.TypesSizes)
	if err != nil {
		return nil, err
	}
	model.Dir = filepath.Dir(p.GoFiles[0])
	return model, nil
}

// supportedBasics are always part of the closure, so later stages can rely on
// them (for example for lengths and indexes) even if no signature uses them.
var supportedBasics = []types.BasicKind{
	types.Bool, types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
	types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr,
	types.Float32, types.Float64, types.String,
}

// Analyze constructs a complete model or returns a contextual error. Package
// internals are ignored; unsupported exported declarations are not skipped.
func Analyze(pkg *types.Package, sizes types.Sizes) (*Package, error) {
	b := &builder{model: &Package{Path: pkg.Path(), Name: pkg.Name(), Sizes: sizes}}
	for _, kind := range supportedBasics {
		if _, err := b.visit(types.Typ[kind], "builtin"); err != nil {
			return nil, err
		}
	}
	// Scope names are sorted, which makes node IDs deterministic.
	for _, name := range pkg.Scope().Names() {
		obj := pkg.Scope().Lookup(name)
		if !obj.Exported() {
			continue
		}
		where := pkg.Path() + "." + name
		switch obj := obj.(type) {
		case *types.TypeName:
			node, err := b.visit(obj.Type(), where)
			if err != nil {
				return nil, err
			}
			b.model.Declarations = append(b.model.Declarations, Declaration{Name: name, Type: node, Alias: obj.IsAlias()})
		case *types.Func:
			fn, err := b.function(obj, nil, where)
			if err != nil {
				return nil, err
			}
			b.model.Functions = append(b.model.Functions, fn)
		default:
			return nil, fmt.Errorf("%s: exported %T is not supported (package variables/constants are not yet mapped)", where, obj)
		}
	}
	return b.model, nil
}

// builder grows the type closure. Every method takes where, a human-readable
// path from the root declaration, used to give diagnostics context.
type builder struct{ model *Package }

// visit returns the node for t, creating it and its dependencies on first use.
func (b *builder) visit(t types.Type, where string) (*Type, error) {
	t = types.Unalias(t)
	if existing := b.model.Find(t); existing != nil {
		return existing, nil
	}
	// Register before expanding, so recursive types terminate.
	n := &Type{ID: len(b.model.Types), Go: t, Origin: where}
	b.model.Types = append(b.model.Types, n)
	b.model.index.Set(t, n)

	var err error
	switch t := t.(type) {
	case *types.Basic:
		if t.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsString) == 0 || t.Info()&types.IsUntyped != 0 {
			return nil, fmt.Errorf("%s: unsupported basic type %s", where, t)
		}
		n.Kind, n.Name = Basic, t.Name()
	case *types.Pointer:
		n.Kind = Pointer
		n.Elem, err = b.visit(t.Elem(), where+" -> pointer target")
	case *types.Slice:
		n.Kind = Slice
		n.Elem, err = b.visit(t.Elem(), where+" -> slice element")
	case *types.Named:
		err = b.named(n, t, where)
	default:
		err = fmt.Errorf("%s: unsupported type %s (%T)", where, t, t)
	}
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (b *builder) named(n *Type, t *types.Named, where string) error {
	n.Kind, n.Name = Named, t.Obj().Name()
	if t.Obj().Pkg() != nil {
		n.PackagePath = t.Obj().Pkg().Path()
	}
	if _, ok := t.Underlying().(*types.Interface); ok {
		return fmt.Errorf("%s: unsupported interface type %s", where, t)
	}
	if !t.Obj().Exported() {
		return fmt.Errorf("%s: non-exported named type %s is not supported", where, t)
	}
	if t.TypeParams().Len() != 0 || t.TypeArgs().Len() != 0 {
		return fmt.Errorf("%s: generic type %s is not supported", where, t)
	}
	if basic, ok := t.Underlying().(*types.Basic); ok && basic.Kind() == types.Bool {
		return fmt.Errorf("%s: named bool %s has no defined Python mapping", where, t)
	}

	if structure, ok := t.Underlying().(*types.Struct); ok {
		if err := b.fields(n, structure, where); err != nil {
			return err
		}
		if err := b.selections(n, t, where); err != nil {
			return err
		}
	} else {
		underlying, err := b.visit(t.Underlying(), where+" -> underlying type")
		if err != nil {
			return err
		}
		n.Underlying = underlying
	}

	var err error
	if n.ValueMethods, err = b.methods(t, t, where); err != nil {
		return err
	}
	n.PointerMethods, err = b.methods(t, types.NewPointer(t), where)
	return err
}

func (b *builder) fields(n *Type, structure *types.Struct, where string) error {
	for i := 0; i < structure.NumFields(); i++ {
		field := structure.Field(i)
		if !field.Exported() {
			continue
		}
		ft, err := b.visit(field.Type(), where+" -> field "+field.Name())
		if err != nil {
			return err
		}
		n.Fields = append(n.Fields, Field{Name: field.Name(), Type: ft, Embedded: field.Embedded(), Index: i})
	}
	return nil
}

// methods collects the exported methods in the method set of selected, which
// is owner or *owner. Method sets include promoted methods and follow Go's
// ambiguity rules, so they are queried rather than derived from declarations.
func (b *builder) methods(owner *types.Named, selected types.Type, where string) ([]*Function, error) {
	var result []*Function
	set := types.NewMethodSet(selected)
	for i := 0; i < set.Len(); i++ {
		method := set.At(i).Obj().(*types.Func)
		if !method.Exported() {
			continue
		}
		context := where + " -> method " + method.Name()
		// Only a receiver declared on owner itself introduces a dependency
		// (for example *owner for a pointer method). A promoted method on a
		// hidden embedded type is still callable through the public owner,
		// without exposing that hidden type.
		declared := types.Unalias(method.Signature().Recv().Type())
		if types.Identical(declared, owner) || types.Identical(declared, types.NewPointer(owner)) {
			if _, err := b.visit(declared, where+" -> receiver of "+method.Name()); err != nil {
				return nil, err
			}
		}
		fn, err := b.function(method, selected, context)
		if err != nil {
			return nil, err
		}
		result = append(result, fn)
	}
	return result, nil
}

// function describes obj's signature. selected is the type whose method set
// produced obj, or nil for a package-level function.
func (b *builder) function(obj *types.Func, selected types.Type, where string) (*Function, error) {
	sig := obj.Signature()
	if sig.TypeParams().Len() != 0 {
		return nil, fmt.Errorf("%s: generic function is not supported", where)
	}
	if sig.Variadic() {
		return nil, fmt.Errorf("%s: variadic function is not supported", where)
	}
	fn := &Function{Name: obj.Name(), SelectedReceiver: selected}
	if sig.Recv() != nil {
		fn.Receiver = types.Unalias(sig.Recv().Type())
		_, fn.PointerReceiver = fn.Receiver.(*types.Pointer)
	}
	var err error
	if fn.Params, err = b.tuple(sig.Params(), "parameter", where); err != nil {
		return nil, err
	}
	if fn.Results, err = b.tuple(sig.Results(), "result", where); err != nil {
		return nil, err
	}
	return fn, nil
}

func (b *builder) tuple(tuple *types.Tuple, label, where string) ([]Parameter, error) {
	var result []Parameter
	for i := 0; i < tuple.Len(); i++ {
		v := tuple.At(i)
		t, err := b.visit(v.Type(), fmt.Sprintf("%s -> %s %d (%s)", where, label, i+1, v.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, Parameter{Name: v.Name(), Type: t})
	}
	return result, nil
}
