#include <pybind11/pybind11.h>
#include <cstdlib>
#include <memory>

#include "libbridge.h"

namespace py = pybind11;

namespace {

SBString c_string(const py::str& input) {
    Py_ssize_t size;
    const char* data = PyUnicode_AsUTF8AndSize(input.ptr(), &size);
    if (!data) throw py::error_already_set();
    return SBString{const_cast<char*>(data), static_cast<size_t>(size)};
}

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

py::str recv_and_return(py::str input) {
    SBString encoded = c_string(input);
    return py_string(SBEchoString(encoded));
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Go strings through a by-value SBString ABI";
    module.def("recv_and_return", &recv_and_return, py::arg("value").noconvert());
}
