#include <pybind11/pybind11.h>
#include <cstdlib>
#include <memory>
#include <string>
#include <utility>
#include <vector>

#include "libbridge.h"

namespace py = pybind11;

namespace {

// Shared lifecycle base for every Python wrapper of a Go-owned value.
// Ref only tells Go when this wrapper no longer needs its pinned value.
class Ref {
    uintptr_t id_;

protected:
    explicit Ref(uintptr_t id) : id_(id) {
        if (!id_) throw py::value_error("cannot create Go Ref");
    }

public:
    Ref(const Ref&) = delete;
    Ref& operator=(const Ref&) = delete;
    Ref(Ref&& other) noexcept : id_(std::exchange(other.id_, 0)) {}
    virtual ~Ref() {
        if (id_) SBUnPin(id_);
    }
    // For binding code only; never exposed to Python.
    uintptr_t handle() const { return id_; }
};

// Go string results, as in 03_str: decode strictly, then free the C copy.
py::str py_string(SBString output) {
    std::unique_ptr<char, decltype(&std::free)> owned(output.data, &std::free);
    if (!owned) {
        throw py::value_error("cannot convert Go string");
    }
    if (output.size > static_cast<size_t>(PY_SSIZE_T_MAX)) {
        throw py::value_error("Go string is too large for Python");
    }
    PyObject* decoded = PyUnicode_DecodeUTF8(
        owned.get(), static_cast<Py_ssize_t>(output.size), "strict");
    if (!decoded) throw py::error_already_set();
    return py::reinterpret_steal<py::str>(decoded);
}

// Go int is intptr_t on the C side; bool is accepted as an int subclass.
intptr_t go_int(py::handle item) {
    py::detail::make_caster<intptr_t> element;
    if (!element.load(item, false)) {
        throw py::type_error("expected an int within Go int range");
    }
    return py::detail::cast_op<intptr_t>(element);
}

// A Go-owned argument must be a wrapper of type T; None and other types are
// rejected instead of converted.
template <class T>
const T& wrapper_arg(py::handle value, const char* expected) {
    if (!py::isinstance<T>(value)) throw py::type_error(std::string("expected ") + expected);
    return value.cast<const T&>();
}

// ---------------------------------------------------------------------------
// MyNamedNonStructInt: a Python int subclass, created at import time because
// pybind11 classes cannot derive from int. The module attribute keeps the
// class alive, so a borrowed handle is enough.

py::handle my_named_non_struct_int;

intptr_t my_named_non_struct_int_value(py::handle value) {
    if (!py::isinstance(value, my_named_non_struct_int)) {
        throw py::type_error("expected MyNamedNonStructInt");
    }
    return go_int(value);
}

py::object define_my_named_non_struct_int(py::module_& module) {
    py::handle int_type(reinterpret_cast<PyObject*>(&PyLong_Type));
    py::handle type_type(reinterpret_cast<PyObject*>(&PyType_Type));
    py::dict body;
    body["__module__"] = "go_named_type";
    body["__qualname__"] = "MyNamedNonStructInt";
    body["__doc__"] = "Go MyNamedNonStructInt: a named int, passed by value";
    body["__slots__"] = py::tuple();  // Immutable like int: no instance __dict__.
    py::object cls = type_type("MyNamedNonStructInt", py::make_tuple(int_type), body);

    // Same checks as a Go int argument; the default is the Go zero value.
    cls.attr("__new__") = py::staticmethod(py::cpp_function(
        [int_type](py::handle cls, py::handle value) {
            return int_type.attr("__new__")(cls, go_int(value));
        },
        py::name("__new__"), py::arg("cls"), py::arg("value") = 0));
    // Go's int(i).
    cls.attr("unname") = py::cpp_function(
        [](py::handle self) { return my_named_non_struct_int_value(self); },
        py::name("unname"), py::is_method(cls));
    cls.attr("echo_non_struct_int") = py::cpp_function(
        [](py::handle self) {
            return py_string(
                SBMyNamedNonStructIntEchoNonStructInt(my_named_non_struct_int_value(self)));
        },
        py::name("echo_non_struct_int"), py::is_method(cls));
    module.attr("MyNamedNonStructInt") = cls;
    my_named_non_struct_int = cls;
    return cls;
}

// ---------------------------------------------------------------------------
// Slices, as in 05_slice.

// Common base of Go slice wrappers; each element type has its own subclass.
// It maps Python indices and slices to Go indices; Go semantics stay unchanged.
class Slice : public Ref {
protected:
    using Ref::Ref;

