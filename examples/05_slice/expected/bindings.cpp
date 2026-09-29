#include <pybind11/pybind11.h>
#include <utility>
#include <vector>

#include "libbridge.h"

namespace py = pybind11;

namespace {

// Shared lifecycle base for every Python wrapper of a Go-owned value.
// Ref only tells Go when this wrapper no longer needs its pinned value.
class Ref {
    uintptr_t id_;

protected:
    explicit Ref(uintptr_t id) : id_(id) {
        if (!id_) throw py::value_error("cannot create Go Ref");
    }

public:
    Ref(const Ref&) = delete;
    Ref& operator=(const Ref&) = delete;
    Ref(Ref&& other) noexcept : id_(std::exchange(other.id_, 0)) {}
    virtual ~Ref() {
        if (id_) BrUnPin(id_);
    }
    // For binding code only; never exposed to Python.
    uintptr_t handle() const { return id_; }
};

// Common base of Go slice wrappers; each element type has its own subclass.
// It maps Python indices and slices to Go indices; Go semantics stay unchanged.
class Slice : public Ref {
protected:
    using Ref::Ref;

    // Negative indices count from the end; out of range raises IndexError.
    size_t element_index(py::ssize_t index) const {
        const auto length = static_cast<py::ssize_t>(size());
        if (index < 0) index += length;
        if (index < 0 || index >= length) throw py::index_error("slice index out of range");
        return static_cast<size_t>(index);
    }

    // Bounds are clamped like a Python list slice, so a reslice never reaches
    // into spare capacity. Go has no step, so only step 1 is accepted.
    std::pair<size_t, size_t> bounds(const py::slice& range) const {
        py::ssize_t start = 0, stop = 0, step = 0, count = 0;
        if (PySlice_Unpack(range.ptr(), &start, &stop, &step) != 0) throw py::error_already_set();
        if (step != 1) throw py::value_error("Go slices do not support a slice step");
        count = PySlice_AdjustIndices(static_cast<py::ssize_t>(size()), &start, &stop, step);
        return {static_cast<size_t>(start), static_cast<size_t>(start + count)};
    }

public:
    virtual size_t size() const = 0;
};

// Go int is intptr_t on the C side; bool is accepted as an int subclass.
intptr_t go_int(py::handle item) {
    py::detail::make_caster<intptr_t> element;
    if (!element.load(item, false)) {
        throw py::type_error("SliceInt elements must be int values within Go int range");
    }
    return py::detail::cast_op<intptr_t>(element);
}

// []int: elements are Python ints copied to and from Go by value.
class SliceInt : public Slice {
public:
    explicit SliceInt(uintptr_t id) : Slice(id) {}
    SliceInt(SliceInt&&) noexcept = default;

    static SliceInt from_values(const py::iterable& values) {
        std::vector<intptr_t> data;
        for (py::handle item : values) data.push_back(go_int(item));
        return SliceInt(BrNewSliceInt(data.data(), data.size()));
    }

    size_t size() const override { return BrSliceIntLen(handle()); }
    intptr_t get(py::ssize_t index) const {
        intptr_t value = 0;
        if (BrSliceIntAt(handle(), element_index(index), &value) != 0) {
            throw py::index_error("slice index out of range");
        }
        return value;
    }
    void set(py::ssize_t index, py::handle value) {
        const auto position = element_index(index);
        if (BrSliceIntSet(handle(), position, go_int(value)) != 0) {
            throw py::index_error("slice index out of range");
        }
    }
    SliceInt reslice(const py::slice& range) const {
        const auto [low, high] = bounds(range);
        return SliceInt(BrSliceIntSlice(handle(), low, high));
    }
    SliceInt clone() const { return SliceInt(BrSliceIntClone(handle())); }
    py::list copy_to_py_list() const {
        std::vector<intptr_t> data(size());
        if (BrCopySliceInt(handle(), data.data(), data.size()) != 0) {
            throw py::value_error("cannot copy Go slice");
        }
        py::list result;
        for (intptr_t value : data) result.append(value);
        return result;
    }
};

// [][]int: elements are Go-owned []int values, each wrapped as a SliceInt.
class SliceSliceInt : public Slice {
public:
    explicit SliceSliceInt(uintptr_t id) : Slice(id) {}
    SliceSliceInt(SliceSliceInt&&) noexcept = default;

