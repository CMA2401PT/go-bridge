package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
	"go-bridge/internal/pyapi"
)

// cppAnyConversion binds boxing and both assertion forms. Builtins become
// module functions; other types get members on their class. The strict and
// try forms share one Go symbol and differ only in how !ok is reported.
func cppAnyConversion(b *strings.Builder, anyType *abi.Type, c *abi.AnyConversion) {
	t, names := c.Type, c.Python
	builtin := t.Python.Shape == pyapi.Builtin
	cls := classVar(t)
	if builtin {
		fmt.Fprintf(b, "module.def(%q, [=](py::handle value) -> py::object {\nauto input = %s(value);\n", names.Box, cppInput(t))
	} else {
		fmt.Fprintf(b, "%s.attr(%q) = py::cpp_function([=](py::handle self) -> py::object {\nauto input = %s(self);\n", cls, names.Box, cppInput(t))
	}
	cppOwnedCall(b, anyType, c.Box+"(input)")
	if builtin {
		b.WriteString("}, py::arg(\"value\"));\n")
	} else {
		fmt.Fprintf(b, "}, py::is_method(%s));\n", cls)
	}
	output := cppOutput(t, "owner.values[0]")
	if t.Python.Shape == pyapi.NamedValue {
		fmt.Fprintf(b, "auto convertAny%s = [type = py::weakref(%s)](%s value) -> py::object { return type()(%s); };\n", t.Cpp, cls, t.C, cppPrimitiveOutput(t, "value"))
		output = "convertAny" + t.Cpp + "(owner.values[0])"
	}
	for _, form := range []struct {
		name     string
		optional bool
	}{{names.Assert, false}, {names.TryAssert, true}} {
		if builtin {
			fmt.Fprintf(b, "module.def(%q, [=](py::handle value) -> py::object {\n", form.name)
		} else {
			fmt.Fprintf(b, "%s.attr(%q) = py::staticmethod(py::cpp_function([=](py::handle value) -> py::object {\n", cls, form.name)
		}
		fmt.Fprintf(b, "auto input = %s(value);\nOwnedBuffer<%s, %s> owner(1);\nauto result = %s(input);\nowner.values[0] = result.value;\nif (!result.ok) {\n", cppInput(anyType), t.C, cppDeleter(t), c.Assert)
		if form.optional {
			b.WriteString("return py::none();\n")
		} else {
			fmt.Fprintf(b, "throw py::type_error(%q);\n", "Go any does not contain "+t.Python.Go.String())
		}
		fmt.Fprintf(b, "}\nreturn %s;\n}, py::arg(\"value\"))", output)
		if !builtin {
			b.WriteString(")")
		}
		b.WriteString(";\n")
	}
}