    // Negative indices count from the end; out of range raises IndexError.
    size_t element_index(py::ssize_t index) const {
        const auto length = static_cast<py::ssize_t>(size());
        if (index < 0) index += length;
        if (index < 0 || index >= length) throw py::index_error("slice index out of range");
        return static_cast<size_t>(index);
    }

    // Bounds are clamped like a Python list slice, so a reslice never reaches
    // into spare capacity. Go has no step, so only step 1 is accepted.
    std::pair<size_t, size_t> bounds(const py::slice& range) const {
        py::ssize_t start = 0, stop = 0, step = 0, count = 0;
        if (PySlice_Unpack(range.ptr(), &start, &stop, &step) != 0) throw py::error_already_set();
        if (step != 1) throw py::value_error("Go slices do not support a slice step");
        count = PySlice_AdjustIndices(static_cast<py::ssize_t>(size()), &start, &stop, step);
        return {static_cast<size_t>(start), static_cast<size_t>(start + count)};
    }

public:
    virtual size_t size() const = 0;
};

// Operations shared by every slice whose handle has the []int layout. Self
// is the Python class that reslice and clone return, so a named slice keeps
// its name, as Go's s[i:j] and slices.Clone do.
template <class Self>
class IntSlice : public Slice {
protected:
    using Slice::Slice;

public:
    size_t size() const override { return SBSliceIntLen(handle()); }
    intptr_t get(py::ssize_t index) const {
        intptr_t value = 0;
        if (SBSliceIntAt(handle(), element_index(index), &value) != 0) {
            throw py::index_error("slice index out of range");
        }
        return value;
    }
    void set(py::ssize_t index, py::handle value) {
        const auto position = element_index(index);
        if (SBSliceIntSet(handle(), position, go_int(value)) != 0) {
            throw py::index_error("slice index out of range");
        }
    }
    Self reslice(const py::slice& range) const {
        const auto [low, high] = bounds(range);
        return Self(SBSliceIntSlice(handle(), low, high));
    }
    Self clone() const { return Self(SBSliceIntClone(handle())); }
    py::list copy_to_py_list() const {
        std::vector<intptr_t> data(size());
        if (SBCopySliceInt(handle(), data.data(), data.size()) != 0) {
            throw py::value_error("cannot copy Go slice");
        }
        py::list result;
        for (intptr_t value : data) result.append(value);
        return result;
    }
};

// []int: elements are Python ints copied to and from Go by value.
class SliceInt : public IntSlice<SliceInt> {
public:
    explicit SliceInt(uintptr_t id) : IntSlice(id) {}
    SliceInt(SliceInt&&) noexcept = default;

    static SliceInt from_values(const py::iterable& values) {
        std::vector<intptr_t> data;
        for (py::handle item : values) data.push_back(go_int(item));
        return SliceInt(SBNewSliceInt(data.data(), data.size()));
    }
};

// MyNamedNonStructSliceInt: a named []int. It shares SliceInt's layout and
// exports but is a distinct type: it converts to and from SliceInt only
// explicitly, and both conversions share the backing array.
class MyNamedNonStructSliceInt : public IntSlice<MyNamedNonStructSliceInt> {
public:
    explicit MyNamedNonStructSliceInt(uintptr_t id) : IntSlice(id) {}
    MyNamedNonStructSliceInt(MyNamedNonStructSliceInt&&) noexcept = default;

