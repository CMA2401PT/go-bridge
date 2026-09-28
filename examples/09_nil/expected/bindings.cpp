#include <pybind11/pybind11.h>
#include <cstdlib>
#include <memory>
#include <string>
#include <type_traits>
#include <utility>

#include "libbridge.h"

namespace py = pybind11;

namespace {

// Shared lifecycle base for every Python wrapper of a Go-owned value. Handle
// 0 is the Go zero value, which is nil for pointer wrappers; it pins nothing,
// so there is nothing to release.
class Ref {
    uintptr_t id_;

protected:
    explicit Ref(uintptr_t id) : id_(id) {}

public:
    Ref(const Ref&) = delete;
    Ref& operator=(const Ref&) = delete;
    Ref(Ref&& other) noexcept : id_(std::exchange(other.id_, 0)) {}
    virtual ~Ref() {
        if (id_) SBUnPin(id_);
    }
    // For binding code only; never exposed to Python.
    uintptr_t handle() const { return id_; }
    bool is_nil() const { return id_ == 0; }
};

// Constructors that allocate must get a live handle back.
uintptr_t allocated(uintptr_t id) {
    if (!id) throw py::value_error("cannot create Go Ref");
    return id;
}

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

// ---------------------------------------------------------------------------
// Go panics. Every guarded export takes a panic out-parameter; a non-empty
// message becomes GoPanic, a RuntimeError subclass. The export's result is
// then the Go zero value and is discarded (a zero handle owns nothing).

py::handle go_panic;

template <class Call>
auto go_call(Call call) {
    SBString panicked{nullptr, 0};
    auto raise_if_panicked = [&panicked] {
        if (!panicked.data) return;
        const py::str message = py_string(panicked);
        PyErr_SetObject(go_panic.ptr(), message.ptr());
        throw py::error_already_set();
    };
    if constexpr (std::is_void_v<decltype(call(&panicked))>) {
        call(&panicked);
        raise_if_panicked();
    } else {
        auto result = call(&panicked);
        if (panicked.data) {
            // A string result is still owned C memory; free it before raising.
            if constexpr (std::is_same_v<decltype(result), SBString>) std::free(result.data);
            raise_if_panicked();
        }
        return result;
    }
}

// Nil pointers have a type, as in Go, and so share the class of non-nil
// ones. Truth testing would be true for nil too, so it is refused.
template <class Class>
void define_nil(Class& cls) {
    using T = typename Class::type;
    cls.def_static("nil", [] { return T(0); })
        .def_property_readonly("is_nil", &T::is_nil)
        .def("__bool__", [](const T&) -> bool {
            throw py::type_error("a Go pointer has no truth value; use is_nil");
        });
}

// A Go pointer argument must be a wrapper of type T, nil included. None and
// other types are rejected instead of converted.
template <class T>
const T& wrapper_arg(py::handle value, const char* expected) {
    if (!py::isinstance<T>(value)) throw py::type_error(std::string("expected ") + expected);
    return value.cast<const T&>();
}

// ---------------------------------------------------------------------------
// *int, as in 07. Dereferencing nil panics inside Go and becomes GoPanic.

class PtrInt : public Ref {
public:
    explicit PtrInt(uintptr_t id) : Ref(id) {}
    PtrInt(PtrInt&&) noexcept = default;

    static PtrInt from_value(py::handle value) {
        return PtrInt(allocated(SBNewPtrInt(go_int(value))));
    }

    intptr_t value() const {
        return go_call([&](SBString* p) { return SBPtrIntLoad(handle(), p); });
    }
    void set_value(py::handle value) {
        const auto v = go_int(value);
        go_call([&](SBString* p) { SBPtrIntStore(handle(), v, p); });
    }
};

// ---------------------------------------------------------------------------
// *List, as a struct wrapper of 06. Pointer receivers accept a nil list; the
// value receiver and field access panic on it, as in Go.

class List : public Ref {
public:
    explicit List(uintptr_t id) : Ref(id) {}
    List(List&&) noexcept = default;

    static List create(py::handle value, py::handle next) {
        const auto v = go_int(value);
        const auto& tail = wrapper_arg<List>(next, "List");
        return List(allocated(SBNewList(v, tail.handle())));
    }

    intptr_t value() const {
        return go_call([&](SBString* p) { return SBListGetValue(handle(), p); });
    }
    void set_value(py::handle value) {
        const auto v = go_int(value);
        go_call([&](SBString* p) { SBListSetValue(handle(), v, p); });
    }
    // The pointer held in the field: may be nil, and shares the list.
    List next() const {
        return List(go_call([&](SBString* p) { return SBListGetNext(handle(), p); }));
    }
    void set_next(py::handle value) {
        const auto& tail = wrapper_arg<List>(value, "List");
        go_call([&](SBString* p) { SBListSetNext(handle(), tail.handle(), p); });
    }

    intptr_t len() const {
        return go_call([&](SBString* p) { return SBListLen(handle(), p); });
    }
    List push(py::handle v) const {
        const auto item = go_int(v);
        return List(go_call([&](SBString* p) { return SBListPush(handle(), item, p); }));
    }
    intptr_t first() const {
        return go_call([&](SBString* p) { return SBListFirst(handle(), p); });
    }
    intptr_t head() const {
        return go_call([&](SBString* p) { return SBListHead(handle(), p); });
    }
};

// ---------------------------------------------------------------------------
// Functions.

List find(const List& l, py::handle v) {
    const auto item = go_int(v);
    return List(go_call([&](SBString* p) { return SBFind(l.handle(), item, p); }));
}

py::str deref(const PtrInt& p) {
    return py_string(go_call([&](SBString* out) { return SBDeref(p.handle(), out); }));
}

PtrInt maybe_int(py::handle ok) {
    if (!PyBool_Check(ok.ptr())) throw py::type_error("expected a bool");
    const uint8_t value = ok.ptr() == Py_True ? 1 : 0;
    return PtrInt(go_call([&](SBString* p) { return SBMaybeInt(value, p); }));
}

intptr_t must_positive(py::handle v) {
    const auto value = go_int(v);
    return go_call([&](SBString* p) { return SBMustPositive(value, p); });
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Go nil pointers, nil receivers and panics";
    static py::exception<void> panic_type(module, "GoPanic", PyExc_RuntimeError);
    go_panic = panic_type;

    py::class_<Ref>(module, "Ref");
    py::class_<PtrInt, Ref> ptr_int(module, "PtrInt");
    ptr_int.def(py::init(&PtrInt::from_value), py::arg("value") = 0)
        .def_static("from_value", &PtrInt::from_value, py::arg("value"))
        .def_property("value", &PtrInt::value, &PtrInt::set_value);
    define_nil(ptr_int);

    py::class_<List, Ref> list(module, "List");
    define_nil(list);
    // The default next is a nil *List, i.e. the field's zero value.
    list.def(py::init(&List::create), py::kw_only(), py::arg("value") = 0,
             py::arg("next") = py::cast(List(0)))
        .def_property("value", &List::value, &List::set_value)
        .def_property("next", &List::next, &List::set_next)
        .def("len", &List::len)
        .def("push", &List::push, py::arg("v"))
        .def("first", &List::first)
        .def("head", &List::head);

    module.def("find", &find, py::arg("l").noconvert().none(false), py::arg("v"));
    module.def("deref", &deref, py::arg("p").noconvert().none(false));
    module.def("maybe_int", &maybe_int, py::arg("ok"));
    module.def("must_positive", &must_positive, py::arg("v"));
    module.def("_holder_count", &SBHolderCount);
    module.def("_collect_go", &SBCollect);
}
