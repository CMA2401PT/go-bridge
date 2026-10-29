package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
	"go-bridge/internal/pyapi"
)

func cppInterfaceBase(b *strings.Builder, anyType *abi.Type) {
	b.WriteString(`auto interface_base=py::class_<InterfaceRef,Ref>(module,"GoInterface");
interface_base.def_property_readonly("is_nil",[](py::handle self){return handle_arg<InterfaceRef>(self)==0;});
interface_base.def("__bool__",[](py::handle self)->bool{
 checked_ref<InterfaceRef>(self);
 throw py::type_error("Go interfaces have no truth value; use is_nil");
});
interface_base.def("as_any",[](py::handle self)->py::object {
 auto input=handle_arg<InterfaceRef>(self);
`)
	cppOwnedCall(b, anyType, abi.InterfaceBoxSymbol+"(input)")
	b.WriteString("});\n")
}

func cppInterfaceConstructors(b *strings.Builder, p *abi.Package) {
	for _, c := range p.Classes {
		t := c.Type
		if len(t.Python.Implementors) == 0 {
			continue
		}
		fmt.Fprintf(b, "%s.def(py::init([=](py::handle value) {\n", classVar(t))
		for _, conversion := range p.InterfaceConversions {
			if conversion.Target != t {
				continue
			}
			source := conversion.Source
			check := "py::isinstance<" + source.Cpp + ">(value)"
			if source.Python.Shape == pyapi.NamedValue {
				check = "py::isinstance(value, " + classVar(source) + ")"
			}
			fmt.Fprintf(b, "if (%s) {auto input = %s(value);return %s(%s(input));}\n", check, cppInput(source), t.Cpp, conversion.Symbol)
		}
		fmt.Fprintf(b, "throw py::type_error(%q);\n}),py::arg(\"value\"));\n", "expected a Go wrapper statically assignable to "+t.Python.Name)
	}
}