    // Go's MyNamedNonStructSliceInt(s).
    static MyNamedNonStructSliceInt from_slice(const SliceInt& values) {
        return MyNamedNonStructSliceInt(SBNewMyNamedNonStructSliceInt(values.handle()));
    }
    // Go's []int(si).
    SliceInt unname() const { return SliceInt(SBMyNamedNonStructSliceIntUnname(handle())); }
    py::str echo_non_struct_slice_int() const {
        return py_string(SBMyNamedNonStructSliceIntEchoNonStructSliceInt(handle()));
    }
};

// ---------------------------------------------------------------------------
// Structs. Each wrapper holds a handle to a Go *T. A struct-typed field is
// read as a view of the parent's field (Go's &s.Field) and written by copying
// the value in (Go's s.Field = v). Omitted constructor fields are 0 handles
// and keep the Go zero value.

template <class T>
uintptr_t optional_handle(py::handle value, const char* expected) {
    return value.is_none() ? 0 : wrapper_arg<T>(value, expected).handle();
}

class MyNamedSubStruct : public Ref {
public:
    explicit MyNamedSubStruct(uintptr_t id) : Ref(id) {}
    MyNamedSubStruct(MyNamedSubStruct&&) noexcept = default;

    static MyNamedSubStruct create(py::handle basic_member_value) {
        return MyNamedSubStruct(SBNewMyNamedSubStruct(go_int(basic_member_value)));
    }

    intptr_t basic_member_value() const { return SBMyNamedSubStructGetBasicMemberValue(handle()); }
    void set_basic_member_value(py::handle value) {
        SBMyNamedSubStructSetBasicMemberValue(handle(), go_int(value));
    }
    MyNamedSubStruct copy() const { return MyNamedSubStruct(SBMyNamedSubStructCopy(handle())); }
};

class MyNamedAnomySubStruct : public Ref {
public:
    explicit MyNamedAnomySubStruct(uintptr_t id) : Ref(id) {}
    MyNamedAnomySubStruct(MyNamedAnomySubStruct&&) noexcept = default;

    static MyNamedAnomySubStruct create(py::handle basic_member_value_2) {
        return MyNamedAnomySubStruct(SBNewMyNamedAnomySubStruct(go_int(basic_member_value_2)));
    }

    intptr_t basic_member_value_2() const {
        return SBMyNamedAnomySubStructGetBasicMemberValue2(handle());
    }
    void set_basic_member_value_2(py::handle value) {
        SBMyNamedAnomySubStructSetBasicMemberValue2(handle(), go_int(value));
    }
    MyNamedAnomySubStruct copy() const {
        return MyNamedAnomySubStruct(SBMyNamedAnomySubStructCopy(handle()));
    }
};

class MyNamedStruct : public Ref {
public:
    explicit MyNamedStruct(uintptr_t id) : Ref(id) {}
    MyNamedStruct(MyNamedStruct&&) noexcept = default;

    static MyNamedStruct create(py::handle my_named_anomy_sub_struct,
                                py::handle basic_member_value,
                                py::handle ref_member_value,
                                py::handle named_basic_member_value,
                                py::handle named_ref_member_value) {
        // Convert everything before calling Go, so a bad argument creates nothing.
        const auto anomy = optional_handle<MyNamedAnomySubStruct>(
            my_named_anomy_sub_struct, "MyNamedAnomySubStruct");
        const auto basic = go_int(basic_member_value);
        const auto ref = optional_handle<SliceInt>(ref_member_value, "SliceInt");
        const auto named_basic = my_named_non_struct_int_value(named_basic_member_value);
        const auto named_ref = optional_handle<MyNamedSubStruct>(
            named_ref_member_value, "MyNamedSubStruct");
        return MyNamedStruct(SBNewMyNamedStruct(anomy, basic, ref, named_basic, named_ref));
    }

