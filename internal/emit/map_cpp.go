package emit

import (
	"fmt"
	"go-bridge/internal/abi"
	"strings"
)

func cppMap(b *strings.Builder, m *abi.Map) {
	t, k, e := m.Type, m.Key, m.Elem
	cls := classVar(t)
	fmt.Fprintf(b, "%s.def_static(\"nil\",[]{return %s(0);});\n%s.def_property_readonly(\"is_nil\",[](py::handle self){return handle_arg<%s>(self)==0;});\n%s.attr(\"__iter__\")=py::none();\n", cls, t.Cpp, cls, t.Cpp, cls)
	if m.New != "" {
		fmt.Fprintf(b, `%s.def(py::init([=](py::iterable items){
 std::vector<py::object> keep;
 std::vector<%s> keys; std::vector<%s> values;
 for (auto item:items) {
  auto pair=py::cast<py::tuple>(item);
  if (pair.size()!=2) throw py::value_error("expected key/value pairs");
  keep.push_back(pair);
  keys.push_back(%s(pair[0])); values.push_back(%s(pair[1]));
 }
 return %s(%s(keys.data(),values.data(),keys.size()));
}),py::arg("items")=py::tuple());
`, cls, k.C, e.C, cppInput(k), cppInput(e), t.Cpp, m.New)
	}
	fmt.Fprintf(b, `%s.def("__len__",[](py::handle self){return %s(handle_arg<%s>(self));});
%s.def("__getitem__",[=](py::handle self,py::handle key)->py::object {
 auto handle=handle_arg<%s>(self);auto input=%s(key);
 OwnedBuffer<%s,%s> owner(1);
 auto result=%s(handle,input);owner.values[0]=result.value;
 if (!result.ok) throw py::key_error();
 return %s;
});
%s.def("__contains__",[=](py::handle self,py::handle key){ return %s(handle_arg<%s>(self),%s(key))!=0; });
%s.def("__setitem__",[=](py::handle self,py::handle key,py::handle value){ %s(handle_arg<%s>(self),%s(key),%s(value)); });
%s.def("delete",[=](py::handle self,py::handle key){ %s(handle_arg<%s>(self),%s(key)); },py::arg("key"));
%s.def("clear",[](py::handle self){ %s(handle_arg<%s>(self)); });
%s.def("clone",[=](py::handle self)->py::object {auto input=handle_arg<%s>(self);
`, cls, m.Len, t.Cpp, cls, t.Cpp, cppInput(k), e.C, cppDeleter(e), m.Get, cppOutput(e, "owner.values[0]"), cls, m.Has, t.Cpp, cppInput(k), cls, m.Set, t.Cpp, cppInput(k), cppInput(e), cls, m.Delete, t.Cpp, cppInput(k), cls, m.Clear, t.Cpp, cls, t.Cpp)
	cppOwnedCall(b, t, m.Clone+"(input)")
	b.WriteString("});\n")
	fmt.Fprintf(b, `%s.def("copy_to_py_items",[=](py::handle self){
 auto handle=handle_arg<%s>(self);auto size=%s(handle);
 OwnedBuffer<%s,%s> keys(size);OwnedBuffer<%s,%s> values(size);
 %s(handle,keys.values.data(),values.values.data());
 py::list result;
 for (size_t i=0;i<size;++i) result.append(py::make_tuple(%s,%s));
 return result;
});
`, cls, t.Cpp, m.Len, k.C, cppDeleter(k), e.C, cppDeleter(e), m.Items, cppOutput(k, "keys.values[i]"), cppOutput(e, "values.values[i]"))
}
