package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
)

func cppRefClasses(b *strings.Builder) {
	b.WriteString(`
py::class_<Ref>(module, "Ref");
auto slice_base = py::class_<Slice, Ref>(module, "Slice");
auto ptr_base = py::class_<Ptr, Ref>(module, "Ptr");
ptr_base.attr("__class_getitem__") = py::module_::import("builtins").attr("classmethod")(
    py::cpp_function([](py::object cls, py::object) {return cls;}));
slice_base.attr("__iter__") = py::none();
slice_base.attr("__reversed__") = py::none();
slice_base.attr("__class_getitem__") = py::module_::import("builtins").attr("classmethod")(
    py::cpp_function([=](py::object cls, py::object) { return cls; }));
`)
	fmt.Fprintf(b, "module.def(\"_holder_count\", [] { return %s(); });\nmodule.def(\"_collect_go\", [] { %s(); });\n", abi.CountSymbol, abi.CollectSymbol)
}

func byteSlice(s *abi.Slice) bool { return s.Type.Python.ByteSlice }

func cppSlice(b *strings.Builder, s *abi.Slice) {
	t, e := s.Type, s.Elem
	cls := classVar(t)
	if s.New != "" {
		cppSliceConstructor(b, s, cls)
	}
	fmt.Fprintf(b, `%[1]s.def("__len__", [=](py::handle object) { const auto& self = checked_ref<%[2]s>(object); return %[3]s(self.handle); });
%[1]s.def("clone", [=](py::handle object) { const auto& self = checked_ref<%[2]s>(object); return %[2]s(%[4]s(self.handle)); });
%[1]s.def("__getitem__", [=](py::handle object, py::object index) -> py::object {
    const auto& self = checked_ref<%[2]s>(object);
    size_t size = %[3]s(self.handle);
    if (py::isinstance<py::slice>(index)) {
        Py_ssize_t start, stop, step, count;
        if (!py::cast<py::slice>(index).compute(static_cast<Py_ssize_t>(size), &start, &stop, &step, &count)) throw py::error_already_set();
        if (step != 1) throw py::value_error("Go slices do not have a step");
        return py::cast(%[2]s(%[5]s(self.handle, start, start + count)));
    }
    OwnedBuffer<%[6]s, %[7]s> owned(1);
    owned.values[0] = %[8]s(self.handle, slice_index(index, size));
    return %[9]s;
});
%[1]s.def("__setitem__", [=](py::handle object, py::object index, py::object value) {
    const auto& self = checked_ref<%[2]s>(object);
    size_t i = slice_index(index, %[3]s(self.handle));
    auto v = %[10]s(value);
    %[11]s(self.handle, i, v);
});
`, cls, t.Cpp, s.Len, s.Clone, s.Reslice, e.C, cppDeleter(e), s.At, cppOutput(e, "owned.values[0]"), cppInput(e), s.Set)
	cppSliceCopy(b, s, cls)
}

func cppSliceConstructor(b *strings.Builder, s *abi.Slice, cls string) {
	t, e := s.Type, s.Elem
	fmt.Fprintf(b, "%s.def(py::init([=](py::object values) {\n", cls)
	if byteSlice(s) {
		b.WriteString("if (!PyBytes_Check(values.ptr())) throw py::type_error(\"expected bytes\");\n")
		fmt.Fprintf(b, "return %s(%s(reinterpret_cast<%s*>(PyBytes_AS_STRING(values.ptr())), static_cast<size_t>(PyBytes_GET_SIZE(values.ptr()))));\n", t.Cpp, s.New, e.C)
	} else {
		// Keep the iterable's objects alive until Go has copied their values.
		fmt.Fprintf(b, "std::vector<%s> input;\nstd::vector<py::object> keep;\n", e.C)
		b.WriteString("for (py::handle item : py::reinterpret_borrow<py::iterable>(values)) {\nkeep.push_back(py::reinterpret_borrow<py::object>(item));\n")
		fmt.Fprintf(b, "input.push_back(%s(item));\n}\nreturn %s(%s(input.data(), input.size()));\n", cppInput(e), t.Cpp, s.New)
	}
	b.WriteString("}), py::arg(\"values\"));\n")
}

func cppDeleter(t *abi.Type) string {
	switch t.Kind {
	case abi.String:
		return "release_string"
	case abi.Handle:
		return "release_handle"
	default:
		return "release_value<" + t.C + ">"
	}
}

func cppSliceCopy(b *strings.Builder, s *abi.Slice, cls string) {
	t, e := s.Type, s.Elem
	fmt.Fprintf(b, `%s.def("copy_to_py_list", [=](py::handle object) {
    const auto& self = checked_ref<%s>(object);
    size_t size = %s(self.handle);
    OwnedBuffer<%s, %s> owned(size);
    %s(self.handle, owned.values.data());
    py::list output(size);
    for (size_t i = 0; i < size; ++i) output[i] = %s;
    return output;
});
`, cls, t.Cpp, s.Len, e.C, cppDeleter(e), s.Copy, cppOutput(e, "owned.values[i]"))
	if byteSlice(s) {
		fmt.Fprintf(b, `%s.def("copy_to_py_bytes", [=](py::handle object) {
    const auto& self = checked_ref<%s>(object);
    size_t size = %s(self.handle);
    auto output = py::reinterpret_steal<py::bytes>(PyBytes_FromStringAndSize(nullptr, static_cast<Py_ssize_t>(size)));
    if (!output) throw py::error_already_set();
    %s(self.handle, reinterpret_cast<%s*>(PyBytes_AS_STRING(output.ptr())));
    return output;
});
`, cls, t.Cpp, s.Len, s.Copy, e.C)
	}
}
