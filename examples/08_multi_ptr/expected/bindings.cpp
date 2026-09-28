#include <pybind11/pybind11.h>
#include <cstdlib>
#include <memory>
#include <string>
#include <utility>

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

// Python only builds non-nil pointers, but Go code may store nil in an inner
// level. Until nullable values are designed, reading it raises ValueError.
template <class T>
T pointer_target(uintptr_t id) {
    if (!id) throw py::value_error("Go pointer is nil");
    return T(id);
}

// ---------------------------------------------------------------------------
// Pointers, as in 07_ptr_basic. When the target is itself a pointer, .value
// is a new wrapper for the same variable, and construction copies the given
// pointer: the new variable points where it points. Go only dereferences a
// pointer once for method calls, so a pointer to a pointer has no methods.

class Ptr : public Ref {
protected:
    using Ref::Ref;
};

class PtrPtrInt;
class PtrPtrPtrInt;

// *int.
class PtrInt : public Ptr {
public:
    explicit PtrInt(uintptr_t id) : Ptr(id) {}
    PtrInt(PtrInt&&) noexcept = default;

    static PtrInt from_value(py::handle value) { return PtrInt(SBNewPtrInt(go_int(value))); }

    intptr_t value() const { return SBPtrIntLoad(handle()); }
    void set_value(py::handle value) { SBPtrIntStore(handle(), go_int(value)); }
    PtrPtrInt ptr() const;
};

// **int: generated from ***int by the type closure.
class PtrPtrInt : public Ptr {
public:
    explicit PtrPtrInt(uintptr_t id) : Ptr(id) {}
    PtrPtrInt(PtrPtrInt&&) noexcept = default;

    static PtrPtrInt from_value(py::handle value) {
        return PtrPtrInt(SBNewPtrPtrInt(wrapper_arg<PtrInt>(value, "PtrInt").handle()));
    }

    PtrInt value() const { return pointer_target<PtrInt>(SBPtrPtrIntLoad(handle())); }
    void set_value(py::handle value) {
        SBPtrPtrIntStore(handle(), wrapper_arg<PtrInt>(value, "PtrInt").handle());
    }
    PtrPtrPtrInt ptr() const;
};

// ***int.
class PtrPtrPtrInt : public Ptr {
public:
    explicit PtrPtrPtrInt(uintptr_t id) : Ptr(id) {}
    PtrPtrPtrInt(PtrPtrPtrInt&&) noexcept = default;

    static PtrPtrPtrInt from_value(py::handle value) {
        return PtrPtrPtrInt(
            SBNewPtrPtrPtrInt(wrapper_arg<PtrPtrInt>(value, "PtrPtrInt").handle()));
    }

    PtrPtrInt value() const { return pointer_target<PtrPtrInt>(SBPtrPtrPtrIntLoad(handle())); }
    void set_value(py::handle value) {
        SBPtrPtrPtrIntStore(handle(), wrapper_arg<PtrPtrInt>(value, "PtrPtrInt").handle());
    }
};

PtrPtrInt PtrInt::ptr() const { return PtrPtrInt(SBNewPtrPtrInt(handle())); }
PtrPtrPtrInt PtrPtrInt::ptr() const { return PtrPtrPtrInt(SBNewPtrPtrPtrInt(handle())); }

// IntPtr: a named pointer (type IntPtr *int). It converts to and from PtrInt
// explicitly, sharing the variable. Go allows *p but no methods on it.
class IntPtr : public Ptr {
public:
    explicit IntPtr(uintptr_t id) : Ptr(id) {}
    IntPtr(IntPtr&&) noexcept = default;

    // Go's IntPtr(p).
    static IntPtr from_ptr(const PtrInt& p) { return IntPtr(SBNewIntPtr(p.handle())); }
    // Go's (*int)(p).
    PtrInt unname() const { return PtrInt(SBIntPtrUnname(handle())); }

    intptr_t value() const { return SBIntPtrLoad(handle()); }
    void set_value(py::handle value) { SBIntPtrStore(handle(), go_int(value)); }
};

