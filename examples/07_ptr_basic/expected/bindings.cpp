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
        if (id_) BrUnPin(id_);
    }
    // For binding code only; never exposed to Python.
    uintptr_t handle() const { return id_; }
};

// Go string results, as in 03_str: decode strictly, then free the C copy.
py::str py_string(BrString output) {
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

// A Go-owned argument must be a wrapper of type T; None and other types are
// rejected instead of converted.
template <class T>
const T& wrapper_arg(py::handle value, const char* expected) {
    if (!py::isinstance<T>(value)) throw py::type_error(std::string("expected ") + expected);
    return value.cast<const T&>();
}

// Go int is intptr_t on the C side; bool is accepted as an int subclass.
intptr_t go_int(py::handle item) {
    py::detail::make_caster<intptr_t> element;
    if (!element.load(item, false)) {
        throw py::type_error("expected an int within Go int range");
    }
    return py::detail::cast_op<intptr_t>(element);
}

// ---------------------------------------------------------------------------
// Pointers. A Ptr wrapper holds a handle to a Go *T, allocated like Go's
// new(T) followed by *p = value. The pointer in the handle never changes, so
// one wrapper always refers to one Go variable; .value reads and writes *p.
// Distinct pointer types are unrelated classes, as in Go.

class Ptr : public Ref {
protected:
    using Ref::Ref;
};

// ---------------------------------------------------------------------------
// MyNamedNonStructInt, as in 06_named_type: a Python int subclass created at
// import time. It carries only value-receiver methods; pointer-receiver
// methods need an addressable Go variable, which ptr() allocates.

py::handle my_named_non_struct_int;

intptr_t my_named_non_struct_int_value(py::handle value) {
    if (!py::isinstance(value, my_named_non_struct_int)) {
        throw py::type_error("expected MyNamedNonStructInt");
    }
    return go_int(value);
}

py::object named_int(intptr_t value) { return my_named_non_struct_int(value); }

// *int.
class PtrInt : public Ptr {
public:
    explicit PtrInt(uintptr_t id) : Ptr(id) {}
    PtrInt(PtrInt&&) noexcept = default;

    static PtrInt from_value(py::handle value) { return PtrInt(BrNewPtrInt(go_int(value))); }

    intptr_t value() const { return BrPtrIntLoad(handle()); }
    void set_value(py::handle value) { BrPtrIntStore(handle(), go_int(value)); }
};

// *MyNamedNonStructInt: *p only accepts the named type, as Go assignment does.
class PtrMyNamedNonStructInt : public Ptr {
public:
    explicit PtrMyNamedNonStructInt(uintptr_t id) : Ptr(id) {}
    PtrMyNamedNonStructInt(PtrMyNamedNonStructInt&&) noexcept = default;

    static PtrMyNamedNonStructInt from_value(py::handle value) {
        return PtrMyNamedNonStructInt(
            BrNewPtrMyNamedNonStructInt(my_named_non_struct_int_value(value)));
    }

    py::object value() const { return named_int(BrPtrMyNamedNonStructIntLoad(handle())); }
    void set_value(py::handle value) {
        BrPtrMyNamedNonStructIntStore(handle(), my_named_non_struct_int_value(value));
    }

    // Value receiver through the pointer: (*p).EchoNonStructInt().
    py::str echo_non_struct_int() const {
        return py_string(BrPtrMyNamedNonStructIntEchoNonStructInt(handle()));
    }
    py::str ptr_echo_non_struct_int() const {
        return py_string(BrPtrMyNamedNonStructIntPtrEchoNonStructInt(handle()));
    }
    void inc() { BrPtrMyNamedNonStructIntInc(handle()); }
};

py::object define_my_named_non_struct_int(py::module_& module) {
    py::handle int_type(reinterpret_cast<PyObject*>(&PyLong_Type));
    py::handle type_type(reinterpret_cast<PyObject*>(&PyType_Type));
    py::dict body;
    body["__module__"] = "go_ptr_basic";
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
                BrMyNamedNonStructIntEchoNonStructInt(my_named_non_struct_int_value(self)));
        },
        py::name("echo_non_struct_int"), py::is_method(cls));
    // A new variable holding a copy: p := new(T); *p = i. A method, not a
    // property, because every call allocates.
    cls.attr("ptr") = py::cpp_function(
        [](py::handle self) { return PtrMyNamedNonStructInt::from_value(self); },
        py::name("ptr"), py::is_method(cls));
    module.attr("MyNamedNonStructInt") = cls;
    my_named_non_struct_int = cls;
    return cls;
}

