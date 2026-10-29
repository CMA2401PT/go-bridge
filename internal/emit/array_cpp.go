package emit

import (
	"fmt"
	"go-bridge/internal/abi"
	"strings"
)

func cppArray(b *strings.Builder, a *abi.Array) {
	t, e := a.Type, a.Elem
	cls := classVar(t)
	if a.New != "" {
		fmt.Fprintf(b, `%[1]s.def(py::init([] {return %[2]s(%[3]s());}));
%[1]s.def(py::init([=](py::iterable values) {
 std::vector<py::object> keep;
 std::vector<%[4]s> input;
 for (py::handle value:values) {
  keep.push_back(py::reinterpret_borrow<py::object>(value));
  input.push_back(%[5]s(value));
 }
 if (input.size()!=%[6]d) throw py::value_error("expected exactly %[6]d elements");
 return %[2]s(%[7]s(input.data()));
}),py::arg("values"));
`, cls, t.Cpp, a.Zero, e.C, cppInput(e), a.Length, a.New)
	}
	fmt.Fprintf(b, `%[1]s.attr("__iter__")=py::none();
%[1]s.def("__len__",[](const %[2]s&) {return %[3]d;});
%[1]s.def("__getitem__",[=](py::handle self,py::handle index) -> py::object {
 auto handle=handle_arg<%[2]s>(self);
 auto i=slice_index(index,%[3]d);
`, cls, t.Cpp, a.Length)
	cppOwnedCall(b, e, a.At+"(handle,i)")
	b.WriteString("});\n")
	fmt.Fprintf(b, `%[1]s.def("__setitem__",[=](py::handle self,py::handle index,py::handle value) {
 auto handle=handle_arg<%[2]s>(self);
 auto i=slice_index(index,%[3]d);
 auto v=%[4]s(value);
 %[5]s(handle,i,v);
});
%[1]s.def("copy_to_py_list",[=](py::handle self) {
 auto handle=handle_arg<%[2]s>(self);
 OwnedBuffer<%[6]s,%[7]s> owner(%[3]d);
 %[8]s(handle,owner.values.data());
 py::list result(%[3]d);
 for (size_t i=0;i<%[3]d;++i) result[i]=%[9]s;
 return result;
});
`, cls, t.Cpp, a.Length, cppInput(e), a.Set, e.C, cppDeleter(e), a.Copy, cppOutput(e, "owner.values[i]"))
}
