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
    explicit Ref(uintptr_t id) : id_(id) {}

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
                BrMyNamedNonStructIntEchoNonStructInt(my_named_non_struct_int_value(self)));
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
    size_t size() const override { return BrSliceIntLen(handle()); }
    intptr_t get(py::ssize_t index) const {
        return BrSliceIntAt(handle(), element_index(index));
    }
    void set(py::ssize_t index, py::handle value) {
        const auto position = element_index(index);
        BrSliceIntSet(handle(), position, go_int(value));
    }
    Self reslice(const py::slice& range) const {
        const auto [low, high] = bounds(range);
        return Self(BrSliceIntSlice(handle(), low, high));
    }
    Self clone() const { return Self(BrSliceIntClone(handle())); }
    py::list copy_to_py_list() const {
        std::vector<intptr_t> data(size());
        BrCopySliceInt(handle(), data.data());
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
        return SliceInt(BrNewSliceInt(data.data(), data.size()));
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
        return MyNamedNonStructSliceInt(BrNewMyNamedNonStructSliceInt(values.handle()));
    }
    // Go's []int(si).
    SliceInt unname() const { return SliceInt(BrMyNamedNonStructSliceIntUnname(handle())); }
    py::str echo_non_struct_slice_int() const {
        return py_string(BrMyNamedNonStructSliceIntEchoNonStructSliceInt(handle()));
    }
};

// ---------------------------------------------------------------------------
// Structs. A struct wrapper T is a Go variable of type T: its handle pins the
// variable's address, a *T. A struct-typed field is read as a view of the
// parent's field (Go's &s.Field) and written by copying the value in (Go's
// s.Field = v). Omitted constructor fields are 0 handles and keep the Go
// zero value.
//
// *T is a separate class, PtrT, whose handle is also a pinned *T. Fields and
// methods are written once as a template over the wrapper base, instantiated
// on Ref for T and on Ptr for PtrT; only the Python types differ.

class Ptr : public Ref {
protected:
    using Ref::Ref;
};

// MyNamedSubStruct and *MyNamedSubStruct.

template <class Base>
class MyNamedSubStructMembers : public Base {
protected:
    explicit MyNamedSubStructMembers(uintptr_t id) : Base(id) {}

public:
    intptr_t basic_member_value() const {
        return BrMyNamedSubStructGetBasicMemberValue(this->handle());
    }
    void set_basic_member_value(py::handle value) {
        BrMyNamedSubStructSetBasicMemberValue(this->handle(), go_int(value));
    }
};

class PtrMyNamedSubStruct;

class MyNamedSubStruct : public MyNamedSubStructMembers<Ref> {
public:
    explicit MyNamedSubStruct(uintptr_t id) : MyNamedSubStructMembers(id) {}
    MyNamedSubStruct(MyNamedSubStruct&&) noexcept = default;

    static MyNamedSubStruct create(py::handle basic_member_value) {
        return MyNamedSubStruct(BrNewMyNamedSubStruct(go_int(basic_member_value)));
    }
    MyNamedSubStruct copy() const { return MyNamedSubStruct(BrMyNamedSubStructCopy(handle())); }
    PtrMyNamedSubStruct addr() const;
};

class PtrMyNamedSubStruct : public MyNamedSubStructMembers<Ptr> {
public:
    explicit PtrMyNamedSubStruct(uintptr_t id) : MyNamedSubStructMembers(id) {}
    PtrMyNamedSubStruct(PtrMyNamedSubStruct&&) noexcept = default;

    // Go's p := new(T); *p = v: a new variable holding a copy.
    static PtrMyNamedSubStruct from_value(const MyNamedSubStruct& value) {
        return PtrMyNamedSubStruct(BrMyNamedSubStructCopy(value.handle()));
    }
    // *p is a variable, so reading it is a view, like a struct field.
    MyNamedSubStruct value() const { return MyNamedSubStruct(BrMyNamedSubStructAlias(handle())); }
    void set_value(py::handle value) {
        const auto& source = wrapper_arg<MyNamedSubStruct>(value, "MyNamedSubStruct");
        BrMyNamedSubStructStore(handle(), source.handle());
    }
};

// Go's &v: shares the variable.
PtrMyNamedSubStruct MyNamedSubStruct::addr() const {
    return PtrMyNamedSubStruct(BrMyNamedSubStructAlias(handle()));
}

template <class Class>
Class& def_my_named_sub_struct_members(Class& cls) {
    using T = typename Class::type;
    return cls.def_property("basic_member_value", &T::basic_member_value,
                            &T::set_basic_member_value);
}