// ---------------------------------------------------------------------------
// Slices, reduced to what this example needs from 05_slice.

class Slice : public Ref {
protected:
    using Ref::Ref;

public:
    // Every slice in this example has the []int layout.
    size_t size() const { return BrSliceIntLen(handle()); }
    py::list copy_to_py_list() const {
        std::vector<intptr_t> data(size());
        if (BrCopySliceInt(handle(), data.data(), data.size()) != 0) {
            throw py::value_error("cannot copy Go slice");
        }
        py::list result;
        for (intptr_t value : data) result.append(value);
        return result;
    }
};

// []int.
class SliceInt : public Slice {
public:
    explicit SliceInt(uintptr_t id) : Slice(id) {}
    SliceInt(SliceInt&&) noexcept = default;

    static SliceInt from_values(const py::iterable& values) {
        std::vector<intptr_t> data;
        for (py::handle item : values) data.push_back(go_int(item));
        return SliceInt(BrNewSliceInt(data.data(), data.size()));
    }
};

// MyNamedNonStructSliceInt: shares SliceInt's layout and exports but is a
// distinct type, as in 06; conversions are explicit and share the array.
class MyNamedNonStructSliceInt : public Slice {
public:
    explicit MyNamedNonStructSliceInt(uintptr_t id) : Slice(id) {}
    MyNamedNonStructSliceInt(MyNamedNonStructSliceInt&&) noexcept = default;

    static MyNamedNonStructSliceInt from_slice(const SliceInt& values) {
        return MyNamedNonStructSliceInt(BrNewMyNamedNonStructSliceInt(values.handle()));
    }
    SliceInt unname() const { return SliceInt(BrMyNamedNonStructSliceIntUnname(handle())); }
    py::str echo_non_struct_slice_int() const {
        return py_string(BrMyNamedNonStructSliceIntEchoNonStructSliceInt(handle()));
    }
};

// *[]int. *p is a slice header: .value returns a copy of the header that
// shares the backing array (s := *p); assigning replaces the header (*p = s).
class PtrSliceInt : public Ptr {
public:
    explicit PtrSliceInt(uintptr_t id) : Ptr(id) {}
    PtrSliceInt(PtrSliceInt&&) noexcept = default;

    static PtrSliceInt from_value(py::handle value) {
        return PtrSliceInt(BrNewPtrSliceInt(wrapper_arg<SliceInt>(value, "SliceInt").handle()));
    }

    SliceInt value() const { return SliceInt(BrPtrSliceIntLoad(handle())); }
    void set_value(py::handle value) {
        BrPtrSliceIntStore(handle(), wrapper_arg<SliceInt>(value, "SliceInt").handle());
    }
};

// *MyNamedNonStructSliceInt. Construction and *p = s take only the named slice.
class PtrMyNamedNonStructSliceInt : public Ptr {
public:
    explicit PtrMyNamedNonStructSliceInt(uintptr_t id) : Ptr(id) {}
    PtrMyNamedNonStructSliceInt(PtrMyNamedNonStructSliceInt&&) noexcept = default;

    static PtrMyNamedNonStructSliceInt from_value(py::handle value) {
        return PtrMyNamedNonStructSliceInt(BrNewPtrMyNamedNonStructSliceInt(
            wrapper_arg<MyNamedNonStructSliceInt>(value, "MyNamedNonStructSliceInt").handle()));
    }

