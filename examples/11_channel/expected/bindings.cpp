#include "libbridge.h"

#include <pybind11/pybind11.h>
#include <cstdlib>
#include <limits>
#include <type_traits>

namespace py = pybind11;

// Convert while holding the GIL; call arguments stay alive during the Go call.
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
    if (value.size > static_cast<size_t>(PY_SSIZE_T_MAX)) throw py::value_error("Go string too large for Python");
    PyObject* result = PyUnicode_DecodeUTF8(value.data, static_cast<Py_ssize_t>(value.size), "strict");
    if (!result) throw py::error_already_set();
    return py::reinterpret_steal<py::str>(result);
}

#include <utility>

struct Ref {
    uintptr_t handle;
    explicit Ref(uintptr_t h) : handle(h) {}
    Ref(const Ref&) = delete;
    Ref(Ref&& other) noexcept : handle(std::exchange(other.handle, 0)) {}
    ~Ref() { BrRelease(handle); }
};
struct ChanInt : Ref { using Ref::Ref; };
struct SendChanInt : Ref { using Ref::Ref; };
struct RecvChanInt : Ref { using Ref::Ref; };
struct ChanString : Ref { using Ref::Ref; };

// Only the ABI call runs without the GIL. Python conversion and callbacks
// execute on the caller's thread with the GIL held.
template<typename F> auto go_call(F call) {
    py::gil_scoped_release release;
    return call();
}
template<typename T> void nil_members(py::class_<T, Ref>& cls) {
    cls.def_static("nil", [] { return T(0); })
       .def_property_readonly("is_nil", [](const T& self) { return self.handle == 0; })
       .def("__bool__", [](const T&) -> bool { throw py::type_error("use is_nil"); });
}
template<typename T, auto Receive> bool recv_int(const T& self, py::function callback) {
    auto result = go_call([&] { return Receive(self.handle); });
    if (!result.ok) return false;
    callback(py::int_(result.value));
    return true;
}
template<typename T, auto Send> void send_int(const T& self, py::handle value) {
    auto input = integer_arg<intptr_t>(value);
    go_call([&] { Send(self.handle, input); });
}

PYBIND11_MODULE(_native, m) {
    py::class_<Ref>(m, "Ref");
    auto both = py::class_<ChanInt, Ref>(m, "ChanInt", py::is_final());
    auto send = py::class_<SendChanInt, Ref>(m, "SendChanInt", py::is_final());
    auto recv = py::class_<RecvChanInt, Ref>(m, "RecvChanInt", py::is_final());
    auto text = py::class_<ChanString, Ref>(m, "ChanString", py::is_final());
    nil_members(both); nil_members(send); nil_members(recv); nil_members(text);
    both.def(py::init([](py::handle capacity) { return ChanInt(BrIntNew(integer_arg<intptr_t>(capacity))); }), py::arg("capacity") = 0)
        .def("send", &send_int<ChanInt, BrIntSend>, py::arg("value"))
        .def("recv", &recv_int<ChanInt, BrIntRecv>, py::arg("callback"))
        .def("close", [](const ChanInt& self) { BrIntClose(self.handle); })
        .def("send_only", [](const ChanInt& self) { return SendChanInt(BrIntSendOnly(self.handle)); })
        .def("recv_only", [](const ChanInt& self) { return RecvChanInt(BrIntRecvOnly(self.handle)); });
    send.def("send", &send_int<SendChanInt, BrSendIntSend>, py::arg("value"))
        .def("close", [](const SendChanInt& self) { BrSendIntClose(self.handle); });
    recv.def("recv", &recv_int<RecvChanInt, BrRecvIntRecv>, py::arg("callback"));
    text.def(py::init([](py::handle capacity) { return ChanString(BrTextNew(integer_arg<intptr_t>(capacity))); }), py::arg("capacity") = 0)
        .def("send", [](const ChanString& self, py::handle value) {
            auto input = string_arg(value);
            go_call([&] { BrTextSend(self.handle, input); });
        }, py::arg("value"))
        .def("recv", [](const ChanString& self, py::function callback) {
            auto result = go_call([&] { return BrTextRecv(self.handle); });
            // Own the received string even when decoding or the callback fails.
            struct Owner { BrString value; ~Owner() { std::free(value.data); } } owner{result.value};
            if (!result.ok) return false;
            callback(decode(result.value));
            return true;
        }, py::arg("callback"))
        .def("close", [](const ChanString& self) { BrTextClose(self.handle); });
    m.def("numbers", [](py::handle count) { return RecvChanInt(BrNumbers(integer_arg<intptr_t>(count))); }, py::arg("count"));
    m.def("sum", [](const RecvChanInt& ch) { return go_call([&] { return BrSum(ch.handle); }); }, py::arg("ch"));
    m.def("send", [](const SendChanInt& ch, py::handle value) {
        auto input = integer_arg<intptr_t>(value);
        go_call([&] { BrSend(ch.handle, input); });
    }, py::arg("ch"), py::arg("value"));
    m.def("echo", [](const ChanInt& ch) { return ChanInt(BrEcho(ch.handle)); }, py::arg("ch"));
    m.def("echo_text", [](const ChanString& ch) { return ChanString(BrEchoText(ch.handle)); }, py::arg("ch"));
    m.def("_holder_count", &BrHolderCount);
}
