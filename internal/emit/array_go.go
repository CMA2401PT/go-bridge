package emit

import (
	"fmt"
	"go-bridge/internal/abi"
	"strings"
)

func goArray(b *strings.Builder, a *abi.Array) {
	t, e := a.Type, a.Elem
	if a.New != "" {
		goExport(b, a.Zero, "", t.C)
		fmt.Fprintf(b, "out = bridgePin(new(%s))\nreturn\n}\n", t.Go)
		goExport(b, a.New, "data *C."+e.C, t.C)
		fmt.Fprintf(b, "value := new(%s)\nfor i,v := range unsafe.Slice(data,%d) {value[i] = %s}\nout = bridgePin(value)\nreturn\n}\n", t.Go, a.Length, goInput(e, "v"))
	}
	goExport(b, a.At, "handle C.uintptr_t,index C.size_t", e.C)
	fmt.Fprintf(b, "value := bridgeGet[%s](handle)\n", t.Cell)
	if e.Python.Variable() {
		b.WriteString("out = bridgePin(&value[int(index)])\n")
	} else {
		goOutput(b, e, "out", "value[int(index)]")
	}
	b.WriteString("return\n}\n")
	goExport(b, a.Set, "handle C.uintptr_t,index C.size_t,value C."+e.C, "")
	fmt.Fprintf(b, "bridgeGet[%s](handle)[int(index)] = %s\n}\n", t.Cell, goInput(e, "value"))
	goExport(b, a.Copy, "handle C.uintptr_t,data *C."+e.C, "")
	fmt.Fprintf(b, "output := unsafe.Slice(data,%d)\nfor i,v := range bridgeGet[%s](handle) {\n", a.Length, t.Cell)
	goOutput(b, e, "output[i]", "v")
	b.WriteString("}\n}\n")
}
