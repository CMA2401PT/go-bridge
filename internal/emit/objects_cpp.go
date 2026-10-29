package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
	"go-bridge/internal/pyapi"
)

func classVar(t *abi.Type) string { return "class" + t.Cpp }

func cppBase(t *abi.Type) string {
	switch t.Python.Shape {
	case pyapi.SliceObject:
		return "Slice"
	case pyapi.PointerObject:
		return "Ptr"
	default:
		return "Ref"
	}
}

// cppClassTypes registers all concrete classes before attaching any member,
// so recursive types and constructor defaults do not depend on graph order.
func cppClassTypes(b *strings.Builder, p *abi.Package) {
	for _, c := range p.Classes {
		t := c.Type
		if t.Python.Shape == pyapi.NamedValue {
			cppNamedValue(b, c)
			continue
		}
		fmt.Fprintf(b, "auto %s = py::class_<%s, %s>(module, %q, py::is_final());\n", classVar(t), t.Cpp, cppBase(t), t.Python.Name)
	}
}

func cppNamedValue(b *strings.Builder, c *abi.Class) {
	t, base := c.Type, c.Underlying
	cls := classVar(t)
	fmt.Fprintf(b, `py::object %[1]s;
{
    auto base = py::module_::import("builtins").attr(%[2]q);
    py::dict body;
    body["__module__"] = module.attr("__name__");
    body["__slots__"] = py::tuple();
    %[1]s = py::module_::import("builtins").attr("type")(%[3]q, py::make_tuple(base), body);
    %[1]s.attr("__new__") = py::staticmethod(py::cpp_function([base](py::object cls, py::object value) {
        auto raw = %[4]s(value);
        return base.attr("__new__")(cls, %[5]s);
    }, py::arg("cls"), py::arg("value") = %[6]s));
    %[1]s.attr("__init_subclass__") = py::module_::import("builtins").attr("classmethod")(py::cpp_function([](py::object) {
        throw py::type_error("concrete Go types cannot be subclassed");
    }));
    module.attr(%[3]q) = %[1]s;
}
auto read%[7]s = [type = py::weakref(%[1]s)](py::handle value) {
    if (!py::isinstance(value, type())) throw py::type_error("expected %[3]s");
    return %[4]s(value);
};
%[1]s.attr("unname") = py::cpp_function([=](py::handle self) {
    auto raw = read%[7]s(self);
    return %[5]s;
}, py::is_method(%[1]s));
`, cls, base.Python.Name, t.Python.Name, cppPrimitiveInput(base), cppPrimitiveOutput(base, "raw"), cppZero(base), t.Cpp)
}

func cppZero(t *abi.Type) string {
	if t.Python.Shape == pyapi.NamedValue {
		return classVar(t) + "(" + cppZeroValue(t) + ")"
	}
	return cppZeroValue(t)
}

func cppZeroValue(t *abi.Type) string {
	switch t.Kind {
	case abi.String:
		return "py::str()"
	case abi.Float:
		return "py::float_(0.0)"
	case abi.Boolean:
		return "py::bool_(false)"
	default:
		return "py::int_(0)"
	}
}

func cppClass(b *strings.Builder, c *abi.Class) {
	t, cls := c.Type, classVar(c.Type)
	if t.Python.Shape == pyapi.CallableObject {
		cppNilMembers(b, t, "Go functions")
	}
	if t.Python.Shape == pyapi.AnyObject {
		cppNilMembers(b, t, "Go any values")
	}
	if t.Python.Shape == pyapi.StructObject {
		cppStructConstructor(b, c)
	}
	if c.New != "" && t.Python.Shape != pyapi.StructObject {
		cppValueConstructor(b, c)
	}
	for _, op := range []struct {
		name, symbol string
		result       *abi.Type
	}{
		{"__copy__", c.Copy, t}, {"addr", c.Address, c.Pointer}, {"ptr", c.ToPointer, c.Pointer}, {"unname", c.Unname, c.Underlying},
	} {
		if op.symbol == "" {
			continue
		}
		fmt.Fprintf(b, "%s.attr(%q) = py::cpp_function([=](py::handle self) -> py::object {\nauto input = %s(self);\n", cls, op.name, cppInput(t))
		cppOwnedCall(b, op.result, op.symbol+"(input)")
		fmt.Fprintf(b, "}, py::is_method(%s));\n", cls)
	}
	if t.Python.Shape == pyapi.PointerObject {
		cppPointerAccess(b, c)
	}
	for _, f := range c.Fields {
		cppField(b, c, f)
	}
}

