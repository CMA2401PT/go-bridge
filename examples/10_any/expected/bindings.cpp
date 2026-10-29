#include <pybind11/pybind11.h>
#include <cstdlib>
#include <limits>
#include <memory>
#include <string>
#include <type_traits>
#include <utility>
#include <vector>

#include "libbridge.h"

namespace py = pybind11;

// The GIL stays held. Python arguments and their UTF-8 caches remain alive
// until Go has copied them. No Python buffer is retained by Go.
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

namespace {

// Same opaque-handle ownership as the current generator.
// No native handle or address is exposed through Python.
struct Ref {
    uintptr_t handle;
    explicit Ref(uintptr_t value) : handle(value) {}
    Ref(const Ref&) = delete;
    Ref& operator=(const Ref&) = delete;
    Ref(Ref&& other) noexcept : handle(std::exchange(other.handle, 0)) {}
    ~Ref() { BrRelease(handle); }
};
struct InterfaceRef : Ref { using Ref::Ref; };
struct Slice : Ref { using Ref::Ref; };
struct Ptr : Ref { using Ref::Ref; };
template<int ID, class Base> struct GoObject : Base { using Base::Base; };
using GoAny = GoObject<0, InterfaceRef>;
using SliceInt = GoObject<1, Slice>;
using MySlice = GoObject<2, Slice>;
using Point = GoObject<3, Ref>;
using PtrInt = GoObject<4, Ptr>;
using PtrPoint = GoObject<5, Ptr>;
using PtrPtrInt = GoObject<6, Ptr>;
using IntPtr = GoObject<7, Ptr>;

template<class T> const T& checked_ref(py::handle value) {
    if (!py::isinstance<T>(value)) throw py::type_error("expected the matching Go wrapper type");
    return py::cast<const T&>(value);
}
template<class T> uintptr_t handle_arg(py::handle value) { return checked_ref<T>(value).handle; }

template<class T> py::object wrap(uintptr_t handle) {
    // The temporary owns the result even if building the Python object fails.
    return py::cast(T(handle));
}

// String ownership precedes all decoding, including invalid UTF-8 failures.
py::str owned_string(BrString value) {
    std::unique_ptr<char, decltype(&std::free)> owner(value.data, &std::free);
    return decode(value);
}

[[noreturn]] void assertion_failed(const char* type) {
    throw py::type_error(std::string("Go any does not contain ") + type);
}

template<class Result, class Convert>
py::object asserted(Result result, bool attempt, const char* type, Convert convert) {
    if (!result.ok) {
        if (attempt) return py::none();
        assertion_failed(type);
    }
    return convert(result.value);
}

template<class T, class Class>
void define_any(Class& cls, uintptr_t (*box)(uintptr_t), BrHandleResult (*unbox)(uintptr_t), const char* type) {
    cls.def("as_any", [box](py::handle self) { return wrap<GoAny>(box(handle_arg<T>(self))); });
    cls.def_static("from_any", [unbox, type](py::handle value) {
        return asserted(unbox(handle_arg<InterfaceRef>(value)), false, type, &wrap<T>);
    }, py::arg("value"));
    cls.def_static("try_from_any", [unbox, type](py::handle value) {
        return asserted(unbox(handle_arg<InterfaceRef>(value)), true, type, &wrap<T>);
    }, py::arg("value"));
}

constexpr auto as_int = [](auto value) { return py::int_(value); };
constexpr auto as_float = [](auto value) { return py::float_(value); };

// Defines T_as_any, any_as_T and try_any_as_T for one Go basic type.
template<class Arg, class Result, class Convert>
void define_scalar(py::module_& module, const std::string& type, uintptr_t (*box)(Arg),
                   Result (*unbox)(uintptr_t), Arg (*input)(py::handle), Convert convert) {
    // Each lambda keeps its own copy of the Go type name for error messages.
    module.def((type + "_as_any").c_str(), [box, input](py::handle value) {
        return wrap<GoAny>(box(input(value)));
    }, py::arg("value"));
    module.def(("any_as_" + type).c_str(), [unbox, type, convert](py::handle value) {
        return asserted(unbox(handle_arg<InterfaceRef>(value)), false, type.c_str(), convert);
    }, py::arg("value"));
    module.def(("try_any_as_" + type).c_str(), [unbox, type, convert](py::handle value) {
        return asserted(unbox(handle_arg<InterfaceRef>(value)), true, type.c_str(), convert);
    }, py::arg("value"));
}

template<class T, class Class> void define_nil(Class& cls) {
    cls.def_static("nil", [] { return T(0); })
       .def_property_readonly("is_nil", [](py::handle self) { return handle_arg<T>(self) == 0; })
       .def("__bool__", [](py::handle self) -> bool {
           checked_ref<T>(self);
           throw py::type_error("use is_nil to test a Go nil value");
       });
}

inline size_t slice_index(py::handle index, size_t size) {
    auto number = py::reinterpret_steal<py::object>(PyNumber_Index(index.ptr()));
    if (!number) throw py::error_already_set();
    Py_ssize_t n = PyNumber_AsSsize_t(number.ptr(), PyExc_IndexError);
    if (n == -1 && PyErr_Occurred()) throw py::error_already_set();
    if (n < 0) n += static_cast<Py_ssize_t>(size);
    if (n < 0 || static_cast<size_t>(n) >= size) throw py::index_error("Go slice index out of range");
    return static_cast<size_t>(n);
}

template<class T, class Class> void slice_members(Class& cls) {
    cls.def("__len__", [](py::handle self) { return BrSliceLen(handle_arg<T>(self)); })
       .def("__getitem__", [](py::handle self, py::handle index) {
           auto handle = handle_arg<T>(self);
           return BrSliceGet(handle, slice_index(index, BrSliceLen(handle)));
       })
       .def("__setitem__", [](py::handle self, py::handle index, py::handle value) {
           auto handle = handle_arg<T>(self);
           auto position = slice_index(index, BrSliceLen(handle));
           auto item = integer_arg<intptr_t>(value);
           BrSliceSet(handle, position, item);
       })
       .def("copy_to_py_list", [](py::handle self) {
           auto handle = handle_arg<T>(self);
           std::vector<intptr_t> values(BrSliceLen(handle));
           BrSliceCopyToPy(handle, values.data());
           py::list result;
           for (auto value : values) result.append(value);
           return result;
       });
    cls.attr("__iter__") = py::none();
}

template<class T, class Class> void int_pointer_members(Class& cls) {
    cls.def_property("value", [](py::handle self) { return BrPtrIntLoad(handle_arg<T>(self)); },
        [](py::handle self, py::handle value) {
            auto handle = handle_arg<T>(self);
            auto item = integer_arg<intptr_t>(value);
            BrPtrIntStore(handle, item);
        });
}

template<class T, class Class> void point_members(Class& cls) {
    cls.def_property("x", [](py::handle self) { return BrPointX(handle_arg<T>(self)); },
        [](py::handle self, py::handle value) {
            auto handle = handle_arg<T>(self);
            auto item = integer_arg<intptr_t>(value);
            BrPointSetX(handle, item);
        })
       .def_property("values", [](py::handle self) { return wrap<SliceInt>(BrPointValues(handle_arg<T>(self))); },
        [](py::handle self, py::handle value) {
            auto handle = handle_arg<T>(self);
            auto values = handle_arg<SliceInt>(value);
            BrPointSetValues(handle, values);
        })
       .def_property("data", [](py::handle self) { return wrap<GoAny>(BrPointData(handle_arg<T>(self))); },
        [](py::handle self, py::handle value) {
            auto handle = handle_arg<T>(self);
            auto data = handle_arg<GoAny>(value);
            BrPointSetData(handle, data);
        });
}

} // namespace

