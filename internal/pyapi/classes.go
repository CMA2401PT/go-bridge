package pyapi

import (
	"fmt"
	"go/types"
	"strings"

	"go-bridge/internal/gomodel"
)

// Shape describes the Python object's semantics, independently of its Go name.
type Shape string

const (
	Builtin         Shape = "builtin"
	NamedValue      Shape = "named-value"
	SliceObject     Shape = "slice"
	StructObject    Shape = "struct"
	PointerObject   Shape = "pointer"
	AnyObject       Shape = "any"
	ChannelObject   Shape = "channel"
	MapObject       Shape = "map"
	ArrayObject     Shape = "array"
	CallableObject  Shape = "callable"
	InterfaceObject Shape = "interface"
)

// Field contains a visible Go selector and its single shared Python spelling.
// Direct fields also appear as optional constructor keywords.
type Field struct {
	Go   gomodel.Selection
	Name string
	Type *Type
}

func shape(t *gomodel.Type) Shape {
	switch t.Kind {
	case gomodel.Basic:
		return Builtin
	case gomodel.Slice:
		return SliceObject
	case gomodel.Signature:
		return CallableObject
	case gomodel.Array:
		return ArrayObject
	case gomodel.Map:
		return MapObject
	case gomodel.Channel:
		return ChannelObject
	case gomodel.Pointer:
		return PointerObject
	case gomodel.Interface:
		if t.Go.Underlying().(*types.Interface).Empty() {
			return AnyObject
		}
		return InterfaceObject
	case gomodel.Named:
		if _, ok := t.Go.Underlying().(*types.Interface); ok {
			return InterfaceObject
		}
		if t.Underlying == nil {
			return StructObject
		}
		switch t.Underlying.Kind {
		case gomodel.Basic:
			return NamedValue
		case gomodel.Slice:
			return SliceObject
		case gomodel.Signature:
			return CallableObject
		case gomodel.Array:
			return ArrayObject
		case gomodel.Map:
			return MapObject
		case gomodel.Channel:
			return ChannelObject
		case gomodel.Pointer:
			return PointerObject
		}
	}
	panic("unsupported Go model kind")
}

// planClasses resolves shape-specific members once, before either backend runs.
func planClasses(p *Package, mapped map[*gomodel.Type]*Type) error {
	for _, t := range p.Types {
		t.Shape = shape(t.Go)
		t.Underlying = mapped[t.Go.Underlying]
		base := t.Go
		if base.Underlying != nil {
			base = base.Underlying
		}
		t.Elem = mapped[base.Elem]
		t.Key = mapped[base.Key]
		t.Length = base.Length
		if base.Call != nil {
			call, err := planFunction(base.Call, mapped, t.Go.String())
			if err != nil {
				return err
			}
			call.Name = "__call__"
			t.Call = call
		}
		if t.Shape == ChannelObject {
			t.CanSend = base.Direction != types.RecvOnly
			t.CanRecv = base.Direction != types.SendOnly
			if base.Direction == types.SendRecv {
				t.SendOnly = mapped[p.Go.Find(types.NewChan(types.SendOnly, base.Elem.Go))]
				t.RecvOnly = mapped[p.Go.Find(types.NewChan(types.RecvOnly, base.Elem.Go))]
			}
		}
		t.ByteSlice = t.Shape == SliceObject && t.Elem.Go.Go == types.Typ[types.Uint8]
		t.Pointer = mapped[p.Go.Find(types.NewPointer(t.Go.Go))]
	}
	for _, t := range p.Types {
		if t.Shape == StructObject {
			for _, f := range t.Go.Selectable {
				t.Fields = append(t.Fields, &Field{Go: f, Name: memberName(t, f.Name), Type: mapped[f.Type]})
			}
			for _, f := range t.Go.Fields {
				for _, selected := range t.Fields {
					if selected.Go.Name == f.Name {
						t.Constructor = append(t.Constructor, selected)
						break
					}
				}
			}
		}
	}
	for _, t := range p.Types {
		var methods []*gomodel.Function
		switch t.Shape {
		case StructObject, ArrayObject:
			methods = t.Go.PointerMethods
		case NamedValue, SliceObject, ChannelObject, MapObject, CallableObject, InterfaceObject:
			methods = t.Go.ValueMethods
		case PointerObject:
			if t.Go.Kind == gomodel.Pointer {
				methods = t.Elem.Go.PointerMethods
			}
			if t.Elem.Shape == StructObject {
				t.Fields = t.Elem.Fields
			}
		}
		names := make(namespace)
		for _, field := range t.Fields {
			if err := names.claim("member", field.Name, t.Go.String()+"."+field.Go.Name); err != nil {
				return err
			}
		}
		for _, method := range methods {
			planned, err := planFunction(method, mapped, t.Go.String()+"."+method.Name)
			if err != nil {
				return err
			}
			planned.Name = memberName(t, method.Name)
			if err := names.claim("member", planned.Name, t.Go.String()+"."+method.Name); err != nil {
				return err
			}
			t.Methods = append(t.Methods, planned)
		}
	}
	return nil
}

