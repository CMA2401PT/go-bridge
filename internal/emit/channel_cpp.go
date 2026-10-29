package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
)

func cppChannel(b *strings.Builder, c *abi.Channel) {
	t, cls := c.Type, classVar(c.Type)
	cppNilMembers(b, t, "Go channels")
	if c.New != "" {
		fmt.Fprintf(b, `%s.def(py::init([](py::handle capacity) {
    auto size = integer_arg<intptr_t>(capacity);
    return %s(%s(size));
}), py::arg("capacity") = 0);
`, cls, t.Cpp, c.New)
	}
	if c.Send != "" {
		fmt.Fprintf(b, `%s.def("send", [=](py::handle self, py::handle value) {
    auto handle = handle_arg<%s>(self);
    auto input = %s(value);
    py::gil_scoped_release release;
    %s(handle, input);
}, py::arg("value"));
%s.def("close", [](py::handle self) {
    auto handle = handle_arg<%s>(self);
    py::gil_scoped_release release;
    %s(handle);
});
`, cls, t.Cpp, cppInput(c.Elem), c.Send, cls, t.Cpp, c.Close)
	}
	if c.Recv != "" {
		// Allocate the owner before waiting. Once received, an element belongs
		// to this call even if Python conversion or the callback raises.
		fmt.Fprintf(b, `%s.def("recv", [=](py::handle self, py::function callback) {
    auto handle = handle_arg<%s>(self);
    OwnedBuffer<%s, %s> owner(1);
    auto result = [&] { py::gil_scoped_release release; return %s(handle); }();
    owner.values[0] = result.value;
    if (!result.ok) return false;
    callback(%s);
    return true;
}, py::arg("callback"));
`, cls, t.Cpp, c.Elem.C, cppDeleter(c.Elem), c.Recv, cppOutput(c.Elem, "owner.values[0]"))
	}
	for _, conversion := range []struct {
		name, symbol string
		target       *abi.Type
	}{
		{"send_only", c.SendOnly, c.SendType}, {"recv_only", c.RecvOnly, c.RecvType},
	} {
		if conversion.symbol == "" {
			continue
		}
		fmt.Fprintf(b, "%s.def(%q, [](py::handle self) -> py::object {\nauto handle = handle_arg<%s>(self);\n", cls, conversion.name, t.Cpp)
		cppOwnedCall(b, conversion.target, conversion.symbol+"(handle)")
		b.WriteString("});\n")
	}
}