// cppOwnedCall takes ownership before any Python allocation/conversion.
func cppOwnedCall(b *strings.Builder, t *abi.Type, call string) {
	if t.Kind == abi.Handle || t.Kind == abi.String {
		fmt.Fprintf(b, "OwnedBuffer<%s, %s> owner(1);\nowner.values[0] = %s;\nreturn %s;\n", t.C, cppDeleter(t), call, cppOutput(t, "owner.values[0]"))
	} else {
		fmt.Fprintf(b, "auto result = %s;\nreturn %s;\n", call, cppOutput(t, "result"))
	}
}

func cppStructConstructor(b *strings.Builder, c *abi.Class) {
	t := c.Type
	fmt.Fprintf(b, "%s.def(py::init([=](py::kwargs fields) {\n", classVar(t))
	b.WriteString("for (auto field : fields) {\nauto key = py::cast<std::string>(field.first);\nif (")
	if len(c.Constructor) == 0 {
		b.WriteString("true")
	} else {
		for i, f := range c.Constructor {
			if i > 0 {
				b.WriteString(" && ")
			}
			fmt.Fprintf(b, "key != %q", f.Python.Name)
		}
	}
	b.WriteString(") throw py::type_error(\"unknown struct field: \" + key);\n}\n")
	for i, f := range c.Constructor {
		fmt.Fprintf(b, "%s v%d{};\nuint8_t p%d = fields.contains(%q);\nif (p%d) v%d = %s(fields[%q]);\n", f.Type.C, i, i, f.Python.Name, i, i, cppInput(f.Type), f.Python.Name)
	}
	args := join(len(c.Constructor), func(i int) string { return fmt.Sprintf("v%d, p%d", i, i) })
	fmt.Fprintf(b, "return %s(%s(%s));\n}));\n", t.Cpp, c.New, args)
}

func cppValueConstructor(b *strings.Builder, c *abi.Class) {
	input := c.Elem
	if c.Underlying != nil {
		input = c.Underlying
	}
	cls := classVar(c.Type)
	parameter := "value"
	if c.Type.Python.Shape == pyapi.SliceObject {
		parameter = "values"
	}
	fmt.Fprintf(b, "%s.def(py::init([=](py::handle value) {\nauto input = %s(value);\nreturn %s(%s(input));\n}), py::arg(%q)", cls, cppInput(input), c.Type.Cpp, c.New, parameter)
	if c.Underlying == nil && (input.Python.Shape == pyapi.Builtin || input.Python.Shape == pyapi.NamedValue) {
		fmt.Fprintf(b, " = %s", cppZero(input))
	}
	b.WriteString(");\n")
	if c.Underlying == nil {
		fmt.Fprintf(b, "%s.def_static(\"from_value\", [=](py::handle value) {\nauto input = %s(value);\nreturn %s(%s(input));\n}, py::arg(\"value\"));\n", cls, cppInput(input), c.Type.Cpp, c.New)
	}
}

func cppPointerAccess(b *strings.Builder, c *abi.Class) {
	t, cls := c.Type, classVar(c.Type)
	cppNilMembers(b, t, "Go pointers")
	fmt.Fprintf(b, `%[1]s.def_property("value", [=](py::handle self) -> py::object {
    auto input = handle_arg<%[2]s>(self);
`, cls, t.Cpp)
	cppOwnedCall(b, c.Elem, c.Load+"(input)")
	fmt.Fprintf(b, "}, [=](py::handle self, py::handle value) {\nauto input = handle_arg<%s>(self);\nauto v = %s(value);\n%s(input, v);\n});\n", t.Cpp, cppInput(c.Elem), c.Store)
}

func cppField(b *strings.Builder, c *abi.Class, f *abi.Field) {
	fmt.Fprintf(b, "%s.def_property(%q, [=](py::handle self) -> py::object {\nauto input = %s(self);\n", classVar(c.Type), f.Python.Name, cppInput(c.Type))
	cppOwnedCall(b, f.Type, f.Get+"(input)")
	fmt.Fprintf(b, "}, [=](py::handle self, py::handle value) {\nauto input = %s(self);\nauto v = %s(value);\n%s(input, v);\n});\n", cppInput(c.Type), cppInput(f.Type), f.Set)
}

// cppNilMembers binds handle 0 as nil for pointers and any. Truth testing is
// rejected so a nil check is never confused with a zero or empty value.
func cppNilMembers(b *strings.Builder, t *abi.Type, what string) {
	fmt.Fprintf(b, `%[1]s.def_static("nil", [] { return %[2]s(0); });
%[1]s.def_property_readonly("is_nil", [](py::handle self) { return handle_arg<%[2]s>(self) == 0; });
%[1]s.def("__bool__", [](py::handle self) -> bool {
    checked_ref<%[2]s>(self);
    throw py::type_error("%[3]s have no truth value; use is_nil");
});
`, classVar(t), t.Cpp, what)
}
