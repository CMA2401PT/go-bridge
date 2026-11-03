#include "libbridge.h"

#include <pybind11/pybind11.h>
#include <cstdlib>
#include <limits>
#include <utility>

namespace py = pybind11;

// 参数仍由 Python 持有；Go 在调用内复制 UTF-8 内容后才保存字符串。
BrString string_arg(py::handle value) {
    if (!PyUnicode_Check(value.ptr())) throw py::type_error("expected str");
    Py_ssize_t size;
    const char* data = PyUnicode_AsUTF8AndSize(value.ptr(), &size);
    if (!data) throw py::error_already_set();
    return {const_cast<char*>(data), static_cast<size_t>(size)};
}

// Python int 没有位宽限制，跨 ABI 时必须检查 Go int 能否容纳它。
intptr_t integer_arg(py::handle value) {
    if (!PyLong_Check(value.ptr())) throw py::type_error("expected int");
    int overflow = 0;
    auto number = PyLong_AsLongLongAndOverflow(value.ptr(), &overflow);
    if (overflow || PyErr_Occurred() ||
        number < std::numeric_limits<intptr_t>::min() ||
        number > std::numeric_limits<intptr_t>::max()) {
        PyErr_Clear();
        PyErr_SetString(PyExc_OverflowError, "integer outside Go int range");
        throw py::error_already_set();
    }
    return static_cast<intptr_t>(number);
}

// 解码或 Python 对象分配失败时，也要释放 Go 返回的 C 字符串。
struct StringOwner {
    BrString value;
    ~StringOwner() { std::free(value.data); }
};

py::str decode(BrString value) {
    PyObject* result = PyUnicode_DecodeUTF8(
        value.data, static_cast<Py_ssize_t>(value.size), "strict");
    if (!result) throw py::error_already_set();
    return py::reinterpret_steal<py::str>(result);
}

// 每个 Python 包装独占一个句柄；移动把释放责任交给新包装。
struct Ref {
    uintptr_t handle;
    explicit Ref(uintptr_t value) : handle(value) {}
    Ref(const Ref&) = delete;
    Ref(Ref&& other) noexcept : handle(std::exchange(other.handle, 0)) {}
    ~Ref() { BrRelease(handle); }
};

// 这些继承只表达包装类别；Dog、Cat 在 Python 里也不继承 Animal。
struct Ptr : Ref { using Ref::Ref; };
struct InterfaceRef : Ref { using Ref::Ref; };
struct GoAny : InterfaceRef { using InterfaceRef::InterfaceRef; };
struct Animal : InterfaceRef { using InterfaceRef::InterfaceRef; };
struct ErrorValue : InterfaceRef { using InterfaceRef::InterfaceRef; };
struct Dog : Ref { using Ref::Ref; };
struct Cat : Ref { using Ref::Ref; };
struct PtrDog : Ptr { using Ptr::Ptr; };
struct PtrCat : Ptr { using Ptr::Ptr; };

// Go 决定动态类型是否匹配；C++ 在分配 Python 包装前接管返回句柄。
template<class T, BrAssertion (*Assert)(uintptr_t)>
py::object assert_value(const InterfaceRef& value, bool optional) {
    auto result = Assert(value.handle);
    T owned(result.value);
    if (!result.ok) {
        if (optional) return py::none();
        throw py::type_error("Go interface does not satisfy the requested type");
    }
    return py::cast(std::move(owned));
}