    static SliceSliceInt from_values(const py::iterable& values) {
        // Keep every element alive until Go has read its handle, even when
        // the iterable produces temporaries.
        std::vector<py::object> elements;
        std::vector<uintptr_t> ids;
        for (py::handle item : values) {
            if (!py::isinstance<SliceInt>(item)) {
                throw py::type_error("SliceSliceInt elements must be SliceInt");
            }
            elements.push_back(py::reinterpret_borrow<py::object>(item));
            ids.push_back(item.cast<const SliceInt&>().handle());
        }
        return SliceSliceInt(BrNewSliceSliceInt(ids.data(), ids.size()));
    }

    size_t size() const override { return BrSliceSliceIntLen(handle()); }
    // Each read pins the element header again; the new SliceInt shares its array.
    SliceInt get(py::ssize_t index) const {
        return SliceInt(BrSliceSliceIntAt(handle(), element_index(index)));
    }
    void set(py::ssize_t index, const SliceInt& value) {
        if (BrSliceSliceIntSet(handle(), element_index(index), value.handle()) != 0) {
            throw py::index_error("slice index out of range");
        }
    }
    SliceSliceInt reslice(const py::slice& range) const {
        const auto [low, high] = bounds(range);
        return SliceSliceInt(BrSliceSliceIntSlice(handle(), low, high));
    }
    SliceSliceInt clone() const { return SliceSliceInt(BrSliceSliceIntClone(handle())); }
    py::list copy_to_py_list() const {
        std::vector<uintptr_t> ids(size());
        std::vector<SliceInt> elements;
        elements.reserve(ids.size());
        if (BrSliceSliceIntItems(handle(), ids.data(), ids.size()) != 0) {
            throw py::value_error("cannot copy Go slice");
        }
        // Take ownership of every new handle before any Python allocation.
        for (uintptr_t id : ids) elements.emplace_back(id);
        py::list result;
        for (auto& element : elements) result.append(py::cast(std::move(element)));
        return result;
    }
};

SliceInt concat_a(const SliceInt& a, const SliceInt& b) {
    return SliceInt(BrConcatA(a.handle(), b.handle()));
}

SliceSliceInt concat_b(const SliceSliceInt& a, const SliceSliceInt& b) {
    return SliceSliceInt(BrConcatB(a.handle(), b.handle()));
}

}  // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Go-owned slices of values and of Go-owned slices";
    py::class_<Ref>(module, "Ref");
    // Element-by-element iteration costs one cgo call per item (and a pin for
    // nested slices); copy_to_py_list() is the bulk read, so iteration is off.
    py::class_<Slice, Ref>(module, "Slice")
        .def("__len__", &Slice::size)
        .attr("__iter__") = py::none();
    py::class_<SliceInt, Slice>(module, "SliceInt")
        .def(py::init(&SliceInt::from_values), py::arg("values"))
        .def("__getitem__", &SliceInt::get, py::arg("index"))
        .def("__getitem__", &SliceInt::reslice, py::arg("index"))
        .def("__setitem__", &SliceInt::set, py::arg("index"), py::arg("value"))
        .def("clone", &SliceInt::clone)
        .def("copy_to_py_list", &SliceInt::copy_to_py_list);
    py::class_<SliceSliceInt, Slice>(module, "SliceSliceInt")
        .def(py::init(&SliceSliceInt::from_values), py::arg("values"))
        .def("__getitem__", &SliceSliceInt::get, py::arg("index"))
        .def("__getitem__", &SliceSliceInt::reslice, py::arg("index"))
        .def("__setitem__", &SliceSliceInt::set, py::arg("index"),
             py::arg("value").noconvert().none(false))
        .def("clone", &SliceSliceInt::clone)
        .def("copy_to_py_list", &SliceSliceInt::copy_to_py_list);
    module.def("concat_a", &concat_a, py::arg("a").noconvert().none(false),
               py::arg("b").noconvert().none(false));
    module.def("concat_b", &concat_b, py::arg("a").noconvert().none(false),
               py::arg("b").noconvert().none(false));
    module.def("_holder_count", &BrHolderCount);
    module.def("_collect_go", &BrCollect);
}
