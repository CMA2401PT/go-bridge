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
struct Array3Int : Ref { using Ref::Ref; };
struct PtrArray3Int : Ref { using Ref::Ref; };
size_t index_arg(py::handle index) {
    auto number=py::reinterpret_steal<py::object>(PyNumber_Index(index.ptr()));
    if (!number) throw py::error_already_set();
    auto value=PyLong_AsSsize_t(number.ptr());
    if (PyErr_Occurred()) throw py::error_already_set();
    if (value<0) value+=3;
    if (value<0 || value>=3) throw py::index_error();
    return static_cast<size_t>(value);
}
PYBIND11_MODULE(_native,m) {
    py::class_<Ref>(m,"Ref");
    py::class_<PtrArray3Int,Ref>(m,"PtrArray3Int",py::is_final());
    auto cls=py::class_<Array3Int,Ref>(m,"Array3Int",py::is_final());
    cls.def(py::init([] { return Array3Int(BrZero()); }))
    .def(py::init([](py::iterable values) {
        std::vector<intptr_t> input;
        for(auto value:values) input.push_back(integer_arg<intptr_t>(value));
        if(input.size()!=3) throw py::value_error("expected exactly 3 elements");
        return Array3Int(BrNew(input.data()));
    }),py::arg("values"))
    .def("__len__",[](const Array3Int&){ return 3; })
    .def("__getitem__",[](const Array3Int& self,py::handle index){ return BrAt(self.handle,index_arg(index)); })
    .def("__setitem__",[](const Array3Int& self,py::handle index,py::handle value){ BrSet(self.handle,index_arg(index),integer_arg<intptr_t>(value)); })
    .def("__copy__",[](const Array3Int& self){ return Array3Int(BrCopy(self.handle)); })
    .def("addr",[](const Array3Int& self){ return PtrArray3Int(BrAddress(self.handle)); })
    .def("copy_to_py_list",[](const Array3Int& self){
        intptr_t values[3];BrItems(self.handle,values);py::list result;
        for(auto value:values) result.append(py::int_(value));
        return result;
    });
    cls.attr("__iter__")=py::none();
    m.def("echo",[](const Array3Int& value){return Array3Int(BrEcho(value.handle));},py::arg("value"));
    m.def("increment",[](const Array3Int& value){return Array3Int(BrIncrement(value.handle));},py::arg("value"));
    m.def("mutate",[](const PtrArray3Int& value){BrMutate(value.handle);},py::arg("value"));
    m.def("_holder_count",&BrHolderCount);
}