PYBIND11_MODULE(_native, module) {
    py::class_<Ref>(module, "Ref");
    py::class_<Ptr, Ref>(module, "Ptr")
        .def_property_readonly("is_nil", [](const Ptr& value) {
            return value.handle == 0;
        });
    py::class_<InterfaceRef, Ref>(module, "GoInterface")
        .def_property_readonly("is_nil", [](const InterfaceRef& value) {
            return value.handle == 0;
        })
        .def("__bool__", [](const InterfaceRef&) -> bool {
            throw py::type_error("Go interfaces have no truth value; use is_nil");
        })
        .def("as_any", [](const InterfaceRef& value) {
            return GoAny(BrBox(value.handle));
        });
    py::class_<GoAny, InterfaceRef>(module, "GoAny", py::is_final())
        .def_static("nil", [] { return GoAny(0); });

    py::class_<Dog, Ref>(module, "Dog", py::is_final())
        .def(py::init([](py::handle name) {
            auto input = string_arg(name);
            py::gil_scoped_release release;
            return Dog(BrNewDog(input));
        }), py::kw_only(), py::arg("name") = "")
        .def_property("name", [](const Dog& value) {
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrDogName(value.handle);
            }()};
            return decode(output.value);
        }, [](const Dog& value, py::handle name) {
            auto input = string_arg(name);
            py::gil_scoped_release release;
            BrSetDogName(value.handle, input);
        })
        .def("addr", [](const Dog& value) {
            return PtrDog(BrDogAddr(value.handle));
        })
        .def_static("from_any", [](const InterfaceRef& value) {
            return assert_value<Dog, BrAssertDog>(value, false);
        }, py::arg("value"))
        .def_static("try_from_any", [](const InterfaceRef& value) {
            return assert_value<Dog, BrAssertDog>(value, true);
        }, py::arg("value"));

    py::class_<Cat, Ref>(module, "Cat", py::is_final())
        .def(py::init([](py::handle name) {
            auto input = string_arg(name);
            py::gil_scoped_release release;
            return Cat(BrNewCat(input));
        }), py::kw_only(), py::arg("name") = "")
        .def_property("name", [](const Cat& value) {
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrCatName(value.handle);
            }()};
            return decode(output.value);
        }, [](const Cat& value, py::handle name) {
            auto input = string_arg(name);
            py::gil_scoped_release release;
            BrSetCatName(value.handle, input);
        })
        .def("addr", [](const Cat& value) {
            return PtrCat(BrCatAddr(value.handle));
        })
        .def_static("from_any", [](const InterfaceRef& value) {
            return assert_value<Cat, BrAssertCat>(value, false);
        }, py::arg("value"))
        .def_static("try_from_any", [](const InterfaceRef& value) {
            return assert_value<Cat, BrAssertCat>(value, true);
        }, py::arg("value"));

    py::class_<PtrDog, Ptr>(module, "PtrDog", py::is_final())
        .def_property("name", [](const PtrDog& value) {
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrDogName(value.handle);
            }()};
            return decode(output.value);
        }, [](const PtrDog& value, py::handle name) {
            auto input = string_arg(name);
            py::gil_scoped_release release;
            BrSetDogName(value.handle, input);
        })
        .def_static("from_any", [](const InterfaceRef& value) {
            return assert_value<PtrDog, BrAssertPtrDog>(value, false);
        }, py::arg("value"))
        .def_static("try_from_any", [](const InterfaceRef& value) {
            return assert_value<PtrDog, BrAssertPtrDog>(value, true);
        }, py::arg("value"));
    py::class_<PtrCat, Ptr>(module, "PtrCat", py::is_final())
        .def_property("name", [](const PtrCat& value) {
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrCatName(value.handle);
            }()};
            return decode(output.value);
        }, [](const PtrCat& value, py::handle name) {
            auto input = string_arg(name);
            py::gil_scoped_release release;
            BrSetCatName(value.handle, input);
        })
        .def_static("from_any", [](const InterfaceRef& value) {
            return assert_value<PtrCat, BrAssertPtrCat>(value, false);
        }, py::arg("value"))
        .def_static("try_from_any", [](const InterfaceRef& value) {
            return assert_value<PtrCat, BrAssertPtrCat>(value, true);
        }, py::arg("value"));

    // 只注册 Go 静态可赋值的构造入口，不注册隐式转换。
    py::class_<Animal, InterfaceRef>(module, "Animal", py::is_final())
        .def(py::init([](const Animal& value) {
            return Animal(BrBox(value.handle));
        }), py::arg("value"))
        .def(py::init([](const Dog& value) {
            return Animal(BrBoxDog(value.handle));
        }), py::arg("value"))
        .def(py::init([](const PtrDog& value) {
            return Animal(BrBoxPtrDog(value.handle));
        }), py::arg("value"))
        .def(py::init([](const PtrCat& value) {
            return Animal(BrBoxPtrCat(value.handle));
        }), py::arg("value"))
        .def_static("nil", [] { return Animal(0); })
        .def_static("from_any", [](const InterfaceRef& value) {
            return assert_value<Animal, BrAssertAnimal>(value, false);
        }, py::arg("value"))
        .def_static("try_from_any", [](const InterfaceRef& value) {
            return assert_value<Animal, BrAssertAnimal>(value, true);
        }, py::arg("value"))
        .def("kind", [](const Animal& value) {
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrAnimalKind(value.handle);
            }()};
            return decode(output.value);
        })
        .def("speak", [](const Animal& value, py::handle times) {
            auto count = integer_arg(times);
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrAnimalSpeak(value.handle, count);
            }()};
            return decode(output.value);
        }, py::arg("times"));

    py::class_<ErrorValue, InterfaceRef>(module, "error", py::is_final())
        .def(py::init([](const ErrorValue& value) {
            return ErrorValue(BrBox(value.handle));
        }), py::arg("value"))
        .def_static("nil", [] { return ErrorValue(0); })
        .def_static("from_any", [](const InterfaceRef& value) {
            return assert_value<ErrorValue, BrAssertError>(value, false);
        }, py::arg("value"))
        .def_static("try_from_any", [](const InterfaceRef& value) {
            return assert_value<ErrorValue, BrAssertError>(value, true);
        }, py::arg("value"))
        .def("error", [](const ErrorValue& value) {
            StringOwner output{[&] {
                py::gil_scoped_release release;
                return BrError(value.handle);
            }()};
            return decode(output.value);
        });

    module.def("adopt", [](py::handle kind, py::handle name) {
        auto kind_input = string_arg(kind);
        auto name_input = string_arg(name);
        py::gil_scoped_release release;
        return Animal(BrAdopt(kind_input, name_input));
    }, py::arg("kind"), py::arg("name"));
    module.def("introduce", [](const Animal& animal) {
        StringOwner output{[&] {
            py::gil_scoped_release release;
            return BrIntroduce(animal.handle);
        }()};
        return decode(output.value);
    }, py::arg("animal"));
    module.def("nobody", [] { return Animal(BrNobody()); });
    module.def("stray", [] { return Animal(BrStray()); });
    module.def("feed", [](const Animal& animal, py::handle food) {
        auto input = string_arg(food);
        auto result = [&] {
            py::gil_scoped_release release;
            return BrFeed(animal.handle, input);
        }();
        StringOwner output{result.value};
        ErrorValue error(result.error);
        return py::make_tuple(decode(output.value), std::move(error));
    }, py::arg("animal"), py::arg("food"));
    module.def("_holder_count", &BrHolderCount);
}