PYBIND11_MODULE(_native, module) {
    module.doc() = "Explicit Go any boxing and exact dynamic type assertions";
    py::class_<Ref>(module, "Ref");
    py::class_<Slice, Ref>(module, "Slice");
    py::class_<Ptr, Ref>(module, "Ptr");
    py::class_<InterfaceRef, Ref>(module, "GoInterface")
       .def_property_readonly("is_nil", [](const InterfaceRef& value) { return value.handle == 0; })
       .def("__bool__", [](const InterfaceRef&) -> bool {
           throw py::type_error("use is_nil to test a Go nil value");
       })
       .def("as_any", [](const InterfaceRef& value) {
           return wrap<GoAny>(BrInterfaceAsAny(value.handle));
       });
    py::class_<GoAny, InterfaceRef> any(module, "GoAny", py::is_final());
    py::class_<SliceInt, Slice> slice(module, "SliceInt", py::is_final());
    py::class_<MySlice, Slice> named_slice(module, "MySlice", py::is_final());
    py::class_<Point, Ref> point(module, "Point", py::is_final());
    py::class_<PtrInt, Ptr> int_ptr(module, "PtrInt", py::is_final());
    py::class_<PtrPoint, Ptr> point_ptr(module, "PtrPoint", py::is_final());
    py::class_<PtrPtrInt, Ptr> double_ptr(module, "PtrPtrInt", py::is_final());
    py::class_<IntPtr, Ptr> named_ptr(module, "IntPtr", py::is_final());
    any.def_static("nil", [] { return GoAny(0); });
    define_nil<PtrInt>(int_ptr);
    define_nil<PtrPoint>(point_ptr);
    define_nil<PtrPtrInt>(double_ptr);
    define_nil<IntPtr>(named_ptr);
    define_any<SliceInt>(slice, &BrSliceIntAsAny, &BrTryAnyAsSliceInt, "[]int");
    define_any<MySlice>(named_slice, &BrMySliceAsAny, &BrTryAnyAsMySlice, "anyvalue.MySlice");
    define_any<Point>(point, &BrPointAsAny, &BrTryAnyAsPoint, "anyvalue.Point");
    define_any<PtrInt>(int_ptr, &BrPtrIntAsAny, &BrTryAnyAsPtrInt, "*int");
    define_any<PtrPoint>(point_ptr, &BrPtrPointAsAny, &BrTryAnyAsPtrPoint, "*anyvalue.Point");
    define_any<PtrPtrInt>(double_ptr, &BrPtrPtrIntAsAny, &BrTryAnyAsPtrPtrInt, "**int");
    define_any<IntPtr>(named_ptr, &BrIntPtrAsAny, &BrTryAnyAsIntPtr, "anyvalue.IntPtr");

    slice.def(py::init([](py::iterable input) {
        std::vector<intptr_t> values;
        for (auto value : input) values.push_back(integer_arg<intptr_t>(value));
        return SliceInt(BrNewSlice(values.data(), values.size()));
    }), py::arg("values"));
    slice_members<SliceInt>(slice);
    named_slice.def(py::init([](py::handle value) { return MySlice(BrCopySlice(handle_arg<SliceInt>(value))); }), py::arg("value"))
       .def("unname", [](py::handle self) { return wrap<SliceInt>(BrCopySlice(handle_arg<MySlice>(self))); });
    slice_members<MySlice>(named_slice);

    point.def(py::init([](py::kwargs args) {
        intptr_t x = 0;
        uintptr_t values = 0, data = 0;
        for (auto entry : args) {
            auto name = py::cast<std::string>(entry.first);
            if (name == "x") x = integer_arg<intptr_t>(entry.second);
            else if (name == "values") values = handle_arg<SliceInt>(entry.second);
            else if (name == "data") data = handle_arg<GoAny>(entry.second);
            else throw py::type_error("unexpected Point constructor keyword: " + name);
        }
        return Point(BrNewPoint(x, values, data));
    }))
       .def("addr", [](py::handle self) { return wrap<PtrPoint>(BrPointView(handle_arg<Point>(self))); })
       .def("__copy__", [](py::handle self) { return wrap<Point>(BrPointCopy(handle_arg<Point>(self))); });
    point_members<Point>(point);
    point_members<PtrPoint>(point_ptr);
    point_ptr.def(py::init([](py::handle value) { return PtrPoint(BrPointCopy(handle_arg<Point>(value))); }), py::arg("value"))
       .def_static("from_value", [](py::handle value) { return wrap<PtrPoint>(BrPointCopy(handle_arg<Point>(value))); }, py::arg("value"))
       .def_property("value", [](py::handle self) { return wrap<Point>(BrPointView(handle_arg<PtrPoint>(self))); },
           [](py::handle self, py::handle value) {
               auto handle = handle_arg<PtrPoint>(self);
               auto source = handle_arg<Point>(value);
               BrPtrPointStore(handle, source);
           });

    int_ptr.def(py::init([](py::handle value) { return PtrInt(BrNewPtrInt(integer_arg<intptr_t>(value))); }), py::arg("value") = 0)
       .def_static("from_value", [](py::handle value) { return wrap<PtrInt>(BrNewPtrInt(integer_arg<intptr_t>(value))); }, py::arg("value"))
       .def("ptr", [](py::handle self) { return wrap<PtrPtrInt>(BrNewPtrPtrInt(handle_arg<PtrInt>(self))); });
    int_pointer_members<PtrInt>(int_ptr);
    named_ptr.def(py::init([](py::handle value) { return IntPtr(BrCopyPointer(handle_arg<PtrInt>(value))); }), py::arg("value"))
       .def("unname", [](py::handle self) { return wrap<PtrInt>(BrCopyPointer(handle_arg<IntPtr>(self))); });
    int_pointer_members<IntPtr>(named_ptr);
    double_ptr.def(py::init([](py::handle value) { return PtrPtrInt(BrNewPtrPtrInt(handle_arg<PtrInt>(value))); }), py::arg("value"))
       .def_static("from_value", [](py::handle value) { return wrap<PtrPtrInt>(BrNewPtrPtrInt(handle_arg<PtrInt>(value))); }, py::arg("value"))
       .def_property("value", [](py::handle self) { return wrap<PtrInt>(BrPtrPtrIntLoad(handle_arg<PtrPtrInt>(self))); },
           [](py::handle self, py::handle value) {
               auto handle = handle_arg<PtrPtrInt>(self);
               auto source = handle_arg<PtrInt>(value);
               BrPtrPtrIntStore(handle, source);
           });

    module.def("_check_int", &integer_arg<intptr_t>);
    module.def("_my_int_as_any", [](py::handle value) { return wrap<GoAny>(BrMyIntAsAny(integer_arg<intptr_t>(value))); });
    module.def("_any_as_my_int", [](py::handle value) {
        return asserted(BrTryAnyAsMyInt(handle_arg<InterfaceRef>(value)), false, "anyvalue.MyInt", as_int);
    });
    module.def("_try_any_as_my_int", [](py::handle value) {
        return asserted(BrTryAnyAsMyInt(handle_arg<InterfaceRef>(value)), true, "anyvalue.MyInt", as_int);
    });
    define_scalar(module, "int", &BrIntAsAny, &BrTryAnyAsInt, &integer_arg<intptr_t>, as_int);
    define_scalar(module, "int8", &BrInt8AsAny, &BrTryAnyAsInt8, &integer_arg<int8_t>, as_int);
    define_scalar(module, "int16", &BrInt16AsAny, &BrTryAnyAsInt16, &integer_arg<int16_t>, as_int);
    define_scalar(module, "int32", &BrInt32AsAny, &BrTryAnyAsInt32, &integer_arg<int32_t>, as_int);
    define_scalar(module, "int64", &BrInt64AsAny, &BrTryAnyAsInt64, &integer_arg<int64_t>, as_int);
    define_scalar(module, "uint", &BrUintAsAny, &BrTryAnyAsUint, &integer_arg<uintptr_t>, as_int);
    define_scalar(module, "uint8", &BrUint8AsAny, &BrTryAnyAsUint8, &integer_arg<uint8_t>, as_int);
    define_scalar(module, "uint16", &BrUint16AsAny, &BrTryAnyAsUint16, &integer_arg<uint16_t>, as_int);
    define_scalar(module, "uint32", &BrUint32AsAny, &BrTryAnyAsUint32, &integer_arg<uint32_t>, as_int);
    define_scalar(module, "uint64", &BrUint64AsAny, &BrTryAnyAsUint64, &integer_arg<uint64_t>, as_int);
    define_scalar(module, "uintptr", &BrUintptrAsAny, &BrTryAnyAsUintptr, &integer_arg<uintptr_t>, as_int);
    define_scalar(module, "float32", &BrFloat32AsAny, &BrTryAnyAsFloat32, &float_arg<float>, as_float);
    define_scalar(module, "float64", &BrFloat64AsAny, &BrTryAnyAsFloat64, &float_arg<double>, as_float);
    define_scalar(module, "bool", &BrBoolAsAny, &BrTryAnyAsBool, &bool_arg, [](uint8_t v) { return py::bool_(v != 0); });
    define_scalar(module, "string", &BrStringAsAny, &BrTryAnyAsString, &string_arg, &owned_string);

    module.def("echo", [](py::handle value) { return wrap<GoAny>(BrEcho(handle_arg<GoAny>(value))); }, py::arg("value"));
    module.def("dynamic_kind", [](py::handle value) { return owned_string(BrDynamicKind(handle_arg<GoAny>(value))); }, py::arg("value"));
    module.def("nil_any", [] { return wrap<GoAny>(BrNilAny()); });
    module.def("nil_pointer", [] { return wrap<GoAny>(BrNilPointer()); });
    module.def("nil_slice", [] { return wrap<GoAny>(BrNilSlice()); });
    module.def("empty_slice", [] { return wrap<GoAny>(BrEmptySlice()); });
    module.def("unknown", [] { return wrap<GoAny>(BrUnknown()); });
    module.def("slice_is_nil", [](py::handle value) { return BrSliceIsNil(handle_arg<GoAny>(value)) != 0; }, py::arg("value"));
    module.def("invalid_string", [] { return wrap<GoAny>(BrInvalidString()); });
    module.def("unknown_value", [](py::handle value) { return BrUnknownValue(handle_arg<GoAny>(value)); }, py::arg("value"));
    module.def("_echo_my_int", [](py::handle value) { return BrEchoMyInt(integer_arg<intptr_t>(value)); });
    module.def("echo_my_slice", [](py::handle value) { return wrap<MySlice>(BrEchoMySlice(handle_arg<MySlice>(value))); }, py::arg("value"));
    module.def("echo_int_ptr", [](py::handle value) { return wrap<IntPtr>(BrEchoIntPtr(handle_arg<IntPtr>(value))); }, py::arg("value"));
    module.def("echo_point", [](py::handle value) { return wrap<Point>(BrEchoPoint(handle_arg<Point>(value))); }, py::arg("value"));
    module.def("echo_double_pointer", [](py::handle value) { return wrap<PtrPtrInt>(BrEchoDoublePointer(handle_arg<PtrPtrInt>(value))); }, py::arg("value"));
    module.def("_holder_count", &BrHolderCount);
    module.def("_collect_go", &BrCollect);
}