    MyNamedNonStructSliceInt value() const {
        return MyNamedNonStructSliceInt(BrPtrMyNamedNonStructSliceIntLoad(handle()));
    }
    void set_value(py::handle value) {
        BrPtrMyNamedNonStructSliceIntStore(
            handle(),
            wrapper_arg<MyNamedNonStructSliceInt>(value, "MyNamedNonStructSliceInt").handle());
    }

    py::str echo_non_struct_slice_int() const {
        return py_string(BrPtrMyNamedNonStructSliceIntEchoNonStructSliceInt(handle()));
    }
    py::str ptr_echo_non_struct_slice_int() const {
        return py_string(BrPtrMyNamedNonStructSliceIntPtrEchoNonStructSliceInt(handle()));
    }
    void append(py::handle v) { BrPtrMyNamedNonStructSliceIntAppend(handle(), go_int(v)); }
};

// ---------------------------------------------------------------------------
// Functions. A pointer parameter takes exactly its pointer class: a Python
// value is never boxed into a temporary, since Go's writes through the
// pointer would be lost silently.

py::str fn_echo_non_struct_int(py::handle i) {
    return py_string(BrFnEchoNonStructInt(my_named_non_struct_int_value(i)));
}

py::str ptr_fn_echo_non_struct_int(const PtrMyNamedNonStructInt& i) {
    return py_string(BrPtrFnEchoNonStructInt(i.handle()));
}

py::str fn_echo_int(py::handle i) { return py_string(BrFnEchoInt(go_int(i))); }

py::str ptr_fn_echo_int(const PtrInt& i) { return py_string(BrPtrFnEchoInt(i.handle())); }

void inc_int(const PtrInt& i) { BrIncInt(i.handle()); }

py::str fn_echo_non_struct_slice_int(const MyNamedNonStructSliceInt& si) {
    return py_string(BrFnEchoNonStructSliceInt(si.handle()));
}

py::str fn_echo_slice_int(const SliceInt& si) {
    return py_string(BrFnEchoSliceInt(si.handle()));
}

py::str ptr_fn_echo_non_struct_slice_int(const PtrMyNamedNonStructSliceInt& si) {
    return py_string(BrPtrFnEchoNonStructSliceInt(si.handle()));
}

py::str ptr_fn_echo_slice_int(const PtrSliceInt& si) {
    return py_string(BrPtrFnEchoSliceInt(si.handle()));
}

