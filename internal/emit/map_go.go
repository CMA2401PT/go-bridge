package emit

import (
	"fmt"
	"go-bridge/internal/abi"
	"strings"
)

func goMap(b *strings.Builder, m *abi.Map) {
	t, k, e := m.Type, m.Key, m.Elem
	if m.New != "" {
		goExport(b, m.New, "keys *C."+k.C+", values *C."+e.C+", size C.size_t", t.C)
		fmt.Fprintf(b, "value := make(%s,int(size))\ninput := unsafe.Slice(values,int(size))\nfor i,key := range unsafe.Slice(keys,int(size)) { value[%s] = %s }\n", t.Go, goInput(k, "key"), goInput(e, "input[i]"))
		goOutput(b, t, "out", "value")
		b.WriteString("return\n}\n")
	}
	goExport(b, m.Len, "handle C.uintptr_t", "size_t")
	fmt.Fprintf(b, "return C.size_t(len(%s))\n}\n", goInput(t, "handle"))
	goExport(b, m.Get, "handle C.uintptr_t, key C."+k.C, m.Result)
	fmt.Fprintf(b, "value,ok := (%s)[%s]\nif !ok {return}\nout.ok = 1\n", goInput(t, "handle"), goInput(k, "key"))
	goOutput(b, e, "out.value", "value")
	b.WriteString("return\n}\n")
	goExport(b, m.Has, "handle C.uintptr_t, key C."+k.C, "uint8_t")
	fmt.Fprintf(b, "_,ok := (%s)[%s]\nif ok {return 1};return 0\n}\n", goInput(t, "handle"), goInput(k, "key"))
	goExport(b, m.Set, "handle C.uintptr_t, key C."+k.C+", value C."+e.C, "")
	fmt.Fprintf(b, "(%s)[%s] = %s\n}\n", goInput(t, "handle"), goInput(k, "key"), goInput(e, "value"))
	goExport(b, m.Delete, "handle C.uintptr_t, key C."+k.C, "")
	fmt.Fprintf(b, "delete(%s,%s)\n}\n", goInput(t, "handle"), goInput(k, "key"))
	goExport(b, m.Clear, "handle C.uintptr_t", "")
	fmt.Fprintf(b, "clear(%s)\n}\n", goInput(t, "handle"))
	goExport(b, m.Clone, "handle C.uintptr_t", t.C)
	fmt.Fprintf(b, "source := %s\nif source == nil {return}\nvalue := make(%s,len(source))\nfor k,v := range source {value[k]=v}\n", goInput(t, "handle"), t.Go)
	goOutput(b, t, "out", "value")
	b.WriteString("return\n}\n")
	goExport(b, m.Items, "handle C.uintptr_t, keys *C."+k.C+", values *C."+e.C, "")
	fmt.Fprintf(b, "source := %s\nks,vs := unsafe.Slice(keys,len(source)),unsafe.Slice(values,len(source))\ni:=0\nfor key,value := range source {\n", goInput(t, "handle"))
	goOutput(b, k, "ks[i]", "key")
	goOutput(b, e, "vs[i]", "value")
	b.WriteString("i++\n}\n}\n")
}
