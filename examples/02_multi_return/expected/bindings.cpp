#include <pybind11/pybind11.h>

#include "libbridge.h"

namespace py = pybind11;

namespace {

py::tuple return_two(intptr_t value) {
    const auto result = MultiReturnTwo(value);
    return py::make_tuple(result.value1, result.value2);
}

py::tuple return_four(intptr_t value) {
    const auto result = MultiReturnFour(value);
    return py::make_tuple(result.value1, result.value2, result.value3, result.value4);
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Native pybind11 bindings for the 02_multi_return Go library";

    module.def("return_two", &return_two, py::arg("value").noconvert());
    module.def("return_four", &return_four, py::arg("value").noconvert());
}
