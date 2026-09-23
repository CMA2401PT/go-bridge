#include <pybind11/pybind11.h>

#include "libbridge.h"

namespace py = pybind11;

namespace {

intptr_t int_in_and_out(intptr_t value) {
    return BasicIntInAndOut(value);
}

bool bool_in_and_out(bool value) {
    return BasicBoolInAndOut(value ? 1 : 0) != 0;
}

uint16_t basic_go_spec_type(uint16_t value) {
    return BasicGoSpecType(value);
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Native pybind11 bindings for the 01_basic Go library";

    module.def(
        "int_in_and_out",
        &int_in_and_out,
        py::arg("value").noconvert());
    module.def(
        "bool_in_and_out",
        &bool_in_and_out,
        py::arg("value").noconvert());
    module.def(
        "basic_go_spec_type",
        &basic_go_spec_type,
        py::arg("value").noconvert());
}