    MyNamedAnomySubStruct my_named_anomy_sub_struct() const {
        return MyNamedAnomySubStruct(SBMyNamedStructGetMyNamedAnomySubStruct(handle()));
    }
    void set_my_named_anomy_sub_struct(py::handle value) {
        const auto& field = wrapper_arg<MyNamedAnomySubStruct>(value, "MyNamedAnomySubStruct");
        SBMyNamedStructSetMyNamedAnomySubStruct(handle(), field.handle());
    }
    // Promoted from the embedded MyNamedAnomySubStruct.
    intptr_t basic_member_value_2() const {
        return SBMyNamedStructGetBasicMemberValue2(handle());
    }
    void set_basic_member_value_2(py::handle value) {
        SBMyNamedStructSetBasicMemberValue2(handle(), go_int(value));
    }
    intptr_t basic_member_value() const { return SBMyNamedStructGetBasicMemberValue(handle()); }
    void set_basic_member_value(py::handle value) {
        SBMyNamedStructSetBasicMemberValue(handle(), go_int(value));
    }
    // Slice fields share their backing array in both directions.
    SliceInt ref_member_value() const {
        return SliceInt(SBMyNamedStructGetRefMemberValue(handle()));
    }
    void set_ref_member_value(py::handle value) {
        SBMyNamedStructSetRefMemberValue(handle(), wrapper_arg<SliceInt>(value, "SliceInt").handle());
    }
    py::object named_basic_member_value() const {
        return my_named_non_struct_int(SBMyNamedStructGetNamedBasicMemberValue(handle()));
    }
    void set_named_basic_member_value(py::handle value) {
        SBMyNamedStructSetNamedBasicMemberValue(handle(), my_named_non_struct_int_value(value));
    }
    MyNamedSubStruct named_ref_member_value() const {
        return MyNamedSubStruct(SBMyNamedStructGetNamedRefMemberValue(handle()));
    }
    void set_named_ref_member_value(py::handle value) {
        const auto& field = wrapper_arg<MyNamedSubStruct>(value, "MyNamedSubStruct");
        SBMyNamedStructSetNamedRefMemberValue(handle(), field.handle());
    }
    MyNamedStruct copy() const { return MyNamedStruct(SBMyNamedStructCopy(handle())); }