void append_slice_int(const PtrSliceInt& si, py::handle v) {
    BrAppendSliceInt(si.handle(), go_int(v));
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Go pointers to named and unnamed value types";
    // ptr() returns PtrMyNamedNonStructInt, which is only cast at call time,
    // so the int subclass can be created before the pointer class.
    py::object named = define_my_named_non_struct_int(module);

    py::class_<Ref>(module, "Ref");
    py::class_<Ptr, Ref>(module, "Ptr");
    // Every pointer is built from a value, like Go's new(T); *p = value.
    // The constructor and from_value are the same operation.
    py::class_<PtrInt, Ptr>(module, "PtrInt")
        .def(py::init(&PtrInt::from_value), py::arg("value") = 0)
        .def_static("from_value", &PtrInt::from_value, py::arg("value"))
        .def_property("value", &PtrInt::value, &PtrInt::set_value);
    py::class_<PtrMyNamedNonStructInt, Ptr>(module, "PtrMyNamedNonStructInt")
        .def(py::init(&PtrMyNamedNonStructInt::from_value), py::arg("value") = named(0))
        .def_static("from_value", &PtrMyNamedNonStructInt::from_value, py::arg("value"))
        .def_property("value", &PtrMyNamedNonStructInt::value,
                      &PtrMyNamedNonStructInt::set_value)
        .def("echo_non_struct_int", &PtrMyNamedNonStructInt::echo_non_struct_int)
        .def("ptr_echo_non_struct_int", &PtrMyNamedNonStructInt::ptr_echo_non_struct_int)
        .def("inc", &PtrMyNamedNonStructInt::inc);

    // Element-by-element iteration costs one cgo call per item;
    // copy_to_py_list() is the bulk read, so iteration is off.
    py::class_<Slice, Ref>(module, "Slice")
        .def("__len__", &Slice::size)
        .def("copy_to_py_list", &Slice::copy_to_py_list)
        .attr("__iter__") = py::none();
    py::class_<SliceInt, Slice>(module, "SliceInt")
        .def(py::init(&SliceInt::from_values), py::arg("values"))
        .def("ptr", [](py::handle self) { return PtrSliceInt::from_value(self); });
    // Not a SliceInt subclass: Go's named and unnamed slices are distinct types.
    py::class_<MyNamedNonStructSliceInt, Slice>(module, "MyNamedNonStructSliceInt")
        .def(py::init(&MyNamedNonStructSliceInt::from_slice),
             py::arg("values").noconvert().none(false))
        .def("unname", &MyNamedNonStructSliceInt::unname)
        .def("echo_non_struct_slice_int", &MyNamedNonStructSliceInt::echo_non_struct_slice_int)
        .def("ptr", [](py::handle self) { return PtrMyNamedNonStructSliceInt::from_value(self); });
    py::class_<PtrSliceInt, Ptr>(module, "PtrSliceInt")
        .def(py::init(&PtrSliceInt::from_value), py::arg("value"))
        .def_static("from_value", &PtrSliceInt::from_value, py::arg("value"))
        .def_property("value", &PtrSliceInt::value, &PtrSliceInt::set_value);
    py::class_<PtrMyNamedNonStructSliceInt, Ptr>(module, "PtrMyNamedNonStructSliceInt")
        .def(py::init(&PtrMyNamedNonStructSliceInt::from_value), py::arg("value"))
        .def_static("from_value", &PtrMyNamedNonStructSliceInt::from_value, py::arg("value"))
        .def_property("value", &PtrMyNamedNonStructSliceInt::value,
                      &PtrMyNamedNonStructSliceInt::set_value)
        .def("echo_non_struct_slice_int",
             &PtrMyNamedNonStructSliceInt::echo_non_struct_slice_int)
        .def("ptr_echo_non_struct_slice_int",
             &PtrMyNamedNonStructSliceInt::ptr_echo_non_struct_slice_int)
        .def("append", &PtrMyNamedNonStructSliceInt::append, py::arg("v"));

    module.def("fn_echo_non_struct_int", &fn_echo_non_struct_int, py::arg("i"));
    module.def("ptr_fn_echo_non_struct_int", &ptr_fn_echo_non_struct_int,
               py::arg("i").noconvert().none(false));
    module.def("fn_echo_int", &fn_echo_int, py::arg("i"));
    module.def("ptr_fn_echo_int", &ptr_fn_echo_int, py::arg("i").noconvert().none(false));
    module.def("inc_int", &inc_int, py::arg("i").noconvert().none(false));
    module.def("fn_echo_non_struct_slice_int", &fn_echo_non_struct_slice_int,
               py::arg("si").noconvert().none(false));
    module.def("fn_echo_slice_int", &fn_echo_slice_int, py::arg("si").noconvert().none(false));
    module.def("ptr_fn_echo_non_struct_slice_int", &ptr_fn_echo_non_struct_slice_int,
               py::arg("si").noconvert().none(false));
    module.def("ptr_fn_echo_slice_int", &ptr_fn_echo_slice_int,
               py::arg("si").noconvert().none(false));
    module.def("append_slice_int", &append_slice_int,
               py::arg("si").noconvert().none(false), py::arg("v"));
    module.def("_holder_count", &BrHolderCount);
    module.def("_collect_go", &BrCollect);
}