// MyNamedAnomySubStruct and *MyNamedAnomySubStruct.

template <class Base>
class MyNamedAnomySubStructMembers : public Base {
protected:
    explicit MyNamedAnomySubStructMembers(uintptr_t id) : Base(id) {}

public:
    intptr_t basic_member_value_2() const {
        return BrMyNamedAnomySubStructGetBasicMemberValue2(this->handle());
    }
    void set_basic_member_value_2(py::handle value) {
        BrMyNamedAnomySubStructSetBasicMemberValue2(this->handle(), go_int(value));
    }
};

class PtrMyNamedAnomySubStruct;

class MyNamedAnomySubStruct : public MyNamedAnomySubStructMembers<Ref> {
public:
    explicit MyNamedAnomySubStruct(uintptr_t id) : MyNamedAnomySubStructMembers(id) {}
    MyNamedAnomySubStruct(MyNamedAnomySubStruct&&) noexcept = default;

    static MyNamedAnomySubStruct create(py::handle basic_member_value_2) {
        return MyNamedAnomySubStruct(BrNewMyNamedAnomySubStruct(go_int(basic_member_value_2)));
    }
    MyNamedAnomySubStruct copy() const {
        return MyNamedAnomySubStruct(BrMyNamedAnomySubStructCopy(handle()));
    }
    PtrMyNamedAnomySubStruct addr() const;
};

class PtrMyNamedAnomySubStruct : public MyNamedAnomySubStructMembers<Ptr> {
public:
    explicit PtrMyNamedAnomySubStruct(uintptr_t id) : MyNamedAnomySubStructMembers(id) {}
    PtrMyNamedAnomySubStruct(PtrMyNamedAnomySubStruct&&) noexcept = default;

    static PtrMyNamedAnomySubStruct from_value(const MyNamedAnomySubStruct& value) {
        return PtrMyNamedAnomySubStruct(BrMyNamedAnomySubStructCopy(value.handle()));
    }
    MyNamedAnomySubStruct value() const {
        return MyNamedAnomySubStruct(BrMyNamedAnomySubStructAlias(handle()));
    }
    void set_value(py::handle value) {
        const auto& source = wrapper_arg<MyNamedAnomySubStruct>(value, "MyNamedAnomySubStruct");
        BrMyNamedAnomySubStructStore(handle(), source.handle());
    }
};

PtrMyNamedAnomySubStruct MyNamedAnomySubStruct::addr() const {
    return PtrMyNamedAnomySubStruct(BrMyNamedAnomySubStructAlias(handle()));
}

template <class Class>
Class& def_my_named_anomy_sub_struct_members(Class& cls) {
    using T = typename Class::type;
    return cls.def_property("basic_member_value_2", &T::basic_member_value_2,
                            &T::set_basic_member_value_2);
}

// MyNamedStruct and *MyNamedStruct.

class MyNamedStruct;

template <class Base>
class MyNamedStructMembers : public Base {
protected:
    explicit MyNamedStructMembers(uintptr_t id) : Base(id) {}

public:
    MyNamedAnomySubStruct my_named_anomy_sub_struct() const {
        return MyNamedAnomySubStruct(BrMyNamedStructGetMyNamedAnomySubStruct(this->handle()));
    }
    void set_my_named_anomy_sub_struct(py::handle value) {
        const auto& field = wrapper_arg<MyNamedAnomySubStruct>(value, "MyNamedAnomySubStruct");
        BrMyNamedStructSetMyNamedAnomySubStruct(this->handle(), field.handle());
    }
    // Promoted from the embedded MyNamedAnomySubStruct.
    intptr_t basic_member_value_2() const {
        return BrMyNamedStructGetBasicMemberValue2(this->handle());
    }
    void set_basic_member_value_2(py::handle value) {
        BrMyNamedStructSetBasicMemberValue2(this->handle(), go_int(value));
    }
    intptr_t basic_member_value() const {
        return BrMyNamedStructGetBasicMemberValue(this->handle());
    }
    void set_basic_member_value(py::handle value) {
        BrMyNamedStructSetBasicMemberValue(this->handle(), go_int(value));
    }
    // Slice fields share their backing array in both directions.
    SliceInt ref_member_value() const {
        return SliceInt(BrMyNamedStructGetRefMemberValue(this->handle()));
    }
    void set_ref_member_value(py::handle value) {
        BrMyNamedStructSetRefMemberValue(this->handle(),
                                         wrapper_arg<SliceInt>(value, "SliceInt").handle());
    }
    py::object named_basic_member_value() const {
        return my_named_non_struct_int(BrMyNamedStructGetNamedBasicMemberValue(this->handle()));
    }
    void set_named_basic_member_value(py::handle value) {
        BrMyNamedStructSetNamedBasicMemberValue(this->handle(),
                                                my_named_non_struct_int_value(value));
    }
    MyNamedSubStruct named_ref_member_value() const {
        return MyNamedSubStruct(BrMyNamedStructGetNamedRefMemberValue(this->handle()));
    }
    void set_named_ref_member_value(py::handle value) {
        const auto& field = wrapper_arg<MyNamedSubStruct>(value, "MyNamedSubStruct");
        BrMyNamedStructSetNamedRefMemberValue(this->handle(), field.handle());
    }

