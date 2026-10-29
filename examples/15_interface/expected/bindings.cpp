#include "libbridge.h"

#include <pybind11/pybind11.h>
#include <cstdlib>
#include <limits>
#include <type_traits>

namespace py = pybind11;

// Conversion holds the GIL. Call arguments keep their UTF-8 caches alive
// while the Go call releases it; Go copies strings before retaining them.
inline BrString string_arg(py::handle value) {
    if (!PyUnicode_Check(value.ptr())) throw py::type_error("expected str");
    Py_ssize_t size;
    const char* data = PyUnicode_AsUTF8AndSize(value.ptr(), &size);
    if (!data) throw py::error_already_set();
    return {const_cast<char*>(data), static_cast<size_t>(size)};
}

inline uint8_t bool_arg(py::handle value) {
    if (!PyBool_Check(value.ptr())) throw py::type_error("expected bool");
    return value.ptr() == Py_True ? 1 : 0;
}

// Raised for an int that the target Go type cannot represent, matching
// Python's convention for C integer conversions. pybind11 has no wrapper.
[[noreturn]] inline void integer_overflow() {
    PyErr_Clear();
    PyErr_SetString(PyExc_OverflowError, "integer outside Go type range");
    throw py::error_already_set();
}

// Accepts int (including its bool subclass) and checks the exact Go range.
template <class T>
T integer_arg(py::handle value) {
    if (!PyLong_Check(value.ptr())) throw py::type_error("expected int");
    if constexpr (std::is_signed_v<T>) {
        int overflow = 0;
        auto number = PyLong_AsLongLongAndOverflow(value.ptr(), &overflow);
        if (overflow || PyErr_Occurred() || number < std::numeric_limits<T>::min() || number > std::numeric_limits<T>::max()) {
            integer_overflow();
        }
        return static_cast<T>(number);
    } else {
        auto number = PyLong_AsUnsignedLongLong(value.ptr());
        if (PyErr_Occurred() || number > std::numeric_limits<T>::max()) {
            integer_overflow();
        }
        return static_cast<T>(number);
    }
}

template <class T>
T float_arg(py::handle value) {
    if (!PyFloat_Check(value.ptr()) && !PyLong_Check(value.ptr())) throw py::type_error("expected float or int");
    double number = PyFloat_AsDouble(value.ptr());
    if (PyErr_Occurred()) throw py::error_already_set();
    return static_cast<T>(number);
}

inline py::str decode(BrString value) {
    PyObject* result = PyUnicode_DecodeUTF8(value.data, static_cast<Py_ssize_t>(value.size), "strict");
    if (!result) throw py::error_already_set();
    return py::reinterpret_steal<py::str>(result);
}

#include <utility>
#include <vector>
struct Ref {
    uintptr_t handle;
    explicit Ref(uintptr_t h): handle(h) {}
    Ref(const Ref&) = delete;
    Ref(Ref&& other) noexcept : handle(std::exchange(other.handle, 0)) {}
    ~Ref() { BrRelease(handle); }
};
struct Counter : Ref {using Ref::Ref;};
struct ErrorValue : Ref {using Ref::Ref;};
struct StringOwner {BrString value;~StringOwner(){std::free(value.data);}};
PYBIND11_MODULE(_native,m) {
 py::class_<Ref>(m,"Ref");
 py::class_<Counter,Ref>(m,"Counter",py::is_final())
 .def_static("nil",[]{return Counter(0);})
 .def_property_readonly("is_nil",[](const Counter& value){return value.handle==0;})
 .def("__bool__",[](const Counter&) -> bool {throw py::type_error("Go interfaces have no truth value; use is_nil");})
 .def("add",[](const Counter& value,py::handle arg1){auto n=integer_arg<intptr_t>(arg1);py::gil_scoped_release release;return BrAdd(value.handle,n);},py::arg("arg1"));
 py::class_<ErrorValue,Ref>(m,"error",py::is_final())
 .def_static("nil",[]{return ErrorValue(0);})
 .def_property_readonly("is_nil",[](const ErrorValue& value){return value.handle==0;})
 .def("__bool__",[](const ErrorValue&) -> bool {throw py::type_error("Go interfaces have no truth value; use is_nil");})
 .def("error",[](const ErrorValue& value){StringOwner owner{[&]{py::gil_scoped_release release;return BrError(value.handle);}()};return decode(owner.value);});
 m.def("new",[](py::handle start){auto n=integer_arg<intptr_t>(start);py::gil_scoped_release release;return Counter(BrNew(n));},py::arg("start"));
 m.def("echo",[](const Counter& value){py::gil_scoped_release release;return Counter(BrEcho(value.handle));},py::arg("value"));
 m.def("nil",[]{return Counter(BrNil());});
 m.def("typed_nil",[]{return Counter(BrTypedNil());});
 m.def("result",[](py::handle fail){auto v=bool_arg(fail);auto result=[&]{py::gil_scoped_release release;return BrResultValue(v);}();ErrorValue error(result.error);return py::make_tuple(result.value,std::move(error));},py::arg("fail"));
 m.def("_holder_count",&BrHolderCount);
}
