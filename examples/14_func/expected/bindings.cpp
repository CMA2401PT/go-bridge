#include "libbridge.h"

#include <pybind11/pybind11.h>
#include <cstdlib>
#include <limits>
#include <type_traits>

namespace py = pybind11;

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

#include <utility>
#include <vector>
struct Ref {
    uintptr_t handle;
    explicit Ref(uintptr_t h): handle(h) {}
    Ref(const Ref&) = delete;
    Ref(Ref&& other) noexcept : handle(std::exchange(other.handle, 0)) {}
    ~Ref() { BrRelease(handle); }
};
struct FuncIntToInt : Ref { using Ref::Ref; };
PYBIND11_MODULE(_native,m) {
 py::class_<Ref>(m,"Ref");
 py::class_<FuncIntToInt,Ref>(m,"FuncIntToInt",py::is_final())
 .def_static("nil",[]{return FuncIntToInt(0);})
 .def_property_readonly("is_nil",[](const FuncIntToInt& value){return value.handle==0;})
 .def("__bool__",[](const FuncIntToInt&) -> bool {throw py::type_error("Go functions have no truth value; use is_nil");})
 .def("__call__",[](const FuncIntToInt& value,py::handle arg1){
  auto delta=integer_arg<intptr_t>(arg1);
  py::gil_scoped_release release;return BrCall(value.handle,delta);
 },py::arg("arg1"));
 m.def("counter",[](py::handle start){auto v=integer_arg<intptr_t>(start);py::gil_scoped_release release;return FuncIntToInt(BrCounter(v));},py::arg("start"));
 m.def("apply",[](const FuncIntToInt& value,py::handle delta){auto v=integer_arg<intptr_t>(delta);py::gil_scoped_release release;return BrApply(value.handle,v);},py::arg("value"),py::arg("delta"));
 m.def("echo",[](const FuncIntToInt& value){py::gil_scoped_release release;return FuncIntToInt(BrEcho(value.handle));},py::arg("value"));
 m.def("nil",[]{return FuncIntToInt(BrNil());});
 m.def("_holder_count",&BrHolderCount);
}