    py::str echo_non_ptr() const { return py_string(SBMyNamedStructEchoNonPtr(handle())); }
    py::str echo_ptr() const { return py_string(SBMyNamedStructEchoPtr(handle())); }
    void inc_basic_ptr() { SBMyNamedStructIncBasicPtr(handle()); }
    MyNamedStruct inc_basic_non_ptr() const {
        return MyNamedStruct(SBMyNamedStructIncBasicNonPtr(handle()));
    }
};

// ---------------------------------------------------------------------------
// Functions.

py::str fn_echo_non_struct_int(py::handle i) {
    return py_string(SBFnEchoNonStructInt(my_named_non_struct_int_value(i)));
}

// Only the named slice: a SliceInt needs an explicit conversion first.
py::str fn_echo_non_struct_slice_int(const MyNamedNonStructSliceInt& si) {
    return py_string(SBFnEchoNonStructSliceInt(si.handle()));
}

py::str fn_echo_non_ptr(const MyNamedStruct& si) {
    return py_string(SBFnEchoNonPtr(si.handle()));
}

py::str ptr_fn_echo_ptr(const MyNamedStruct& si) {
    return py_string(SBPtrFnEchoPtr(si.handle()));
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Go named types: a named int, a named slice and structs";
    py::object named_int = define_my_named_non_struct_int(module);

    py::class_<Ref>(module, "Ref");
    // Element-by-element iteration costs one cgo call per item;
    // copy_to_py_list() is the bulk read, so iteration is off.
    py::class_<Slice, Ref>(module, "Slice")
        .def("__len__", &Slice::size)
        .attr("__iter__") = py::none();
    py::class_<SliceInt, Slice>(module, "SliceInt")
        .def(py::init(&SliceInt::from_values), py::arg("values"))
        .def("__getitem__", &SliceInt::get, py::arg("index"))
        .def("__getitem__", &SliceInt::reslice, py::arg("index"))
        .def("__setitem__", &SliceInt::set, py::arg("index"), py::arg("value"))
        .def("clone", &SliceInt::clone)
        .def("copy_to_py_list", &SliceInt::copy_to_py_list);
    // Not a SliceInt subclass: Go's named and unnamed slices are distinct types.
    py::class_<MyNamedNonStructSliceInt, Slice>(module, "MyNamedNonStructSliceInt")
        .def(py::init(&MyNamedNonStructSliceInt::from_slice),
             py::arg("values").noconvert().none(false))
        .def("__getitem__", &MyNamedNonStructSliceInt::get, py::arg("index"))
        .def("__getitem__", &MyNamedNonStructSliceInt::reslice, py::arg("index"))
        .def("__setitem__", &MyNamedNonStructSliceInt::set, py::arg("index"), py::arg("value"))
        .def("clone", &MyNamedNonStructSliceInt::clone)
        .def("copy_to_py_list", &MyNamedNonStructSliceInt::copy_to_py_list)
        .def("unname", &MyNamedNonStructSliceInt::unname)
        .def("echo_non_struct_slice_int", &MyNamedNonStructSliceInt::echo_non_struct_slice_int);

    // Constructors are keyword-only, like keyed Go composite literals; every
    // field defaults to its Go zero value.
    py::class_<MyNamedSubStruct, Ref>(module, "MyNamedSubStruct")
        .def(py::init(&MyNamedSubStruct::create), py::kw_only(),
             py::arg("basic_member_value") = 0)
        .def_property("basic_member_value", &MyNamedSubStruct::basic_member_value,
                      &MyNamedSubStruct::set_basic_member_value)
        .def("__copy__", &MyNamedSubStruct::copy);
    py::class_<MyNamedAnomySubStruct, Ref>(module, "MyNamedAnomySubStruct")
        .def(py::init(&MyNamedAnomySubStruct::create), py::kw_only(),
             py::arg("basic_member_value_2") = 0)
        .def_property("basic_member_value_2", &MyNamedAnomySubStruct::basic_member_value_2,
                      &MyNamedAnomySubStruct::set_basic_member_value_2)
        .def("__copy__", &MyNamedAnomySubStruct::copy);
    py::class_<MyNamedStruct, Ref>(module, "MyNamedStruct")
        .def(py::init(&MyNamedStruct::create), py::kw_only(),
             py::arg("my_named_anomy_sub_struct") = py::none(),
             py::arg("basic_member_value") = 0,
             py::arg("ref_member_value") = py::none(),
             py::arg("named_basic_member_value") = named_int(0),
             py::arg("named_ref_member_value") = py::none())
        .def_property("my_named_anomy_sub_struct", &MyNamedStruct::my_named_anomy_sub_struct,
                      &MyNamedStruct::set_my_named_anomy_sub_struct)
        .def_property("basic_member_value_2", &MyNamedStruct::basic_member_value_2,
                      &MyNamedStruct::set_basic_member_value_2)
        .def_property("basic_member_value", &MyNamedStruct::basic_member_value,
                      &MyNamedStruct::set_basic_member_value)
        .def_property("ref_member_value", &MyNamedStruct::ref_member_value,
                      &MyNamedStruct::set_ref_member_value)
        .def_property("named_basic_member_value", &MyNamedStruct::named_basic_member_value,
                      &MyNamedStruct::set_named_basic_member_value)
        .def_property("named_ref_member_value", &MyNamedStruct::named_ref_member_value,
                      &MyNamedStruct::set_named_ref_member_value)
        .def("__copy__", &MyNamedStruct::copy)
        .def("echo_non_ptr", &MyNamedStruct::echo_non_ptr)
        .def("echo_ptr", &MyNamedStruct::echo_ptr)
        .def("inc_basic_ptr", &MyNamedStruct::inc_basic_ptr)
        .def("inc_basic_non_ptr", &MyNamedStruct::inc_basic_non_ptr);

    module.def("fn_echo_non_struct_int", &fn_echo_non_struct_int, py::arg("i"));
    module.def("fn_echo_non_struct_slice_int", &fn_echo_non_struct_slice_int,
               py::arg("si").noconvert().none(false));
    module.def("fn_echo_non_ptr", &fn_echo_non_ptr, py::arg("si").noconvert().none(false));
    module.def("ptr_fn_echo_ptr", &ptr_fn_echo_ptr, py::arg("si").noconvert().none(false));
    module.def("_holder_count", &SBHolderCount);
    module.def("_collect_go", &SBCollect);
}
