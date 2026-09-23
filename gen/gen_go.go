package gen

import (
	"fmt"
	"strings"
)

func GenerateGoExport(ir *PackageIR) string {
	var b strings.Builder

	fmt.Fprintf(&b, "package %s\n\n", ir.BridgePackage)

	b.WriteString("/*\n#include <stdlib.h>\n*/\n")
	b.WriteString("import \"C\"\n")
	fmt.Fprintf(&b, "import %s \"%s\"\n\n",
		ir.TargetPackage, ir.TargetImport)

	for _, v := range ir.Vars {
		genGoVar(&b, ir.TargetPackage, v)
	}

	return b.String()
}
func genGoVar(b *strings.Builder, pkg string, v VarIR) {
	switch v.TypeKind {
	case TypeString:
		fmt.Fprintf(b, "//export %s_%s\n", pkg, v.GoName)
		fmt.Fprintf(b,
			"func %s_%s() *C.char { return C.CString(%s.%s) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

		fmt.Fprintf(b, "//export %s_Set_%s\n", pkg, v.GoName)
		fmt.Fprintf(b,
			"func %s_Set_%s(val *C.char) { %s.%s = C.GoString(val) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

	case TypeInt:
		fmt.Fprintf(b, "//export %s_%s\n", pkg, v.GoName)
		fmt.Fprintf(b, "func %s_%s() C.longlong { return C.longlong(%s.%s) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

		fmt.Fprintf(b, "//export %s_Set_%s\n", pkg, v.GoName)
		fmt.Fprintf(b, "func %s_Set_%s(val C.longlong) { %s.%s = int(val) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

	case TypeAliasInt:
		fmt.Fprintf(b, "//export %s_%s\n", pkg, v.GoName)
		fmt.Fprintf(b,
			"func %s_%s() C.longlong { return C.longlong(%s.%s) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

		fmt.Fprintf(b, "//export %s_Set_%s\n", pkg, v.GoName)
		fmt.Fprintf(b,
			"func %s_Set_%s(val C.longlong) { %s.%s = %s.%s(val) }\n\n",
			pkg, v.GoName, pkg, v.GoName, pkg, v.GoType)

	case TypeUint:
		fmt.Fprintf(b, "//export %s_%s\n", pkg, v.GoName)
		fmt.Fprintf(b, "func %s_%s() C.ulonglong { return C.ulonglong(%s.%s) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

		fmt.Fprintf(b, "//export %s_Set_%s\n", pkg, v.GoName)
		fmt.Fprintf(b, "func %s_Set_%s(val C.ulonglong) { %s.%s = uint(val) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

	case TypeFloat64:
		fmt.Fprintf(b, "//export %s_%s\n", pkg, v.GoName)
		fmt.Fprintf(b, "func %s_%s() C.double { return C.double(%s.%s) }\n\n",
			pkg, v.GoName, pkg, v.GoName)

		fmt.Fprintf(b, "//export %s_Set_%s\n", pkg, v.GoName)
		fmt.Fprintf(b, "func %s_Set_%s(val C.double) { %s.%s = float64(val) }\n\n",
			pkg, v.GoName, pkg, v.GoName)
	}
}
