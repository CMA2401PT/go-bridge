package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/pyapi"
)

func stubClasses(b *strings.Builder, p *pyapi.Package) {
	if hasRefs(p) {
		b.WriteString(`class Ref:
    def __new__(cls, _handle: _typing.Never) -> _typing.Self: ...
_Elem = _typing.TypeVar("_Elem")
class Slice(Ref, _typing.Generic[_Elem]):
    __iter__: _typing.ClassVar[None]
    __reversed__: _typing.ClassVar[None]
    def __len__(self) -> _builtins.int: ...
    @_typing.overload
    def __getitem__(self, index: _typing.SupportsIndex) -> _Elem: ...
    @_typing.overload
    def __getitem__(self, index: _builtins.slice) -> _typing.Self: ...
    def __setitem__(self, index: _typing.SupportsIndex, value: _Elem) -> None: ...
    def clone(self) -> _typing.Self: ...
    def copy_to_py_list(self) -> _builtins.list[_Elem]: ...
_Target = _typing.TypeVar("_Target")
class Ptr(Ref, _typing.Generic[_Target]):
    value: _Target
    @_builtins.classmethod
    def nil(cls) -> _typing.Self: ...
    @_builtins.property
    def is_nil(self) -> _builtins.bool: ...
    def __bool__(self) -> _typing.Never: ...
`)
	}
	conversions := make(map[*pyapi.Type]*pyapi.AnyConversion)
	for _, c := range p.AnyConversions {
		conversions[c.Type] = c
	}
	for _, t := range p.Types {
		if t.Shape == pyapi.Builtin {
			continue
		}
		base := "Ref"
		switch t.Shape {
		case pyapi.NamedValue:
			base = annotation(t.Underlying)
		case pyapi.SliceObject:
			base = fmt.Sprintf("Slice[%q]", annotation(t.Elem))
		case pyapi.PointerObject:
			base = fmt.Sprintf("Ptr[%q]", annotation(t.Elem))
		}
		fmt.Fprintf(b, "@_typing.final\nclass %s(%s):\n", t.Name, base)
		if t.Shape == pyapi.AnyObject || t.Shape == pyapi.ChannelObject {
			b.WriteString("    @_builtins.classmethod\n    def nil(cls) -> _typing.Self: ...\n    @_builtins.property\n    def is_nil(self) -> _builtins.bool: ...\n    def __bool__(self) -> _typing.Never: ...\n")
		} else if t.Shape == pyapi.ArrayObject {
			stubArray(b, t)
		} else if t.Shape == pyapi.MapObject {
			stubMap(b, t)
		} else {
			stubConstructor(b, t)
		}
		if t.Shape == pyapi.ChannelObject {
			stubChannel(b, t)
		}
		if c := conversions[t]; c != nil {
			fmt.Fprintf(b, "    def %[1]s(self) -> %[4]s: ...\n    @_builtins.classmethod\n    def %[2]s(cls, value: %[4]s) -> _typing.Self: ...\n    @_builtins.classmethod\n    def %[3]s(cls, value: %[4]s) -> _typing.Self | None: ...\n", c.Box, c.Assert, c.TryAssert, p.Any.Name)
		}
		if t.Underlying != nil {
			fmt.Fprintf(b, "    def unname(self) -> %s: ...\n", annotation(t.Underlying))
		}
		if t.Variable() {
			fmt.Fprintf(b, "    def __copy__(self) -> %s: ...\n    def addr(self) -> %s: ...\n", t.Name, t.Pointer.Name)
		} else if t.Pointer != nil {
			fmt.Fprintf(b, "    def ptr(self) -> %s: ...\n", t.Pointer.Name)
		}
		if t.Shape == pyapi.SliceObject && t.ByteSlice {
			b.WriteString("    def copy_to_py_bytes(self) -> _builtins.bytes: ...\n")
		}
		for _, f := range t.Fields {
			fmt.Fprintf(b, "    %s: %s\n", f.Name, annotation(f.Type))
		}
		for _, m := range t.Methods {
			params := "self"
			for _, arg := range m.Params {
				params += ", " + arg.Name + ": " + annotation(arg.Type)
			}
			fmt.Fprintf(b, "    def %s(%s) -> %s: ...\n", m.Name, params, returnAnnotation(m))
		}
	}
}

func stubConstructor(b *strings.Builder, t *pyapi.Type) {
	params := ""
	switch t.Shape {
	case pyapi.StructObject:
		if len(t.Constructor) > 0 {
			params = ", *"
		}
		for _, f := range t.Constructor {
			params += ", " + f.Name + ": " + annotation(f.Type) + " = ..."
		}
	case pyapi.NamedValue:
		params = ", value: " + annotation(t.Underlying) + " = ..."
	case pyapi.SliceObject:
		input := "_typing.Iterable[" + annotation(t.Elem) + "]"
		if t.ByteSlice {
			input = "_builtins.bytes"
		}
		if t.Underlying != nil {
			input = annotation(t.Underlying)
		}
		params = ", values: " + input
	case pyapi.PointerObject:
		input := t.Elem
		if t.Underlying != nil {
			input = t.Underlying
		}
		params = ", value: " + annotation(input)
		if t.Underlying == nil && (input.Shape == pyapi.Builtin || input.Shape == pyapi.NamedValue) {
			params += " = ..."
		}
	}
	// Exported Go fields cannot start with an underscore. Keep implicit
	// constructor receivers outside their keyword namespace (Self/Cls).
	fmt.Fprintf(b, "    def __new__(_cls%s) -> _typing.Self: ...\n    def __init__(_self%s) -> None: ...\n", params, params)
	if t.Shape == pyapi.PointerObject && t.Underlying == nil {
		fmt.Fprintf(b, "    @_builtins.classmethod\n    def from_value(cls, value: %s) -> _typing.Self: ...\n", annotation(t.Elem))
	}
}