    // T is a variable, so like *T it has both value and pointer receivers.
    py::str echo_non_ptr() const { return py_string(BrMyNamedStructEchoNonPtr(this->handle())); }
    py::str echo_ptr() const { return py_string(BrMyNamedStructEchoPtr(this->handle())); }
    void inc_basic_ptr() { BrMyNamedStructIncBasicPtr(this->handle()); }
    MyNamedStruct inc_basic_non_ptr() const;
};

class PtrMyNamedStruct;

class MyNamedStruct : public MyNamedStructMembers<Ref> {
public:
    explicit MyNamedStruct(uintptr_t id) : MyNamedStructMembers(id) {}
    MyNamedStruct(MyNamedStruct&&) noexcept = default;

    static MyNamedStruct create(py::kwargs args) {
        uintptr_t anomy = 0, ref = 0, named_ref = 0;
        intptr_t basic = 0, named_basic = 0;
        for (auto entry : args) {
            const auto name = py::cast<std::string>(entry.first);
            const auto value = entry.second;
            if (name == "my_named_anomy_sub_struct") {
                anomy = wrapper_arg<MyNamedAnomySubStruct>(value, "MyNamedAnomySubStruct").handle();
            } else if (name == "basic_member_value") {
                basic = go_int(value);
            } else if (name == "ref_member_value") {
                ref = wrapper_arg<SliceInt>(value, "SliceInt").handle();
            } else if (name == "named_basic_member_value") {
                named_basic = my_named_non_struct_int_value(value);
            } else if (name == "named_ref_member_value") {
                named_ref = wrapper_arg<MyNamedSubStruct>(value, "MyNamedSubStruct").handle();
            } else {
                throw py::type_error("unexpected MyNamedStruct constructor keyword: " + name);
            }
        }
        return MyNamedStruct(BrNewMyNamedStruct(anomy, basic, ref, named_basic, named_ref));
    }
    MyNamedStruct copy() const { return MyNamedStruct(BrMyNamedStructCopy(handle())); }
    PtrMyNamedStruct addr() const;
};

class PtrMyNamedStruct : public MyNamedStructMembers<Ptr> {
public:
    explicit PtrMyNamedStruct(uintptr_t id) : MyNamedStructMembers(id) {}
    PtrMyNamedStruct(PtrMyNamedStruct&&) noexcept = default;

    static PtrMyNamedStruct from_value(const MyNamedStruct& value) {
        return PtrMyNamedStruct(BrMyNamedStructCopy(value.handle()));
    }
    MyNamedStruct value() const { return MyNamedStruct(BrMyNamedStructAlias(handle())); }
    void set_value(py::handle value) {
        BrMyNamedStructStore(handle(), wrapper_arg<MyNamedStruct>(value, "MyNamedStruct").handle());
    }
};

template <class Base>
MyNamedStruct MyNamedStructMembers<Base>::inc_basic_non_ptr() const {
    return MyNamedStruct(BrMyNamedStructIncBasicNonPtr(this->handle()));
}

PtrMyNamedStruct MyNamedStruct::addr() const {
    return PtrMyNamedStruct(BrMyNamedStructAlias(handle()));
}

template <class Class>
Class& def_my_named_struct_members(Class& cls) {
    using T = typename Class::type;
    return cls
        .def_property("my_named_anomy_sub_struct", &T::my_named_anomy_sub_struct,
                      &T::set_my_named_anomy_sub_struct)
        .def_property("basic_member_value_2", &T::basic_member_value_2,
                      &T::set_basic_member_value_2)
        .def_property("basic_member_value", &T::basic_member_value, &T::set_basic_member_value)
        .def_property("ref_member_value", &T::ref_member_value, &T::set_ref_member_value)
        .def_property("named_basic_member_value", &T::named_basic_member_value,
                      &T::set_named_basic_member_value)
        .def_property("named_ref_member_value", &T::named_ref_member_value,
                      &T::set_named_ref_member_value)
        .def("echo_non_ptr", &T::echo_non_ptr)
        .def("echo_ptr", &T::echo_ptr)
        .def("inc_basic_ptr", &T::inc_basic_ptr)
        .def("inc_basic_non_ptr", &T::inc_basic_non_ptr);
}

