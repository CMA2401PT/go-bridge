package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
	"go-bridge/internal/pyapi"
)

// goSlice renders typed buffer transfer and ordinary Go slice operations.
// C++ validates Python indexes; internal ABI violations panic normally in Go.
func goSlice(b *strings.Builder, s *abi.Slice) {
	t, e := s.Type, s.Elem
	if s.New != "" {
		fmt.Fprintf(b, `
//export %[1]s
func %[1]s(data *C.%[2]s, size C.size_t) C.uintptr_t {
    input := unsafe.Slice(data, int(size))
    result := make(%[3]s, len(input))
    for i, v := range input { result[i] = %[4]s }
    return bridgePin(result)
}
`, s.New, e.C, t.Cell, goInput(e, "v"))
	}
	fmt.Fprintf(b, "\n//export %s\nfunc %s(handle C.uintptr_t) C.size_t {return C.size_t(len(bridgeGet[%s](handle)))}\n", s.Len, s.Len, t.Cell)
	goExport(b, s.At, "handle C.uintptr_t, index C.size_t", e.C)
	if e.Python.Shape == pyapi.StructObject {
		fmt.Fprintf(b, "value := bridgeGet[%s](handle)\nout = bridgePin(&value[int(index)])\n", t.Cell)
	} else {
		fmt.Fprintf(b, "value := bridgeGet[%s](handle)[int(index)]\n", t.Cell)
		goOutput(b, e, "out", "value")
	}
	b.WriteString("return\n}\n")
	fmt.Fprintf(b, "\n//export %s\nfunc %s(handle C.uintptr_t, index C.size_t, value C.%s) {\nbridgeGet[%s](handle)[int(index)] = %s\n}\n", s.Set, s.Set, e.C, t.Cell, goInput(e, "value"))
	fmt.Fprintf(b, `
//export %[1]s
func %[1]s(handle C.uintptr_t, start, stop C.size_t) C.uintptr_t {
    return bridgePin(bridgeGet[%[2]s](handle)[int(start):int(stop)])
}
//export %[3]s
func %[3]s(handle C.uintptr_t) C.uintptr_t {
    value := bridgeGet[%[2]s](handle)
    return bridgePin(append(value[:0:0], value...))
}
//export %[4]s
func %[4]s(handle C.uintptr_t, data *C.%[5]s, size C.size_t) {
    value := bridgeGet[%[2]s](handle)
    if int(size) != len(value) { panic("slice copy length mismatch") }
    output := unsafe.Slice(data, int(size))
    for i, v := range value {
`, s.Reslice, t.Cell, s.Clone, s.Copy, e.C)
	goOutput(b, e, "output[i]", "v")
	b.WriteString("}\n}\n")
}
