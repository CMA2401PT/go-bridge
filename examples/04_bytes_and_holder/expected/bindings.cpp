#include <pybind11/pybind11.h>
#include <utility>

#include "libbridge.h"

namespace py = pybind11;

namespace {

// Shared lifecycle base for every Python wrapper of a Go-owned value.
// Ref only tells Go when this wrapper no longer needs its pinned value.
class Ref {
    uintptr_t id_;

protected:
    explicit Ref(uintptr_t id) : id_(id) {}
    uintptr_t handle() const { return id_; }

public:
    Ref(const Ref&) = delete;
    Ref& operator=(const Ref&) = delete;
    Ref(Ref&& other) noexcept : id_(std::exchange(other.id_, 0)) {}
    virtual ~Ref() {
        if (id_) BrUnPin(id_);
    }
};

class SliceBytes : public Ref {
public:
    explicit SliceBytes(uintptr_t id) : Ref(id) {}
    explicit SliceBytes(py::bytes input) : Ref(BrNewBytes(
        PyBytes_AS_STRING(input.ptr()), static_cast<size_t>(PyBytes_GET_SIZE(input.ptr())))) {}
    SliceBytes(SliceBytes&&) noexcept = default;

    size_t size() const { return BrBytesLen(handle()); }
    py::bytes copy_to_py_bytes() const {
        const auto length = size();
        PyObject* raw = PyBytes_FromStringAndSize(nullptr, static_cast<Py_ssize_t>(length));
        if (!raw) throw py::error_already_set();
        auto result = py::reinterpret_steal<py::bytes>(raw);
        BrCopyBytes(handle(), PyBytes_AS_STRING(raw));
        return result;
    }
    SliceBytes echo() const { return SliceBytes(BrEchoBytes(handle())); }
};

SliceBytes recv_and_return_bytes(const SliceBytes& input) {
    return input.echo();
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Ref lifetimes and Go-owned byte slices";
    py::class_<Ref>(module, "Ref");
    py::class_<SliceBytes, Ref>(module, "SliceBytes")
        .def(py::init<py::bytes>(), py::arg("value").noconvert())
        .def("__len__", &SliceBytes::size)
        .def("copy_to_py_bytes", &SliceBytes::copy_to_py_bytes);
    module.def("recv_and_return_bytes", &recv_and_return_bytes, py::arg("value").noconvert());
    module.def("_holder_count", &BrHolderCount);
    module.def("_collect_go", &BrCollect);
}