// ---------------------------------------------------------------------------
// Functions.

py::str fn_echo_non_struct_int(py::handle i) {
    return py_string(BrFnEchoNonStructInt(my_named_non_struct_int_value(i)));
}

// Only the named slice: a SliceInt needs an explicit conversion first.
py::str fn_echo_non_struct_slice_int(const MyNamedNonStructSliceInt& si) {
    return py_string(BrFnEchoNonStructSliceInt(si.handle()));
}

py::str fn_echo_non_ptr(const MyNamedStruct& si) {
    return py_string(BrFnEchoNonPtr(si.handle()));
}

py::str ptr_fn_echo_ptr(const PtrMyNamedStruct& si) {
    return py_string(BrPtrFnEchoPtr(si.handle()));
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
    // field defaults to its Go zero value. A pointer class is built from a
    // value (a copy) or by addr() (shared), and has the fields and methods too.
    py::class_<Ptr, Ref>(module, "Ptr");

    py::class_<MyNamedSubStruct, Ref> sub(module, "MyNamedSubStruct");
    sub.def(py::init(&MyNamedSubStruct::create), py::kw_only(),
            py::arg("basic_member_value") = 0)
        .def("__copy__", &MyNamedSubStruct::copy)
        .def("addr", &MyNamedSubStruct::addr);
    def_my_named_sub_struct_members(sub);
    py::class_<PtrMyNamedSubStruct, Ptr> ptr_sub(module, "PtrMyNamedSubStruct");
    ptr_sub.def(py::init(&PtrMyNamedSubStruct::from_value), py::arg("value").noconvert().none(false))
        .def_static("from_value", &PtrMyNamedSubStruct::from_value,
                    py::arg("value").noconvert().none(false))
        .def_property("value", &PtrMyNamedSubStruct::value, &PtrMyNamedSubStruct::set_value);
    def_my_named_sub_struct_members(ptr_sub);

    py::class_<MyNamedAnomySubStruct, Ref> anomy(module, "MyNamedAnomySubStruct");
    anomy.def(py::init(&MyNamedAnomySubStruct::create), py::kw_only(),
              py::arg("basic_member_value_2") = 0)
        .def("__copy__", &MyNamedAnomySubStruct::copy)
        .def("addr", &MyNamedAnomySubStruct::addr);
    def_my_named_anomy_sub_struct_members(anomy);
    py::class_<PtrMyNamedAnomySubStruct, Ptr> ptr_anomy(module, "PtrMyNamedAnomySubStruct");
    ptr_anomy
        .def(py::init(&PtrMyNamedAnomySubStruct::from_value),
             py::arg("value").noconvert().none(false))
        .def_static("from_value", &PtrMyNamedAnomySubStruct::from_value,
                    py::arg("value").noconvert().none(false))
        .def_property("value", &PtrMyNamedAnomySubStruct::value,
                      &PtrMyNamedAnomySubStruct::set_value);
    def_my_named_anomy_sub_struct_members(ptr_anomy);

    py::class_<MyNamedStruct, Ref> named(module, "MyNamedStruct");
    named.def(py::init(&MyNamedStruct::create))
        .def("__copy__", &MyNamedStruct::copy)
        .def("addr", &MyNamedStruct::addr);
    def_my_named_struct_members(named);
    py::class_<PtrMyNamedStruct, Ptr> ptr_named(module, "PtrMyNamedStruct");
    ptr_named.def(py::init(&PtrMyNamedStruct::from_value), py::arg("value").noconvert().none(false))
        .def_static("from_value", &PtrMyNamedStruct::from_value,
                    py::arg("value").noconvert().none(false))
        .def_property("value", &PtrMyNamedStruct::value, &PtrMyNamedStruct::set_value);
    def_my_named_struct_members(ptr_named);

    module.def("fn_echo_non_struct_int", &fn_echo_non_struct_int, py::arg("i"));
    module.def("fn_echo_non_struct_slice_int", &fn_echo_non_struct_slice_int,
               py::arg("si").noconvert().none(false));
    module.def("fn_echo_non_ptr", &fn_echo_non_ptr, py::arg("si").noconvert().none(false));
    module.def("ptr_fn_echo_ptr", &ptr_fn_echo_ptr, py::arg("si").noconvert().none(false));
    module.def("_holder_count", &BrHolderCount);
    module.def("_collect_go", &BrCollect);
}