// ---------------------------------------------------------------------------
// Counter: a named int that only appears as a Box field, as in 06. Its
// pointer type does not appear, so it has no ptr().

py::handle counter;

intptr_t counter_value(py::handle value) {
    if (!py::isinstance(value, counter)) throw py::type_error("expected Counter");
    return go_int(value);
}

py::object define_counter(py::module_& module) {
    py::handle int_type(reinterpret_cast<PyObject*>(&PyLong_Type));
    py::handle type_type(reinterpret_cast<PyObject*>(&PyType_Type));
    py::dict body;
    body["__module__"] = "go_multi_ptr";
    body["__qualname__"] = "Counter";
    body["__doc__"] = "Go Counter: a named int, passed by value";
    body["__slots__"] = py::tuple();
    py::object cls = type_type("Counter", py::make_tuple(int_type), body);
    cls.attr("__new__") = py::staticmethod(py::cpp_function(
        [int_type](py::handle cls, py::handle value) {
            return int_type.attr("__new__")(cls, go_int(value));
        },
        py::name("__new__"), py::arg("cls"), py::arg("value") = 0));
    cls.attr("unname") = py::cpp_function(
        [](py::handle self) { return counter_value(self); },
        py::name("unname"), py::is_method(cls));
    cls.attr("echo") = py::cpp_function(
        [](py::handle self) { return py_string(SBCounterEcho(counter_value(self))); },
        py::name("echo"), py::is_method(cls));
    module.attr("Counter") = cls;
    counter = cls;
    return cls;
}

// ---------------------------------------------------------------------------
// Structs, as in 06: each wrapper holds a *T.

class Inner : public Ref {
public:
    explicit Inner(uintptr_t id) : Ref(id) {}
    Inner(Inner&&) noexcept = default;

    static Inner create(py::handle n) { return Inner(SBNewInner(go_int(n))); }
    intptr_t n() const { return SBInnerGetN(handle()); }
    void set_n(py::handle value) { SBInnerSetN(handle(), go_int(value)); }
};

class PtrPtrBox;

class Box : public Ref {
public:
    explicit Box(uintptr_t id) : Ref(id) {}
    Box(Box&&) noexcept = default;

    static Box create(py::handle count, py::handle inner) {
        const auto count_value = counter_value(count);
        const auto inner_handle =
            inner.is_none() ? 0 : wrapper_arg<Inner>(inner, "Inner").handle();
        return Box(SBNewBox(count_value, inner_handle));
    }

    py::object count() const { return counter(SBBoxGetCount(handle())); }
    void set_count(py::handle value) { SBBoxSetCount(handle(), counter_value(value)); }
    Inner inner() const { return Inner(SBBoxGetInner(handle())); }
    void set_inner(py::handle value) {
        SBBoxSetInner(handle(), wrapper_arg<Inner>(value, "Inner").handle());
    }
    py::str echo() const { return py_string(SBBoxEcho(handle())); }
    void inc() { SBBoxInc(handle()); }
    PtrPtrBox ptr() const;
};

// **Box. Box already stands for *Box, so the target is a Box wrapper and
// storing one copies the pointer, not the struct.
class PtrPtrBox : public Ptr {
public:
    explicit PtrPtrBox(uintptr_t id) : Ptr(id) {}
    PtrPtrBox(PtrPtrBox&&) noexcept = default;

    static PtrPtrBox from_value(py::handle value) {
        return PtrPtrBox(SBNewPtrPtrBox(wrapper_arg<Box>(value, "Box").handle()));
    }

    Box value() const { return pointer_target<Box>(SBPtrPtrBoxLoad(handle())); }
    void set_value(py::handle value) {
        SBPtrPtrBoxStore(handle(), wrapper_arg<Box>(value, "Box").handle());
    }
};

PtrPtrBox Box::ptr() const { return PtrPtrBox(SBNewPtrPtrBox(handle())); }

// ---------------------------------------------------------------------------
// Functions.