// T and *T share one spelling, including bridge-reserved member renaming.
var bridgeMembers = map[string]bool{
	"value": true, "is_nil": true, "nil": true, "from_value": true, "addr": true, "ptr": true,
	"clone": true, "copy_to_py_list": true, "copy_to_py_bytes": true, "unname": true,
	"send": true, "recv": true, "close": true, "send_only": true, "recv_only": true,
	"delete": true, "clear": true, "copy_to_py_items": true,
	"as_any": true, "from_any": true, "try_from_any": true,
}

func memberName(owner *Type, name string) string {
	name = Snake(name)
	if bridgeMembers[name] || builtinMember(owner, name) {
		name += "_"
	}
	return name
}

// className gives named Go types their public name, and derives only unnamed
// slice/pointer names. Distinct types with equal spellings are rejected later.
func className(t *gomodel.Type) (string, error) {
	if t.Kind == gomodel.Interface {
		if t.Go.Underlying().(*types.Interface).Empty() {
			return "GoAny", nil
		}
		var name strings.Builder
		name.WriteString("Interface")
		for _, m := range t.ValueMethods {
			name.WriteString(m.Name)
			for _, p := range m.Params {
				stem, err := typeStem(p.Type)
				if err != nil {
					return "", err
				}
				name.WriteString(stem)
			}
			name.WriteString("To")
			if len(m.Results) == 0 {
				name.WriteString("Void")
			}
			for _, r := range m.Results {
				stem, err := typeStem(r.Type)
				if err != nil {
					return "", err
				}
				name.WriteString(stem)
			}
		}
		return name.String(), nil
	}
	if t.Kind == gomodel.Named {
		return avoidKeyword(t.Name), nil
	}
	if t.Kind == gomodel.Signature {
		parts := []string{"Func"}
		for _, tuple := range [][]gomodel.Parameter{t.Call.Params, t.Call.Results} {
			if len(tuple) == 0 {
				parts = append(parts, "Void")
			}
			for i, p := range tuple {
				if i > 0 {
					parts = append(parts, "And")
				}
				stem, err := typeStem(p.Type)
				if err != nil {
					return "", err
				}
				parts = append(parts, stem)
			}
			parts = append(parts, "To")
		}
		return strings.Join(parts[:len(parts)-1], ""), nil
	}
	if t.Kind == gomodel.Array {
		elem, err := typeStem(t.Elem)
		return fmt.Sprintf("Array%d%s", t.Length, elem), err
	}
	if t.Kind == gomodel.Map {
		key, err := typeStem(t.Key)
		if err != nil {
			return "", err
		}
		elem, err := typeStem(t.Elem)
		return "Map" + key + elem, err
	}
	if t.Kind == gomodel.Channel {
		elem, err := typeStem(t.Elem)
		prefix := "Chan"
		if t.Direction == types.SendOnly {
			prefix = "SendChan"
		}
		if t.Direction == types.RecvOnly {
			prefix = "RecvChan"
		}
		return prefix + elem, err
	}
	if t.Kind == gomodel.Pointer {
		elem, err := typeStem(t.Elem)
		return "Ptr" + elem, err
	}
	if t.Kind == gomodel.Slice {
		if t.Elem.Go == types.Typ[types.Uint8] {
			return "SliceBytes", nil
		}
		elem, err := typeStem(t.Elem)
		return "Slice" + elem, err
	}
	if t.Kind == gomodel.Basic {
		return basicName(t)
	}
	return "", fmt.Errorf("no Python mapping for %s", t)
}

// Named scalar methods must not replace the Python builtin's own protocol.
// Apply the same spelling to *T so a Go method keeps its name across receivers.
func builtinMember(owner *Type, name string) bool {
	if owner.Shape == PointerObject {
		owner = owner.Elem
	}
	if owner.Shape != NamedValue {
		return false
	}
	names := builtinMembers[owner.Underlying.Name]
	return strings.Contains(" "+names+" ", " "+name+" ")
}

var builtinMembers = map[string]string{
	"int":   "as_integer_ratio bit_count bit_length conjugate denominator from_bytes imag is_integer numerator real to_bytes from_number",
	"float": "as_integer_ratio conjugate fromhex hex imag is_integer real from_number",
	"str":   "capitalize casefold center count encode endswith expandtabs find format format_map index isalnum isalpha isascii isdecimal isdigit isidentifier islower isnumeric isprintable isspace istitle isupper join ljust lower lstrip maketrans partition removeprefix removesuffix replace rfind rindex rjust rpartition rsplit rstrip split splitlines startswith strip swapcase title translate upper zfill",
}

// Variable reports wrappers whose contents occupy addressable Go storage.
func (t *Type) Variable() bool { return t.Shape == StructObject || t.Shape == ArrayObject }
