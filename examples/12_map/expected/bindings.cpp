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
struct MapStringInt : Ref { using Ref::Ref; };
struct Strings {
    std::vector<BrString> values;
    explicit Strings(size_t size): values(size) {}
    ~Strings() { for (auto value: values) std::free(value.data); }
};
PYBIND11_MODULE(_native,m) {
    py::class_<Ref>(m,"Ref");
    auto cls=py::class_<MapStringInt,Ref>(m,"MapStringInt",py::is_final());
    cls.def(py::init([](py::iterable items) {
        std::vector<py::object> keep;
        std::vector<BrString> keys;
        std::vector<intptr_t> values;
        for (auto item: items) {
            auto pair=py::cast<py::tuple>(item);
            if (pair.size()!=2) throw py::value_error("expected key/value pairs");
            keep.push_back(pair);
            keys.push_back(string_arg(pair[0])); values.push_back(integer_arg<intptr_t>(pair[1]));
        }
        return MapStringInt(BrNew(keys.data(),values.data(),keys.size()));
    }),py::arg("items")=py::tuple())
    .def_static("nil",[]{ return MapStringInt(0); })
    .def_property_readonly("is_nil",[](const MapStringInt& self){ return self.handle==0; })
    .def("__len__",[](const MapStringInt& self){ return BrLen(self.handle); })
    .def("__getitem__",[](const MapStringInt& self,py::handle key){
        auto result=BrGet(self.handle,string_arg(key));
        if (!result.ok) throw py::key_error();
        return result.value;
    })
    .def("__contains__",[](const MapStringInt& self,py::handle key){ return BrGet(self.handle,string_arg(key)).ok!=0; })
    .def("__setitem__",[](const MapStringInt& self,py::handle key,py::handle value){ BrSet(self.handle,string_arg(key),integer_arg<intptr_t>(value)); })
    .def("delete",[](const MapStringInt& self,py::handle key){ BrDelete(self.handle,string_arg(key)); },py::arg("key"))
    .def("clear",[](const MapStringInt& self){ BrClear(self.handle); })
    .def("clone",[](const MapStringInt& self){ return MapStringInt(BrClone(self.handle)); })
    .def("copy_to_py_items",[](const MapStringInt& self){
        size_t size=BrLen(self.handle);
        Strings keys(size); std::vector<intptr_t> values(size);
        BrItems(self.handle,keys.values.data(),values.data());
        py::list result;
        for (size_t i=0;i<size;++i) result.append(py::make_tuple(decode(keys.values[i]),values[i]));
        return result;
    });
    cls.attr("__iter__")=py::none();
    m.def("echo",[](const MapStringInt& value){ return MapStringInt(BrEcho(value.handle)); },py::arg("value"));
    m.def("put",[](const MapStringInt& value,py::handle key,py::handle item){ BrPut(value.handle,string_arg(key),integer_arg<intptr_t>(item)); },py::arg("value"),py::arg("key"),py::arg("item"));
    m.def("nil",[]{ return MapStringInt(BrNil()); });
    m.def("_holder_count",&BrHolderCount);
}