void inc_ppp(const PtrPtrPtrInt& p) { SBIncPPP(p.handle()); }
py::str echo_ppp(const PtrPtrPtrInt& p) { return py_string(SBEchoPPP(p.handle())); }
void retarget(const PtrPtrInt& pp, const PtrInt& p) { SBRetarget(pp.handle(), p.handle()); }
void clear(const PtrPtrInt& pp) { SBClear(pp.handle()); }
void inc_int_ptr(const IntPtr& p) { SBIncIntPtr(p.handle()); }
void inc_box_pp(const PtrPtrBox& pp) { SBIncBoxPP(pp.handle()); }

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Go multi-level and named pointers";
    py::object counter_class = define_counter(module);

    py::class_<Ref>(module, "Ref");
    py::class_<Ptr, Ref>(module, "Ptr");
    py::class_<PtrInt, Ptr>(module, "PtrInt")
        .def(py::init(&PtrInt::from_value), py::arg("value") = 0)
        .def_static("from_value", &PtrInt::from_value, py::arg("value"))
        .def_property("value", &PtrInt::value, &PtrInt::set_value)
        .def("ptr", &PtrInt::ptr);
    // A pointer to a pointer has no zero-value constructor: Go's zero is nil.
    py::class_<PtrPtrInt, Ptr>(module, "PtrPtrInt")
        .def(py::init(&PtrPtrInt::from_value), py::arg("value"))
        .def_static("from_value", &PtrPtrInt::from_value, py::arg("value"))
        .def_property("value", &PtrPtrInt::value, &PtrPtrInt::set_value)
        .def("ptr", &PtrPtrInt::ptr);
    py::class_<PtrPtrPtrInt, Ptr>(module, "PtrPtrPtrInt")
        .def(py::init(&PtrPtrPtrInt::from_value), py::arg("value"))
        .def_static("from_value", &PtrPtrPtrInt::from_value, py::arg("value"))
        .def_property("value", &PtrPtrPtrInt::value, &PtrPtrPtrInt::set_value);
    py::class_<IntPtr, Ptr>(module, "IntPtr")
        .def(py::init(&IntPtr::from_ptr), py::arg("p").noconvert().none(false))
        .def("unname", &IntPtr::unname)
        .def_property("value", &IntPtr::value, &IntPtr::set_value);

    py::class_<Inner, Ref>(module, "Inner")
        .def(py::init(&Inner::create), py::kw_only(), py::arg("n") = 0)
        .def_property("n", &Inner::n, &Inner::set_n);
    py::class_<Box, Ref>(module, "Box")
        .def(py::init(&Box::create), py::kw_only(), py::arg("count") = counter_class(0),
             py::arg("inner") = py::none())
        .def_property("count", &Box::count, &Box::set_count)
        .def_property("inner", &Box::inner, &Box::set_inner)
        .def("echo", &Box::echo)
        .def("inc", &Box::inc)
        .def("ptr", &Box::ptr);
    py::class_<PtrPtrBox, Ptr>(module, "PtrPtrBox")
        .def(py::init(&PtrPtrBox::from_value), py::arg("value"))
        .def_static("from_value", &PtrPtrBox::from_value, py::arg("value"))
        .def_property("value", &PtrPtrBox::value, &PtrPtrBox::set_value);

    module.def("inc_ppp", &inc_ppp, py::arg("p").noconvert().none(false));
    module.def("echo_ppp", &echo_ppp, py::arg("p").noconvert().none(false));
    module.def("retarget", &retarget, py::arg("pp").noconvert().none(false),
               py::arg("p").noconvert().none(false));
    module.def("clear", &clear, py::arg("pp").noconvert().none(false));
    module.def("inc_int_ptr", &inc_int_ptr, py::arg("p").noconvert().none(false));
    module.def("inc_box_pp", &inc_box_pp, py::arg("pp").noconvert().none(false));
    module.def("_holder_count", &SBHolderCount);
    module.def("_collect_go", &SBCollect);
}
